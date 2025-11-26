package handlers

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

// CreateTarget هندلر ساخت یک هدف جدید است
func CreateTarget(c *fiber.Ctx) error {
	// 1. Parse Body: داده‌های جیسون رو از درخواست می‌خونیم و می‌ریزیم توی DTO
	req := new(dto.CreateTargetRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body format",
			"error":   err.Error(),
		})
	}

	// TODO: اینجا باید اعتبارسنجی (Validation) اضافه بشه که فعلا رد می‌شیم

	// 2. Map to Model: داده‌های DTO رو تبدیل می‌کنیم به مدل دیتابیس
	target := models.Target{
		Name:        req.Name,
		RootDomain:  req.RootDomain,
		Description: req.Description,
		InScope:     true, // پیشفرض فعال است
	}

	// 3. Save to DB
	result := database.DB.Create(&target)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Could not save target to database",
			"error":   result.Error.Error(),
		})
	}

	// 👇👇👇 بخش جدید: ارسال کار به صف Redis 👇👇👇

	// یک پیام ساده می‌سازیم: "ID:Domain"
	// مثال: "1:example.com"
	taskPayload := fmt.Sprintf("%d:%s", target.ID, target.RootDomain)

	// با دستور RPUSH پیام رو به انتهای صف اضافه می‌کنیم
	err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err()
	if err != nil {
		// نکته مهم: اگر اینجا خطا بده، تارگت ذخیره شده ولی ریکان شروع نمیشه.
		// در سیستم‌های واقعی اینجا باید لاگ دقیق بزنیم.
		fmt.Printf("⚠️ Failed to enqueue discovery task for %s: %v\n", target.RootDomain, err)
	} else {
		fmt.Printf("Have enqueued discovery task for: %s [Payload: %s]\n", target.RootDomain, taskPayload)
	}

	// 👆👆👆 پایان بخش جدید 👆👆👆

	// 4. Return Response
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  "success",
		"message": "Target created successfully AND discovery queued!", // پیام رو آپدیت کردیم
		"data":    target,
	})
}
