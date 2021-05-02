package main

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/gommon/log"

	"encoding/json"
)

func intoJson(b []byte) json.RawMessage {
	var j json.RawMessage
	if err := json.Unmarshal(b, &j); err != nil {
		r, err := json.Marshal(string(b))
		if err != nil {
			panic(err)
		}
		return r
	}

	return j
}

func handleBodyDump(c echo.Context, req, res []byte) {
	body := log.JSON{
		"req": intoJson(req),
		"res": intoJson(res),
	}

	c.Echo().Logger.Infoj(body)
}
