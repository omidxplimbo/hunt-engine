package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
)

// GetMe اطلاعات کاربر فعلی را برمی‌گرداند
func GetMe(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data": dto.MeResponse{
			ID:        user.ID,
			Username:  user.Username,
			Role:      user.Role,
			CreatedAt: user.CreatedAt.Format(time.RFC3339),
		},
	})
}

// UpdateMe ویرایش پروفایل کاربر فعلی (فعلاً فقط username)
func UpdateMe(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.UpdateMeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if req.Username != nil {
		u := strings.TrimSpace(*req.Username)
		if u == "" {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Username cannot be empty"})
		}
		user.Username = u
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to update profile"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Profile updated"})
}

// ChangeMyPassword تغییر رمز عبور کاربر فعلی (با تایید رمز فعلی)
func ChangeMyPassword(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.ChangePasswordRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.CurrentPassword == "" || req.NewPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_password and new_password are required"})
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if !utils.CheckPasswordHash(req.CurrentPassword, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Current password is incorrect"})
	}

	hash, _ := utils.HashPassword(req.NewPassword)
	user.Password = hash
	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to change password"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Password changed"})
}

// DeleteMe حذف اکانت کاربر فعلی (با تایید رمز فعلی)
func DeleteMe(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.DeleteMeRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	req.CurrentPassword = strings.TrimSpace(req.CurrentPassword)
	if req.CurrentPassword == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "current_password is required"})
	}

	var user models.User
	if err := database.DB.First(&user, uid).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	if !utils.CheckPasswordHash(req.CurrentPassword, user.Password) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Current password is incorrect"})
	}

	// جلوگیری از حذف آخرین ادمین
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		var otherAdmins int64
		database.DB.Model(&models.User{}).
			Where("role = ? AND id <> ?", "admin", user.ID).
			Count(&otherAdmins)
		if otherAdmins == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot delete the last admin user"})
		}
	}

	if err := database.DB.Delete(&models.User{}, uid).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete account"})
	}

	return c.JSON(fiber.Map{"status": "success", "message": "Account deleted"})
}



