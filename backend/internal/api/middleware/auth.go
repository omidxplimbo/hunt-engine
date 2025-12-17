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

		// ذخیره اطلاعات کاربر در کانتکست برای استفاده در هندلرها
		c.Locals("user_id", claims["user_id"])
		c.Locals("username", claims["username"])
		// role ممکن است در توکن‌های قدیمی وجود نداشته باشد، پس fallback می‌زنیم
		if roleVal, ok := claims["role"]; ok {
			if roleStr, ok2 := roleVal.(string); ok2 {
				c.Locals("role", roleStr)
			}
		}
		if c.Locals("role") == nil {
			// Fallback: role را از دیتابیس می‌خوانیم (برای سازگاری با توکن‌های قدیمی)
			var uid uint
			switch v := claims["user_id"].(type) {
			case float64:
				uid = uint(v)
			case int:
				uid = uint(v)
			case uint:
				uid = v
			}
			if uid > 0 {
				var user models.User
				if err := database.DB.Select("id", "role").First(&user, uid).Error; err == nil {
					c.Locals("role", user.Role)
				}
			}
		}

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
