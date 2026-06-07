package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
)

func ensureBugPatternRegistryEnabled(c *fiber.Ctx) error {
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}
	if !featureflags.IsEnabledForOwner(ownerKey, featureflags.KeyBugPatternRegistry) {
		return featureflags.DisabledResponse(c, featureflags.KeyBugPatternRegistry)
	}
	return nil
}

func GetBugPatterns(c *fiber.Ctx) error {
	if err := ensureBugPatternRegistryEnabled(c); err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)

	db := database.DB.Model(&models.BugPattern{})

	if bugType := normalizeBugType(c.Query("bug_type")); bugType != models.BugTypeUnknown {
		db = db.Where("bug_type = ?", bugType)
	}

	if mode := strings.ToLower(strings.TrimSpace(c.Query("mode"))); mode != "" {
		db = db.Where("mode = ?", mode)
	}

	if enabled := strings.ToLower(strings.TrimSpace(c.Query("enabled"))); enabled != "" {
		switch enabled {
		case "true", "1", "yes":
			db = db.Where("enabled = ?", true)
		case "false", "0", "no":
			db = db.Where("enabled = ?", false)
		}
	}

	var total int64
	if err := db.Count(&total).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	rows := make([]models.BugPattern, 0)
	if err := db.Order("bug_type ASC, test_level ASC, key ASC").Limit(limit).Offset(offset).Find(&rows).Error; err != nil {
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

func GetBugPattern(c *fiber.Ctx) error {
	if err := ensureBugPatternRegistryEnabled(c); err != nil {
		return err
	}

	raw := strings.TrimSpace(c.Params("id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug pattern id"})
	}

	var row models.BugPattern
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug pattern not found"})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   row,
	})
}

type updateBugPatternEnabledRequest struct {
	Enabled bool `json:"enabled"`
}

func UpdateBugPatternEnabled(c *fiber.Ctx) error {
	if err := ensureBugPatternRegistryEnabled(c); err != nil {
		return err
	}

	raw := strings.TrimSpace(c.Params("id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug pattern id"})
	}

	req := new(updateBugPatternEnabledRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid request body"})
	}

	var row models.BugPattern
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug pattern not found"})
	}

	row.Enabled = req.Enabled
	if err := database.DB.Save(&row).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   row,
	})
}
