package hunter

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

// HTTPClient is the Hunter Agent's HTTP engine for making real requests with payloads
type HTTPClient struct {
	client    *http.Client
	jar       *cookiejar.Jar
	headers   map[string]string
	cookies   []*http.Cookie
	baseURL   string
	userAgent string
	timeout   time.Duration
}

// HTTPResult captures the full request/response for evidence
type HTTPResult struct {
	RequestURL     string            `json:"request_url"`
	RequestMethod  string            `json:"request_method"`
	RequestHeaders map[string]string `json:"request_headers"`
	RequestBody    string            `json:"request_body,omitempty"`
	ResponseStatus int               `json:"response_status"`
	ResponseHeaders map[string][]string `json:"response_headers"`
	ResponseBody   string            `json:"response_body"`
	ResponseLength int               `json:"response_length"`
	ResponseTime   time.Duration     `json:"response_time"`
	FinalURL       string            `json:"final_url"`
	Redirected     bool              `json:"redirected"`
	Error          string            `json:"error,omitempty"`
}

// NewHTTPClient creates a new HTTP client for the Hunter Agent
func NewHTTPClient(baseURL string, timeout time.Duration) *HTTPClient {
	jar, _ := cookiejar.New(nil)
	
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Pentesting - we need to test even with invalid certs
		},
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
	}

	return &HTTPClient{
		client: &http.Client{
			Jar:       jar,
			Transport: transport,
			Timeout:   timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		jar:       jar,
		headers:   make(map[string]string),
		baseURL:   baseURL,
		userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		timeout:   timeout,
	}
}

// SetAuth sets authentication headers (Bearer token, Basic auth, etc.)
func (h *HTTPClient) SetAuth(authType, value string) {
	switch strings.ToLower(authType) {
	case "bearer":
		h.headers["Authorization"] = "Bearer " + value
	case "basic":
		h.headers["Authorization"] = "Basic " + value
	case "cookie":
		// Parse cookie string "name=value"
		parts := strings.SplitN(value, "=", 2)
		if len(parts) == 2 {
			u, _ := url.Parse(h.baseURL)
			h.jar.SetCookies(u, []*http.Cookie{
				{Name: strings.TrimSpace(parts[0]), Value: strings.TrimSpace(parts[1])},
			})
		}
	}
}

// SetHeader sets a custom header
func (h *HTTPClient) SetHeader(key, value string) {
	h.headers[key] = value
}

// SetCookies sets cookies from a string like "name1=val1; name2=val2"
func (h *HTTPClient) SetCookies(cookieStr string) {
	u, _ := url.Parse(h.baseURL)
	for _, part := range strings.Split(cookieStr, ";") {
		kv := strings.SplitN(strings.TrimSpace(part), "=", 2)
		if len(kv) == 2 {
			h.jar.SetCookies(u, []*http.Cookie{
				{Name: strings.TrimSpace(kv[0]), Value: strings.TrimSpace(kv[1])},
			})
		}
	}
}

// Get makes a GET request and captures full evidence
func (h *HTTPClient) Get(path string, params map[string]string) *HTTPResult {
	fullURL := h.buildURL(path, params)
	req, err := http.NewRequest("GET", fullURL, nil)
	if err != nil {
		return &HTTPResult{Error: err.Error()}
	}
	return h.do(req)
}

// Post makes a POST request with a body
func (h *HTTPClient) Post(path string, body string, contentType string) *HTTPResult {
	fullURL := h.buildURL(path, nil)
	req, err := http.NewRequest("POST", fullURL, strings.NewReader(body))
	if err != nil {
		return &HTTPResult{Error: err.Error()}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return h.do(req)
}

// PostForm makes a POST request with form data
func (h *HTTPClient) PostForm(path string, data map[string]string) *HTTPResult {
	form := url.Values{}
	for k, v := range data {
		form.Set(k, v)
	}
	fullURL := h.buildURL(path, nil)
	req, err := http.NewRequest("POST", fullURL, strings.NewReader(form.Encode()))
	if err != nil {
		return &HTTPResult{Error: err.Error()}
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return h.do(req)
}

// SendPayload sends a request with a payload injected into a specific position
// injectionPoint can be: "param:name", "header:name", "path", "body:name"
func (h *HTTPClient) SendPayload(method, path, payload, injectionPoint string) *HTTPResult {
	fullURL := h.buildURL(path, nil)
	
	var body io.Reader
	switch {
	case strings.HasPrefix(injectionPoint, "param:"):
		paramName := strings.TrimPrefix(injectionPoint, "param:")
		u, _ := url.Parse(fullURL)
		q := u.Query()
		q.Set(paramName, payload)
		u.RawQuery = q.Encode()
		fullURL = u.String()
		
	case strings.HasPrefix(injectionPoint, "header:"):
		headerName := strings.TrimPrefix(injectionPoint, "header:")
		req, err := http.NewRequest(method, fullURL, nil)
		if err != nil {
			return &HTTPResult{Error: err.Error()}
		}
		req.Header.Set(headerName, payload)
		return h.do(req)
		
	case injectionPoint == "path":
		fullURL = strings.TrimSuffix(fullURL, "/") + "/" + payload
		
	case strings.HasPrefix(injectionPoint, "body:"):
		paramName := strings.TrimPrefix(injectionPoint, "body:")
		form := url.Values{}
		form.Set(paramName, payload)
		body = strings.NewReader(form.Encode())
	}
	
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return &HTTPResult{Error: err.Error()}
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	return h.do(req)
}

// do executes the request and captures full evidence
func (h *HTTPClient) do(req *http.Request) *HTTPResult {
	// Set default headers
	req.Header.Set("User-Agent", h.userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	for k, v := range h.headers {
		req.Header.Set(k, v)
	}
	
	// Capture request headers
	reqHeaders := make(map[string]string)
	for k := range req.Header {
		reqHeaders[k] = req.Header.Get(k)
	}
	
	// Capture request body
	var reqBody string
	if req.Body != nil {
		bodyBytes, _ := io.ReadAll(req.Body)
		reqBody = string(bodyBytes)
		req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
	}
	
	start := time.Now()
	resp, err := h.client.Do(req)
	elapsed := time.Since(start)
	
	if err != nil {
		return &HTTPResult{
			RequestURL:     req.URL.String(),
			RequestMethod:  req.Method,
			RequestHeaders: reqHeaders,
			RequestBody:    reqBody,
			Error:          err.Error(),
			ResponseTime:   elapsed,
		}
	}
	defer resp.Body.Close()
	
	// Read response body (limit to 1MB for memory safety)
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	
	return &HTTPResult{
		RequestURL:      req.URL.String(),
		RequestMethod:   req.Method,
		RequestHeaders:  reqHeaders,
		RequestBody:     reqBody,
		ResponseStatus:  resp.StatusCode,
		ResponseHeaders: resp.Header,
		ResponseBody:    string(bodyBytes),
		ResponseLength:  len(bodyBytes),
		ResponseTime:    elapsed,
		FinalURL:        resp.Request.URL.String(),
		Redirected:      resp.Request.URL.String() != req.URL.String(),
	}
}

// buildURL constructs the full URL from base + path + params
func (h *HTTPClient) buildURL(path string, params map[string]string) string {
	base := strings.TrimRight(h.baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	fullURL := base + path
	
	if len(params) > 0 {
		u, _ := url.Parse(fullURL)
		q := u.Query()
		for k, v := range params {
			q.Set(k, v)
		}
		u.RawQuery = q.Encode()
		fullURL = u.String()
	}
	
	return fullURL
}

// GetCookies returns all cookies for the base URL
func (h *HTTPClient) GetCookies() []*http.Cookie {
	u, _ := url.Parse(h.baseURL)
	return h.jar.Cookies(u)
}
