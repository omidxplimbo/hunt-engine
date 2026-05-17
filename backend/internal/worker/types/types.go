package types

// HTTPXResult is the parsed JSONL output shape emitted by httpx.
// It is kept in a shared worker package so phase packages can depend on
// a stable type instead of importing runner.go internals.
type HTTPXResult struct {
	Input         string   `json:"input"`
	URL           string   `json:"url"`
	StatusCode    int      `json:"status_code"`
	Title         string   `json:"title"`
	ContentLength int64    `json:"content_length"`
	A             []string `json:"a"`
	WebServer     string   `json:"webserver"`
	CDNName       string   `json:"cdn_name"`
	Technologies  []string `json:"tech"`
	BodyHash      string   `json:"body_hash"`
	HeaderHash    string   `json:"header_hash"`
	ResponseTime  string   `json:"time"`
	RawJSON       string   `json:"-"`
}

// DNSXResult is one JSON result row emitted by dnsx.
type DNSXResult struct {
	Host string   `json:"host"`
	A    []string `json:"a"`
}

// CDNCheckResult is one JSON result row emitted by cdncheck.
type CDNCheckResult struct {
	IP        string `json:"ip"`
	IsCDN     bool   `json:"cdn"`
	CDNName   string `json:"cdn_name"`
	IsWAF     bool   `json:"waf"`
	WAFName   string `json:"waf_name"`
	IsCloud   bool   `json:"cloud"`
	CloudName string `json:"cloud_name"`
}
