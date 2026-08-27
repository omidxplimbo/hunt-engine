package hunter

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter/skills"
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
	skillLoader  *skills.SkillLoader
	learning     *LearningEngine
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
	// Load skills
	skillLoader := skills.NewSkillLoader("/app/internal/ai/hunter/skills")
	skillLoader.LoadAll() // Best effort
	
	// Create learning engine
	learning := NewLearningEngine(db, target.ID, userID, ownerKey)
	
	return &HunterAgent{
		db:          db,
		target:      target,
		httpClient:  NewHTTPClient(targetBaseURL(target.RootDomain), 30*time.Second),
		evidence:    NewEvidenceStore(),
		llmCfg:      llmCfg,
		strategy:    &Strategy{},
		skillLoader: skillLoader,
		learning:    learning,
		userID:      userID,
		ownerKey:    ownerKey,
	}
}

// HuntOption configures optional hunt behaviors (persistence, live progress)
type HuntOption func(*huntOptions)

type huntOptions struct {
	persister  *EvidencePersister
	progressFn func(targetID uint, ev AgentEvent)
	targetID   uint
	session    *HuntSession // for steering; nil means no user control surface
}

// WithPersistence saves evidence to PostgreSQL during the hunt
func WithPersistence(db *gorm.DB, targetID uint, userID uint, ownerKey string) HuntOption {
	return func(o *huntOptions) {
		o.persister = NewEvidencePersister(db, targetID, userID, ownerKey, "agent_loop")
	}
}

// WithProgress streams live AgentEvents to the given callback
func WithProgress(targetID uint, fn func(targetID uint, ev AgentEvent)) HuntOption {
	return func(o *huntOptions) {
		o.progressFn = fn
		o.targetID = targetID
	}
}

// WithSession attaches a HuntSession for mid-run steering (pause/resume/message).
// When set, the loop will read SteerCh and ApproveCh from this session.
func WithSession(s *HuntSession) HuntOption {
	return func(o *huntOptions) {
		o.session = s
	}
}

func applyOptions(opts []HuntOption) *huntOptions {
	o := &huntOptions{}
	for _, opt := range opts {
		opt(o)
	}
	return o
}

// Hunt starts the hunting process with a given objective.
// Uses the LLM-driven AgentLoop for real AI-powered testing.
func (h *HunterAgent) Hunt(ctx context.Context, objective string, opts ...HuntOption) (*HunterResult, error) {
	log.Printf("[Hunter] Starting hunt on %s: %s", h.target.RootDomain, objective)
	options := applyOptions(opts)

	// Use the new LLM-driven AgentLoop
	agentLoop := NewAgentLoop(
		h.llmCfg,
		targetBaseURL(h.target.RootDomain),
		objective,
		h.skillLoader,
		h.evidence,
		h.learning,
	)
	if options.persister != nil {
		agentLoop.SetPersister(options.persister)
	}
	if options.progressFn != nil {
		tid := options.targetID
		agentLoop.SetProgressCallback(func(ev AgentEvent) {
			options.progressFn(tid, ev)
		})
	}
	if options.session != nil {
		agentLoop.AttachSession(options.session)
	}

	result, err := agentLoop.Run(ctx)
	if err != nil {
		log.Printf("[Hunter] AgentLoop error: %v, falling back to legacy mode", err)
		return h.legacyHunt(ctx, objective)
	}

	// Save to memory
	h.saveToMemory(result)

	return result, nil
}

// legacyHunt is the old scripted hunt mode, used as fallback
func (h *HunterAgent) legacyHunt(ctx context.Context, objective string) (*HunterResult, error) {
	log.Printf("[Hunter] Using legacy hunt mode")

	strategy, err := h.buildStrategy(ctx, objective)
	if err != nil {
		return nil, fmt.Errorf("failed to build strategy: %w", err)
	}
	h.strategy = strategy

	err = h.executeStrategy(ctx)
	if err != nil {
		log.Printf("[Hunter] Strategy execution error: %v", err)
	}

	result := h.analyzeResults()
	h.saveToMemory(result)

	return result, nil
}

// HuntMultiAgent runs the multi-agent mode: a supervisor dispatches one
// worker per bug class, all sharing the same evidence store.
func (h *HunterAgent) HuntMultiAgent(ctx context.Context, objective string, opts ...HuntOption) (*HunterResult, error) {
	log.Printf("[Hunter] Starting MULTI-AGENT hunt on %s: %s", h.target.RootDomain, objective)
	options := applyOptions(opts)

	targetURL := targetBaseURL(h.target.RootDomain)

	// Ask the LLM which bug classes to cover; fall back to defaults.
	bugClasses := h.selectBugClasses(ctx, objective)

	supervisor := NewSupervisor(h.llmCfg, targetURL, objective, bugClasses, h.skillLoader, h.evidence, h.learning)
	if options.persister != nil {
		supervisor.SetPersister(options.persister)
	}
	if options.progressFn != nil {
		tid := options.targetID
		supervisor.SetProgress(func(ev AgentEvent) {
			options.progressFn(tid, ev)
		})
	}
	if options.session != nil {
		supervisor.AttachSession(options.session)
	}

	supResult, err := supervisor.Run(ctx)
	if err != nil {
		log.Printf("[Hunter] Supervisor error: %v, falling back to single agent", err)
		return h.Hunt(ctx, objective, opts...)
	}

	result := &HunterResult{
		Strategy: &Strategy{
			Objective:  objective,
			BugClasses: bugClasses,
			Targets:    []string{targetURL},
			Status:     "completed",
			Findings:   []string{supResult.Summary},
		},
		Evidence:   h.evidence.GetAll(),
		Summary:    supResult.Summary,
		VulnsFound: supResult.VulnsFound,
		NextSteps:  h.multiAgentNextSteps(supResult),
	}

	h.saveToMemory(result)
	return result, nil
}

// selectBugClasses asks the LLM for relevant bug classes with a safe default.
func (h *HunterAgent) selectBugClasses(ctx context.Context, objective string) []string {
	if h.llmCfg == nil {
		return []string{"xss", "sqli"}
	}
	res, err := llmclient.GenerateJSON(ctx, h.llmCfg, llmclient.JSONRequest{
		SystemPrompt: "You are a penetration testing planner. Given an objective and target, return JSON with key bug_classes: a list of at most 3 vulnerability classes to test, chosen from: xss, sqli, ssrf, idor, ssti, command_injection. Return only the JSON.",
		UserPayload: map[string]interface{}{
			"objective": objective,
			"target":    h.target.RootDomain,
		},
		Temperature: 0.2,
		MaxTokens:   400,
	})
	if err != nil {
		log.Printf("[Hunter] Bug class selection failed, using defaults: %v", err)
		return []string{"xss", "sqli"}
	}
	classes := normalizeBugClasses(h.getStringSlice(res, "bug_classes"))
	if len(classes) == 0 || len(classes) > 3 {
		return []string{"xss", "sqli"}
	}
	return classes
}

func (h *HunterAgent) multiAgentNextSteps(sr *SupervisorResult) []string {
	steps := []string{}
	for _, ws := range sr.WorkerStatuses {
		if ws.Status == "failed" {
			steps = append(steps, fmt.Sprintf("Re-run failed worker %s: %s", ws.Name, ws.Error))
		}
	}
	if sr.VulnsFound > 0 {
		steps = append(steps, "Generate PoC reports for confirmed vulnerabilities")
	} else {
		steps = append(steps, "Expand scope or try different bug classes")
	}
	return steps
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
			BugClasses: normalizeBugClasses(h.getStringSlice(result, "bug_classes")),
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
	
	// If no URLs in DB, probe the target directly
	if len(urls) == 0 {
		log.Printf("[Hunter] No URLs in database, probing target directly: %s", h.target.RootDomain)
		urls = h.probeTarget()
	}
	
	// Filter to URLs with parameters
	var testableURLs []models.FoundURL
	for _, u := range urls {
		if extractParams(u.Value) != nil {
			testableURLs = append(testableURLs, u)
		}
	}
	
	// If still no testable URLs, test common endpoints
	if len(testableURLs) == 0 {
		log.Printf("[Hunter] No URLs with parameters, testing common endpoints")
		testableURLs = h.generateCommonEndpoints()
	}
	
	// Test each bug class
	for _, bugClass := range h.strategy.BugClasses {
		log.Printf("[Hunter] Testing bug class: %s on %d URLs", bugClass, len(testableURLs))
		
		// Use contains-based matching for flexibility
		classLower := strings.ToLower(bugClass)
		switch {
		case strings.Contains(classLower, "xss") || strings.Contains(classLower, "script"):
			h.testXSS(ctx, testableURLs)
		case strings.Contains(classLower, "sqli") || strings.Contains(classLower, "sql"):
			h.testSQLi(ctx, testableURLs)
		case strings.Contains(classLower, "idor") || strings.Contains(classLower, "bola") || strings.Contains(classLower, "authorization"):
			h.testIDOR(ctx, testableURLs)
		default:
			// For unknown classes, run XSS as default
			log.Printf("[Hunter] Unknown bug class '%s', running XSS test", bugClass)
			h.testXSS(ctx, testableURLs)
		}
		
		// Check if we found something - be creative!
		if h.evidence.Count() > 0 {
			confirmed := h.evidence.GetConfirmed()
			if len(confirmed) > 0 {
				log.Printf("[Hunter] Found %d confirmed vulnerabilities!", len(confirmed))
			}
		}
	}
	
	h.strategy.Status = "completed"
	return nil
}

// probeTarget makes initial requests to discover endpoints
func (h *HunterAgent) probeTarget() []models.FoundURL {
	var urls []models.FoundURL
	
	// Test common paths
	commonPaths := []string{
		"/", "/index.php", "/index.html", "/search", "/search.php",
		"/login", "/login.php", "/admin", "/api", "/api/v1",
		"/user", "/profile", "/dashboard", "/contact", "/about",
	}
	
	for _, path := range commonPaths {
		result := h.httpClient.Get(path, nil)
		if result.Error == "" && result.ResponseStatus == 200 {
			// Check if response contains forms or parameters
			if strings.Contains(result.ResponseBody, "<form") || 
			   strings.Contains(result.ResponseBody, "action=") ||
			   strings.Contains(result.ResponseBody, "<input") {
				log.Printf("[Hunter] Found form at %s", path)
			}
			// Add to URLs for further testing
			urls = append(urls, models.FoundURL{
				Value:  targetBaseURL(h.target.RootDomain) + path,
				Source: "hunter_probe",
			})
		}
	}
	
	return urls
}

// generateCommonEndpoints creates test URLs with common parameter patterns
func (h *HunterAgent) generateCommonEndpoints() []models.FoundURL {
	var urls []models.FoundURL
	baseURL := targetBaseURL(h.target.RootDomain)
	
	// Common parameter patterns to test
	testPatterns := []struct {
		path   string
		params []string
	}{
		{"/search", []string{"q", "query", "search", "keyword"}},
		{"/login", []string{"username", "password", "user", "email"}},
		{"/user", []string{"id", "user_id", "uid"}},
		{"/profile", []string{"id", "user_id"}},
		{"/api/users", []string{"id", "page", "limit"}},
		{"/api/search", []string{"q", "query"}},
		{"/page", []string{"id", "page", "p"}},
		{"/product", []string{"id", "product_id"}},
		{"/item", []string{"id", "item_id"}},
		{"/redirect", []string{"url", "next", "return", "goto"}},
	}
	
	for _, pattern := range testPatterns {
		for _, param := range pattern.params {
			testURL := fmt.Sprintf("%s%s?%s=test", baseURL, pattern.path, param)
			urls = append(urls, models.FoundURL{
				Value:  testURL,
				Source: "hunter_generated",
			})
		}
	}
	
	log.Printf("[Hunter] Generated %d test URLs with parameters", len(urls))
	return urls
}

// testXSS tests for XSS vulnerabilities with multiple payload types
func (h *HunterAgent) testXSS(ctx context.Context, urls []models.FoundURL) {
	// Categorized payloads for different contexts
	payloadSets := map[string][]string{
		"basic": {
			"<script>alert(1)</script>",
			"'><script>alert(1)</script>",
			"\"><script>alert(1)</script>",
		},
		"event_handler": {
			"<img src=x onerror=alert(1)>",
			"<svg onload=alert(1)>",
			"<body onload=alert(1)>",
			"<input onfocus=alert(1) autofocus>",
			"<details open ontoggle=alert(1)>",
		},
		"encoding_bypass": {
			"<script>alert(String.fromCharCode(88,83,83))</script>",
			"javascript:alert(1)",
			"<img src=\"javascript:alert(1)\">",
			"<a href=\"data:text/html,<script>alert(1)</script>\">click</a>",
		},
		"filter_bypass": {
			"<scr<script>ipt>alert(1)</scr</script>ipt>",
			"<SCRIPT>alert(1)</SCRIPT>",
			"<scr\x00ipt>alert(1)</scr\x00ipt>",
			"\\x3cscript\\x3ealert(1)\\x3c/script\\x3e",
		},
	}
	
	for _, u := range urls {
		params := extractParams(u.Value)
		if len(params) == 0 {
			continue
		}
		
		for _, param := range params {
			found := false
			for category, payloads := range payloadSets {
				if found {
					break
				}
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
						Metadata: map[string]interface{}{
							"payload_category": category,
						},
					}
					
					if analysis.IsVulnerable {
						evidence.Status = "confirmed"
						evidence.PoC = fmt.Sprintf("GET %s?%s=%s", u.Value, param, payload)
						log.Printf("[Hunter] XSS FOUND [%s]: %s?%s=%s (confidence: %.2f)", category, u.Value, param, payload, analysis.Confidence)
						found = true
					}
					
					h.evidence.Add(evidence)
				}
			}
		}
	}
}

// testSQLi tests for SQL injection with multiple techniques
func (h *HunterAgent) testSQLi(ctx context.Context, urls []models.FoundURL) {
	// Categorized SQLi payloads
	payloadSets := map[string][]string{
		"error_based": {
			"'",
			"\"",
			"' OR '1'='1",
			"' OR '1'='1'--",
			"' OR '1'='1'/*",
			"1' AND '1'='1",
			"admin'--",
		},
		"union_based": {
			"' UNION SELECT NULL--",
			"' UNION SELECT NULL,NULL--",
			"' UNION SELECT NULL,NULL,NULL--",
			"' UNION ALL SELECT NULL--",
		},
		"blind_boolean": {
			"' AND 1=1--",
			"' AND 1=2--",
			"' OR 1=1--",
			"' OR 1=2--",
		},
		"blind_time": {
			"'; WAITFOR DELAY '0:0:5'--",
			"' AND SLEEP(5)--",
			"' AND pg_sleep(5)--",
			"1; WAITFOR DELAY '0:0:5'--",
		},
	}
	
	for _, u := range urls {
		params := extractParams(u.Value)
		if len(params) == 0 {
			continue
		}
		
		for _, param := range params {
			found := false
			for category, payloads := range payloadSets {
				if found {
					break
				}
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
						Metadata: map[string]interface{}{
							"payload_category": category,
						},
					}
					
					if analysis.IsVulnerable {
						evidence.Status = "confirmed"
						evidence.PoC = fmt.Sprintf("GET %s?%s=%s", u.Value, param, payload)
						log.Printf("[Hunter] SQLi FOUND [%s]: %s?%s=%s (confidence: %.2f)", category, u.Value, param, payload, analysis.Confidence)
						found = true
					}
					
					h.evidence.Add(evidence)
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
	clientA := NewHTTPClient(targetBaseURL(h.target.RootDomain), 30*time.Second)
	clientB := NewHTTPClient(targetBaseURL(h.target.RootDomain), 30*time.Second)
	
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
	
	// Learn from results
	insights := h.learning.LearnFromResults(h.evidence.GetAll())
	
	// Check if we should spawn sub-agents for creative discoveries
	spawnClasses := h.learning.ShouldSpawnSubAgent(insights)
	if len(spawnClasses) > 0 {
		log.Printf("[Hunter] Creative discovery! Should spawn sub-agents for: %v", spawnClasses)
		// TODO: Actually spawn sub-agents in future version
	}
	
	// Build next steps from learning insights
	nextSteps := []string{}
	if len(confirmed) > 0 {
		nextSteps = append(nextSteps, "Generate detailed PoC reports for confirmed vulnerabilities")
		nextSteps = append(nextSteps, "Test for vulnerability chaining")
	}
	for _, insight := range insights {
		if insight.Type == "creative" {
			nextSteps = append(nextSteps, insight.NextSteps...)
		}
	}
	if len(nextSteps) == 0 {
		nextSteps = append(nextSteps, "Try different payloads or injection points")
		nextSteps = append(nextSteps, "Expand URL discovery with more crawling")
	}
	
	summary := fmt.Sprintf("Hunt completed on %s. Tested %d bug classes. Found %d confirmed vulnerabilities out of %d tests. Learned %d insights.",
		h.target.RootDomain, len(h.strategy.BugClasses), len(confirmed), h.evidence.Count(), len(insights))
	
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
	objective := h.strategy.Objective
	if objective == "" && result.Strategy != nil {
		objective = result.Strategy.Objective
	}
	if objective == "" {
		objective = "General assessment"
	}
	memoryItem := models.TargetMemoryItem{
		TargetID:   h.target.ID,
		UserID:     h.userID,
		OwnerKey:   h.ownerKey,
		Title:      fmt.Sprintf("Hunt: %s", objective),
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

// targetBaseURL builds the base URL for a target, honoring an explicit
// http:// scheme in RootDomain (used by local test fixtures).
func targetBaseURL(domain string) string {
	if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
		return domain
	}
	return "https://" + domain
}

// normalizeBugClasses normalizes LLM-generated bug class names to standard names
func normalizeBugClasses(classes []string) []string {
	normalizationMap := map[string]string{
		"reflected xss":              "xss",
		"stored xss":                 "xss",
		"dom xss":                    "xss",
		"dom-based xss":              "xss",
		"xss via query parameters":   "xss",
		"xss via post data":          "xss",
		"xss in http headers":        "xss",
		"cross-site scripting":       "xss",
		"cross-site scripting (xss)": "xss",
		"xss_reflected":              "xss",
		"xss_stored":                 "xss",
		"xss_dom":                    "xss",
		"sql injection":              "sqli",
		"sql injection (sqli)":       "sqli",
		"blind sql injection":        "sqli",
		"error-based sqli":           "sqli",
		"union-based sqli":           "sqli",
		"sqli_injection":             "sqli",
		"insecure direct object reference": "idor",
		"broken object level authorization": "idor",
		"idor/bola":                  "idor",
		"idor_bola":                  "idor",
		"server-side request forgery": "ssrf",
		"ssrf (server-side request forgery)": "ssrf",
		"ssrf_testing":               "ssrf",
		"command injection":          "command_injection",
		"os command injection":       "command_injection",
		"remote code execution":      "rce",
		"template injection":         "ssti",
		"server-side template injection": "ssti",
		"open redirect":              "open_redirect",
		"path traversal":             "path_traversal",
		"local file inclusion":       "lfi",
		"cross-site request forgery": "csrf",
		"parameter injection":        "parameter_injection",
		"header injection":           "header_injection",
		"crlf injection":             "crlf",
	}
	
	result := make([]string, 0, len(classes))
	seen := make(map[string]bool)
	
	for _, class := range classes {
		normalized := strings.ToLower(strings.TrimSpace(class))
		if mapped, ok := normalizationMap[normalized]; ok {
			normalized = mapped
		}
		if !seen[normalized] {
			seen[normalized] = true
			result = append(result, normalized)
		}
	}
	
	return result
}
