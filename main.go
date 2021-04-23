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
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"text/template"
)

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
	Status int    `json:"Status"`
	Body   string `json:"Body"`
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

func hello(c echo.Context) error {
	return c.String(http.StatusOK, "Hello, World!")
}

func webhook(c echo.Context) error {
	env := c.Get("Env").(*Env)

	payload, err := github.ValidatePayload(c.Request(), env.WebhookSecret)
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

	request := new(InvokeRequest)
	if err := c.Bind(request); err != nil {
		return err
	}

	msg := new(QueueMessage)
	if err := request.Bind("msg", msg); err != nil {
		return err
	}

	transport := http.DefaultTransport
	installationTransport, err := ghinstallation.New(transport, env.AppId, msg.InstallationId, env.Secret)
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
			if err := next(&ProxyContext{Context: newCtx, Parent: c}); err != nil {
				return err
			}

			Outputs, ok := newCtx.Get("Outputs").(map[string]interface{})
			if !ok {
				Outputs = map[string]interface{}{}
			}
			invokeResponse := InvokeResponse{
				ReturnValue: HttpBindingOutput{
					Status: writer.StatusCode,
					Body:   writer.Body.String(),
				},
				Outputs: Outputs,
			}
			return c.JSON(http.StatusOK, invokeResponse)
		}
	}
}

type Env struct {
	Port          string
	AppId         int64
	WebhookSecret []byte
	Secret        []byte
}

func NewEnv() (*Env, error) {
	port, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if !exists {
		port = "8080"
	}

	appId, present := os.LookupEnv("APP_ID")
	if !present {
		return nil, fmt.Errorf("no APP_ID specified.")
	}
	appIdInt, err := strconv.ParseInt(appId, 10, 64)
	if !present {
		return nil, fmt.Errorf("incorrect APP_ID")
	}

	webhookSecret, present := os.LookupEnv("WEBHOOK_SECRET")
	if !present {
		return nil, fmt.Errorf("no WEBHOOK_SECRET specified.")
	}

	secretBase64, present := os.LookupEnv("SECRET")
	if !present {
		return nil, fmt.Errorf("no SECRET specified.")
	}
	secret, err := base64.StdEncoding.DecodeString(secretBase64)
	if err != nil {
		return nil, fmt.Errorf("incorrect SECRET.")
	}

	return &Env{
		Port:          port,
		AppId:         appIdInt,
		WebhookSecret: []byte(webhookSecret),
		Secret:        secret,
	}, nil
}

func (e *Env) InjectEnv(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Set("Env", e)
		return next(c)
	}
}

func main() {
	env, err := NewEnv()
	if err != nil {
		panic(err)
	}

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
	e.POST("/webhook", webhook, AzureFunctionsHttp("req"))
	e.POST("/process", process)
	e.GET("/", func(c echo.Context) error { return c.NoContent(http.StatusNoContent) })

	e.Logger.Fatal(e.Start(":" + env.Port))
}

// vim:set noet:
