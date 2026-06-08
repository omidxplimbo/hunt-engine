package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
)

func ensureBugPayloadRegistryEnabled(c *fiber.Ctx) error {
	_, ownerKey, _, _, err := currentAccountOwner(c)
	if err != nil {
		return err
	}

	// v3.8.3 payload registry uses the same feature flag as pattern registry.
	if !featureflags.IsEnabledForOwner(ownerKey, featureflags.KeyBugPatternRegistry) {
		return featureflags.DisabledResponse(c, featureflags.KeyBugPatternRegistry)
	}

	return nil
}

func GetBugPayloads(c *fiber.Ctx) error {
	if err := ensureBugPayloadRegistryEnabled(c); err != nil {
		return err
	}

	limit, offset, page := parseDataLayerPagination(c)

	db := database.DB.Model(&models.BugPayload{})

	if bugType := normalizeBugType(c.Query("bug_type")); bugType != models.BugTypeUnknown {
		db = db.Where("bug_type = ?", bugType)
	}

	if safetyClass := strings.ToLower(strings.TrimSpace(c.Query("safety_class"))); safetyClass != "" {
		db = db.Where("safety_class = ?", safetyClass)
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

	rows := make([]models.BugPayload, 0)
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

func GetBugPayload(c *fiber.Ctx) error {
	if err := ensureBugPayloadRegistryEnabled(c); err != nil {
		return err
	}

	raw := strings.TrimSpace(c.Params("id"))
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || id == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid bug payload id"})
	}

	var row models.BugPayload
	if err := database.DB.First(&row, uint(id)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "bug payload not found"})
	}

	return c.JSON(fiber.Map{"status": "success", "data": row})
}
