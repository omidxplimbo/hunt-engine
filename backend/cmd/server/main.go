package main

import (
	"fmt"
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"

	"github.com/omidxplimbo/hunt-engine/backend/internal/api/handlers"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/middleware" // 👈 ایمپورت جدید
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/scheduler"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker"

	"github.com/gofiber/contrib/websocket"
)

func main() {
	database.Connect()
	database.CleanupZombieScans()

	// 👇👇👇 سید کردن کاربر ادمین اولیه
	seedAdminUser()

	redisq.Connect()
	telegram.Init()
	go scheduler.Start()
	go worker.Start()

	app := fiber.New(fiber.Config{
		AppName: "Hunt Engine API v2.0 (Secured)",
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "*",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization", // Authorization اضافه شد
		AllowMethods: "GET, POST, HEAD, PUT, DELETE, PATCH",
	}))
	app.Use(logger.New())
	app.Use(recover.New())

	api := app.Group("/api")

	// --- Public Routes (عمومی) ---
	api.Get("/", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "OK", "message": "Engine is running"})
	})
	api.Post("/auth/login", handlers.Login) // 👈 روت لاگین

	// --- Protected Routes (محافظت شده) ---
	// همه روت‌های زیر نیاز به توکن دارند
	api.Use(middleware.Protected())

	// Target Routes
	// ⚠️ مهم: route‌های static باید قبل از route‌های dynamic قرار بگیرند
	api.Post("/targets", handlers.CreateTarget)
	api.Get("/targets", handlers.GetTargets)

	// Export/Import Routes (باید قبل از /targets/:id قرار بگیرند)
	api.Get("/targets/export", handlers.ExportTarget)  // Export تارگت‌ها (با query param ?id= برای یک تارگت خاص)
	api.Post("/targets/export", handlers.ExportTarget) // Export تارگت‌ها (با body شامل لیست IDها)
	api.Post("/targets/import", handlers.ImportTarget) // Import تارگت‌ها

	// Dynamic Routes (باید بعد از static routes قرار بگیرند)
	api.Get("/targets/:id", handlers.GetTargetDetails)
	api.Patch("/targets/:id", handlers.UpdateTarget)
	api.Delete("/targets/:id", handlers.DeleteTarget)

	// Operations
	api.Post("/targets/:id/probe", handlers.StartProbing)
	api.Post("/targets/:id/scan", handlers.StartDiscovery) // 👈 Restore scan endpoint
	api.Post("/targets/:id/discovery", handlers.StartDiscovery)
	api.Post("/targets/:id/resume", handlers.ResumeTargetScan)
	api.Post("/targets/:id/stop", handlers.StopScan)

	// Asset Routes
	api.Get("/targets/:id/assets", handlers.GetTargetAssets)
	api.Get("/targets/:id/assets/download", handlers.DownloadTargetAssets) // 👈 Download filtered assets
	api.Get("/targets/:id/urls", handlers.GetTargetURLs)
	api.Get("/targets/:id/urls/download", handlers.DownloadTargetURLs) // 👈 Download filtered URLs
	api.Get("/targets/:id/ips", handlers.ExportTargetIPs) // Export IPs به صورت txt
	api.Get("/targets/:id/scan-state", handlers.GetTargetScanState)

	// 👇 Self-service (کاربر روی اکانت خودش)
	api.Get("/me", handlers.GetMe)
	api.Patch("/me", handlers.UpdateMe)
	api.Post("/me/change-password", handlers.ChangeMyPassword)
	api.Delete("/me", handlers.DeleteMe)
	// 👇 Subfinder API keys/config (per-user)
	api.Get("/me/subfinder/providers", handlers.GetMySubfinderProviders)
	api.Put("/me/subfinder/providers", handlers.PutMySubfinderProviders)
	api.Delete("/me/subfinder/providers/:provider", handlers.DeleteMySubfinderProvider)

	// 👇👇👇 روت‌های مدیریت کاربران (فقط ادمین)
	adminUsers := api.Group("/users", middleware.AdminOnly())
	adminUsers.Get("/", handlers.GetUsers)
	adminUsers.Post("/", handlers.AddUser)
	adminUsers.Patch("/:id", handlers.UpdateUser)
	adminUsers.Delete("/:id", handlers.DeleteUser)

	api.Get("/dashboard/stats", handlers.GetDashboardStats)
	api.Get("/monitor/stats", handlers.GetMonitorData)

	// WebSocket for System Logs (Admin Only)
	api.Use("/monitor/logs", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return c.Status(fiber.StatusUpgradeRequired).SendString("Upgrade Required")
	})
	api.Get("/monitor/logs", websocket.New(handlers.StreamSystemLogs))

	// Wordlists endpoint
	api.Get("/wordlists", handlers.GetWordlists)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("🔥 Server is starting on port %s...\n", port)
	log.Fatal(app.Listen(":" + port))
}

// seedAdminUser یک کاربر ادمین پیش‌فرض می‌سازد اگر وجود نداشته باشد
func seedAdminUser() {
	var count int64
	database.DB.Model(&models.User{}).Count(&count)
	if count == 0 {
		log.Println("🌱 Seeding default admin user...")
		// رمز عبور پیش‌فرض: admin123
		// در پروداکشن حتماً این را تغییر دهید!
		hash, _ := utils.HashPassword("admin123")
		admin := models.User{
			Username: "admin",
			Password: hash,
			Role:     "admin",
		}
		if err := database.DB.Create(&admin).Error; err != nil {
			log.Printf("❌ Failed to seed admin user: %v\n", err)
		} else {
			log.Println("✅ Default admin created: username='admin', password='admin123'")
		}
	}
}
