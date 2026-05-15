package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
	"gorm.io/gorm"
)

func normalizeRole(role string) (string, bool) {
	r := strings.ToLower(strings.TrimSpace(role))
	switch r {
	case "admin", "viewer":
		return r, true
	default:
		return "", false
	}
}

// GetUsers لیست تمام کاربران را برمی‌گرداند
func GetUsers(c *fiber.Ctx) error {
	var users []models.User
	if err := database.DB.Find(&users).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch users"})
	}

	var response []dto.UserResponse
	for _, u := range users {
		response = append(response, dto.UserResponse{
			ID:        u.ID,
			Username:  u.Username,
			Role:      u.Role,
			IsActive:  u.IsActive,
			CreatedAt: u.CreatedAt,
		})
	}

	return c.JSON(fiber.Map{"status": "success", "data": response})
}

// AddUser افزودن کاربر جدید توسط ادمین
func AddUser(c *fiber.Ctx) error {
	req := new(dto.CreateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	hashedPassword, err := utils.HashPassword(req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
	}
	user := models.User{
		Username: req.Username,
		Password: hashedPassword,
		Role:     req.Role,
		IsActive: true,
	}
	if user.Role == "" {
		user.Role = "viewer"
	}
	if role, ok := normalizeRole(user.Role); ok {
		user.Role = role
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid role (allowed: admin, viewer)"})
	}

	if err := database.DB.Create(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "User already exists or invalid data"})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "message": "User created"})
}

// UpdateUser ویرایش کاربر (رمز عبور یا نقش)
func UpdateUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	req := new(dto.UpdateUserRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	if req.Username != nil {
		user.Username = *req.Username
	}
	if req.Role != nil {
		newRole, ok := normalizeRole(*req.Role)
		if !ok {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid role (allowed: admin, viewer)"})
		}
		// جلوگیری از حذف آخرین ادمین (demote)
		if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" && newRole != "admin" {
			var otherAdmins int64
			database.DB.Model(&models.User{}).
				Where("role = ? AND id <> ?", "admin", user.ID).
				Count(&otherAdmins)
			if otherAdmins == 0 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot change role of the last admin user"})
			}
		}
		user.Role = newRole
	}
	if req.Password != nil && *req.Password != "" {
		hash, err := utils.HashPassword(*req.Password)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to hash password"})
		}
		user.Password = hash
	}
	if req.IsActive != nil {
		// جلوگیری از دی‌اکتیو کردن خود ادمین لاگین شده (برای جلوگیری از قفل شدن)
		currentUserID, err := currentUserID(c)
		if err != nil {
			return err
		}
		if currentUserID == user.ID && !*req.IsActive {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot deactivate yourself"})
		}
		user.IsActive = *req.IsActive
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to update user"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "User updated"})
}

func deleteTargetDeep(tx *gorm.DB, targetID uint) error {
	// حذف کامل دیتای مرتبط
	if err := tx.Exec("DELETE FROM asset_histories WHERE asset_id IN (SELECT id FROM assets WHERE target_id = ?)", targetID).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM assets WHERE target_id = ?", targetID).Error; err != nil {
		return err
	}
	if err := tx.Exec("DELETE FROM found_urls WHERE target_id = ?", targetID).Error; err != nil {
		return err
	}
	if err := tx.Unscoped().Delete(&models.Target{}, targetID).Error; err != nil {
		return err
	}
	return nil
}

// DeleteUser حذف کاربر
func DeleteUser(c *fiber.Ctx) error {
	id, _ := strconv.Atoi(c.Params("id"))

	// جلوگیری از حذف خود کاربر لاگین شده (اختیاری ولی توصیه میشه)
	currentUserID, err := currentUserID(c)
	if err != nil {
		return err
	}
	if currentUserID == uint(id) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Cannot delete yourself via /users. Use /me"})
	}

	// جلوگیری از حذف آخرین ادمین
	var user models.User
	if err := database.DB.First(&user, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		var otherAdmins int64
		database.DB.Model(&models.User{}).
			Where("role = ? AND id <> ?", "admin", user.ID).
			Count(&otherAdmins)
		if otherAdmins == 0 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot delete the last admin user"})
		}
	}

	// حذف کامل کاربر + تمام تارگت‌ها و دیتای مرتبط
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		// گرفتن تارگت‌های کاربر
		var targets []models.Target
		if err := tx.Select("id").Where("created_by_user_id = ?", uint(id)).Find(&targets).Error; err != nil {
			return err
		}
		for _, t := range targets {
			if err := deleteTargetDeep(tx, t.ID); err != nil {
				return err
			}
		}
		// حذف کامل خود کاربر
		if err := tx.Unscoped().Delete(&models.User{}, uint(id)).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete user"})
	}

	// پاک کردن کش‌های لیست تارگت‌ها
	cache.ClearCache()

	return c.JSON(fiber.Map{"status": "success", "message": "User deleted permanently"})
}
