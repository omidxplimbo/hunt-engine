package handlers

import (
	"strconv"
	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

// GetTwoAccountSessions لیست سشن‌های تست دو حسابی برای یک تارگت
func GetTwoAccountSessions(c *fiber.Ctx) error {
	targetIDStr := c.Params("target_id")
	targetID, err := strconv.Atoi(targetIDStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid target ID"})
	}

	userID, err := currentUserID(c)
	if err != nil {
		return c.Status(401).JSON(fiber.Map{"error": "Unauthorized"})
	}

	var sessions []models.TwoAccountSession
	if err := database.DB.Where("target_id = ? AND owner_id = ?", uint(targetID), userID).
		Order("created_at DESC").
		Find(&sessions).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{"data": sessions, "status": "success"})
}

// StartTwoAccountSession شروع تست هوشمند
func StartTwoAccountSession(c *fiber.Ctx) error {
	return c.Status(202).JSON(fiber.Map{
		"status": "accepted",
		"message": "Smart IDOR test session initiated.",
	})
}
