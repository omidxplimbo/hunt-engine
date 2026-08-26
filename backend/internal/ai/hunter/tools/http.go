package tools

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// HTTPTool makes HTTP requests with payload injection support
type HTTPTool struct {
	client *http.Client
	jar    *cookiejar.Jar
}

func NewHTTPTool() *HTTPTool {
	jar, _ := cookiejar.New(nil)
	return &HTTPTool{
		client: &http.Client{
			Jar: jar,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		jar: jar,
	}
}

func (h *HTTPTool) Name() string { return "http" }

func (h *HTTPTool) Description() string {
	return `Make HTTP requests to a target URL. Supports GET, POST, PUT, DELETE.
Can inject payloads into query parameters, headers, URL path, or request body.
Returns full request/response details including status, headers, body, and timing.
Use this for testing web applications for vulnerabilities.`
}

func (h *HTTPTool) Schema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"method": map[string]any{
				"type":        "string",
				"description": "HTTP method: GET, POST, PUT, DELETE, HEAD",
			},
			"url": map[string]any{
				"type":        "string",
				"description": "Full URL to request",
			},
			"payload": map[string]any{
				"type":        "string",
				"description": "Payload to inject (optional)",
			},
			"inject_point": map[string]any{
				"type":        "string",
				"description": "Where to inject payload: param:<name>, header:<name>, path, body:<name>",
			},
			"body": map[string]any{
				"type":        "string",
				"description": "Request body for POST/PUT",
			},
			"content_type": map[string]any{
				"type":        "string",
				"description": "Content-Type header for POST/PUT",
			},
			"headers": map[string]any{
				"type":        "object",
				"description": "Additional headers as key-value pairs",
			},
			"cookies": map[string]any{
				"type":        "string",
				"description": "Cookies as 'name=val; name2=val2'",
			},
		},
		"required": []string{"method", "url"},
	}
}

func (h *HTTPTool) Execute(ctx context.Context, params map[string]any) (string, error) {
	method, _ := params["method"].(string)
	targetURL, _ := params["url"].(string)
	if method == "" || targetURL == "" {
		return "", fmt.Errorf("method and url are required")
	}

	method = strings.ToUpper(method)

	// Set cookies if provided
	if cookies, ok := params["cookies"].(string); ok && cookies != "" {
		u, _ := url.Parse(targetURL)
		var cks []*http.Cookie
		for _, part := range strings.Split(cookies, ";") {
			kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
			if len(kv) == 2 {
				cks = append(cks, &http.Cookie{Name: strings.TrimSpace(kv[0]), Value: strings.TrimSpace(kv[1])})
			}
		}
		h.jar.SetCookies(u, cks)
	}

	// Handle payload injection
	payload, _ := params["payload"].(string)
	injectPoint, _ := params["inject_point"].(string)

	var body io.Reader
	if payload != "" && injectPoint != "" {
		switch {
		case strings.HasPrefix(injectPoint, "param:"):
			paramName := strings.TrimPrefix(injectPoint, "param:")
			u, _ := url.Parse(targetURL)
			q := u.Query()
			q.Set(paramName, payload)
			u.RawQuery = q.Encode()
			targetURL = u.String()

		case strings.HasPrefix(injectPoint, "header:"):
			headerName := strings.TrimPrefix(injectPoint, "header:")
			req, _ := http.NewRequestWithContext(ctx, method, targetURL, nil)
			req.Header.Set(headerName, payload)
			return h.doRequest(req, params)

		case injectPoint == "path":
			targetURL = strings.TrimSuffix(targetURL, "/") + "/" + payload

		case strings.HasPrefix(injectPoint, "body:"):
			paramName := strings.TrimPrefix(injectPoint, "body:")
			form := url.Values{}
			form.Set(paramName, payload)
			body = strings.NewReader(form.Encode())
		}
	} else if bodyStr, ok := params["body"].(string); ok && bodyStr != "" {
		body = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, targetURL, body)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	// Set content type
	if ct, ok := params["content_type"].(string); ok && ct != "" {
		req.Header.Set("Content-Type", ct)
	} else if body != nil && injectPoint != "" && strings.HasPrefix(injectPoint, "body:") {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}

	return h.doRequest(req, params)
}

func (h *HTTPTool) doRequest(req *http.Request, params map[string]any) (string, error) {
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36")

	// Set custom headers
	if headers, ok := params["headers"].(map[string]any); ok {
		for k, v := range headers {
			if s, ok := v.(string); ok {
				req.Header.Set(k, s)
			}
		}
	}

	start := time.Now()
	resp, err := h.client.Do(req)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Sprintf("[ERROR] %v (took %v)", err, elapsed), nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("[REQUEST] %s %s\n", req.Method, req.URL.String()))
	sb.WriteString(fmt.Sprintf("[RESPONSE] %d %s (took %v)\n", resp.StatusCode, resp.Status, elapsed))

	if resp.Header.Get("Content-Type") != "" {
		sb.WriteString(fmt.Sprintf("[CONTENT-TYPE] %s\n", resp.Header.Get("Content-Type")))
	}

	sb.WriteString(fmt.Sprintf("[BODY LENGTH] %d bytes\n", len(bodyBytes)))
	sb.WriteString("[BODY]\n")
	sb.WriteString(string(bodyBytes))

	return sb.String(), nil
}
