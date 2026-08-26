package tools

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestShellTool(t *testing.T) {
	st := NewShellTool()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	out, err := st.Execute(ctx, map[string]any{
		"command":         "echo hello-hunt-engine",
		"timeout_seconds": 10,
	})
	if err != nil {
		t.Fatalf("shell tool failed: %v", err)
	}
	if !strings.Contains(out, "hello-hunt-engine") {
		t.Fatalf("unexpected output: %s", out)
	}
}

func TestHTTPTool(t *testing.T) {
	ht := NewHTTPTool()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := ht.Execute(ctx, map[string]any{
		"method": "GET",
		"url":    "https://httpbin.org/get?test=hunter",
	})
	if err != nil {
		t.Fatalf("http tool failed: %v", err)
	}
	if !strings.Contains(out, "[RESPONSE] 200") && !strings.Contains(out, "[ERROR]") {
		t.Fatalf("unexpected output: %s", out[:200])
	}
	t.Logf("output: %.200s", out)
}

func TestHTTPToolPayloadInjection(t *testing.T) {
	ht := NewHTTPTool()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	out, err := ht.Execute(ctx, map[string]any{
		"method":       "GET",
		"url":          "https://httpbin.org/get",
		"payload":      "<script>alert(1)</script>",
		"inject_point": "param:q",
	})
	if err != nil {
		t.Fatalf("http tool failed: %v", err)
	}
	if strings.Contains(out, "<script>alert(1)</script>") || strings.Contains(out, "&lt;script&gt;") {
		t.Log("payload reflected (encoded or raw)")
	}
	t.Logf("output: %.300s", out)
}
