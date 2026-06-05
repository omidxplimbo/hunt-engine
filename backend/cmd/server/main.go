package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

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

func requestBodyLimitBytes() int {
	defaultMB := 5120 // 5GB transport ceiling; app/user quota still applies after auth.

	raw := os.Getenv("HUNT_MAX_REQUEST_BODY_MB")
	if raw == "" {
		return defaultMB * 1024 * 1024
	}

	mb, err := strconv.Atoi(raw)
	if err != nil || mb <= 0 {
		return defaultMB * 1024 * 1024
	}

	return mb * 1024 * 1024
}

func main() {
	database.Connect()
	database.CleanupZombieScans()

	// 👇👇👇 سید کردن کاربر ادمین اولیه
	seedAdminUser()

	redisq.Connect()

	// Check for queued items consistency
	// We need to fetch Redis items first.
	ctx := redisq.Ctx
	items, err := redisq.Client.LRange(ctx, redisq.QueueName, 0, -1).Result()
	if err == nil {
		database.CleanupZombieQueuedItems(items)
	} else {
		log.Println("⚠️ Failed to fetch queue for zombie check:", err)
	}

	telegram.Init()
	go scheduler.Start()
	go worker.Start()

	app := fiber.New(fiber.Config{
		AppName:   "Hunt Engine API v2.1.0 (Secured)",
		BodyLimit: requestBodyLimitBytes(),
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

	// --- System Configuration & Queue ---
	api.Get("/config", handlers.GetSystemConfig)
	api.Put("/config/:key", handlers.UpdateSystemConfig)

	api.Get("/queue", handlers.GetQueueList)
	api.Delete("/queue", handlers.ClearQueue)
	api.Delete("/queue/:index", handlers.RemoveFromQueue)
	api.Post("/queue/:index/move-top", handlers.MoveItemToTop)
	api.Post("/queue/:index/move-bottom", handlers.MoveItemToBottom)

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
	api.Post("/targets/:id/restart", handlers.RestartTargetScan)
	api.Post("/targets/:id/stop", handlers.StopScan)

	// Asset Routes
	api.Get("/targets/:id/assets", handlers.GetTargetAssets)
	api.Get("/targets/:id/assets/download", handlers.DownloadTargetAssets) // 👈 Download filtered assets
	api.Get("/targets/:id/urls", handlers.GetTargetURLs)
	api.Get("/targets/:id/urls/download", handlers.DownloadTargetURLs) // 👈 Download filtered URLs
	api.Get("/targets/:id/ips", handlers.ExportTargetIPs)              // Export IPs به صورت txt
	api.Get("/targets/:id/scan-state", handlers.GetTargetScanState)
	api.Get("/targets/:id/policy", handlers.GetTargetPolicy)
	api.Put("/targets/:id/policy", handlers.PutTargetPolicy)
	api.Delete("/targets/:id/policy", handlers.DeleteTargetPolicy)
	api.Get("/targets/:id/report.pdf", handlers.DownloadTargetPDFReport)
	// Agent Action Routes
	api.Get("/targets/:id/agent-actions", handlers.GetTargetAgentActions)
	api.Post("/targets/:id/agent-actions/propose", handlers.ProposeTargetAgentAction)
	api.Post("/targets/:id/agent-actions/:action_id/approve", handlers.ApproveTargetAgentAction)
	api.Post("/targets/:id/agent-actions/:action_id/reject", handlers.RejectTargetAgentAction)
	api.Post("/targets/:id/agent-actions/:action_id/dispatch", handlers.DispatchTargetAgentAction)
	api.Delete("/targets/:id/agent-actions/:action_id", handlers.DeleteTargetAgentAction)
	// Agent Chat Routes
	api.Get("/targets/:id/agent-chat/sessions", handlers.GetTargetAgentChatSessions)
	api.Post("/targets/:id/agent-chat/sessions", handlers.CreateTargetAgentChatSession)
	api.Get("/targets/:id/agent-chat/sessions/:session_id/messages", handlers.GetTargetAgentChatMessages)
	api.Post("/targets/:id/agent-chat/sessions/:session_id/messages", handlers.CreateTargetAgentChatMessage)
	api.Delete("/targets/:id/agent-chat/sessions/:session_id", handlers.DeleteTargetAgentChatSession)

	// 👇 Self-service (کاربر روی اکانت خودش)
	api.Get("/me", handlers.GetMe)
	api.Patch("/me", handlers.UpdateMe)
	api.Post("/me/change-password", handlers.ChangeMyPassword)
	api.Delete("/me", handlers.DeleteMe)
	api.Get("/me/telegram-config", handlers.GetMyTelegramConfig)
	api.Put("/me/telegram-config", handlers.PutMyTelegramConfig)
	api.Get("/me/llm-providers", handlers.GetMyLLMProviders)
	api.Put("/me/llm-providers", handlers.PutMyLLMProviders)
	api.Delete("/me/llm-providers/:provider", handlers.DeleteMyLLMProvider)
	api.Get("/me/feature-flags", handlers.GetMyFeatureFlags)
	api.Put("/me/feature-flags", handlers.PutMyFeatureFlags)
	api.Delete("/me/feature-flags/:key", handlers.DeleteMyFeatureFlag)
	// 👇 Subfinder API keys/config (per-user)
	api.Get("/me/subfinder/providers", handlers.GetMySubfinderProviders)
	api.Put("/me/subfinder/providers", handlers.PutMySubfinderProviders)
	api.Delete("/me/subfinder/providers/:provider", handlers.DeleteMySubfinderProvider)

	api.Get("/me/wordlists", handlers.GetMyWordlists)
	api.Post("/me/wordlists/upload", handlers.UploadMyWordlistFile)
	api.Post("/me/wordlists/upload-url", handlers.UploadMyWordlistURL)
	api.Get("/me/wordlists/:id/download", handlers.DownloadMyWordlist)
	api.Delete("/me/wordlists/:id", handlers.DeleteMyWordlist)
	// 👇👇👇 روت‌های مدیریت کاربران (فقط ادمین)
	adminUsers := api.Group("/users", middleware.AdminOnly())
	adminUsers.Get("/", handlers.GetUsers)
	adminUsers.Post("/", handlers.AddUser)
	adminUsers.Patch("/:id", handlers.UpdateUser)
	adminUsers.Delete("/:id", handlers.DeleteUser)

	// Finding Routes
	api.Get("/findings", handlers.GetFindings)
	api.Get("/targets/:id/findings/export", handlers.ExportTargetFindings)
	api.Get("/targets/:id/findings/stats", handlers.GetTargetFindingsStats)
	api.Get("/targets/:id/findings", handlers.GetTargetFindings)
	api.Patch("/findings/:id/status", handlers.UpdateFindingStatus)

	api.Get("/targets/:id/ai/analyses", handlers.GetTargetAIAnalyses)
	api.Post("/targets/:id/ai/analyses/generate", handlers.GenerateTargetAIAnalysis)
	api.Delete("/targets/:id/ai/analyses/:analysis_id", handlers.DeleteTargetAIAnalysis)
	api.Get("/targets/:id/ai/recommendations", handlers.GetTargetAIRecommendations)
	api.Post("/targets/:id/ai/recommendations/generate", handlers.GenerateTargetAIRecommendations)
	api.Delete("/targets/:id/ai/recommendations/:recommendation_id", handlers.DeleteTargetAIRecommendation)
	api.Get("/targets/:id/agents/runs", handlers.GetTargetAgentRuns)
	api.Post("/targets/:id/agents/triage/run", handlers.RunTargetTriageAgent)
	api.Post("/targets/:id/agents/summary/run", handlers.RunTargetSummaryAgent)
	api.Post("/targets/:id/agents/report/run", handlers.RunTargetReportAgent)
	api.Delete("/targets/:id/agents/runs/:run_id", handlers.DeleteTargetAgentRun)
	// Safe Bug Testing Routes
	api.Get("/targets/:id/bug-tests/runs", handlers.GetTargetBugTestRuns)
	api.Post("/targets/:id/bug-tests/runs", handlers.CreateTargetBugTestRun)
	api.Get("/targets/:id/bug-tests/results", handlers.GetTargetBugTestResults)

	api.Get("/targets/:id/audit-logs", handlers.GetTargetAuditLogs)

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

	// Nuclei custom template management (Admin Only)
	nucleiTemplates := api.Group("/nuclei/templates")
	nucleiTemplates.Get("/", handlers.ListNucleiTemplates)
	nucleiTemplates.Post("/", handlers.UpsertNucleiTemplate)
	nucleiTemplates.Post("/validate", handlers.ValidateNucleiTemplate)
	nucleiTemplates.Get("/:name", handlers.GetNucleiTemplate)
	nucleiTemplates.Delete("/:name", handlers.DeleteNucleiTemplate)

	// Nuclei AI template draft foundation (Admin Only, disabled by default)
	nucleiTemplateDrafts := api.Group("/nuclei/template-drafts", middleware.AdminOnly())
	nucleiTemplateDrafts.Get("/status", handlers.GetNucleiTemplateDraftStatus)
	nucleiTemplateDrafts.Post("/", handlers.GenerateNucleiTemplateDraft)
	nucleiTemplateDrafts.Get("/targets/:id/strategy", handlers.GetNucleiTargetTemplateStrategy)

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
