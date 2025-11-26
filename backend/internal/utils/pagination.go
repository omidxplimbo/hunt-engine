package utils

import (
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// DefaultLimit تعداد پیش‌فرض آیتم‌ها در هر صفحه
const DefaultLimit = 50

// MaxLimit حداکثر تعداد آیتم‌های مجاز در یک درخواست (برای جلوگیری از ابیوز)
const MaxLimit = 1000

// Paginate یک Scope برای GORM برمی‌گرداند که limit و offset را از کوئری استرینگ می‌خواند.
// مثال استفاده: ?limit=20&offset=40 (صفحه ۳ با سایز ۲۰)
func Paginate(c *fiber.Ctx) func(db *gorm.DB) *gorm.DB {
	return func(db *gorm.DB) *gorm.DB {
		// خواندن limit از کوئری پارامتر
		limitStr := c.Query("limit")
		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = DefaultLimit
		}
		if limit > MaxLimit {
			limit = MaxLimit
		}

		// خواندن offset از کوئری پارامتر
		offsetStr := c.Query("offset")
		offset, err := strconv.Atoi(offsetStr)
		if err != nil || offset < 0 {
			offset = 0
		}

		// اعمال limit و offset روی کوئری دیتابیس
		return db.Offset(offset).Limit(limit)
	}
}
