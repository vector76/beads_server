package server

import (
	"bytes"
	"html/template"

	"github.com/yuin/goldmark"
)

func renderMarkdown(s string) template.HTML {
	var buf bytes.Buffer
	if err := goldmark.Convert([]byte(s), &buf); err != nil {
		return template.HTML(template.HTMLEscapeString(s))
	}
	return template.HTML(buf.String())
}
