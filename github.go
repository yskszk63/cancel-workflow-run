package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/bradleyfalzon/ghinstallation"
	"github.com/google/go-github/v35/github"
	"github.com/labstack/echo/v4"
)

type gitHubAppsManifestHookAttrs struct {
	Url    string `json:"url"`
	Active bool   `json:"active,omitempty"`
}

type gitHubAppsManifest struct {
	Name               string                         `json:"name,omitempty"`
	Url                string                         `json:"url"`
	HookAttrs          gitHubAppsManifestHookAttrs    `json:"hook_attributes,omitempty"`
	RedirectUrl        string                         `json:"redirect_url,omitempty"`
	CallbackUrls       []string                       `json:"callback_urls,omitempty"`
	Description        string                         `json:"description,omitempty"`
	Public             bool                           `json:"public,omitempty"`
	DefaultEvents      []string                       `json:"default_events,omitempty"`
	DefaultPermissions github.InstallationPermissions `json:"default_permissions,omitempty"`
}

type gitHubAppsManifestResult struct {
	Code  string `query:"code"`
	State string `query:"state"`
}

func newGitHubAppsManifest(name string, funcUrl url.URL, webhookPath string) gitHubAppsManifest {
	redirecturl := funcUrl
	redirecturl.RawQuery = ""

	webhookUrl := funcUrl
	webhookUrl.Path = webhookPath
	webhookUrl.RawQuery = ""

	write := "write"
	read := "read"

	// https://docs.github.com/en/developers/apps/creating-a-github-app-from-a-manifest
	return gitHubAppsManifest{
		Name:        name,
		Url:         redirecturl.String(),
		RedirectUrl: redirecturl.String(),
		HookAttrs: gitHubAppsManifestHookAttrs{
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
}

func newGitHubClient(env env, httpClient *http.Client) *github.Client {
	if baseurl := env.gitHubBaseUrl(); baseurl != nil {
		result, err := github.NewEnterpriseClient(*baseurl, *baseurl, httpClient)
		if err != nil {
			panic(err)
		}
		return result
	}
	return github.NewClient(httpClient)
}

func newGitHubClientAsApp(env env, installationId int64) (*github.Client, error) {
	transport := http.DefaultTransport
	installationTransport, err := ghinstallation.New(transport, env.appId(), installationId, env.secret())
	if err != nil {
		return nil, err
	}
	client := github.NewClient(&http.Client{Transport: installationTransport})
	return client, nil
}

func completeAppManifest(context context.Context, env env, query *gitHubAppsManifestResult) (*github.AppConfig, error) {
	client := newGitHubClient(env, nil)
	appconf, _, err := client.Apps.CompleteAppManifest(context, query.Code)
	if err != nil {
		return nil, err
	}

	return appconf, nil
}

func validatePayload(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		env := getEnv(c)
		webhookSecret := env.webhookSecret()

		payload, err := github.ValidatePayload(c.Request(), webhookSecret)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "signature mismatch.")
		}

		c.Request().Body = io.NopCloser(bytes.NewBuffer(payload))

		return next(c)
	}
}
