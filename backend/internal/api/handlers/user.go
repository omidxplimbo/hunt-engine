package handlers

import (
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
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

	hashedPassword, _ := utils.HashPassword(req.Password)
	user := models.User{
		Username: req.Username,
		Password: hashedPassword,
		Role:     req.Role,
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
		hash, _ := utils.HashPassword(*req.Password)
		user.Password = hash
	}

	if err := database.DB.Save(&user).Error; err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Failed to update user"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "User updated"})
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

	if err := database.DB.Delete(&models.User{}, id).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to delete user"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "User deleted"})
}
