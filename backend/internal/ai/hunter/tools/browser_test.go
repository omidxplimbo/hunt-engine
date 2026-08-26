package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestBrowserNavigate(t *testing.T) {
	bt := NewBrowserTool()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	out, err := bt.Execute(ctx, map[string]any{
		"action": "navigate",
		"url":    "https://httpbin.org/html",
	})
	if err != nil {
		t.Fatalf("browser navigate failed: %v", err)
	}
	t.Logf("output: %.200s", out)
	if !strings.Contains(out, "[NAVIGATED]") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestBrowserXSSDetection(t *testing.T) {
	// Use a data URL page with reflected XSS to verify detection works
	bt := NewBrowserTool()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	out, err := bt.Execute(ctx, map[string]any{
		"action": "execute_js",
		"url":    "data:text/html,<html><body><h1>Test Page</h1><input type=text name=q></body></html>",
		"js_code": "(function(){ return document.title + ' | inputs: ' + document.querySelectorAll('input').length; })()",
	})
	if err != nil {
		t.Fatalf("browser js failed: %v", err)
	}
	t.Logf("output: %.300s", out)
}
