package main

import (
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"

	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
)

type EventGridEvent struct {
	Subject     string          `json:"subject"`
	Id          string          `json:"id"`
	EventType   string          `json:"eventType"`
	Data        json.RawMessage `json:"data"`
	DataVersion string          `json:"dataVersion"`
}

type QueueMessage struct {
	InstallationId  int64  `json:"InstallationId"`
	Owner           string `json:"Owner"`
	RepositoryName  string `json:"RepositoryName"`
	WorkflowRunId   int64  `json:"WorkflowRunId"`
	PullRequestNums []int  `json:"PullRequestNums"`
}

type GitHubAppsManifestHookAttrs struct {
	Url    string `json:"url"`
	Active bool   `json:"active,omitempty"`
}

type GitHubAppsManifest struct {
	Name               string                         `json:"name,omitempty"`
	Url                string                         `json:"url"`
	HookAttrs          GitHubAppsManifestHookAttrs    `json:"hook_attributes,omitempty"`
	RedirectUrl        string                         `json:"redirect_url,omitempty"`
	CallbackUrls       []string                       `json:"callback_urls,omitempty"`
	Description        string                         `json:"description,omitempty"`
	Public             bool                           `json:"public,omitempty"`
	DefaultEvents      []string                       `json:"default_events,omitempty"`
	DefaultPermissions github.InstallationPermissions `json:"default_permissions,omitempty"`
}

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func install_github_app(c echo.Context) error {
	if c.Request().URL.Query().Get("code") != "" {
		return post_install_github_app(c)
	}

	env := c.Get("Env").(*Env)
	connStr, err := env.StorageConnectionString()
	if err != nil {
		return err
	}

	container, blob := "install", "azuredeploy.json"

	cred, err := newAzblobCredential(*connStr)
	if err != nil {
		return err
	}

	conurl, err := ensureContainer(context.Background(), cred, container, defaultContainerTemplate)
	if err != nil {
		panic(err)
	}

	b := conurl.NewBlobURL(blob)
	bloburl, err := newBlobUrlWithSas(cred, &b, 15, false, true)

	blobWoSas := conurl.NewBlobURL(blob)
	exists, err := existsBlob(context.Background(), &blobWoSas)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("Already setup. If retry, please remove /%s/%s", container, blob)
	}

	state := bloburl.String()

	webhookUrl := *c.Request().URL
	webhookUrl.Path = "/api/webhook"

	redirecturl := c.Request().URL
	redirecturl.RawQuery = ""

	write := "write"
	read := "read"
	// https://docs.github.com/en/developers/apps/creating-a-github-app-from-a-manifest
	manifest := GitHubAppsManifest{
		Name:        "CancelWorkflowRun",
		Url:         redirecturl.String(),
		RedirectUrl: redirecturl.String(),
		HookAttrs: GitHubAppsManifestHookAttrs{
			Url: webhookUrl.String(),
		},
		Public:        false,
		DefaultEvents: []string{"workflow_run"},
		DefaultPermissions: github.InstallationPermissions{
			Actions:      &write,
			PullRequests: &write,
			Metadata:     &read,
		},
	}
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

func post_install_github_app(c echo.Context) error {
	code := c.Request().URL.Query().Get("code")
	state := c.Request().URL.Query().Get("state")

	env := c.Get("Env").(*Env)
	connStr, err := env.StorageConnectionString()
	if err != nil {
		return err
	}

	cred, err := newAzblobCredential(*connStr)
	if err != nil {
		return err
	}

	bloburl, err := newBlobUrlFromSas(state)
	if err != nil {
		return err
	}
	created, err := touchIfAbsent(context.Background(), bloburl)

	client := github.NewClient(nil)
	appconf, _, err := client.Apps.CompleteAppManifest(context.Background(), code)
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
	if err := c.Echo().Renderer.Render(deployjson, "", data, c); err != nil {
		return err
	}
	err = putIfUnmodified(context.Background(), bloburl, deployjson.String(), created)
	if err != nil {
		return err
	}

	refurl, err := newBlobUrlWithSas(cred, bloburl, 15, true, false)
	deployurl := fmt.Sprintf("https://portal.azure.com/#create/Microsoft.Template/uri/%s", url.QueryEscape(refurl.String()))

	return c.Redirect(http.StatusFound, deployurl)
}

func webhook(c echo.Context) error {
	env := c.Get("Env").(*Env)
	webhookSecret, err := env.WebhookSecret()
	if err != nil {
		return err
	}

	payload, err := github.ValidatePayload(c.Request(), webhookSecret)
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

		} else {
			whPayload := new(github.WebHookPayload)
			if err := json.Unmarshal(payload, whPayload); err != nil {
				return err
			}

			var pullRequestNums []int
			for _, pr := range event.GetWorkflowRun().PullRequests {
				pullRequestNums = append(pullRequestNums, pr.GetNumber())
			}
			msg := QueueMessage{
				InstallationId:  whPayload.GetInstallation().GetID(),
				Owner:           event.GetRepo().GetOwner().GetLogin(),
				RepositoryName:  event.GetRepo().GetName(),
				WorkflowRunId:   event.GetWorkflowRun().GetID(),
				PullRequestNums: pullRequestNums,
			}
			rawmsg, err := json.Marshal(msg)
			if err != nil {
				return err
			}
			evt := EventGridEvent{
				Id:          "test",
				Subject:     "testsubject",
				EventType:   "testtype",
				Data:        rawmsg,
				DataVersion: "testversion",
			}
			outputs := map[string]interface{}{"msg": []interface{}{evt}}
			c.Set("Outputs", outputs)
			return c.NoContent(http.StatusAccepted)
		}

	default:
		return fmt.Errorf("Unsupported Event Type %s", event)
	}
}

func process(c echo.Context) error {
	env := c.Get("Env").(*Env)
	appId, err := env.AppId()
	if err != nil {
		return err
	}
	secret, err := env.Secret()
	if err != nil {
		return err
	}

	request := new(invokeRequest)
	if err := c.Bind(request); err != nil {
		return err
	}

	rawevent := request.Data["event"]
	event := new(EventGridEvent)
	if err := json.Unmarshal(rawevent, event); err != nil {
		return err
	}

	msg := new(QueueMessage)
	if err := json.Unmarshal(event.Data, msg); err != nil {
		return err
	}

	transport := http.DefaultTransport
	installationTransport, err := ghinstallation.New(transport, appId, msg.InstallationId, secret)
	if err != nil {
		return err
	}
	client := github.NewClient(&http.Client{Transport: installationTransport})

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

type Env struct{}

func NewEnv() *Env {
	return &Env{}
}

func (_ *Env) Port() string {
	port, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if !exists {
		port = "8080"
	}
	return port
}

func (*Env) AppId() (int64, error) {
	appId, present := os.LookupEnv("APP_ID")
	if !present {
		return 0, fmt.Errorf("no APP_ID specified.")
	}
	appIdInt, err := strconv.ParseInt(appId, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("incorrect APP_ID")
	}
	return appIdInt, nil
}

func (*Env) WebhookSecret() ([]byte, error) {
	webhookSecret, present := os.LookupEnv("WEBHOOK_SECRET")
	if !present {
		return nil, fmt.Errorf("no WEBHOOK_SECRET specified.")
	}
	return []byte(webhookSecret), nil
}

func (*Env) Secret() ([]byte, error) {
	secretBase64, present := os.LookupEnv("SECRET")
	if !present {
		return nil, fmt.Errorf("no SECRET specified.")
	}
	secret, err := base64.StdEncoding.DecodeString(secretBase64)
	if err != nil {
		return nil, fmt.Errorf("incorrect SECRET.")
	}

	return []byte(secret), nil
}

func (*Env) StorageConnectionString() (*string, error) {
	connStr, present := os.LookupEnv("AzureWebJobsStorage")
	if !present {
		return nil, fmt.Errorf("no AzureWebJobsStorage found.")
	}
	return &connStr, nil
}

func (e *Env) InjectEnv(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("Env", e)
		return next(c)
	}
}

func main() {
	env := NewEnv()

	e := echo.New()
	if l, ok := e.Logger.(*log.Logger); ok {
		l.SetHeader("${time_rfc3339} ${level}")
	}

	e.Renderer = newTemplateRenderer()

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(env.InjectEnv)

	e.Use(middleware.BodyDump(func(c echo.Context, req, res []byte) {
		fmt.Printf("REQ: %s\n", req)
		fmt.Printf("RES: %s\n", res)
	}))

	e.POST("/hello", hello, azureFunctionsHttpAware("req"))
	e.POST("/install_github_app", install_github_app, azureFunctionsHttpAware("req"))
	e.POST("/webhook", webhook, azureFunctionsHttpAware("req"))
	e.POST("/process", process)
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	e.Logger.Fatal(e.Start(":" + env.Port()))
}

// vim:set noet:
