package hunter

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
	"gorm.io/gorm"
)

// HunterAgent is the main AI hacking agent
type HunterAgent struct {
	db           *gorm.DB
	target       models.Target
	httpClient   *HTTPClient
	evidence     *EvidenceStore
	llmCfg       *llmclient.Config
	strategy     *Strategy
	sessionID    uint
	userID       uint
	ownerKey     string
}

// Strategy represents the current hunting strategy
type Strategy struct {
	Objective    string   `json:"objective"`     // e.g., "Find XSS vulnerabilities"
	BugClasses   []string `json:"bug_classes"`    // e.g., ["xss", "sqli"]
	Targets      []string `json:"targets"`        // URLs/endpoints to test
	Priority     int      `json:"priority"`       // 1-10
	Status       string   `json:"status"`         // planning, executing, completed
	CurrentStep  int      `json:"current_step"`
	TotalSteps   int      `json:"total_steps"`
	Findings     []string `json:"findings"`
	NextActions  []string `json:"next_actions"`
}

// HunterResult represents the result of a hunting session
type HunterResult struct {
	Strategy     *Strategy    `json:"strategy"`
	Evidence     []Evidence   `json:"evidence"`
	Summary      string       `json:"summary"`
	VulnsFound   int          `json:"vulns_found"`
	Confidence   float64      `json:"confidence"`
	NextSteps    []string     `json:"next_steps"`
}

// NewHunterAgent creates a new Hunter Agent
func NewHunterAgent(db *gorm.DB, target models.Target, llmCfg *llmclient.Config, userID uint, ownerKey string) *HunterAgent {
	return &HunterAgent{
		db:         db,
		target:     target,
		httpClient: NewHTTPClient(fmt.Sprintf("https://%s", target.RootDomain), 30*time.Second),
		evidence:   NewEvidenceStore(),
		llmCfg:     llmCfg,
		strategy:   &Strategy{},
		userID:     userID,
		ownerKey:   ownerKey,
	}
}

// Hunt starts the hunting process with a given objective
func (h *HunterAgent) Hunt(ctx context.Context, objective string) (*HunterResult, error) {
	log.Printf("[Hunter] Starting hunt on %s: %s", h.target.RootDomain, objective)
	
	// Step 1: Build strategy using LLM
	strategy, err := h.buildStrategy(ctx, objective)
	if err != nil {
		return nil, fmt.Errorf("failed to build strategy: %w", err)
	}
	h.strategy = strategy
	
	// Step 2: Execute the strategy
	err = h.executeStrategy(ctx)
	if err != nil {
		log.Printf("[Hunter] Strategy execution error: %v", err)
	}
	
	// Step 3: Analyze results and learn
	result := h.analyzeResults()
	
	// Step 4: Save to memory
	h.saveToMemory(result)
	
	return result, nil
}

// buildStrategy uses LLM to build a hunting strategy
func (h *HunterAgent) buildStrategy(ctx context.Context, objective string) (*Strategy, error) {
	// Load target context
	targetContext := h.buildTargetContext()
	
	// Load memory context
	memoryContext := h.buildMemoryContext()
	
	// Build prompt for strategy
	systemPrompt := `You are an expert penetration tester creating a hunting strategy.
Based on the target information and objective, create a detailed testing strategy.

Return JSON with:
- objective: clear objective statement
- bug_classes: list of vulnerability classes to test (xss, sqli, idor, ssrf, etc.)
- targets: list of specific URLs/endpoints to test
- priority: 1-10
- steps: numbered list of testing steps

Be specific and practical. Focus on high-impact vulnerabilities.`

	if h.llmCfg != nil {
		result, err := llmclient.GenerateJSON(ctx, h.llmCfg, llmclient.JSONRequest{
			SystemPrompt: systemPrompt,
			UserPayload: map[string]interface{}{
				"objective": objective,
				"target":    h.target.RootDomain,
				"context":   targetContext,
				"memory":    memoryContext,
			},
			Temperature: 0.3,
			MaxTokens:   2000,
		})
		if err != nil {
			log.Printf("[Hunter] LLM strategy generation failed: %v, using fallback", err)
			return h.fallbackStrategy(objective), nil
		}
		
		strategy := &Strategy{
			Objective:  objective,
			BugClasses: h.getStringSlice(result, "bug_classes"),
			Targets:    h.getStringSlice(result, "targets"),
			Priority:   h.getInt(result, "priority"),
			Status:     "planning",
		}
		
		if len(strategy.BugClasses) == 0 {
			strategy.BugClasses = []string{"xss", "sqli", "idor"}
		}
		
		return strategy, nil
	}
	
	return h.fallbackStrategy(objective), nil
}

// fallbackStrategy creates a basic strategy without LLM
func (h *HunterAgent) fallbackStrategy(objective string) *Strategy {
	objLower := strings.ToLower(objective)
	bugClasses := []string{}
	
	if strings.Contains(objLower, "xss") {
		bugClasses = append(bugClasses, "xss")
	}
	if strings.Contains(objLower, "sqli") || strings.Contains(objLower, "sql") {
		bugClasses = append(bugClasses, "sqli")
	}
	if strings.Contains(objLower, "idor") || strings.Contains(objLower, "id=") {
		bugClasses = append(bugClasses, "idor")
	}
	if strings.Contains(objLower, "ssrf") {
		bugClasses = append(bugClasses, "ssrf")
	}
	if len(bugClasses) == 0 {
		bugClasses = []string{"xss", "sqli", "idor"}
	}
	
	return &Strategy{
		Objective:  objective,
		BugClasses: bugClasses,
		Targets:    []string{"/"},
		Priority:   5,
		Status:     "planning",
	}
}

// executeStrategy executes the hunting strategy
func (h *HunterAgent) executeStrategy(ctx context.Context) error {
	h.strategy.Status = "executing"
	
	// Load found URLs from database
	var urls []models.FoundURL
	h.db.Where("target_id = ?", h.target.ID).Limit(100).Find(&urls)
	
	// Test each bug class
	for _, bugClass := range h.strategy.BugClasses {
		log.Printf("[Hunter] Testing bug class: %s", bugClass)
		
		switch bugClass {
		case "xss":
			h.testXSS(ctx, urls)
		case "sqli":
			h.testSQLi(ctx, urls)
		case "idor":
			h.testIDOR(ctx, urls)
		}
		
		// Check if we found something - be creative!
		if h.evidence.Count() > 0 {
			confirmed := h.evidence.GetConfirmed()
			if len(confirmed) > 0 {
				log.Printf("[Hunter] Found %d confirmed vulnerabilities!", len(confirmed))
				// Could spawn sub-agents here for deeper testing
			}
		}
	}
	
	h.strategy.Status = "completed"
	return nil
}

// testXSS tests for XSS vulnerabilities
func (h *HunterAgent) testXSS(ctx context.Context, urls []models.FoundURL) {
	payloads := []string{
		"<script>alert(1)</script>",
		"'><script>alert(1)</script>",
		"\"><script>alert(1)</script>",
		"<img src=x onerror=alert(1)>",
		"javascript:alert(1)",
	}
	
	for _, u := range urls {
		params := extractParams(u.Value)
		if len(params) == 0 {
			continue
		}
		
		for _, param := range params {
			for _, payload := range payloads {
				result := h.httpClient.SendPayload("GET", u.Value, payload, "param:"+param)
				
				analysis := AnalyzeXSSResponse(result, payload)
				
				evidence := Evidence{
					TestType:   "xss",
					Target:     u.Value,
					Parameter:  param,
					Payload:    payload,
					Result:     result,
					Analysis:   analysis,
					Confidence: analysis.Confidence,
					Severity:   analysis.Severity,
				}
				
				if analysis.IsVulnerable {
					evidence.Status = "confirmed"
					evidence.PoC = fmt.Sprintf("GET %s?%s=%s", u.Value, param, payload)
					log.Printf("[Hunter] XSS FOUND: %s?%s=%s (confidence: %.2f)", u.Value, param, payload, analysis.Confidence)
				}
				
				h.evidence.Add(evidence)
				
				if analysis.IsVulnerable && analysis.Confidence > 0.7 {
					break
				}
			}
		}
	}
}

// testSQLi tests for SQL injection
func (h *HunterAgent) testSQLi(ctx context.Context, urls []models.FoundURL) {
	payloads := []string{
		"'",
		"' OR '1'='1",
		"1' AND '1'='1",
		"' UNION SELECT NULL--",
		"1; WAITFOR DELAY '0:0:5'--",
	}
	
	for _, u := range urls {
		params := extractParams(u.Value)
		if len(params) == 0 {
			continue
		}
		
		for _, param := range params {
			for _, payload := range payloads {
				result := h.httpClient.SendPayload("GET", u.Value, payload, "param:"+param)
				
				analysis := AnalyzeSQLiResponse(result, payload)
				
				evidence := Evidence{
					TestType:   "sqli",
					Target:     u.Value,
					Parameter:  param,
					Payload:    payload,
					Result:     result,
					Analysis:   analysis,
					Confidence: analysis.Confidence,
					Severity:   analysis.Severity,
				}
				
				if analysis.IsVulnerable {
					evidence.Status = "confirmed"
					evidence.PoC = fmt.Sprintf("GET %s?%s=%s", u.Value, param, payload)
					log.Printf("[Hunter] SQLi FOUND: %s?%s=%s (confidence: %.2f)", u.Value, param, payload, analysis.Confidence)
				}
				
				h.evidence.Add(evidence)
				
				if analysis.IsVulnerable && analysis.Confidence > 0.7 {
					break
				}
			}
		}
	}
}

// testIDOR tests for IDOR vulnerabilities
func (h *HunterAgent) testIDOR(ctx context.Context, urls []models.FoundURL) {
	// IDOR testing requires two accounts - check if we have auth contexts
	var contexts []models.AuthContext
	h.db.Where("target_id = ? AND is_active = ?", h.target.ID, true).Find(&contexts)
	
	if len(contexts) < 2 {
		log.Printf("[Hunter] IDOR testing requires 2 auth contexts, found %d", len(contexts))
		return
	}
	
	// Use two different accounts
	clientA := NewHTTPClient(fmt.Sprintf("https://%s", h.target.RootDomain), 30*time.Second)
	clientB := NewHTTPClient(fmt.Sprintf("https://%s", h.target.RootDomain), 30*time.Second)
	
	// Set auth for each client
	applyAuthToClient(clientA, &contexts[0])
	applyAuthToClient(clientB, &contexts[1])
	
	// Test endpoints that might have IDOR
	for _, u := range urls {
		params := extractParams(u.Value)
		hasIDParam := false
		for _, p := range params {
			if isIDParam(p) {
				hasIDParam = true
				break
			}
		}
		
		if !hasIDParam {
			continue
		}
		
		// Send same request with both accounts
		resultA := clientA.Get(u.Value, nil)
		resultB := clientB.Get(u.Value, nil)
		
		analysis := AnalyzeIDORResponse(resultA, resultB)
		
		evidence := Evidence{
			TestType:   "idor",
			Target:     u.Value,
			Result:     resultA,
			Analysis:   analysis,
			Confidence: analysis.Confidence,
			Severity:   analysis.Severity,
		}
		
		if analysis.IsVulnerable {
			evidence.Status = "confirmed"
			evidence.PoC = fmt.Sprintf("Account A: %s -> %d, Account B: %s -> %d", 
				contexts[0].Name, resultA.ResponseStatus,
				contexts[1].Name, resultB.ResponseStatus)
			log.Printf("[Hunter] IDOR FOUND: %s (confidence: %.2f)", u.Value, analysis.Confidence)
		}
		
		h.evidence.Add(evidence)
	}
}

// analyzeResults analyzes all collected evidence
func (h *HunterAgent) analyzeResults() *HunterResult {
	confirmed := h.evidence.GetConfirmed()
	
	summary := fmt.Sprintf("Hunt completed on %s. Tested %d bug classes. Found %d confirmed vulnerabilities out of %d tests.",
		h.target.RootDomain, len(h.strategy.BugClasses), len(confirmed), h.evidence.Count())
	
	nextSteps := []string{}
	if len(confirmed) > 0 {
		nextSteps = append(nextSteps, "Generate detailed PoC reports for confirmed vulnerabilities")
		nextSteps = append(nextSteps, "Test for vulnerability chaining")
	} else {
		nextSteps = append(nextSteps, "Try different payloads or injection points")
		nextSteps = append(nextSteps, "Expand URL discovery with more crawling")
	}
	
	return &HunterResult{
		Strategy:   h.strategy,
		Evidence:   h.evidence.GetAll(),
		Summary:    summary,
		VulnsFound: len(confirmed),
		NextSteps:  nextSteps,
	}
}

// saveToMemory saves the hunting results to target memory
func (h *HunterAgent) saveToMemory(result *HunterResult) {
	// Save summary as memory item
	memoryItem := models.TargetMemoryItem{
		TargetID:   h.target.ID,
		UserID:     h.userID,
		OwnerKey:   h.ownerKey,
		Title:      fmt.Sprintf("Hunt: %s", h.strategy.Objective),
		Summary:    result.Summary,
		Content:    fmt.Sprintf("Found %d vulnerabilities", result.VulnsFound),
		MemoryType: "hunt_result",
		SourceType: "hunter_agent",
		Confidence: int(result.Confidence * 100),
		Importance: 80,
	}
	h.db.Create(&memoryItem)
}

// buildTargetContext builds context about the target
func (h *HunterAgent) buildTargetContext() string {
	var assetCount, urlCount, findingCount int64
	h.db.Model(&models.Asset{}).Where("target_id = ?", h.target.ID).Count(&assetCount)
	h.db.Model(&models.FoundURL{}).Where("target_id = ?", h.target.ID).Count(&urlCount)
	h.db.Model(&models.Finding{}).Where("target_id = ?", h.target.ID).Count(&findingCount)
	
	return fmt.Sprintf("Domain: %s\nAssets: %d\nURLs: %d\nFindings: %d",
		h.target.RootDomain, assetCount, urlCount, findingCount)
}

// buildMemoryContext builds context from previous hunts
func (h *HunterAgent) buildMemoryContext() string {
	var memories []models.TargetMemoryItem
	h.db.Where("target_id = ? AND memory_type = ?", h.target.ID, "hunt_result").
		Order("created_at DESC").Limit(5).Find(&memories)
	
	if len(memories) == 0 {
		return "No previous hunt results."
	}
	
	var sb strings.Builder
	for _, m := range memories {
		sb.WriteString(fmt.Sprintf("- %s: %s\n", m.Title, m.Summary))
	}
	return sb.String()
}

// Helper functions
func (h *HunterAgent) getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			result := make([]string, 0, len(arr))
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func (h *HunterAgent) getInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		if f, ok := v.(float64); ok {
			return int(f)
		}
	}
	return 5
}

func extractParams(rawURL string) []string {
	// Simple parameter extraction from URL
	idx := strings.Index(rawURL, "?")
	if idx == -1 {
		return nil
	}
	query := rawURL[idx+1:]
	params := []string{}
	for _, pair := range strings.Split(query, "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) > 0 {
			params = append(params, kv[0])
		}
	}
	return params
}

func isIDParam(name string) bool {
	lower := strings.ToLower(name)
	idParams := []string{"id", "user_id", "account_id", "uid", "profile_id", "order_id", "item_id"}
	for _, p := range idParams {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func applyAuthToClient(client *HTTPClient, ctx *models.AuthContext) {
	switch ctx.ContextType {
	case models.AuthContextCookie:
		client.SetCookies(fmt.Sprintf("%s=%s", ctx.KeyName, ctx.Value))
	case models.AuthContextHeader:
		client.SetHeader(ctx.KeyName, ctx.Value)
	case models.AuthContextToken:
		client.SetAuth("bearer", ctx.Value)
	}
}
