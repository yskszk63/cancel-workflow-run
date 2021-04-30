package main

import (
	"github.com/google/go-github/v35/github"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"

	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type queueMessage struct {
	InstallationId  int64  `json:"InstallationId"`
	Owner           string `json:"Owner"`
	RepositoryName  string `json:"RepositoryName"`
	WorkflowRunId   int64  `json:"WorkflowRunId"`
	PullRequestNums []int  `json:"PullRequestNums"`
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func setupGitHubApp(c echo.Context) error {
	if c.Request().URL.Query().Get("code") != "" {
		return postSetupGitHubApp(c)
	}

	env := getEnv(c)
	connStr := env.storageConnectionString()

	container, blob := "setup", "azuredeploy.json"

	cred, err := newAzblobCredential(connStr)
	if err != nil {
		return err
	}

	conurl, err := ensureContainer(env, context.Background(), cred, container)
	if err != nil {
		return err
	}

	b := conurl.NewBlobURL(blob)
	bloburl, err := newBlobUrlWithSas(env, cred, &b, 15, false, true)
	if err != nil {
		return err
	}

	blobWoSas := conurl.NewBlobURL(blob)
	exists, err := existsBlob(context.Background(), &blobWoSas)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("Already setup. If retry, please remove /%s/%s", container, blob)
	}
	state := bloburl.String()

	manifest := newGitHubAppsManifest("CancelWorkflowRun", *c.Request().URL, "/api/webhook")
	manifestJson, err := json.Marshal(manifest)
	if err != nil {
		return err
	}

	data := struct {
		Manifest string
		State    string
	}{
		Manifest: string(manifestJson),
		State:    state,
	}

	return c.Render(http.StatusOK, "post_manifest.html", data)
}

func postSetupGitHubApp(c echo.Context) error {
	query := new(gitHubAppsManifestResult)
	if err := c.Bind(query); err != nil {
		return err
	}

	env := getEnv(c)
	connStr := env.storageConnectionString()

	cred, err := newAzblobCredential(connStr)
	if err != nil {
		return err
	}

	bloburl, err := newBlobUrlFromSas(query.State)
	if err != nil {
		return err
	}
	created, err := touchIfAbsent(context.Background(), bloburl)
	if err != nil {
		return err
	}

	appconf, err := completeAppManifest(context.Background(), env, query)
	if err != nil {
		return err
	}

	deployjson := bytes.NewBufferString("")
	data := struct {
		AppId         int64
		WebHookSecret string
		Secret        string
	}{
		AppId:         appconf.GetID(),
		WebHookSecret: appconf.GetWebhookSecret(),
		Secret:        base64.StdEncoding.EncodeToString([]byte(appconf.GetPEM())),
	}
	if err := c.Echo().Renderer.Render(deployjson, "setup.json", data, c); err != nil {
		return err
	}
	err = putIfUnmodified(context.Background(), bloburl, deployjson.String(), created)
	if err != nil {
		return err
	}

	refurl, err := newBlobUrlWithSas(env, cred, bloburl, 15, true, false)
	if err != nil {
		return err
	}
	deployurl := fmt.Sprintf("https://portal.azure.com/#create/Microsoft.Template/uri/%s", url.QueryEscape(refurl.String()))

	return c.Redirect(http.StatusFound, deployurl)
}

func webhook(c echo.Context) error {
	payload, err := io.ReadAll(c.Request().Body)
	if err != nil {
		return err
	}
	event, err := github.ParseWebHook(github.WebHookType(c.Request()), payload)
	if err != nil {
		return err
	}

	switch event := event.(type) {
	case *github.PingEvent:
		return c.NoContent(http.StatusNoContent)

	case *github.InstallationEvent:
		return c.NoContent(http.StatusNoContent)

	case *github.WorkflowRunEvent:
		if event.GetAction() == "completed" {
			return c.NoContent(http.StatusNoContent)
		}

		whPayload := new(github.WebHookPayload)
		if err := json.Unmarshal(payload, whPayload); err != nil {
			return err
		}

		var pullRequestNums []int
		for _, pr := range event.GetWorkflowRun().PullRequests {
			pullRequestNums = append(pullRequestNums, pr.GetNumber())
		}
		msg := queueMessage{
			InstallationId:  whPayload.GetInstallation().GetID(),
			Owner:           event.GetRepo().GetOwner().GetLogin(),
			RepositoryName:  event.GetRepo().GetName(),
			WorkflowRunId:   event.GetWorkflowRun().GetID(),
			PullRequestNums: pullRequestNums,
		}
		evt, err := newEventGridEvent(fmt.Sprintf("%d", whPayload.GetInstallation().GetID()), "CancelWorkflowRunJob", "0", msg)
		if err != nil {
			return err
		}
		setOutput(c, "msg", evt)
		return c.NoContent(http.StatusAccepted)

	default:
		return fmt.Errorf("Unsupported Event Type %s", event)
	}
}

func process(c echo.Context) error {
	env := getEnv(c)

	request := new(invokeRequest)
	if err := c.Bind(request); err != nil {
		return err
	}

	rawevent := request.Data["event"]
	event := new(eventGridEvent)
	if err := json.Unmarshal(rawevent, event); err != nil {
		return err
	}

	msg := new(queueMessage)
	if err := json.Unmarshal(event.Data, msg); err != nil {
		return err
	}

	client, err := newGitHubClientAsApp(env, msg.InstallationId)
	if err != nil {
		return err
	}

	run, _, err := client.Actions.GetWorkflowRunByID(context.Background(), msg.Owner, msg.RepositoryName, msg.WorkflowRunId)
	if err != nil {
		return err
	}

	workflow, _, err := client.Actions.GetWorkflowByID(context.Background(), msg.Owner, msg.RepositoryName, run.GetWorkflowID())
	if err != nil {
		return err
	}

	for _, prnum := range msg.PullRequestNums {
		pr, _, err := client.PullRequests.Get(context.Background(), msg.Owner, msg.RepositoryName, prnum)
		if err != nil {
			return err
		}

		prfiles, _, err := client.PullRequests.ListFiles(context.Background(), msg.Owner, msg.RepositoryName, pr.GetNumber(), nil)
		if err != nil {
			return err
		}

		for _, prfile := range prfiles {
			if prfile.GetFilename() == workflow.GetPath() && prfile.GetStatus() == "added" {
				response, _ := client.Actions.CancelWorkflowRunByID(context.Background(), msg.Owner, msg.RepositoryName, run.GetID())
				c.Echo().Logger.Infof("%s", response)

				commentTextBuf := bytes.NewBufferString("")
				data := struct {
					Opener string
					Owner  string
					RunUrl string
				}{
					Opener: pr.GetUser().GetLogin(),
					Owner:  msg.Owner,
					RunUrl: run.GetHTMLURL(),
				}
				if err := c.Echo().Renderer.Render(commentTextBuf, "comment.md", data, c); err != nil {
					return err
				}
				commentText := commentTextBuf.String()
				comment := github.IssueComment{Body: &commentText}

				_, _, err = client.Issues.CreateComment(context.Background(), msg.Owner, msg.RepositoryName, pr.GetNumber(), &comment)
				if err != nil {
					return err
				}
			}
		}
	}

	response := invokeResponse{}
	return c.JSON(http.StatusOK, response)
}

func main() {
	env := newEnv()

	e := echo.New()
	if l, ok := e.Logger.(*log.Logger); ok {
		l.SetHeader("${time_rfc3339} ${level}")
	}

	e.Renderer = newTemplateRenderer()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(injectEnv(env))

	e.Use(middleware.BodyDump(func(c echo.Context, req, res []byte) {
		fmt.Printf("REQ: %s\n", req)
		fmt.Printf("RES: %s\n", res)
	}))

	e.POST("/hello", hello, azureFunctionsHttpAware("req"))
	e.POST("/setup_github_app", setupGitHubApp, azureFunctionsHttpAware("req"))
	e.POST("/webhook", webhook, azureFunctionsHttpAware("req"), validatePayload)
	e.POST("/process", process)
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	e.Logger.Fatal(e.Start(":" + env.port()))
}

// vim:set noet:
