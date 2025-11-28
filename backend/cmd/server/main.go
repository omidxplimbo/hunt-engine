package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	// 👇👇👇 ایمپورت جدید
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/handlers"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/scheduler"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker"
)

func main() {
	database.Connect()
	redisq.Connect()
	telegram.Init()
	go scheduler.Start()
	go worker.Start()

	app := fiber.New(fiber.Config{
		AppName: "Hunt Engine API v0.3",
	})

	app.Use(logger.New())
	app.Use(recover.New())

	api := app.Group("/api")

	// روت تست سلامتی قبلی
	api.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "OK", "message": "Engine is running"})
	})

	// 👇👇👇 روت جدید برای ساخت تارگت
	// متد POST روی آدرس /api/targets
	api.Post("/targets", handlers.CreateTarget)
	api.Get("/targets", handlers.GetTargets)

	// --- Asset Routes ---
	// 👇👇👇 روت جدید: گرفتن دارایی‌های یک تارگت خاص (با قابلیت فیلتر)
	api.Get("/targets/:id/assets", handlers.GetTargetAssets)
	api.Post("/targets/:id/probe", handlers.StartProbing)
	api.Post("/targets/:id/discovery", handlers.StartDiscovery) // شروع فاز ۱
	api.Patch("/targets/:id", handlers.UpdateTarget)            // ویرایش
	api.Delete("/targets/:id", handlers.DeleteTarget)           //

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🔥 Server is starting on port %s...\n", port)
	log.Fatal(app.Listen(":" + port))
}
