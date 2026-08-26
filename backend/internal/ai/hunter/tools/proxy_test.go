package tools

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestProxyInterception(t *testing.T) {
	pt := NewProxyTool()
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Start proxy on a test port
	out, err := pt.Execute(ctx, map[string]any{
		"action": "start",
		"port":   18899,
	})
	if err != nil {
		t.Fatalf("proxy start failed: %v", err)
	}
	t.Log(out)

	// Make a request through the proxy
	proxyURL, _ := url.Parse("http://127.0.0.1:18899")
	client := &http.Client{
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
		Timeout:   30 * time.Second,
	}
	resp, err := client.Get("http://httpbin.org/get?via=proxy")
	if err != nil {
		t.Fatalf("request through proxy failed: %v", err)
	}
	resp.Body.Close()
	t.Logf("response status through proxy: %d", resp.StatusCode)

	// Check logs captured the request
	time.Sleep(500 * time.Millisecond)
	logs, err := pt.Execute(ctx, map[string]any{
		"action":      "get_logs",
		"filter_host": "httpbin",
	})
	if err != nil {
		t.Fatalf("get_logs failed: %v", err)
	}
	if !strings.Contains(logs, "httpbin.org") {
		t.Fatalf("expected httpbin in logs, got: %.300s", logs)
	}
	t.Logf("logs: %.300s", logs)

	// Stop proxy
	stopOut, _ := pt.Execute(ctx, map[string]any{"action": "stop"})
	t.Log(stopOut)
}
