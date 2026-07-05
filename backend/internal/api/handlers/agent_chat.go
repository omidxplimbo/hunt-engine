package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/ai/operator"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type createAgentChatSessionRequest struct {
	Title string `json:"title"`
}

type createAgentChatMessageRequest struct {
	Content string `json:"content"`
}

type plannedChatAction struct {
	Request proposeAgentActionRequest `json:"request"`
	Reason  string                    `json:"reason"`
}

func ensureAgentChatEnabled(c *fiber.Ctx) error {
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}
	if !featureflags.IsEnabledForOwner(ownerKey, featureflags.KeyAgentChat) {
		return featureflags.DisabledResponse(c, featureflags.KeyAgentChat)
	}
	return nil
}

func parseAgentChatSessionIDParam(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("session_id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid chat session id")
	}
	return uint(id), nil
}

func chatJSON(value interface{}) datatypes.JSON {
	if value == nil {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}

func shortChatTitle(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return "Attack Surface Chat"
	}
	content = strings.Join(strings.Fields(content), " ")
	if len(content) > 80 {
		return content[:80]
	}
	return content
}

func buildAgentChatContext(target models.Target) map[string]interface{} {
	var assetCount int64
	var liveAssetCount int64
	var urlCount int64
	var findingCount int64
	var activeFindingCount int64

	_ = database.DB.Model(&models.Asset{}).
		Where("target_id = ?", target.ID).
		Count(&assetCount).Error

	_ = database.DB.Model(&models.Asset{}).
		Where("target_id = ? AND is_live = ?", target.ID, true).
		Count(&liveAssetCount).Error

	_ = database.DB.Model(&models.FoundURL{}).
		Where("target_id = ?", target.ID).
		Count(&urlCount).Error

	_ = database.DB.Model(&models.Finding{}).
		Where("target_id = ?", target.ID).
		Count(&findingCount).Error

	_ = database.DB.Model(&models.Finding{}).
		Where("target_id = ? AND status NOT IN ?", target.ID, []string{"fixed", "false_positive"}).
		Count(&activeFindingCount).Error

	var policy models.TargetPolicy
	hasPolicy := database.DB.Where("target_id = ?", target.ID).First(&policy).Error == nil

	ctx := map[string]interface{}{
		"target_id":             target.ID,
		"root_domain":           target.RootDomain,
		"target_name":           target.Name,
		"asset_count":           assetCount,
		"live_asset_count":      liveAssetCount,
		"url_count":             urlCount,
		"finding_count":         findingCount,
		"active_finding_count":  activeFindingCount,
		"has_target_policy":     hasPolicy,
		"guardrail_mode":        "agent-chat-v1-foundation",
		"execution_enabled":     false,
		"action_execution_path": "agent_actions approval workflow",
	}

	if hasPolicy {
		ctx["platform_name"] = policy.PlatformName
		ctx["program_url"] = policy.ProgramURL
		ctx["max_test_intensity"] = policy.MaxTestIntensity
		ctx["auth_required"] = policy.AuthRequired
		ctx["safe_testing_notes"] = policy.SafeTestingNotes
		ctx["reporting_preferences"] = policy.ReportingPreferences
	}

	return ctx
}

func planActionsFromChat(content string) ([]plannedChatAction, string) {
	text := strings.ToLower(strings.TrimSpace(content))
	actions := make([]plannedChatAction, 0)

	add := func(req proposeAgentActionRequest, reason string) {
		req.ActionType = normalizeAgentActionType(req.ActionType)
		if req.ActionType == "" {
			return
		}
		if req.Title == "" {
			req.Title = req.ActionType
		}
		if req.RiskLevel == "" {
			req.RiskLevel = models.AgentActionRiskLow
		}
		req.RequestedByAgent = boolPtr(true)
		req.RequiresApproval = boolPtr(true)
		actions = append(actions, plannedChatAction{Request: req, Reason: reason})
	}

	if strings.Contains(text, "owasp") || strings.Contains(text, "wstg") || strings.Contains(text, "asvs") || strings.Contains(text, "checklist") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeRunOWASPChecklist,
			Title:       "Run OWASP passive coverage checklist",
			Description: "Create an OWASP/WSTG-style passive coverage checklist based on existing assets, URLs, findings, evidence, target policy, and scan history.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"standards": []string{"OWASP", "WSTG", "ASVS", "API Security"},
				"mode":      "passive",
			},
		}, "OWASP/standard coverage request detected")
	}

	if strings.Contains(text, "crawl") || strings.Contains(text, "url") || strings.Contains(text, "endpoint") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeRunCrawling,
			Title:       "Run or re-run crawling workflow",
			Description: "Improve URL and endpoint coverage before deeper manual or automated testing.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"reason": "chat requested URL/endpoint/crawling coverage",
			},
		}, "crawling or endpoint coverage request detected")
	}

	if strings.Contains(text, "nuclei") || strings.Contains(text, "template") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeRunNucleiProfile,
			Title:       "Run safe Nuclei profile",
			Description: "Run a safe/profile-based Nuclei workflow after approval.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 1,
			TestLevel:   1,
			InputJSON: map[string]interface{}{
				"profile": "safe",
				"source":  "chat_agent",
			},
		}, "Nuclei/template request detected")
	}

	if strings.Contains(text, "javascript") || strings.Contains(text, " js ") || strings.Contains(text, "js intel") || strings.Contains(text, "source map") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeRunJSIntelligence,
			Title:       "Run JavaScript intelligence review",
			Description: "Analyze JavaScript assets, source maps, endpoints, and client-side security signals.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"focus": []string{"js-endpoints", "source-maps", "client-side-secrets", "dom-sinks"},
			},
		}, "JavaScript intelligence request detected")
	}

	if strings.Contains(text, "review bug") || strings.Contains(text, "bug result") || strings.Contains(text, "نتایج باگ") || strings.Contains(text, "بررسی نتیجه") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeReviewBugTestResults,
			Title:       "Review safe bug test results",
			Description: "Review Safe Bug Testing results, summarize candidate/validated/manual-validation items, and identify what should be manually validated next.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"source":            "agent_chat",
				"review_scope":      "bug_test_results",
				"active_testing":    false,
				"payload_execution": false,
			},
		}, "bug test result review request detected")
	}

	if strings.Contains(text, "promote") || strings.Contains(text, "finding کن") || strings.Contains(text, "تبدیل به finding") || strings.Contains(text, "فایندینگ کن") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypePromoteBugTestResults,
			Title:       "Review bug test candidates for finding promotion",
			Description: "Identify validated/manual-validation Safe Bug Testing candidates that may be promoted to Findings. Bulk promotion remains guarded and review-only in this bridge step.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"source":              "agent_chat",
				"promotion_mode":      "review_only_foundation",
				"auto_severity_apply": false,
			},
		}, "finding promotion request detected")
	}

	if strings.Contains(text, "pattern registry") || strings.Contains(text, "patternها") || strings.Contains(text, "پترن") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeInspectBugPatterns,
			Title:       "Inspect bug pattern registry",
			Description: "Inspect enabled/safe bug testing patterns and summarize registry coverage. This is metadata-only.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"source":        "agent_chat",
				"metadata_only": true,
			},
		}, "pattern registry inspection request detected")
	}

	if strings.Contains(text, "payload registry") || strings.Contains(text, "payloadها") || strings.Contains(text, "پیلود") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeInspectBugPayloads,
			Title:       "Inspect bug payload registry",
			Description: "Inspect payload registry metadata and safety classes. Payload execution remains disabled.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"source":            "agent_chat",
				"metadata_only":     true,
				"payload_execution": false,
			},
		}, "payload registry inspection request detected")
	}

	if strings.Contains(text, "safe header") || strings.Contains(text, "security header") || strings.Contains(text, "headers safely") || strings.Contains(text, "header check") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeRunSafeBugTests,
			Title:       "Run safe security header checks",
			Description: "Run level-1 safe active security header checks against live HTTP assets. No payloads, exploitation, brute force, or destructive testing are performed.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 1,
			TestLevel:   1,
			InputJSON: map[string]interface{}{
				"bug_types":                       []string{"security_headers"},
				"execution_enabled":               false,
				"test_profile":                    "safe",
				"safe_active_security_headers_v1": true,
				"active_testing":                  true,
				"payload_execution":               false,
			},
		}, "safe security header check request detected")
	}

	if strings.Contains(text, "xss") || strings.Contains(text, "cors") || strings.Contains(text, "redirect") || ((strings.Contains(text, "bug test") || strings.Contains(text, "safe bug test")) && !strings.Contains(text, "bug test result") && !strings.Contains(text, "bug test results")) {
		bugTypes := []string{}
		if strings.Contains(text, "xss") {
			bugTypes = append(bugTypes, "xss")
		}
		if strings.Contains(text, "cors") {
			bugTypes = append(bugTypes, "cors")
		}
		if strings.Contains(text, "redirect") {
			bugTypes = append(bugTypes, "open_redirect")
		}
		if strings.Contains(text, "header") {
			bugTypes = append(bugTypes, "security_headers")
		}
		if len(bugTypes) == 0 {
			bugTypes = append(bugTypes, "safe_bug_tests")
		}

		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeRunSafeBugTests,
			Title:       "Prepare safe bug test workflow",
			Description: "Prepare safe, approval-gated bug testing for requested bug classes. Execution will be added through the Safe Bug Testing Engine.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 1,
			TestLevel:   1,
			InputJSON: map[string]interface{}{
				"bug_types":         bugTypes,
				"execution_enabled": false,
				"test_profile":      "safe",
			},
		}, "safe bug testing request detected")
	}

	if (strings.Contains(text, "payload") && !strings.Contains(text, "payload registry")) || strings.Contains(text, "exploit") || strings.Contains(text, "validate") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeGeneratePayload,
			Title:       "Prepare controlled payload generation plan",
			Description: "Prepare a policy-aware payload generation plan. Payload execution remains approval-gated and requires controlled runtime support.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 2,
			TestLevel:   2,
			InputJSON: map[string]interface{}{
				"execution_enabled": false,
				"requires_policy":   true,
				"requires_approval": true,
			},
		}, "payload/exploit/validation request detected")
	}

	if strings.Contains(text, "report") || strings.Contains(text, "گزارش") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeGenerateReport,
			Title:       "Generate report draft from validated evidence",
			Description: "Generate a human-validation-required report draft based on stored findings and evidence.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"auto_submit": false,
			},
		}, "report generation request detected")
	}

	if strings.Contains(text, "severity") || strings.Contains(text, "شدت") || strings.Contains(text, "critical") || strings.Contains(text, "high") {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeProposeSeverityChange,
			Title:       "Review finding severity changes",
			Description: "Review whether stored evidence supports deterministic severity/status changes. Auto-apply is not enabled by default.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"auto_apply": false,
			},
		}, "severity/status review request detected")
	}

	if len(actions) == 0 {
		add(proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeReviewEndpoint,
			Title:       "Review target context and propose next steps",
			Description: "Review current target data and propose a safe next-step action plan.",
			RiskLevel:   models.AgentActionRiskLow,
			SafetyLevel: 0,
			TestLevel:   0,
			InputJSON: map[string]interface{}{
				"source": "fallback_chat_intent",
			},
		}, "fallback planning action")
	}

	response := "I converted your request into approval-gated agent action proposals. Safe metadata workflows and approved Safe Bug Testing actions can be dispatched through the controlled Agent Actions workflow; payload execution, exploit validation, destructive testing, and out-of-scope testing remain blocked."
	return actions, response
}

func buildAgentChatMemoryContext(targetID uint, ownerKey string) map[string]interface{} {
	items := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND owner_key = ?", targetID, ownerKey).
		Order("importance desc, updated_at desc").
		Limit(8).
		Find(&items).Error

	memories := make([]map[string]interface{}, 0, len(items))
	itemIDs := make([]uint, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
		memories = append(memories, map[string]interface{}{
			"id":          item.ID,
			"memory_type": item.MemoryType,
			"source_type": item.SourceType,
			"title":       item.Title,
			"summary":     item.Summary,
			"tags":        item.Tags,
			"importance":  item.Importance,
			"confidence":  item.Confidence,
			"updated_at":  item.UpdatedAt,
		})
	}

	controlledRuntimeItems := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND owner_key = ? AND source_type = ?", targetID, ownerKey, models.TargetMemorySourceControlledTestResult).
		Order("updated_at desc, importance desc").
		Limit(6).
		Find(&controlledRuntimeItems).Error

	controlledRuntimeEvidence := make([]map[string]interface{}, 0, len(controlledRuntimeItems))
	for _, item := range controlledRuntimeItems {
		text := strings.TrimSpace(item.Summary)
		if text == "" {
			text = strings.TrimSpace(item.Content)
		}
		if len([]rune(text)) > 700 {
			text = string([]rune(text)[:700]) + "..."
		}
		controlledRuntimeEvidence = append(controlledRuntimeEvidence, map[string]interface{}{
			"id":          item.ID,
			"memory_type": item.MemoryType,
			"source_type": item.SourceType,
			"source_id":   item.SourceID,
			"title":       item.Title,
			"summary":     item.Summary,
			"content":     text,
			"tags":        item.Tags,
			"importance":  item.Importance,
			"confidence":  item.Confidence,
			"metadata":    item.Metadata,
			"updated_at":  item.UpdatedAt,
		})
	}

	chunks := make([]models.TargetMemoryChunk, 0)
	if len(itemIDs) > 0 {
		_ = database.DB.
			Where("target_id = ? AND owner_key = ? AND memory_item_id IN ?", targetID, ownerKey, itemIDs).
			Order("token_count asc, updated_at desc").
			Limit(10).
			Find(&chunks).Error
	}

	chunkOut := make([]map[string]interface{}, 0, len(chunks))
	for _, chunk := range chunks {
		text := strings.TrimSpace(chunk.ChunkText)
		if len(text) > 900 {
			text = text[:900]
		}
		chunkOut = append(chunkOut, map[string]interface{}{
			"id":             chunk.ID,
			"memory_item_id": chunk.MemoryItemID,
			"chunk_index":    chunk.ChunkIndex,
			"chunk_text":     text,
			"token_count":    chunk.TokenCount,
		})
	}

	return map[string]interface{}{
		"context_version":             "agent-chat-memory-context-v1",
		"memory_count":                len(memories),
		"memories":                    memories,
		"controlled_runtime_evidence": controlledRuntimeEvidence,
		"controlled_runtime_count":    len(controlledRuntimeEvidence),
		"chunks":                      chunkOut,
	}
}

func chatWantsPentestHypothesisMode(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}
	keywords := []string{
		"real bug", "real bugs", "critical bug", "critical bugs",
		"high impact", "high-value", "high value", "reportable",
		"pentest", "hunt everything", "all bug classes",
		"sqli", "sql injection", "command injection", "rce", "ssrf",
		"idor", "xss", "crlf", "cache poisoning", "cache deception",
		"auth bypass", "path traversal", "file upload",
		"باگ واقعی", "باگ های واقعی", "باگ‌های واقعی",
		"کریتیکال", "حیاتی", "مهم", "همه کلاس", "همه باگ", "نفوذ",
	}
	for _, keyword := range keywords {
		if strings.Contains(t, keyword) {
			return true
		}
	}
	return false
}

func replaceLowValueActionsForHypothesisMode(content string, actions []plannedChatAction, plan *operator.Plan) []plannedChatAction {
	if !chatWantsPentestHypothesisMode(content) && (plan == nil || len(plan.Hypotheses) == 0) {
		return actions
	}

	filtered := make([]plannedChatAction, 0, len(actions))
	for _, action := range actions {
		if normalizeAgentActionType(action.Request.ActionType) == models.AgentActionTypeRunSafeBugTests {
			continue
		}
		filtered = append(filtered, action)
	}

	strategy := plannedChatAction{
		Request: proposeAgentActionRequest{
			ActionType:  models.AgentActionTypeGeneratePayload,
			Title:       "Build controlled validation strategy for high-impact hypotheses",
			Description: "Prepare a policy-aware controlled validation strategy for high-impact bug hypotheses across injection, SSRF, access-control, cache, XSS/CRLF, and file/path classes. This is plan-only; payload execution remains disabled until explicit approval and controlled runtime support.",
			RiskLevel:   models.AgentActionRiskMedium,
			SafetyLevel: 2,
			TestLevel:   2,
			InputJSON: map[string]interface{}{
				"source":                  "pentest_hypothesis_agent_v1",
				"execution_enabled":       false,
				"payload_execution":       false,
				"requires_policy":         true,
				"requires_approval":       true,
				"hypothesis_driven":       true,
				"objective":               "turn high-impact hypotheses into controlled validation plans",
				"blocked_from_autopilot":  true,
				"validation_only_by_plan": true,
			},
			RequestedByAgent: boolPtr(true),
			RequiresApproval: boolPtr(true),
		},
		Reason: "Critical/high-impact hunting mode should produce controlled validation strategy, not another low-value safe bug test run.",
	}

	return append([]plannedChatAction{strategy}, filtered...)
}

func convertOperatorPlanActions(plan *operator.Plan) []plannedChatAction {
	if plan == nil {
		return nil
	}

	out := make([]plannedChatAction, 0, len(plan.Actions))
	for _, action := range plan.Actions {
		input := action.InputJSON
		if input == nil {
			input = map[string]interface{}{}
		}
		input["llm_assisted"] = true
		input["operator_prompt_version"] = plan.OperatorPromptVersion

		out = append(out, plannedChatAction{
			Request: proposeAgentActionRequest{
				ActionType:       action.ActionType,
				Title:            action.Title,
				Description:      action.Description,
				RiskLevel:        action.RiskLevel,
				SafetyLevel:      action.SafetyLevel,
				TestLevel:        action.TestLevel,
				InputJSON:        input,
				RequestedByAgent: boolPtr(true),
				RequiresApproval: boolPtr(true),
			},
			Reason: action.Reason,
		})
	}

	return out
}

func firstNonEmptyStrings(items []string, limit int) []string {
	if limit <= 0 {
		return nil
	}
	out := make([]string, 0, limit)
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		out = append(out, item)
		if len(out) >= limit {
			break
		}
	}
	return out
}

func buildOperatorAssistantText(plan *operator.Plan) string {
	if plan == nil {
		return ""
	}

	parts := []string{}
	if strings.TrimSpace(plan.ResponseSummary) != "" {
		parts = append(parts, strings.TrimSpace(plan.ResponseSummary))
	}
	if len(plan.ClarifyingQuestions) > 0 {
		parts = append(parts, "Clarifying questions:")
		for _, q := range plan.ClarifyingQuestions {
			if q = strings.TrimSpace(q); q != "" {
				parts = append(parts, "- "+q)
			}
		}
	}
	if len(plan.RecommendedNextSteps) > 0 {
		parts = append(parts, "Recommended next steps:")
		for _, step := range plan.RecommendedNextSteps {
			if step = strings.TrimSpace(step); step != "" {
				parts = append(parts, "- "+step)
			}
		}
	}
	if len(plan.Hypotheses) > 0 {
		parts = append(parts, "Pentest hypotheses:")
		for _, h := range plan.Hypotheses {
			title := strings.TrimSpace(h.Title)
			if title == "" {
				title = strings.TrimSpace(h.BugClass)
			}
			if title == "" {
				title = "Untitled hypothesis"
			}

			line := "- " + title
			if strings.TrimSpace(h.BugClass) != "" {
				line += " [" + strings.TrimSpace(h.BugClass) + "]"
			}
			if strings.TrimSpace(h.ImpactPotential) != "" {
				line += " impact=" + strings.TrimSpace(h.ImpactPotential)
			}
			if strings.TrimSpace(h.Confidence) != "" {
				line += " confidence=" + strings.TrimSpace(h.Confidence)
			}
			parts = append(parts, line)

			if strings.TrimSpace(h.WhyThisMightBeReal) != "" {
				parts = append(parts, "  why: "+strings.TrimSpace(h.WhyThisMightBeReal))
			}
			if strings.TrimSpace(h.SafeNextTest) != "" {
				parts = append(parts, "  next: "+strings.TrimSpace(h.SafeNextTest))
			}
			if len(h.MissingEvidence) > 0 {
				parts = append(parts, "  missing: "+strings.Join(firstNonEmptyStrings(h.MissingEvidence, 4), ", "))
			}
		}
	}

	if len(plan.Actions) > 0 {
		parts = append(parts, fmt.Sprintf("I proposed %d approval-gated operator action(s). Review and approve before dispatch.", len(plan.Actions)))
	}

	if len(parts) == 0 {
		return "I prepared an approval-gated operator plan from the available target context and memory."
	}
	return strings.Join(parts, "\n")
}

func boolPtr(value bool) *bool {
	return &value
}

func chatActionInputMap(raw datatypes.JSON) map[string]interface{} {
	out := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func uintFromAny(value interface{}) uint {
	switch x := value.(type) {
	case uint:
		return x
	case int:
		return uint(x)
	case int64:
		return uint(x)
	case float64:
		return uint(x)
	case json.Number:
		n, _ := x.Int64()
		return uint(n)
	default:
		return 0
	}
}

func appendUniqueUintInterface(items []interface{}, id uint) []interface{} {
	for _, item := range items {
		if uintFromAny(item) == id {
			return items
		}
	}
	return append(items, id)
}

func uintListFromInterface(value interface{}) []interface{} {
	if items, ok := value.([]interface{}); ok {
		return items
	}
	return []interface{}{}
}

func markDuplicateActionReusedByChat(c *fiber.Ctx, target *models.Target, uid uint, ownerKey string, duplicate *models.AgentAction, req proposeAgentActionRequest) (*models.AgentAction, error) {
	if duplicate == nil {
		return nil, nil
	}

	input := chatActionInputMap(duplicate.InputJSON)
	if input == nil {
		input = map[string]interface{}{}
	}

	chatSessionID := uintFromAny(req.InputJSON["chat_session_id"])
	userMessageID := uintFromAny(req.InputJSON["user_message_id"])

	input["reused_by_chat"] = true
	input["last_reused_by_chat"] = map[string]interface{}{
		"chat_session_id": chatSessionID,
		"user_message_id": userMessageID,
		"owner_key":       ownerKey,
		"source":          "agent_chat_duplicate_reuse",
	}

	if chatSessionID > 0 {
		input["last_reused_chat_session_id"] = chatSessionID
		input["related_chat_session_ids"] = appendUniqueUintInterface(uintListFromInterface(input["related_chat_session_ids"]), chatSessionID)
	}
	if userMessageID > 0 {
		input["last_reused_user_message_id"] = userMessageID
		input["related_user_message_ids"] = appendUniqueUintInterface(uintListFromInterface(input["related_user_message_ids"]), userMessageID)
	}

	for k, v := range req.InputJSON {
		if _, exists := input[k]; !exists {
			input[k] = v
		}
	}

	if err := database.DB.Model(&models.AgentAction{}).
		Where("id = ? AND target_id = ? AND owner_key = ?", duplicate.ID, target.ID, ownerKey).
		Update("input_json", agentActionJSON(input)).Error; err != nil {
		return nil, err
	}

	var updated models.AgentAction
	if err := database.DB.Where("id = ? AND target_id = ? AND owner_key = ?", duplicate.ID, target.ID, ownerKey).First(&updated).Error; err != nil {
		return duplicate, nil
	}

	return &updated, nil
}

func actionReusedForCurrentChat(action models.AgentAction, sessionID uint, messageID uint) bool {
	input := chatActionInputMap(action.InputJSON)
	reused, _ := input["reused_by_chat"].(bool)
	if !reused {
		return false
	}

	last, ok := input["last_reused_by_chat"].(map[string]interface{})
	if !ok {
		return false
	}

	return uintFromAny(last["chat_session_id"]) == sessionID && uintFromAny(last["user_message_id"]) == messageID
}

func shouldReuseChatActionByType(actionType string) bool {
	switch normalizeAgentActionType(actionType) {
	case models.AgentActionTypeRunCrawling,
		models.AgentActionTypeReviewBugTestResults,
		models.AgentActionTypeInspectBugPatterns,
		models.AgentActionTypeInspectBugPayloads,
		models.AgentActionTypePromoteBugTestResults,
		models.AgentActionTypeRunOWASPChecklist,
		models.AgentActionTypeReviewEndpoint,
		models.AgentActionTypeRunJSIntelligence,
		models.AgentActionTypeRunSafeBugTests:
		return true
	default:
		return false
	}
}

type chatAutopilotResult struct {
	Enabled           bool                     `json:"enabled"`
	Mode              string                   `json:"mode"`
	PolicySource      string                   `json:"policy_source"`
	AutoExecuteLevel0 bool                     `json:"auto_execute_level_0"`
	AutoExecuteLevel1 bool                     `json:"auto_execute_level_1"`
	ExecutedActions   []uint                   `json:"executed_actions"`
	SkippedActions    []map[string]interface{} `json:"skipped_actions"`
	ControlledRuns    []uint                   `json:"controlled_runs"`
	ControlledResults []uint                   `json:"controlled_results"`
	MemoryIngested    bool                     `json:"memory_ingested"`
	Errors            []string                 `json:"errors"`
	Summary           []map[string]interface{} `json:"summary"`
}

type chatAutopilotPolicy struct {
	OperatorMode          string `json:"operator_mode"`
	PolicySource          string `json:"policy_source"`
	AutoExecuteLevel0     bool   `json:"auto_execute_level_0"`
	AutoExecuteLevel1     bool   `json:"auto_execute_level_1"`
	RequireApprovalLevel2 bool   `json:"require_approval_level_2"`
	RequireApprovalLevel3 bool   `json:"require_approval_level_3"`
}

func loadChatAutopilotPolicy(targetID uint) chatAutopilotPolicy {
	policy := chatAutopilotPolicy{
		OperatorMode:          "strict_approval",
		PolicySource:          "missing_target_policy",
		AutoExecuteLevel0:     false,
		AutoExecuteLevel1:     false,
		RequireApprovalLevel2: true,
		RequireApprovalLevel3: true,
	}

	var row models.TargetPolicy
	if err := database.DB.Where("target_id = ?", targetID).First(&row).Error; err != nil {
		return policy
	}

	mode := strings.ToLower(strings.TrimSpace(row.OperatorMode))
	switch mode {
	case "manual_only", "assisted_autopilot", "strict_approval":
		policy.OperatorMode = mode
	default:
		policy.OperatorMode = "strict_approval"
	}

	policy.PolicySource = "target_policy"
	policy.AutoExecuteLevel0 = row.AutoExecuteLevel0
	policy.AutoExecuteLevel1 = row.AutoExecuteLevel1
	policy.RequireApprovalLevel2 = row.RequireApprovalLevel2
	policy.RequireApprovalLevel3 = row.RequireApprovalLevel3

	return policy
}

func isChatAutopilotAllowedAction(action models.AgentAction, policy chatAutopilotPolicy) (bool, string) {
	if policy.OperatorMode == "manual_only" {
		return false, "operator_mode=manual_only disables autopilot"
	}
	if policy.OperatorMode == "strict_approval" {
		return false, "operator_mode=strict_approval requires explicit approval"
	}
	if policy.OperatorMode != "assisted_autopilot" {
		return false, "operator mode does not allow autopilot"
	}
	if action.TestLevel <= 0 && !policy.AutoExecuteLevel0 {
		return false, "auto_execute_level_0 is disabled by target policy"
	}
	if action.TestLevel == 1 && !policy.AutoExecuteLevel1 {
		return false, "auto_execute_level_1 is disabled by target policy"
	}
	if action.TestLevel >= 2 && policy.RequireApprovalLevel2 {
		return false, "level 2+ actions require explicit approval"
	}
	if action.TestLevel >= 3 && policy.RequireApprovalLevel3 {
		return false, "level 3 actions require explicit approval"
	}

	actionType := normalizeAgentActionType(action.ActionType)
	switch actionType {
	case models.AgentActionTypeRunCrawling, models.AgentActionTypeRunJSIntelligence, models.AgentActionTypeRunSafeBugTests, models.AgentActionTypeReviewEndpoint:
		// supported below
	default:
		return false, "autopilot currently supports run_crawling, run_js_intelligence, run_safe_bug_tests, and review_endpoint"
	}

	if action.Status == models.AgentActionStatusBlockedByPolicy || action.PolicyStatus == models.AgentActionPolicyStatusBlocked {
		return false, "action is blocked by policy"
	}
	if strings.EqualFold(action.RiskLevel, models.AgentActionRiskHigh) || strings.EqualFold(action.RiskLevel, models.AgentActionRiskCritical) {
		return false, "high/critical risk requires explicit approval"
	}
	if action.SafetyLevel > 1 || action.TestLevel > 1 || action.AutonomyLevel > 1 {
		return false, "action level exceeds autopilot limits"
	}

	if actionType == models.AgentActionTypeReviewEndpoint {
		input := chatActionInputMap(action.InputJSON)
		if strings.TrimSpace(controlledCleanString(input["url"])) == "" {
			return false, "review_endpoint action has no input_json.url"
		}
	}

	return true, ""
}

func updateChatActionOutput(action *models.AgentAction, output map[string]interface{}, errorMessage string, status string, executedAt *time.Time, completedAt *time.Time) error {
	if action == nil {
		return nil
	}

	updates := map[string]interface{}{
		"output_json":   agentActionJSON(output),
		"error_message": strings.TrimSpace(errorMessage),
	}
	if strings.TrimSpace(status) != "" {
		updates["status"] = status
	}
	if executedAt != nil {
		updates["executed_at"] = executedAt
	}
	if completedAt != nil {
		updates["completed_at"] = completedAt
	}

	if err := database.DB.Model(&models.AgentAction{}).
		Where("id = ? AND target_id = ? AND owner_key = ?", action.ID, action.TargetID, action.OwnerKey).
		Updates(updates).Error; err != nil {
		return err
	}

	_ = database.DB.Where("id = ? AND target_id = ? AND owner_key = ?", action.ID, action.TargetID, action.OwnerKey).First(action).Error
	return nil
}

func uintFromInterface(value interface{}) uint {
	switch v := value.(type) {
	case uint:
		return v
	case int:
		if v > 0 {
			return uint(v)
		}
	case int64:
		if v > 0 {
			return uint(v)
		}
	case float64:
		if v > 0 {
			return uint(v)
		}
	case json.Number:
		i, _ := v.Int64()
		if i > 0 {
			return uint(i)
		}
	case string:
		n, err := strconv.ParseUint(strings.TrimSpace(v), 10, 64)
		if err == nil && n > 0 {
			return uint(n)
		}
	}
	return 0
}

func stringSliceFromInterface(value interface{}) []string {
	out := make([]string, 0)
	switch v := value.(type) {
	case []string:
		out = append(out, v...)
	case []interface{}:
		for _, item := range v {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
	case string:
		if strings.TrimSpace(v) != "" {
			out = append(out, strings.TrimSpace(v))
		}
	}
	return out
}

func headerValuesFromMap(headers map[string]interface{}, name string) []string {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return nil
	}
	for key, value := range headers {
		if strings.ToLower(strings.TrimSpace(key)) == name {
			return stringSliceFromInterface(value)
		}
	}
	return nil
}

func hasHeader(headers map[string]interface{}, name string) bool {
	return len(headerValuesFromMap(headers, name)) > 0
}

func joinedHeader(headers map[string]interface{}, name string) string {
	return strings.ToLower(strings.Join(headerValuesFromMap(headers, name), " "))
}

func analyzeSecurityHeadersFromRuntime(runtimeResult *models.ControlledTestResult) map[string]interface{} {
	analysis := map[string]interface{}{
		"analyzer":  "security-headers-analyzer-v1",
		"available": false,
	}

	if runtimeResult == nil {
		analysis["reason"] = "runtime result is missing"
		return analysis
	}

	response := map[string]interface{}{}
	if len(runtimeResult.ResponseJSON) > 0 {
		_ = json.Unmarshal(runtimeResult.ResponseJSON, &response)
	}

	headers := map[string]interface{}{}
	if raw, ok := response["headers"].(map[string]interface{}); ok {
		headers = raw
	}
	if len(headers) == 0 {
		analysis["reason"] = "runtime response headers are missing"
		return analysis
	}

	missing := make([]string, 0)
	weak := make([]string, 0)
	present := make([]string, 0)

	if hasHeader(headers, "strict-transport-security") {
		present = append(present, "strict-transport-security")
		hsts := joinedHeader(headers, "strict-transport-security")
		if !strings.Contains(hsts, "max-age=") {
			weak = append(weak, "strict-transport-security:missing_max_age")
		}
	} else if strings.HasPrefix(strings.ToLower(runtimeResult.URL), "https://") {
		missing = append(missing, "strict-transport-security")
	}

	csp := joinedHeader(headers, "content-security-policy")
	if csp != "" {
		present = append(present, "content-security-policy")
		if strings.Contains(csp, "unsafe-inline") {
			weak = append(weak, "content-security-policy:unsafe-inline")
		}
		if strings.Contains(csp, "unsafe-eval") {
			weak = append(weak, "content-security-policy:unsafe-eval")
		}
	} else {
		missing = append(missing, "content-security-policy")
	}

	xfo := joinedHeader(headers, "x-frame-options")
	hasFrameAncestors := strings.Contains(csp, "frame-ancestors")
	if xfo != "" {
		present = append(present, "x-frame-options")
	}
	if xfo == "" && !hasFrameAncestors {
		missing = append(missing, "x-frame-options-or-csp-frame-ancestors")
	}

	xcto := joinedHeader(headers, "x-content-type-options")
	if xcto != "" {
		present = append(present, "x-content-type-options")
		if !strings.Contains(xcto, "nosniff") {
			weak = append(weak, "x-content-type-options:not_nosniff")
		}
	} else {
		missing = append(missing, "x-content-type-options")
	}

	if hasHeader(headers, "referrer-policy") {
		present = append(present, "referrer-policy")
	} else {
		missing = append(missing, "referrer-policy")
	}

	if hasHeader(headers, "permissions-policy") {
		present = append(present, "permissions-policy")
	} else {
		missing = append(missing, "permissions-policy")
	}

	status := "passed"
	severity := "info"
	confidence := "medium"
	if len(missing) > 0 || len(weak) > 0 {
		status = "validated"
		severity = "low"
		confidence = "medium"
	}

	analysis["available"] = true
	analysis["status"] = status
	analysis["severity_hint"] = severity
	analysis["confidence"] = confidence
	analysis["present"] = present
	analysis["missing"] = missing
	analysis["weak"] = weak
	analysis["status_code"] = response["status_code"]
	analysis["final_url"] = response["final_url"]
	analysis["headers_seen"] = len(headers)
	analysis["operator_note"] = "security headers were analyzed from controlled runtime response headers"

	return analysis
}

func linkBugTestResultToControlledRuntime(targetID uint, bugResultID uint, runID uint, controlledResultID uint, output map[string]interface{}, runtimeResult *models.ControlledTestResult) map[string]interface{} {
	link := map[string]interface{}{
		"attempted":                 true,
		"bug_test_result_id":        bugResultID,
		"controlled_test_run_id":    runID,
		"controlled_test_result_id": controlledResultID,
	}

	var bugResult models.BugTestResult
	if err := database.DB.Where("id = ? AND target_id = ?", bugResultID, targetID).First(&bugResult).Error; err != nil {
		link["linked"] = false
		link["error"] = err.Error()
		return link
	}

	evidence := map[string]interface{}{}
	if len(bugResult.EvidenceJSON) > 0 {
		_ = json.Unmarshal(bugResult.EvidenceJSON, &evidence)
	}

	resultStatus := ""
	resultConfidence := ""
	resultSeverity := ""
	if runtimeResult != nil {
		resultStatus = runtimeResult.Status
		resultConfidence = runtimeResult.Confidence
		resultSeverity = runtimeResult.SeverityHint
	}

	controlledValidation := map[string]interface{}{
		"attempted":                 true,
		"linked":                    true,
		"controlled_test_run_id":    runID,
		"controlled_test_result_id": controlledResultID,
		"runtime_result_status":     resultStatus,
		"runtime_confidence":        resultConfidence,
		"runtime_severity_hint":     resultSeverity,
		"status_code":               output["status_code"],
		"final_url":                 output["final_url"],
		"classification":            output["classification"],
		"duration_ms":               output["duration_ms"],
		"memory_ingested":           output["memory_ingested"],
		"operator_note":             "controlled runtime evidence collected; candidate still requires bug-specific validation before promotion",
		"updated_at":                time.Now().UTC().Format(time.RFC3339),
	}

	evidence["controlled_validation"] = controlledValidation

	if bugResult.BugType == models.BugTypeSecurityHeaders {
		securityHeaderAnalysis := analyzeSecurityHeadersFromRuntime(runtimeResult)
		evidence["security_header_analysis"] = securityHeaderAnalysis
		if available, _ := securityHeaderAnalysis["available"].(bool); available {
			link["security_header_analysis"] = securityHeaderAnalysis
		}
	}

	newStatus := bugResult.Status
	if runtimeResult != nil {
		switch runtimeResult.Status {
		case models.ControlledTestResultStatusBlocked:
			newStatus = models.BugTestResultStatusBlocked
		case models.ControlledTestResultStatusInconclusive, models.ControlledTestResultStatusNeedsManualReview, models.ControlledTestResultStatusFailed:
			newStatus = models.BugTestResultStatusInconclusive
		default:
			if newStatus == models.BugTestResultStatusCandidate || newStatus == models.BugTestResultStatusNeedsManualValidation {
				newStatus = models.BugTestResultStatusNeedsManualValidation
			}
		}
	}

	updates := map[string]interface{}{
		"evidence_json": bugTestJSON(evidence),
		"status":        newStatus,
	}

	if sha, ok := evidence["security_header_analysis"].(map[string]interface{}); ok {
		if available, _ := sha["available"].(bool); available {
			if shaStatus := strings.TrimSpace(controlledCleanString(sha["status"])); shaStatus == "validated" {
				updates["status"] = models.BugTestResultStatusValidated
			} else if shaStatus == "passed" {
				updates["status"] = models.BugTestResultStatusPassed
			}
			if c := strings.TrimSpace(controlledCleanString(sha["confidence"])); c != "" {
				updates["confidence"] = c
			}
			if sev := strings.TrimSpace(controlledCleanString(sha["severity_hint"])); sev != "" {
				updates["severity_hint"] = sev
			}
		}
	}

	if resultConfidence != "" && strings.EqualFold(bugResult.Confidence, "low") {
		if _, alreadySet := updates["confidence"]; !alreadySet {
			updates["confidence"] = resultConfidence
		}
	}

	if err := database.DB.Model(&models.BugTestResult{}).
		Where("id = ? AND target_id = ?", bugResult.ID, targetID).
		Updates(updates).Error; err != nil {
		link["linked"] = false
		link["error"] = err.Error()
		return link
	}

	link["linked"] = true
	link["bug_test_status"] = newStatus
	link["runtime_result_status"] = resultStatus
	return link
}

func runAgentChatAutopilotForActions(c *fiber.Ctx, target *models.Target, uid uint, ownerKey string, actions []models.AgentAction) chatAutopilotResult {
	policy := loadChatAutopilotPolicy(target.ID)

	result := chatAutopilotResult{
		Enabled:           policy.OperatorMode == "assisted_autopilot",
		Mode:              policy.OperatorMode,
		PolicySource:      policy.PolicySource,
		AutoExecuteLevel0: policy.AutoExecuteLevel0,
		AutoExecuteLevel1: policy.AutoExecuteLevel1,
		ExecutedActions:   []uint{},
		SkippedActions:    []map[string]interface{}{},
		ControlledRuns:    []uint{},
		ControlledResults: []uint{},
		Errors:            []string{},
		Summary:           []map[string]interface{}{},
	}

	if target == nil || strings.TrimSpace(ownerKey) == "" {
		result.Errors = append(result.Errors, "target or owner scope missing")
		return result
	}

	for _, action := range actions {
		allowed, reason := isChatAutopilotAllowedAction(action, policy)
		if !allowed {
			result.SkippedActions = append(result.SkippedActions, map[string]interface{}{
				"action_id":   action.ID,
				"action_type": action.ActionType,
				"reason":      reason,
			})
			continue
		}

		now := time.Now().UTC()

		if action.Status == models.AgentActionStatusProposed {
			if err := database.DB.Model(&models.AgentAction{}).
				Where("id = ? AND target_id = ? AND owner_key = ?", action.ID, target.ID, ownerKey).
				Updates(map[string]interface{}{
					"status":              models.AgentActionStatusApproved,
					"approved_by_user_id": uid,
					"approved_at":         &now,
					"reject_reason":       "",
				}).Error; err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("auto approve action %d failed: %v", action.ID, err))
				continue
			}

			action.Status = models.AgentActionStatusApproved
			action.ApprovedByUserID = &uid
			action.ApprovedAt = &now

			entityID := action.ID
			_ = auditlog.Record(auditlog.Entry{
				ActorUserID: &uid,
				Action:      "target.agent_action.auto_approve",
				EntityType:  "agent_action",
				EntityID:    &entityID,
				TargetID:    &target.ID,
				IPAddress:   auditlog.ClientIP(c),
				UserAgent:   auditlog.UserAgent(c),
				Metadata: map[string]interface{}{
					"source":      "agent_chat_autopilot",
					"mode":        result.Mode,
					"policy":      result.PolicySource,
					"reason":      "low-risk controlled action auto-approved by target policy",
					"action_type": action.ActionType,
				},
			})
		}

		if action.Status != models.AgentActionStatusApproved {
			result.SkippedActions = append(result.SkippedActions, map[string]interface{}{
				"action_id":   action.ID,
				"action_type": action.ActionType,
				"reason":      "action is not approved after autopilot approval gate",
			})
			continue
		}

		preview := buildDispatchPreview(*target, action, false, "agent chat autopilot dispatch")

		if normalizeAgentActionType(action.ActionType) == models.AgentActionTypeRunSafeBugTests {
			preview := buildDispatchPreview(*target, action, false, "agent chat autopilot safe bug testing dispatch")

			run, resultCount, runErr := CreateBugTestRunFromAgentAction(target, &action, uid)
			if runErr != nil {
				preview["execution_enabled"] = false
				preview["executed"] = false
				preview["hard_blocked"] = false
				preview["reason"] = "agent chat autopilot failed to execute safe bug testing workflow"
				preview["safe_bug_testing"] = map[string]interface{}{
					"attempted": true,
					"error":     runErr.Error(),
				}

				completedAt := time.Now().UTC()
				if err := updateChatActionOutput(&action, preview, runErr.Error(), "", nil, &completedAt); err != nil {
					result.Errors = append(result.Errors, fmt.Sprintf("update action %d safe bug autopilot failure failed: %v", action.ID, err))
				}
				result.Errors = append(result.Errors, fmt.Sprintf("safe bug testing action %d failed: %v", action.ID, runErr))
				continue
			}

			runOutput := map[string]interface{}{}
			if len(run.OutputJSON) > 0 {
				_ = json.Unmarshal(run.OutputJSON, &runOutput)
			}
			activeTesting, _ := runOutput["active_testing"].(bool)
			safeActiveHeaders, _ := runOutput["safe_active_security_headers_v1"].(bool)

			preview["execution_enabled"] = true
			preview["executed"] = true
			preview["hard_blocked"] = false
			preview["reason"] = "agent chat autopilot executed safe bug testing workflow"
			preview["safe_bug_testing"] = map[string]interface{}{
				"attempted":                       true,
				"bug_test_run_id":                 run.ID,
				"bug_test_status":                 run.Status,
				"results_created":                 resultCount,
				"active_testing":                  activeTesting,
				"safe_active_security_headers_v1": safeActiveHeaders,
				"manual_validation":               true,
				"autopilot":                       true,
			}
			preview["autopilot"] = map[string]interface{}{
				"enabled": true,
				"mode":    result.Mode,
				"source":  "agent_chat",
			}

			executedAt := time.Now().UTC()
			completedAt := time.Now().UTC()
			if err := updateChatActionOutput(&action, preview, "", models.AgentActionStatusExecuted, &executedAt, &completedAt); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("update action %d safe bug autopilot output failed: %v", action.ID, err))
				continue
			}

			result.ExecutedActions = append(result.ExecutedActions, action.ID)
			result.Summary = append(result.Summary, map[string]interface{}{
				"action_id":       action.ID,
				"action_type":     action.ActionType,
				"bug_test_run_id": run.ID,
				"results_created": resultCount,
				"status":          run.Status,
				"active_testing":  activeTesting,
				"memory_ingest":   "bug_test_results_available",
			})

			entityID := action.ID
			_ = auditlog.Record(auditlog.Entry{
				ActorUserID: &uid,
				Action:      "target.agent_action.autopilot_execute",
				EntityType:  "agent_action",
				EntityID:    &entityID,
				TargetID:    &target.ID,
				IPAddress:   auditlog.ClientIP(c),
				UserAgent:   auditlog.UserAgent(c),
				Metadata: map[string]interface{}{
					"source":          "agent_chat_autopilot",
					"mode":            result.Mode,
					"action_type":     action.ActionType,
					"bug_test_run_id": run.ID,
					"results_created": resultCount,
				},
			})

			continue
		}

		if normalizeAgentActionType(action.ActionType) == models.AgentActionTypeRunCrawling || normalizeAgentActionType(action.ActionType) == models.AgentActionTypeRunJSIntelligence {
			var bridgeOutput map[string]interface{}
			if normalizeAgentActionType(action.ActionType) == models.AgentActionTypeRunCrawling {
				bridgeOutput = dispatchRunCrawling(*target, action, uid)
			} else {
				bridgeOutput = dispatchRunJSIntelligence(*target, action, uid)
			}
			queued, _ := bridgeOutput["queued"].(bool)
			executed, _ := bridgeOutput["executed"].(bool)
			workflowSucceeded := queued || executed
			bridgeReason, _ := bridgeOutput["reason"].(string)

			preview["execution_enabled"] = workflowSucceeded
			preview["executed"] = workflowSucceeded
			preview["hard_blocked"] = false
			if normalizeAgentActionType(action.ActionType) == models.AgentActionTypeRunCrawling {
				preview["reason"] = "agent chat autopilot queued controlled crawling workflow"
			} else {
				preview["reason"] = "agent chat autopilot executed controlled JS intelligence workflow"
			}
			preview["bridge_output"] = bridgeOutput
			preview["autopilot"] = map[string]interface{}{
				"enabled": true,
				"mode":    result.Mode,
				"source":  "agent_chat",
			}

			errorMessage := ""
			status := ""
			var executedAt *time.Time
			if workflowSucceeded {
				executedAtTime := time.Now().UTC()
				executedAt = &executedAtTime
				status = models.AgentActionStatusExecuted
			} else {
				errorMessage = bridgeReason
				if strings.TrimSpace(errorMessage) == "" {
					errorMessage = "autopilot crawling dispatch did not queue a job"
				}
			}

			completedAt := time.Now().UTC()
			if err := updateChatActionOutput(&action, preview, errorMessage, status, executedAt, &completedAt); err != nil {
				result.Errors = append(result.Errors, fmt.Sprintf("update action %d crawling autopilot output failed: %v", action.ID, err))
				continue
			}

			if workflowSucceeded {
				result.ExecutedActions = append(result.ExecutedActions, action.ID)
				if mem, _ := bridgeOutput["memory_ingested"].(bool); mem {
					result.MemoryIngested = true
				}
				result.Summary = append(result.Summary, map[string]interface{}{
					"action_id":      action.ID,
					"action_type":    action.ActionType,
					"runtime_status": bridgeOutput["runtime_status"],
					"job_payload":    bridgeOutput["job_payload"],
					"queue_name":     bridgeOutput["queue_name"],
					"memory_ingest":  bridgeOutput["memory_ingested"],
				})
			} else {
				result.SkippedActions = append(result.SkippedActions, map[string]interface{}{
					"action_id":   action.ID,
					"action_type": action.ActionType,
					"reason":      errorMessage,
				})
			}

			entityID := action.ID
			_ = auditlog.Record(auditlog.Entry{
				ActorUserID: &uid,
				Action:      "target.agent_action.autopilot_execute",
				EntityType:  "agent_action",
				EntityID:    &entityID,
				TargetID:    &target.ID,
				IPAddress:   auditlog.ClientIP(c),
				UserAgent:   auditlog.UserAgent(c),
				Metadata: map[string]interface{}{
					"source":      "agent_chat_autopilot",
					"mode":        result.Mode,
					"action_type": action.ActionType,
					"queued":      queued,
					"executed":    executed,
					"succeeded":   workflowSucceeded,
					"job_payload": bridgeOutput["job_payload"],
				},
			})

			continue
		}

		run, runErr := CreateControlledTestRunFromAgentAction(target, &action, uid)
		if runErr != nil {
			preview["controlled_runtime"] = map[string]interface{}{
				"attempted": true,
				"error":     runErr.Error(),
			}
			_ = updateChatActionOutput(&action, preview, runErr.Error(), "", nil, &now)
			result.Errors = append(result.Errors, fmt.Sprintf("create controlled run for action %d failed: %v", action.ID, runErr))
			continue
		}

		result.ControlledRuns = append(result.ControlledRuns, run.ID)

		executedAt := time.Now().UTC()
		runAfterExecute, runtimeResult, output, execErr := ExecuteControlledTestRunInternal(
			target.ID,
			run.ID,
			ownerKey,
			uid,
			"agent chat autopilot execute controlled endpoint review",
			8000,
			256*1024,
			"agent_chat_autopilot",
		)

		if output == nil {
			output = map[string]interface{}{}
		}

		controlledRuntime := map[string]interface{}{
			"attempted":              true,
			"controlled_test_run_id": run.ID,
			"runtime_type":           run.RuntimeType,
			"runtime_status":         run.Status,
			"real_execution_pending": false,
			"evidence_capture":       true,
			"manual_review":          false,
			"autopilot":              true,
			"output":                 output,
		}

		if runAfterExecute != nil {
			controlledRuntime["runtime_status"] = runAfterExecute.Status
		}
		if runtimeResult != nil {
			controlledRuntime["controlled_test_result_id"] = runtimeResult.ID
			controlledRuntime["result_status"] = runtimeResult.Status
			controlledRuntime["confidence"] = runtimeResult.Confidence
			controlledRuntime["severity_hint"] = runtimeResult.SeverityHint
			result.ControlledResults = append(result.ControlledResults, runtimeResult.ID)

			input := chatActionInputMap(action.InputJSON)
			bugResultID := uintFromInterface(input["bug_test_result_id"])
			if bugResultID > 0 {
				linkOutput := linkBugTestResultToControlledRuntime(target.ID, bugResultID, run.ID, runtimeResult.ID, output, runtimeResult)
				controlledRuntime["bug_test_result_link"] = linkOutput
			}
		}
		if mem, _ := output["memory_ingested"].(bool); mem {
			result.MemoryIngested = true
		}

		preview["execution_enabled"] = true
		preview["executed"] = true
		preview["hard_blocked"] = false
		preview["reason"] = "agent chat autopilot created and executed controlled runtime run"
		preview["controlled_runtime"] = controlledRuntime
		preview["autopilot"] = map[string]interface{}{
			"enabled": true,
			"mode":    result.Mode,
			"source":  "agent_chat",
		}

		errorMessage := ""
		if execErr != nil {
			errorMessage = execErr.Error()
			result.Errors = append(result.Errors, fmt.Sprintf("execute controlled run %d failed: %v", run.ID, execErr))
		}

		completedAt := time.Now().UTC()
		if err := updateChatActionOutput(&action, preview, errorMessage, models.AgentActionStatusExecuted, &executedAt, &completedAt); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("update action %d autopilot output failed: %v", action.ID, err))
		}

		result.ExecutedActions = append(result.ExecutedActions, action.ID)
		summary := map[string]interface{}{
			"action_id":     action.ID,
			"run_id":        run.ID,
			"status":        output["status"],
			"status_code":   output["status_code"],
			"memory_ingest": output["memory_ingested"],
			"error":         errorMessage,
		}
		if runtimeResult != nil {
			summary["result_id"] = runtimeResult.ID
		}
		result.Summary = append(result.Summary, summary)

		entityID := action.ID
		_ = auditlog.Record(auditlog.Entry{
			ActorUserID: &uid,
			Action:      "target.agent_action.autopilot_execute",
			EntityType:  "agent_action",
			EntityID:    &entityID,
			TargetID:    &target.ID,
			IPAddress:   auditlog.ClientIP(c),
			UserAgent:   auditlog.UserAgent(c),
			Metadata: map[string]interface{}{
				"source":            "agent_chat_autopilot",
				"mode":              result.Mode,
				"controlled_run_id": run.ID,
				"memory_ingested":   result.MemoryIngested,
			},
		})
	}

	return result
}

func highValueSurfaceBugClasses(rawURL string) ([]string, []string) {
	classes := make([]string, 0)
	reasons := make([]string, 0)

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return classes, reasons
	}

	path := strings.ToLower(parsed.Path)
	query := parsed.Query()

	add := func(cls string, reason string) {
		for _, existing := range classes {
			if existing == cls {
				return
			}
		}
		classes = append(classes, cls)
		reasons = append(reasons, reason)
	}

	if strings.Contains(path, "api") || strings.Contains(path, "graphql") {
		add("api_authorization", "API-like endpoint path")
	}
	if strings.Contains(path, "admin") || strings.Contains(path, "dashboard") || strings.Contains(path, "account") || strings.Contains(path, "profile") {
		add("idor", "account/admin/profile-like endpoint path")
	}
	if strings.Contains(path, "upload") || strings.Contains(path, "import") || strings.Contains(path, "attachment") {
		add("file_upload_abuse", "upload/import/attachment-like endpoint path")
	}
	if strings.Contains(path, "download") || strings.Contains(path, "export") || strings.Contains(path, "file") || strings.Contains(path, "template") {
		add("path_traversal", "download/export/file/template-like endpoint path")
	}
	if strings.Contains(path, "callback") || strings.Contains(path, "webhook") || strings.Contains(path, "fetch") || strings.Contains(path, "proxy") {
		add("ssrf", "callback/webhook/fetch/proxy-like endpoint path")
	}

	for key := range query {
		k := strings.ToLower(strings.TrimSpace(key))
		switch k {
		case "id", "user", "userid", "user_id", "account", "account_id", "org", "org_id", "team", "team_id", "role", "uid":
			add("idor", "identifier-like query parameter: "+k)
		case "url", "uri", "target", "dest", "destination", "callback", "webhook", "next", "return", "redirect", "redirect_uri", "image", "avatar", "proxy", "fetch":
			add("ssrf", "URL/redirect/fetch-like query parameter: "+k)
			add("open_redirect_chain", "redirect-like query parameter: "+k)
		case "file", "path", "filename", "template", "download", "export", "doc", "document", "attachment":
			add("path_traversal", "file/path-like query parameter: "+k)
		case "q", "query", "search", "filter", "sort", "where", "select":
			add("sqli", "query/filter/search-like query parameter: "+k)
			add("xss", "reflectable search/query parameter: "+k)
		case "cmd", "command", "exec", "ping", "host", "domain":
			add("command_injection", "command/host-like query parameter: "+k)
		case "html", "message", "msg", "comment", "name", "title", "text", "content":
			add("xss", "HTML/text-like query parameter: "+k)
		case "header", "returnurl", "return_url":
			add("crlf", "header/return-like query parameter: "+k)
		}
	}

	return classes, reasons
}

const highValueHuntingLoopMaxCandidates = 3

type highValueSurfaceCandidate struct {
	URL     string
	Score   int
	Classes []string
	Reasons []string
	Source  string
}

func appendUniqueHighValueString(items []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return items
	}
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}

func highValueCandidateKey(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(strings.ToLower(rawURL), "/")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""

	if parsed.Scheme == "https" && strings.HasSuffix(parsed.Host, ":443") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":443")
	}
	if parsed.Scheme == "http" && strings.HasSuffix(parsed.Host, ":80") {
		parsed.Host = strings.TrimSuffix(parsed.Host, ":80")
	}

	if parsed.Path != "/" {
		parsed.Path = strings.TrimRight(parsed.Path, "/")
	}

	return strings.TrimRight(strings.ToLower(parsed.String()), "/")
}

func candidateLooksRecentlyBad(text string) bool {
	text = strings.ToLower(strings.TrimSpace(text))
	if text == "" {
		return false
	}

	badSignals := []string{
		"tls mismatch",
		"certificate",
		"connection error",
		"timeout",
		"deadline exceeded",
		"no such host",
		"429",
		"rate limit",
		"cloudflare",
		"waf",
		"blocked",
		"inconclusive",
		"forbidden",
		" 403",
		"status_code:403",
		"status code 403",
		" 5xx",
		"status_code:5",
		"status code 5",
	}

	for _, signal := range badSignals {
		if strings.Contains(text, signal) {
			return true
		}
	}
	return false
}

func highValueMemoryMetadata(item models.TargetMemoryItem) map[string]interface{} {
	out := map[string]interface{}{}
	if len(item.Metadata) > 0 {
		_ = json.Unmarshal(item.Metadata, &out)
	}
	return out
}

func highValueMemoryString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	value, ok := m[key]
	if !ok || value == nil {
		return ""
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func highValueMemoryBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	switch v := m[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func highValueMemoryIndicatesAvoidRetesting(item models.TargetMemoryItem) (bool, string, string) {
	metadata := highValueMemoryMetadata(item)

	candidateURL := highValueMemoryString(metadata, "candidate_url")
	analysisStatus := strings.ToLower(highValueMemoryString(metadata, "analysis_status"))
	classification := strings.ToLower(highValueMemoryString(metadata, "classification"))
	httpStatus := strings.ToLower(highValueMemoryString(metadata, "http_status"))
	avoidReason := highValueMemoryString(metadata, "avoid_reason")

	if highValueMemoryBool(metadata, "avoid_retesting") {
		return true, candidateURL, avoidReason
	}

	if analysisStatus == "blocked" || analysisStatus == "inconclusive" || analysisStatus == "failed" {
		return true, candidateURL, analysisStatus
	}

	if strings.Contains(classification, "blocked") ||
		strings.Contains(classification, "challenged") ||
		strings.Contains(classification, "cloudflare") ||
		strings.Contains(classification, "waf") {
		return true, candidateURL, classification
	}

	switch httpStatus {
	case "401", "403", "429":
		return true, candidateURL, "http_" + httpStatus
	}

	text := strings.ToLower(strings.Join([]string{
		item.Title,
		item.Summary,
		item.Content,
		string(item.Tags),
		string(item.Metadata),
	}, " "))
	if candidateLooksRecentlyBad(text) {
		return true, candidateURL, "text_bad_signal"
	}

	return false, candidateURL, ""
}

func badHighValueCandidateURLsFromMemory(targetID uint, candidates []highValueSurfaceCandidate) map[string]bool {
	bad := map[string]bool{}
	if targetID == 0 || len(candidates) == 0 {
		return bad
	}

	candidateKeys := map[string]bool{}
	for _, candidate := range candidates {
		key := highValueCandidateKey(candidate.URL)
		if key != "" {
			candidateKeys[key] = true
		}
	}

	items := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND source_type = ?", targetID, models.TargetMemorySourceControlledTestResult).
		Order("updated_at desc, importance desc").
		Limit(160).
		Find(&items).Error

	for _, item := range items {
		shouldAvoid, memoryCandidateURL, _ := highValueMemoryIndicatesAvoidRetesting(item)
		if !shouldAvoid {
			continue
		}

		memoryCandidateKey := highValueCandidateKey(memoryCandidateURL)
		if memoryCandidateKey != "" && candidateKeys[memoryCandidateKey] {
			bad[memoryCandidateKey] = true
			continue
		}

		text := strings.ToLower(strings.Join([]string{
			item.Title,
			item.Summary,
			item.Content,
			string(item.Metadata),
			string(item.Tags),
		}, " "))

		for _, candidate := range candidates {
			key := highValueCandidateKey(candidate.URL)
			if key == "" {
				continue
			}
			if strings.Contains(text, key) || strings.Contains(text, strings.ToLower(candidate.URL)) {
				bad[key] = true
			}
		}
	}

	return bad
}

func dedupeAndRankHighValueCandidates(targetID uint, candidates []highValueSurfaceCandidate) []highValueSurfaceCandidate {
	if len(candidates) == 0 {
		return candidates
	}

	badURLs := badHighValueCandidateURLsFromMemory(targetID, candidates)
	byURL := map[string]highValueSurfaceCandidate{}

	for _, candidate := range candidates {
		candidate.URL = strings.TrimSpace(candidate.URL)
		if candidate.URL == "" {
			continue
		}

		key := highValueCandidateKey(candidate.URL)
		if key == "" || badURLs[key] {
			continue
		}

		existing, exists := byURL[key]
		if !exists {
			classes := []string{}
			for _, cls := range candidate.Classes {
				classes = appendUniqueHighValueString(classes, cls)
			}

			reasons := []string{}
			for _, reason := range candidate.Reasons {
				reasons = appendUniqueHighValueString(reasons, reason)
			}

			candidate.Classes = classes
			candidate.Reasons = reasons
			byURL[key] = candidate
			continue
		}

		if candidate.Score > existing.Score {
			existing.Score = candidate.Score
			existing.Source = candidate.Source
		}

		for _, cls := range candidate.Classes {
			existing.Classes = appendUniqueHighValueString(existing.Classes, cls)
		}
		for _, reason := range candidate.Reasons {
			existing.Reasons = appendUniqueHighValueString(existing.Reasons, reason)
		}

		byURL[key] = existing
	}

	out := make([]highValueSurfaceCandidate, 0, len(byURL))
	for _, candidate := range byURL {
		out = append(out, candidate)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].URL < out[j].URL
		}
		return out[i].Score > out[j].Score
	})

	if len(out) > highValueHuntingLoopMaxCandidates {
		out = out[:highValueHuntingLoopMaxCandidates]
	}

	return out
}

func buildHighValueSurfaceProbeAction(candidate highValueSurfaceCandidate, stepNumber int, totalSteps int) proposeAgentActionRequest {
	return proposeAgentActionRequest{
		ActionType:       models.AgentActionTypeReviewEndpoint,
		Title:            fmt.Sprintf("Hunting step %d/%d: controlled baseline probe", stepNumber, totalSteps),
		Description:      "Run a controlled baseline HTTP probe against a high-value surface candidate selected from crawled URLs/live assets. This collects real runtime evidence before bug-class-specific validation.",
		RiskLevel:        models.AgentActionRiskLow,
		SafetyLevel:      1,
		TestLevel:        1,
		AutonomyLevel:    1,
		RequestedByAgent: boolPtr(true),
		RequiresApproval: boolPtr(true),
		InputJSON: map[string]interface{}{
			"url":                   candidate.URL,
			"method":                "GET",
			"source":                "high_value_hunting_loop_v1",
			"previous_source":       "high_value_surface_miner_v1",
			"hypothesis_driven":     true,
			"hunting_loop":          true,
			"hunting_loop_version":  "small_controlled_baseline_hunting_loop_v1",
			"hunting_step":          stepNumber,
			"hunting_total_steps":   totalSteps,
			"hunting_candidate_url": candidate.URL,
			"hunting_action_key":    fmt.Sprintf("high_value_hunting_loop_v1:%d:%s", stepNumber, highValueCandidateKey(candidate.URL)),
			"surface_score":         candidate.Score,
			"candidate_bug_classes": candidate.Classes,
			"candidate_reasons":     candidate.Reasons,
			"candidate_source":      candidate.Source,
			"validation_goal":       "collect controlled baseline HTTP evidence for high-value bug hunting",
			"payload_execution":     false,
			"exploit_validation":    false,
			"operator_visible_step": fmt.Sprintf("Hunting step %d/%d: selected high-value candidate and ran controlled baseline probe", stepNumber, totalSteps),
		},
	}
}

func maybeBuildHighValueSurfaceProbeActions(target *models.Target, text string) []proposeAgentActionRequest {
	if target == nil || !chatWantsPentestHypothesisMode(text) {
		return nil
	}

	candidates := make([]highValueSurfaceCandidate, 0)

	rows := make([]models.FoundURL, 0)
	_ = database.DB.
		Where("target_id = ?", target.ID).
		Order("occurrence_count desc, id desc").
		Limit(300).
		Find(&rows).Error

	for _, row := range rows {
		raw := strings.TrimSpace(row.Value)
		if raw == "" {
			continue
		}

		lowerRaw := strings.ToLower(raw)
		if !strings.HasPrefix(lowerRaw, "http://") && !strings.HasPrefix(lowerRaw, "https://") {
			continue
		}

		classes, reasons := highValueSurfaceBugClasses(raw)
		if len(classes) == 0 {
			continue
		}

		score := len(classes)*10 + len(reasons)*3
		if strings.Contains(raw, "?") {
			score += 15
		}

		candidates = append(candidates, highValueSurfaceCandidate{
			URL:     raw,
			Score:   score,
			Classes: classes,
			Reasons: reasons,
			Source:  "found_urls",
		})
	}

	assets := make([]models.Asset, 0)
	_ = database.DB.
		Where("target_id = ? AND is_live = ?", target.ID, true).
		Order("status_code asc, id desc").
		Limit(200).
		Find(&assets).Error

	for _, asset := range assets {
		raw := strings.TrimSpace(asset.FinalURL)
		if raw == "" {
			value := strings.TrimSpace(asset.Value)
			if value == "" {
				continue
			}
			raw = "https://" + value
		}

		classes, reasons := highValueSurfaceBugClasses(raw)
		host := strings.ToLower(strings.TrimSpace(asset.Value))

		if strings.Contains(host, "api") {
			classes = appendUniqueHighValueString(classes, "api_authorization")
			reasons = appendUniqueHighValueString(reasons, "api-like live asset hostname")
		}
		if strings.Contains(host, "admin") || strings.Contains(host, "dashboard") || strings.Contains(host, "sso") || strings.Contains(host, "auth") {
			classes = appendUniqueHighValueString(classes, "auth_bypass")
			reasons = appendUniqueHighValueString(reasons, "admin/auth/dashboard-like live asset hostname")
		}
		if strings.Contains(host, "dev") || strings.Contains(host, "stage") || strings.Contains(host, "staging") || strings.Contains(host, "test") {
			classes = appendUniqueHighValueString(classes, "sensitive_data_exposure")
			reasons = appendUniqueHighValueString(reasons, "dev/staging/test-like live asset hostname")
		}
		if len(classes) == 0 {
			continue
		}

		score := len(classes)*10 + len(reasons)*3
		if asset.StatusCode >= 200 && asset.StatusCode < 400 {
			score += 5
		}

		candidates = append(candidates, highValueSurfaceCandidate{
			URL:     raw,
			Score:   score,
			Classes: classes,
			Reasons: reasons,
			Source:  "live_assets",
		})
	}

	candidates = dedupeAndRankHighValueCandidates(target.ID, candidates)
	if len(candidates) == 0 {
		return nil
	}

	actions := make([]proposeAgentActionRequest, 0, len(candidates))
	totalSteps := len(candidates)

	for i, candidate := range candidates {
		actions = append(actions, buildHighValueSurfaceProbeAction(candidate, i+1, totalSteps))
	}

	return actions
}

func maybeBuildBugTestValidationAction(target *models.Target, text string) *proposeAgentActionRequest {
	if target == nil {
		return nil
	}

	lower := strings.ToLower(strings.TrimSpace(text))
	if lower == "" {
		return nil
	}

	wantsValidation := strings.Contains(lower, "validate") ||
		strings.Contains(lower, "validation") ||
		strings.Contains(lower, "verify") ||
		strings.Contains(lower, "check candidate") ||
		strings.Contains(lower, "test candidate") ||
		strings.Contains(lower, "تایید") ||
		strings.Contains(lower, "اعتبارسنج") ||
		strings.Contains(lower, "ولید")

	wantsBugCandidate := strings.Contains(lower, "bug") ||
		strings.Contains(lower, "candidate") ||
		strings.Contains(lower, "result") ||
		strings.Contains(lower, "security header") ||
		strings.Contains(lower, "header")

	if !wantsValidation || !wantsBugCandidate {
		return nil
	}

	var result models.BugTestResult
	if err := database.DB.
		Where("target_id = ? AND status = ? AND bug_type = ?", target.ID, models.BugTestResultStatusNeedsManualValidation, models.BugTypeSecurityHeaders).
		Order("id DESC").
		First(&result).Error; err != nil {
		return nil
	}

	evidence := map[string]interface{}{}
	if len(result.EvidenceJSON) > 0 {
		_ = json.Unmarshal(result.EvidenceJSON, &evidence)
	}

	rawURL := strings.TrimSpace(controlledCleanString(evidence["final_url"]))
	if rawURL == "" {
		asset := strings.TrimSpace(controlledCleanString(evidence["asset"]))
		if asset != "" {
			rawURL = asset
		}
	}
	if rawURL == "" {
		return nil
	}

	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") && !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		rawURL = "https://" + rawURL
	}

	req := proposeAgentActionRequest{
		ActionType:       models.AgentActionTypeReviewEndpoint,
		Title:            fmt.Sprintf("Validate bug test candidate #%d", result.ID),
		Description:      fmt.Sprintf("Run a controlled HTTP endpoint review for Safe Bug Testing candidate #%d (%s) to collect runtime evidence before validation or promotion.", result.ID, result.BugType),
		RiskLevel:        models.AgentActionRiskLow,
		SafetyLevel:      1,
		TestLevel:        1,
		AutonomyLevel:    1,
		RequestedByAgent: boolPtr(true),
		RequiresApproval: boolPtr(true),
		InputJSON: map[string]interface{}{
			"url":                rawURL,
			"method":             "GET",
			"source":             "bug_test_candidate_validation",
			"bug_test_result_id": result.ID,
			"bug_test_run_id":    result.RunID,
			"bug_type":           result.BugType,
			"candidate_status":   result.Status,
			"validation_goal":    "collect controlled HTTP response/header evidence for candidate triage",
		},
	}

	return &req
}

func chatActionRequestSource(req proposeAgentActionRequest) string {
	if req.InputJSON == nil {
		return ""
	}
	value, ok := req.InputJSON["source"]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func shouldBypassChatActionDuplicateReuse(req proposeAgentActionRequest) bool {
	switch chatActionRequestSource(req) {
	case "high_value_hunting_loop_v1":
		return true
	default:
		return false
	}
}

func createActionFromChat(c *fiber.Ctx, target *models.Target, uid uint, ownerKey string, req proposeAgentActionRequest) (*models.AgentAction, error) {
	actionType := normalizeAgentActionType(req.ActionType)
	if actionType == "" {
		return nil, fmt.Errorf("invalid action type")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = actionType
	}

	var duplicate *models.AgentAction
	var duplicateErr error
	if !shouldBypassChatActionDuplicateReuse(req) {
		duplicate, duplicateErr = findDuplicateAgentAction(target.ID, actionType, title)
		if duplicateErr != nil {
			return nil, duplicateErr
		}
		if duplicate == nil && shouldReuseChatActionByType(actionType) {
			duplicate, duplicateErr = findDuplicateAgentAction(target.ID, actionType, "")
			if duplicateErr != nil {
				return nil, duplicateErr
			}
		}
	}
	if duplicate != nil {
		updatedDuplicate, reuseErr := markDuplicateActionReusedByChat(c, target, uid, ownerKey, duplicate, req)
		if reuseErr != nil {
			return nil, reuseErr
		}
		if updatedDuplicate != nil {
			duplicate = updatedDuplicate
		}

		entityID := duplicate.ID
		_ = auditlog.Record(auditlog.Entry{
			ActorUserID: &uid,
			Action:      "target.agent_action.propose_duplicate",
			EntityType:  "agent_action",
			EntityID:    &entityID,
			TargetID:    &target.ID,
			IPAddress:   auditlog.ClientIP(c),
			UserAgent:   auditlog.UserAgent(c),
			Metadata: map[string]interface{}{
				"target_id":      target.ID,
				"root_domain":    target.RootDomain,
				"source":         "agent_chat",
				"action_type":    duplicate.ActionType,
				"status":         duplicate.Status,
				"policy_status":  duplicate.PolicyStatus,
				"risk_level":     duplicate.RiskLevel,
				"duplicate":      true,
				"reused_by_chat": true,
			},
		})
		return duplicate, nil
	}

	risk := normalizeAgentRisk(req.RiskLevel)
	safetyLevel := clampAgentLevel(req.SafetyLevel)
	testLevel := clampAgentLevel(req.TestLevel)
	autonomyLevel := clampAgentLevel(req.AutonomyLevel)

	requestedByAgent := true
	if req.RequestedByAgent != nil {
		requestedByAgent = *req.RequestedByAgent
	}

	requiresApproval := true
	if req.RequiresApproval != nil {
		requiresApproval = *req.RequiresApproval
	}

	policyStatus, policyReason, policyCheck := policyCheckForAgentAction(target, req)

	status := models.AgentActionStatusProposed
	if policyStatus == models.AgentActionPolicyStatusBlocked {
		status = models.AgentActionStatusBlockedByPolicy
		requiresApproval = true
	}

	row := models.AgentAction{
		TargetID:         target.ID,
		OwnerKey:         ownerKey,
		AgentRunID:       req.AgentRunID,
		CreatedByUserID:  &uid,
		ActionType:       actionType,
		Title:            title,
		Description:      strings.TrimSpace(req.Description),
		Status:           status,
		PolicyStatus:     policyStatus,
		RiskLevel:        risk,
		SafetyLevel:      safetyLevel,
		TestLevel:        testLevel,
		AutonomyLevel:    autonomyLevel,
		RequestedByAgent: requestedByAgent,
		RequiresApproval: requiresApproval,
		InputJSON:        agentActionJSON(req.InputJSON),
		OutputJSON:       datatypes.JSON([]byte("{}")),
		PolicyCheckJSON:  agentActionJSON(policyCheck),
	}

	if policyStatus == models.AgentActionPolicyStatusBlocked {
		row.ErrorMessage = policyReason
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return nil, err
	}

	entityID := row.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.agent_action.propose",
		EntityType:  "agent_action",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":     target.ID,
			"root_domain":   target.RootDomain,
			"source":        "agent_chat",
			"action_type":   row.ActionType,
			"status":        row.Status,
			"policy_status": row.PolicyStatus,
			"risk_level":    row.RiskLevel,
			"safety_level":  row.SafetyLevel,
			"test_level":    row.TestLevel,
		},
	})

	return &row, nil
}

func huntingLoopActions(actions []models.AgentAction) []models.AgentAction {
	out := make([]models.AgentAction, 0)
	for _, action := range actions {
		input := chatActionInputMap(action.InputJSON)
		source := strings.TrimSpace(fmt.Sprint(input["source"]))
		if source == "high_value_hunting_loop_v1" {
			out = append(out, action)
		}
	}
	return out
}

func huntingLoopInputString(input map[string]interface{}, key string) string {
	if input == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(input[key]))
}

func huntingLoopInputBool(input map[string]interface{}, key string) bool {
	if input == nil {
		return false
	}
	switch v := input[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func huntingLoopInputStringSlice(input map[string]interface{}, key string) []string {
	if input == nil {
		return nil
	}
	raw, ok := input[key]
	if !ok || raw == nil {
		return nil
	}
	out := make([]string, 0)
	switch values := raw.(type) {
	case []string:
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" {
				out = append(out, value)
			}
		}
	case []interface{}:
		for _, value := range values {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				out = append(out, text)
			}
		}
	default:
		text := strings.TrimSpace(fmt.Sprint(values))
		if text != "" && text != "<nil>" {
			out = append(out, text)
		}
	}
	return out
}

func buildHuntingLoopAssistantText(actions []models.AgentAction) string {
	loopActions := huntingLoopActions(actions)
	if len(loopActions) == 0 {
		return ""
	}

	lines := []string{
		"Controlled baseline hunting loop:",
		"",
	}

	for _, action := range loopActions {
		input := chatActionInputMap(action.InputJSON)
		step := huntingLoopInputString(input, "hunting_step")
		total := huntingLoopInputString(input, "hunting_total_steps")
		if step == "" || step == "<nil>" {
			step = "?"
		}
		if total == "" || total == "<nil>" {
			total = fmt.Sprint(len(loopActions))
		}

		urlValue := huntingLoopInputString(input, "hunting_candidate_url")
		if urlValue == "" || urlValue == "<nil>" {
			urlValue = huntingLoopInputString(input, "url")
		}

		reasons := huntingLoopInputStringSlice(input, "candidate_reasons")
		reason := "selected as a high-value surface candidate from target evidence"
		if len(reasons) > 0 {
			reason = reasons[0]
		}

		classes := huntingLoopInputStringSlice(input, "candidate_bug_classes")
		next := "analyze controlled runtime evidence and only then propose class-specific validation."
		if len(classes) > 0 {
			next = "baseline evidence will guide next validation path for: " + strings.Join(classes, ", ")
		}

		lines = append(lines,
			fmt.Sprintf("- Hunting step %s/%s", step, total),
			fmt.Sprintf("  Candidate: %s", urlValue),
			fmt.Sprintf("  Why selected: %s", reason),
			"  Action: controlled baseline HTTP probe",
			fmt.Sprintf("  Action ID: #%d status=%s", action.ID, action.Status),
			"  Payload/exploit execution: disabled",
			"  Next: "+next,
			"",
		)
	}

	return strings.Join(lines, "\n")
}

func buildHuntingSessionOutput(actions []models.AgentAction) map[string]interface{} {
	loopActions := huntingLoopActions(actions)
	steps := make([]map[string]interface{}, 0, len(loopActions))
	actionIDs := make([]uint, 0, len(loopActions))

	for _, action := range loopActions {
		input := chatActionInputMap(action.InputJSON)
		actionIDs = append(actionIDs, action.ID)

		steps = append(steps, map[string]interface{}{
			"step":                  huntingLoopInputString(input, "hunting_step"),
			"total_steps":           huntingLoopInputString(input, "hunting_total_steps"),
			"action_id":             action.ID,
			"action_type":           action.ActionType,
			"action_status":         action.Status,
			"risk_level":            action.RiskLevel,
			"safety_level":          action.SafetyLevel,
			"test_level":            action.TestLevel,
			"autonomy_level":        action.AutonomyLevel,
			"candidate_url":         huntingLoopInputString(input, "hunting_candidate_url"),
			"url":                   huntingLoopInputString(input, "url"),
			"candidate_source":      huntingLoopInputString(input, "candidate_source"),
			"candidate_bug_classes": huntingLoopInputStringSlice(input, "candidate_bug_classes"),
			"candidate_reasons":     huntingLoopInputStringSlice(input, "candidate_reasons"),
			"surface_score":         huntingLoopInputString(input, "surface_score"),
			"hunting_action_key":    huntingLoopInputString(input, "hunting_action_key"),
			"validation_goal":       huntingLoopInputString(input, "validation_goal"),
			"payload_execution":     huntingLoopInputBool(input, "payload_execution"),
			"exploit_validation":    huntingLoopInputBool(input, "exploit_validation"),
			"result":                "pending_runtime_evidence",
			"learned":               "waiting for controlled runtime result analysis",
		})
	}

	return map[string]interface{}{
		"version":            "small_controlled_baseline_hunting_loop_v1",
		"active":             len(loopActions) > 0,
		"objective":          "collect controlled baseline HTTP evidence across multiple high-value candidates",
		"created_action_ids": actionIDs,
		"candidate_count":    len(loopActions),
		"max_candidates":     highValueHuntingLoopMaxCandidates,
		"steps":              steps,
		"payload_execution":  false,
		"exploit_validation": false,
		"next":               "analyze runtime results, learn from passed/failed/inconclusive probes, and propose class-specific validation paths when evidence supports them",
	}
}

func formatAssistantTextForChatReadability(text string) string {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return text
	}

	sectionPrefixes := []string{
		"Clarifying questions:",
		"Recommended next steps:",
		"Pentest hypotheses:",
		"Controlled baseline hunting loop:",
		"Operator Autopilot:",
		"Guardrails:",
	}

	lines := strings.Split(text, "\n")
	out := make([]string, 0, len(lines)*2)

	appendBlankIfNeeded := func() {
		if len(out) == 0 {
			return
		}
		if strings.TrimSpace(out[len(out)-1]) != "" {
			out = append(out, "")
		}
	}

	for _, rawLine := range lines {
		line := strings.TrimRight(rawLine, " \t")
		trimmed := strings.TrimSpace(line)

		if trimmed == "" {
			if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
				out = append(out, "")
			}
			continue
		}

		for _, prefix := range sectionPrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				appendBlankIfNeeded()
				break
			}
		}

		if strings.HasPrefix(trimmed, "I proposed ") ||
			strings.HasPrefix(trimmed, "I reused ") ||
			strings.HasPrefix(trimmed, "Autopilot executed ") ||
			strings.HasPrefix(trimmed, "Autopilot did not execute ") {
			appendBlankIfNeeded()
		}

		if strings.HasPrefix(trimmed, "- Potential ") && len(out) > 0 {
			appendBlankIfNeeded()
		}

		out = append(out, line)
	}

	// Collapse excessive blank lines to at most one blank line.
	collapsed := make([]string, 0, len(out))
	previousBlank := false
	for _, line := range out {
		blank := strings.TrimSpace(line) == ""
		if blank && previousBlank {
			continue
		}
		collapsed = append(collapsed, line)
		previousBlank = blank
	}

	return strings.TrimSpace(strings.Join(collapsed, "\n"))
}

func controlledEvidenceMap(result models.ControlledTestResult) map[string]interface{} {
	out := map[string]interface{}{}
	if len(result.EvidenceJSON) == 0 {
		return out
	}
	_ = json.Unmarshal(result.EvidenceJSON, &out)
	return out
}

func controlledEvidenceString(evidence map[string]interface{}, key string) string {
	if evidence == nil {
		return ""
	}
	value, ok := evidence[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func controlledEvidenceBool(evidence map[string]interface{}, key string) bool {
	if evidence == nil {
		return false
	}
	switch v := evidence[key].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

func controlledEvidenceInt(evidence map[string]interface{}, key string) int {
	if evidence == nil {
		return 0
	}
	switch v := evidence[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case json.Number:
		n, _ := v.Int64()
		return int(n)
	case string:
		var n int
		_, _ = fmt.Sscanf(strings.TrimSpace(v), "%d", &n)
		return n
	default:
		return 0
	}
}

func analyzeControlledProbeResult(result models.ControlledTestResult) map[string]interface{} {
	evidence := controlledEvidenceMap(result)

	httpStatus := controlledEvidenceInt(evidence, "status_code")
	classification := controlledEvidenceString(evidence, "classification")
	errorText := controlledEvidenceString(evidence, "error")
	finalURL := controlledEvidenceString(evidence, "final_url")
	durationMS := controlledEvidenceInt(evidence, "duration_ms")

	cloudflareChallenge := controlledEvidenceBool(evidence, "cloudflare_challenge") ||
		strings.EqualFold(controlledEvidenceString(evidence, "cf_mitigated"), "challenge") ||
		strings.Contains(strings.ToLower(classification), "challenged")

	status := strings.TrimSpace(result.Status)
	if status == "" {
		status = "inconclusive"
	}

	analysisStatus := status
	why := "Controlled baseline probe completed and produced runtime evidence."
	learned := "The candidate has baseline runtime evidence available for follow-up reasoning."
	next := "Use this baseline result to decide whether class-specific validation is justified."
	usefulForNextValidation := false
	avoidRetesting := false
	avoidReason := ""

	switch {
	case errorText != "":
		analysisStatus = "inconclusive"
		why = "The controlled HTTP request failed before a useful baseline response was collected."
		learned = "This candidate is currently unreliable for unauthenticated baseline validation."
		next = "Avoid repeating the same probe immediately unless DNS/TLS/connectivity context changes."
		avoidRetesting = true
		avoidReason = errorText

	case cloudflareChallenge:
		analysisStatus = "blocked"
		why = "The endpoint returned a Cloudflare/WAF challenge instead of normal application content."
		learned = "The response is blocked/challenged evidence, not proof of a vulnerability."
		next = "Do not promote this as a finding. Skip unauthenticated retest or ask for approved context if this surface is important."
		avoidRetesting = true
		avoidReason = "cloudflare_or_waf_challenge"

	case httpStatus == 401 || httpStatus == 403:
		analysisStatus = "inconclusive"
		why = fmt.Sprintf("The endpoint returned HTTP %d, so baseline access is blocked or requires authorization.", httpStatus)
		learned = "This candidate likely needs auth/session context before meaningful validation."
		next = "Ask for authenticated context or move to another candidate for unauthenticated low-risk hunting."
		avoidRetesting = true
		avoidReason = fmt.Sprintf("http_%d_requires_context", httpStatus)

	case httpStatus == 429:
		analysisStatus = "blocked"
		why = "The endpoint returned HTTP 429 rate limiting."
		learned = "Further probing may create noisy or rate-limited behavior."
		next = "Stop retesting this candidate until the rate-limit window and policy budget are clear."
		avoidRetesting = true
		avoidReason = "rate_limited"

	case httpStatus >= 500:
		analysisStatus = "inconclusive"
		why = fmt.Sprintf("The endpoint returned HTTP %d server-side error during baseline probing.", httpStatus)
		learned = "The candidate may be unstable or backend-protected; this is not enough to claim a vulnerability."
		next = "Avoid immediate repetition. Revisit only if there is a strong hypothesis and a strict request budget."
		avoidRetesting = true
		avoidReason = fmt.Sprintf("http_%d_server_error", httpStatus)

	case httpStatus >= 200 && httpStatus < 400:
		analysisStatus = "passed"
		why = fmt.Sprintf("The endpoint returned HTTP %d and is reachable with a normal baseline response.", httpStatus)
		learned = "This candidate is suitable for later class-specific controlled validation if its parameters, headers, or behavior support a hypothesis."
		next = "Prioritize parameter inventory, reflection/context checks, cache/header review, or open-redirect/path baseline depending on URL shape and evidence."
		usefulForNextValidation = true

	default:
		analysisStatus = "inconclusive"
		if httpStatus > 0 {
			why = fmt.Sprintf("The endpoint returned HTTP %d, which does not provide enough confidence for validation.", httpStatus)
		}
		learned = "The probe did not establish a useful positive baseline."
		next = "Move to a better candidate unless new target-specific evidence appears."
		avoidRetesting = true
		avoidReason = "weak_or_unknown_baseline"
	}

	return map[string]interface{}{
		"status":                     analysisStatus,
		"why":                        why,
		"learned":                    learned,
		"next":                       next,
		"useful_for_next_validation": usefulForNextValidation,
		"avoid_retesting":            avoidRetesting,
		"avoid_reason":               avoidReason,
		"http_status":                httpStatus,
		"classification":             classification,
		"final_url":                  finalURL,
		"duration_ms":                durationMS,
		"runtime_result_status":      result.Status,
	}
}

func enrichHuntingSessionWithRuntimeResults(session map[string]interface{}, autopilot chatAutopilotResult) map[string]interface{} {
	if session == nil {
		return session
	}

	rawSteps, ok := session["steps"].([]map[string]interface{})
	if !ok || len(rawSteps) == 0 || len(autopilot.ControlledResults) == 0 {
		return session
	}

	results := make([]models.ControlledTestResult, 0)
	if err := database.DB.
		Where("id IN ?", autopilot.ControlledResults).
		Order("id asc").
		Find(&results).Error; err != nil {
		return session
	}

	byActionID := map[uint]models.ControlledTestResult{}
	for _, result := range results {
		if result.AgentActionID != nil {
			byActionID[*result.AgentActionID] = result
		}
	}

	analyzedCount := 0
	for _, step := range rawSteps {
		actionID := uintFromAny(step["action_id"])
		if actionID == 0 {
			continue
		}

		result, exists := byActionID[actionID]
		if !exists {
			continue
		}

		analysis := analyzeControlledProbeResult(result)
		step["controlled_result_id"] = result.ID
		step["controlled_run_id"] = result.RunID
		step["runtime_status"] = result.Status
		step["http_status"] = analysis["http_status"]
		step["classification"] = analysis["classification"]
		step["final_url"] = analysis["final_url"]
		step["duration_ms"] = analysis["duration_ms"]
		step["result"] = analysis["status"]
		step["learned"] = analysis["learned"]
		step["analysis"] = analysis
		analyzedCount++
	}

	session["steps"] = rawSteps
	session["analyzed_result_count"] = analyzedCount
	session["controlled_results"] = autopilot.ControlledResults
	session["controlled_runs"] = autopilot.ControlledRuns
	session["memory_ingested"] = autopilot.MemoryIngested
	return session
}

func buildHuntingRuntimeAnalysisText(session map[string]interface{}) string {
	if session == nil {
		return ""
	}
	rawSteps, ok := session["steps"].([]map[string]interface{})
	if !ok || len(rawSteps) == 0 {
		return ""
	}

	lines := []string{"Runtime analysis:", ""}
	hasAnalysis := false

	for _, step := range rawSteps {
		analysis, _ := step["analysis"].(map[string]interface{})
		if len(analysis) == 0 {
			continue
		}
		hasAnalysis = true

		stepNo := strings.TrimSpace(fmt.Sprint(step["step"]))
		total := strings.TrimSpace(fmt.Sprint(step["total_steps"]))
		candidate := strings.TrimSpace(fmt.Sprint(step["candidate_url"]))
		if candidate == "" || candidate == "<nil>" {
			candidate = strings.TrimSpace(fmt.Sprint(step["url"]))
		}

		status := strings.TrimSpace(fmt.Sprint(analysis["status"]))
		httpStatus := strings.TrimSpace(fmt.Sprint(analysis["http_status"]))
		learned := strings.TrimSpace(fmt.Sprint(analysis["learned"]))
		next := strings.TrimSpace(fmt.Sprint(analysis["next"]))

		lines = append(lines,
			fmt.Sprintf("- Hunting step %s/%s", stepNo, total),
			fmt.Sprintf("  Candidate: %s", candidate),
			fmt.Sprintf("  Result: %s, HTTP %s", status, httpStatus),
			fmt.Sprintf("  Learned: %s", learned),
			fmt.Sprintf("  Next: %s", next),
			"",
		)
	}

	if !hasAnalysis {
		return ""
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func huntingAnalysisMemoryType(status string, usefulForNext bool) string {
	status = strings.ToLower(strings.TrimSpace(status))
	switch {
	case status == "passed" || usefulForNext:
		return models.TargetMemoryTypeSuccessfulTest
	case status == "blocked" || status == "inconclusive" || status == "failed":
		return models.TargetMemoryTypeFailedTest
	default:
		return models.TargetMemoryTypeTestResult
	}
}

func huntingAnalysisMemoryImportance(status string, usefulForNext bool, avoidRetesting bool) int {
	status = strings.ToLower(strings.TrimSpace(status))
	switch {
	case status == "passed" || usefulForNext:
		return 84
	case status == "blocked" && avoidRetesting:
		return 88
	case status == "inconclusive" && avoidRetesting:
		return 84
	case avoidRetesting:
		return 80
	default:
		return 72
	}
}

func huntingAnalysisMemoryConfidence(status string) int {
	status = strings.ToLower(strings.TrimSpace(status))
	switch status {
	case "passed":
		return 78
	case "blocked":
		return 82
	case "inconclusive":
		return 70
	default:
		return 65
	}
}

func createHuntingAnalysisMemoryItems(userID uint, ownerKey string, target *models.Target, sessionID uint, messageID uint, huntingSession map[string]interface{}) error {
	if target == nil || strings.TrimSpace(ownerKey) == "" || huntingSession == nil {
		return nil
	}

	rawSteps, ok := huntingSession["steps"].([]map[string]interface{})
	if !ok || len(rawSteps) == 0 {
		return nil
	}

	for _, step := range rawSteps {
		analysis, _ := step["analysis"].(map[string]interface{})
		if len(analysis) == 0 {
			continue
		}

		resultID := uintFromAny(step["controlled_result_id"])
		runID := uintFromAny(step["controlled_run_id"])
		actionID := uintFromAny(step["action_id"])

		candidateURL := strings.TrimSpace(fmt.Sprint(step["candidate_url"]))
		if candidateURL == "" || candidateURL == "<nil>" {
			candidateURL = strings.TrimSpace(fmt.Sprint(step["url"]))
		}
		if candidateURL == "" || candidateURL == "<nil>" {
			candidateURL = "unknown-candidate"
		}

		status := strings.TrimSpace(fmt.Sprint(analysis["status"]))
		if status == "" || status == "<nil>" {
			status = "inconclusive"
		}

		httpStatus := strings.TrimSpace(fmt.Sprint(analysis["http_status"]))
		classification := strings.TrimSpace(fmt.Sprint(analysis["classification"]))
		learned := strings.TrimSpace(fmt.Sprint(analysis["learned"]))
		next := strings.TrimSpace(fmt.Sprint(analysis["next"]))
		why := strings.TrimSpace(fmt.Sprint(analysis["why"]))
		avoidReason := strings.TrimSpace(fmt.Sprint(analysis["avoid_reason"]))

		usefulForNext := false
		if v, ok := analysis["useful_for_next_validation"].(bool); ok {
			usefulForNext = v
		}

		avoidRetesting := false
		if v, ok := analysis["avoid_retesting"].(bool); ok {
			avoidRetesting = v
		}

		stepNo := strings.TrimSpace(fmt.Sprint(step["step"]))
		totalSteps := strings.TrimSpace(fmt.Sprint(step["total_steps"]))

		lines := []string{
			"Hunting loop analyzer learning:",
			"Candidate URL: " + candidateURL,
			"Hunting step: " + stepNo + "/" + totalSteps,
			"Action ID: " + fmt.Sprint(actionID),
			"Controlled run ID: " + fmt.Sprint(runID),
			"Controlled result ID: " + fmt.Sprint(resultID),
			"Analyzer status: " + status,
			"HTTP status: " + httpStatus,
			"Classification: " + classification,
			"Useful for next validation: " + fmt.Sprint(usefulForNext),
			"Avoid retesting: " + fmt.Sprint(avoidRetesting),
			"Avoid reason: " + avoidReason,
			"Why: " + why,
			"Learned: " + learned,
			"Next: " + next,
		}

		memoryType := huntingAnalysisMemoryType(status, usefulForNext)
		title := "Hunting analyzer learning: " + candidateURL
		summary := fmt.Sprintf(
			"Hunting step %s/%s for %s analyzed as %s with http_status=%s classification=%s avoid_retesting=%v.",
			stepNo,
			totalSteps,
			candidateURL,
			status,
			httpStatus,
			classification,
			avoidRetesting,
		)

		sourceID := resultID
		if sourceID == 0 {
			sourceID = actionID
		}

		item := models.TargetMemoryItem{
			UserID:     userID,
			OwnerKey:   ownerKey,
			TargetID:   target.ID,
			SourceType: models.TargetMemorySourceControlledTestResult,
			MemoryType: memoryType,
			Title:      title,
			Content:    strings.Join(lines, "\n"),
			Summary:    summary,
			Tags: memoryJSON([]string{
				"controlled_runtime",
				"hunting_loop",
				"probe_result_analyzer",
				"operator_learning",
				status,
				classification,
			}, "[]"),
			Importance: huntingAnalysisMemoryImportance(status, usefulForNext, avoidRetesting),
			Confidence: huntingAnalysisMemoryConfidence(status),
			SourceHash: memoryHash(
				"hunting_probe_analysis_v1",
				fmt.Sprint(target.ID),
				fmt.Sprint(sessionID),
				fmt.Sprint(messageID),
				fmt.Sprint(actionID),
				fmt.Sprint(runID),
				fmt.Sprint(resultID),
				candidateURL,
				status,
				httpStatus,
				classification,
			),
			Metadata: memoryJSON(map[string]interface{}{
				"ingest_version":                "hunting-probe-analysis-memory-v1",
				"source":                        "agent_chat_probe_result_analyzer",
				"chat_session_id":               sessionID,
				"chat_message_id":               messageID,
				"agent_action_id":               actionID,
				"controlled_run_id":             runID,
				"controlled_result_id":          resultID,
				"candidate_url":                 candidateURL,
				"hunting_step":                  stepNo,
				"hunting_total_steps":           totalSteps,
				"analysis_status":               status,
				"http_status":                   httpStatus,
				"classification":                classification,
				"useful_for_next_validation":    usefulForNext,
				"avoid_retesting":               avoidRetesting,
				"avoid_reason":                  avoidReason,
				"next_operator_hint":            next,
				"operator_ready":                true,
				"execution_evidence":            true,
				"should_not_promote_as_finding": status == "blocked" || status == "inconclusive",
				"analysis":                      analysis,
			}, "{}"),
		}

		if sourceID > 0 {
			item.SourceID = &sourceID
		}

		if _, _, err := upsertTargetMemoryItem(item); err != nil {
			return err
		}
	}

	return nil
}

// GetTargetAgentChatSessions lists chat sessions for a target.
func GetTargetAgentChatSessions(c *fiber.Ctx) error {
	if err := ensureAgentChatEnabled(c); err != nil {
		return err
	}

	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)

	db := database.DB.Model(&models.AgentChatSession{}).Where("target_id = ?", target.ID)
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		db = db.Where("status = ?", status)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AgentChatSession, 0)
	if err := db.Order("updated_at desc, id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"data":        rows,
		"count":       len(rows),
		"total_count": total,
		"page":        page,
	})
}

// CreateTargetAgentChatSession creates a new chat session for a target.
func CreateTargetAgentChatSession(c *fiber.Ctx) error {
	if err := ensureAgentChatEnabled(c); err != nil {
		return err
	}

	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	req := new(createAgentChatSessionRequest)
	_ = c.BodyParser(req)

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Attack Surface Chat"
	}

	row := models.AgentChatSession{
		TargetID:        target.ID,
		CreatedByUserID: &uid,
		Title:           title,
		Status:          models.AgentChatSessionStatusOpen,
		ContextJSON:     chatJSON(buildAgentChatContext(*target)),
		LastMessageAt:   &now,
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	entityID := row.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.agent_chat.session.create",
		EntityType:  "agent_chat_session",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":   target.ID,
			"root_domain": target.RootDomain,
			"title":       row.Title,
		},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": row})
}

func loadAccessibleAgentChatSession(c *fiber.Ctx) (*models.Target, *models.AgentChatSession, error) {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return nil, nil, err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return nil, nil, err
	}

	sessionID, err := parseAgentChatSessionIDParam(c)
	if err != nil {
		return nil, nil, err
	}

	var session models.AgentChatSession
	if err := database.DB.Where("id = ? AND target_id = ?", sessionID, target.ID).First(&session).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil, fiber.NewError(fiber.StatusNotFound, "chat session not found")
		}
		return nil, nil, err
	}

	return target, &session, nil
}

// GetTargetAgentChatMessages lists messages in one session.
func GetTargetAgentChatMessages(c *fiber.Ctx) error {
	if err := ensureAgentChatEnabled(c); err != nil {
		return err
	}

	_, session, err := loadAccessibleAgentChatSession(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AgentChatMessage, 0)
	if err := database.DB.
		Where("session_id = ?", session.ID).
		Order("created_at asc, id asc").
		Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   rows,
		"count":  len(rows),
	})
}

func chatSkillSelectionIntent(text string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}

	needles := []string{
		"real bug",
		"real bugs",
		"critical",
		"high impact",
		"high-impact",
		"find bugs",
		"hunt bugs",
		"bug bounty",
		"pentest",
		"exploit",
		"exploitation",
		"vulnerability",
		"vulnerabilities",
		"authorized",
		"بگرد",
		"باگ",
		"باگ واقعی",
		"کریتیکال",
		"حفره",
		"آسیب پذیری",
		"اکسپلویت",
		"نفوذ",
	}

	for _, needle := range needles {
		if strings.Contains(t, needle) {
			return true
		}
	}

	return false
}

type operatorSkillSelectorSignals struct {
	FoundURLCount          int64
	ParameterizedURLCount  int64
	ControlledResultCount  int64
	ParameterMemoryCount   int64
	NoParameterMemoryCount int64
	AuthNeedMemoryCount    int64
	HasXSSSignal           bool
	HasRedirectSignal      bool
	HasFilePathSignal      bool
	HasAuthAccessSignal    bool
	HasJSAuditSignal       bool
	SignalSummary          []string
}

func textHasAnySignal(text string, needles ...string) bool {
	t := strings.ToLower(strings.TrimSpace(text))
	if t == "" {
		return false
	}

	for _, needle := range needles {
		n := strings.ToLower(strings.TrimSpace(needle))
		if n != "" && strings.Contains(t, n) {
			return true
		}
	}

	return false
}

func collectOperatorSkillSelectorSignals(text string, targetID uint, ownerKey string, uid uint) operatorSkillSelectorSignals {
	signals := operatorSkillSelectorSignals{
		HasXSSSignal: textHasAnySignal(text,
			"xss", "reflection", "reflect", "search", "query", "dom", "script", "html injection",
			"رفلکت", "ایکس اس اس",
		),
		HasRedirectSignal: textHasAnySignal(text,
			"redirect", "open redirect", "return_url", "return url", "callback", "next=", "url=", "redirect_uri",
			"ریدایرکت",
		),
		HasFilePathSignal: textHasAnySignal(text,
			"path traversal", "lfi", "file read", "download", "upload", "export", "import", "file=", "path=", "template",
			"فایل", "دانلود", "آپلود",
		),
		HasAuthAccessSignal: textHasAnySignal(text,
			"idor", "bola", "bfla", "authorization", "access control", "auth", "account", "tenant", "role", "jwt", "oauth", "api authorization",
			"احراز", "دسترسی", "اکانت",
		),
		HasJSAuditSignal: textHasAnySignal(text,
			"js", "javascript", "dom", "spa", "api", "endpoint", "crawl", "crawling", "route",
			"جاوااسکریپت", "کراول", "اندپوینت",
		),
	}

	if targetID == 0 {
		return signals
	}

	_ = database.DB.Model(&models.FoundURL{}).
		Where("target_id = ?", targetID).
		Count(&signals.FoundURLCount).Error

	_ = database.DB.Model(&models.FoundURL{}).
		Where("target_id = ? AND value LIKE ?", targetID, "%?%").
		Count(&signals.ParameterizedURLCount).Error

	_ = database.DB.Model(&models.ControlledTestResult{}).
		Where("target_id = ? AND user_id = ? AND owner_key = ?", targetID, uid, ownerKey).
		Count(&signals.ControlledResultCount).Error

	_ = database.DB.Model(&models.TargetMemoryItem{}).
		Where("target_id = ? AND owner_key = ? AND memory_type = ?", targetID, ownerKey, models.TargetMemoryTypeParameterNote).
		Count(&signals.ParameterMemoryCount).Error

	_ = database.DB.Model(&models.TargetMemoryItem{}).
		Where("target_id = ? AND owner_key = ? AND memory_type = ? AND (summary ILIKE ? OR content ILIKE ?)",
			targetID,
			ownerKey,
			models.TargetMemoryTypeParameterNote,
			"%no query parameters%",
			"%No parameterized URLs%",
		).
		Count(&signals.NoParameterMemoryCount).Error

	_ = database.DB.Model(&models.TargetMemoryItem{}).
		Where("target_id = ? AND owner_key = ? AND (content ILIKE ? OR summary ILIKE ? OR content ILIKE ? OR summary ILIKE ?)",
			targetID,
			ownerKey,
			"%auth%",
			"%auth%",
			"%authenticated%",
			"%authenticated%",
		).
		Count(&signals.AuthNeedMemoryCount).Error

	if signals.FoundURLCount == 0 || signals.NoParameterMemoryCount > 0 {
		signals.HasJSAuditSignal = true
		signals.SignalSummary = append(signals.SignalSummary, "URL/parameter inventory is sparse; JS/API discovery can expand attack surface.")
	}

	if signals.ParameterizedURLCount > 0 || signals.ParameterMemoryCount > 0 {
		signals.SignalSummary = append(signals.SignalSummary, "Parameterized URL or parameter memory exists.")
	}

	if signals.ControlledResultCount > 0 {
		signals.SignalSummary = append(signals.SignalSummary, "Controlled runtime evidence exists for HTTP evidence analysis.")
	}

	if signals.AuthNeedMemoryCount > 0 {
		signals.HasAuthAccessSignal = true
		signals.SignalSummary = append(signals.SignalSummary, "Target memory indicates auth/authenticated context may be needed.")
	}

	if signals.HasXSSSignal {
		signals.SignalSummary = append(signals.SignalSummary, "XSS/reflection signal detected.")
	}
	if signals.HasRedirectSignal {
		signals.SignalSummary = append(signals.SignalSummary, "Redirect/url-like signal detected.")
	}
	if signals.HasFilePathSignal {
		signals.SignalSummary = append(signals.SignalSummary, "File/path-like signal detected.")
	}
	if signals.HasAuthAccessSignal {
		signals.SignalSummary = append(signals.SignalSummary, "Auth/access-control signal detected.")
	}

	return signals
}

func addOperatorSkillSeed(seeds *[]selectedSkillSeed, seen map[string]bool, slug string, reason string) {
	slug = strings.ToLower(strings.TrimSpace(slug))
	reason = strings.TrimSpace(reason)
	if slug == "" || seen[slug] {
		return
	}

	seen[slug] = true
	*seeds = append(*seeds, selectedSkillSeed{Slug: slug, Reason: reason})
}

type selectedSkillSeed struct {
	Slug   string
	Reason string
}

func selectedOperatorSkillsForChat(text string, targetID uint, ownerKey string, uid uint) []map[string]interface{} {
	if !chatSkillSelectionIntent(text) {
		return nil
	}

	signals := collectOperatorSkillSelectorSignals(text, targetID, ownerKey, uid)

	seeds := make([]selectedSkillSeed, 0, 8)
	seen := map[string]bool{}

	addOperatorSkillSeed(&seeds, seen, "parameter_inventory", "High-impact authorized hunting needs a parameter and surface inventory before class-specific validation.")
	addOperatorSkillSeed(&seeds, seen, "http_evidence_analysis", "Controlled runtime evidence must be classified before choosing authorized exploit validation skills.")

	if signals.HasJSAuditSignal {
		addOperatorSkillSeed(&seeds, seen, "js_audit", "Sparse URL/parameter inventory or JS/API/crawling signals indicate JavaScript and API route mining can expand the attack surface.")
	}

	if signals.HasAuthAccessSignal {
		addOperatorSkillSeed(&seeds, seen, "auth_context_needed", "High-impact IDOR/API authorization/access-control validation requires authenticated context and usually a second test account.")
	}

	if signals.HasXSSSignal || signals.ParameterizedURLCount > 0 {
		addOperatorSkillSeed(&seeds, seen, "xss_reflection", "Parameterized/reflection signals exist; controlled XSS reflection/context validation may be useful when policy allows.")
	}

	if signals.HasRedirectSignal {
		addOperatorSkillSeed(&seeds, seen, "open_redirect", "Redirect or URL-like signals exist; open redirect and redirect-chain validation may be useful when policy allows.")
	}

	if signals.HasFilePathSignal {
		addOperatorSkillSeed(&seeds, seen, "path_traversal_baseline", "File/path/download/template signals exist; non-destructive path traversal/file-read baseline validation may be useful when policy allows.")
	}

	slugs := make([]string, 0, len(seeds))
	reasons := map[string]string{}
	order := map[string]int{}
	for idx, seed := range seeds {
		slugs = append(slugs, seed.Slug)
		reasons[seed.Slug] = seed.Reason
		order[seed.Slug] = idx + 1
	}

	rows := make([]models.OperatorSkill, 0)
	if err := database.DB.
		Where("slug IN ? AND is_enabled = true", slugs).
		Find(&rows).Error; err != nil {
		return nil
	}

	sort.Slice(rows, func(i, j int) bool {
		return order[rows[i].Slug] < order[rows[j].Slug]
	})

	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		out = append(out, map[string]interface{}{
			"id":                     row.ID,
			"slug":                   row.Slug,
			"name":                   row.Name,
			"category":               row.Category,
			"bug_class":              row.BugClass,
			"default_risk_level":     row.DefaultRiskLevel,
			"default_safety_level":   row.DefaultSafetyLevel,
			"default_test_level":     row.DefaultTestLevel,
			"default_autonomy_level": row.DefaultAutonomyLevel,
			"permission_mode":        row.PermissionMode,
			"is_builtin":             row.IsBuiltIn,
			"is_enabled":             row.IsEnabled,
			"reason":                 reasons[row.Slug],
			"status":                 "selected",
			"selector_version":       "operator-skill-selector-evidence-aware-v1",
			"selector_signals": map[string]interface{}{
				"found_url_count":           signals.FoundURLCount,
				"parameterized_url_count":   signals.ParameterizedURLCount,
				"controlled_result_count":   signals.ControlledResultCount,
				"parameter_memory_count":    signals.ParameterMemoryCount,
				"no_parameter_memory_count": signals.NoParameterMemoryCount,
				"auth_need_memory_count":    signals.AuthNeedMemoryCount,
				"signal_summary":            signals.SignalSummary,
			},
		})
	}

	return out
}

func buildSkillExecutionText(skillExecution map[string]interface{}) string {
	if len(skillExecution) == 0 {
		return ""
	}

	parameterRaw, ok := skillExecution["parameter_inventory"]
	if !ok || parameterRaw == nil {
		return ""
	}

	parameterOutput, ok := parameterRaw.(map[string]interface{})
	if !ok {
		return ""
	}

	lines := []string{"Operator skill execution:"}

	status := strings.TrimSpace(fmt.Sprint(parameterOutput["status"]))
	runCount := strings.TrimSpace(fmt.Sprint(parameterOutput["run_count"]))
	parameterCount := strings.TrimSpace(fmt.Sprint(parameterOutput["parameter_count"]))
	foundURLSample := strings.TrimSpace(fmt.Sprint(parameterOutput["found_url_sample"]))

	if status == "" {
		status = "completed"
	}
	if runCount == "" || runCount == "<nil>" {
		runCount = "0"
	}
	if parameterCount == "" || parameterCount == "<nil>" {
		parameterCount = "0"
	}
	if foundURLSample == "" || foundURLSample == "<nil>" {
		foundURLSample = "0"
	}

	lines = append(lines, fmt.Sprintf("- Parameter Inventory: status=%s, runs=%s, sampled_urls=%s, parameters=%s", status, runCount, foundURLSample, parameterCount))

	if runsRaw, ok := parameterOutput["runs"].([]map[string]interface{}); ok && len(runsRaw) > 0 {
		run := runsRaw[0]
		observationCount := strings.TrimSpace(fmt.Sprint(run["observation_count"]))
		memoryItemID := strings.TrimSpace(fmt.Sprint(run["memory_item_id"]))
		memoryIngested := strings.TrimSpace(fmt.Sprint(run["memory_ingested"]))

		if observationCount == "" || observationCount == "<nil>" {
			observationCount = "0"
		}

		lines = append(lines, fmt.Sprintf("  Observations: %s", observationCount))
		if memoryItemID != "" && memoryItemID != "<nil>" && memoryItemID != "0" {
			lines = append(lines, fmt.Sprintf("  Memory: ingested=%s memory_item_id=%s", memoryIngested, memoryItemID))
		}

		if topRaw, ok := run["top_parameters"].([]parameterInventoryCandidate); ok && len(topRaw) > 0 {
			limit := len(topRaw)
			if limit > 5 {
				limit = 5
			}
			lines = append(lines, "  Top parameter candidates:")
			for i := 0; i < limit; i++ {
				candidate := topRaw[i]
				lines = append(lines, fmt.Sprintf("  - %s: score=%d classes=%s", candidate.Name, candidate.Score, strings.Join(candidate.BugClasses, ", ")))
			}
		}
	}

	if parameterCount == "0" {
		lines = append(lines, "  Learned: no parameterized URLs were available in the sampled URL inventory for this target at execution time.")
		lines = append(lines, "  Next: improve URL discovery/crawling/JS/API mining before class-specific parameter validation.")
	} else {
		lines = append(lines, "  Next: use observations to choose class-specific controlled validation skills for authorized exploitation workflows.")
	}

	if httpRaw, ok := skillExecution["http_evidence_analysis"]; ok && httpRaw != nil {
		if httpOutput, ok := httpRaw.(map[string]interface{}); ok {
			status := strings.TrimSpace(fmt.Sprint(httpOutput["status"]))
			runCount := strings.TrimSpace(fmt.Sprint(httpOutput["run_count"]))
			resultCount := strings.TrimSpace(fmt.Sprint(httpOutput["result_count"]))
			if status == "" || status == "<nil>" {
				status = "completed"
			}
			if runCount == "" || runCount == "<nil>" {
				runCount = "0"
			}
			if resultCount == "" || resultCount == "<nil>" {
				resultCount = "0"
			}

			lines = append(lines, fmt.Sprintf("- HTTP Evidence Analysis: status=%s, runs=%s, controlled_results=%s", status, runCount, resultCount))

			if runsRaw, ok := httpOutput["runs"].([]map[string]interface{}); ok && len(runsRaw) > 0 {
				run := runsRaw[0]
				observationCount := strings.TrimSpace(fmt.Sprint(run["observation_count"]))
				memoryItemID := strings.TrimSpace(fmt.Sprint(run["memory_item_id"]))
				memoryIngested := strings.TrimSpace(fmt.Sprint(run["memory_ingested"]))
				if observationCount == "" || observationCount == "<nil>" {
					observationCount = "0"
				}
				lines = append(lines, fmt.Sprintf("  Observations: %s", observationCount))
				if memoryItemID != "" && memoryItemID != "<nil>" && memoryItemID != "0" {
					lines = append(lines, fmt.Sprintf("  Memory: ingested=%s memory_item_id=%s", memoryIngested, memoryItemID))
				}
			}

			if resultCount == "0" {
				lines = append(lines, "  Learned: no controlled runtime HTTP evidence was available for this target at execution time.")
				lines = append(lines, "  Next: run controlled baseline probes before relying on HTTP evidence analysis.")
			} else {
				lines = append(lines, "  Learned: controlled runtime evidence has been analyzed for follow-up authorized validation decisions.")
				lines = append(lines, "  Next: use evidence observations to avoid overclaiming blocked/inconclusive results and select class-specific validation.")
			}
		}
	}

	if authRaw, ok := skillExecution["auth_context_needed"]; ok && authRaw != nil {
		if authOutput, ok := authRaw.(map[string]interface{}); ok {
			status := strings.TrimSpace(fmt.Sprint(authOutput["status"]))
			runCount := strings.TrimSpace(fmt.Sprint(authOutput["run_count"]))
			if status == "" || status == "<nil>" {
				status = "needs_context"
			}
			if runCount == "" || runCount == "<nil>" {
				runCount = "0"
			}

			lines = append(lines, fmt.Sprintf("- Auth Context Needed: status=%s, runs=%s", status, runCount))
			lines = append(lines, "  Needed: authenticated session/test credentials, preferably two accounts, role/tenant boundaries, explicit authorization, and stop conditions.")
			lines = append(lines, "  Next: ask the user for auth context before real IDOR/API authorization/business-logic validation.")
		}
	}

	return strings.Join(lines, "\n")
}

func buildSelectedOperatorSkillsText(selectedSkills []map[string]interface{}) string {
	if len(selectedSkills) == 0 {
		return ""
	}

	lines := []string{
		"Operator skills selected:",
	}

	for _, skill := range selectedSkills {
		slug, _ := skill["slug"].(string)
		name, _ := skill["name"].(string)
		reason, _ := skill["reason"].(string)

		if name == "" {
			name = slug
		}
		if reason == "" {
			reason = "Selected for this authorized pentest objective."
		}

		lines = append(lines, fmt.Sprintf("- %s (%s): %s", name, slug, reason))
	}

	return strings.Join(lines, "\n")
}

func uintFromSelectedSkill(value interface{}) uint {
	switch v := value.(type) {
	case uint:
		return v
	case int:
		if v > 0 {
			return uint(v)
		}
	case int64:
		if v > 0 {
			return uint(v)
		}
	case float64:
		if v > 0 {
			return uint(v)
		}
	}

	return 0
}

func stringFromSelectedSkill(skill map[string]interface{}, key string) string {
	if value, ok := skill[key].(string); ok {
		return strings.TrimSpace(value)
	}
	return ""
}

func intFromSelectedSkill(skill map[string]interface{}, key string, fallback int) int {
	switch value := skill[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	}

	return fallback
}

func createPlannedOperatorSkillRunsFromChat(tx *gorm.DB, uid uint, ownerKey string, target *models.Target, sessionID uint, userMessageID uint, selectedSkills []map[string]interface{}, objective string) ([]uint, error) {
	if len(selectedSkills) == 0 || target == nil {
		return nil, nil
	}

	runIDs := make([]uint, 0, len(selectedSkills))
	now := time.Now().UTC()

	for _, skill := range selectedSkills {
		slug := stringFromSelectedSkill(skill, "slug")
		if slug == "" {
			continue
		}

		name := stringFromSelectedSkill(skill, "name")
		reason := stringFromSelectedSkill(skill, "reason")
		riskLevel := stringFromSelectedSkill(skill, "default_risk_level")
		if riskLevel == "" {
			riskLevel = models.AgentActionRiskLow
		}

		skillID := uintFromSelectedSkill(skill["id"])
		var skillIDPtr *uint
		if skillID > 0 {
			skillIDPtr = &skillID
		}

		safetyLevel := intFromSelectedSkill(skill, "default_safety_level", 1)
		testLevel := intFromSelectedSkill(skill, "default_test_level", 1)
		autonomyLevel := intFromSelectedSkill(skill, "default_autonomy_level", 1)

		var registrySkill models.OperatorSkill
		if err := tx.Where("slug = ? AND is_enabled = true", slug).First(&registrySkill).Error; err == nil {
			if registrySkill.ID > 0 {
				if skillIDPtr == nil {
					skillID := registrySkill.ID
					skillIDPtr = &skillID
				}
				if name == "" {
					name = registrySkill.Name
				}
				if riskLevel == "" {
					riskLevel = registrySkill.DefaultRiskLevel
				}
				safetyLevel = registrySkill.DefaultSafetyLevel
				testLevel = registrySkill.DefaultTestLevel
				autonomyLevel = registrySkill.DefaultAutonomyLevel
			}
		}

		row := models.OperatorSkillRun{
			UserID:        uid,
			OwnerKey:      ownerKey,
			TargetID:      target.ID,
			SkillID:       skillIDPtr,
			SkillSlug:     slug,
			SkillName:     name,
			ChatSessionID: &sessionID,
			ChatMessageID: &userMessageID,
			Status:        models.OperatorSkillRunStatusPlanned,
			RiskLevel:     riskLevel,
			SafetyLevel:   safetyLevel,
			TestLevel:     testLevel,
			AutonomyLevel: autonomyLevel,

			Objective:       objective,
			SelectedBecause: reason,
			InputJSON: chatJSON(map[string]interface{}{
				"source":          "agent_chat_skill_selector_v1",
				"user_message_id": userMessageID,
				"selected_skill":  skill,
			}),
			OutputJSON: chatJSON(map[string]interface{}{
				"status": "planned",
				"next":   "Skill execution is not implemented in this patch; this run records operator routing intent.",
			}),
			ResultSummary: "Skill selected and queued as a planned operator skill run.",
			NextStep:      "Execute the skill through the v3.14 operator skill runtime in a later patch.",
			Metadata: chatJSON(map[string]interface{}{
				"created_by":    "agent_chat_skill_selector_v1",
				"selected_at":   now.Format(time.RFC3339),
				"skill_version": "v1",
			}),
		}

		// Use Select("*") so level-0 skill runs are persisted instead of being
		// replaced by DB/GORM default tags.
		if err := tx.Select("*").Create(&row).Error; err != nil {
			return runIDs, err
		}

		// PostgreSQL column defaults and GORM default tags can still result in
		// level-0 values being stored as 1 on create. Force-update numeric levels
		// with a map so zero-values are preserved.
		if err := tx.Model(&models.OperatorSkillRun{}).
			Where("id = ?", row.ID).
			Updates(map[string]interface{}{
				"safety_level":   safetyLevel,
				"test_level":     testLevel,
				"autonomy_level": autonomyLevel,
			}).Error; err != nil {
			return runIDs, err
		}

		runIDs = append(runIDs, row.ID)
	}

	return runIDs, nil
}

// CreateTargetAgentChatMessage creates a user message, assistant plan message, and proposed actions.
func CreateTargetAgentChatMessage(c *fiber.Ctx) error {
	if err := ensureAgentChatEnabled(c); err != nil {
		return err
	}

	target, session, err := loadAccessibleAgentChatSession(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	req := new(createAgentChatMessageRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body", "error": err.Error()})
	}

	content := strings.TrimSpace(req.Content)
	if content == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "message content is required"})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	chatContext := buildAgentChatContext(*target)

	_, ownerKey, _, _, ownerErr := currentAccountOwner(c)
	if ownerErr != nil {
		return ownerErr
	}

	memoryContext := buildAgentChatMemoryContext(target.ID, ownerKey)

	plannedActions, assistantText := planActionsFromChat(content)
	var operatorPlan *operator.Plan
	operatorMode := "deterministic_fallback"
	operatorError := ""

	if plan, err := operator.GeneratePlan(context.Background(), operator.PlanRequest{
		TargetID:      target.ID,
		UserID:        uid,
		OwnerKey:      ownerKey,
		UserMessage:   content,
		TargetContext: chatContext,
		MemoryContext: memoryContext,
	}); err == nil && plan != nil {
		llmActions := convertOperatorPlanActions(plan)
		llmActions = replaceLowValueActionsForHypothesisMode(content, llmActions, plan)
		if len(llmActions) > 0 {
			plannedActions = llmActions
			assistantText = buildOperatorAssistantText(plan)
			operatorPlan = plan
			operatorMode = "llm_operator_plan"
		}
	} else if err != nil {
		operatorError = err.Error()
	}

	now := time.Now().UTC()

	var userMessage models.AgentChatMessage
	var assistantMessage models.AgentChatMessage
	createdActions := make([]models.AgentAction, 0)
	reusedActionIDs := make([]uint, 0)
	reusedActionCount := 0
	selectedSkills := selectedOperatorSkillsForChat(content, target.ID, ownerKey, uid)
	selectedSkillRunIDs := make([]uint, 0, len(selectedSkills))
	if selectedSkillsText := buildSelectedOperatorSkillsText(selectedSkills); selectedSkillsText != "" {
		assistantText = strings.TrimSpace(assistantText + "\n\n" + selectedSkillsText)
	}

	txErr := database.DB.Transaction(func(tx *gorm.DB) error {
		userMessage = models.AgentChatMessage{
			SessionID:       session.ID,
			TargetID:        target.ID,
			CreatedByUserID: &uid,
			Role:            models.AgentChatMessageRoleUser,
			MessageType:     models.AgentChatMessageTypeText,
			Content:         content,
			InputJSON:       chatJSON(map[string]interface{}{"source": "user"}),
			OutputJSON:      chatJSON(map[string]interface{}{}),
		}
		if err := tx.Create(&userMessage).Error; err != nil {
			return err
		}

		var skillRunErr error
		selectedSkillRunIDs, skillRunErr = createPlannedOperatorSkillRunsFromChat(tx, uid, ownerKey, target, session.ID, userMessage.ID, selectedSkills, content)
		if skillRunErr != nil {
			return skillRunErr
		}

		for _, planned := range plannedActions {
			planned.Request.InputJSON = mergeChatActionInput(planned.Request.InputJSON, map[string]interface{}{
				"chat_session_id": session.ID,
				"user_message_id": userMessage.ID,
				"chat_reason":     planned.Reason,
				"owner_key":       ownerKey,
			})

			action, err := createActionFromChat(c, target, uid, ownerKey, planned.Request)
			if err != nil {
				return err
			}
			createdActions = append(createdActions, *action)
		}

		if validationReq := maybeBuildBugTestValidationAction(target, req.Content); validationReq != nil {
			action, err := createActionFromChat(c, target, uid, ownerKey, *validationReq)
			if err == nil && action != nil {
				createdActions = append(createdActions, *action)
			}
		}

		seenHighValueActionIDs := map[uint]bool{}
		for _, highValueReq := range maybeBuildHighValueSurfaceProbeActions(target, req.Content) {
			highValueReq.InputJSON = mergeChatActionInput(highValueReq.InputJSON, map[string]interface{}{
				"chat_session_id": session.ID,
				"user_message_id": userMessage.ID,
				"chat_reason":     "High-impact hunting mode selected this endpoint as part of the controlled baseline hunting loop.",
				"owner_key":       ownerKey,
			})
			action, err := createActionFromChat(c, target, uid, ownerKey, highValueReq)
			if err == nil && action != nil {
				if action.ID > 0 && seenHighValueActionIDs[action.ID] {
					continue
				}
				if action.ID > 0 {
					seenHighValueActionIDs[action.ID] = true
				}
				createdActions = append(createdActions, *action)
			}
		}

		actionIDs := make([]uint, 0, len(createdActions))
		for _, action := range createdActions {
			actionIDs = append(actionIDs, action.ID)
			if actionReusedForCurrentChat(action, session.ID, userMessage.ID) {
				reusedActionCount++
				reusedActionIDs = append(reusedActionIDs, action.ID)
			}
		}

		if reusedActionCount > 0 {
			assistantText = strings.TrimSpace(assistantText + "\n" + fmt.Sprintf("I reused %d existing similar approval-gated operator action(s) and linked them to this chat session instead of creating duplicates.", reusedActionCount))
		}

		assistantText = formatAssistantTextForChatReadability(assistantText)

		assistantMessage = models.AgentChatMessage{
			SessionID:       session.ID,
			TargetID:        target.ID,
			CreatedByUserID: nil,
			Role:            models.AgentChatMessageRoleAssistant,
			MessageType:     models.AgentChatMessageTypeActionPlan,
			Content:         assistantText,
			InputJSON: chatJSON(map[string]interface{}{
				"user_message_id": userMessage.ID,
				"context":         chatContext,
				"memory_context":  memoryContext,
				"operator_mode":   operatorMode,
				"operator_error":  operatorError,
			}),
			OutputJSON: chatJSON(map[string]interface{}{
				"action_ids":             actionIDs,
				"operator_mode":          operatorMode,
				"operator_plan":          operatorPlan,
				"operator_error":         operatorError,
				"llm_assisted":           operatorMode == "llm_operator_plan",
				"actions_created":        len(actionIDs) - reusedActionCount,
				"actions_reused":         reusedActionCount,
				"reused_action_ids":      reusedActionIDs,
				"selected_skills":        selectedSkills,
				"selected_skill_run_ids": selectedSkillRunIDs,
				"hunting_session":        buildHuntingSessionOutput(createdActions),
				"execution_enabled":      false,
				"guardrails": []string{
					"low-risk controlled actions may be auto-executed by Operator Autopilot when target policy permits",
					"higher-risk, active, exploit, payload, auth, rate-limit, or brute-force actions require explicit approval",
					"blocked_by_policy actions cannot be approved or executed",
				},
			}),
		}

		if err := tx.Create(&assistantMessage).Error; err != nil {
			return err
		}

		updates := map[string]interface{}{
			"last_message_at": &now,
			"updated_at":      now,
		}
		if session.Title == "Attack Surface Chat" {
			updates["title"] = shortChatTitle(content)
		}

		return tx.Model(&models.AgentChatSession{}).Where("id = ?", session.ID).Updates(updates).Error
	})

	if txErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": txErr.Error()})
	}

	skillExecution := map[string]interface{}{}
	if dispatcherOutput, err := executeSelectedOperatorSkillRunsFromChat(database.DB, uid, ownerKey, target, selectedSkillRunIDs); err != nil {
		skillExecution["dispatcher_error"] = err.Error()
	} else if dispatcherOutput != nil {
		skillExecution = dispatcherOutput
	}

	autopilot := runAgentChatAutopilotForActions(c, target, uid, ownerKey, createdActions)

	assistantOutput := map[string]interface{}{}
	if len(assistantMessage.OutputJSON) > 0 {
		_ = json.Unmarshal(assistantMessage.OutputJSON, &assistantOutput)
	}

	if len(autopilot.ExecutedActions) > 0 {
		assistantMessage.Content = strings.ReplaceAll(
			assistantMessage.Content,
			"I proposed 1 approval-gated operator action(s). Review and approve before dispatch.",
			"I routed the low-risk controlled action through Operator Autopilot instead of requiring manual approval.",
		)
		assistantMessage.Content = strings.TrimSpace(assistantMessage.Content + "\n" + fmt.Sprintf("Autopilot executed %d low-risk controlled action(s). Queued workflows will update target memory when their worker phase completes; higher-risk actions still require explicit approval.", len(autopilot.ExecutedActions)))
	}

	if operatorPlan != nil && len(operatorPlan.Hypotheses) > 0 {
		assistantOutput["hypotheses"] = operatorPlan.Hypotheses
		assistantOutput["hypothesis_count"] = len(operatorPlan.Hypotheses)
	}
	huntingSession := buildHuntingSessionOutput(createdActions)
	huntingSession = enrichHuntingSessionWithRuntimeResults(huntingSession, autopilot)
	if err := createHuntingAnalysisMemoryItems(uid, ownerKey, target, session.ID, assistantMessage.ID, huntingSession); err != nil {
		autopilot.Errors = append(autopilot.Errors, fmt.Sprintf("hunting analysis memory ingestion failed: %v", err))
	} else if active, _ := huntingSession["active"].(bool); active {
		autopilot.MemoryIngested = true
		huntingSession["analysis_memory_ingested"] = true
	}
	if runtimeAnalysisText := buildHuntingRuntimeAnalysisText(huntingSession); runtimeAnalysisText != "" {
		assistantMessage.Content = strings.TrimSpace(assistantMessage.Content + "\n\n" + runtimeAnalysisText)
		assistantMessage.Content = formatAssistantTextForChatReadability(assistantMessage.Content)
	}
	assistantOutput["selected_skills"] = selectedSkills
	assistantOutput["selected_skill_run_ids"] = selectedSkillRunIDs
	assistantOutput["skill_execution"] = skillExecution

	if skillExecutionText := buildSkillExecutionText(skillExecution); skillExecutionText != "" {
		assistantMessage.Content = strings.TrimSpace(assistantMessage.Content + "\n\n" + skillExecutionText)
		assistantMessage.Content = formatAssistantTextForChatReadability(assistantMessage.Content)
	}

	assistantOutput["hunting_session"] = huntingSession
	assistantOutput["autopilot"] = autopilot
	assistantOutput["execution_enabled"] = len(autopilot.ExecutedActions) > 0

	_ = database.DB.Model(&models.AgentChatMessage{}).
		Where("id = ?", assistantMessage.ID).
		Updates(map[string]interface{}{
			"content":     assistantMessage.Content,
			"output_json": chatJSON(assistantOutput),
		}).Error
	_ = database.DB.First(&assistantMessage, assistantMessage.ID)

	_ = maybeCreateChatMemoryFromMessage(uid, target, session.ID, userMessage, len(createdActions))
	_ = maybeCreateChatMemoryFromMessage(uid, target, session.ID, assistantMessage, len(createdActions))

	entityID := assistantMessage.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.agent_chat.message.create",
		EntityType:  "agent_chat_message",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":       target.ID,
			"root_domain":     target.RootDomain,
			"session_id":      session.ID,
			"user_message_id": userMessage.ID,
			"action_count":    len(createdActions),
			"autopilot":       true,
		},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"user_message":      userMessage,
			"assistant_message": assistantMessage,
			"proposed_actions":  createdActions,
		},
	})
}

func mergeChatActionInput(base map[string]interface{}, extra map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{})
	for k, v := range base {
		out[k] = v
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func classifyChatMemory(content string, actionCount int) (bool, string, []string, int, int, string) {
	text := strings.ToLower(strings.TrimSpace(content))
	if text == "" {
		return false, "", nil, 0, 0, ""
	}

	tags := []string{"chat", "operator"}
	importance := 65
	confidence := 70
	reason := "chat message may help future operator planning"
	memoryType := models.TargetMemoryTypeUserDecision

	hasAny := func(words ...string) bool {
		for _, word := range words {
			if strings.Contains(text, word) {
				return true
			}
		}
		return false
	}

	switch {
	case hasAny("approve", "approved", "yes", "ok", "اوکی", "تایید", "بله", "مجاز", "اجازه"):
		tags = append(tags, "approval", "decision")
		importance = 88
		confidence = 82
		reason = "user approval or authorization decision"
		memoryType = models.TargetMemoryTypeUserDecision
		return true, memoryType, tags, importance, confidence, reason

	case hasAny("reject", "rejected", "deny", "denied", "no", "نه", "رد", "اجازه نده", "انجام نده"):
		tags = append(tags, "rejection", "decision")
		importance = 88
		confidence = 82
		reason = "user rejection or denial decision"
		memoryType = models.TargetMemoryTypeUserDecision
		return true, memoryType, tags, importance, confidence, reason

	case hasAny("scope", "out of scope", "in scope", "policy", "rate limit", "destructive", "production", "lab", "محدوده", "اسکوپ", "پالیسی", "سیاست", "مخرب", "نرخ"):
		tags = append(tags, "policy", "scope", "constraint")
		importance = 92
		confidence = 86
		reason = "scope or policy constraint for future testing"
		memoryType = models.TargetMemoryTypePolicyConstraint
		return true, memoryType, tags, importance, confidence, reason

	case hasAny("test xss", "xss", "ssrf", "idor", "bola", "sqli", "sql injection", "nosql", "ssti", "path traversal", "lfi", "rfi", "file upload", "csrf", "cors", "open redirect", "takeover", "secret", "secrets", "cve", "misconfig", "باگ", "آسیب", "نفوذ"):
		tags = append(tags, "objective", "vulnerability_hypothesis")
		importance = 84
		confidence = 78
		reason = "user selected or discussed a vulnerability testing objective"
		memoryType = models.TargetMemoryTypeVulnerabilityHypothesis
		return true, memoryType, tags, importance, confidence, reason

	case hasAny("crawl", "crawling", "recon", "discover", "subdomain", "url", "endpoint", "js", "javascript", "کرال", "ساب دامین", "یوارال", "اندپوینت"):
		tags = append(tags, "recon", "crawl", "attack_surface")
		importance = 80
		confidence = 78
		reason = "recon/crawling or attack-surface expansion decision"
		memoryType = models.TargetMemoryTypeEndpointNote
		return true, memoryType, tags, importance, confidence, reason

	case hasAny("confirmed", "validated", "false positive", "not exploitable", "manual validation", "evidence", "یافتم", "تایید شد", "فالس", "ولید", "شواهد"):
		tags = append(tags, "evidence", "validation", "triage")
		importance = 86
		confidence = 82
		reason = "finding interpretation or validation signal"
		memoryType = models.TargetMemoryTypeFindingEvidence
		return true, memoryType, tags, importance, confidence, reason

	case actionCount > 0:
		tags = append(tags, "action_plan", "operator")
		importance = 74
		confidence = 72
		reason = "chat produced proposed operator actions"
		memoryType = models.TargetMemoryTypeUserDecision
		return true, memoryType, tags, importance, confidence, reason
	}

	return false, "", nil, 0, 0, ""
}

func maybeCreateChatMemoryFromMessage(userID uint, target *models.Target, sessionID uint, message models.AgentChatMessage, actionCount int) error {
	shouldRemember, memoryType, tags, importance, confidence, reason := classifyChatMemory(message.Content, actionCount)
	if !shouldRemember {
		return nil
	}

	title := "Chat memory: " + reason
	content := strings.TrimSpace(message.Content)
	if len([]rune(content)) > 2500 {
		content = string([]rune(content)[:2500]) + "..."
	}

	summary := content
	if len([]rune(summary)) > 420 {
		summary = string([]rune(summary)[:420]) + "..."
	}

	sourceID := message.ID
	item := models.TargetMemoryItem{
		UserID:     userID,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceChatMessage,
		SourceID:   &sourceID,
		MemoryType: memoryType,
		Title:      title,
		Content: strings.Join([]string{
			"Operator chat memory:",
			"Reason: " + reason,
			"Role: " + message.Role,
			"Message type: " + message.MessageType,
			"Content:",
			content,
		}, "\n"),
		Summary:    summary,
		Tags:       memoryJSON(tags, "[]"),
		Importance: importance,
		Confidence: confidence,
		SourceHash: memoryHash(
			"agent_chat_message",
			target.RootDomain,
			strconv.FormatUint(uint64(sessionID), 10),
			strconv.FormatUint(uint64(message.ID), 10),
			message.Role,
			memoryType,
			content,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"memory_hook_version": "agent-chat-memory-hook-v1",
			"chat_session_id":     sessionID,
			"chat_message_id":     message.ID,
			"role":                message.Role,
			"message_type":        message.MessageType,
			"reason":              reason,
			"action_count":        actionCount,
		}, "{}"),
	}

	_, _, err := upsertTargetMemoryItem(item)
	return err
}
