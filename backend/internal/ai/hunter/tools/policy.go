package tools

import (
	"regexp"
	"strings"
)

// RiskLevel classifies how dangerous a tool is. The AgentLoop consults
// the policy before invoking any tool that is risk:high or risk:medium
// and pauses for human approval.
type RiskLevel string

const (
	RiskLow    RiskLevel = "low"    // passive probe; auto-approve
	RiskMedium RiskLevel = "medium" // side effects on browser/auth state; first call per session needs approval
	RiskHigh   RiskLevel = "high"   // arbitrary subprocess execution; every call needs approval
)

// DefaultPolicy maps the four built-in hunter tools to their risk level
// and what the AgentLoop should do. Multi-agent mode reuses this — each
// worker has its own approval counter.
var DefaultPolicy = Policy{
	Levels: map[string]RiskLevel{
		"http":    RiskLow,
		"browser": RiskMedium,
		"proxy":   RiskMedium,
		"shell":   RiskHigh,
	},
}

// Policy is the read-only risk table the AgentLoop consults.
type Policy struct {
	Levels map[string]RiskLevel
}

// RiskFor returns the risk level for a tool, defaulting to High (safest
// fallback: if we don't know what a tool does, we ask the operator).
func (p Policy) RiskFor(toolName string) RiskLevel {
	if r, ok := p.Levels[toolName]; ok {
		return r
	}
	return RiskHigh
}

// RequiresApproval reports whether the policy says the agent should
// block on human approval before invoking the given tool. The
// "first call per session" rule is handled by the AgentLoop, not here.
func (p Policy) RequiresApproval(toolName string) bool {
	return p.RiskFor(toolName) == RiskHigh
}

// sensitiveParamPatterns are substrings (case-insensitive) in a key that
// indicate the value should be masked before being broadcast over the
// WebSocket. The masker replaces the value with "***".
var sensitiveParamPatterns = []string{
	"authorization",
	"cookie",
	"x-api-key",
	"x-auth",
	"apikey",
	"api_key",
	"token",
	"password",
	"secret",
}

// MaskSensitiveParams returns a shallow copy of params where values for
// keys that look like credentials are replaced with "***". The original
// map is untouched; this is for WS broadcast only.
func MaskSensitiveParams(params map[string]any) map[string]any {
	if params == nil {
		return nil
	}
	out := make(map[string]any, len(params))
	for k, v := range params {
		if isSensitiveKey(k) {
			out[k] = "***"
			continue
		}
		// Recurse one level into nested maps (common for HTTP headers).
		if nested, ok := v.(map[string]any); ok {
			out[k] = MaskSensitiveParams(nested)
			continue
		}
		// Also mask string values that LOOK like "Authorization: Bearer ..."
		if s, ok := v.(string); ok && looksLikeHeader(s) {
			out[k] = maskHeaderValue(s)
			continue
		}
		out[k] = v
	}
	return out
}

func isSensitiveKey(k string) bool {
	low := strings.ToLower(k)
	for _, p := range sensitiveParamPatterns {
		if strings.Contains(low, p) {
			return true
		}
	}
	return false
}

var headerPattern = regexp.MustCompile(`(?i)^[A-Za-z0-9-]+:\s+(.+)$`)

func looksLikeHeader(s string) bool {
	return headerPattern.MatchString(strings.TrimSpace(s))
}

func maskHeaderValue(s string) string {
	idx := strings.Index(s, ":")
	if idx < 0 {
		return "***"
	}
	return s[:idx+1] + " ***"
}
