package main

import (
	"embed"
	htmlTemplate "html/template"
	"io"
	"strings"
	"text/template"

	"github.com/labstack/echo/v4"
)

//go:embed templates
var templatesFs embed.FS

type templateRenderer struct {
	htmlTemplates *htmlTemplate.Template
	templates     *template.Template
}

func newTemplateRenderer() *templateRenderer {
	return &templateRenderer{
		htmlTemplates: htmlTemplate.Must(htmlTemplate.ParseFS(templatesFs, "templates/*.html")),
		templates:     template.Must(template.ParseFS(templatesFs, "templates/*")),
	}
}

func (t *templateRenderer) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	if strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".xml") {
		return t.htmlTemplates.ExecuteTemplate(w, name, data)
	}
	return t.templates.ExecuteTemplate(w, name, data)
}
