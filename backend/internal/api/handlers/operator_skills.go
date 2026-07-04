package handlers

import (
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

// ListOperatorSkills lists enabled built-in and future owner-scoped pentest skills.
// By default disabled skills are hidden. Use ?include_disabled=true to include them.
func ListOperatorSkills(c *fiber.Ctx) error {
	if _, err := currentUserID(c); err != nil {
		return err
	}

	includeDisabled := strings.EqualFold(strings.TrimSpace(c.Query("include_disabled")), "true")
	category := strings.ToLower(strings.TrimSpace(c.Query("category")))
	bugClass := strings.ToLower(strings.TrimSpace(c.Query("bug_class")))

	q := database.DB.Model(&models.OperatorSkill{}).
		Where("deleted_at IS NULL")

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
	if _, err := currentUserID(c); err != nil {
		return err
	}

	slug := strings.ToLower(strings.TrimSpace(c.Params("slug")))
	if slug == "" {
		return fiber.NewError(fiber.StatusBadRequest, "skill slug is required")
	}

	includeDisabled := strings.EqualFold(strings.TrimSpace(c.Query("include_disabled")), "true")

	q := database.DB.Model(&models.OperatorSkill{}).
		Where("deleted_at IS NULL AND slug = ?", slug)

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

	return c.JSON(fiber.Map{
		"status": "success",
		"count":  count,
		"limit":  limit,
		"offset": offset,
		"target": fiber.Map{
			"id":          target.ID,
			"root_domain": target.RootDomain,
		},
		"skill_runs": rows,
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
