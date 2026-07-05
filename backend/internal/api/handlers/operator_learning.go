package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

func normalizeOperatorLearningScope(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorLearningScopeTarget:
		return models.OperatorLearningScopeTarget
	case models.OperatorLearningScopeProject:
		return models.OperatorLearningScopeProject
	case models.OperatorLearningScopeUserGlobal:
		return models.OperatorLearningScopeUserGlobal
	case models.OperatorLearningScopeOrganization:
		return models.OperatorLearningScopeOrganization
	default:
		return ""
	}
}

func normalizeOperatorLearningSource(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorLearningSourceUserConfirmed:
		return models.OperatorLearningSourceUserConfirmed
	case models.OperatorLearningSourceSkillResult:
		return models.OperatorLearningSourceSkillResult
	case models.OperatorLearningSourceOperatorInference:
		return models.OperatorLearningSourceOperatorInference
	default:
		return ""
	}
}

func normalizeOperatorLearningStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.OperatorLearningStatusActive:
		return models.OperatorLearningStatusActive
	case models.OperatorLearningStatusDisabled:
		return models.OperatorLearningStatusDisabled
	case models.OperatorLearningStatusSuperseded:
		return models.OperatorLearningStatusSuperseded
	default:
		return ""
	}
}

// ListOperatorLearningRecords lists the authenticated user's reusable operator
// learning records. These records are personal/project/target methodology and
// execution hints that can later influence skill selection across projects.
func ListOperatorLearningRecords(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	scope := normalizeOperatorLearningScope(c.Query("scope"))
	source := normalizeOperatorLearningSource(c.Query("source"))
	status := normalizeOperatorLearningStatus(c.Query("status"))
	if status == "" {
		status = models.OperatorLearningStatusActive
	}

	bugClass := strings.ToLower(strings.TrimSpace(c.Query("bug_class")))
	skillSlug := strings.ToLower(strings.TrimSpace(c.Query("skill_slug")))
	projectKey := strings.TrimSpace(c.Query("project_key"))

	var targetID uint64
	targetIDRaw := strings.TrimSpace(c.Query("target_id"))
	if targetIDRaw != "" {
		parsed, parseErr := strconv.ParseUint(targetIDRaw, 10, 64)
		if parseErr != nil || parsed == 0 {
			return fiber.NewError(fiber.StatusBadRequest, "invalid target_id")
		}
		targetID = parsed
	}

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

	q := database.DB.Model(&models.OperatorLearningRecord{}).
		Where("deleted_at IS NULL AND user_id = ?", uid)

	if scope != "" {
		q = q.Where("scope = ?", scope)
	}
	if source != "" {
		q = q.Where("source = ?", source)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if bugClass != "" {
		q = q.Where("bug_class = ?", bugClass)
	}
	if skillSlug != "" {
		q = q.Where("skill_slug = ?", skillSlug)
	}
	if projectKey != "" {
		q = q.Where("project_key = ?", projectKey)
	}
	if targetID > 0 {
		q = q.Where("target_id = ?", targetID)
	}

	var count int64
	if err := q.Count(&count).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to count operator learning records")
	}

	rows := make([]models.OperatorLearningRecord, 0)
	if err := q.Order("last_used_at DESC NULLS LAST, confidence DESC, created_at DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&rows).Error; err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to list operator learning records")
	}

	return c.JSON(fiber.Map{
		"status":   "success",
		"count":    count,
		"limit":    limit,
		"offset":   offset,
		"learning": rows,
	})
}

// GetOperatorLearningRecord returns one learning record owned by the current user.
func GetOperatorLearningRecord(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	raw := strings.TrimSpace(c.Params("id"))
	if raw == "" {
		return fiber.NewError(fiber.StatusBadRequest, "learning record id is required")
	}

	id, parseErr := strconv.ParseUint(raw, 10, 64)
	if parseErr != nil || id == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "invalid learning record id")
	}

	var row models.OperatorLearningRecord
	if err := database.DB.
		Where("deleted_at IS NULL AND user_id = ? AND id = ?", uid, id).
		First(&row).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "operator learning record not found")
	}

	return c.JSON(fiber.Map{
		"status":   "success",
		"learning": row,
	})
}
