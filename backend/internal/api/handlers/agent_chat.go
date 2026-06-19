package handlers

import (
	"context"
	"encoding/json"
	"fmt"
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
		"context_version": "agent-chat-memory-context-v1",
		"memory_count":    len(memories),
		"memories":        memories,
		"chunks":          chunkOut,
	}
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

func createActionFromChat(c *fiber.Ctx, target *models.Target, uid uint, ownerKey string, req proposeAgentActionRequest) (*models.AgentAction, error) {
	actionType := normalizeAgentActionType(req.ActionType)
	if actionType == "" {
		return nil, fmt.Errorf("invalid action type")
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = actionType
	}

	duplicate, duplicateErr := findDuplicateAgentAction(target.ID, actionType, title)
	if duplicateErr != nil {
		return nil, duplicateErr
	}
	if duplicate != nil {
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
				"target_id":     target.ID,
				"root_domain":   target.RootDomain,
				"source":        "agent_chat",
				"action_type":   duplicate.ActionType,
				"status":        duplicate.Status,
				"policy_status": duplicate.PolicyStatus,
				"risk_level":    duplicate.RiskLevel,
				"duplicate":     true,
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

		actionIDs := make([]uint, 0, len(createdActions))
		for _, action := range createdActions {
			actionIDs = append(actionIDs, action.ID)
		}

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
				"action_ids":        actionIDs,
				"operator_mode":     operatorMode,
				"operator_plan":     operatorPlan,
				"operator_error":    operatorError,
				"llm_assisted":      operatorMode == "llm_operator_plan",
				"actions_created":   len(actionIDs),
				"execution_enabled": false,
				"guardrails": []string{
					"chat agent creates approval-gated action proposals only",
					"v3.11.0 operator actions are approval-gated; direct chat execution remains disabled",
					"blocked_by_policy actions cannot be approved",
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
