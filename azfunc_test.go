package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestDelegateContext(t *testing.T) {
	e := echo.New()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	res := httptest.NewRecorder()

	parent := e.NewContext(req, res)
	parent.Set("parent", "OK")
	self := e.NewContext(req, res)
	self.Set("child", "OK")
	ctx := delegateContext{Context: self, parent: parent}
	ctx.Set("child2", "OK")

	if ctx.Get("parent") != "OK" {
		t.Fail()
	}
	if ctx.Get("child") != "OK" {
		t.Fail()
	}
	if ctx.Get("child2") != "OK" {
		t.Fail()
	}
	if ctx.Get("notexists") != nil {
		t.Fail()
	}
	if parent.Get("child2") != nil {
		t.Fail()
	}
}

func TestAzureFunctionsHttpAware(t *testing.T) {
	var cases = []struct {
		name    string
		body    string
		status  int
		message string
		handler echo.HandlerFunc
	}{
		{
			name:    "ok",
			body:    `{"Data": {"req":{"Url":"/", "Method": "GET", "Body": "ok", "Headers": {"x-test": ["ok"]}}}}`,
			status:  http.StatusOK,
			message: `{"ReturnValue":{"Status":200,"Body":"ok","Headers":{"Content-Type":"text/plain; charset=UTF-8"}}}`,
			handler: func(c echo.Context) error {
				body, err := io.ReadAll(c.Request().Body)
				if err != nil {
					return err
				}
				if string(body) != "ok" {
					return fmt.Errorf("%s", body)
				}
				if c.Request().Header["X-Test"][0] != "ok" {
					return fmt.Errorf("no header found")
				}
				return c.String(http.StatusOK, "ok")
			},
		},
		{
			name:    "not found",
			body:    `{"Data": {"req":{"Url":"/", "Method": "GET", "Body": "ok", "Headers": {"x-test": ["ok"]}}}}`,
			status:  http.StatusNotFound,
			message: `{"message":"Not Found"}`,
		},
		{
			name:    "incorrectPayload",
			body:    `0`,
			status:  http.StatusBadRequest,
			message: `{"message":"Unmarshal type error: expected=main.invokeRequest, got=number, field=, offset=1"}`,
		},
		{
			name:    "bindingNotFound",
			body:    `{}`,
			status:  http.StatusBadRequest,
			message: `{"message":"Http binding not found."}`,
		},
		{
			name:    "incorrectData",
			body:    `{"Data": {"req":0}}`,
			status:  http.StatusBadRequest,
			message: `{"message":"Incorrect Http binding."}`,
		},
		{
			name:    "invalidData",
			body:    `{"Data": {"req":{"Url":":"}}}`,
			status:  http.StatusBadRequest,
			message: `{"message":"Incorrect Http binding."}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			e := echo.New()
			e.Use(azureFunctionsHttpAware("req"))

			req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(c.body))
			req.Header.Set("Content-Type", "application/json")
			res := httptest.NewRecorder()

			if c.handler != nil {
				e.POST("/", c.handler)
			}
			e.ServeHTTP(res, req)

			if res.Result().StatusCode != c.status {
				t.Fatalf("%d != %d: %s", c.status, res.Result().StatusCode, res.Body.String())
			}
			if strings.Trim(res.Body.String(), " \r\n") != c.message {
				t.Fatalf("%s != %s", c.message, res.Body.String())
			}
		})
	}
}

func TestSetOutput(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	res := httptest.NewRecorder()
	ctx := echo.New().NewContext(req, res)

	setOutput(ctx, "k", "OK")

	outputs := ctx.Get(contextAttrOutputs)
	if outputs.(map[string]interface{})["k"] != "OK" {
		t.Fail()
	}
}
