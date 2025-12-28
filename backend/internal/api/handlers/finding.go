package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

func toFindingResponse(f models.Finding) dto.FindingResponse {
	var tags interface{} = []string{}
	if f.Tags != "" {
		_ = json.Unmarshal([]byte(f.Tags), &tags)
	}
	var evidence interface{} = map[string]interface{}{}
	if f.Evidence != "" {
		_ = json.Unmarshal([]byte(f.Evidence), &evidence)
	}
	return dto.FindingResponse{
		ID:        f.ID,
		CreatedAt: f.CreatedAt,
		UpdatedAt: f.UpdatedAt,
		TargetID:  f.TargetID,
		AssetID:   f.AssetID,
		URLID:     f.URLID,
		Title:     f.Title,
		Type:      f.Type,
		Severity:  f.Severity,
		Status:    f.Status,
		Hash:      f.Hash,
		TemplateID:  f.TemplateID,
		MatcherName: f.MatcherName,
		MatchedAt:   f.MatchedAt,
		Host:        f.Host,
		CurlCommand: f.CurlCommand,
		Tags:       tags,
		Evidence:   evidence,
		Notes:      f.Notes,
		ReportedTo: f.ReportedTo,
	}
}

// GetTargetFindings lists findings for a target (paginated + filters).
func GetTargetFindings(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)

	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 && l <= 500 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	}

	search := strings.TrimSpace(c.Query("search"))
	severity := strings.TrimSpace(strings.ToLower(c.Query("severity")))
	status := strings.TrimSpace(strings.ToLower(c.Query("status")))
	fType := strings.TrimSpace(strings.ToLower(c.Query("type")))

	db := database.DB.Model(&models.Finding{}).Where("target_id = ?", target.ID)
	if search != "" {
		like := "%" + search + "%"
		db = db.Where("(title ILIKE ? OR template_id ILIKE ? OR matched_at ILIKE ? OR host ILIKE ?)", like, like, like, like)
	}
	if severity != "" {
		db = db.Where("severity = ?", severity)
	}
	if status != "" {
		db = db.Where("status = ?", status)
	}
	if fType != "" {
		db = db.Where("type = ?", fType)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var findings []models.Finding
	if err := db.Order("created_at desc").Limit(limit).Offset(offset).Find(&findings).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	resp := make([]dto.FindingResponse, len(findings))
	for i := range findings {
		resp[i] = toFindingResponse(findings[i])
	}

	return c.JSON(dto.ListFindingsResponse{
		Status:     "success",
		Data:       resp,
		PageCount:  len(resp),
		TotalCount: total,
	})
}

// UpdateFinding updates triage fields for a finding (status/notes/reported_to).
func UpdateFinding(c *fiber.Ctx) error {
	id := c.Params("id")
	findingID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &findingID)

	req := new(dto.UpdateFindingRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	var f models.Finding
	if err := database.DB.First(&f, findingID).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "Finding not found")
	}

	// enforce ownership via target access
	if _, err := getAccessibleTarget(c, f.TargetID); err != nil {
		return err
	}

	updates := map[string]interface{}{}
	if req.Status != nil {
		s := strings.ToLower(strings.TrimSpace(*req.Status))
		switch s {
		case "open", "triaged", "duplicate", "fixed", "wontfix":
			updates["status"] = s
		default:
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid status"})
		}
	}
	if req.Notes != nil {
		updates["notes"] = *req.Notes
	}
	if req.ReportedTo != nil {
		updates["reported_to"] = *req.ReportedTo
	}

	if len(updates) == 0 {
		return c.JSON(fiber.Map{"status": "success", "message": "No changes"})
	}

	if err := database.DB.Model(&models.Finding{}).Where("id = ?", f.ID).Updates(updates).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update finding", "error": err.Error()})
	}

	_ = database.DB.First(&f, findingID).Error
	return c.JSON(fiber.Map{"status": "success", "data": toFindingResponse(f)})
}


