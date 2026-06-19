package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type proposeAgentActionRequest struct {
	AgentRunID       *uint                  `json:"agent_run_id"`
	ActionType       string                 `json:"action_type"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	RiskLevel        string                 `json:"risk_level"`
	SafetyLevel      int                    `json:"safety_level"`
	TestLevel        int                    `json:"test_level"`
	AutonomyLevel    int                    `json:"autonomy_level"`
	RequestedByAgent *bool                  `json:"requested_by_agent"`
	RequiresApproval *bool                  `json:"requires_approval"`
	InputJSON        map[string]interface{} `json:"input_json"`
}

type actionDecisionRequest struct {
	Reason string `json:"reason"`
}

func normalizeActionStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.AgentActionStatusProposed:
		return models.AgentActionStatusProposed
	case models.AgentActionStatusApproved:
		return models.AgentActionStatusApproved
	case models.AgentActionStatusRejected:
		return models.AgentActionStatusRejected
	case models.AgentActionStatusExecuted:
		return models.AgentActionStatusExecuted
	case models.AgentActionStatusFailed:
		return models.AgentActionStatusFailed
	case models.AgentActionStatusBlockedByPolicy:
		return models.AgentActionStatusBlockedByPolicy
	default:
		return ""
	}
}

func normalizeAgentActionType(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
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
		models.AgentActionTypeRunCommandSchema,
		models.AgentActionTypeGeneratePayload,
		models.AgentActionTypeExecutePayloadTest,
		models.AgentActionTypeRunOWASPChecklist,
		models.AgentActionTypeValidateFinding,
		models.AgentActionTypeProposeSeverityChange,
		models.AgentActionTypeApplySeverityChange,
		models.AgentActionTypeGenerateReport,
		models.AgentActionTypeSubmitReport:
		return v
	default:
		return ""
	}
}

func normalizeAgentRisk(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.AgentActionRiskLow:
		return models.AgentActionRiskLow
	case models.AgentActionRiskMedium:
		return models.AgentActionRiskMedium
	case models.AgentActionRiskHigh:
		return models.AgentActionRiskHigh
	case models.AgentActionRiskCritical:
		return models.AgentActionRiskCritical
	default:
		return models.AgentActionRiskLow
	}
}

func clampAgentLevel(value int) int {
	if value < 0 {
		return 0
	}
	if value > 5 {
		return 5
	}
	return value
}

func duplicateActionStatuses() []string {
	return []string{
		models.AgentActionStatusProposed,
		models.AgentActionStatusApproved,
		models.AgentActionStatusBlockedByPolicy,
	}
}

func findDuplicateAgentAction(targetID uint, actionType string, title string) (*models.AgentAction, error) {
	actionType = normalizeAgentActionType(actionType)
	title = strings.TrimSpace(title)
	if actionType == "" {
		return nil, nil
	}

	var row models.AgentAction
	q := database.DB.
		Where("target_id = ? AND action_type = ? AND status IN ?", targetID, actionType, duplicateActionStatuses())

	if title != "" {
		q = q.Where("LOWER(title) = LOWER(?)", title)
	}

	err := q.Order("created_at DESC, id DESC").First(&row).Error
	if err == nil {
		return &row, nil
	}
	if err == gorm.ErrRecordNotFound {
		return nil, nil
	}
	return nil, err
}

func agentActionJSON(value interface{}) datatypes.JSON {
	if value == nil {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}

func parseAgentActionIDParam(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("action_id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid agent action id")
	}
	return uint(id), nil
}

func ensureAgentActionsEnabled(c *fiber.Ctx) error {
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}
	if !featureflags.IsEnabledForOwner(ownerKey, featureflags.KeyAgentActions) {
		return featureflags.DisabledResponse(c, featureflags.KeyAgentActions)
	}
	return nil
}

func jsonStringList(value datatypes.JSON) []string {
	items := make([]string, 0)
	if len(value) == 0 {
		return items
	}

	if err := json.Unmarshal(value, &items); err == nil {
		for i := range items {
			items[i] = strings.ToLower(strings.TrimSpace(items[i]))
		}
		return items
	}

	var raw []interface{}
	if err := json.Unmarshal(value, &raw); err == nil {
		for _, item := range raw {
			text := strings.ToLower(strings.TrimSpace(fmt.Sprint(item)))
			if text != "" {
				items = append(items, text)
			}
		}
	}

	return items
}

func containsPolicyToken(items []string, tokens ...string) bool {
	for _, item := range items {
		item = strings.ToLower(strings.TrimSpace(item))
		if item == "" {
			continue
		}
		for _, token := range tokens {
			token = strings.ToLower(strings.TrimSpace(token))
			if token != "" && (item == token || strings.Contains(item, token)) {
				return true
			}
		}
	}
	return false
}

func intensityLevel(value string) int {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "passive", "low", "level0", "level_0", "0":
		return 0
	case "safe", "safe_active", "level1", "level_1", "1":
		return 1
	case "balanced", "controlled", "controlled_active", "level2", "level_2", "2":
		return 2
	case "aggressive", "exploit_validation", "manual", "manual_approved", "level3", "level_3", "3":
		return 3
	default:
		return 1
	}
}

func actionClass(actionType string) string {
	switch normalizeAgentActionType(actionType) {
	case models.AgentActionTypeRunOWASPChecklist,
		models.AgentActionTypeReviewEndpoint,
		models.AgentActionTypeProposeSeverityChange,
		models.AgentActionTypeGenerateReport:
		return "advisory"
	case models.AgentActionTypeRunCrawling,
		models.AgentActionTypeRunJSIntelligence:
		return "passive_recon"
	case models.AgentActionTypeRunNucleiProfile,
		models.AgentActionTypeGenerateNucleiDraft:
		return "template_scan"
	case models.AgentActionTypeRunSafeBugTests:
		return "safe_bug_test"
	case models.AgentActionTypeReviewBugTestResults:
		return "bug_test_review"
	case models.AgentActionTypePromoteBugTestResults:
		return "finding_promotion"
	case models.AgentActionTypeInspectBugPatterns,
		models.AgentActionTypeInspectBugPayloads:
		return "registry_inspection"
	case models.AgentActionTypeDeepScanAsset:
		return "deep_scan"
	case models.AgentActionTypeRunCommandSchema:
		return "command_execution"
	case models.AgentActionTypeGeneratePayload:
		return "payload_generation"
	case models.AgentActionTypeExecutePayloadTest:
		return "payload_execution"
	case models.AgentActionTypeValidateFinding:
		return "finding_validation"
	case models.AgentActionTypeApplySeverityChange:
		return "severity_apply"
	case models.AgentActionTypeSubmitReport:
		return "report_submission"
	default:
		return "unknown"
	}
}

func actionPolicyTokens(actionType string) []string {
	switch actionClass(actionType) {
	case "advisory":
		return []string{"advisory", "passive", "owasp", "report", "severity_review"}
	case "passive_recon":
		return []string{"passive", "recon", "crawl", "js", "javascript"}
	case "template_scan":
		return []string{"nuclei", "template", "scan", "safe_active"}
	case "safe_bug_test":
		return []string{"safe_bug_tests", "xss", "cors", "open_redirect", "security_headers", "safe_active"}
	case "deep_scan":
		return []string{"deep_scan", "scan", "controlled_active"}
	case "command_execution":
		return []string{"command", "command_execution"}
	case "payload_generation":
		return []string{"payload", "payload_generation"}
	case "payload_execution":
		return []string{"payload_execution", "exploit_validation"}
	case "finding_validation":
		return []string{"finding_validation", "validation"}
	case "severity_apply":
		return []string{"severity_apply", "severity_change"}
	case "report_submission":
		return []string{"report_submission", "submit_report"}
	default:
		return []string{actionType}
	}
}

func policyCheckForAgentAction(target *models.Target, req proposeAgentActionRequest) (string, string, map[string]interface{}) {
	actionType := normalizeAgentActionType(req.ActionType)
	risk := normalizeAgentRisk(req.RiskLevel)
	safetyLevel := clampAgentLevel(req.SafetyLevel)
	testLevel := clampAgentLevel(req.TestLevel)
	class := actionClass(actionType)
	tokens := actionPolicyTokens(actionType)

	var policy models.TargetPolicy
	hasPolicy := database.DB.Where("target_id = ?", target.ID).First(&policy).Error == nil

	status := models.AgentActionPolicyStatusAllowed
	reason := "action is allowed by baseline guardrails"
	requiresApproval := true

	blockedReasons := make([]string, 0)
	warningReasons := make([]string, 0)
	requiredControls := make([]string, 0)

	allowedTypes := make([]string, 0)
	disallowedTypes := make([]string, 0)
	maxIntensity := "safe"
	maxIntensityLevel := 1

	if hasPolicy {
		allowedTypes = jsonStringList(policy.AllowedTestTypes)
		disallowedTypes = jsonStringList(policy.DisallowedTestTypes)
		maxIntensity = strings.ToLower(strings.TrimSpace(policy.MaxTestIntensity))
		maxIntensityLevel = intensityLevel(maxIntensity)

		if len(disallowedTypes) > 0 && containsPolicyToken(disallowedTypes, tokens...) {
			blockedReasons = append(blockedReasons, "action class is explicitly disallowed by target policy")
		}

		if len(allowedTypes) > 0 && !containsPolicyToken(allowedTypes, tokens...) {
			warningReasons = append(warningReasons, "action class is not explicitly listed in allowed_test_types")
		}

		if testLevel > maxIntensityLevel || safetyLevel > maxIntensityLevel {
			blockedReasons = append(blockedReasons, fmt.Sprintf("action level exceeds target max_test_intensity=%s", maxIntensity))
		}

		if policy.AuthRequired && (class == "deep_scan" || class == "safe_bug_test" || class == "payload_execution") {
			warningReasons = append(warningReasons, "target policy indicates auth_required; verify authorization/session handling before execution")
		}

		if strings.TrimSpace(policy.RateLimitNotes) != "" {
			warningReasons = append(warningReasons, "target policy contains rate limit notes that must be respected")
		}
	} else {
		warningReasons = append(warningReasons, "target policy is missing; manual review is required before execution")
	}

	if risk == models.AgentActionRiskHigh || risk == models.AgentActionRiskCritical {
		blockedReasons = append(blockedReasons, "high or critical risk action is blocked by default")
	}

	if testLevel >= 3 || safetyLevel >= 3 {
		blockedReasons = append(blockedReasons, "level 3 exploit-validation/manual-approved action is blocked in v3.7.0 foundation")
	}

	switch class {
	case "command_execution":
		requiredControls = append(requiredControls, "allow_command_execution")
		warningReasons = append(warningReasons, "command execution requires future explicit account and policy controls")
	case "payload_generation":
		requiredControls = append(requiredControls, "allow_payload_generation")
		warningReasons = append(warningReasons, "payload generation requires future explicit account and policy controls")
	case "payload_execution":
		requiredControls = append(requiredControls, "allow_payload_execution", "can_run_exploit_validation")
		blockedReasons = append(blockedReasons, "payload execution is blocked until controlled exploit validation is implemented")
	case "severity_apply":
		requiredControls = append(requiredControls, "allow_severity_auto_update")
		warningReasons = append(warningReasons, "severity changes require evidence thresholds and explicit approval")
	case "report_submission":
		requiredControls = append(requiredControls, "allow_report_auto_submit")
		blockedReasons = append(blockedReasons, "automatic report submission is blocked until submission controls are implemented")
	}

	if len(blockedReasons) > 0 {
		status = models.AgentActionPolicyStatusBlocked
		reason = blockedReasons[0]
	} else if len(warningReasons) > 0 {
		status = models.AgentActionPolicyStatusWarning
		reason = warningReasons[0]
	}

	check := map[string]interface{}{
		"target_id":             target.ID,
		"root_domain":           target.RootDomain,
		"has_target_policy":     hasPolicy,
		"action_type":           actionType,
		"action_class":          class,
		"policy_tokens":         tokens,
		"risk_level":            risk,
		"safety_level":          safetyLevel,
		"test_level":            testLevel,
		"requires_approval":     requiresApproval,
		"policy_status":         status,
		"reason":                reason,
		"blocked_reasons":       blockedReasons,
		"warning_reasons":       warningReasons,
		"required_controls":     requiredControls,
		"execution_enabled":     false,
		"foundation_version":    "agent-actions-policy-v2",
		"max_test_intensity":    maxIntensity,
		"max_intensity_level":   maxIntensityLevel,
		"allowed_test_types":    allowedTypes,
		"disallowed_test_types": disallowedTypes,
		"autonomy_controls": map[string]interface{}{
			"allow_command_execution":    false,
			"allow_payload_generation":   false,
			"allow_payload_execution":    false,
			"allow_severity_auto_update": false,
			"allow_report_auto_submit":   false,
			"can_run_exploit_validation": false,
			"max_test_level":             maxIntensityLevel,
		},
		"guardrails": []string{
			"v3.7.0 action execution is not enabled in this foundation step",
			"blocked_by_policy actions cannot be approved",
			"policy max_test_intensity is enforced against safety_level and test_level",
			"disallowed_test_types takes precedence over allowed_test_types",
			"high-risk, destructive, out-of-scope, brute force, DoS, and data extraction actions remain blocked",
			"future execution must be scope-checked, approval-gated, rate-limited, and audited",
		},
	}

	if hasPolicy {
		check["auth_required"] = policy.AuthRequired
		check["safe_testing_notes"] = policy.SafeTestingNotes
		check["rate_limit_notes"] = policy.RateLimitNotes
		check["reporting_preferences"] = policy.ReportingPreferences
	}

	return status, reason, check
}

// GetTargetAgentActions lists proposed/approved/rejected agent actions.
func GetTargetAgentActions(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	if err := ensureAgentActionsEnabled(c); err != nil {
		return err
	}

	_, ownerKey, _, _, ownerErr := currentAccountOwner(c)
	if ownerErr != nil {
		return ownerErr
	}

	limit, offset, page := parseDataLayerPagination(c)

	db := database.DB.Model(&models.AgentAction{}).Where("target_id = ? AND owner_key = ?", target.ID, ownerKey)

	if status := normalizeActionStatus(c.Query("status")); status != "" {
		db = db.Where("status = ?", status)
	}
	if actionType := normalizeAgentActionType(c.Query("action_type")); actionType != "" {
		db = db.Where("action_type = ?", actionType)
	}
	if policyStatus := strings.ToLower(strings.TrimSpace(c.Query("policy_status"))); policyStatus != "" {
		db = db.Where("policy_status = ?", policyStatus)
	}
	if riskLevel := strings.ToLower(strings.TrimSpace(c.Query("risk_level"))); riskLevel != "" {
		db = db.Where("risk_level = ?", riskLevel)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AgentAction, 0)
	if err := db.Order("created_at desc, id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
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

// ProposeTargetAgentAction creates a proposed or policy-blocked agent action.
func ProposeTargetAgentAction(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	if err := ensureAgentActionsEnabled(c); err != nil {
		return err
	}

	req := new(proposeAgentActionRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body", "error": err.Error()})
	}

	actionType := normalizeAgentActionType(req.ActionType)
	if actionType == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid or unsupported action_type"})
	}

	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = actionType
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	duplicate, duplicateErr := findDuplicateAgentAction(target.ID, actionType, title)
	if duplicateErr != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": duplicateErr.Error()})
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
				"action_type":   duplicate.ActionType,
				"status":        duplicate.Status,
				"policy_status": duplicate.PolicyStatus,
				"risk_level":    duplicate.RiskLevel,
				"duplicate":     true,
			},
		})

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":    "success",
			"duplicate": true,
			"message":   "similar active agent action already exists",
			"data":      duplicate,
		})
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

	policyStatus, policyReason, policyCheck := policyCheckForAgentAction(target, *req)

	status := models.AgentActionStatusProposed
	if policyStatus == models.AgentActionPolicyStatusBlocked {
		status = models.AgentActionStatusBlockedByPolicy
		requiresApproval = true
	}

	row := models.AgentAction{
		TargetID:         target.ID,
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
		ErrorMessage:     "",
		RejectReason:     "",
	}

	if policyStatus == models.AgentActionPolicyStatusBlocked {
		row.ErrorMessage = policyReason
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
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
			"action_type":   row.ActionType,
			"status":        row.Status,
			"policy_status": row.PolicyStatus,
			"risk_level":    row.RiskLevel,
			"safety_level":  row.SafetyLevel,
			"test_level":    row.TestLevel,
		},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": row})
}

func loadAccessibleAgentAction(c *fiber.Ctx) (models.Target, models.AgentAction, error) {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return models.Target{}, models.AgentAction{}, err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return models.Target{}, models.AgentAction{}, err
	}

	actionID, err := parseAgentActionIDParam(c)
	if err != nil {
		return models.Target{}, models.AgentAction{}, err
	}

	var row models.AgentAction
	if err := database.DB.Where("id = ? AND target_id = ?", actionID, target.ID).First(&row).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return models.Target{}, models.AgentAction{}, fiber.NewError(fiber.StatusNotFound, "agent action not found")
		}
		return models.Target{}, models.AgentAction{}, err
	}

	return *target, row, nil
}

// ApproveTargetAgentAction marks a proposed/warning/allowed action as approved.
// Execution is intentionally not performed in this v3.7.0 data-layer step.
func ApproveTargetAgentAction(c *fiber.Ctx) error {
	if err := ensureAgentActionsEnabled(c); err != nil {
		return err
	}

	target, row, err := loadAccessibleAgentAction(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if row.Status == models.AgentActionStatusBlockedByPolicy || row.PolicyStatus == models.AgentActionPolicyStatusBlocked {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "blocked_by_policy actions cannot be approved"})
	}

	if row.Status != models.AgentActionStatusProposed {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "only proposed actions can be approved"})
	}

	req := new(actionDecisionRequest)
	_ = c.BodyParser(req)

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	snapshot := map[string]interface{}{
		"action_id":     row.ID,
		"old_status":    row.Status,
		"new_status":    models.AgentActionStatusApproved,
		"policy_status": row.PolicyStatus,
		"risk_level":    row.RiskLevel,
		"safety_level":  row.SafetyLevel,
		"test_level":    row.TestLevel,
		"reason":        strings.TrimSpace(req.Reason),
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AgentAction{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
			"status":              models.AgentActionStatusApproved,
			"approved_by_user_id": uid,
			"approved_at":         &now,
		}).Error; err != nil {
			return err
		}

		return tx.Create(&models.AgentActionApproval{
			ActionID:     row.ID,
			UserID:       uid,
			Decision:     models.AgentActionApprovalDecisionApproved,
			Reason:       strings.TrimSpace(req.Reason),
			SnapshotJSON: agentActionJSON(snapshot),
		}).Error
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = database.DB.First(&row, row.ID)

	entityID := row.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.agent_action.approve",
		EntityType:  "agent_action",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata:    snapshot,
	})

	return c.JSON(fiber.Map{"status": "success", "data": row})
}

// RejectTargetAgentAction rejects a proposed/approved action before execution.
func RejectTargetAgentAction(c *fiber.Ctx) error {
	if err := ensureAgentActionsEnabled(c); err != nil {
		return err
	}

	target, row, err := loadAccessibleAgentAction(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if row.Status == models.AgentActionStatusExecuted {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "executed actions cannot be rejected"})
	}

	req := new(actionDecisionRequest)
	_ = c.BodyParser(req)

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	reason := strings.TrimSpace(req.Reason)
	snapshot := map[string]interface{}{
		"action_id":     row.ID,
		"old_status":    row.Status,
		"new_status":    models.AgentActionStatusRejected,
		"policy_status": row.PolicyStatus,
		"risk_level":    row.RiskLevel,
		"reason":        reason,
	}

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.AgentAction{}).Where("id = ?", row.ID).Updates(map[string]interface{}{
			"status":        models.AgentActionStatusRejected,
			"rejected_at":   &now,
			"reject_reason": reason,
		}).Error; err != nil {
			return err
		}

		return tx.Create(&models.AgentActionApproval{
			ActionID:     row.ID,
			UserID:       uid,
			Decision:     models.AgentActionApprovalDecisionRejected,
			Reason:       reason,
			SnapshotJSON: agentActionJSON(snapshot),
		}).Error
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = database.DB.First(&row, row.ID)

	entityID := row.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.agent_action.reject",
		EntityType:  "agent_action",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata:    snapshot,
	})

	return c.JSON(fiber.Map{"status": "success", "data": row})
}
