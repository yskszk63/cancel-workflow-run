package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/go-github/v35/github"
	"github.com/labstack/echo/v4"
)

type testEnv struct {
	env
	conTemplate string
	ghurl       string
}

func newTestEnv(url string) env {
	return &testEnv{
		env:         newEnv(),
		conTemplate: url + "/%s/%s",
		ghurl:       url,
	}
}

func (*testEnv) storageConnectionString() string {
	return "AccountName=myaccount;AccountKey=cGFzcw=="
}

func (e *testEnv) gitHubBaseUrl() *string {
	return &e.ghurl
}

func (e *testEnv) containerTemplate() *string {
	return &e.conTemplate
}

func (e *testEnv) appId() int64 {
	return 0xcafe
}

func (e *testEnv) secret() []byte {
	return []byte(`-----BEGIN RSA PRIVATE KEY-----
MIIBOwIBAAJBALSUtRDxO8xQOuj3WYzjMGHl/rdzEEVONH9XZrF3bL+Is0PGlR7D
mlDIpo2mgCcK98JK/JqjcG2DeKZaCPubFCkCAwEAAQJBAKT2mDYi+Yqa3EYH+AGR
ZZh5P+icL4fwelq+IC0MuQZ6NNL8MTELCvKE4Q95EjvC1D8GJJ4MsHyIJ5Y6pVpj
p4UCIQDZ59nfH21Waw5IqfBqFnYqkyzUUppEE1262YMvtEyI9wIhANQmbFlONPNw
DZrvvTYVjdGaDxgy0j7mhp42ML6z+iPfAiAZZ9/OFOLxlW/H5xBhvhau5hPu+WaF
E2D1PRD/idz2hwIhAJkf7p57A18eZsOI/NH3trgt8W0u6W+7Jjk1tfM/pnGTAiA4
cwy8RkvtUc2d8Q3p5hqxRMfFci+htH0zTTuu3nsKgQ==
-----END RSA PRIVATE KEY-----`)
}

func (e *testEnv) now() time.Time {
	return time.Unix(0, 0)
}

type testRenderer struct{}

func (testRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return json.NewEncoder(w).Encode(data)
}

func TestHello(t *testing.T) {
	e := echo.New()
	e.GET("/", hello)

	req := httptest.NewRequest("GET", "/", nil)
	res := httptest.NewRecorder()
	e.ServeHTTP(res, req)

	if res.Result().StatusCode != http.StatusOK {
		t.Fail()
	}
}

func TestSetupGitHubApp(t *testing.T) {
	cases := []struct {
		name         string
		wantManifest string
		wantState    string
	}{
		{
			name:         "ok",
			wantManifest: `{"name":"CancelWorkflowRun","url":"/","hook_attributes":{"url":"/api/webhook"},"redirect_url":"/","default_events":["workflow_run"],"default_permissions":{"actions":"write","metadata":"read","pull_requests":"write"}}`,
			wantState:    `http://xxx/myaccount/setup/azuredeploy.json?se=1970-01-01T00%3A15%3A00Z&sig=dUIFrvS7Hccv5e8zaDZrUtfsQCJeFH9WKmFbucK03IA%3D&sp=w&spr=https&sr=b&sv=2019-12-12`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/myaccount/setup":
					if r.URL.Query().Get("restype") == "container" {
						w.Header().Add("x-ms-error-code", "ContainerAlreadyExists")
						w.WriteHeader(404)
					}
				case "/myaccount/setup/azuredeploy.json":
					w.Header().Add("x-ms-error-code", "BlobNotFound")
					w.WriteHeader(404)
				default:
					fmt.Printf("%s\n", r.URL)
					w.WriteHeader(501)
				}
			}))
			defer dummy.Close()

			e := echo.New()
			e.Debug = true
			e.Use(injectEnv(newTestEnv(dummy.URL)))
			e.Renderer = testRenderer{}
			e.GET("/", setupGitHubApp)

			req := httptest.NewRequest("GET", "/", nil)
			res := httptest.NewRecorder()
			e.ServeHTTP(res, req)

			if res.Result().StatusCode != http.StatusOK {
				t.Fatalf("%d %s", res.Result().StatusCode, res.Body.String())
			}

			body := struct {
				Manifest string
				State    string
			}{}

			if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if body.Manifest != c.wantManifest {
				t.Fatal(body.Manifest)
			}
			s, err := url.Parse(body.State)
			if err != nil {
				t.Fatal(err)
			}
			s.Host = "xxx"
			if s.String() != c.wantState {
				t.Fatal(s)
			}
		})
	}
}

func TestPostSetupGitHubApp(t *testing.T) {
	cases := []struct {
		name           string
		locationStarts string
		locationEnd    string
	}{
		{
			name:           "ok",
			locationStarts: "",
			locationEnd:    "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/myaccount/setup/azuredeploy.json":
					w.WriteHeader(200)
				case "/api/v3/app-manifests/xxx/conversions":
					w.WriteHeader(200)
				default:
					fmt.Printf("%s\n", r.URL)
					w.WriteHeader(501)
				}
			}))
			defer dummy.Close()

			e := echo.New()
			e.Debug = true
			e.Use(injectEnv(newTestEnv(dummy.URL)))
			e.Renderer = testRenderer{}
			e.GET("/", setupGitHubApp)

			req := httptest.NewRequest("GET", "/?code=xxx&state="+dummy.URL+"/myaccount/setup/azuredeploy.json", nil)
			res := httptest.NewRecorder()
			e.ServeHTTP(res, req)

			if res.Result().StatusCode != http.StatusFound {
				t.Fatalf("%d %s", res.Result().StatusCode, res.Body.String())
			}
			location := res.Header().Get("Location")
			if !strings.HasPrefix(location, c.locationStarts) {
				t.Fatal(location)
			}
			if !strings.HasSuffix(location, c.locationEnd) {
				t.Fatal(location)
			}
		})
	}
}

func TestWebHook(t *testing.T) {
	cases := []struct {
		name      string
		eventName string
		payload   string
		status    int
		hasOutput bool
	}{
		{
			name:      "ping",
			eventName: "ping",
			payload:   "{}",
			status:    http.StatusNoContent,
		},
		{
			name:      "installation",
			eventName: "installation",
			payload:   "{}",
			status:    http.StatusNoContent,
		},
		{
			name:      "workflow_run",
			eventName: "workflow_run",
			payload: `{
	"workflow_run": {
		"pull_requests": [
			{
				"number": 0
			}
		]
	}
}`,
			status:    http.StatusAccepted,
			hasOutput: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				default:
					fmt.Printf("%s\n", r.URL)
					w.WriteHeader(501)
				}
			}))
			defer dummy.Close()

			e := echo.New()
			e.Debug = true
			e.Use(injectEnv(newTestEnv(dummy.URL)))
			e.Use(func(next echo.HandlerFunc) echo.HandlerFunc {
				return func(ctx echo.Context) error {
					if err := next(ctx); err != nil {
						return err
					}
					outputs, exists := ctx.Get("Outputs").(map[string]interface{})
					if exists != c.hasOutput {
						t.Fail()
					}
					_, exists = outputs["msg"]
					if exists != c.hasOutput {
						t.Fail()
					}
					return nil
				}
			})
			e.Renderer = testRenderer{}
			e.POST("/", webhook)

			req := httptest.NewRequest("POST", "/", bytes.NewBufferString(c.payload))
			req.Header.Set("X-GitHub-Event", c.eventName)
			res := httptest.NewRecorder()
			e.ServeHTTP(res, req)

			if res.Result().StatusCode != c.status {
				t.Fatalf("%d %s", res.Result().StatusCode, res.Body.String())
			}
		})
	}
}

func TestProcess(t *testing.T) {
	cases := []struct {
		name string
	}{
		{
			name: "ok",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/app/installations/0/access_tokens":
					token := github.InstallationToken{}
					w.WriteHeader(200)
					encoder := json.NewEncoder(w)
					encoder.Encode(token)
				case "/api/v3/repos///actions/runs/0":
					w.WriteHeader(200)
				case "/api/v3/repos///actions/workflows/0":
					filename := "ok.txt"
					data := github.Workflow{
						Path: &filename,
					}
					w.WriteHeader(200)
					encoder := json.NewEncoder(w)
					encoder.Encode(data)
				case "/api/v3/repos///pulls/0":
					w.WriteHeader(200)
				case "/api/v3/repos///pulls/0/files":
					filename := "ok.txt"
					status := "added"
					data := []github.CommitFile{
						{
							Filename: &filename,
							Status:   &status,
						},
					}
					w.WriteHeader(200)
					encoder := json.NewEncoder(w)
					encoder.Encode(data)
				case "/api/v3/repos///actions/runs/0/cancel":
					w.WriteHeader(200)
				case "/api/v3/repos///issues/0/comments":
					w.WriteHeader(200)
				default:
					fmt.Printf("%s\n", r.URL)
					w.WriteHeader(501)
				}
			}))
			defer dummy.Close()

			e := echo.New()
			e.Debug = true
			e.Use(injectEnv(newTestEnv(dummy.URL)))
			e.Renderer = testRenderer{}
			e.POST("/", process)

			msg := queueMessage{
				PullRequestNums: []int{0},
			}
			payload, err := newEventGridEvent("subject", "event", "version", msg)
			if err != nil {
				t.Fatal(err)
			}
			j, err := json.Marshal(payload)
			if err != nil {
				t.Fatal(err)
			}
			body, err := json.Marshal(invokeRequest{
				Data: map[string]json.RawMessage{
					"event": j,
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest("POST", "/", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			e.ServeHTTP(res, req)

			if res.Result().StatusCode != http.StatusOK {
				t.Fatalf("%d %s", res.Result().StatusCode, res.Body.String())
			}
			if strings.TrimSpace(res.Body.String()) != "{}" {
				t.Fatalf("%s", res.Body.String())
			}
		})
	}
}
