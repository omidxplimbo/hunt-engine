package service

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"gorm.io/gorm"
)

// Action types for two-account smart testing
const (
	ActionSwitchContext   = "SWITCH_CONTEXT"
	ActionCompareResponse = "COMPARE_RESPONSE"
	ActionFuzzParameter   = "FUZZ_PARAMETER"
	ActionExtractData     = "EXTRACT_DATA"
)

type TwoAccountService struct {
	db *gorm.DB
}

func NewTwoAccountService(db *gorm.DB) *TwoAccountService {
	return &TwoAccountService{db: db}
}

// CreateSession creates a new two-account test session
func (s *TwoAccountService) CreateSession(ctx context.Context, targetID, userID, contextA_ID, contextB_ID uint, targetURL string) (*models.TwoAccountSession, error) {
	var ctxA, ctxB models.AuthContext
	if err := s.db.First(&ctxA, contextA_ID).Error; err != nil || !ctxA.IsActive {
		return nil, fmt.Errorf("invalid or inactive context A")
	}
	if err := s.db.First(&ctxB, contextB_ID).Error; err != nil || !ctxB.IsActive {
		return nil, fmt.Errorf("invalid or inactive context B")
	}

	session := &models.TwoAccountSession{
		TargetID:        targetID,
		OwnerID:         userID,
		AuthContextID_A: contextA_ID,
		AuthContextID_B: contextB_ID,
		ContextA_Name:   ctxA.Name,
		ContextB_Name:   ctxB.Name,
		Status:          models.StatusTwoAccountInitializing,
		CurrentStrategy: "adaptive_fuzzing",
	}

	if err := s.db.Create(session).Error; err != nil {
		return nil, err
	}
	return session, nil
}

// GetActiveSession returns the active session for a target
func (s *TwoAccountService) GetActiveSession(ctx context.Context, targetID uint) (*models.TwoAccountSession, error) {
	var session models.TwoAccountSession
	err := s.db.Where("target_id = ? AND status NOT IN (?, ?, ?)",
		targetID,
		models.StatusTwoAccountCompleted,
		models.StatusTwoAccountFailed,
		models.StatusTwoAccountInitializing,
	).First(&session).Error

	if err != nil {
		return nil, err
	}
	return &session, nil
}

// UpdateStatus updates the session status
func (s *TwoAccountService) UpdateStatus(ctx context.Context, sessionID uint, status models.TwoAccountStatus, finding string) error {
	updateData := map[string]interface{}{
		"status": status,
	}
	if finding != "" {
		updateData["last_significant_finding"] = finding
	}
	if status == models.StatusTwoAccountCompleted || status == models.StatusTwoAccountFailed {
		now := time.Now()
		updateData["ended_at"] = &now
	}

	return s.db.Model(&models.TwoAccountSession{}).Where("id = ?", sessionID).Updates(updateData).Error
}

// ExecuteSmartAction executes a smart action with auth context injection
func (s *TwoAccountService) ExecuteSmartAction(ctx context.Context, session *models.TwoAccountSession, actionType string, params map[string]string) (*AnalysisResult, error) {
	// Load auth contexts
	var ctxA, ctxB models.AuthContext
	if err := s.db.First(&ctxA, session.AuthContextID_A).Error; err != nil {
		return nil, fmt.Errorf("failed to load auth context A: %w", err)
	}
	if err := s.db.First(&ctxB, session.AuthContextID_B).Error; err != nil {
		return nil, fmt.Errorf("failed to load auth context B: %w", err)
	}

	// Build target URL
	targetURL := params["url"]
	if targetURL == "" {
		targetURL = fmt.Sprintf("/api/targets/%d", session.TargetID)
		if id, ok := params["resource_id"]; ok {
			targetURL = fmt.Sprintf("%s/%s", targetURL, id)
		}
	}

	client := &http.Client{Timeout: 10 * time.Second}

	// Build request with Account A (victim)
	reqA, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request A: %w", err)
	}
	applyAuthContextToRequest(reqA, &ctxA)

	// Build request with Account B (attacker)
	reqB, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request B: %w", err)
	}
	applyAuthContextToRequest(reqB, &ctxB)

	// Send requests
	respA, err := client.Do(reqA)
	if err != nil {
		return nil, fmt.Errorf("request A failed: %w", err)
	}
	defer respA.Body.Close()

	respB, err := client.Do(reqB)
	if err != nil {
		return nil, fmt.Errorf("request B failed: %w", err)
	}
	defer respB.Body.Close()

	// Read response bodies
	bodyA, _ := io.ReadAll(respA.Body)
	bodyB, _ := io.ReadAll(respB.Body)

	// Compare responses
	result := CompareResponses(bodyA, bodyB, respA.StatusCode, respB.StatusCode)

	// Log to database
	s.logTestResult(session, actionType, targetURL, result)

	return result, nil
}

// applyAuthContextToRequest applies auth context headers to an HTTP request
func applyAuthContextToRequest(req *http.Request, ctx *models.AuthContext) {
	switch ctx.ContextType {
	case models.AuthContextCookie:
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", ctx.KeyName, ctx.Value))
	case models.AuthContextHeader:
		req.Header.Set(ctx.KeyName, ctx.Value)
	case models.AuthContextToken:
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", ctx.Value))
	case models.AuthContextSession:
		req.Header.Set("Cookie", fmt.Sprintf("%s=%s", ctx.KeyName, ctx.Value))
	}
}

// logTestResult logs the test result to the database
func (s *TwoAccountService) logTestResult(session *models.TwoAccountSession, actionType, targetURL string, result *AnalysisResult) {
	// Update session metrics
	session.RequestsSent += 2
	if result.IsVulnerable {
		session.DifferencesFound++
		session.PotentialBugs++
		session.LastSignificantFinding = result.DiffDetails
	}
	s.db.Save(session)
}

// AnalysisResult represents the result of a two-account analysis
type AnalysisResult struct {
	IsVulnerable    bool
	ConfidenceScore float64
	DiffDetails     string
	StatusCodeDiff  bool
	SizeDiff        int
}

// CompareResponses compares two HTTP responses for IDOR/BOLA indicators
func CompareResponses(bodyA, bodyB []byte, statusA, statusB int) *AnalysisResult {
	result := &AnalysisResult{
		StatusCodeDiff: statusA != statusB,
		SizeDiff:       len(bodyA) - len(bodyB),
	}

	// IDOR golden rule: status code mismatch (e.g., 200 vs 403)
	if result.StatusCodeDiff {
		if (statusA == 200 && statusB == 403) || (statusA == 200 && statusB == 401) {
			result.IsVulnerable = true
			result.ConfidenceScore = 0.95
			result.DiffDetails = fmt.Sprintf("Status Code Mismatch: A=%d, B=%d", statusA, statusB)
		}
	} else if statusA == 200 {
		// Both 200 - check content differences
		diffScore := analyzeContentDiff(bodyA, bodyB)
		if diffScore > 0.3 {
			result.IsVulnerable = true
			result.ConfidenceScore = 0.8 + (diffScore * 0.2)
			result.DiffDetails = "Significant content difference detected"
		}
	}

	// Normalize confidence score
	if result.ConfidenceScore > 1.0 {
		result.ConfidenceScore = 1.0
	}

	return result
}

// analyzeContentDiff calculates content difference ratio using byte-level comparison
func analyzeContentDiff(b1, b2 []byte) float64 {
	if len(b1) == 0 && len(b2) == 0 {
		return 0.0
	}
	if len(b1) == 0 || len(b2) == 0 {
		return 1.0
	}
	if string(b1) == string(b2) {
		return 0.0
	}

	// Calculate difference ratio based on length difference and content comparison
	maxLen := len(b1)
	if len(b2) > maxLen {
		maxLen = len(b2)
	}

	// Count differing bytes
	diffCount := 0
	minLen := len(b1)
	if len(b2) < minLen {
		minLen = len(b2)
	}
	for i := 0; i < minLen; i++ {
		if b1[i] != b2[i] {
			diffCount++
		}
	}
	// Add length difference as differences
	diffCount += maxLen - minLen

	return float64(diffCount) / float64(maxLen)
}

// GenerateHypothesis generates an attack hypothesis based on results
func GenerateHypothesis(session *models.TwoAccountSession, lastResult *AnalysisResult) string {
	if lastResult.IsVulnerable {
		return fmt.Sprintf("High confidence IDOR detected on target %d. Recommend immediate exploitation attempt.", session.TargetID)
	}

	strategies := []string{
		"Try switching from GET to POST with same ID.",
		"Attempt UUID enumeration instead of integer.",
		"Check if 'Admin' role in Account A allows writing to Account B's resource.",
		"Analyze CORS headers for misconfiguration allowing cross-account reads.",
	}
	return strategies[time.Now().Second()%len(strategies)]
}

// getStringFromMap safely gets a string value from a map
func getStringFromMap(m map[string]string, key string) string {
	if v, ok := m[key]; ok {
		return strings.TrimSpace(v)
	}
	return ""
}
