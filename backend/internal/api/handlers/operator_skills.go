package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

func normalizeOperatorSkillStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorSkillRunStatusPlanned:
		return models.OperatorSkillRunStatusPlanned
	case models.OperatorSkillRunStatusRunning:
		return models.OperatorSkillRunStatusRunning
	case models.OperatorSkillRunStatusCompleted:
		return models.OperatorSkillRunStatusCompleted
	case models.OperatorSkillRunStatusFailed:
		return models.OperatorSkillRunStatusFailed
	case models.OperatorSkillRunStatusBlockedByPolicy:
		return models.OperatorSkillRunStatusBlockedByPolicy
	case models.OperatorSkillRunStatusNeedsContext:
		return models.OperatorSkillRunStatusNeedsContext
	case models.OperatorSkillRunStatusNeedsApproval:
		return models.OperatorSkillRunStatusNeedsApproval
	case models.OperatorSkillRunStatusCancelled:
		return models.OperatorSkillRunStatusCancelled
	default:
		return ""
	}
}

func parseOperatorSkillTargetID(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("id"))
	if raw == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "target id is required")
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid target id")
	}

	return uint(id), nil
}

type updateOperatorTargetSkillProfileRequest struct {
	IsEnabled                  *bool           `json:"is_enabled"`
	PermissionMode             string          `json:"permission_mode"`
	EnabledSkillSlugs          json.RawMessage `json:"enabled_skill_slugs"`
	DisabledSkillSlugs         json.RawMessage `json:"disabled_skill_slugs"`
	PreferredLearningRecordIDs json.RawMessage `json:"preferred_learning_record_ids"`
	AllowedRuntimeBackends     json.RawMessage `json:"allowed_runtime_backends"`
	BudgetDefaults             json.RawMessage `json:"budget_defaults"`
	StopConditions             json.RawMessage `json:"stop_conditions"`
	Metadata                   json.RawMessage `json:"metadata"`
}

type upsertOperatorSkillRequest struct {
	Name                      string          `json:"name"`
	Title                     string          `json:"title"`
	Slug                      string          `json:"slug"`
	Description               string          `json:"description"`
	Scope                     string          `json:"scope"`
	SkillType                 string          `json:"skill_type"`
	RuntimeBackend            string          `json:"runtime_backend"`
	ProjectKey                string          `json:"project_key"`
	TargetID                  *uint           `json:"target_id"`
	Category                  string          `json:"category"`
	BugClass                  string          `json:"bug_class"`
	DefaultRiskLevel          string          `json:"default_risk_level"`
	DefaultSafetyLevel        *int            `json:"default_safety_level"`
	DefaultTestLevel          *int            `json:"default_test_level"`
	DefaultAutonomyLevel      *int            `json:"default_autonomy_level"`
	PermissionMode            string          `json:"permission_mode"`
	RequiredContext           json.RawMessage `json:"required_context"`
	TriggerSignals            json.RawMessage `json:"trigger_signals"`
	SupportedActions          json.RawMessage `json:"supported_actions"`
	SuccessCriteria           json.RawMessage `json:"success_criteria"`
	FailureCriteria           json.RawMessage `json:"failure_criteria"`
	MemoryPolicy              json.RawMessage `json:"memory_policy"`
	ExecutionProfile          json.RawMessage `json:"execution_profile"`
	AuthorizationRequirements json.RawMessage `json:"authorization_requirements"`
	BudgetDefaults            json.RawMessage `json:"budget_defaults"`
	StopConditions            json.RawMessage `json:"stop_conditions"`
	UserLearningPolicy        json.RawMessage `json:"user_learning_policy"`
	CustomDefinition          json.RawMessage `json:"custom_definition"`
	Metadata                  json.RawMessage `json:"metadata"`
	IsEnabled                 *bool           `json:"is_enabled"`
}

func operatorSkillJSONOrDefault(raw json.RawMessage, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	if json.Valid(raw) {
		return raw
	}
	return []byte(fallback)
}

func normalizeCustomOperatorSkillScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorSkillScopeProject:
		return models.OperatorSkillScopeProject
	case models.OperatorSkillScopeTarget:
		return models.OperatorSkillScopeTarget
	case models.OperatorSkillScopeUser, "":
		return models.OperatorSkillScopeUser
	default:
		return models.OperatorSkillScopeUser
	}
}

func normalizeOperatorSkillType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorSkillTypeAdvisory:
		return models.OperatorSkillTypeAdvisory
	case models.OperatorSkillTypeAnalysis:
		return models.OperatorSkillTypeAnalysis
	case models.OperatorSkillTypeActiveValidation:
		return models.OperatorSkillTypeActiveValidation
	case models.OperatorSkillTypeExploitRuntime:
		return models.OperatorSkillTypeExploitRuntime
	case models.OperatorSkillTypeChain:
		return models.OperatorSkillTypeChain
	case models.OperatorSkillTypePlanning, "":
		return models.OperatorSkillTypePlanning
	default:
		return models.OperatorSkillTypePlanning
	}
}

func normalizeOperatorSkillRuntimeBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorSkillRuntimeBackendInternalHTTP:
		return models.OperatorSkillRuntimeBackendInternalHTTP
	case models.OperatorSkillRuntimeBackendBrowser:
		return models.OperatorSkillRuntimeBackendBrowser
	case models.OperatorSkillRuntimeBackendShellRunner:
		return models.OperatorSkillRuntimeBackendShellRunner
	case models.OperatorSkillRuntimeBackendToolRunner:
		return models.OperatorSkillRuntimeBackendToolRunner
	case models.OperatorSkillRuntimeBackendCustomScript:
		return models.OperatorSkillRuntimeBackendCustomScript
	case models.OperatorSkillRuntimeBackendPayloadGenerator:
		return models.OperatorSkillRuntimeBackendPayloadGenerator
	case models.OperatorSkillRuntimeBackendBruteForce:
		return models.OperatorSkillRuntimeBackendBruteForce
	case models.OperatorSkillRuntimeBackendNone, "":
		return models.OperatorSkillRuntimeBackendNone
	default:
		return models.OperatorSkillRuntimeBackendNone
	}
}

func normalizeOperatorSkillRiskLevel(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "info", "informational":
		return "info"
	case "medium":
		return "medium"
	case "high":
		return "high"
	case "critical":
		return "critical"
	case "low", "":
		return "low"
	default:
		return "low"
	}
}

func normalizeOperatorSkillCategory(value string) string {
	category := strings.ToLower(strings.TrimSpace(value))
	if category == "" {
		return models.OperatorSkillCategoryExploitValidation
	}
	return category
}

func sanitizeOperatorSkillSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		isAllowed := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if isAllowed {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if r == '_' || r == '-' || r == ' ' || r == '.' || r == '/' {
			if !lastDash && b.Len() > 0 {
				b.WriteRune('_')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "_")
	if out == "" {
		out = "custom_skill"
	}
	return out
}

func userOperatorSkillSlug(uid uint, raw string) string {
	slug := sanitizeOperatorSkillSlug(raw)
	prefix := fmt.Sprintf("user_%d_", uid)
	if strings.HasPrefix(slug, prefix) {
		return slug
	}
	return prefix + slug
}

func parseOperatorSkillID(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("id"))
	if raw == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "operator skill id is required")
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid operator skill id")
	}

	return uint(id), nil
}

func operatorProfileJSONOrDefault(raw json.RawMessage, fallback string) []byte {
	if len(raw) == 0 {
		return []byte(fallback)
	}
	if json.Valid(raw) {
		return raw
	}
	return []byte(fallback)
}

func normalizeOperatorSkillPermissionMode(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorSkillPermissionScopeAwareAuthorized:
		return models.OperatorSkillPermissionScopeAwareAuthorized
	case models.OperatorSkillPermissionManualApproval:
		return models.OperatorSkillPermissionManualApproval
	case models.OperatorSkillPermissionAssistedAutopilot:
		return models.OperatorSkillPermissionAssistedAutopilot
	case models.OperatorSkillPermissionAuthorizedAutonomous:
		return models.OperatorSkillPermissionAuthorizedAutonomous
	default:
		return models.OperatorSkillPermissionScopeAwareAuthorized
	}
}

func defaultOperatorTargetSkillProfile(uid uint, ownerKey string, targetID uint) models.OperatorTargetSkillProfile {
	return models.OperatorTargetSkillProfile{
		UserID:                     uid,
		OwnerKey:                   ownerKey,
		TargetID:                   targetID,
		IsEnabled:                  true,
		PermissionMode:             models.OperatorSkillPermissionScopeAwareAuthorized,
		EnabledSkillSlugs:          []byte("[]"),
		DisabledSkillSlugs:         []byte("[]"),
		PreferredLearningRecordIDs: []byte("[]"),
		AllowedRuntimeBackends:     []byte("[]"),
		BudgetDefaults:             []byte("{}"),
		StopConditions:             []byte("{}"),
		Metadata:                   []byte("{}"),
	}
}

// GetTargetOperatorSkillProfile returns the authenticated user's skill usage
// profile for one accessible target. If no profile exists yet, default values
// are returned without creating a row.
func GetTargetOperatorSkillProfile(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	targetID, err := parseOperatorSkillTargetID(c)
	if err != nil {
		return err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	ownerKey := fmt.Sprintf("user:%d", uid)
	profile := defaultOperatorTargetSkillProfile(uid, ownerKey, target.ID)

	if err := database.DB.
		Where("target_id = ? AND user_id = ? AND owner_key = ?", target.ID, uid, ownerKey).
		FirstOrInit(&profile).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to load operator skill profile")
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"profile": profile,
	})
}

// UpsertTargetOperatorSkillProfile creates or updates the authenticated user's
// skill usage profile for one accessible target.
func UpsertTargetOperatorSkillProfile(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	targetID, err := parseOperatorSkillTargetID(c)
	if err != nil {
		return err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	var req updateOperatorTargetSkillProfileRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid operator skill profile payload")
	}

	ownerKey := fmt.Sprintf("user:%d", uid)
	profile := defaultOperatorTargetSkillProfile(uid, ownerKey, target.ID)

	if err := database.DB.
		Where("target_id = ? AND user_id = ? AND owner_key = ?", target.ID, uid, ownerKey).
		FirstOrCreate(&profile).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to initialize operator skill profile")
	}

	if req.IsEnabled != nil {
		profile.IsEnabled = *req.IsEnabled
	}

	profile.PermissionMode = normalizeOperatorSkillPermissionMode(req.PermissionMode)
	profile.EnabledSkillSlugs = operatorProfileJSONOrDefault(req.EnabledSkillSlugs, "[]")
	profile.DisabledSkillSlugs = operatorProfileJSONOrDefault(req.DisabledSkillSlugs, "[]")
	profile.PreferredLearningRecordIDs = operatorProfileJSONOrDefault(req.PreferredLearningRecordIDs, "[]")
	profile.AllowedRuntimeBackends = operatorProfileJSONOrDefault(req.AllowedRuntimeBackends, "[]")
	profile.BudgetDefaults = operatorProfileJSONOrDefault(req.BudgetDefaults, "{}")
	profile.StopConditions = operatorProfileJSONOrDefault(req.StopConditions, "{}")
	profile.Metadata = operatorProfileJSONOrDefault(req.Metadata, "{}")

	if err := database.DB.Save(&profile).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to save operator skill profile")
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"profile": profile,
	})
}

// ListOperatorSkills lists enabled built-in and future owner-scoped pentest skills.
// By default disabled skills are hidden. Use ?include_disabled=true to include them.
func ListOperatorSkills(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}
	ownerKey := fmt.Sprintf("user:%d", uid)

	includeDisabled := strings.EqualFold(strings.TrimSpace(c.Query("include_disabled")), "true")
	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	bugClass := strings.ToLower(strings.TrimSpace(c.Query("bug_class")))

	q := database.DB.Model(&models.OperatorSkill{}).
		Where("deleted_at IS NULL").
		Where("(is_built_in = true OR origin = ? AND created_by_user_id = ? OR owner_key = ?)", models.OperatorSkillOriginUser, uid, ownerKey)

	if !includeDisabled {
		q = q.Where("is_enabled = true")
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}
	if bugClass != "" {
		q = q.Where("bug_class = ?", bugClass)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count operator skills")
	}

	rows := make([]models.OperatorSkill, 0)
	if err := q.Order("is_built_in DESC, category ASC, slug ASC").Find(&rows).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list operator skills")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"count":  count,
		"skills": rows,
	})
}

// GetOperatorSkill returns one skill by slug.
func GetOperatorSkill(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}
	ownerKey := fmt.Sprintf("user:%d", uid)

	slug := strings.ToLower(strings.TrimSpace(c.Params("slug")))
	if slug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "skill slug is required")
	}

	includeDisabled := strings.EqualFold(strings.TrimSpace(c.Query("include_disabled")), "true")

	q := database.DB.Model(&models.OperatorSkill{}).
		Where("deleted_at IS NULL AND slug = ?", slug).
		Where("(is_built_in = true OR origin = ? AND created_by_user_id = ? OR owner_key = ?)", models.OperatorSkillOriginUser, uid, ownerKey)

	if !includeDisabled {
		q = q.Where("is_enabled = true")
	}

	var row models.OperatorSkill
	if err := q.First(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "operator skill not found")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"skill":  row,
	})
}

// CreateOperatorSkill creates a user-defined executable/plannable operator skill.
// This stores the definition only; runtime execution is wired in later patches.
func CreateOperatorSkill(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var req upsertOperatorSkillRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid operator skill payload")
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = strings.TrimSpace(req.Title)
	}
	if name == "" {
		return fiber.NewError(fiber.StatusBadRequest, "skill name is required")
	}

	rawSlug := strings.TrimSpace(req.Slug)
	if rawSlug == "" {
		rawSlug = name
	}
	slug := userOperatorSkillSlug(uid, rawSlug)
	ownerKey := fmt.Sprintf("user:%d", uid)

	var existing int64
	if err := database.DB.Model(&models.OperatorSkill{}).
		Where("deleted_at IS NULL AND slug = ?", slug).
		Count(&existing).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to check operator skill slug")
	}
	if existing > 0 {
		return fiber.NewError(fiber.StatusConflict, "operator skill slug already exists")
	}

	isEnabled := true
	if req.IsEnabled != nil {
		isEnabled = *req.IsEnabled
	}

	defaultSafetyLevel := 1
	if req.DefaultSafetyLevel != nil {
		defaultSafetyLevel = *req.DefaultSafetyLevel
	}
	defaultTestLevel := 1
	if req.DefaultTestLevel != nil {
		defaultTestLevel = *req.DefaultTestLevel
	}
	defaultAutonomyLevel := 1
	if req.DefaultAutonomyLevel != nil {
		defaultAutonomyLevel = *req.DefaultAutonomyLevel
	}

	row := models.OperatorSkill{
		OwnerKey:                  ownerKey,
		CreatedByUserID:           &uid,
		Scope:                     normalizeCustomOperatorSkillScope(req.Scope),
		Origin:                    models.OperatorSkillOriginUser,
		SkillType:                 normalizeOperatorSkillType(req.SkillType),
		RuntimeBackend:            normalizeOperatorSkillRuntimeBackend(req.RuntimeBackend),
		ProjectKey:                strings.TrimSpace(req.ProjectKey),
		TargetID:                  req.TargetID,
		Name:                      name,
		Slug:                      slug,
		Version:                   "v1",
		Category:                  normalizeOperatorSkillCategory(req.Category),
		BugClass:                  strings.ToLower(strings.TrimSpace(req.BugClass)),
		Description:               strings.TrimSpace(req.Description),
		DefaultRiskLevel:          normalizeOperatorSkillRiskLevel(req.DefaultRiskLevel),
		DefaultSafetyLevel:        defaultSafetyLevel,
		DefaultTestLevel:          defaultTestLevel,
		DefaultAutonomyLevel:      defaultAutonomyLevel,
		PermissionMode:            normalizeOperatorSkillPermissionMode(req.PermissionMode),
		RequiredContext:           operatorSkillJSONOrDefault(req.RequiredContext, "[]"),
		TriggerSignals:            operatorSkillJSONOrDefault(req.TriggerSignals, "[]"),
		SupportedActions:          operatorSkillJSONOrDefault(req.SupportedActions, "[]"),
		SuccessCriteria:           operatorSkillJSONOrDefault(req.SuccessCriteria, "[]"),
		FailureCriteria:           operatorSkillJSONOrDefault(req.FailureCriteria, "[]"),
		MemoryPolicy:              operatorSkillJSONOrDefault(req.MemoryPolicy, "{}"),
		ExecutionProfile:          operatorSkillJSONOrDefault(req.ExecutionProfile, "{}"),
		AuthorizationRequirements: operatorSkillJSONOrDefault(req.AuthorizationRequirements, "{}"),
		BudgetDefaults:            operatorSkillJSONOrDefault(req.BudgetDefaults, "{}"),
		StopConditions:            operatorSkillJSONOrDefault(req.StopConditions, "{}"),
		UserLearningPolicy:        operatorSkillJSONOrDefault(req.UserLearningPolicy, "{}"),
		CustomDefinition:          operatorSkillJSONOrDefault(req.CustomDefinition, "{}"),
		Metadata:                  operatorSkillJSONOrDefault(req.Metadata, "{}"),
		IsBuiltIn:                 false,
		IsEnabled:                 isEnabled,
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to create operator skill")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "success",
		"skill":  row,
	})
}

// UpdateOperatorSkill updates a user-owned custom operator skill definition.
// Built-in skills are immutable through this API.
func UpdateOperatorSkill(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	skillID, err := parseOperatorSkillID(c)
	if err != nil {
		return err
	}

	var row models.OperatorSkill
	if err := database.DB.
		Where("id = ? AND deleted_at IS NULL AND origin = ? AND created_by_user_id = ?", skillID, models.OperatorSkillOriginUser, uid).
		First(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user-defined operator skill not found")
	}

	var req upsertOperatorSkillRequest
	if err := json.Unmarshal(c.Body(), &req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid operator skill payload")
	}

	if strings.TrimSpace(req.Name) != "" {
		row.Name = strings.TrimSpace(req.Name)
	}
	if strings.TrimSpace(req.Title) != "" && strings.TrimSpace(req.Name) == "" {
		row.Name = strings.TrimSpace(req.Title)
	}
	if strings.TrimSpace(req.Description) != "" {
		row.Description = strings.TrimSpace(req.Description)
	}
	if strings.TrimSpace(req.Scope) != "" {
		row.Scope = normalizeCustomOperatorSkillScope(req.Scope)
	}
	if strings.TrimSpace(req.SkillType) != "" {
		row.SkillType = normalizeOperatorSkillType(req.SkillType)
	}
	if strings.TrimSpace(req.RuntimeBackend) != "" {
		row.RuntimeBackend = normalizeOperatorSkillRuntimeBackend(req.RuntimeBackend)
	}
	if req.ProjectKey != "" {
		row.ProjectKey = strings.TrimSpace(req.ProjectKey)
	}
	if req.TargetID != nil {
		row.TargetID = req.TargetID
	}
	if strings.TrimSpace(req.Category) != "" {
		row.Category = normalizeOperatorSkillCategory(req.Category)
	}
	if strings.TrimSpace(req.BugClass) != "" {
		row.BugClass = strings.ToLower(strings.TrimSpace(req.BugClass))
	}
	if strings.TrimSpace(req.DefaultRiskLevel) != "" {
		row.DefaultRiskLevel = normalizeOperatorSkillRiskLevel(req.DefaultRiskLevel)
	}
	if req.DefaultSafetyLevel != nil {
		row.DefaultSafetyLevel = *req.DefaultSafetyLevel
	}
	if req.DefaultTestLevel != nil {
		row.DefaultTestLevel = *req.DefaultTestLevel
	}
	if req.DefaultAutonomyLevel != nil {
		row.DefaultAutonomyLevel = *req.DefaultAutonomyLevel
	}
	if strings.TrimSpace(req.PermissionMode) != "" {
		row.PermissionMode = normalizeOperatorSkillPermissionMode(req.PermissionMode)
	}
	if len(req.RequiredContext) > 0 {
		row.RequiredContext = operatorSkillJSONOrDefault(req.RequiredContext, "[]")
	}
	if len(req.TriggerSignals) > 0 {
		row.TriggerSignals = operatorSkillJSONOrDefault(req.TriggerSignals, "[]")
	}
	if len(req.SupportedActions) > 0 {
		row.SupportedActions = operatorSkillJSONOrDefault(req.SupportedActions, "[]")
	}
	if len(req.SuccessCriteria) > 0 {
		row.SuccessCriteria = operatorSkillJSONOrDefault(req.SuccessCriteria, "[]")
	}
	if len(req.FailureCriteria) > 0 {
		row.FailureCriteria = operatorSkillJSONOrDefault(req.FailureCriteria, "[]")
	}
	if len(req.MemoryPolicy) > 0 {
		row.MemoryPolicy = operatorSkillJSONOrDefault(req.MemoryPolicy, "{}")
	}
	if len(req.ExecutionProfile) > 0 {
		row.ExecutionProfile = operatorSkillJSONOrDefault(req.ExecutionProfile, "{}")
	}
	if len(req.AuthorizationRequirements) > 0 {
		row.AuthorizationRequirements = operatorSkillJSONOrDefault(req.AuthorizationRequirements, "{}")
	}
	if len(req.BudgetDefaults) > 0 {
		row.BudgetDefaults = operatorSkillJSONOrDefault(req.BudgetDefaults, "{}")
	}
	if len(req.StopConditions) > 0 {
		row.StopConditions = operatorSkillJSONOrDefault(req.StopConditions, "{}")
	}
	if len(req.UserLearningPolicy) > 0 {
		row.UserLearningPolicy = operatorSkillJSONOrDefault(req.UserLearningPolicy, "{}")
	}
	if len(req.CustomDefinition) > 0 {
		row.CustomDefinition = operatorSkillJSONOrDefault(req.CustomDefinition, "{}")
	}
	if len(req.Metadata) > 0 {
		row.Metadata = operatorSkillJSONOrDefault(req.Metadata, "{}")
	}
	if req.IsEnabled != nil {
		row.IsEnabled = *req.IsEnabled
	}

	if err := database.DB.Save(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to update operator skill")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"skill":  row,
	})
}

// DeleteOperatorSkill soft-deletes a user-owned custom operator skill definition.
func DeleteOperatorSkill(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	skillID, err := parseOperatorSkillID(c)
	if err != nil {
		return err
	}

	var row models.OperatorSkill
	if err := database.DB.
		Where("id = ? AND deleted_at IS NULL AND origin = ? AND created_by_user_id = ?", skillID, models.OperatorSkillOriginUser, uid).
		First(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "user-defined operator skill not found")
	}

	if row.IsBuiltIn {
		return fiber.NewError(fiber.StatusForbidden, "built-in operator skills cannot be deleted")
	}

	if err := database.DB.Delete(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to delete operator skill")
	}

	return c.JSON(fiber.Map{
		"status":     "success",
		"deleted_id": skillID,
	})
}

func operatorSkillRunStatusGroup(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case models.OperatorSkillRunStatusCompleted:
		return "done"
	case models.OperatorSkillRunStatusNeedsContext, models.OperatorSkillRunStatusNeedsApproval:
		return "waiting"
	case models.OperatorSkillRunStatusPlanned, models.OperatorSkillRunStatusRunning:
		return "active"
	case models.OperatorSkillRunStatusFailed, models.OperatorSkillRunStatusBlockedByPolicy, models.OperatorSkillRunStatusCancelled:
		return "stopped"
	default:
		return "unknown"
	}
}

func operatorSkillRunNeedsContext(status string, output map[string]interface{}) bool {
	if strings.EqualFold(strings.TrimSpace(status), models.OperatorSkillRunStatusNeedsContext) {
		return true
	}

	if value, ok := output["needs_context"].(bool); ok {
		return value
	}

	return false
}

func operatorSkillRunMemoryItemID(output map[string]interface{}) interface{} {
	if value, ok := output["memory_item_id"]; ok {
		return value
	}

	if runsRaw, ok := output["runs"].([]interface{}); ok && len(runsRaw) > 0 {
		if first, ok := runsRaw[0].(map[string]interface{}); ok {
			if value, ok := first["memory_item_id"]; ok {
				return value
			}
		}
	}

	return nil
}

func operatorSkillRunOutputMap(row models.OperatorSkillRun) map[string]interface{} {
	out := map[string]interface{}{}
	if len(row.OutputJSON) > 0 {
		_ = json.Unmarshal(row.OutputJSON, &out)
	}
	return out
}

func enrichOperatorSkillRun(row models.OperatorSkillRun, observationCount int64, latestObservation *models.OperatorSkillObservation) fiber.Map {
	output := operatorSkillRunOutputMap(row)

	enriched := fiber.Map{
		"run":               row,
		"observation_count": observationCount,
		"has_observations":  observationCount > 0,
		"status_group":      operatorSkillRunStatusGroup(row.Status),
		"needs_context":     operatorSkillRunNeedsContext(row.Status, output),
		"memory_item_id":    operatorSkillRunMemoryItemID(output),
	}

	if latestObservation != nil {
		enriched["latest_observation"] = fiber.Map{
			"id":               latestObservation.ID,
			"created_at":       latestObservation.CreatedAt,
			"observation_type": latestObservation.ObservationType,
			"title":            latestObservation.Title,
			"summary":          latestObservation.Summary,
			"bug_class":        latestObservation.BugClass,
			"status":           latestObservation.Status,
			"severity":         latestObservation.Severity,
			"confidence":       latestObservation.Confidence,
			"url":              latestObservation.URL,
			"param_name":       latestObservation.ParamName,
		}
	} else {
		enriched["latest_observation"] = nil
	}

	return enriched
}

// GetTargetOperatorSkillRuns lists skill run records for an accessible target.
func GetTargetOperatorSkillRuns(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	targetID, err := parseOperatorSkillTargetID(c)
	if err != nil {
		return err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	skillSlug := strings.ToLower(strings.TrimSpace(c.Query("skill_slug")))
	status := normalizeOperatorSkillStatus(c.Query("status"))

	limit := c.QueryInt("limit", 50)
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	offset := c.QueryInt("offset", 0)
	if offset < 0 {
		offset = 0
	}

	q := database.DB.Model(&models.OperatorSkillRun{}).
		Where("target_id = ? AND user_id = ?", target.ID, uid)

	if skillSlug != "" {
		q = q.Where("skill_slug = ?", skillSlug)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count operator skill runs")
	}

	rows := make([]models.OperatorSkillRun, 0)
	if err := q.Order("created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list operator skill runs")
	}

	runIDs := make([]uint, 0, len(rows))
	for _, row := range rows {
		runIDs = append(runIDs, row.ID)
	}

	observationCounts := map[uint]int64{}
	latestObservations := map[uint]models.OperatorSkillObservation{}

	if len(runIDs) > 0 {
		type observationCountRow struct {
			SkillRunID uint  `json:"skill_run_id"`
			Count      int64 `json:"count"`
		}

		countRows := make([]observationCountRow, 0)
		_ = database.DB.Model(&models.OperatorSkillObservation{}).
			Select("skill_run_id, COUNT(*) as count").
			Where("target_id = ? AND user_id = ? AND skill_run_id IN ?", target.ID, uid, runIDs).
			Group("skill_run_id").
			Scan(&countRows).Error

		for _, countRow := range countRows {
			observationCounts[countRow.SkillRunID] = countRow.Count
		}

		latestRows := make([]models.OperatorSkillObservation, 0)
		_ = database.DB.
			Where("target_id = ? AND user_id = ? AND skill_run_id IN ?", target.ID, uid, runIDs).
			Order("skill_run_id ASC, created_at DESC, id DESC").
			Find(&latestRows).Error

		for _, observation := range latestRows {
			if _, exists := latestObservations[observation.SkillRunID]; !exists {
				latestObservations[observation.SkillRunID] = observation
			}
		}
	}

	enrichedRows := make([]fiber.Map, 0, len(rows))
	for _, row := range rows {
		var latest *models.OperatorSkillObservation
		if observation, ok := latestObservations[row.ID]; ok {
			obs := observation
			latest = &obs
		}
		enrichedRows = append(enrichedRows, enrichOperatorSkillRun(row, observationCounts[row.ID], latest))
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"count":  count,
		"limit":  limit,
		"offset": offset,
		"target": fiber.Map{
			"id":          target.ID,
			"root_domain": target.RootDomain,
		},
		"skill_runs":          rows,
		"skill_runs_enriched": enrichedRows,
	})
}

func parseOperatorSkillRunID(c *fiber.Ctx) (uint, error) {
	raw := strings.TrimSpace(c.Params("run_id"))
	if raw == "" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "skill run id is required")
	}

	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "invalid skill run id")
	}

	return uint(id), nil
}

// GetTargetOperatorSkillRunObservations lists observations created by a specific operator skill run.
func GetTargetOperatorSkillRunObservations(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	targetID, err := parseOperatorSkillTargetID(c)
	if err != nil {
		return err
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	runID, err := parseOperatorSkillRunID(c)
	if err != nil {
		return err
	}

	var run models.OperatorSkillRun
	if err := database.DB.
		Where("id = ? AND target_id = ? AND user_id = ?", runID, target.ID, uid).
		First(&run).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "operator skill run not found")
	}

	observationType := strings.ToLower(strings.TrimSpace(c.Query("observation_type")))
	bugClass := strings.ToLower(strings.TrimSpace(c.Query("bug_class")))

	limit := c.QueryInt("limit", 50)
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}

	offset := c.QueryInt("offset", 0)
	if offset < 0 {
		offset = 0
	}

	q := database.DB.Model(&models.OperatorSkillObservation{}).
		Where("target_id = ? AND user_id = ? AND skill_run_id = ?", target.ID, uid, run.ID)

	if observationType != "" {
		q = q.Where("observation_type = ?", observationType)
	}
	if bugClass != "" {
		q = q.Where("bug_class = ?", bugClass)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count operator skill observations")
	}

	rows := make([]models.OperatorSkillObservation, 0)
	if err := q.Order("confidence DESC, created_at DESC, id DESC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list operator skill observations")
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"count":  count,
		"limit":  limit,
		"offset": offset,
		"target": fiber.Map{
			"id":          target.ID,
			"root_domain": target.RootDomain,
		},
		"skill_run": fiber.Map{
			"id":         run.ID,
			"skill_slug": run.SkillSlug,
			"status":     run.Status,
		},
		"observations": rows,
	})
}
