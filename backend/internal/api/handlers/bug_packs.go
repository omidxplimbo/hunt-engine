package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

func GetBugPatternPacks(c *fiber.Ctx) error {
	if err := ensureBugPatternRegistryEnabled(c); err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)
	db := database.DB.Model(&models.BugPatternPack{})

	if enabled := strings.ToLower(strings.TrimSpace(c.Query("enabled"))); enabled != "" {
		switch enabled {
		case "true", "1", "yes":
			db = db.Where("enabled = ?", true)
		case "false", "0", "no":
			db = db.Where("enabled = ?", false)
		}
	}

	if source := strings.ToLower(strings.TrimSpace(c.Query("source"))); source != "" {
		db = db.Where("source = ?", source)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.BugPatternPack, 0)
	if err := db.Order("source ASC, key ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "data": rows, "count": len(rows), "total_count": total, "page": page})
}

func GetBugPatternPack(c *fiber.Ctx) error {
	if err := ensureBugPatternRegistryEnabled(c); err != nil {
		return err
	}

	id, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug pattern pack id"})
	}

	var row models.BugPatternPack
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug pattern pack not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": row})
}

func GetBugPayloadPacks(c *fiber.Ctx) error {
	if err := ensureBugPayloadRegistryEnabled(c); err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)
	db := database.DB.Model(&models.BugPayloadPack{})

	if enabled := strings.ToLower(strings.TrimSpace(c.Query("enabled"))); enabled != "" {
		switch enabled {
		case "true", "1", "yes":
			db = db.Where("enabled = ?", true)
		case "false", "0", "no":
			db = db.Where("enabled = ?", false)
		}
	}

	if source := strings.ToLower(strings.TrimSpace(c.Query("source"))); source != "" {
		db = db.Where("source = ?", source)
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.BugPayloadPack, 0)
	if err := db.Order("source ASC, key ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{"status": "success", "data": rows, "count": len(rows), "total_count": total, "page": page})
}

func GetBugPayloadPack(c *fiber.Ctx) error {
	if err := ensureBugPayloadRegistryEnabled(c); err != nil {
		return err
	}

	id, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug payload pack id"})
	}

	var row models.BugPayloadPack
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug payload pack not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": row})
}

type updateBugPackEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func UpdateBugPatternPackEnabled(c *fiber.Ctx) error {
	if err := ensureBugPatternRegistryEnabled(c); err != nil {
		return err
	}

	actor, _, actorName, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}
	actorID := actor.ID

	id, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug pattern pack id"})
	}

	req := new(updateBugPackEnabledRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body"})
	}

	var row models.BugPatternPack
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug pattern pack not found"})
	}

	previous := row.Enabled
	row.Enabled = req.Enabled
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &actorID,
		ActorName:   actorName,
		Action:      "bug_pattern_pack.enabled_update",
		EntityType:  "bug_pattern_pack",
		EntityID:    &row.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"pack_key":         row.Key,
			"previous_enabled": previous,
			"enabled":          row.Enabled,
			"source":           row.Source,
			"version":          row.Version,
			"trust_level":      row.TrustLevel,
			"locked":           row.Locked,
			"v3_9_foundation":  true,
		},
	})

	return c.JSON(fiber.Map{"status": "success", "data": row})
}

func UpdateBugPayloadPackEnabled(c *fiber.Ctx) error {
	if err := ensureBugPayloadRegistryEnabled(c); err != nil {
		return err
	}

	actor, _, actorName, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}
	actorID := actor.ID

	id, err := strconv.ParseUint(strings.TrimSpace(c.Params("id")), 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug payload pack id"})
	}

	req := new(updateBugPackEnabledRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body"})
	}

	var row models.BugPayloadPack
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug payload pack not found"})
	}

	previous := row.Enabled
	row.Enabled = req.Enabled
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &actorID,
		ActorName:   actorName,
		Action:      "bug_payload_pack.enabled_update",
		EntityType:  "bug_payload_pack",
		EntityID:    &row.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"pack_key":          row.Key,
			"previous_enabled":  previous,
			"enabled":           row.Enabled,
			"source":            row.Source,
			"version":           row.Version,
			"trust_level":       row.TrustLevel,
			"locked":            row.Locked,
			"payload_execution": false,
			"v3_9_foundation":   true,
		},
	})

	return c.JSON(fiber.Map{"status": "success", "data": row})
}
