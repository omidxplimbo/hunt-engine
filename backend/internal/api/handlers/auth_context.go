package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/gorm"
)

func currentUserID(c *fiber.Ctx) (uint, error) {
	val := c.Locals("user_id")
	switch v := val.(type) {
	case float64:
		if v <= 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return uint(v), nil
	case int:
		if v <= 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return uint(v), nil
	case uint:
		if v == 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return v, nil
	case string:
		n, err := strconv.ParseUint(v, 10, 64)
		if err != nil || n == 0 {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
		}
		return uint(n), nil
	default:
		return 0, fiber.NewError(fiber.StatusUnauthorized, "Invalid user context")
	}
}

func currentUserRole(c *fiber.Ctx) string {
	role, _ := c.Locals("role").(string)
	return strings.ToLower(strings.TrimSpace(role))
}

func isAdmin(c *fiber.Ctx) bool {
	return currentUserRole(c) == "admin"
}

func scopedTargetsDB(c *fiber.Ctx) (*gorm.DB, uint, error) {
	uid, err := currentUserID(c)
	if err != nil {
		return nil, 0, err
	}
	db := database.DB.Model(&models.Target{})
	if !isAdmin(c) {
		db = db.Where("created_by_user_id = ?", uid)
	}
	return db, uid, nil
}

func getAccessibleTarget(c *fiber.Ctx, targetID uint) (*models.Target, error) {
	db, _, err := scopedTargetsDB(c)
	if err != nil {
		return nil, err
	}
	var t models.Target
	if err := db.First(&t, targetID).Error; err != nil {
		// 404 هم برای not-found و هم برای not-authorized (برای جلوگیری از enumeration)
		return nil, fiber.NewError(fiber.StatusNotFound, "Target not found")
	}
	return &t, nil
}
