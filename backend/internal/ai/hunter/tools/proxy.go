package tools

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/elazarl/goproxy"
)

// ProxyTool provides MITM HTTP proxy capabilities
type ProxyTool struct {
	proxy    *goproxy.ProxyHttpServer
	server   *http.Server
	mu       sync.Mutex
	running  bool
	port     int
	logs     []ProxyLog
	logMu    sync.Mutex
	maxLogs  int
}

// ProxyLog represents a captured request/response pair
type ProxyLog struct {
	Timestamp      time.Time         `json:"timestamp"`
	RequestURL     string            `json:"request_url"`
	RequestMethod  string            `json:"request_method"`
	RequestHeaders map[string][]string `json:"request_headers"`
	RequestBody    string            `json:"request_body,omitempty"`
	ResponseStatus int               `json:"response_status"`
	ResponseHeaders map[string][]string `json:"response_headers"`
	ResponseBody   string            `json:"response_body,omitempty"`
}

func NewProxyTool() *ProxyTool {
	return &ProxyTool{
		port:    8899,
		maxLogs: 1000,
	}
}

func (p *ProxyTool) Name() string { return "proxy" }

func (p *ProxyTool) Description() string {
	return `MITM (Man-In-The-Middle) HTTP proxy for intercepting and analyzing traffic.
Use this to:
- Start a proxy to capture all HTTP/HTTPS traffic
- View captured requests and responses
- Analyze traffic for vulnerabilities (tokens, cookies, sensitive data)
- Replay captured requests with modifications
- Filter traffic by host or path
The proxy runs on the server and logs all traffic for analysis.`
}

func (p *ProxyTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"action": map[string]any{
				"type":        "string",
				"description": "Action: start, stop, get_logs, clear_logs, get_log_count",
			},
			"port": map[string]any{
				"type":        "integer",
				"description": "Port to run proxy on (default: 8899)",
			},
			"filter_host": map[string]any{
				"type":        "string",
				"description": "Filter logs by host (for get_logs)",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Max number of logs to return (default: 50)",
			},
		},
		"required": []string{"action"},
	}
}

func (p *ProxyTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	action, _ := params["action"].(string)
	if action == "" {
		return "", fmt.Errorf("action is required")
	}

	switch action {
	case "start":
		return p.start(params)
	case "stop":
		return p.stop()
	case "get_logs":
		return p.getLogs(params)
	case "clear_logs":
		return p.clearLogs()
	case "get_log_count":
		return fmt.Sprintf("[PROXY] %d logs captured", len(p.logs)), nil
	default:
		return "", fmt.Errorf("unknown action: %s", action)
	}
}

func (p *ProxyTool) start(params map[string]any) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Sprintf("[PROXY] Already running on port %d", p.port), nil
	}

	switch v := params["port"].(type) {
	case float64:
		if v > 0 {
			p.port = int(v)
		}
	case int:
		if v > 0 {
			p.port = v
		}
	}

	p.proxy = goproxy.NewProxyHttpServer()
	p.proxy.Verbose = false

	// Enable MITM for HTTPS
	p.proxy.OnRequest().HandleConnect(goproxy.AlwaysMitm)

	// Log all requests
	p.proxy.OnRequest().DoFunc(func(req *http.Request, ctx *goproxy.ProxyCtx) (*http.Request, *http.Response) {
		// Read request body
		var reqBody string
		if req.Body != nil {
			bodyBytes, _ := io.ReadAll(io.LimitReader(req.Body, 64*1024))
			reqBody = string(bodyBytes)
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		// Store in context for response handler
		ctx.UserData = &ProxyLog{
			Timestamp:      time.Now(),
			RequestURL:     req.URL.String(),
			RequestMethod:  req.Method,
			RequestHeaders: req.Header,
			RequestBody:    reqBody,
		}

		return req, nil
	})

	// Log all responses
	p.proxy.OnResponse().DoFunc(func(resp *http.Response, ctx *goproxy.ProxyCtx) *http.Response {
		if logEntry, ok := ctx.UserData.(*ProxyLog); ok {
			if resp != nil {
				logEntry.ResponseStatus = resp.StatusCode
				logEntry.ResponseHeaders = resp.Header

				// Read response body
				if resp.Body != nil {
					bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
					logEntry.ResponseBody = string(bodyBytes)
					resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				}
			}

			p.logMu.Lock()
			p.logs = append(p.logs, *logEntry)
			if len(p.logs) > p.maxLogs {
				p.logs = p.logs[len(p.logs)-p.maxLogs:]
			}
			p.logMu.Unlock()
		}
		return resp
	})

	p.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", p.port),
		Handler: p.proxy,
	}

	ln, err := net.Listen("tcp", p.server.Addr)
	if err != nil {
		return "", fmt.Errorf("proxy failed to bind port %d: %w", p.port, err)
	}

	go func() {
		if err := p.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[PROXY] Error: %v\n", err)
		}
	}()

	p.running = true
	return fmt.Sprintf("[PROXY] Started on port %d\nUse: curl -x http://localhost:%d http://target.com", p.port, p.port), nil
}

func (p *ProxyTool) stop() (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.running {
		return "[PROXY] Not running", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := p.server.Shutdown(ctx); err != nil {
		return fmt.Sprintf("[PROXY] Error stopping: %v", err), nil
	}

	p.running = false
	logCount := len(p.logs)
	return fmt.Sprintf("[PROXY] Stopped. Captured %d logs.", logCount), nil
}

func (p *ProxyTool) getLogs(params map[string]any) (string, error) {
	p.logMu.Lock()
	defer p.logMu.Unlock()

	filterHost, _ := params["filter_host"].(string)
	limit := 50
	if l, ok := params["limit"].(float64); ok && l > 0 {
		limit = int(l)
	}

	var filtered []ProxyLog
	for i := len(p.logs) - 1; i >= 0 && len(filtered) < limit; i-- {
		log := p.logs[i]
		if filterHost != "" && !strings.Contains(log.RequestURL, filterHost) {
			continue
		}
		filtered = append(filtered, log)
	}

	if len(filtered) == 0 {
		return "[PROXY] No logs found", nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[PROXY LOGS] Showing %d entries\n\n", len(filtered)))
	for i, log := range filtered {
		sb.WriteString(fmt.Sprintf("--- Log %d ---\n", i+1))
		sb.WriteString(fmt.Sprintf("Time: %s\n", log.Timestamp.Format(time.RFC3339)))
		sb.WriteString(fmt.Sprintf("Request: %s %s\n", log.RequestMethod, log.RequestURL))
		sb.WriteString(fmt.Sprintf("Response: %d\n", log.ResponseStatus))
		if log.RequestBody != "" {
			sb.WriteString(fmt.Sprintf("Request Body: %s\n", truncate(log.RequestBody, 500)))
		}
		if log.ResponseBody != "" {
			sb.WriteString(fmt.Sprintf("Response Body: %s\n", truncate(log.ResponseBody, 500)))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

func (p *ProxyTool) clearLogs() (string, error) {
	p.logMu.Lock()
	defer p.logMu.Unlock()
	p.logs = nil
	return "[PROXY] Logs cleared", nil
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "... [truncated]"
}
