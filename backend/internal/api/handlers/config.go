package handlers

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	configsvc "github.com/omidxplimbo/hunt-engine/backend/internal/platform/config"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
)

// GetSystemConfig retrieves all system configurations.
// Feature flag defaults are included even when they have not been persisted yet.
func GetSystemConfig(c *fiber.Ctx) error {
	var configs []models.SystemConfig
	if err := database.DB.Find(&configs).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch configs"})
	}

	existing := make(map[string]struct{}, len(configs))
	for _, cfg := range configs {
		existing[cfg.Key] = struct{}{}
	}

	for _, def := range featureflags.All() {
		if _, ok := existing[def.Key]; ok {
			continue
		}

		configs = append(configs, models.SystemConfig{
			Key:       def.Key,
			Value:     featureflags.BoolString(def.Default),
			UpdatedAt: time.Now().UTC(),
		})
	}

	return c.JSON(configs)
}

// UpdateSystemConfig updates a specific configuration key.
func UpdateSystemConfig(c *fiber.Ctx) error {
	type UpdateReq struct {
		Value string `json:"value"`
	}

	key := c.Params("key")

	var req UpdateReq
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request"})
	}

	systemConfig := models.SystemConfig{Key: key, Value: req.Value}

	if key == configsvc.KeyMaxConcurrentScans {
		newLimit, err := strconv.Atoi(req.Value)
		if err != nil || newLimit <= 0 {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid value for max_concurrent_scans"})
		}

		var activeCount int64
		if err := database.DB.Model(&models.TargetScanState{}).Where("status = ?", "RUNNING").Count(&activeCount).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to validate active scans"})
		}

		if activeCount > int64(newLimit) {
			return c.Status(400).JSON(fiber.Map{
				"error": "You have active processes, you cannot decrease the concurrent scan limit.",
			})
		}
	}

	if key == configsvc.KeyTelegramNotificationsEnabled ||
		key == configsvc.KeyTelegramFreshAssetScreenshot ||
		featureflags.IsKnownKey(key) {
		if req.Value != "true" && req.Value != "false" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid boolean config value"})
		}
	}

	if key == configsvc.KeyTelegramNotificationEvents {
		var events []string
		if err := json.Unmarshal([]byte(req.Value), &events); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid telegram notification events JSON"})
		}

		allowed := map[string]struct{}{}
		for _, event := range configsvc.DefaultTelegramNotificationEvents {
			allowed[event] = struct{}{}
		}

		for _, event := range events {
			if _, ok := allowed[event]; !ok {
				return c.Status(400).JSON(fiber.Map{"error": "Unknown telegram notification event: " + event})
			}
		}
	}

	if err := database.DB.Save(&systemConfig).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save config"})
	}

	if featureflags.IsKnownKey(key) {
		featureflags.Invalidate(key)
	}

	return c.JSON(systemConfig)
}
