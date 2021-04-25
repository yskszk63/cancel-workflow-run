package main

import (
	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/labstack/gommon/log"
	"github.com/Azure/azure-storage-blob-go/azblob"

	_ "embed"
	"time"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"text/template"
	"net/url"
)

func ParseConnectionString(s string) (*string, *string, error) {
	var account *string
	var key *string
	for _, kv := range strings.Split(s, ";") {
		kv := strings.SplitN(kv, "=", 2)
		k, v := kv[0], kv[1]
		switch k {
		case "AccountName":
			account = &v
		case "AccountKey":
			key = &v
		}
	}
	if account == nil || key == nil {
		return nil, nil, fmt.Errorf("no AccountName or AccountKey")
	}
	return account, key, nil
}

type HttpTriggerBinding struct {
	Url        string                   `json:"Url,omitempty"`
	Method     string                   `json:"Method,omitempty"`
	Query      map[string]string        `json:"Query,omitempty"`
	Headers    map[string][]string      `json:"Headers,omitempty"`
	Params     map[string]string        `json:"Params,omitempty"`
	Identities []map[string]interface{} `json:"Identities,omitempty"`
	Body       string                   `json:"Body,omitempty"`
}

type InvokeRequest struct {
	Data     map[string]json.RawMessage `json:"Data,omitempty"`
	Metadata map[string]interface{}     `json:"Metadata,omitempty"`
}

func (i *InvokeRequest) Bind(name string, m interface{}) error {
	// RawMessage to json string
	var data string
	if err := json.Unmarshal(i.Data[name], &data); err != nil {
		return err
	}
	// json string to string
	var data2 string
	if err := json.Unmarshal([]byte(data), &data2); err != nil {
		return err
	}

	return json.Unmarshal([]byte(data2), m)
}

type HttpBindingOutput struct {
	Status  int               `json:"Status"`
	Body    string            `json:"Body"`
	Headers map[string]string `json:"Headers"`
}

type InvokeResponse struct {
	Outputs     map[string]interface{} `json:"Outputs,omitempty"`
	Logs        []string               `json:"Logs,omitempty"`
	ReturnValue HttpBindingOutput      `json:"ReturnValue,omitempty"`
}

type QueueMessage struct {
	InstallationId  int64  `json:"InstallationId"`
	Owner           string `json:"Owner"`
	RepositoryName  string `json:"RepositoryName"`
	WorkflowRunId   int64  `json:"WorkflowRunId"`
	PullRequestNums []int  `json:"PullRequestNums"`
}

type CommentTemplate struct {
	Opener string
	Owner  string
	RunUrl string
}

func (c *CommentTemplate) Render() (*string, error) {
	text := `@{{.Opener}} @{{.Owner}}
Hi, I'm a bot.

Sorry, [This Workflow Run]({{.RunUrl}}) is cancelled.
Because currently could not accept added at pull request.

If needed, please re-run [This Workflow Run]({{.RunUrl}})`

	t, err := template.New("Comment").Parse(text)
	if err != nil {
		return nil, err
	}

	buf := bytes.NewBufferString("")
	t.Execute(buf, c)
	r := buf.String()
	return &r, nil
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

	account, key, err := ParseConnectionString(*connStr)
	if err != nil {
		return err
	}
	container, blob := "install", "azuredeploy.json"

	cred, err := azblob.NewSharedKeyCredential(*account, *key)
	if err != nil {
		panic(err)
	}

	rawconurl, err := url.Parse(fmt.Sprintf("https://%s.blob.core.windows.net/%s", *account, container))
	if err != nil {
		panic(err)
	}
	conurl := azblob.NewContainerURL(*rawconurl, azblob.NewPipeline(cred, azblob.PipelineOptions{}))
	_, err = conurl.Create(context.Background(), azblob.Metadata{}, azblob.PublicAccessNone)
	if err != nil && err.(azblob.StorageError).ServiceCode() != azblob.ServiceCodeContainerAlreadyExists {
		panic(err)
	}

	sas, err := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    time.Now().UTC().Add(15 * time.Minute),
		ContainerName: container,
		BlobName:      blob,
		Permissions:   azblob.BlobSASPermissions{Write: true}.String(),
	}.NewSASQueryParameters(cred)
	if err != nil {
		panic(err)
	}
	bloburl := conurl.NewBlockBlobURL(blob).URL()
	bloburl.RawQuery = sas.Encode()
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

	html := `<form action="https://github.com/settings/apps/new" method="POST">
	<textarea name="manifest" hidden>{{.Manifest}}</textarea>
	<input type="hidden" name="state" value="{{.State}}" />
</form>
<script>document.querySelector("form").submit()</script>`
	_ = manifestJson

	t, err := template.New("Html").Parse(html)
	if err != nil {
		return err
	}

	s := struct {
		Manifest string
		State    string
	}{
		Manifest: string(manifestJson),
		State:    state,
	}
	buf := bytes.NewBufferString("")
	t.Execute(buf, s)

	return c.HTML(http.StatusOK, buf.String())
}

//go:embed templates/install.json
var installTemplate []byte

func post_install_github_app(c echo.Context) error {
	code := c.Request().URL.Query().Get("code")
	state := c.Request().URL.Query().Get("state")
	fmt.Printf("**** %s\n", state)

	env := c.Get("Env").(*Env)
	connStr, err := env.StorageConnectionString()
	if err != nil {
		return err
	}

	account, key, err := ParseConnectionString(*connStr)
	if err != nil {
		return err
	}

	rawurl, err := url.Parse(state)
	if err != nil {
		return err
	}
	bloburl := azblob.NewBlockBlobURL(*rawurl, azblob.NewPipeline(azblob.NewAnonymousCredential(), azblob.PipelineOptions{}))

	client := github.NewClient(nil)
	appconf, _, err := client.Apps.CompleteAppManifest(context.Background(), code)
	if err != nil {
		return err
	}

	app_id := appconf.GetID()
	webhookSecret := appconf.GetWebhookSecret()
	secret := appconf.GetPEM()
	secretb64 := base64.StdEncoding.EncodeToString([]byte(secret))

	deployjson := fmt.Sprintf(string(installTemplate), app_id, webhookSecret, secretb64)
	_, err = azblob.UploadBufferToBlockBlob(context.Background(), []byte(deployjson), bloburl, azblob.UploadToBlockBlobOptions{})
	if err != nil {
		return err
	}

	parts := azblob.NewBlobURLParts(bloburl.URL())
	fmt.Printf("%s %s\n", parts.ContainerName, parts.BlobName)
	cred, err := azblob.NewSharedKeyCredential(*account, *key)
	if err != nil {
		panic(err)
	}
	sas, err := azblob.BlobSASSignatureValues{
		Protocol:      azblob.SASProtocolHTTPS,
		ExpiryTime:    time.Now().UTC().Add(15 * time.Minute),
		ContainerName: parts.ContainerName,
		BlobName:      parts.BlobName,
		Permissions:   azblob.BlobSASPermissions{Read: true}.String(),
	}.NewSASQueryParameters(cred)
	if err != nil {
		panic(err)
	}

	refurl := bloburl.URL()
	refurl.RawQuery = sas.Encode()

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

			outputs := map[string]interface{}{
				"msg": msg,
			}
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

	request := new(InvokeRequest)
	if err := c.Bind(request); err != nil {
		return err
	}

	msg := new(QueueMessage)
	if err := request.Bind("msg", msg); err != nil {
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
				c.Echo().Logger.Debugf("%s", response)

				t := CommentTemplate{
					Opener: pr.GetUser().GetLogin(),
					Owner:  msg.Owner,
					RunUrl: run.GetHTMLURL(),
				}
				rendered, err := t.Render()
				if err != nil {
					return err
				}
				comment := github.IssueComment{
					Body: rendered,
				}

				_, _, err = client.Issues.CreateComment(context.Background(), msg.Owner, msg.RepositoryName, pr.GetNumber(), &comment)
				if err != nil {
					return err
				}
			}
		}
	}

	response := InvokeResponse{}
	return c.JSON(http.StatusOK, response)
}

type ProxyContext struct {
	echo.Context
	Parent echo.Context
}

func (p *ProxyContext) Get(key string) interface{} {
	if ret := p.Context.Get(key); ret != nil {
		return ret
	}
	return p.Parent.Get(key)
}

type ProxyWriter struct {
	StatusCode int
	ResHeader  http.Header
	Body       *bytes.Buffer
}

func (p *ProxyWriter) Header() http.Header {
	return p.ResHeader
}
func (p *ProxyWriter) Write(b []byte) (int, error) {
	return p.Body.Write(b)
}
func (p *ProxyWriter) WriteHeader(statusCode int) {
	p.StatusCode = statusCode
}

func AzureFunctionsHttp(name string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			request := new(InvokeRequest)
			if err := c.Bind(request); err != nil {
				return err
			}

			hostReq := new(HttpTriggerBinding)
			if err := json.Unmarshal(request.Data[name], hostReq); err != nil {
				return err
			}

			body := ioutil.NopCloser(bytes.NewReader([]byte(hostReq.Body)))
			newReq, err := http.NewRequest(hostReq.Method, hostReq.Url, body)
			if err != nil {
				return err
			}
			for key, val := range hostReq.Headers {
				for _, v := range val {
					newReq.Header.Set(key, v)
				}
			}

			writer := ProxyWriter{
				ResHeader: http.Header{},
				Body:      bytes.NewBuffer([]byte{}),
			}
			newCtx := c.Echo().NewContext(newReq, &writer)
			newCtx.Set("AzureRequest", request)
			if err := next(&ProxyContext{Context: newCtx, Parent: c}); err != nil {
				return err
			}

			Outputs, ok := newCtx.Get("Outputs").(map[string]interface{})
			if !ok {
				Outputs = map[string]interface{}{}
			}
			headers := map[string]string{}
			for key, val := range newCtx.Response().Header() {
				for _, v := range val {
					headers[key] = v
				}
			}

			invokeResponse := InvokeResponse{
				ReturnValue: HttpBindingOutput{
					Status:  writer.StatusCode,
					Body:    writer.Body.String(),
					Headers: headers,
				},
				Outputs: Outputs,
			}
			return c.JSON(http.StatusOK, invokeResponse)
		}
	}
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
	e.Debug = true
	if l, ok := e.Logger.(*log.Logger); ok {
		l.SetHeader("${time_rfc3339} ${level}")
	}

	e.Use(middleware.Logger())
	e.Use(middleware.Recover())
	e.Use(env.InjectEnv)

	e.Use(middleware.BodyDump(func(c echo.Context, req, res []byte) {
		fmt.Printf("REQ: %s\n", req)
		fmt.Printf("RES: %s\n", res)
	}))

	e.POST("/hello", hello, AzureFunctionsHttp("req"))
	e.POST("/install_github_app", install_github_app, AzureFunctionsHttp("req"))
	e.POST("/webhook", webhook, AzureFunctionsHttp("req"))
	e.POST("/process", process)
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	e.Logger.Fatal(e.Start(":" + env.Port()))
}

// vim:set noet:
