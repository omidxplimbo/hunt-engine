package validators

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func startVulnServer(t *testing.T, safe bool) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if safe {
			// textContent = encoded, NOT executable
			_, _ = w.Write([]byte(`<html><body><div id="out"></div><script>
				var p = new URLSearchParams(location.search).get('q');
				if (p) { document.getElementById('out').textContent = p; }
			</script></body></html>`))
		} else {
			// innerHTML = raw reflection, executable
			_, _ = w.Write([]byte(`<html><body><div id="out"></div><script>
				var p = new URLSearchParams(location.search).get('q');
				if (p) { document.getElementById('out').innerHTML = p; }
			</script></body></html>`))
		}
	})
	return httptest.NewServer(mux)
}

func TestValidateXSSRealExecution(t *testing.T) {
	srv := startVulnServer(t, false)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := ValidateXSSInBrowser(ctx, srv.URL+"/?q=x", "q", `<img src=x onerror=alert('HUNT_XSS')>`)
	if err != nil {
		t.Fatalf("validator error: %v", err)
	}
	t.Logf("result: executed=%v detail=%s", res.Executed, res.Detail)
	if !res.Executed {
		t.Fatalf("expected XSS to execute in real browser, got: %s", res.Detail)
	}
}

func TestValidateXSSSafePage(t *testing.T) {
	srv := startVulnServer(t, true)
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := ValidateXSSInBrowser(ctx, srv.URL+"/?q=x", "q", `<img src=x onerror=alert(1)>`)
	if err != nil {
		t.Fatalf("validator error: %v", err)
	}
	t.Logf("result: executed=%v detail=%s", res.Executed, res.Detail)
	if res.Executed {
		t.Fatalf("false positive on safe page")
	}
}
