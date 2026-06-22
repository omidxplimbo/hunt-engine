package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	runtimeType := normalizeControlledRuntimeType(c.Query("runtime_type"))
	runID := uint(c.QueryInt("run_id", 0))
	agentActionID := uint(c.QueryInt("agent_action_id", 0))

	q := database.DB.
		Model(&models.ControlledTestResult{}).
		Where("target_id = ? AND owner_key = ?", target.ID, ownerKey)

	if status != "" {
		q = q.Where("status = ?", status)
	}
	if strings.TrimSpace(c.Query("runtime_type")) != "" {
		q = q.Where("runtime_type = ?", runtimeType)
	}
	if runID > 0 {
		q = q.Where("run_id = ?", runID)
	}
	if agentActionID > 0 {
		q = q.Where("agent_action_id = ?", agentActionID)
	}

	if bugType := strings.TrimSpace(c.Query("bug_type")); bugType != "" {
		q = q.Where("bug_type = ?", bugType)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return err
	}

	rows := make([]models.ControlledTestResult, 0)
	if err := q.Order("created_at DESC").Limit(limit).Find(&rows).Error; err != nil {
		return err
	}

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

type executeControlledTestRunRequest struct {
	Note      string `json:"note"`
	TimeoutMS int    `json:"timeout_ms"`
	MaxBytes  int64  `json:"max_bytes"`
}

type controlledHTTPProbeInput struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
}

func normalizeControlledHTTPMethod(value string) string {
	method := strings.ToUpper(strings.TrimSpace(value))
	switch method {
	case "", "GET":
		return "GET"
	case "HEAD":
		return "HEAD"
	case "POST":
		return "POST"
	default:
		return ""
	}
}

func controlledCleanString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case []byte:
		return strings.TrimSpace(string(v))
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func controlledBodyExcerpt(text string, max int) string {
	if max <= 0 {
		max = 2048
	}
	runes := []rune(text)
	if len(runes) <= max {
		return text
	}
	return string(runes[:max])
}

func executeControlledHTTPProbe(run *models.ControlledTestRun, uid uint, timeoutMS int, maxBytes int64) (*models.ControlledTestResult, map[string]interface{}, error) {
	input := map[string]interface{}{}
	if len(run.InputJSON) > 0 {
		_ = json.Unmarshal(run.InputJSON, &input)
	}

	rawURL := strings.TrimSpace(controlledCleanString(input["url"]))
	if rawURL == "" {
		return nil, nil, fiber.NewError(fiber.StatusBadRequest, "http_probe input_json.url is required")
	}
	if !strings.HasPrefix(strings.ToLower(rawURL), "http://") && !strings.HasPrefix(strings.ToLower(rawURL), "https://") {
		return nil, nil, fiber.NewError(fiber.StatusBadRequest, "http_probe url must be http or https")
	}

	method := normalizeControlledHTTPMethod(controlledCleanString(input["method"]))
	if method == "" {
		return nil, nil, fiber.NewError(fiber.StatusBadRequest, "unsupported http_probe method")
	}

	if timeoutMS <= 0 || timeoutMS > 30000 {
		timeoutMS = 8000
	}
	if maxBytes <= 0 || maxBytes > 1024*1024 {
		maxBytes = 256 * 1024
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutMS) * time.Millisecond,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}

	var bodyReader io.Reader
	if method == "POST" {
		bodyReader = strings.NewReader(controlledCleanString(input["body"]))
	}

	req, err := http.NewRequest(method, rawURL, bodyReader)
	if err != nil {
		return nil, nil, err
	}

	req.Header.Set("User-Agent", "HuntEngine-ControlledRuntime/1.0")
	req.Header.Set("Accept", "*/*")

	if headersRaw, ok := input["headers"].(map[string]interface{}); ok {
		for key, value := range headersRaw {
			key = strings.TrimSpace(key)
			if key == "" {
				continue
			}
			lower := strings.ToLower(key)
			if lower == "host" || lower == "content-length" {
				continue
			}
			req.Header.Set(key, controlledCleanString(value))
		}
	}

	start := time.Now().UTC()
	resp, err := client.Do(req)
	durationMS := time.Since(start).Milliseconds()

	requestJSON := map[string]interface{}{
		"method":     method,
		"url":        rawURL,
		"headers":    req.Header,
		"timeout_ms": timeoutMS,
		"max_bytes":  maxBytes,
	}

	if err != nil {
		result := models.ControlledTestResult{
			UserID:        uid,
			OwnerKey:      run.OwnerKey,
			TargetID:      run.TargetID,
			RunID:         run.ID,
			AgentActionID: run.AgentActionID,
			RuntimeType:   run.RuntimeType,
			BugType:       "http_probe",
			CheckKey:      "controlled.http_probe.v1",
			URL:           rawURL,
			Method:        method,
			Status:        models.ControlledTestResultStatusInconclusive,
			Confidence:    "low",
			SeverityHint:  "info",
			RequestJSON:   controlledJSON(requestJSON, "{}"),
			ResponseJSON: controlledJSON(map[string]interface{}{
				"error":       err.Error(),
				"duration_ms": durationMS,
			}, "{}"),
			EvidenceJSON: controlledJSON(map[string]interface{}{
				"proof":       "http request failed or was inconclusive",
				"error":       err.Error(),
				"duration_ms": durationMS,
			}, "{}"),
			Metadata: controlledJSON(map[string]interface{}{
				"source": "controlled_http_probe_v1",
			}, "{}"),
		}
		if createErr := database.DB.Create(&result).Error; createErr != nil {
			return nil, nil, createErr
		}
		return &result, map[string]interface{}{
			"status":      result.Status,
			"duration_ms": durationMS,
			"error":       err.Error(),
		}, nil
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBytes)
	bodyBytes, readErr := io.ReadAll(limited)
	bodyText := string(bodyBytes)

	headers := map[string][]string(resp.Header)
	responseJSON := map[string]interface{}{
		"status_code":    resp.StatusCode,
		"status":         resp.Status,
		"headers":        headers,
		"body_excerpt":   controlledBodyExcerpt(bodyText, 4096),
		"body_size":      len(bodyBytes),
		"duration_ms":    durationMS,
		"final_url":      resp.Request.URL.String(),
		"content_type":   resp.Header.Get("Content-Type"),
		"content_length": resp.ContentLength,
	}
	if readErr != nil {
		responseJSON["read_error"] = readErr.Error()
	}

	status := models.ControlledTestResultStatusPassed
	confidence := "medium"
	classification := "http_response_observed"

	cfMitigated := strings.TrimSpace(resp.Header.Get("Cf-Mitigated"))
	serverHeader := strings.ToLower(strings.TrimSpace(resp.Header.Get("Server")))
	isCloudflareChallenge := strings.EqualFold(cfMitigated, "challenge") ||
		(strings.Contains(serverHeader, "cloudflare") && resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(bodyText), "just a moment"))

	if resp.StatusCode >= 500 || resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || isCloudflareChallenge {
		status = models.ControlledTestResultStatusInconclusive
		confidence = "low"
		classification = "http_response_blocked_or_challenged"
	}

	evidence := map[string]interface{}{
		"proof":                "controlled HTTP probe completed",
		"classification":       classification,
		"cloudflare_challenge": isCloudflareChallenge,
		"cf_mitigated":         cfMitigated,
		"status_code":          resp.StatusCode,
		"duration_ms":          durationMS,
		"final_url":            resp.Request.URL.String(),
		"body_excerpt":         controlledBodyExcerpt(bodyText, 1200),
		"headers_seen":         len(headers),
	}

	result := models.ControlledTestResult{
		UserID:        uid,
		OwnerKey:      run.OwnerKey,
		TargetID:      run.TargetID,
		RunID:         run.ID,
		AgentActionID: run.AgentActionID,
		RuntimeType:   run.RuntimeType,
		BugType:       "http_probe",
		CheckKey:      "controlled.http_probe.v1",
		URL:           rawURL,
		Method:        method,
		Status:        status,
		Confidence:    confidence,
		SeverityHint:  "info",
		RequestJSON:   controlledJSON(requestJSON, "{}"),
		ResponseJSON:  controlledJSON(responseJSON, "{}"),
		EvidenceJSON:  controlledJSON(evidence, "{}"),
		MatcherJSON: controlledJSON(map[string]interface{}{
			"matcher": "http_probe_completed",
		}, "{}"),
		Metadata: controlledJSON(map[string]interface{}{
			"source": "controlled_http_probe_v1",
		}, "{}"),
	}

	if err := database.DB.Create(&result).Error; err != nil {
		return nil, nil, err
	}

	return &result, map[string]interface{}{
		"status":       result.Status,
		"status_code":  resp.StatusCode,
		"duration_ms":  durationMS,
		"body_size":    len(bodyBytes),
		"final_url":    resp.Request.URL.String(),
		"result_id":    result.ID,
		"evidence_key": "controlled.http_probe.v1",
	}, nil
}

func ExecuteControlledTestRunInternal(targetID uint, runID uint, ownerKey string, uid uint, note string, timeoutMS int, maxBytes int64, source string) (*models.ControlledTestRun, *models.ControlledTestResult, map[string]interface{}, error) {
	source = strings.TrimSpace(source)
	if source == "" {
		source = "controlled_runtime_execute_internal"
	}

	var run models.ControlledTestRun
	if err := database.DB.
		Where("id = ? AND target_id = ? AND owner_key = ?", runID, targetID, ownerKey).
		First(&run).Error; err != nil {
		return nil, nil, nil, fiber.NewError(fiber.StatusNotFound, "controlled test run not found")
	}

	if run.Status != models.ControlledTestRunStatusQueued && run.Status != models.ControlledTestRunStatusRunning {
		return nil, nil, nil, fiber.NewError(fiber.StatusConflict, "controlled test run is not executable")
	}

	if run.RuntimeType != models.ControlledTestRuntimeHTTPProbe {
		return nil, nil, nil, fiber.NewError(fiber.StatusConflict, "controlled runtime worker for this runtime_type is not implemented yet")
	}

	now := time.Now().UTC()
	_ = database.DB.Model(&models.ControlledTestRun{}).
		Where("id = ?", run.ID).
		Updates(map[string]interface{}{
			"status":     models.ControlledTestRunStatusRunning,
			"started_at": &now,
		}).Error

	startEvent := models.ControlledTestEvent{
		UserID:    uid,
		OwnerKey:  ownerKey,
		TargetID:  targetID,
		RunID:     &run.ID,
		EventType: models.ControlledTestEventStarted,
		Metadata: controlledJSON(map[string]interface{}{
			"source": source,
			"note":   strings.TrimSpace(note),
		}, "{}"),
	}
	_ = database.DB.Create(&startEvent).Error

	result, output, execErr := executeControlledHTTPProbe(&run, uid, timeoutMS, maxBytes)

	completedAt := time.Now().UTC()
	status := models.ControlledTestRunStatusCompleted
	errorMessage := ""
	if execErr != nil {
		status = models.ControlledTestRunStatusFailed
		errorMessage = execErr.Error()
		output = map[string]interface{}{
			"error": execErr.Error(),
		}
	}

	resultCount := 0
	if result != nil {
		resultCount = 1
		resultID := result.ID
		event := models.ControlledTestEvent{
			UserID:    uid,
			OwnerKey:  ownerKey,
			TargetID:  targetID,
			RunID:     &run.ID,
			ResultID:  &resultID,
			EventType: models.ControlledTestEventResultCreated,
			AfterJSON: controlledJSON(map[string]interface{}{
				"result_id":     result.ID,
				"status":        result.Status,
				"check_key":     result.CheckKey,
				"severity_hint": result.SeverityHint,
			}, "{}"),
			Metadata: controlledJSON(map[string]interface{}{
				"source": source,
			}, "{}"),
		}
		_ = database.DB.Create(&event).Error

		if memErr := createControlledRuntimeMemoryFromResult(&run, result, output); memErr != nil {
			if output == nil {
				output = map[string]interface{}{}
			}
			output["memory_ingest_error"] = memErr.Error()
		} else {
			if output == nil {
				output = map[string]interface{}{}
			}
			output["memory_ingested"] = true
		}
	}

	updates := map[string]interface{}{
		"status":        status,
		"completed_at":  &completedAt,
		"attempt_count": 1,
		"result_count":  resultCount,
		"output_json": controlledJSON(map[string]interface{}{
			"execution_enabled": true,
			"runtime_type":      run.RuntimeType,
			"worker":            "controlled-http-probe-v1",
			"completed_at":      completedAt.Format(time.RFC3339),
			"result":            output,
		}, "{}"),
		"error_message": errorMessage,
	}

	if err := database.DB.Model(&models.ControlledTestRun{}).
		Where("id = ?", run.ID).
		Updates(updates).Error; err != nil {
		return nil, result, output, err
	}

	finalEventType := models.ControlledTestEventCompleted
	if status == models.ControlledTestRunStatusFailed {
		finalEventType = models.ControlledTestEventFailed
	}
	finalEvent := models.ControlledTestEvent{
		UserID:    uid,
		OwnerKey:  ownerKey,
		TargetID:  targetID,
		RunID:     &run.ID,
		EventType: finalEventType,
		AfterJSON: controlledJSON(map[string]interface{}{
			"status":       status,
			"result_count": resultCount,
			"error":        errorMessage,
		}, "{}"),
		Metadata: controlledJSON(map[string]interface{}{
			"source": source,
		}, "{}"),
	}
	_ = database.DB.Create(&finalEvent).Error

	_ = database.DB.First(&run, run.ID)
	return &run, result, output, execErr
}

func ExecuteTargetControlledTestRun(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, target, ownerKey, err := controlledTestOwner(c, targetID)
	if err != nil {
		return err
	}

	runID, err := parseControlledRunIDParam(c)
	if err != nil {
		return err
	}

	req := new(executeControlledTestRunRequest)
	_ = c.BodyParser(req)

	run, result, output, execErr := ExecuteControlledTestRunInternal(
		target.ID,
		runID,
		ownerKey,
		user.ID,
		req.Note,
		req.TimeoutMS,
		req.MaxBytes,
		"controlled_runtime_execute_api",
	)
	if execErr != nil && run == nil {
		return execErr
	}

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"run":    run,
			"result": result,
			"output": output,
		},
	})
}

func controlledRuntimeMemoryTypeForResult(result models.ControlledTestResult) string {
	switch result.Status {
	case models.ControlledTestResultStatusVulnerable:
		return models.TargetMemoryTypeFindingEvidence
	case models.ControlledTestResultStatusPassed:
		return models.TargetMemoryTypeSuccessfulTest
	case models.ControlledTestResultStatusFailed:
		return models.TargetMemoryTypeFailedTest
	default:
		return models.TargetMemoryTypeTestResult
	}
}

func controlledRuntimeMemoryImportance(result models.ControlledTestResult) int {
	switch result.Status {
	case models.ControlledTestResultStatusVulnerable:
		return 95
	case models.ControlledTestResultStatusPassed:
		return 78
	case models.ControlledTestResultStatusInconclusive, models.ControlledTestResultStatusNeedsManualReview:
		return 82
	case models.ControlledTestResultStatusBlocked:
		return 84
	case models.ControlledTestResultStatusFailed:
		return 65
	default:
		return 75
	}
}

func controlledRuntimeMemoryConfidence(result models.ControlledTestResult) int {
	switch strings.ToLower(strings.TrimSpace(result.Confidence)) {
	case "high":
		return 90
	case "medium":
		return 75
	case "low":
		return 45
	default:
		return 60
	}
}

func controlledJSONMap(raw []byte) map[string]interface{} {
	out := map[string]interface{}{}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &out)
	}
	return out
}

func controlledIntFromMap(m map[string]interface{}, key string) int {
	v, ok := m[key]
	if !ok || v == nil {
		return 0
	}
	switch x := v.(type) {
	case int:
		return x
	case int64:
		return int(x)
	case float64:
		return int(x)
	case json.Number:
		i, _ := x.Int64()
		return int(i)
	default:
		return 0
	}
}

func createControlledRuntimeMemoryFromResult(run *models.ControlledTestRun, result *models.ControlledTestResult, output map[string]interface{}) error {
	if run == nil || result == nil {
		return nil
	}
	if strings.TrimSpace(result.OwnerKey) == "" {
		return nil
	}

	response := controlledJSONMap(result.ResponseJSON)
	evidence := controlledJSONMap(result.EvidenceJSON)

	statusCode := controlledIntFromMap(response, "status_code")
	if statusCode == 0 {
		statusCode = controlledIntFromMap(evidence, "status_code")
	}

	classification := strings.TrimSpace(controlledCleanString(evidence["classification"]))
	if classification == "" {
		classification = "runtime_result_observed"
	}

	bodyExcerpt := strings.TrimSpace(controlledCleanString(evidence["body_excerpt"]))
	if bodyExcerpt != "" {
		bodyExcerpt = controlledBodyExcerpt(bodyExcerpt, 700)
	}

	agentActionID := interface{}(nil)
	agentActionLabel := "none"
	if result.AgentActionID != nil {
		agentActionID = *result.AgentActionID
		agentActionLabel = fmt.Sprint(*result.AgentActionID)
	}

	lines := []string{
		"Controlled runtime test result:",
		"Runtime type: " + result.RuntimeType,
		"Check key: " + result.CheckKey,
		"URL: " + result.URL,
		"Method: " + result.Method,
		"Result status: " + result.Status,
		"Confidence: " + result.Confidence,
		"Severity hint: " + result.SeverityHint,
		"HTTP status code: " + fmt.Sprint(statusCode),
		"Classification: " + classification,
		"Run ID: " + fmt.Sprint(run.ID),
		"Result ID: " + fmt.Sprint(result.ID),
		"Agent action ID: " + agentActionLabel,
		"Operator note: This is evidence from controlled runtime execution and should guide the next LLM pentest step.",
	}

	if cfChallenge, ok := evidence["cloudflare_challenge"]; ok {
		lines = append(lines, "Cloudflare challenge observed: "+fmt.Sprint(cfChallenge))
	}
	if finalURL := strings.TrimSpace(controlledCleanString(evidence["final_url"])); finalURL != "" {
		lines = append(lines, "Final URL: "+finalURL)
	}
	if bodyExcerpt != "" {
		lines = append(lines, "Body excerpt: "+bodyExcerpt)
	}

	nextHint := "Use this runtime evidence to decide whether to continue, retest with a different method, escalate to a more specific validation, or mark as inconclusive."
	if strings.Contains(classification, "blocked") || strings.Contains(classification, "challenged") || result.Status == models.ControlledTestResultStatusInconclusive {
		nextHint = "Treat this as blocked/challenged/inconclusive evidence, not vulnerability proof. Consider browser-aware validation, alternate endpoint selection, authenticated context, or policy-approved deeper testing."
	}
	lines = append(lines, "Next operator hint: "+nextHint)

	titleURL := result.URL
	if titleURL == "" {
		titleURL = fmt.Sprintf("run-%d", run.ID)
	}

	memoryType := controlledRuntimeMemoryTypeForResult(*result)
	item := models.TargetMemoryItem{
		UserID:     result.UserID,
		OwnerKey:   result.OwnerKey,
		TargetID:   result.TargetID,
		SourceType: models.TargetMemorySourceControlledTestResult,
		SourceID:   &result.ID,
		MemoryType: memoryType,
		Title:      "Controlled runtime result: " + titleURL,
		Content:    strings.Join(lines, "\n"),
		Summary: fmt.Sprintf(
			"Controlled %s produced %s for %s with status_code=%d classification=%s.",
			result.RuntimeType,
			result.Status,
			titleURL,
			statusCode,
			classification,
		),
		Tags: memoryJSON([]string{
			"controlled_runtime",
			"real_test_result",
			result.RuntimeType,
			result.BugType,
			result.Status,
			"operator",
			"rag",
		}, "[]"),
		Importance: controlledRuntimeMemoryImportance(*result),
		Confidence: controlledRuntimeMemoryConfidence(*result),
		SourceHash: memoryHash(
			"controlled_runtime_result",
			fmt.Sprint(result.TargetID),
			fmt.Sprint(run.ID),
			fmt.Sprint(result.ID),
			result.RuntimeType,
			result.URL,
			result.Status,
			fmt.Sprint(statusCode),
			classification,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":     "controlled-runtime-memory-v1",
			"runtime_type":       result.RuntimeType,
			"run_id":             run.ID,
			"result_id":          result.ID,
			"agent_action_id":    agentActionID,
			"status":             result.Status,
			"status_code":        statusCode,
			"classification":     classification,
			"check_key":          result.CheckKey,
			"source":             "controlled_runtime_execute_api",
			"operator_ready":     true,
			"next_operator_hint": nextHint,
			"execution_evidence": true,
			"output":             output,
		}, "{}"),
	}

	_, _, err := upsertTargetMemoryItem(item)
	return err
}
