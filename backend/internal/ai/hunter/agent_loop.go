package hunter

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter/skills"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/hunter/tools"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
)

const (
	maxTurns         = 20
	maxToolOutputLen = 4000
)

// AgentLoop is the LLM-driven tool-calling agent loop.
// It replaces the old scripted executeStrategy() with a real AI agent
// that decides which tools to use based on the target and objective.
type AgentLoop struct {
	llmCfg      *llmclient.Config
	registry    *tools.ToolRegistry
	skillLoader *skills.SkillLoader
	evidence    *EvidenceStore
	learning    *LearningEngine
	persister   *EvidencePersister // optional DB persistence
	progressFn  func(AgentEvent)   // optional live progress callback
	session     *HuntSession       // optional steering target (T2 wires this in)
	target      string
	objective   string
	history     []map[string]string
	findings    []string
	// strategyTargets records URLs that returned 200 during recon
	strategyTargets map[string]bool
}

// AgentEvent is a live progress event emitted during the hunt (for WebSocket)
type AgentEvent struct {
	Type       string `json:"type"` // turn, tool_call, tool_result, finding, done, error
	Turn       int    `json:"turn,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	Detail     string `json:"detail"`
	BugClass   string `json:"bug_class,omitempty"`
	Timestamp  string `json:"timestamp"`
}

// AgentTurnResult represents the result of a single agent turn
type AgentTurnResult struct {
	Turn      int    `json:"turn"`
	Thinking  string `json:"thinking"`
	ToolName  string `json:"tool_name,omitempty"`
	ToolInput string `json:"tool_input,omitempty"`
	ToolOutput string `json:"tool_output,omitempty"`
	Response  string `json:"response,omitempty"`
	Error     string `json:"error,omitempty"`
}

// NewAgentLoop creates a new LLM-driven agent loop
func NewAgentLoop(
	llmCfg *llmclient.Config,
	target string,
	objective string,
	skillLoader *skills.SkillLoader,
	evidence *EvidenceStore,
	learning *LearningEngine,
) *AgentLoop {
	registry := tools.NewToolRegistry()

	// Register all available tools
	registry.Register(tools.NewShellTool())
	registry.Register(tools.NewHTTPTool())
	registry.Register(tools.NewBrowserTool())
	registry.Register(tools.NewProxyTool())

	return &AgentLoop{
		llmCfg:          llmCfg,
		registry:        registry,
		skillLoader:     skillLoader,
		evidence:        evidence,
		learning:        learning,
		target:          target,
		objective:       objective,
		history:         make([]map[string]string, 0),
		findings:        make([]string, 0),
		strategyTargets: make(map[string]bool),
	}
}

// SetPersister enables DB persistence of evidence
func (a *AgentLoop) SetPersister(p *EvidencePersister) { a.persister = p }

// SetProgressCallback wires live progress events (WebSocket streaming)
func (a *AgentLoop) SetProgressCallback(fn func(AgentEvent)) { a.progressFn = fn }

// AttachSession binds the loop to a HuntSession for mid-run steering.
// T1 plumbs the option; T2 wires the actual select{} on SteerCh and
// the pause/approval gates into the turn loop.
func (a *AgentLoop) AttachSession(s *HuntSession) { a.session = s }

// emit sends a progress event if a callback is registered
func (a *AgentLoop) emit(eventType, detail, bugClass string, turn int) {
	if a.progressFn == nil {
		return
	}
	a.progressFn(AgentEvent{
		Type:      eventType,
		Turn:      turn,
		ToolName:  "",
		Detail:    detail,
		BugClass:  bugClass,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

// Run executes the agent loop until the objective is achieved or max turns reached
func (a *AgentLoop) Run(ctx context.Context) (*HunterResult, error) {
	log.Printf("[AgentLoop] Starting on %s: %s", a.target, a.objective)

	// Build system prompt with skills
	systemPrompt := a.buildSystemPrompt()

	// Initialize conversation
	a.history = append(a.history, map[string]string{
		"role":    "system",
		"content": systemPrompt,
	})

	a.history = append(a.history, map[string]string{
		"role":    "user",
		"content": fmt.Sprintf("Target: %s\nObjective: %s\n\nBegin the security assessment. Use the available tools to test the target.", a.target, a.objective),
	})

	// Agent loop
	llmErrors := 0
	for turn := 0; turn < maxTurns; turn++ {
		log.Printf("[AgentLoop] Turn %d/%d", turn+1, maxTurns)
		a.emit("turn", fmt.Sprintf("Turn %d of %d", turn+1, maxTurns), "", turn+1)

		// Call LLM with retry/backoff on rate limits and transient errors
		var response string
		var err error
		for attempt := 0; attempt < 3; attempt++ {
			response, err = a.callLLM(ctx)
			if err == nil {
				break
			}
			errStr := err.Error()
			isRateLimit := strings.Contains(errStr, "429") || strings.Contains(errStr, "rate")
			backoff := time.Duration(1<<uint(llmErrors)) * 5 * time.Second
			if backoff > 60*time.Second {
				backoff = 60 * time.Second
			}
			log.Printf("[AgentLoop] LLM error on turn %d (attempt %d): %v — backing off %v", turn+1, attempt+1, err, backoff)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			if isRateLimit {
				continue
			}
			// Non-rate-limit errors: one retry then give up this turn
			response, err = a.callLLM(ctx)
			break
		}
		if err != nil {
			llmErrors++
			if llmErrors >= 3 {
				log.Printf("[AgentLoop] Giving up after %d consecutive LLM failures", llmErrors)
				return nil, fmt.Errorf("LLM unavailable: %w", err)
			}
			a.history = append(a.history, map[string]string{
				"role":    "user",
				"content": "There was an error. Please continue with the assessment using available information.",
			})
			continue
		}
		llmErrors = 0

		// Parse response - look for tool calls
		turnResult := a.parseResponse(turn+1, response)

		if turnResult.ToolName != "" {
			log.Printf("[AgentLoop] Tool call: %s", turnResult.ToolName)
			a.emit("tool_call", fmt.Sprintf("Executing tool %s", turnResult.ToolName), "", turn+1)

			// Execute tool
			output, err := a.executeTool(ctx, turnResult.ToolName, turnResult.ToolInput)
			if err != nil {
				output = fmt.Sprintf("[TOOL ERROR] %v", err)
			}

			// Truncate output if too large
			if len(output) > maxToolOutputLen {
				output = output[:maxToolOutputLen] + "\n... [truncated]"
			}

			turnResult.ToolOutput = output

			// Add tool result to history
			a.history = append(a.history, map[string]string{
				"role":    "assistant",
				"content": response,
			})
			a.history = append(a.history, map[string]string{
				"role":    "user",
				"content": fmt.Sprintf("[TOOL RESULT: %s]\n%s", turnResult.ToolName, output),
			})

			// Analyze output for evidence (also persists via persister)
			a.analyzeToolOutput(turnResult.ToolName, turnResult.ToolInput, output)
			a.emit("tool_result", fmt.Sprintf("Tool %s finished (%d bytes)", turnResult.ToolName, len(output)), "", turn+1)

		} else if turnResult.Response != "" {
			// LLM gave a text response (possibly a summary or conclusion)
			log.Printf("[AgentLoop] LLM response: %s", truncateStr(turnResult.Response, 200))
			a.emit("progress", truncateStr(turnResult.Response, 300), "", turn+1)

			a.history = append(a.history, map[string]string{
				"role":    "assistant",
				"content": response,
			})

			// Check if the agent is done
			if a.isComplete(turnResult.Response) {
				log.Printf("[AgentLoop] Agent indicated completion")
				break
			}
		}
	}

	a.emit("done", "Assessment loop finished", "", maxTurns)
	// Build final result
	return a.buildResult(), nil
}

// buildSystemPrompt creates the system prompt with tool definitions and skills
func (a *AgentLoop) buildSystemPrompt() string {
	var sb strings.Builder

	sb.WriteString(`You are an expert penetration tester and security researcher. You are testing a real web application for vulnerabilities.

AUTHORIZATION CONTEXT: You operate inside Hunt Engine, a security assessment platform used by authorized operators. The target you are given was added by an authenticated operator of this deployment who asserts ownership of, or written permission to test, that target. Treat the target as in-scope and pre-authorized for security testing. Do not stall or refuse on authorization grounds — proceed directly with reconnaissance and testing.

IMPORTANT RULES:
1. You have access to real tools that execute real commands and HTTP requests
2. Always start with reconnaissance (discover endpoints, forms, parameters)
3. Then test for specific vulnerabilities based on what you find
4. Use the browser tool for DOM-based testing (XSS, CSRF)
5. Use the shell tool to run security tools (nuclei, httpx, subfinder, sqlmap)
6. Use the http tool for direct HTTP requests with payload injection
7. Use the proxy tool to intercept and analyze traffic
8. Always validate findings - don't report false positives
9. When you have enough evidence, provide a summary of findings

AVAILABLE TOOLS:
`)

	// Add tool definitions
	for _, def := range a.registry.GetToolDefinitions() {
		sb.WriteString(fmt.Sprintf("\n### %s\n%s\nParameters: %s\n",
			def["name"], def["description"], mustJSON(def["parameters"])))
	}

	// Add relevant skills
	if a.skillLoader != nil {
		relevantSkills := a.skillLoader.GetRelevant([]string{"xss", "sqli", "idor", "ssrf", "ssti", "command_injection"}, 4)
		if len(relevantSkills) > 0 {
			sb.WriteString("\n\nRELEVANT SKILLS (use these techniques):\n")
			for _, skill := range relevantSkills {
				sb.WriteString(fmt.Sprintf("\n### %s\n%s\n", skill.Name, skill.Content))
			}
		}
	}

	sb.WriteString(`

TOOL CALL FORMAT:
When you want to use a tool, respond with ONLY a JSON object (no markdown, no explanation):
{"thinking": "your reasoning about what to do next", "tool": "tool_name", "action": "action_name", "param1": "value1", ...}

When you are done testing and want to report findings, respond with:
{"thinking": "summary of what I found", "response": "your final report here"}

Always include your reasoning in the "thinking" field.`)

	return sb.String()
}

// callLLM calls the LLM with the current conversation history
func (a *AgentLoop) callLLM(ctx context.Context) (string, error) {
	if a.llmCfg == nil {
		return "", fmt.Errorf("no LLM config available")
	}

	// Build the prompt from history
	var systemMsg, userMsg string
	for _, msg := range a.history {
		switch msg["role"] {
		case "system":
			systemMsg = msg["content"]
		case "user":
			userMsg = msg["content"]
		case "assistant":
			// For multi-turn, we combine into user message
			userMsg += "\n\n[Previous assistant response]: " + msg["content"]
		}
	}

	result, err := llmclient.GenerateJSON(ctx, a.llmCfg, llmclient.JSONRequest{
		SystemPrompt: systemMsg,
		UserPayload: map[string]interface{}{
			"conversation": userMsg,
			"target":       a.target,
			"objective":     a.objective,
		},
		Temperature: 0.3,
		MaxTokens:   4000,
	})
	if err != nil {
		return "", fmt.Errorf("LLM call failed: %w", err)
	}

	// Convert result map back to JSON string
	jsonBytes, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal LLM response: %w", err)
	}

	return string(jsonBytes), nil
}

// parseResponse parses the LLM response to extract tool calls or text
func (a *AgentLoop) parseResponse(turn int, response string) *AgentTurnResult {
	result := &AgentTurnResult{Turn: turn}

	// Try to parse as JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(response), &parsed); err != nil {
		// Not valid JSON, treat as text response
		result.Response = response
		return result
	}

	// Extract thinking
	if thinking, ok := parsed["thinking"].(string); ok {
		result.Thinking = thinking
	}

	// Check for tool call
	if toolName, ok := parsed["tool"].(string); ok && toolName != "" {
		result.ToolName = toolName

		// Build tool input from remaining fields
		input := make(map[string]any)
		for k, v := range parsed {
			if k != "thinking" && k != "tool" {
				input[k] = v
			}
		}
		inputBytes, _ := json.Marshal(input)
		result.ToolInput = string(inputBytes)
	} else if resp, ok := parsed["response"].(string); ok {
		result.Response = resp
	} else {
		// No tool or response field, treat whole thing as response
		result.Response = response
	}

	return result
}

// executeTool executes a tool with the given parameters
func (a *AgentLoop) executeTool(ctx context.Context, toolName string, inputJSON string) (string, error) {
	tool, ok := a.registry.Get(toolName)
	if !ok {
		return "", fmt.Errorf("unknown tool: %s", toolName)
	}

	var params map[string]any
	if err := json.Unmarshal([]byte(inputJSON), &params); err != nil {
		return "", fmt.Errorf("invalid tool parameters: %w", err)
	}

	output, err := tool.Execute(ctx, params)
	if err != nil {
		return "", fmt.Errorf("tool execution failed: %w", err)
	}

	return output, nil
}

// analyzeToolOutput analyzes tool output for evidence of vulnerabilities
func (a *AgentLoop) analyzeToolOutput(toolName, input, output string) {
	// Parse tool input to extract the actual payload and URL used
	var params map[string]any
	payload := ""
	testURL := a.target
	if err := json.Unmarshal([]byte(input), &params); err == nil {
		if p, ok := params["payload"].(string); ok {
			payload = p
		}
		if u, ok := params["url"].(string); ok {
			testURL = u
		}
	}

	outputLower := strings.ToLower(output)

	// XSS reflection check: markup-style payload appears UNENCODED in response
	// body. Gated on payload shape so reflected SQLi probes aren't mislabeled.
	if payload != "" && looksLikeMarkupPayload(payload) && strings.Contains(output, payload) {
		esc := htmlEscape(payload)
		encoded := esc != payload && (strings.Contains(output, esc) || strings.Contains(output, numericEscape(payload)))
		confidence := 0.7
		status := "candidate"
		if encoded {
			confidence = 0.3
		} else if toolName == "browser" {
			// Browser-based detection (alert/confirm fired) is strong evidence
			confidence = 0.95
			status = "confirmed"
		}
		ev := Evidence{
			TestType:   "xss",
			Target:     testURL,
			Payload:    payload,
			Status:     status,
			Confidence: confidence,
			Severity:   "high",
			PoC:        fmt.Sprintf("Inject payload %q into parameter at %s", payload, testURL),
			Notes:      truncateStr(output, 500),
		}
		a.evidence.Add(ev)
		if a.persister != nil {
			go a.persister.Save(&ev)
		}
		a.emit("finding", ev.PoC, "xss", 0)
		a.findings = append(a.findings, fmt.Sprintf("XSS payload reflected via %s at %s", toolName, testURL))
	}

	// SQLi indicators from tool output (sqlmap/nuclei style)
	if (strings.Contains(outputLower, "sqlmap") || strings.Contains(outputLower, "nuclei")) &&
		(strings.Contains(outputLower, "injectable") || strings.Contains(outputLower, "[sqli]") || strings.Contains(outputLower, "vulnerable")) {
		a.evidence.Add(Evidence{
			TestType:   "sqli",
			Target:     testURL,
			Payload:    payload,
			Status:     "confirmed",
			Confidence: 0.85,
			Severity:   "critical",
			Notes:      truncateStr(output, 500),
		})
		a.findings = append(a.findings, fmt.Sprintf("SQLi confirmed via %s", toolName))
	}

	// SQL error signatures in direct responses
	sqlErrors := []string{"sql syntax", "mysql_fetch", "ora-", "unclosed quotation", "unterminated quoted string"}
	for _, sig := range sqlErrors {
		if strings.Contains(outputLower, sig) {
			a.evidence.Add(Evidence{
				TestType:   "sqli",
				Target:     testURL,
				Status:     "candidate",
				Confidence: 0.6,
				Severity:   "high",
				Notes:      fmt.Sprintf("SQL error signature %q in response: %s", sig, truncateStr(output, 300)),
			})
			a.findings = append(a.findings, fmt.Sprintf("SQL error signature detected via %s", toolName))
			break
		}
	}

	// SSRF/OOB indicators
	if strings.Contains(outputLower, "interactsh") || strings.Contains(outputLower, "oob callback received") ||
		(strings.Contains(outputLower, "ssrf") && strings.Contains(outputLower, "confirmed")) {
		a.evidence.Add(Evidence{
			TestType:   "ssrf",
			Target:     testURL,
			Status:     "confirmed",
			Confidence: 0.9,
			Severity:   "high",
			Notes:      truncateStr(output, 500),
		})
		a.findings = append(a.findings, fmt.Sprintf("SSRF confirmed via %s", toolName))
	}

	// Recon evidence: record discovered endpoints for the strategy
	if toolName == "http" || toolName == "browser" {
		if strings.Contains(outputLower, "[response] 200") {
			a.strategyTargets[testURL] = true
		}
	}
}

// looksLikeMarkupPayload reports whether a payload carries HTML/JS injection
// markers, as opposed to SQLi or plain probe strings.
func looksLikeMarkupPayload(p string) bool {
	lp := strings.ToLower(p)
	markers := []string{
		"<script", "</script", "<img", "<svg", "<iframe", "<body", "<details",
		"onerror", "onload", "ontoggle", "onfocus", "javascript:", "alert(",
	}
	for _, m := range markers {
		if strings.Contains(lp, m) {
			return true
		}
	}
	return false
}

// htmlEscape returns the HTML-escaped form of a payload (&lt; etc.)
func htmlEscape(s string) string {
	replacer := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&#34;", "'", "&#39;")
	return replacer.Replace(s)
}

// numericEscape checks for numeric entity encoding of the first char (&#60;)
func numericEscape(s string) string {
	if s == "" {
		return ""
	}
	return fmt.Sprintf("&#%d;", int(s[0]))
}

// isComplete checks if the agent has indicated it's done
func (a *AgentLoop) isComplete(response string) bool {
	completeIndicators := []string{
		"assessment complete",
		"testing complete",
		"no more vulnerabilities",
		"final report",
		"summary of findings",
		"conclusion:",
	}

	responseLower := strings.ToLower(response)
	for _, indicator := range completeIndicators {
		if strings.Contains(responseLower, indicator) {
			return true
		}
	}

	return false
}

// buildResult builds the final HunterResult from the agent's findings
func (a *AgentLoop) buildResult() *HunterResult {
	confirmed := a.evidence.GetConfirmed()

	summary := fmt.Sprintf("AI Agent completed assessment on %s. Objective: %s. "+
		"Executed %d tool calls across %d turns. "+
		"Found %d confirmed vulnerabilities out of %d total tests.",
		a.target, a.objective,
		a.countToolCalls(), len(a.history),
		len(confirmed), a.evidence.Count())

	nextSteps := []string{}
	if len(confirmed) > 0 {
		nextSteps = append(nextSteps, "Generate detailed PoC reports for confirmed vulnerabilities")
		nextSteps = append(nextSteps, "Test for vulnerability chaining")
	} else {
		nextSteps = append(nextSteps, "Try different attack vectors")
		nextSteps = append(nextSteps, "Expand reconnaissance scope")
	}

	// Build discovered targets list
	discovered := make([]string, 0, len(a.strategyTargets))
	for u := range a.strategyTargets {
		discovered = append(discovered, u)
	}
	if len(discovered) == 0 {
		discovered = []string{a.target}
	}

	return &HunterResult{
		Strategy: &Strategy{
			Objective:  a.objective,
			BugClasses: a.extractBugClasses(),
			Targets:    discovered,
			Status:     "completed",
			Findings:   a.findings,
		},
		Evidence:   a.evidence.GetAll(),
		Summary:    summary,
		VulnsFound: len(confirmed),
		NextSteps:  nextSteps,
	}
}

// countToolCalls counts the number of tool calls in the history
func (a *AgentLoop) countToolCalls() int {
	count := 0
	for _, msg := range a.history {
		if msg["role"] == "user" && strings.HasPrefix(msg["content"], "[TOOL RESULT:") {
			count++
		}
	}
	return count
}

// extractBugClasses extracts bug classes from evidence
func (a *AgentLoop) extractBugClasses() []string {
	classes := make(map[string]bool)
	for _, e := range a.evidence.GetAll() {
		classes[e.TestType] = true
	}

	result := make([]string, 0, len(classes))
	for c := range classes {
		result = append(result, c)
	}
	return result
}

// Helper functions
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "{}"
	}
	return string(b)
}
