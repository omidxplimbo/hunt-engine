package handlers

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
	"gorm.io/gorm"
)

func applyAgentRunFilters(db *gorm.DB, c *fiber.Ctx) *gorm.DB {
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		db = db.Where("status = ?", status)
	}

	if agentType := strings.ToLower(strings.TrimSpace(c.Query("agent_type"))); agentType != "" {
		db = db.Where("agent_type = ?", agentType)
	}

	if source := strings.ToLower(strings.TrimSpace(c.Query("source"))); source != "" {
		db = db.Where("source = ?", source)
	}

	if policyStatus := strings.ToLower(strings.TrimSpace(c.Query("policy_status"))); policyStatus != "" {
		db = db.Where("policy_status = ?", policyStatus)
	}

	if provider := strings.TrimSpace(c.Query("provider")); provider != "" {
		db = db.Where("provider = ?", provider)
	}

	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		db = db.Where("agent_type ILIKE ? OR provider ILIKE ? OR model ILIKE ? OR error_message ILIKE ?", like, like, like, like)
	}

	return db
}

// GetTargetAgentRuns lists agent execution records for one accessible target.
// v3.6.0 foundation endpoint: later triage/summary/report agents will populate this table.
func GetTargetAgentRuns(c *fiber.Ctx) error {
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

	if !featureflags.IsEnabledForOwner(featureOwnerKey, featureflags.KeyAgentRuns) {
		return featureflags.DisabledResponse(c, featureflags.KeyAgentRuns)
	}

	limit, offset, page := parseDataLayerPagination(c)

	db := database.DB.Model(&models.AgentRun{}).Where("target_id = ?", target.ID)
	db = applyAgentRunFilters(db, c)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AgentRun, 0)
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
