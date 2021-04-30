package main

import (
	"encoding/base64"
	"os"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"
)

type env interface {
	port() string
	appId() int64
	webhookSecret() []byte
	secret() []byte
	storageConnectionString() string
	gitHubBaseUrl() *string
	containerTemplate() *string
	now() time.Time
}

type defaultEnv struct{}

func newEnv() env {
	return &defaultEnv{}
}

func (*defaultEnv) port() string {
	port, exists := os.LookupEnv("FUNCTIONS_CUSTOMHANDLER_PORT")
	if !exists {
		port = "8080"
	}
	return port
}

func (*defaultEnv) appId() int64 {
	appId, present := os.LookupEnv("APP_ID")
	if !present {
		panic("no APP_ID specified.")
	}
	appIdInt, err := strconv.ParseInt(appId, 10, 64)
	if err != nil {
		panic("incorrect APP_ID")
	}
	return appIdInt
}

func (*defaultEnv) webhookSecret() []byte {
	webhookSecret, present := os.LookupEnv("WEBHOOK_SECRET")
	if !present {
		panic("no WEBHOOK_SECRET specified.")
	}
	return []byte(webhookSecret)
}

func (*defaultEnv) secret() []byte {
	secretBase64, present := os.LookupEnv("SECRET")
	if !present {
		panic("no SECRET specified.")
	}
	secret, err := base64.StdEncoding.DecodeString(secretBase64)
	if err != nil {
		panic("incorrect SECRET.")
	}

	return []byte(secret)
}

func (*defaultEnv) storageConnectionString() string {
	connStr, present := os.LookupEnv("AzureWebJobsStorage")
	if !present {
		panic("no AzureWebJobsStorage found.")
	}
	return connStr
}

func (*defaultEnv) gitHubBaseUrl() *string {
	return nil
}

func (*defaultEnv) containerTemplate() *string {
	return nil
}

func (*defaultEnv) now() time.Time {
	return time.Now()
}

func injectEnv(e env) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			c.Set("Env", e)
			return next(c)
		}
	}
}

func getEnv(c echo.Context) env {
	return c.Get("Env").(env)
}
