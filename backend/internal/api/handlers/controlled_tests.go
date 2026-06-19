package handlers

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
)

type createControlledTestRunRequest struct {
	AgentActionID *uint                  `json:"agent_action_id"`
	RuntimeType   string                 `json:"runtime_type"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	RiskLevel     string                 `json:"risk_level"`
	SafetyLevel   int                    `json:"safety_level"`
	TestLevel     int                    `json:"test_level"`
	AutonomyLevel int                    `json:"autonomy_level"`
	InputJSON     map[string]interface{} `json:"input_json"`
}

func normalizeControlledRuntimeType(value string) string {
	switch strings.TrimSpace(value) {
	case models.ControlledTestRuntimeHTTPProbe:
		return models.ControlledTestRuntimeHTTPProbe
	case models.ControlledTestRuntimeOWASPValidation:
		return models.ControlledTestRuntimeOWASPValidation
	case models.ControlledTestRuntimeAuthRateLimit:
		return models.ControlledTestRuntimeAuthRateLimit
	case models.ControlledTestRuntimeControlledBruteForce:
		return models.ControlledTestRuntimeControlledBruteForce
	case models.ControlledTestRuntimeExploitValidation:
		return models.ControlledTestRuntimeExploitValidation
	default:
		return models.ControlledTestRuntimeHTTPProbe
	}
}

func normalizeControlledRunStatus(value string) string {
	switch strings.TrimSpace(value) {
	case models.ControlledTestRunStatusQueued:
		return models.ControlledTestRunStatusQueued
	case models.ControlledTestRunStatusRunning:
		return models.ControlledTestRunStatusRunning
	case models.ControlledTestRunStatusCompleted:
		return models.ControlledTestRunStatusCompleted
	case models.ControlledTestRunStatusFailed:
		return models.ControlledTestRunStatusFailed
	case models.ControlledTestRunStatusBlockedByPolicy:
		return models.ControlledTestRunStatusBlockedByPolicy
	case models.ControlledTestRunStatusCancelled:
		return models.ControlledTestRunStatusCancelled
	default:
		return ""
	}
}

func normalizeControlledResultStatus(value string) string {
	switch strings.TrimSpace(value) {
	case models.ControlledTestResultStatusPassed:
		return models.ControlledTestResultStatusPassed
	case models.ControlledTestResultStatusFailed:
		return models.ControlledTestResultStatusFailed
	case models.ControlledTestResultStatusVulnerable:
		return models.ControlledTestResultStatusVulnerable
	case models.ControlledTestResultStatusInconclusive:
		return models.ControlledTestResultStatusInconclusive
	case models.ControlledTestResultStatusBlocked:
		return models.ControlledTestResultStatusBlocked
	case models.ControlledTestResultStatusNeedsManualReview:
		return models.ControlledTestResultStatusNeedsManualReview
	default:
		return ""
	}
}

func controlledJSON(value interface{}, fallback string) datatypes.JSON {
	if value == nil {
		return datatypes.JSON([]byte(fallback))
	}
	data, err := json.Marshal(value)
	if err != nil || len(data) == 0 {
		return datatypes.JSON([]byte(fallback))
	}
	return datatypes.JSON(data)
}

func controlledTestOwner(c *fiber.Ctx, targetID uint) (models.User, *models.Target, string, error) {
	user, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return models.User{}, nil, "", err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return models.User{}, nil, "", err
	}

	return user, target, ownerKey, nil
}

func parseControlledRunIDParam(c *fiber.Ctx) (uint, error) {
	return parseUintParam(c, "run_id")
}

func GetTargetControlledTestRuns(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	_, target, ownerKey, err := controlledTestOwner(c, targetID)
	if err != nil {
		return err
	}

	limit := c.QueryInt("limit", 30)
	if limit <= 0 || limit > 200 {
		limit = 30
	}

	status := normalizeControlledRunStatus(c.Query("status"))
	runtimeType := normalizeControlledRuntimeType(c.Query("runtime_type"))

	q := database.DB.
		Model(&models.ControlledTestRun{}).
		Where("target_id = ? AND owner_key = ?", target.ID, ownerKey)

	if status != "" {
		q = q.Where("status = ?", status)
	}
	if strings.TrimSpace(c.Query("runtime_type")) != "" {
		q = q.Where("runtime_type = ?", runtimeType)
	}

	rows := make([]models.ControlledTestRun, 0)
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return err
	}

	var total int64
	_ = database.DB.Model(&models.ControlledTestRun{}).
		Where("target_id = ? AND owner_key = ?", target.ID, ownerKey).
		Count(&total).Error

	return c.JSON(fiber.Map{
		"status":      "ok",
		"data":        rows,
		"count":       len(rows),
		"total_count": total,
		"limit":       limit,
	})
}

func GetTargetControlledTestRun(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	_, target, ownerKey, err := controlledTestOwner(c, targetID)
	if err != nil {
		return err
	}

	runID, err := parseControlledRunIDParam(c)
	if err != nil {
		return err
	}

	var row models.ControlledTestRun
	if err := database.DB.
		Where("id = ? AND target_id = ? AND owner_key = ?", runID, target.ID, ownerKey).
		First(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "controlled test run not found")
	}

	results := make([]models.ControlledTestResult, 0)
	_ = database.DB.
		Where("run_id = ? AND target_id = ? AND owner_key = ?", row.ID, target.ID, ownerKey).
		Order("created_at ASC").
		Find(&results).Error

	events := make([]models.ControlledTestEvent, 0)
	_ = database.DB.
		Where("run_id = ? AND target_id = ? AND owner_key = ?", row.ID, target.ID, ownerKey).
		Order("created_at ASC").
		Limit(100).
		Find(&events).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"run":     row,
			"results": results,
			"events":  events,
		},
	})
}

func GetTargetControlledTestResults(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	_, target, ownerKey, err := controlledTestOwner(c, targetID)
	if err != nil {
		return err
	}

	limit := c.QueryInt("limit", 50)
	if limit <= 0 || limit > 300 {
		limit = 50
	}

	status := normalizeControlledResultStatus(c.Query("status"))
	runID, _ := parseControlledRunIDParam(c)

	q := database.DB.
		Model(&models.ControlledTestResult{}).
		Where("target_id = ? AND owner_key = ?", target.ID, ownerKey)

	if status != "" {
		q = q.Where("status = ?", status)
	}
	if runID > 0 {
		q = q.Where("run_id = ?", runID)
	}

	if bugType := strings.TrimSpace(c.Query("bug_type")); bugType != "" {
		q = q.Where("bug_type = ?", bugType)
	}

	rows := make([]models.ControlledTestResult, 0)
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return err
	}

	var total int64
	_ = database.DB.Model(&models.ControlledTestResult{}).
		Where("target_id = ? AND owner_key = ?", target.ID, ownerKey).
		Count(&total).Error

	return c.JSON(fiber.Map{
		"status":      "ok",
		"data":        rows,
		"count":       len(rows),
		"total_count": total,
		"limit":       limit,
	})
}

func CreateTargetControlledTestRun(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, target, ownerKey, err := controlledTestOwner(c, targetID)
	if err != nil {
		return err
	}

	req := new(createControlledTestRunRequest)
	if err := c.BodyParser(req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	runtimeType := normalizeControlledRuntimeType(req.RuntimeType)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = "Controlled " + strings.ReplaceAll(runtimeType, "_", " ") + " run"
	}

	riskLevel := strings.TrimSpace(req.RiskLevel)
	if riskLevel == "" {
		riskLevel = models.AgentActionRiskMedium
	}

	input := req.InputJSON
	if input == nil {
		input = map[string]interface{}{}
	}
	input["source"] = "controlled_test_runtime_api"
	input["runtime_type"] = runtimeType
	input["owner_key"] = ownerKey

	policy := map[string]interface{}{
		"runtime_version":   "controlled-test-runtime-v1",
		"execution_enabled": false,
		"target_id":         target.ID,
		"owner_key":         ownerKey,
		"runtime_type":      runtimeType,
		"approval_required": true,
		"policy_status":     models.AgentActionPolicyStatusWarning,
		"reason":            "runtime data layer created; real execution will be enabled through approved controlled operator runtime",
		"required_controls": []string{
			"scope_check",
			"target_policy_check",
			"explicit_approval",
			"rate_limit",
			"audit_log",
			"evidence_capture",
		},
	}

	now := time.Now()
	row := models.ControlledTestRun{
		UserID:          user.ID,
		OwnerKey:        ownerKey,
		TargetID:        target.ID,
		AgentActionID:   req.AgentActionID,
		RuntimeType:     runtimeType,
		Title:           title,
		Description:     strings.TrimSpace(req.Description),
		Status:          models.ControlledTestRunStatusQueued,
		PolicyStatus:    models.AgentActionPolicyStatusWarning,
		RiskLevel:       riskLevel,
		SafetyLevel:     req.SafetyLevel,
		TestLevel:       req.TestLevel,
		AutonomyLevel:   req.AutonomyLevel,
		StartedAt:       nil,
		CompletedAt:     nil,
		InputJSON:       controlledJSON(input, "{}"),
		PolicyCheckJSON: controlledJSON(policy, "{}"),
		OutputJSON: controlledJSON(map[string]interface{}{
			"created_at":        now.Format(time.RFC3339),
			"execution_enabled": false,
			"next_step":         "dispatch through controlled runtime once policy, scope, and approval gates are implemented",
		}, "{}"),
	}

	if row.AgentActionID != nil {
		var action models.AgentAction
		if err := database.DB.
			Where("id = ? AND target_id = ? AND owner_key = ?", *row.AgentActionID, target.ID, ownerKey).
			First(&action).Error; err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "agent action not found for target owner scope")
		}
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return err
	}

	event := models.ControlledTestEvent{
		UserID:    user.ID,
		OwnerKey:  ownerKey,
		TargetID:  target.ID,
		RunID:     &row.ID,
		EventType: models.ControlledTestEventCreated,
		AfterJSON: controlledJSON(map[string]interface{}{
			"id":           row.ID,
			"runtime_type": row.RuntimeType,
			"status":       row.Status,
		}, "{}"),
		Metadata: controlledJSON(map[string]interface{}{
			"source":            "controlled_test_runtime_api",
			"execution_enabled": false,
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "ok",
		"data":   row,
	})
}

// controlledRuntimeTypeForAgentAction maps approved Agent Actions to controlled runtime types.
// It creates runtime jobs, not direct chat execution.
func controlledRuntimeTypeForAgentAction(actionType string) string {
	switch normalizeAgentActionType(actionType) {
	case models.AgentActionTypeRunOWASPChecklist:
		return models.ControlledTestRuntimeOWASPValidation
	case models.AgentActionTypeValidateFinding:
		return models.ControlledTestRuntimeExploitValidation
	case models.AgentActionTypeReviewEndpoint, models.AgentActionTypeDeepScanAsset:
		return models.ControlledTestRuntimeHTTPProbe
	default:
		return ""
	}
}

// CreateControlledTestRunFromAgentAction converts an approved AgentAction into a queued
// controlled runtime run. Real HTTP/probe/exploit execution is intentionally not performed here;
// execution will be handled by the controlled runtime worker in later v3.12 steps.
func CreateControlledTestRunFromAgentAction(target *models.Target, action *models.AgentAction, uid uint) (*models.ControlledTestRun, error) {
	if target == nil || action == nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "target and action are required")
	}

	runtimeType := controlledRuntimeTypeForAgentAction(action.ActionType)
	if runtimeType == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "agent action is not controlled-runtime compatible")
	}

	if action.OwnerKey == "" {
		return nil, fiber.NewError(fiber.StatusBadRequest, "agent action owner scope is missing")
	}

	input := map[string]interface{}{}
	if len(action.InputJSON) > 0 {
		_ = json.Unmarshal(action.InputJSON, &input)
	}
	input["source"] = "agent_action_dispatch"
	input["runtime_type"] = runtimeType
	input["owner_key"] = action.OwnerKey
	input["agent_action_id"] = action.ID
	input["action_type"] = action.ActionType

	policyCheck := map[string]interface{}{}
	if len(action.PolicyCheckJSON) > 0 {
		_ = json.Unmarshal(action.PolicyCheckJSON, &policyCheck)
	}
	policyCheck["runtime_version"] = "controlled-test-runtime-v1"
	policyCheck["controlled_runtime_created"] = true
	policyCheck["execution_enabled"] = false
	policyCheck["real_execution_pending"] = true
	policyCheck["reason"] = "approved action converted into queued controlled runtime run; worker execution will be enabled in later v3.12 steps"
	policyCheck["required_controls"] = []string{
		"scope_check",
		"target_policy_check",
		"explicit_approval",
		"rate_limit",
		"audit_log",
		"evidence_capture",
	}

	now := time.Now().UTC()
	run := models.ControlledTestRun{
		UserID:          uid,
		OwnerKey:        action.OwnerKey,
		TargetID:        target.ID,
		AgentActionID:   &action.ID,
		RuntimeType:     runtimeType,
		Title:           action.Title,
		Description:     action.Description,
		Status:          models.ControlledTestRunStatusQueued,
		PolicyStatus:    action.PolicyStatus,
		RiskLevel:       action.RiskLevel,
		SafetyLevel:     action.SafetyLevel,
		TestLevel:       action.TestLevel,
		AutonomyLevel:   action.AutonomyLevel,
		InputJSON:       controlledJSON(input, "{}"),
		PolicyCheckJSON: controlledJSON(policyCheck, "{}"),
		OutputJSON: controlledJSON(map[string]interface{}{
			"created_at":        now.Format(time.RFC3339),
			"execution_enabled": false,
			"queued":            true,
			"runtime_type":      runtimeType,
			"agent_action_id":   action.ID,
			"next_step":         "controlled runtime worker execution",
		}, "{}"),
	}

	if run.RiskLevel == "" {
		run.RiskLevel = models.AgentActionRiskMedium
	}
	if run.PolicyStatus == "" {
		run.PolicyStatus = models.AgentActionPolicyStatusWarning
	}

	if err := database.DB.Create(&run).Error; err != nil {
		return nil, err
	}

	event := models.ControlledTestEvent{
		UserID:    uid,
		OwnerKey:  action.OwnerKey,
		TargetID:  target.ID,
		RunID:     &run.ID,
		EventType: models.ControlledTestEventCreated,
		AfterJSON: controlledJSON(map[string]interface{}{
			"id":              run.ID,
			"runtime_type":    run.RuntimeType,
			"status":          run.Status,
			"agent_action_id": action.ID,
		}, "{}"),
		Metadata: controlledJSON(map[string]interface{}{
			"source":            "agent_action_dispatch",
			"execution_enabled": false,
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	return &run, nil
}
