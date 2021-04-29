package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNewGitHubAppsManifest(t *testing.T) {
	url, err := url.Parse("http://example.com/foo/bar?q=v")
	if err != nil {
		t.Fatal(err)
	}

	manifest := newGitHubAppsManifest("test", *url, "/api/webhook")
	if manifest.Name != "test" {
		t.Fatal(manifest.Name)
	}
	if manifest.Url != "http://example.com/foo/bar" {
		t.Fatal(manifest.Url)
	}
	if manifest.Url != manifest.RedirectUrl {
		t.Fatal(manifest.RedirectUrl)
	}
	if manifest.HookAttrs.Url != "http://example.com/api/webhook" {
		t.Fatal(manifest.HookAttrs.Url)
	}
}

type testenv struct {
	env
	baseUrl string
}

func (e *testenv) gitHubBaseUrl() *string {
	return &e.baseUrl
}

func newTestenv(c *httptest.Server) env {
	e := newEnv()
	url := c.URL
	return &testenv{
		env:     e,
		baseUrl: url,
	}
}

func TestCompleteAppManifest(t *testing.T) {
	cases := []struct {
		name   string
		status int
		haserr bool
	}{
		{
			name:   "ok",
			status: 201,
			haserr: false,
		},
		{
			name:   "notfound",
			status: 404,
			haserr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dummy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(c.status)
			}))
			defer dummy.Close()

			env := newTestenv(dummy)

			result := gitHubAppsManifestResult{}
			_, err := completeAppManifest(context.Background(), env, &result)
			if c.haserr != (err != nil) {
				t.Fatal(err)
			}
		})
	}
}

type testValidatePayloadEnv struct {
	env
	wbsec string
}

func (t *testValidatePayloadEnv) webhookSecret() []byte {
	return []byte(t.wbsec)
}

func TestValidatePayload(t *testing.T) {
	cases := []struct {
		name             string
		secret           string
		payload          string
		requestsignature string
		status           int
	}{
		{
			name:             "ok",
			secret:           "secret",
			payload:          "empty",
			requestsignature: "sha256=9051c49f2eed4cfe8125cba47a275e1460402ad2a4bc94d417fd1e10d9c55339",
			status:           200,
		},
		{
			name:             "bad",
			secret:           "secret",
			payload:          "empty",
			requestsignature: "sha256=xxxx",
			status:           400,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			env := testValidatePayloadEnv{
				env:   &defaultEnv{},
				wbsec: c.secret,
			}

			e := echo.New()
			e.Use(injectEnv(&env))
			e.Use(validatePayload)
			e.GET("/", func(c echo.Context) error {
				return c.NoContent(http.StatusOK)
			})

			req := httptest.NewRequest("GET", "/", bytes.NewBufferString(c.payload))
			req.Header.Set("X-Hub-Signature-256", c.requestsignature)
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()
			e.ServeHTTP(res, req)

			if res.Result().StatusCode != c.status {
				t.Fatalf("%d: %s", res.Result().StatusCode, res.Body.String())
			}
		})
	}
}
