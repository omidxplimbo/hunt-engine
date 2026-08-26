package validators

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// OOBServer is a minimal out-of-band callback catcher.
// The agent embeds a unique token into SSRF payloads pointing at this server;
// any inbound HTTP request carrying the token proves server-side fetch.
type OOBServer struct {
	server   *http.Server
	port     int
	mu       sync.Mutex
	callbacks map[string]OOBCallback // token -> callback info
}

// OOBCallback records an inbound hit
type OOBCallback struct {
	Token     string    `json:"token"`
	SourceIP  string    `json:"source_ip"`
	Path      string    `json:"path"`
	Headers   http.Header `json:"headers"`
	Timestamp time.Time `json:"timestamp"`
}

// NewOOBServer creates but does not start the OOB listener
func NewOOBServer(port int) *OOBServer {
	o := &OOBServer{port: port}
	mux := http.NewServeMux()
	mux.HandleFunc("/", o.handle)
	o.server = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: mux,
	}
	return o
}

func (o *OOBServer) handle(w http.ResponseWriter, r *http.Request) {
	// Token is expected as first path segment: /<token>/...
	path := strings.TrimPrefix(r.URL.Path, "/")
	token := path
	if idx := strings.Index(path, "/"); idx > 0 {
		token = path[:idx]
	}

	cb := OOBCallback{
		Token:     token,
		SourceIP:  r.RemoteAddr,
		Path:      r.URL.Path,
		Headers:   r.Header,
		Timestamp: time.Now(),
	}
	o.mu.Lock()
	o.callbacks[token] = cb
	o.mu.Unlock()
	RegisterGlobalCallback(cb)

	w.WriteHeader(200)
	fmt.Fprintln(w, "ok")
}

// Start begins listening synchronously binding then serving in background
func (o *OOBServer) Start() error {
	go func() {
		if err := o.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("[OOB] error: %v\n", err)
		}
	}()
	return nil
}

// Stop shuts down the listener
func (o *OOBServer) Stop(ctx context.Context) error {
	return o.server.Shutdown(ctx)
}

// URL returns the base URL agents should point SSRF payloads at
func (o *OOBServer) URL(host string) string {
	return fmt.Sprintf("http://%s:%d", host, o.port)
}

// CheckToken returns true if the token received a callback
func (o *OOBServer) CheckToken(token string) (OOBCallback, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	cb, ok := o.callbacks[token]
	return cb, ok
}

// GenerateToken produces a unique per-test token
func GenerateToken(testID string) string {
	return fmt.Sprintf("hunt%d%d", time.Now().UnixNano(), len(testID))
}

// ValidateSSRF tests whether injecting an OOB URL into the given parameter
// triggers a server-side request that reaches our OOB listener.
func ValidateSSRF(ctx context.Context, client *http.Client, targetURL, paramName, oobBaseURL string) (bool, string) {
	token := GenerateToken(targetURL)
	oobURL := fmt.Sprintf("%s/%s", oobBaseURL, token)

	// Try several payload shapes referencing the OOB URL
	payloads := []string{
		oobURL,
		fmt.Sprintf("http://%%31%02x@%s/%s", 27, stripScheme(oobURL), token), // partial @-bypass shape
	}

	for _, p := range payloads {
		req, err := http.NewRequestWithContext(ctx, "GET", injectParam(targetURL, paramName, p), nil)
		if err != nil {
			continue
		}
		resp, err := client.Do(req)
		if err == nil {
			resp.Body.Close()
		}
	}

	// Give the target time to make the outbound call
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if cb, ok := NewOOBCheck(oobBaseURL, token); ok {
			return true, fmt.Sprintf("OOB callback from %s at %s", cb.SourceIP, cb.Timestamp.Format(time.RFC3339))
		}
		time.Sleep(1 * time.Second)
	}
	return false, "no OOB callback within 20s"
}

func stripScheme(u string) string {
	u = strings.TrimPrefix(u, "http://")
	u = strings.TrimPrefix(u, "https://")
	if idx := strings.Index(u, "/"); idx > 0 {
		u = u[:idx]
	}
	return u
}

// NewOOBCheck is a placeholder hook — real deployment wires CheckToken via
// the running OOBServer instance; kept simple for single-process use.
func NewOOBCheck(baseURL, token string) (OOBCallback, bool) {
	globalMu.Lock()
	defer globalMu.Unlock()
	cb, ok := globalCallbacks[token]
	return cb, ok
}

var (
	globalMu        sync.Mutex
	globalCallbacks = map[string]OOBCallback{}
)

// RegisterGlobalCallback lets the OOBServer publish hits to the package-level store
func RegisterGlobalCallback(cb OOBCallback) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalCallbacks[cb.Token] = cb
}
