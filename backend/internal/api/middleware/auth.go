package middleware

import (
	"strings"

	"github.com/gofiber/fiber/v2"
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

		return c.Next()
	}
}
