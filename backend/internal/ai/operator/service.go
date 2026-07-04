package operator

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
)

func GeneratePlan(ctx context.Context, req PlanRequest) (*Plan, error) {
	cfg, err := llmclient.ResolveDefault(req.OwnerKey)
	if err != nil {
		return nil, err
	}
	return GeneratePlanWithConfig(ctx, cfg, req)
}

var endpointURLPattern = regexp.MustCompile(`https?://[^\s"'<>(){}[\]]+`)

func firstEndpointURLFromText(text string) string {
	raw := endpointURLPattern.FindString(text)
	if raw == "" {
		return ""
	}
	return strings.TrimRight(raw, ".,;:!?)].\"'")
}

func endpointIntentRequestsSingleAction(text string) bool {
	normalized := strings.ToLower(strings.TrimSpace(text))
	return strings.Contains(normalized, "فقط") ||
		strings.Contains(normalized, "only") ||
		strings.Contains(normalized, "just create") ||
		strings.Contains(normalized, "single action")
}

func ensureEndpointReviewAction(plan *Plan, userMessage string) {
	if plan == nil {
		return
	}

	url := firstEndpointURLFromText(userMessage)
	if url == "" {
		return
	}

	endpointAction := ActionProposal{
		ActionType:       models.AgentActionTypeReviewEndpoint,
		Title:            "Review endpoint with controlled HTTP probe",
		Description:      "Review the user-specified endpoint through the approval-gated controlled runtime.",
		Reason:           "The user provided a concrete endpoint URL, so the next operator step should preserve that URL and route it through the controlled runtime approval flow.",
		RiskLevel:        models.AgentActionRiskLow,
		SafetyLevel:      1,
		TestLevel:        1,
		RequiresApproval: true,
		InputJSON: map[string]interface{}{
			"url":                      url,
			"method":                   "GET",
			"source":                   "operator_endpoint_intent",
			"objective":                "Review user-specified endpoint through controlled runtime",
			"endpoint_intent_detected": true,
			"operator_prompt_version":  PromptVersion,
		},
	}

	found := false
	for i := range plan.Actions {
		if strings.TrimSpace(plan.Actions[i].ActionType) != models.AgentActionTypeReviewEndpoint {
			continue
		}

		found = true
		if plan.Actions[i].InputJSON == nil {
			plan.Actions[i].InputJSON = map[string]interface{}{}
		}
		if strings.TrimSpace(fmt.Sprint(plan.Actions[i].InputJSON["url"])) == "" {
			plan.Actions[i].InputJSON["url"] = url
		}
		if strings.TrimSpace(fmt.Sprint(plan.Actions[i].InputJSON["method"])) == "" {
			plan.Actions[i].InputJSON["method"] = "GET"
		}
		plan.Actions[i].InputJSON["endpoint_intent_detected"] = true
		plan.Actions[i].InputJSON["operator_prompt_version"] = PromptVersion

		if plan.Actions[i].RiskLevel == "" {
			plan.Actions[i].RiskLevel = models.AgentActionRiskLow
		}
		if plan.Actions[i].SafetyLevel <= 0 {
			plan.Actions[i].SafetyLevel = 1
		}
		if plan.Actions[i].TestLevel <= 0 {
			plan.Actions[i].TestLevel = 1
		}
		plan.Actions[i].RequiresApproval = true
		endpointAction = plan.Actions[i]
		break
	}

	if endpointIntentRequestsSingleAction(userMessage) {
		plan.Actions = []ActionProposal{endpointAction}
		return
	}

	if !found {
		plan.Actions = append([]ActionProposal{endpointAction}, plan.Actions...)
	}
}

func wantsPentestHypothesisMode(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}

	keywords := []string{
		"real bug",
		"real bugs",
		"critical bug",
		"critical bugs",
		"high impact",
		"high-value",
		"high value",
		"reportable",
		"pentest",
		"hunt everything",
		"all bug classes",
		"sql injection",
		"sqli",
		"command injection",
		"rce",
		"ssrf",
		"idor",
		"xss",
		"crlf",
		"cache poisoning",
		"cache deception",
		"auth bypass",
		"path traversal",
		"file upload",
		"باگ واقعی",
		"باگ های واقعی",
		"باگ‌های واقعی",
		"کریتیکال",
		"حیاتی",
		"مهم",
		"همه کلاس",
		"همه باگ",
		"نفوذ",
	}
	for _, keyword := range keywords {
		if strings.Contains(t, keyword) {
			return true
		}
	}
	return false
}

func enforcePentestHypotheses(plan *Plan, req PlanRequest, facts []string) {
	if plan == nil || !wantsPentestHypothesisMode(req.UserMessage) {
		return
	}
	if len(plan.Hypotheses) > 0 {
		return
	}

	evidence := firstStrings(facts, 6)
	if len(evidence) == 0 {
		evidence = []string{
			"target context is available but sparse",
			"operator should request more recon, URLs, JS/API data, and authenticated context before claiming vulnerabilities",
		}
	}

	plan.Hypotheses = []PentestHypothesis{
		{
			Title:                    "Potential API authorization / IDOR surface needs authenticated validation",
			BugClass:                 "api_authorization",
			ImpactPotential:          "high",
			Confidence:               "low",
			Evidence:                 evidence,
			WhyThisMightBeReal:       "High-impact access-control bugs usually require comparing authorized behavior across users, accounts, organizations, or roles. Current evidence is not enough to confirm or dismiss this class.",
			MissingEvidence:          []string{"authenticated session", "second test account", "account/org/user identifier endpoints", "baseline authorized and unauthorized responses"},
			RequiredContext:          []string{"authorized test account", "second account or role", "target policy approval for authenticated checks"},
			SafeNextTest:             "Ask for authenticated test context and enumerate account/API endpoints before validation.",
			ControlledValidationPath: "Create approval-gated IDOR/API authorization validation actions once authenticated context is available.",
			RiskLevel:                "medium",
			TestLevel:                2,
			RequiresApproval:         true,
		},
		{
			Title:                    "Potential injection surfaces require parameter and response-context analysis",
			BugClass:                 "sqli",
			ImpactPotential:          "critical",
			Confidence:               "low",
			Evidence:                 evidence,
			WhyThisMightBeReal:       "Critical injection classes such as SQLi/NoSQLi/command injection/RCE depend on parameters, backend behavior, error/timing differences, or execution sinks. Current evidence should be used to identify candidate parameters before any controlled probe.",
			MissingEvidence:          []string{"parameter inventory", "request/response baselines", "technology hints", "safe error/timing evidence", "approval for level-2/3 validation"},
			RequiredContext:          []string{"in-scope authorization", "rate limits", "explicit approval before active injection validation"},
			SafeNextTest:             "Build a high-value parameter inventory from URLs, forms, APIs, and JS-discovered endpoints.",
			ControlledValidationPath: "Propose safe SQLi/NoSQLi/command-injection strategy actions; execute only through controlled runtime after approval.",
			RiskLevel:                "high",
			TestLevel:                2,
			RequiresApproval:         true,
		},
		{
			Title:                    "Potential SSRF / server-side fetch sinks need URL-like parameter discovery",
			BugClass:                 "ssrf",
			ImpactPotential:          "critical",
			Confidence:               "low",
			Evidence:                 evidence,
			WhyThisMightBeReal:       "SSRF becomes plausible when endpoints accept URL, callback, webhook, image, import, fetch, proxy, or redirect-like inputs. The operator should search target evidence for such sinks instead of relying on generic checks.",
			MissingEvidence:          []string{"url-like parameters", "webhook/fetch/import endpoints", "server-side request behavior", "safe callback/OOB approval"},
			RequiredContext:          []string{"explicit approval for SSRF validation", "safe callback infrastructure policy", "rate limits"},
			SafeNextTest:             "Identify URL-like parameters and server-side fetch candidates from crawled URLs and JS/API endpoints.",
			ControlledValidationPath: "Create approval-gated SSRF candidate validation plan; no uncontrolled callbacks or internal network probing.",
			RiskLevel:                "high",
			TestLevel:                3,
			RequiresApproval:         true,
		},
		{
			Title:                    "Potential XSS/CRLF/cache bugs require reflection, header, and cache-behavior proof",
			BugClass:                 "xss",
			ImpactPotential:          "medium",
			Confidence:               "low",
			Evidence:                 evidence,
			WhyThisMightBeReal:       "Client-side and HTTP-layer bugs depend on reflection context, header injection behavior, cacheability, and browser-visible impact. Current evidence should drive targeted context probes.",
			MissingEvidence:          []string{"reflection proof", "HTML/JS/header context", "cache hit/miss evidence", "cache-control behavior", "browser validation if needed"},
			RequiredContext:          []string{"controlled reflection probes", "approval for active cache/header validation where required"},
			SafeNextTest:             "Select endpoints with query parameters and cacheable responses for controlled reflection/cacheability review.",
			ControlledValidationPath: "Propose xss_reflection_probe, crlf_header_probe, cache_poisoning_probe, or cache_deception_probe actions by evidence class.",
			RiskLevel:                "medium",
			TestLevel:                2,
			RequiresApproval:         true,
		},
		{
			Title:                    "Potential file read / path traversal / upload abuse requires file-like endpoint evidence",
			BugClass:                 "path_traversal",
			ImpactPotential:          "high",
			Confidence:               "low",
			Evidence:                 evidence,
			WhyThisMightBeReal:       "File read and upload chains become plausible around download, export, import, upload, file, path, template, image, or attachment endpoints. The operator should identify these surfaces first.",
			MissingEvidence:          []string{"file-like parameters or routes", "upload/download endpoints", "baseline response behavior", "approval for controlled non-destructive validation"},
			RequiredContext:          []string{"in-scope endpoint confirmation", "explicit approval before path/file validation"},
			SafeNextTest:             "Search crawled URLs and JS-discovered APIs for file, upload, export, import, download, and attachment surfaces.",
			ControlledValidationPath: "Create approval-gated path traversal/file-read/file-upload validation plans without destructive writes.",
			RiskLevel:                "high",
			TestLevel:                2,
			RequiresApproval:         true,
		},
	}

	if strings.TrimSpace(plan.ResponseSummary) == "" || isGenericOperatorText(plan.ResponseSummary) {
		plan.ResponseSummary = "I switched into AI Pentest Hypothesis mode and generated evidence-grounded hypotheses across high-impact bug classes. These are not confirmed vulnerabilities yet; each requires controlled validation, missing context, and policy/approval checks."
	}

	plan.RecommendedNextSteps = append([]string{
		"Build a high-value endpoint and parameter inventory from URLs, JS/API discoveries, controlled runtime evidence, and memory.",
		"Request authenticated context and a second test account before attempting IDOR/API authorization/business-logic validation.",
		"Convert the strongest high/critical hypotheses into approval-gated controlled validation actions.",
	}, plan.RecommendedNextSteps...)

	plan.Guardrails = append(plan.Guardrails,
		"critical/high-impact hypotheses are not vulnerability claims until controlled evidence validates them",
		"SQLi, command injection, RCE, SSRF, auth bypass, and exploit validation require explicit approval and controlled runtime execution",
	)
}

func GeneratePlanWithConfig(ctx context.Context, cfg *llmclient.Config, req PlanRequest) (*Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config is required")
	}
	req.UserMessage = strings.TrimSpace(req.UserMessage)
	if req.UserMessage == "" {
		return nil, fmt.Errorf("user message is required")
	}

	contextFacts := buildContextFacts(req.TargetContext, req.MemoryContext)

	payload := map[string]interface{}{
		"task":                    "Create a safe, approval-gated pentest operator plan for this target chat request.",
		"context_facts_required":  contextFacts,
		"operator_prompt_version": PromptVersion,
		"user_message":            req.UserMessage,
		"pentest_hypothesis_mode": wantsPentestHypothesisMode(req.UserMessage),
		"required_min_hypotheses": 5,
		"target": map[string]interface{}{
			"target_id": req.TargetID,
			"user_id":   req.UserID,
			"owner_key": req.OwnerKey,
		},
		"target_context": req.TargetContext,
		"memory_context": req.MemoryContext,
		"allowed_action_types": []string{
			models.AgentActionTypeRunCrawling,
			models.AgentActionTypeRunNucleiProfile,
			models.AgentActionTypeGenerateNucleiDraft,
			models.AgentActionTypeRunJSIntelligence,
			models.AgentActionTypeRunSafeBugTests,
			models.AgentActionTypeReviewBugTestResults,
			models.AgentActionTypePromoteBugTestResults,
			models.AgentActionTypeInspectBugPatterns,
			models.AgentActionTypeInspectBugPayloads,
			models.AgentActionTypeDeepScanAsset,
			models.AgentActionTypeReviewEndpoint,
			models.AgentActionTypeGeneratePayload,
			models.AgentActionTypeRunOWASPChecklist,
			models.AgentActionTypeValidateFinding,
			models.AgentActionTypeProposeSeverityChange,
			models.AgentActionTypeGenerateReport,
		},
	}

	raw, err := llmclient.GenerateJSON(ctx, cfg, llmclient.JSONRequest{
		SystemPrompt:    SystemPrompt(),
		UserPayload:     payload,
		Temperature:     0.2,
		MaxTokens:       1400,
		ResponseVersion: PromptVersion,
	})
	if err != nil {
		return nil, err
	}

	plan := sanitizePlan(raw)
	plan.LLMAssisted = true
	plan.LLMProvider = cfg.Provider
	plan.LLMModel = cfg.Model
	plan.OperatorPromptVersion = PromptVersion

	groundPlan(plan, contextFacts)
	enforcePentestHypotheses(plan, req, contextFacts)
	ensureEndpointReviewAction(plan, req.UserMessage)

	if plan.ResponseSummary == "" {
		plan.ResponseSummary = "I prepared an approval-gated operator plan from the available target context and memory."
	}
	if len(plan.Guardrails) == 0 {
		plan.Guardrails = defaultGuardrails()
	}

	return plan, nil
}

func sanitizePlan(raw map[string]interface{}) *Plan {
	plan := &Plan{
		Mode:       cleanString(raw["mode"]),
		Guardrails: cleanStringSlice(raw["guardrails"]),
	}
	if plan.Mode == "" {
		plan.Mode = "llm_operator_plan"
	}

	plan.ResponseSummary = cleanString(raw["response_summary"])
	plan.ClarifyingQuestions = cleanStringSlice(raw["clarifying_questions"])
	plan.ReasoningSummary = cleanString(raw["reasoning_summary"])
	plan.RecommendedNextSteps = cleanStringSlice(raw["recommended_next_steps"])
	plan.MemoryNotes = cleanStringSlice(raw["memory_notes"])

	if items, ok := raw["actions"].([]interface{}); ok {
		for _, item := range items {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			action := ActionProposal{
				ActionType:       cleanString(obj["action_type"]),
				Title:            cleanString(obj["title"]),
				Description:      cleanString(obj["description"]),
				RiskLevel:        cleanString(obj["risk_level"]),
				SafetyLevel:      cleanInt(obj["safety_level"]),
				TestLevel:        cleanInt(obj["test_level"]),
				RequiresApproval: true,
				InputJSON:        cleanMap(obj["input_json"]),
				Reason:           cleanString(obj["reason"]),
			}
			if action.ActionType == "" || action.Title == "" {
				continue
			}
			if !isAllowedOperatorActionType(action.ActionType) {
				continue
			}
			if action.RiskLevel == "" {
				action.RiskLevel = models.AgentActionRiskLow
			}
			if action.InputJSON == nil {
				action.InputJSON = map[string]interface{}{}
			}
			action.InputJSON["source"] = "llm_operator"
			action.InputJSON["operator_prompt_version"] = PromptVersion
			plan.Actions = append(plan.Actions, action)
		}
	}

	return plan
}

func isAllowedOperatorActionType(actionType string) bool {
	switch strings.TrimSpace(actionType) {
	case models.AgentActionTypeRunCrawling,
		models.AgentActionTypeRunNucleiProfile,
		models.AgentActionTypeGenerateNucleiDraft,
		models.AgentActionTypeRunJSIntelligence,
		models.AgentActionTypeRunSafeBugTests,
		models.AgentActionTypeReviewBugTestResults,
		models.AgentActionTypePromoteBugTestResults,
		models.AgentActionTypeInspectBugPatterns,
		models.AgentActionTypeInspectBugPayloads,
		models.AgentActionTypeDeepScanAsset,
		models.AgentActionTypeReviewEndpoint,
		models.AgentActionTypeGeneratePayload,
		models.AgentActionTypeRunOWASPChecklist,
		models.AgentActionTypeValidateFinding,
		models.AgentActionTypeProposeSeverityChange,
		models.AgentActionTypeGenerateReport:
		return true
	default:
		return false
	}
}

func buildContextFacts(targetContext map[string]interface{}, memoryContext map[string]interface{}) []string {
	facts := []string{}

	add := func(label string, value interface{}) {
		text := cleanString(value)
		if text != "" {
			facts = append(facts, fmt.Sprintf("%s=%s", label, text))
		}
	}

	add("target_name", targetContext["target_name"])
	add("root_domain", targetContext["root_domain"])
	add("asset_count", targetContext["asset_count"])
	add("live_asset_count", targetContext["live_asset_count"])
	add("url_count", targetContext["url_count"])
	add("finding_count", targetContext["finding_count"])
	add("active_finding_count", targetContext["active_finding_count"])
	add("platform_name", targetContext["platform_name"])
	add("max_test_intensity", targetContext["max_test_intensity"])
	add("auth_required", targetContext["auth_required"])

	if count := cleanString(memoryContext["memory_count"]); count != "" {
		facts = append(facts, "memory_count="+count)
	}

	if evidence, ok := memoryContext["controlled_runtime_evidence"].([]map[string]interface{}); ok {
		for _, item := range evidence {
			title := cleanString(item["title"])
			summary := cleanString(item["summary"])
			content := cleanString(item["content"])
			fact := strings.TrimSpace(title + ": " + summary)
			if fact == "" {
				fact = content
			}
			if fact != "" {
				if len([]rune(fact)) > 260 {
					fact = string([]rune(fact)[:260])
				}
				facts = append(facts, "controlled_runtime_evidence="+fact)
			}
			if len(facts) >= 18 {
				break
			}
		}
	} else if raw, ok := memoryContext["controlled_runtime_evidence"].([]interface{}); ok {
		for _, item := range raw {
			memory, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			title := cleanString(memory["title"])
			summary := cleanString(memory["summary"])
			content := cleanString(memory["content"])
			fact := strings.TrimSpace(title + ": " + summary)
			if fact == "" {
				fact = content
			}
			if fact != "" {
				if len([]rune(fact)) > 260 {
					fact = string([]rune(fact)[:260])
				}
				facts = append(facts, "controlled_runtime_evidence="+fact)
			}
			if len(facts) >= 18 {
				break
			}
		}
	}

	if memories, ok := memoryContext["memories"].([]map[string]interface{}); ok {
		for _, memory := range memories {
			title := cleanString(memory["title"])
			summary := cleanString(memory["summary"])
			if title != "" || summary != "" {
				fact := strings.TrimSpace(title + ": " + summary)
				if len([]rune(fact)) > 220 {
					fact = string([]rune(fact)[:220])
				}
				facts = append(facts, fact)
			}
			if len(facts) >= 14 {
				break
			}
		}
	} else if raw, ok := memoryContext["memories"].([]interface{}); ok {
		for _, item := range raw {
			memory, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			title := cleanString(memory["title"])
			summary := cleanString(memory["summary"])
			if title != "" || summary != "" {
				fact := strings.TrimSpace(title + ": " + summary)
				if len([]rune(fact)) > 220 {
					fact = string([]rune(fact)[:220])
				}
				facts = append(facts, fact)
			}
			if len(facts) >= 14 {
				break
			}
		}
	}

	return facts
}

func hasControlledRuntimeBlockedEvidence(facts []string) bool {
	for _, fact := range facts {
		text := strings.ToLower(fact)
		if !strings.Contains(text, "controlled_runtime_evidence") {
			continue
		}
		if strings.Contains(text, "inconclusive") ||
			strings.Contains(text, "blocked") ||
			strings.Contains(text, "challenged") ||
			strings.Contains(text, "cloudflare") ||
			strings.Contains(text, "waf") ||
			strings.Contains(text, "403") ||
			strings.Contains(text, "429") {
			return true
		}
	}
	return false
}

func groundPlan(plan *Plan, facts []string) {
	if plan == nil {
		return
	}

	if len(facts) > 0 && isGenericOperatorText(plan.ResponseSummary) {
		plan.ResponseSummary = "Using current target memory (" + strings.Join(firstStrings(facts, 5), ", ") + "), I prepared an approval-gated operator plan focused on the most relevant OWASP and evidence-review work."
	}

	if hasControlledRuntimeBlockedEvidence(facts) {
		if isGenericOperatorText(plan.ResponseSummary) || !strings.Contains(strings.ToLower(plan.ResponseSummary), "controlled") {
			plan.ResponseSummary = "The latest controlled runtime evidence shows an inconclusive or blocked response, not confirmed vulnerability proof. I will use that observation to propose the next controlled validation step."
		}
		if len(plan.RecommendedNextSteps) == 0 {
			plan.RecommendedNextSteps = []string{
				"Treat the challenged or blocked runtime result as inconclusive evidence rather than a confirmed vulnerability.",
				"Review alternate live endpoints and collected URLs for less-protected validation targets.",
				"Consider browser-aware or authenticated validation only if target policy and user approval allow it.",
				"Keep the next action approval-gated and backed by captured evidence.",
			}
		}
	}

	if len(plan.RecommendedNextSteps) == 0 && len(facts) > 0 {
		plan.RecommendedNextSteps = []string{
			"Review stored findings, bug test results, and controlled runtime evidence before running new checks.",
			"Prioritize endpoints and live assets from the current memory context.",
			"Keep testing within the target policy and approval workflow.",
		}
	}

	if len(plan.Guardrails) == 0 || (len(plan.Guardrails) == 1 && strings.EqualFold(plan.Guardrails[0], "guardrails applied")) {
		plan.Guardrails = defaultGuardrails()
	}

	for i := range plan.Actions {
		if plan.Actions[i].InputJSON == nil {
			plan.Actions[i].InputJSON = map[string]interface{}{}
		}
		if _, ok := plan.Actions[i].InputJSON["evidence_basis"]; !ok && len(facts) > 0 {
			plan.Actions[i].InputJSON["evidence_basis"] = firstStrings(facts, 6)
		}
		if plan.Actions[i].Reason == "" && len(facts) > 0 {
			plan.Actions[i].Reason = "Selected from current target context: " + strings.Join(firstStrings(facts, 3), ", ")
		}
	}
}

func isGenericOperatorText(value string) bool {
	text := strings.ToLower(strings.TrimSpace(value))
	if text == "" {
		return true
	}

	generic := []string{
		"i will create a prioritized owasp test plan",
		"create a prioritized owasp test plan",
		"identify owasp top 10 vulnerabilities",
		"propose safe actions",
		"prepared an approval-gated operator plan",
	}
	for _, item := range generic {
		if strings.Contains(text, item) {
			return true
		}
	}
	return false
}

func firstStrings(items []string, limit int) []string {
	if limit <= 0 || len(items) == 0 {
		return nil
	}
	out := make([]string, 0, limit)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
		if len(out) >= limit {
			break
		}
	}
	return out
}

func defaultGuardrails() []string {
	return []string{
		"target-scoped and owner-scoped context only",
		"chat responses do not execute tests directly",
		"authorized controlled validation/exploitation must use approval-gated Agent Actions and audited runtime execution",
		"active/risky actions require scope checks, policy checks, rate limits, and explicit approval",
		"destructive, out-of-scope, unauthorized brute-force, credential stuffing, password spraying, DoS-like testing, data-exfiltration, persistence, malware, credential theft, and uncontrolled payload execution remain blocked",
	}
}

func cleanString(value interface{}) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func cleanStringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := cleanString(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func cleanMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	if obj, ok := value.(map[string]interface{}); ok {
		return obj
	}
	return map[string]interface{}{}
}

func cleanInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
