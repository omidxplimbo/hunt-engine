package handlers

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
	"gorm.io/gorm"
)

func toFindingResponse(f models.Finding) dto.FindingResponse {
	return dto.FindingResponse{
		ID:             f.ID,
		TargetID:       f.TargetID,
		AssetID:        f.AssetID,
		URLID:          f.URLID,
		Title:          f.Title,
		Description:    f.Description,
		Severity:       f.Severity,
		Category:       f.Category,
		SourceTool:     f.SourceTool,
		Evidence:       f.Evidence,
		Recommendation: f.Recommendation,
		Status:         f.Status,
		Fingerprint:    f.Fingerprint,
		FirstSeen:      f.FirstSeen,
		LastSeen:       f.LastSeen,
		CreatedAt:      f.CreatedAt,
		UpdatedAt:      f.UpdatedAt,
	}
}

func normalizeFindingStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func isValidFindingStatus(status string) bool {
	switch normalizeFindingStatus(status) {
	case models.FindingStatusOpen,
		models.FindingStatusAccepted,
		models.FindingStatusFalsePositive,
		models.FindingStatusFixed:
		return true
	default:
		return false
	}
}

func parseFindingPagination(c *fiber.Ctx) (int, int, int) {
	limit := utils.DefaultLimit
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	if limit > 500 {
		limit = 500
	}

	offset := 0
	page := 1
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
		page = (offset / limit) + 1
	} else if p, err := strconv.Atoi(c.Query("page", "1")); err == nil && p > 0 {
		page = p
		offset = (page - 1) * limit
	}

	return limit, offset, page
}

func applyFindingFilters(db *gorm.DB, c *fiber.Ctx) *gorm.DB {
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		db = db.Where("status = ?", normalizeFindingStatus(status))
	}
	if severity := strings.TrimSpace(c.Query("severity")); severity != "" {
		db = db.Where("severity = ?", strings.ToLower(severity))
	}
	if source := strings.TrimSpace(c.Query("source_tool")); source != "" {
		db = db.Where("source_tool = ?", strings.ToLower(source))
	}
	if category := strings.TrimSpace(c.Query("category")); category != "" {
		db = db.Where("category = ?", strings.ToLower(category))
	}
	if search := strings.TrimSpace(c.Query("search")); search != "" {
		like := "%" + search + "%"
		db = db.Where("title ILIKE ? OR description ILIKE ? OR evidence ILIKE ?", like, like, like)
	}
	return db
}

// GetFindings lists findings across all accessible targets.
func GetFindings(c *fiber.Ctx) error {
	limit, offset, page := parseFindingPagination(c)

	db := database.DB.Model(&models.Finding{})

	if !isAdmin(c) {
		uid, err := currentUserID(c)
		if err != nil {
			return err
		}
		db = db.Joins("JOIN targets ON targets.id = findings.target_id").Where("targets.created_by_user_id = ?", uid)
	}

	if targetIDRaw := strings.TrimSpace(c.Query("target_id")); targetIDRaw != "" {
		var targetID uint
		if _, err := fmt.Sscanf(targetIDRaw, "%d", &targetID); err != nil || targetID == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid target_id"})
		}
		if _, err := getAccessibleTarget(c, targetID); err != nil {
			return err
		}
		db = db.Where("findings.target_id = ?", targetID)
	}

	db = applyFindingFilters(db, c)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var findings []models.Finding
	if err := db.Order("severity desc, last_seen desc, id desc").Limit(limit).Offset(offset).Find(&findings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	responses := make([]dto.FindingResponse, len(findings))
	for i, f := range findings {
		responses[i] = toFindingResponse(f)
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"data":        responses,
		"count":       len(responses),
		"total_count": total,
		"page":        page,
	})
}

// GetTargetFindings lists findings for one accessible target.
func GetTargetFindings(c *fiber.Ctx) error {
	id := c.Params("id")
	var targetID uint
	_, _ = fmt.Sscanf(id, "%d", &targetID)

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	limit, offset, page := parseFindingPagination(c)
	db := database.DB.Model(&models.Finding{}).Where("target_id = ?", target.ID)
	db = applyFindingFilters(db, c)

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var findings []models.Finding
	if err := db.Order("severity desc, last_seen desc, id desc").Limit(limit).Offset(offset).Find(&findings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	responses := make([]dto.FindingResponse, len(findings))
	for i, f := range findings {
		responses[i] = toFindingResponse(f)
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"data":        responses,
		"count":       len(responses),
		"total_count": total,
		"page":        page,
	})
}

// UpdateFindingStatus updates triage status for a finding after checking target ownership.
func UpdateFindingStatus(c *fiber.Ctx) error {
	id := c.Params("id")
	var findingID uint
	_, _ = fmt.Sscanf(id, "%d", &findingID)
	if findingID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid finding id"})
	}

	req := new(dto.UpdateFindingStatusRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	status := normalizeFindingStatus(req.Status)
	if !isValidFindingStatus(status) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid finding status"})
	}

	var finding models.Finding
	if err := database.DB.First(&finding, findingID).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Finding not found"})
	}

	if _, err := getAccessibleTarget(c, finding.TargetID); err != nil {
		return err
	}

	if err := database.DB.Model(&finding).Update("status", status).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	finding.Status = status
	return c.JSON(fiber.Map{"status": "success", "message": "Finding status updated", "data": toFindingResponse(finding)})
}
