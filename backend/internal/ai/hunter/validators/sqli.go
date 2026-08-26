package validators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// SQLiResult holds sqlmap validation output
type SQLiResult struct {
	Injectable   bool     `json:"injectable"`
	DBMS         string   `json:"dbms,omitempty"`
	Payloads     []string `json:"payloads,omitempty"`
	RawOutput    string   `json:"raw_output,omitempty"`
	Confidence   float64  `json:"confidence"`
	Detail       string   `json:"detail"`
}

// ValidateSQLiWithSQLMap runs sqlmap in batch mode against the URL.
// Requires sqlmap installed on the system. Returns structured findings.
func ValidateSQLiWithSQLMap(ctx context.Context, targetURL, param string) (*SQLiResult, error) {
	result := &SQLiResult{}

	if _, err := exec.LookPath("sqlmap"); err != nil {
		result.Detail = "sqlmap not installed — install with: pip3 install sqlmap"
		return result, nil
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()

	outDir, err := os.MkdirTemp("", "sqlmap_hunt_")
	if err != nil {
		return nil, fmt.Errorf("temp dir: %w", err)
	}
	defer os.RemoveAll(outDir)

	args := []string{
		"-u", targetURL,
		"--batch",
		"--output-dir", outDir,
		"--level", "2",
		"--risk", "2",
		"--threads", "3",
		"--flush-session",
	}
	if param != "" {
		args = append(args, "-p", param)
	}

	cmd := exec.CommandContext(ctx, "sqlmap", args...)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout

	err = cmd.Run()
	raw := stdout.String()
	result.RawOutput = truncate(raw, 8000)

	if ctx.Err() == context.DeadlineExceeded {
		result.Detail = "sqlmap timed out after 10 minutes"
		return result, nil
	}

	rawLower := strings.ToLower(raw)
	switch {
	case strings.Contains(rawLower, "is vulnerable") || strings.Contains(rawLower, "injectable"):
		result.Injectable = true
		result.Confidence = 0.95
		result.Detail = "sqlmap confirmed injection point"
		result.DBMS = extractDBMS(raw)
		result.Payloads = extractPayloads(raw)
	case strings.Contains(rawLower, "all tested parameters do not appear to be injectable"):
		result.Confidence = 0.9
		result.Detail = "sqlmap found no injectable parameters"
	default:
		result.Confidence = 0.2
		result.Detail = "sqlmap result inconclusive"
	}

	return result, nil
}

func extractDBMS(raw string) string {
	dbmsList := []string{"MySQL", "PostgreSQL", "Microsoft SQL Server", "Oracle", "SQLite"}
	for _, d := range dbmsList {
		if strings.Contains(strings.ToLower(raw), strings.ToLower("back-end DBMS: "+d)) ||
			strings.Contains(raw, d) {
			return d
		}
	}
	return ""
}

func extractPayloads(raw string) []string {
	var payloads []string
	for _, line := range strings.Split(raw, "\n") {
		if strings.Contains(line, "Payload:") {
			p := strings.TrimSpace(strings.SplitN(line, "Payload:", 2)[1])
			payloads = append(payloads, p)
			if len(payloads) >= 5 {
				break
			}
		}
	}
	return payloads
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "... [truncated]"
}

// jsonSafe ensures results marshal cleanly for API responses
var _ = json.Marshal // keep import if unused elsewhere
