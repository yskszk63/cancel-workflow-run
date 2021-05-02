package main

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

type invokeRequest struct {
	Data     map[string]json.RawMessage `json:"Data,omitempty"`
	Metadata map[string]interface{}     `json:"Metadata,omitempty"`
}

type invokeResponse struct {
	Outputs     map[string]interface{} `json:"Outputs"`
	Logs        []string               `json:"Logs,omitempty"`
	ReturnValue *httpBindingOut        `json:"ReturnValue,omitempty"`
}

type httpTriggerIn struct {
	Url        string                   `json:"Url,omitempty"`
	Method     string                   `json:"Method,omitempty"`
	Query      map[string]string        `json:"Query,omitempty"`
	Headers    map[string][]string      `json:"Headers,omitempty"`
	Params     map[string]string        `json:"Params,omitempty"`
	Identities []map[string]interface{} `json:"Identities,omitempty"`
	Body       string                   `json:"Body,omitempty"`
}

type httpBindingOut struct {
	Status  int               `json:"Status"`
	Body    string            `json:"Body"`
	Headers map[string]string `json:"Headers"`
}

type delegateContext struct {
	echo.Context
	parent echo.Context
}

func (c *delegateContext) Get(key string) interface{} {
	if ret := c.Context.Get(key); ret != nil {
		return ret
	}
	return c.parent.Get(key)
}

type bufferResponseWriter struct {
	statusCode int
	header     http.Header
	body       *bytes.Buffer
}

func (b *bufferResponseWriter) Header() http.Header {
	return b.header
}

func (b *bufferResponseWriter) Write(buf []byte) (int, error) {
	return b.body.Write(buf)
}

func (b *bufferResponseWriter) WriteHeader(statusCode int) {
	b.statusCode = statusCode
}

func azureFunctionsHttpAware(name string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := new(invokeRequest)
			if err := c.Bind(req); err != nil {
				return err
			}

			in := new(httpTriggerIn)
			data, exists := req.Data[name]
			if !exists {
				return echo.NewHTTPError(http.StatusBadRequest, "Http binding not found.")
			}
			if err := json.Unmarshal(data, in); err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Incorrect Http binding.").SetInternal(err)
			}

			innerReq, err := http.NewRequest(in.Method, in.Url, bytes.NewBufferString(in.Body))
			if err != nil {
				return echo.NewHTTPError(http.StatusBadRequest, "Incorrect Http binding.").SetInternal(err)
			}
			for key, val := range in.Headers {
				for _, v := range val {
					// change key case
					innerReq.Header.Set(key, v)
				}
			}

			innerRes := bufferResponseWriter{
				statusCode: http.StatusInternalServerError,
				header:     http.Header{},
				body:       bytes.NewBuffer([]byte{}),
			}

			innerCtx := c.Echo().NewContext(innerReq, &innerRes)

			ctx := delegateContext{
				Context: innerCtx,
				parent:  c,
			}

			if err = next(&ctx); err != nil {
				c.Echo().Logger.Error(err)
				if err, ok := err.(*echo.HTTPError); ok {
					response := invokeResponse{
						ReturnValue: &httpBindingOut{
							Status:  err.Code,
							Body:    err.Error(),
							Headers: make(map[string]string),
						},
						Outputs: make(map[string]interface{}),
					}
					return c.JSON(http.StatusOK, response)
				}

				response := invokeResponse{
					ReturnValue: &httpBindingOut{
						Status:  http.StatusInternalServerError,
						Body:    "Internal Server Error",
						Headers: make(map[string]string),
					},
					Outputs: make(map[string]interface{}),
				}
				return c.JSON(http.StatusOK, response)
			}

			outputs, ok := ctx.Get("Outputs").(map[string]interface{})
			if !ok {
				outputs = make(map[string]interface{})
			}
			headers := make(map[string]string)
			for key, val := range ctx.Response().Header() {
				for _, v := range val {
					headers[key] = v
				}
			}

			response := invokeResponse{
				ReturnValue: &httpBindingOut{
					Status:  innerRes.statusCode,
					Body:    innerRes.body.String(),
					Headers: headers,
				},
				Outputs: outputs,
			}
			return c.JSON(http.StatusOK, response)
		}
	}
}

const contextAttrOutputs = "Outputs"

func setOutput(c echo.Context, name string, val interface{}) {
	outputs, ok := c.Get(contextAttrOutputs).(map[string]interface{})
	if !ok {
		outputs = make(map[string]interface{})
		c.Set(contextAttrOutputs, outputs)
	}

	outputs[name] = val
}
