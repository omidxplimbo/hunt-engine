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

func policyCheckForAgentAction(target *models.Target, req proposeAgentActionRequest) (string, string, map[string]interface{}) {
	actionType := normalizeAgentActionType(req.ActionType)
	risk := normalizeAgentRisk(req.RiskLevel)
	safetyLevel := clampAgentLevel(req.SafetyLevel)
	testLevel := clampAgentLevel(req.TestLevel)

	var policy models.TargetPolicy
	hasPolicy := database.DB.Where("target_id = ?", target.ID).First(&policy).Error == nil

	status := models.AgentActionPolicyStatusAllowed
	reason := "action is allowed by baseline guardrails"
	requiresApproval := true

	if !hasPolicy {
		status = models.AgentActionPolicyStatusWarning
		reason = "target policy is missing; manual review is required before execution"
	}

	if risk == models.AgentActionRiskHigh || risk == models.AgentActionRiskCritical || testLevel >= 3 || safetyLevel >= 3 {
		status = models.AgentActionPolicyStatusBlocked
		reason = "high-risk or exploit-validation action is blocked by default in v3.7.0 foundation"
	}

	if actionType == models.AgentActionTypeSubmitReport {
		status = models.AgentActionPolicyStatusBlocked
		reason = "automatic report submission is blocked until report submission controls are implemented"
	}

	if actionType == models.AgentActionTypeApplySeverityChange {
		status = models.AgentActionPolicyStatusWarning
		reason = "severity changes require explicit human approval and evidence thresholds"
	}

	if actionType == models.AgentActionTypeRunCommandSchema || actionType == models.AgentActionTypeExecutePayloadTest || actionType == models.AgentActionTypeGeneratePayload {
		if status == models.AgentActionPolicyStatusAllowed {
			status = models.AgentActionPolicyStatusWarning
			reason = "command/payload actions require policy review and approval before execution"
		}
	}

	check := map[string]interface{}{
		"target_id":          target.ID,
		"root_domain":        target.RootDomain,
		"has_target_policy":  hasPolicy,
		"action_type":        actionType,
		"risk_level":         risk,
		"safety_level":       safetyLevel,
		"test_level":         testLevel,
		"requires_approval":  requiresApproval,
		"policy_status":      status,
		"reason":             reason,
		"execution_enabled":  false,
		"foundation_version": "agent-actions-v1",
		"guardrails": []string{
			"v3.7.0 action execution is not enabled in this foundation step",
			"blocked_by_policy actions cannot be approved",
			"high-risk, destructive, out-of-scope, brute force, DoS, and data extraction actions must remain blocked",
			"future execution must be scope-checked, approval-gated, rate-limited, and audited",
		},
	}

	if hasPolicy {
		check["max_test_intensity"] = policy.MaxTestIntensity
		check["auth_required"] = policy.AuthRequired
		check["safe_testing_notes"] = policy.SafeTestingNotes
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

	limit, offset, page := parseDataLayerPagination(c)

	db := database.DB.Model(&models.AgentAction{}).Where("target_id = ?", target.ID)

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
