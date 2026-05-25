package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/gorm"
)

func parseDataLayerPagination(c *fiber.Ctx) (int, int, int) {
	limit := 50
	if raw := strings.TrimSpace(c.Query("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 250 {
		limit = 250
	}

	offset := 0
	page := 1
	if raw := strings.TrimSpace(c.Query("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v >= 0 {
			offset = v
			page = (offset / limit) + 1
		}
	} else if raw := strings.TrimSpace(c.Query("page")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			page = v
			offset = (page - 1) * limit
		}
	}

	return limit, offset, page
}

func parseTargetIDParam(c *fiber.Ctx) (uint, error) {
	var targetID uint
	_, _ = fmt.Sscanf(c.Params("id"), "%d", &targetID)
	if targetID == 0 {
		return 0, fmt.Errorf("invalid target id")
	}
	return targetID, nil
}

func applyAIAnalysisFilters(db *gorm.DB, c *fiber.Ctx) *gorm.DB {
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		db = db.Where("status = ?", status)
	}
	if scope := strings.ToLower(strings.TrimSpace(c.Query("scope"))); scope != "" {
		db = db.Where("scope = ?", scope)
	}
	if source := strings.ToLower(strings.TrimSpace(c.Query("source"))); source != "" {
		db = db.Where("source = ?", source)
	}
	if provider := strings.TrimSpace(c.Query("provider")); provider != "" {
		db = db.Where("provider = ?", provider)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		db = db.Where("title ILIKE ? OR summary ILIKE ? OR error_message ILIKE ?", like, like, like)
	}
	return db
}

func applyAIRecommendationFilters(db *gorm.DB, c *fiber.Ctx) *gorm.DB {
	if status := strings.ToLower(strings.TrimSpace(c.Query("status"))); status != "" {
		db = db.Where("status = ?", status)
	}
	if priority := strings.ToLower(strings.TrimSpace(c.Query("priority"))); priority != "" {
		db = db.Where("priority = ?", priority)
	}
	if confidence := strings.ToLower(strings.TrimSpace(c.Query("confidence"))); confidence != "" {
		db = db.Where("confidence = ?", confidence)
	}
	if source := strings.ToLower(strings.TrimSpace(c.Query("source"))); source != "" {
		db = db.Where("source = ?", source)
	}
	if recType := strings.ToLower(strings.TrimSpace(c.Query("recommendation_type"))); recType != "" {
		db = db.Where("recommendation_type = ?", recType)
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		db = db.Where("title ILIKE ? OR description ILIKE ? OR rationale ILIKE ?", like, like, like)
	}
	return db
}

// GetTargetAIAnalyses lists AI analysis rows for one accessible target.
// v3.5.0 foundation endpoint: generation is added in later milestones.
func GetTargetAIAnalyses(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)
	db := database.DB.Model(&models.AIAnalysis{}).Where("target_id = ?", target.ID)
	db = applyAIAnalysisFilters(db, c)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AIAnalysis, 0)
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

// GetTargetAIRecommendations lists AI recommendation rows for one accessible target.
func GetTargetAIRecommendations(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)
	db := database.DB.Model(&models.AIRecommendation{}).Where("target_id = ?", target.ID)
	db = applyAIRecommendationFilters(db, c)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AIRecommendation, 0)
	if err := db.Order("priority desc, created_at desc, id desc").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
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

// GetTargetAuditLogs lists audit records tied to one accessible target.
func GetTargetAuditLogs(c *fiber.Ctx) error {
	targetID, err := parseTargetIDParam(c)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)
	db := database.DB.Model(&models.AuditLog{}).Where("target_id = ?", target.ID)

	if action := strings.TrimSpace(c.Query("action")); action != "" {
		db = db.Where("action = ?", action)
	}
	if entityType := strings.TrimSpace(c.Query("entity_type")); entityType != "" {
		db = db.Where("entity_type = ?", entityType)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.AuditLog, 0)
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
