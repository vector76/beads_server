package server

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Heading(t *testing.T) {
	out := string(renderMarkdown("# Hello"))
	if !strings.Contains(out, "<h1>") {
		t.Errorf("expected <h1> in output, got: %s", out)
	}
}

func TestRenderMarkdown_ListItem(t *testing.T) {
	out := string(renderMarkdown("- item"))
	if !strings.Contains(out, "<li>") {
		t.Errorf("expected <li> in output, got: %s", out)
	}
}

func TestRenderMarkdown_CodeSpan(t *testing.T) {
	out := string(renderMarkdown("`code`"))
	if !strings.Contains(out, "<code>") {
		t.Errorf("expected <code> in output, got: %s", out)
	}
}

func TestRenderMarkdown_RawHTMLEscaped(t *testing.T) {
	out := string(renderMarkdown("<script>alert(1)</script>"))
	// Goldmark's default mode suppresses raw HTML (replaces with <!-- raw HTML omitted -->),
	// so the literal <script> tag must not appear in the output.
	if strings.Contains(out, "<script>") {
		t.Errorf("expected <script> to be suppressed, got: %s", out)
	}
}

func TestRenderMarkdown_PlainText(t *testing.T) {
	out := string(renderMarkdown("hello world"))
	if !strings.Contains(out, "hello world") {
		t.Errorf("expected plain text in output, got: %s", out)
	}
}
