package handlers

import (
	"encoding/json"
	"fmt"
	"log"
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

type createBugTestRunRequest struct {
	AgentActionID *uint                  `json:"agent_action_id"`
	Profile       string                 `json:"profile"`
	BugTypes      []string               `json:"bug_types"`
	OWASPRefs     []string               `json:"owasp_refs"`
	SafetyLevel   int                    `json:"safety_level"`
	TestLevel     int                    `json:"test_level"`
	InputJSON     map[string]interface{} `json:"input_json"`
}

func ensureSafeBugTestingEnabled(c *fiber.Ctx) error {
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}
	if !featureflags.IsEnabledForOwner(ownerKey, featureflags.KeySafeBugTesting) {
		return featureflags.DisabledResponse(c, featureflags.KeySafeBugTesting)
	}
	return nil
}

func bugTestJSON(value interface{}) datatypes.JSON {
	if value == nil {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}

func bugTestJSONArray(values []string) datatypes.JSON {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.ToLower(strings.TrimSpace(value))
		if v != "" {
			clean = append(clean, v)
		}
	}
	b, err := json.Marshal(clean)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	return datatypes.JSON(b)
}

func normalizeBugTestProfile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.BugTestProfilePassive:
		return models.BugTestProfilePassive
	case models.BugTestProfileSafe:
		return models.BugTestProfileSafe
	case models.BugTestProfileBalanced:
		return models.BugTestProfileBalanced
	case models.BugTestProfileManualApproved:
		return models.BugTestProfileManualApproved
	default:
		return models.BugTestProfilePassive
	}
}

func normalizeBugType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.BugTypeXSS:
		return models.BugTypeXSS
	case models.BugTypeCORS:
		return models.BugTypeCORS
	case models.BugTypeOpenRedirect:
		return models.BugTypeOpenRedirect
	case models.BugTypeSecurityHeaders:
		return models.BugTypeSecurityHeaders
	case models.BugTypeAPI:
		return models.BugTypeAPI
	default:
		return models.BugTypeUnknown
	}
}

func normalizeBugTypes(values []string) []string {
	out := make([]string, 0)
	seen := map[string]bool{}
	for _, value := range values {
		v := normalizeBugType(value)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	if len(out) == 0 {
		out = []string{
			models.BugTypeXSS,
			models.BugTypeCORS,
			models.BugTypeOpenRedirect,
			models.BugTypeSecurityHeaders,
		}
	}
	return out
}

func parseBugTestRunIDParam(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("run_id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid bug test run id")
	}
	return uint(id), nil
}

func bugTestPolicyCheck(target models.Target, profile string, safetyLevel int, testLevel int, bugTypes []string) (string, string, map[string]interface{}) {
	status := models.AgentActionPolicyStatusAllowed
	reason := "safe bug test run is allowed by baseline v3.8.0 guardrails"

	blockedReasons := make([]string, 0)
	warningReasons := make([]string, 0)

	var policy models.TargetPolicy
	hasPolicy := database.DB.Where("target_id = ?", target.ID).First(&policy).Error == nil

	if !hasPolicy {
		warningReasons = append(warningReasons, "target policy is missing; passive-only behavior is recommended")
	}

	if profile == models.BugTestProfileBalanced || profile == models.BugTestProfileManualApproved {
		blockedReasons = append(blockedReasons, "balanced/manual-approved profiles are not executable in v3.8.0 foundation")
	}

	if safetyLevel > 1 || testLevel > 1 {
		blockedReasons = append(blockedReasons, "v3.8.0 foundation only permits level 0 passive and level 1 safe checks")
	}

	if hasPolicy {
		maxLevel := intensityLevel(policy.MaxTestIntensity)
		if safetyLevel > maxLevel || testLevel > maxLevel {
			blockedReasons = append(blockedReasons, "bug test level exceeds target max_test_intensity")
		}
		if strings.TrimSpace(policy.RateLimitNotes) != "" {
			warningReasons = append(warningReasons, "target policy contains rate limit notes")
		}
		if policy.AuthRequired {
			warningReasons = append(warningReasons, "target policy indicates auth_required; authenticated checks require explicit handling")
		}
	}

	if len(blockedReasons) > 0 {
		status = models.AgentActionPolicyStatusBlocked
		reason = blockedReasons[0]
	} else if len(warningReasons) > 0 {
		status = models.AgentActionPolicyStatusWarning
		reason = warningReasons[0]
	}

	check := map[string]interface{}{
		"foundation_version": "safe-bug-testing-v1",
		"target_id":          target.ID,
		"root_domain":        target.RootDomain,
		"profile":            profile,
		"bug_types":          bugTypes,
		"safety_level":       safetyLevel,
		"test_level":         testLevel,
		"policy_status":      status,
		"reason":             reason,
		"blocked_reasons":    blockedReasons,
		"warning_reasons":    warningReasons,
		"execution_enabled":  false,
		"active_testing":     false,
		"guardrails": []string{
			"v3.8.0 foundation runner is passive/stub-only",
			"no exploit payloads are executed",
			"results are candidates and need manual validation",
			"level 2+ controlled active checks require future explicit approval and policy controls",
		},
	}

	if hasPolicy {
		check["max_test_intensity"] = policy.MaxTestIntensity
		check["auth_required"] = policy.AuthRequired
		check["safe_testing_notes"] = policy.SafeTestingNotes
		check["rate_limit_notes"] = policy.RateLimitNotes
	}

	return status, reason, check
}

func parseStringArrayJSON(value datatypes.JSON) []string {
	out := make([]string, 0)
	if len(value) == 0 {
		return out
	}

	var raw []string
	if err := json.Unmarshal(value, &raw); err == nil {
		for _, item := range raw {
			item = strings.TrimSpace(item)
			if item != "" {
				out = append(out, item)
			}
		}
		return out
	}

	var anyRaw []interface{}
	if err := json.Unmarshal(value, &anyRaw); err == nil {
		for _, item := range anyRaw {
			text := strings.TrimSpace(fmt.Sprint(item))
			if text != "" {
				out = append(out, text)
			}
		}
	}

	return out
}

func parsePatternMatcher(pattern models.BugPattern) map[string]interface{} {
	out := map[string]interface{}{}
	if len(pattern.MatcherJSON) == 0 {
		return out
	}
	_ = json.Unmarshal(pattern.MatcherJSON, &out)
	return out
}

func loadRunnableBugPatterns(bugTypes []string) ([]models.BugPattern, error) {
	q := database.DB.
		Where("enabled = ?", true).
		Where("safe_by_default = ?", true).
		Where("requires_approval = ?", false).
		Where("mode = ?", "passive").
		Where("test_level <= ? AND safety_level <= ?", 1, 1)

	if len(bugTypes) > 0 {
		q = q.Where("bug_type IN ?", bugTypes)
	}

	rows := make([]models.BugPattern, 0)
	if err := q.Order("bug_type ASC, key ASC").Find(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

func queryParamsFromURL(rawURL string) map[string]bool {
	out := map[string]bool{}
	parts := strings.SplitN(rawURL, "?", 2)
	if len(parts) != 2 {
		return out
	}

	query := parts[1]
	if hashIdx := strings.Index(query, "#"); hashIdx >= 0 {
		query = query[:hashIdx]
	}

	for _, pair := range strings.Split(query, "&") {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		name := pair
		if eqIdx := strings.Index(pair, "="); eqIdx >= 0 {
			name = pair[:eqIdx]
		}
		name = strings.ToLower(strings.TrimSpace(name))
		if name != "" {
			out[name] = true
		}
	}

	return out
}

func matchedPatternParameter(rawURL string, pattern models.BugPattern) string {
	matcher := parsePatternMatcher(pattern)
	matcherType := strings.ToLower(strings.TrimSpace(fmt.Sprint(matcher["type"])))
	if matcherType != "url_query_param_name_contains" {
		return ""
	}

	expected := make([]string, 0)
	switch v := matcher["parameters"].(type) {
	case []interface{}:
		for _, item := range v {
			text := strings.ToLower(strings.TrimSpace(fmt.Sprint(item)))
			if text != "" {
				expected = append(expected, text)
			}
		}
	case []string:
		for _, item := range v {
			text := strings.ToLower(strings.TrimSpace(item))
			if text != "" {
				expected = append(expected, text)
			}
		}
	}

	if len(expected) == 0 {
		return ""
	}

	params := queryParamsFromURL(rawURL)
	for name := range params {
		for _, candidate := range expected {
			if name == candidate || strings.Contains(name, candidate) {
				return name
			}
		}
	}

	return ""
}

func createBugTestResultFromPattern(run *models.BugTestRun, target *models.Target, pattern models.BugPattern, assetID *uint, urlID *uint, status string, evidence map[string]interface{}) models.BugTestResult {
	patternID := pattern.ID
	if status == "" {
		status = models.BugTestResultStatusCandidate
	}

	evidence["pattern_id"] = pattern.ID
	evidence["pattern_key"] = pattern.Key
	evidence["pattern_name"] = pattern.Name
	evidence["active_testing"] = false
	evidence["manual_validation"] = true

	return models.BugTestResult{
		RunID:        run.ID,
		TargetID:     target.ID,
		AssetID:      assetID,
		URLID:        urlID,
		PatternID:    &patternID,
		PatternKey:   pattern.Key,
		BugType:      pattern.BugType,
		TestName:     pattern.Key,
		Status:       status,
		Confidence:   pattern.ConfidenceDefault,
		SeverityHint: pattern.SeverityHint,
		EvidenceJSON: bugTestJSON(evidence),
		OWASPRefs:    pattern.OWASPRefs,
		Tags:         pattern.Tags,
	}
}

func seedPassiveBugTestResults(run *models.BugTestRun, target *models.Target, bugTypes []string) (int, error) {
	count := 0
	now := time.Now().UTC()

	patterns, err := loadRunnableBugPatterns(bugTypes)
	if err != nil {
		return 0, err
	}

	if len(patterns) == 0 {
		return 0, nil
	}

	var assets []models.Asset
	_ = database.DB.Where("target_id = ? AND is_live = ?", target.ID, true).
		Order("status_code DESC, id DESC").
		Limit(20).
		Find(&assets).Error

	var urls []models.FoundURL
	_ = database.DB.Where("target_id = ?", target.ID).
		Order("id DESC").
		Limit(50).
		Find(&urls).Error

	for _, pattern := range patterns {
		matcher := parsePatternMatcher(pattern)
		matcherType := strings.ToLower(strings.TrimSpace(fmt.Sprint(matcher["type"])))

		switch matcherType {
		case "live_http_asset":
			for _, asset := range assets {
				evidence := map[string]interface{}{
					"mode":        "passive_registry",
					"asset":       asset.Value,
					"final_url":   asset.FinalURL,
					"status_code": asset.StatusCode,
					"web_server":  asset.WebServer,
					"note":        pattern.Description,
					"created_at":  now,
				}

				row := createBugTestResultFromPattern(
					run,
					target,
					pattern,
					&asset.ID,
					nil,
					models.BugTestResultStatusNeedsManualValidation,
					evidence,
				)

				if err := database.DB.Create(&row).Error; err != nil {
					return count, err
				}
				count++
			}

		case "url_query_param_name_contains":
			for _, foundURL := range urls {
				param := matchedPatternParameter(foundURL.Value, pattern)
				if param == "" {
					continue
				}

				evidence := map[string]interface{}{
					"mode":       "passive_registry",
					"url":        foundURL.Value,
					"parameter":  param,
					"note":       pattern.Description,
					"created_at": now,
				}

				row := createBugTestResultFromPattern(
					run,
					target,
					pattern,
					nil,
					&foundURL.ID,
					models.BugTestResultStatusCandidate,
					evidence,
				)

				if err := database.DB.Create(&row).Error; err != nil {
					return count, err
				}
				count++
			}

		default:
			log.Printf("⚠️ Unsupported bug pattern matcher type %q for pattern %s", matcherType, pattern.Key)
		}
	}

	return count, nil
}

func GetTargetBugTestRuns(c *fiber.Ctx) error {
	if err := ensureSafeBugTestingEnabled(c); err != nil {
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

	db := database.DB.Model(&models.BugTestRun{}).Where("target_id = ?", target.ID)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.BugTestRun, 0)
	if err := db.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "data": rows, "count": len(rows), "total_count": total, "page": page})
}

func CreateTargetBugTestRun(c *fiber.Ctx) error {
	if err := ensureSafeBugTestingEnabled(c); err != nil {
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

	req := new(createBugTestRunRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	profile := normalizeBugTestProfile(req.Profile)
	bugTypes := normalizeBugTypes(req.BugTypes)

	safetyLevel := clampAgentLevel(req.SafetyLevel)
	testLevel := clampAgentLevel(req.TestLevel)

	if profile == models.BugTestProfilePassive {
		safetyLevel = 0
		testLevel = 0
	}
	if profile == models.BugTestProfileSafe && testLevel == 0 {
		testLevel = 1
		safetyLevel = 1
	}

	policyStatus, policyReason, policyCheck := bugTestPolicyCheck(*target, profile, safetyLevel, testLevel, bugTypes)

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	now := time.Now().UTC()
	run := models.BugTestRun{
		TargetID:        target.ID,
		CreatedByUserID: &uid,
		AgentActionID:   req.AgentActionID,
		Profile:         profile,
		Status:          models.BugTestRunStatusRunning,
		PolicyStatus:    policyStatus,
		SafetyLevel:     safetyLevel,
		TestLevel:       testLevel,
		BugTypes:        bugTestJSONArray(bugTypes),
		OWASPRefs:       bugTestJSONArray(req.OWASPRefs),
		InputJSON: bugTestJSON(map[string]interface{}{
			"request":          req.InputJSON,
			"requested_by_api": true,
			"active_testing":   false,
		}),
		PolicyCheckJSON: bugTestJSON(policyCheck),
		StartedAt:       &now,
	}

	if policyStatus == models.AgentActionPolicyStatusBlocked {
		run.Status = models.BugTestRunStatusBlockedByPolicy
		run.ErrorMessage = policyReason
		run.CompletedAt = &now
	}

	if err := database.DB.Create(&run).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	resultCount := 0
	if run.Status == models.BugTestRunStatusRunning {
		resultCount, err = seedPassiveBugTestResults(&run, target, bugTypes)
		completed := time.Now().UTC()
		run.Status = models.BugTestRunStatusCompleted
		run.CompletedAt = &completed
		run.OutputJSON = bugTestJSON(map[string]interface{}{
			"runner_version":      "safe-bug-testing-v1",
			"active_testing":      false,
			"results_created":     resultCount,
			"manual_validation":   true,
			"execution_statement": "passive/stub foundation only; no payloads were sent",
		})
		if err != nil {
			run.Status = models.BugTestRunStatusFailed
			run.ErrorMessage = err.Error()
		}
		_ = database.DB.Save(&run).Error
	}

	entityID := run.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.bug_test.run.create",
		EntityType:  "bug_test_run",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":       target.ID,
			"root_domain":     target.RootDomain,
			"profile":         run.Profile,
			"status":          run.Status,
			"policy_status":   run.PolicyStatus,
			"safety_level":    run.SafetyLevel,
			"test_level":      run.TestLevel,
			"results_created": resultCount,
			"active_testing":  false,
		},
	})

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "data": run})
}

func GetTargetBugTestResults(c *fiber.Ctx) error {
	if err := ensureSafeBugTestingEnabled(c); err != nil {
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

	db := database.DB.Model(&models.BugTestResult{}).Where("target_id = ?", target.ID)

	if runID, err := parseBugTestRunIDParam(c); err == nil && runID > 0 {
		db = db.Where("run_id = ?", runID)
	}

	if bugType := normalizeBugType(c.Query("bug_type")); bugType != models.BugTypeUnknown {
		db = db.Where("bug_type = ?", bugType)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.BugTestResult, 0)
	if err := db.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "data": rows, "count": len(rows), "total_count": total, "page": page})
}

// CreateBugTestRunFromAgentAction executes the safe bug testing engine foundation
// from an approved agent action. v3.8.0 remains passive/stub-only.
func CreateBugTestRunFromAgentAction(target *models.Target, action *models.AgentAction, uid uint) (*models.BugTestRun, int, error) {
	if target == nil || action == nil {
		return nil, 0, fmt.Errorf("missing target or action")
	}

	if normalizeAgentActionType(action.ActionType) != models.AgentActionTypeRunSafeBugTests {
		return nil, 0, fmt.Errorf("agent action type %s is not supported by safe bug testing engine", action.ActionType)
	}

	input := map[string]interface{}{}
	if len(action.InputJSON) > 0 {
		_ = json.Unmarshal(action.InputJSON, &input)
	}

	bugTypes := make([]string, 0)
	if raw, ok := input["bug_types"].([]interface{}); ok {
		for _, item := range raw {
			bugTypes = append(bugTypes, fmt.Sprint(item))
		}
	}
	if raw, ok := input["bug_types"].([]string); ok {
		bugTypes = append(bugTypes, raw...)
	}
	if len(bugTypes) == 0 {
		bugTypes = []string{
			models.BugTypeXSS,
			models.BugTypeCORS,
			models.BugTypeOpenRedirect,
			models.BugTypeSecurityHeaders,
		}
	}
	bugTypes = normalizeBugTypes(bugTypes)

	profile := models.BugTestProfilePassive
	if rawProfile, ok := input["test_profile"].(string); ok {
		profile = normalizeBugTestProfile(rawProfile)
	}
	if profile != models.BugTestProfilePassive && profile != models.BugTestProfileSafe {
		profile = models.BugTestProfilePassive
	}

	safetyLevel := action.SafetyLevel
	testLevel := action.TestLevel
	if profile == models.BugTestProfilePassive {
		safetyLevel = 0
		testLevel = 0
	}
	if profile == models.BugTestProfileSafe {
		if safetyLevel == 0 {
			safetyLevel = 1
		}
		if testLevel == 0 {
			testLevel = 1
		}
	}

	policyStatus, policyReason, policyCheck := bugTestPolicyCheck(*target, profile, safetyLevel, testLevel, bugTypes)

	now := time.Now().UTC()
	run := models.BugTestRun{
		TargetID:        target.ID,
		CreatedByUserID: &uid,
		AgentActionID:   &action.ID,
		Profile:         profile,
		Status:          models.BugTestRunStatusRunning,
		PolicyStatus:    policyStatus,
		SafetyLevel:     safetyLevel,
		TestLevel:       testLevel,
		BugTypes:        bugTestJSONArray(bugTypes),
		OWASPRefs:       bugTestJSONArray([]string{"OWASP-WSTG", "OWASP-ASVS"}),
		InputJSON: bugTestJSON(map[string]interface{}{
			"source":             "agent_action_dispatch",
			"agent_action_id":    action.ID,
			"agent_action_title": action.Title,
			"agent_action_input": input,
			"active_testing":     false,
		}),
		PolicyCheckJSON: bugTestJSON(policyCheck),
		StartedAt:       &now,
	}

	if policyStatus == models.AgentActionPolicyStatusBlocked {
		run.Status = models.BugTestRunStatusBlockedByPolicy
		run.ErrorMessage = policyReason
		run.CompletedAt = &now
	}

	if err := database.DB.Create(&run).Error; err != nil {
		return nil, 0, err
	}

	resultCount := 0
	if run.Status == models.BugTestRunStatusRunning {
		var err error
		resultCount, err = seedPassiveBugTestResults(&run, target, bugTypes)

		completed := time.Now().UTC()
		run.CompletedAt = &completed
		run.Status = models.BugTestRunStatusCompleted
		run.OutputJSON = bugTestJSON(map[string]interface{}{
			"runner_version":      "safe-bug-testing-v1",
			"source":              "agent_action_dispatch",
			"agent_action_id":     action.ID,
			"active_testing":      false,
			"results_created":     resultCount,
			"manual_validation":   true,
			"execution_statement": "passive/stub foundation only; no payloads were sent",
		})

		if err != nil {
			run.Status = models.BugTestRunStatusFailed
			run.ErrorMessage = err.Error()
		}

		if saveErr := database.DB.Save(&run).Error; saveErr != nil {
			return &run, resultCount, saveErr
		}

		if err != nil {
			return &run, resultCount, err
		}
	}

	entityID := run.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.bug_test.run.create",
		EntityType:  "bug_test_run",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		Metadata: map[string]interface{}{
			"target_id":       target.ID,
			"root_domain":     target.RootDomain,
			"source":          "agent_action_dispatch",
			"agent_action_id": action.ID,
			"profile":         run.Profile,
			"status":          run.Status,
			"policy_status":   run.PolicyStatus,
			"safety_level":    run.SafetyLevel,
			"test_level":      run.TestLevel,
			"results_created": resultCount,
			"active_testing":  false,
		},
	})

	return &run, resultCount, nil
}

func parseBugTestResultIDParam(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("result_id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fmt.Errorf("invalid bug test result id")
	}
	return uint(id), nil
}

// DeleteTargetBugTestRun soft-deletes one bug test run and its results.
func DeleteTargetBugTestRun(c *fiber.Ctx) error {
	if err := ensureSafeBugTestingEnabled(c); err != nil {
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

	runID, err := parseBugTestRunIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var run models.BugTestRun
	if err := database.DB.Where("id = ? AND target_id = ?", runID, target.ID).First(&run).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug test run not found"})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var resultCount int64
	_ = database.DB.Model(&models.BugTestResult{}).
		Where("run_id = ? AND target_id = ?", run.ID, target.ID).
		Count(&resultCount).Error

	if err := database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("run_id = ? AND target_id = ?", run.ID, target.ID).
			Delete(&models.BugTestResult{}).Error; err != nil {
			return err
		}
		return tx.Delete(&run).Error
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	entityID := run.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.bug_test.run.delete",
		EntityType:  "bug_test_run",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":     target.ID,
			"root_domain":   target.RootDomain,
			"profile":       run.Profile,
			"status":        run.Status,
			"policy_status": run.PolicyStatus,
			"result_count":  resultCount,
			"deleted":       true,
		},
	})

	return c.JSON(fiber.Map{"status": "success", "message": "bug test run deleted"})
}

// DeleteTargetBugTestResult soft-deletes one bug test result.
func DeleteTargetBugTestResult(c *fiber.Ctx) error {
	if err := ensureSafeBugTestingEnabled(c); err != nil {
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

	resultID, err := parseBugTestResultIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var result models.BugTestResult
	if err := database.DB.Where("id = ? AND target_id = ?", resultID, target.ID).First(&result).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug test result not found"})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	if err := database.DB.Delete(&result).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	entityID := result.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.bug_test.result.delete",
		EntityType:  "bug_test_result",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":     target.ID,
			"root_domain":   target.RootDomain,
			"run_id":        result.RunID,
			"bug_type":      result.BugType,
			"test_name":     result.TestName,
			"status":        result.Status,
			"confidence":    result.Confidence,
			"severity_hint": result.SeverityHint,
			"deleted":       true,
		},
	})

	return c.JSON(fiber.Map{"status": "success", "message": "bug test result deleted"})
}

type updateBugTestResultStatusRequest struct {
	Status string `json:"status"`
}

func normalizeBugTestResultStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.BugTestResultStatusCandidate:
		return models.BugTestResultStatusCandidate
	case models.BugTestResultStatusNeedsManualValidation:
		return models.BugTestResultStatusNeedsManualValidation
	case models.BugTestResultStatusPassed:
		return models.BugTestResultStatusPassed
	case models.BugTestResultStatusFailed:
		return models.BugTestResultStatusFailed
	case models.BugTestResultStatusBlocked:
		return models.BugTestResultStatusBlocked
	case models.BugTestResultStatusInconclusive:
		return models.BugTestResultStatusInconclusive
	case models.BugTestResultStatusValidated:
		return models.BugTestResultStatusValidated
	case models.BugTestResultStatusFalsePositive:
		return models.BugTestResultStatusFalsePositive
	case models.BugTestResultStatusIgnored:
		return models.BugTestResultStatusIgnored
	default:
		return ""
	}
}

// UpdateTargetBugTestResultStatus updates triage/validation status for one bug test result.
func UpdateTargetBugTestResultStatus(c *fiber.Ctx) error {
	if err := ensureSafeBugTestingEnabled(c); err != nil {
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

	resultID, err := parseBugTestResultIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	req := new(updateBugTestResultStatusRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body"})
	}

	nextStatus := normalizeBugTestResultStatus(req.Status)
	if nextStatus == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug test result status"})
	}

	var result models.BugTestResult
	if err := database.DB.Where("id = ? AND target_id = ?", resultID, target.ID).First(&result).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug test result not found"})
	}

	previousStatus := result.Status
	result.Status = nextStatus

	if err := database.DB.Save(&result).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	uid, _ := currentUserID(c)
	actorID := uid
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &actorID,
		TargetID:    &target.ID,
		Action:      "target.bug_test.result.status_update",
		EntityType:  "bug_test_result",
		EntityID:    &result.ID,
		Metadata: fiber.Map{
			"run_id":          result.RunID,
			"bug_type":        result.BugType,
			"pattern_key":     result.PatternKey,
			"previous_status": previousStatus,
			"new_status":      nextStatus,
		},
	})

	return c.JSON(fiber.Map{"status": "success", "data": result})
}
