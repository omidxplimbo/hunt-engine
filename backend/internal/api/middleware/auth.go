package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
)

// Protected میدل‌ویری که چک می‌کند کاربر توکن معتبر دارد یا نه
func Protected() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// گرفتن هدر Authorization
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Missing authorization token"})
		}

		// فرمت باید "Bearer TOKEN" باشد
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid token format"})
		}

		tokenString := parts[1]

		// اعتبارسنجی توکن
		claims, err := utils.ValidateToken(tokenString)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		// همیشه اطلاعات کاربر را از DB می‌خوانیم تا:
		// - اگر user دی‌اکتیو شد حتی با توکن قبلی هم دسترسی نداشته باشد
		// - اگر role تغییر کرد سریع اعمال شود
		var uid uint
		switch v := claims["user_id"].(type) {
		case float64:
			uid = uint(v)
		case int:
			uid = uint(v)
		case uint:
			uid = v
		}
		if uid == 0 {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
		}

		var user models.User
		if err := database.DB.Select("id", "username", "role", "is_active").First(&user, uid).Error; err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid or expired token"})
		}
		if !user.IsActive {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Account is deactivated. Please contact an administrator.",
			})
		}

		c.Locals("user_id", user.ID)
		c.Locals("username", user.Username)
		c.Locals("role", user.Role)

		return c.Next()
	}
}

// AdminOnly فقط به کاربران با نقش admin اجازه عبور می‌دهد
func AdminOnly() fiber.Handler {
	return func(c *fiber.Ctx) error {
		role, _ := c.Locals("role").(string)
		if strings.ToLower(strings.TrimSpace(role)) != "admin" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
		}
		return c.Next()
	}
}
