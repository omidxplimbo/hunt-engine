package handlers

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type targetPolicyRequest struct {
	PlatformName string `json:"platform_name"`
	ProgramURL   string `json:"program_url"`

	InScopePatterns    []string `json:"in_scope_patterns"`
	OutOfScopePatterns []string `json:"out_of_scope_patterns"`

	AllowedTestTypes    []string `json:"allowed_test_types"`
	DisallowedTestTypes []string `json:"disallowed_test_types"`

	MaxTestIntensity string `json:"max_test_intensity"`
	RateLimitNotes   string `json:"rate_limit_notes"`
	AuthRequired     bool   `json:"auth_required"`
	SafeTestingNotes string `json:"safe_testing_notes"`

	ReportingPreferences string `json:"reporting_preferences"`
	BusinessContext      string `json:"business_context"`

	AssetCriticalityDefault string `json:"asset_criticality_default"`
}

func GetTargetPolicy(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	_, featureOwnerKey, _, _, ownerErr := currentAccountOwner(c)
	if ownerErr != nil {
		return ownerErr
	}
	if !featureflags.IsEnabledForOwner(featureOwnerKey, featureflags.KeyTargetPolicy) {
		return featureflags.DisabledResponse(c, featureflags.KeyTargetPolicy)
	}

	var row models.TargetPolicy
	if err := database.DB.Where("target_id = ?", target.ID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.JSON(fiber.Map{
				"status": "success",
				"data":   nil,
			})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   row,
	})
}

func PutTargetPolicy(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	_, featureOwnerKey, _, _, ownerErr := currentAccountOwner(c)
	if ownerErr != nil {
		return ownerErr
	}
	if !featureflags.IsEnabledForOwner(featureOwnerKey, featureflags.KeyTargetPolicy) {
		return featureflags.DisabledResponse(c, featureflags.KeyTargetPolicy)
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := targetPolicyRequest{}
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body"})
	}

	req.PlatformName = limitString(cleanPolicyText(req.PlatformName), 128)
	req.ProgramURL = limitString(cleanPolicyText(req.ProgramURL), 1024)
	req.MaxTestIntensity = normalizePolicyChoice(req.MaxTestIntensity, "safe", map[string]bool{
		"passive":         true,
		"safe":            true,
		"balanced":        true,
		"manual-approved": true,
	})
	req.AssetCriticalityDefault = normalizePolicyChoice(req.AssetCriticalityDefault, "medium", map[string]bool{
		"low":      true,
		"medium":   true,
		"high":     true,
		"critical": true,
	})

	req.RateLimitNotes = limitString(cleanPolicyText(req.RateLimitNotes), 8000)
	req.SafeTestingNotes = limitString(cleanPolicyText(req.SafeTestingNotes), 8000)
	req.ReportingPreferences = limitString(cleanPolicyText(req.ReportingPreferences), 8000)
	req.BusinessContext = limitString(cleanPolicyText(req.BusinessContext), 8000)

	inScope := jsonStringArray(sanitizePolicyList(req.InScopePatterns, 250, 512))
	outScope := jsonStringArray(sanitizePolicyList(req.OutOfScopePatterns, 250, 512))
	allowed := jsonStringArray(sanitizePolicyList(req.AllowedTestTypes, 100, 128))
	disallowed := jsonStringArray(sanitizePolicyList(req.DisallowedTestTypes, 100, 128))

	var row models.TargetPolicy
	isCreate := false

	err = database.DB.Where("target_id = ?", target.ID).First(&row).Error
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}

		isCreate = true
		row = models.TargetPolicy{
			TargetID:        target.ID,
			CreatedByUserID: &uid,
		}
	}

	row.PlatformName = req.PlatformName
	row.ProgramURL = req.ProgramURL
	row.InScopePatterns = inScope
	row.OutOfScopePatterns = outScope
	row.AllowedTestTypes = allowed
	row.DisallowedTestTypes = disallowed
	row.MaxTestIntensity = req.MaxTestIntensity
	row.RateLimitNotes = req.RateLimitNotes
	row.AuthRequired = req.AuthRequired
	row.SafeTestingNotes = req.SafeTestingNotes
	row.ReportingPreferences = req.ReportingPreferences
	row.BusinessContext = req.BusinessContext
	row.AssetCriticalityDefault = req.AssetCriticalityDefault

	if isCreate {
		if err := database.DB.Create(&row).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
	} else {
		if err := database.DB.Save(&row).Error; err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
	}

	entityID := row.ID
	action := "target.policy.update"
	statusCode := fiber.StatusOK
	if isCreate {
		action = "target.policy.create"
		statusCode = fiber.StatusCreated
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      action,
		EntityType:  "target_policy",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":                   target.ID,
			"root_domain":                 target.RootDomain,
			"platform_name":               row.PlatformName,
			"max_test_intensity":          row.MaxTestIntensity,
			"auth_required":               row.AuthRequired,
			"asset_criticality_default":   row.AssetCriticalityDefault,
			"in_scope_patterns_count":     len(req.InScopePatterns),
			"out_of_scope_patterns_count": len(req.OutOfScopePatterns),
			"allowed_test_types_count":    len(req.AllowedTestTypes),
			"disallowed_test_types_count": len(req.DisallowedTestTypes),
		},
	})

	return c.Status(statusCode).JSON(fiber.Map{
		"status": "success",
		"data":   row,
	})
}

func DeleteTargetPolicy(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	_, featureOwnerKey, _, _, ownerErr := currentAccountOwner(c)
	if ownerErr != nil {
		return ownerErr
	}
	if !featureflags.IsEnabledForOwner(featureOwnerKey, featureflags.KeyTargetPolicy) {
		return featureflags.DisabledResponse(c, featureflags.KeyTargetPolicy)
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var row models.TargetPolicy
	if err := database.DB.Where("target_id = ?", target.ID).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target policy not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	entityID := row.ID
	if err := database.DB.Delete(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.policy.delete",
		EntityType:  "target_policy",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":   target.ID,
			"root_domain": target.RootDomain,
		},
	})

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Target policy deleted",
	})
}

func cleanPolicyText(value string) string {
	return strings.TrimSpace(strings.ReplaceAll(value, "\x00", ""))
}

func limitString(value string, max int) string {
	if max <= 0 || len(value) <= max {
		return value
	}
	return value[:max]
}

func sanitizePolicyList(values []string, maxItems int, maxLen int) []string {
	out := make([]string, 0, len(values))
	seen := make(map[string]bool)

	for _, value := range values {
		item := limitString(cleanPolicyText(value), maxLen)
		if item == "" {
			continue
		}

		key := strings.ToLower(item)
		if seen[key] {
			continue
		}
		seen[key] = true

		out = append(out, item)
		if maxItems > 0 && len(out) >= maxItems {
			break
		}
	}

	return out
}

func jsonStringArray(values []string) datatypes.JSON {
	raw, err := json.Marshal(values)
	if err != nil {
		return datatypes.JSON([]byte("[]"))
	}
	return datatypes.JSON(raw)
}

func normalizePolicyChoice(value string, fallback string, allowed map[string]bool) string {
	v := strings.ToLower(strings.TrimSpace(value))
	if allowed[v] {
		return v
	}
	return fallback
}
