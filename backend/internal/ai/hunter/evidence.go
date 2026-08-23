package hunter

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Evidence represents a captured piece of evidence from a test
type Evidence struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	TestType    string                 `json:"test_type"`    // xss, sqli, idor, ssrf, etc.
	Target      string                 `json:"target"`       // URL or endpoint tested
	Parameter   string                 `json:"parameter"`    // Parameter tested
	Payload     string                 `json:"payload"`      // Payload used
	Result      *HTTPResult            `json:"result"`       // Full HTTP result
	Analysis    *AnalysisResult        `json:"analysis"`     // Analysis of the result
	Confidence  float64                `json:"confidence"`   // 0.0 - 1.0
	Severity    string                 `json:"severity"`     // info, low, medium, high, critical
	Status      string                 `json:"status"`       // candidate, testing, confirmed, false_positive
	PoC         string                 `json:"poc,omitempty"` // Proof of Concept
	Notes       string                 `json:"notes,omitempty"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// AnalysisResult represents the analysis of an HTTP response
type AnalysisResult struct {
	IsVulnerable    bool     `json:"is_vulnerable"`
	VulnType        string   `json:"vuln_type"`
	Confidence      float64  `json:"confidence"`
	Severity        string   `json:"severity"`
	Indicators      []string `json:"indicators"`      // What indicates a vulnerability
	FalsePositives  []string `json:"false_positives"` // What might be a false positive
	NextSteps       []string `json:"next_steps"`      // Recommended next steps
	Description     string   `json:"description"`
}

// EvidenceStore manages captured evidence
type EvidenceStore struct {
	evidences []Evidence
}

// NewEvidenceStore creates a new evidence store
func NewEvidenceStore() *EvidenceStore {
	return &EvidenceStore{
		evidences: make([]Evidence, 0),
	}
}

// Add adds a new evidence entry
func (s *EvidenceStore) Add(e Evidence) {
	if e.ID == "" {
		e.ID = fmt.Sprintf("ev_%d_%s", time.Now().UnixNano(), e.TestType)
	}
	if e.Timestamp.IsZero() {
		e.Timestamp = time.Now().UTC()
	}
	if e.Status == "" {
		e.Status = "candidate"
	}
	s.evidences = append(s.evidences, e)
}

// GetByType returns all evidence of a specific type
func (s *EvidenceStore) GetByType(testType string) []Evidence {
	var result []Evidence
	for _, e := range s.evidences {
		if e.TestType == testType {
			result = append(result, e)
		}
	}
	return result
}

// GetConfirmed returns all confirmed vulnerabilities
func (s *EvidenceStore) GetConfirmed() []Evidence {
	var result []Evidence
	for _, e := range s.evidences {
		if e.Status == "confirmed" {
			result = append(result, e)
		}
	}
	return result
}

// GetAll returns all evidence
func (s *EvidenceStore) GetAll() []Evidence {
	return s.evidences
}

// Count returns the total number of evidence entries
func (s *EvidenceStore) Count() int {
	return len(s.evidences)
}

// AnalyzeXSSResponse analyzes an HTTP response for XSS indicators
func AnalyzeXSSResponse(result *HTTPResult, payload string) *AnalysisResult {
	analysis := &AnalysisResult{
		VulnType: "xss",
	}
	
	if result.Error != "" {
		analysis.Description = "Request failed: " + result.Error
		return analysis
	}
	
	// Check if payload is reflected in response
	if strings.Contains(result.ResponseBody, payload) {
		analysis.IsVulnerable = true
		analysis.Confidence = 0.7
		analysis.Indicators = append(analysis.Indicators, "Payload reflected in response")
		
		// Check if it's in an executable context
		body := result.ResponseBody
		idx := strings.Index(body, payload)
		if idx > 0 {
			before := body[max(0, idx-100):idx]
			if strings.Contains(before, "<script") {
				analysis.Confidence = 0.95
				analysis.Indicators = append(analysis.Indicators, "Payload in script context")
				analysis.Severity = "high"
			} else if strings.Contains(before, "on") && strings.Contains(before, "=") {
				analysis.Confidence = 0.85
				analysis.Indicators = append(analysis.Indicators, "Payload in event handler context")
				analysis.Severity = "medium"
			} else if strings.Contains(before, "<") {
				analysis.Confidence = 0.6
				analysis.Indicators = append(analysis.Indicators, "Payload in HTML context")
				analysis.Severity = "medium"
			}
		}
		
		// Check for encoding
		if strings.Contains(result.ResponseBody, "&lt;") || strings.Contains(result.ResponseBody, "&#60;") {
			analysis.Confidence *= 0.3
			analysis.FalsePositives = append(analysis.FalsePositives, "Payload appears to be HTML encoded")
		}
	} else {
		analysis.IsVulnerable = false
		analysis.Confidence = 0.1
		analysis.Description = "Payload not reflected in response"
	}
	
	return analysis
}

// AnalyzeSQLiResponse analyzes an HTTP response for SQL injection indicators
func AnalyzeSQLiResponse(result *HTTPResult, payload string) *AnalysisResult {
	analysis := &AnalysisResult{
		VulnType: "sqli",
	}
	
	if result.Error != "" {
		analysis.Description = "Request failed: " + result.Error
		return analysis
	}
	
	body := strings.ToLower(result.ResponseBody)
	
	// SQL error patterns
	sqlErrors := []string{
		"sql syntax", "mysql_fetch", "sqlite3", "postgresql",
		"ora-", "microsoft sql", "jdbc", "odbc", "unclosed quotation",
		"syntax error", "unterminated quoted string",
	}
	
	for _, errPattern := range sqlErrors {
		if strings.Contains(body, errPattern) {
			analysis.IsVulnerable = true
			analysis.Confidence = 0.8
			analysis.Indicators = append(analysis.Indicators, "SQL error detected: "+errPattern)
			analysis.Severity = "high"
			break
		}
	}
	
	// Time-based detection
	if result.ResponseTime > 5*time.Second {
		analysis.Confidence += 0.2
		analysis.Indicators = append(analysis.Indicators, "Delayed response (possible time-based SQLi)")
	}
	
	// Check for different response than baseline (would need baseline comparison)
	if result.ResponseStatus == 500 {
		analysis.Confidence += 0.1
		analysis.Indicators = append(analysis.Indicators, "Server error (500)")
	}
	
	if !analysis.IsVulnerable {
		analysis.Description = "No SQL injection indicators detected"
	}
	
	return analysis
}

// AnalyzeIDORResponse analyzes responses for IDOR indicators
func AnalyzeIDORResponse(resultA, resultB *HTTPResult) *AnalysisResult {
	analysis := &AnalysisResult{
		VulnType: "idor",
	}
	
	if resultA.Error != "" || resultB.Error != "" {
		analysis.Description = "One or both requests failed"
		return analysis
	}
	
	// Status code mismatch
	if resultA.ResponseStatus != resultB.ResponseStatus {
		analysis.IsVulnerable = true
		analysis.Confidence = 0.6
		analysis.Indicators = append(analysis.Indicators, 
			fmt.Sprintf("Status code mismatch: %d vs %d", resultA.ResponseStatus, resultB.ResponseStatus))
	}
	
	// Content length difference
	lenDiff := abs(resultA.ResponseLength - resultB.ResponseLength)
	if lenDiff > 100 {
		analysis.Confidence += 0.2
		analysis.Indicators = append(analysis.Indicators, 
			fmt.Sprintf("Significant content length difference: %d bytes", lenDiff))
	}
	
	// Both returned 200 with different content
	if resultA.ResponseStatus == 200 && resultB.ResponseStatus == 200 {
		if resultA.ResponseBody != resultB.ResponseBody {
			analysis.IsVulnerable = true
			analysis.Confidence = 0.8
			analysis.Indicators = append(analysis.Indicators, "Different content returned for different users")
			analysis.Severity = "high"
		}
	}
	
	if !analysis.IsVulnerable {
		analysis.Description = "No IDOR indicators detected"
	}
	
	return analysis
}

// ToJSON converts evidence to JSON
func (e *Evidence) ToJSON() (string, error) {
	b, err := json.MarshalIndent(e, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
