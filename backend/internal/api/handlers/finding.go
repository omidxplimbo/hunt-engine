package handlers

import (
	"bytes"
	"encoding/csv"
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

type findingStatsRow struct {
	Key   string `gorm:"column:key"`
	Count int64  `gorm:"column:count"`
}

func emptyFindingStats() dto.FindingStatsResponse {
	return dto.FindingStatsResponse{
		BySeverity: map[string]int64{
			models.FindingSeverityCritical: 0,
			models.FindingSeverityHigh:     0,
			models.FindingSeverityMedium:   0,
			models.FindingSeverityLow:      0,
			models.FindingSeverityInfo:     0,
		},
		ByStatus: map[string]int64{
			models.FindingStatusOpen:          0,
			models.FindingStatusAccepted:      0,
			models.FindingStatusFalsePositive: 0,
			models.FindingStatusFixed:         0,
		},
		BySource:   map[string]int64{},
		ByCategory: map[string]int64{},
	}
}

func countFindingsBy(base *gorm.DB, field string) (map[string]int64, error) {
	rows := make([]findingStatsRow, 0)
	query := base.Session(&gorm.Session{}).
		Select(field + " AS key, COUNT(*) AS count").
		Where(field + " <> ''").
		Group(field)

	if err := query.Scan(&rows).Error; err != nil {
		return nil, err
	}

	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.Key] = row.Count
	}

	return result, nil
}

func buildFindingStats(base *gorm.DB) (dto.FindingStatsResponse, error) {
	stats := emptyFindingStats()

	if err := base.Session(&gorm.Session{}).Count(&stats.Total).Error; err != nil {
		return stats, err
	}

	bySeverity, err := countFindingsBy(base, "severity")
	if err != nil {
		return stats, err
	}
	for key, count := range bySeverity {
		stats.BySeverity[key] = count
	}

	byStatus, err := countFindingsBy(base, "status")
	if err != nil {
		return stats, err
	}
	for key, count := range byStatus {
		stats.ByStatus[key] = count
	}

	stats.Open = stats.ByStatus[models.FindingStatusOpen]
	stats.Accepted = stats.ByStatus[models.FindingStatusAccepted]
	stats.FalsePositive = stats.ByStatus[models.FindingStatusFalsePositive]
	stats.Fixed = stats.ByStatus[models.FindingStatusFixed]

	stats.BySource, err = countFindingsBy(base, "source_tool")
	if err != nil {
		return stats, err
	}

	stats.ByCategory, err = countFindingsBy(base, "category")
	if err != nil {
		return stats, err
	}

	return stats, nil
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

// GetTargetFindingsStats returns aggregate finding counts for one accessible target.
func GetTargetFindingsStats(c *fiber.Ctx) error {
	id := c.Params("id")
	var targetID uint
	_, _ = fmt.Sscanf(id, "%d", &targetID)

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	base := database.DB.Model(&models.Finding{}).Where("target_id = ?", target.ID)
	stats, err := buildFindingStats(base)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "data": stats})
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

// ExportTargetFindings exports findings for a single accessible target as CSV or JSON.
func ExportTargetFindings(c *fiber.Ctx) error {
	id := c.Params("id")
	var targetID uint
	_, _ = fmt.Sscanf(id, "%d", &targetID)

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	format := strings.ToLower(strings.TrimSpace(c.Query("format", "csv")))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "json" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid export format. Use csv or json"})
	}

	db := database.DB.Model(&models.Finding{}).Where("target_id = ?", target.ID)
	db = applyFindingFilters(db, c)

	var findings []models.Finding
	if err := db.Order("severity desc, last_seen desc, id desc").Find(&findings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	responses := make([]dto.FindingResponse, len(findings))
	for i, finding := range findings {
		responses[i] = toFindingResponse(finding)
	}

	if format == "json" {
		c.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSONCharsetUTF8)
		c.Attachment(fmt.Sprintf("target-%d-findings.json", target.ID))
		return c.JSON(fiber.Map{
			"status": "success",
			"data":   responses,
			"count":  len(responses),
		})
	}

	var buffer bytes.Buffer
	writer := csv.NewWriter(&buffer)

	header := []string{
		"id",
		"target_id",
		"asset_id",
		"url_id",
		"title",
		"severity",
		"status",
		"category",
		"source_tool",
		"evidence",
		"recommendation",
		"fingerprint",
		"first_seen",
		"last_seen",
		"created_at",
		"updated_at",
	}
	if err := writer.Write(header); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	for _, finding := range responses {
		assetID := ""
		if finding.AssetID != nil {
			assetID = strconv.FormatUint(uint64(*finding.AssetID), 10)
		}

		urlID := ""
		if finding.URLID != nil {
			urlID = strconv.FormatUint(uint64(*finding.URLID), 10)
		}

		record := []string{
			strconv.FormatUint(uint64(finding.ID), 10),
			strconv.FormatUint(uint64(finding.TargetID), 10),
			assetID,
			urlID,
			finding.Title,
			finding.Severity,
			finding.Status,
			finding.Category,
			finding.SourceTool,
			finding.Evidence,
			finding.Recommendation,
			finding.Fingerprint,
			finding.FirstSeen.Format(time.RFC3339),
			finding.LastSeen.Format(time.RFC3339),
			finding.CreatedAt.Format(time.RFC3339),
			finding.UpdatedAt.Format(time.RFC3339),
		}

		if err := writer.Write(record); err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	c.Set(fiber.HeaderContentType, "text/csv; charset=utf-8")
	c.Attachment(fmt.Sprintf("target-%d-findings.csv", target.ID))
	return c.Send(buffer.Bytes())
}
