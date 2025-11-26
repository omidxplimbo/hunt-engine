package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
)

func main() {
	// 1. ساخت یک نمونه جدید از برنامه Fiber
	// تنظیمات مربوط به پرفورمنس اینجا قرار می‌گیرد (بعداً تکمیل می‌کنیم)
	app := fiber.New(fiber.Config{
		AppName: "Hunt Engine API v0.1",
	})

	// 2. اضافه کردن Middlewareهای حیاتی
	app.Use(logger.New())  // برای لاگ کردن درخواست‌ها در کنسول
	app.Use(recover.New()) // جلوگیری از کرش کردن برنامه در صورت بروز پنیک

	// 3. تعریف روت‌ها (Routes)
	// یک روت ساده برای تست سلامتی سیستم (Health Check)
	api := app.Group("/api") // تمام APIها با پیشوند /api شروع می‌شوند
	api.Get("/", func(c *fiber.Ctx) error {
		// ارسال پاسخ به صورت JSON
		return c.JSON(fiber.Map{
			"status":  "success",
			"message": "🚀 Hunt Engine Backend is Running Correctly!",
			"stack":   "Golang + Fiber v2",
		})
	})

	// 4. تنظیم پورت و شروع سرور
	// پورت را از متغیر محیطی می‌خوانیم (برای سازگاری با داکر)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // پورت پیشفرض
	}

	fmt.Printf("🔥 Server is starting on port %s...\n", port)
	// شروع گوش دادن به درخواست‌ها
	log.Fatal(app.Listen(":" + port))
}