package handlers

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

// GetDashboardStats آمار کلی سیستم را برای داشبورد برمی‌گرداند
func GetDashboardStats(c *fiber.Ctx) error {
	stats := dto.DashboardStats{}

	// 1. آمار کلی (Cards)
	database.DB.Model(&models.Target{}).Count(&stats.TotalTargets)
	database.DB.Model(&models.Asset{}).Count(&stats.TotalAssets)
	database.DB.Model(&models.Asset{}).Where("is_live = ?", true).Count(&stats.LiveAssets)

	// دارایی‌های جدید در ۲۴ ساعت گذشته
	yesterday := time.Now().Add(-24 * time.Hour)
	database.DB.Model(&models.Asset{}).Where("created_at > ?", yesterday).Count(&stats.FreshAssets)

	// 2. آمار وضعیت‌ها (برای Pie Chart) - فقط زنده‌ها
	// گروپ بای روی status_code
	database.DB.Model(&models.Asset{}).
		Select("CAST(status_code AS VARCHAR) as name, count(*) as count").
		Where("is_live = ? AND status_code > 0", true).
		Group("status_code").
		Order("count desc").
		Scan(&stats.AssetsByStatus)

	// 3. تکنولوژی‌های برتر (برای Bar Chart)
	// استفاده از تابع jsonb_array_elements_text برای باز کردن آرایه تکنولوژی‌ها
	database.DB.Raw(`
		SELECT t.name, count(*) as count 
		FROM assets, jsonb_array_elements_text(technologies) as t(name) 
		WHERE is_live = true 
		GROUP BY t.name 
		ORDER BY count DESC 
		LIMIT 10
	`).Scan(&stats.TopTechnologies)

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   stats,
	})
}
