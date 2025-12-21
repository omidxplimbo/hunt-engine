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

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}
	admin := isAdmin(c)

	// 1. آمار کلی (Cards)
	targetQ := database.DB.Model(&models.Target{})
	assetQ := database.DB.Model(&models.Asset{})
	if !admin {
		targetQ = targetQ.Where("created_by_user_id = ?", uid)
		assetQ = assetQ.Joins("JOIN targets ON targets.id = assets.target_id").Where("targets.created_by_user_id = ?", uid)
	}

	targetQ.Count(&stats.TotalTargets)
	assetQ.Count(&stats.TotalAssets)
	assetQ.Where("is_live = ?", true).Count(&stats.LiveAssets)

	// دارایی‌های جدید در ۲۴ ساعت گذشته
	yesterday := time.Now().Add(-24 * time.Hour)
	assetQ.Where("created_at > ?", yesterday).Count(&stats.FreshAssets)

	// 2. آمار وضعیت‌ها (برای Pie Chart) - فقط زنده‌ها
	// گروپ بای روی status_code
	byStatusQ := database.DB.Model(&models.Asset{})
	if !admin {
		byStatusQ = byStatusQ.Joins("JOIN targets ON targets.id = assets.target_id").Where("targets.created_by_user_id = ?", uid)
	}
	byStatusQ.
		Select("CAST(status_code AS VARCHAR) as name, count(*) as count").
		Where("is_live = ? AND status_code > 0", true).
		Group("status_code").
		Order("count desc").
		Scan(&stats.AssetsByStatus)

	// 3. تکنولوژی‌های برتر (برای Bar Chart)
	// استفاده از تابع jsonb_array_elements_text برای باز کردن آرایه تکنولوژی‌ها
	if admin {
	database.DB.Raw(`
		SELECT t.name, count(*) as count 
		FROM assets, jsonb_array_elements_text(technologies) as t(name) 
		WHERE is_live = true 
		GROUP BY t.name 
		ORDER BY count DESC 
		LIMIT 10
	`).Scan(&stats.TopTechnologies)
	} else {
		database.DB.Raw(`
			SELECT t.name, count(*) as count
			FROM assets
			JOIN targets ON targets.id = assets.target_id
			, jsonb_array_elements_text(technologies) as t(name)
			WHERE is_live = true AND targets.created_by_user_id = ?
			GROUP BY t.name
			ORDER BY count DESC
			LIMIT 10
		`, uid).Scan(&stats.TopTechnologies)
	}

	// 4. پورت‌های باز برتر (از open_ports) - شمارش بر اساس تعداد assetهای منحصر به فرد
	// open_ports ساختار: {"ip":[80,443]} => باید آرایه‌ها را باز کنیم و روی port group کنیم
	if admin {
	database.DB.Raw(`
		SELECT p.port as name, count(DISTINCT assets.id) as count
		FROM assets
		CROSS JOIN LATERAL jsonb_each(open_ports) e(ip, ports)
		CROSS JOIN LATERAL jsonb_array_elements_text(e.ports) p(port)
		WHERE open_ports IS NOT NULL AND open_ports <> '{}'::jsonb
		GROUP BY p.port
		ORDER BY count DESC
		LIMIT 10
	`).Scan(&stats.TopOpenPorts)
	} else {
		database.DB.Raw(`
			SELECT p.port as name, count(DISTINCT assets.id) as count
			FROM assets
			JOIN targets ON targets.id = assets.target_id
			CROSS JOIN LATERAL jsonb_each(open_ports) e(ip, ports)
			CROSS JOIN LATERAL jsonb_array_elements_text(e.ports) p(port)
			WHERE open_ports IS NOT NULL AND open_ports <> '{}'::jsonb
			  AND targets.created_by_user_id = ?
			GROUP BY p.port
			ORDER BY count DESC
			LIMIT 10
		`, uid).Scan(&stats.TopOpenPorts)
	}

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   stats,
	})
}
