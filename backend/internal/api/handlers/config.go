package handlers

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

// GetSystemConfig retrieves all system configurations
func GetSystemConfig(c *fiber.Ctx) error {
	var configs []models.SystemConfig
	if err := database.DB.Find(&configs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch configs"})
	}
	return c.JSON(configs)
}

// UpdateSystemConfig updates a specific configuration key
func UpdateSystemConfig(c *fiber.Ctx) error {
	type UpdateReq struct {
		Value string `json:"value"`
	}
	key := c.Params("key")
	var req UpdateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	config := models.SystemConfig{Key: key, Value: req.Value}

	// Validation for max_concurrent_scans
	if key == "max_concurrent_scans" {
		newLimit, err := strconv.Atoi(req.Value)
		if err != nil || newLimit <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid value for max_concurrent_scans"})
		}

		var activeCount int64
		// Check active scans from TargetScanState (more accurate for running processes)
		if err := database.DB.Model(&models.TargetScanState{}).Where("status = ?", "RUNNING").Count(&activeCount).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to validate active scans"})
		}

		if activeCount > int64(newLimit) {
			return c.Status(400).JSON(fiber.Map{
				"error": "You have active processes, you cannot decrease the concurrent scan limit.",
			})
		}
	}

	if err := database.DB.Save(&config).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save config"})
	}

	return c.JSON(config)
}
