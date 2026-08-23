package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/engine/smart_analyzer"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

// ExecuteTwoAccountTestWithRecovery is a safe wrapper with panic recovery
func ExecuteTwoAccountTestWithRecovery(targetID, userID, actionID uint) (result map[string]interface{}) {
	defer func() {
		if r := recover(); r != nil {
			result = map[string]interface{}{
				"status":   "error",
				"message":  fmt.Sprintf("Panic recovered: %v", r),
				"executed": false,
			}
		}
	}()
	return ExecuteTwoAccountTest(targetID, userID, actionID)
}

// ExecuteTwoAccountTest performs real two-account IDOR/BOLA testing
func ExecuteTwoAccountTest(targetID, userID, actionID uint) map[string]interface{} {
	// 1. Load the agent action to get input parameters
	var action models.AgentAction
	if err := database.DB.First(&action, actionID).Error; err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to load agent action: %v", err),
		}
	}

	// 2. Parse input JSON to get test parameters
	inputMap := make(map[string]interface{})
	if len(action.InputJSON) > 0 {
		_ = json.Unmarshal(action.InputJSON, &inputMap)
	}

	// Get target URL from input or use default API endpoint
	targetURL := getStringFromMap(inputMap, "url")
	if targetURL == "" {
		// Default: test the target's API endpoints
		var target models.Target
		if err := database.DB.First(&target, targetID).Error; err != nil {
			return map[string]interface{}{
				"status":  "error",
				"message": fmt.Sprintf("Failed to load target: %v", err),
			}
		}
		targetURL = fmt.Sprintf("https://%s", target.RootDomain)
	}

	// 3. Get auth context IDs from input or find active contexts
	contextAID := getUintFromMap(inputMap, "auth_context_id_a")
	contextBID := getUintFromMap(inputMap, "auth_context_id_b")

	if contextAID == 0 || contextBID == 0 {
		// Try to find two active auth contexts for this target
		var contexts []models.AuthContext
		database.DB.Where("target_id = ? AND user_id = ? AND is_active = ?", targetID, userID, true).
			Limit(2).Find(&contexts)

		if len(contexts) < 2 {
			return map[string]interface{}{
				"status":  "error",
				"message": "Two auth contexts required for two-account testing. Please create Auth Context A (victim) and Auth Context B (attacker).",
			}
		}
		contextAID = contexts[0].ID
		contextBID = contexts[1].ID
	}

	// 4. Load auth contexts
	var ctxA, ctxB models.AuthContext
	if err := database.DB.First(&ctxA, contextAID).Error; err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to load auth context A: %v", err),
		}
	}
	if err := database.DB.First(&ctxB, contextBID).Error; err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to load auth context B: %v", err),
		}
	}

	// 5. Create or update two-account session
	session := models.TwoAccountSession{
		TargetID:        targetID,
		OwnerID:         userID,
		AuthContextID_A: contextAID,
		AuthContextID_B: contextBID,
		ContextA_Name:   ctxA.Name,
		ContextB_Name:   ctxB.Name,
		Status:          models.TwoAccountStatusRunning,
		CurrentStrategy: "parameter_fuzzing",
		StartedAt:       time.Now().UTC(),
	}
	database.DB.Create(&session)

	// 6. Execute test requests with both accounts
	client := &http.Client{Timeout: 15 * time.Second}

	// Build request for Account A (victim)
	reqA, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to create request A: %v", err),
		}
	}
	applyAuthContext(reqA, &ctxA)

	// Build request for Account B (attacker)
	reqB, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to create request B: %v", err),
		}
	}
	applyAuthContext(reqB, &ctxB)

	// Send requests
	respA, err := client.Do(reqA)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Request A failed: %v", err),
		}
	}
	defer respA.Body.Close()

	respB, err := client.Do(reqB)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Request B failed: %v", err),
		}
	}
	defer respB.Body.Close()

	// Read response bodies
	bodyA, _ := io.ReadAll(respA.Body)
	bodyB, _ := io.ReadAll(respB.Body)

	// 7. Analyze responses using smart_analyzer
	analysis := smart_analyzer.IntelligentComparator(respA, respB, bodyA, bodyB)

	// 8. Update session with results
	session.RequestsSent = 2
	if analysis.IsVulnerable {
		session.DifferencesFound = 1
		session.PotentialBugs = 1
		if len(analysis.Evidence) > 0 {
			session.LastSignificantFinding = analysis.Evidence[0]
		}
	}
	session.Status = models.StatusTwoAccountCompleted
	now := time.Now().UTC()
	session.EndedAt = &now
	database.DB.Save(&session)

	// 9. Generate hypothesis for next steps
	hypothesis := smart_analyzer.GenerateHypothesis(&session, analysis)

	// 10. Return structured result
	return map[string]interface{}{
		"status":            "success",
		"executed":          true,
		"session_id":        session.ID,
		"target_url":        targetURL,
		"is_vulnerable":     analysis.IsVulnerable,
		"confidence_score":  analysis.ConfidenceScore,
		"vulnerability_type": analysis.VulnerabilityType,
		"evidence":          analysis.Evidence,
		"next_step_hint":    analysis.NextStepHint,
		"hypothesis":        hypothesis,
		"requests_sent":     2,
		"account_a":         ctxA.Name,
		"account_b":         ctxB.Name,
		"status_code_a":     respA.StatusCode,
		"status_code_b":     respB.StatusCode,
		"body_size_a":       len(bodyA),
		"body_size_b":       len(bodyB),
	}
}

// applyAuthContext applies authentication context to an HTTP request
func applyAuthContext(req *http.Request, ctx *models.AuthContext) {
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

// getStringFromMap safely gets a string value from a map
func getStringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

// getUintFromMap safely gets a uint value from a map
func getUintFromMap(m map[string]interface{}, key string) uint {
	if v, ok := m[key]; ok {
		switch val := v.(type) {
		case float64:
			return uint(val)
		case int:
			return uint(val)
		case uint:
			return val
		}
	}
	return 0
}
