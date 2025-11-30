package handlers

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils" // 👈 ایمپورت جدید
	"gorm.io/gorm"
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

	modulesJSON := "[]"
	if len(req.Modules) > 0 {
		// اینجا خیلی ساده تبدیل می‌کنیم، در کد واقعی بهتره از json.Marshal استفاده بشه
		// فرض می‌کنیم کاربر درست می‌فرسته. برای سادگی فعلا از کتابخانه استفاده می‌کنیم:
		bytes, _ := json.Marshal(req.Modules)
		modulesJSON = string(bytes)
	} else {
		// پیش‌فرض
		modulesJSON = "[\"DISCOVERY\", \"PROBING\"]"
	}

	target := models.Target{
		Name:        req.Name,
		RootDomain:  req.RootDomain,
		Description: req.Description,
		InScope:     true,
		Frequency:   req.Frequency, // 👈 ذخیره فرکانس
		ScanModules: modulesJSON,   // 👈 ذخیره ماژول‌ها
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
	taskPayload := fmt.Sprintf("DISCOVERY:%d:%s", target.ID, target.RootDomain)

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

// ==========================================
// New Handlers for Data Retrieval
// ==========================================

// GetTargets لیست تمام تارگت‌ها رو به صورت صفحه‌بندی شده برمی‌گردونه (با کش ۵ ثانیه)
func GetTargets(c *fiber.Ctx) error {
	// پارامترهای صفحه‌بندی
	page, _ := strconv.Atoi(c.Query("page", "1"))
	limit := 50 // پیش‌فرض
	offset := (page - 1) * limit

	// 1. ساخت کلید کش برای لیست تارگت‌ها
	// نکته: ما اینجا ورژن نمی‌ذاریم، بلکه به TTL کوتاه اکتفا می‌کنیم چون وضعیت مدام تغییر می‌کنه
	cacheKey := fmt.Sprintf("targets:list:p:%d:l:%d", page, limit)

	// 2. چک کردن کش
	var cachedResponse fiber.Map
	if cache.GetCache(cacheKey, &cachedResponse) {
		c.Set("X-Cache", "HIT")
		return c.JSON(cachedResponse)
	}

	// 3. اگر در کش نبود -> اجرای کوئری سنگین دیتابیس
	var targets []models.Target
	// دریافت لیست (بدون شمارش است‌ها)
	result := database.DB.Order("created_at desc").Limit(limit).Offset(offset).Find(&targets)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": result.Error.Error()})
	}

	// تبدیل به DTO و شمارش است‌ها (اینجاست که سنگینه!)
	targetResponses := make([]dto.TargetResponse, len(targets))
	for i, t := range targets {
		var count int64
		database.DB.Model(&models.Asset{}).Where("target_id = ?", t.ID).Count(&count) // 👈 کوئری سنگین
		targetResponses[i] = toTargetResponse(t, count)
	}

	// محاسبه تعداد کل تارگت‌ها (برای صفحه‌بندی)
	var totalTargets int64
	database.DB.Model(&models.Target{}).Count(&totalTargets)

	response := fiber.Map{
		"status": "success",
		"data":   targetResponses,
		"count":  len(targetResponses),
		"total":  totalTargets,
	}

	// 4. ذخیره در کش برای ۵ ثانیه ⏳
	// این یعنی حداکثر ۱۲ بار در دقیقه به دیتابیس فشار میاد، نه ۳۰ بار (اگر هر ۲ ثانیه رفرش بشه)
	cache.SetCacheWithTTL(cacheKey, response, 5*time.Second)

	c.Set("X-Cache", "MISS")
	return c.JSON(response)
}

// GetTargetAssets لیست دارایی‌ها رو با فیلتر، صفحه‌بندی و کشینگ برمی‌گردونه
func GetTargetAssets(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	// پارامترهای صفحه‌بندی
	page, _ := strconv.Atoi(c.Query("page", "1")) // دیفالت 1
	limit := utils.DefaultLimit
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := (page - 1) * limit

	// پارامترهای فیلتر (برای ساخت کلید کش)
	isLive := c.Query("is_live")
	isNew := c.Query("is_new")
	search := c.Query("search")
	filtersKey := fmt.Sprintf("live:%s|new:%s|s:%s", isLive, isNew, search)

	// 1. تلاش برای خواندن از کش (Cache Hit) 🚀
	cacheKey := cache.GenerateAssetKey(targetID, page, limit, filtersKey)
	var cachedResponse fiber.Map
	if cache.GetCache(cacheKey, &cachedResponse) {
		// اگر در کش بود، همون رو برمی‌گردونیم و بیخیال دیتابیس می‌شیم
		c.Set("X-Cache", "HIT") // هدر برای دیباگ
		return c.JSON(cachedResponse)
	}

	// 2. اگر در کش نبود (Cache Miss)، میره سراغ دیتابیس 🐢
	db := database.DB.Model(&models.Asset{}).Where("target_id = ?", id)

	if isLive != "" {
		db = db.Where("is_live = ?", isLive == "true")
	}
	if isNew != "" {
		db = db.Where("is_new = ?", isNew == "true")
	}
	if search != "" {
		db = db.Where("value LIKE ?", "%"+search+"%")
	}

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var assets []models.Asset
	// نکته: اینجا دیگه از Scope صفحه‌بندی استفاده نمی‌کنیم چون دستی محاسبه کردیم برای کش
	result := db.Order("value asc").Limit(limit).Offset(offset).Find(&assets)

	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": result.Error.Error()})
	}

	assetResponses := make([]dto.AssetResponse, len(assets))
	for i, a := range assets {
		assetResponses[i] = toAssetResponse(a)
	}

	response := fiber.Map{
		"status":      "success",
		"data":        assetResponses,
		"page_count":  len(assetResponses),
		"total_count": totalCount,
	}

	// 3. ذخیره در کش برای دفعه بعد 💾
	cache.SetCache(cacheKey, response)

	c.Set("X-Cache", "MISS")
	return c.JSON(response)
}

// StartProbing هندلر شروع دستی فاز ۲ (پروبینگ) برای یک تارگت است
func StartProbing(c *fiber.Ctx) error {
	id := c.Params("id")
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	if target.Status == "SCANNING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Target is already being scanned."})
	}

	// 👇👇👇 فیکس باگ ۲
	database.DB.Model(&target).Updates(map[string]interface{}{
		"status":        "SCANNING",
		"current_phase": "QUEUED: STARTING PHASE 2...",
	})

	taskPayload := fmt.Sprintf("PROBING:%d:%s", target.ID, target.RootDomain)
	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err(); err != nil {
		database.DB.Model(&target).Update("status", "READY")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue job"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Phase 2 started for target: %s", target.Name),
	})
}

// ==========================================
// Helper Functions (Model -> DTO Mapper)
// ==========================================
func toTargetResponse(t models.Target, assetCount int64) dto.TargetResponse {
	return dto.TargetResponse{
		ID:          t.ID,
		Name:        t.Name,
		RootDomain:  t.RootDomain,
		Description: t.Description,
		InScope:     t.InScope,
		CreatedAt:   t.CreatedAt,
		AssetCount:  assetCount,

		// 👇👇👇 مپ کردن فیلدهای جدید
		Frequency:    t.Frequency,
		LastScanAt:   t.LastScanAt,
		Status:       t.Status,
		CurrentPhase: t.CurrentPhase,
	}
}

// toAssetResponse مدل دیتابیس رو به DTO تبدیل می‌کنه
func toAssetResponse(a models.Asset) dto.AssetResponse {
	// پارس کردن فیلد Technologies (که یک رشته JSON هست) به یک آبجکت
	var techs interface{}
	if a.Technologies != "" && a.Technologies != "[]" && a.Technologies != "null" {
		// خطا رو نادیده می‌گیریم، اگر پارس نشه null می‌مونه
		_ = json.Unmarshal([]byte(a.Technologies), &techs)
	}

	// 👇👇👇 پارس کردن فیلد جدید RawHttpx
	var rawHttpx interface{}
	// چک می‌کنیم که خالی یا آبجکت خالی نباشه
	if a.RawHttpx != "" && a.RawHttpx != "{}" && a.RawHttpx != "null" {
		_ = json.Unmarshal([]byte(a.RawHttpx), &rawHttpx)
	}

	return dto.AssetResponse{
		ID:            a.ID,
		Value:         a.Value,
		Type:          a.Type,
		IsNew:         a.IsNew,
		IsLive:        a.IsLive,
		CreatedAt:     a.CreatedAt,
		FinalURL:      a.FinalURL,
		StatusCode:    a.StatusCode,
		Title:         a.Title,
		ContentLength: a.ContentLength,
		HostIP:        a.HostIP,
		DnsxIP:        a.DnsxIP, // 👈 اضافه شد
		WebServer:     a.WebServer,
		CDNName:       a.CDNName,
		Technologies:  techs, // استفاده از آبجکت پارس شده
		ResponseTime:  a.ResponseTimeMs,
		// 👇👇👇 اضافه کردن فیلد خام به خروجی
		RawHttpx: rawHttpx,
	}
}

// StartDiscovery هندلر شروع مجدد فاز ۱ (کشف) برای یک تارگت موجود است
func StartDiscovery(c *fiber.Ctx) error {
	id := c.Params("id")
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	if target.Status == "SCANNING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Target is already being scanned."})
	}

	// 👇👇👇 فیکس باگ ۲: تنظیم وضعیت و فاز اولیه بلافاصله
	database.DB.Model(&target).Updates(map[string]interface{}{
		"status":        "SCANNING",
		"current_phase": "QUEUED: STARTING PHASE 1...", // تا کاربر بفهمد دستور ثبت شده
	})

	taskPayload := fmt.Sprintf("DISCOVERY:%d:%s", target.ID, target.RootDomain)
	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err(); err != nil {
		// اگر خطا داد، وضعیت را برگردان
		database.DB.Model(&target).Update("status", "READY")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue job"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Phase 1 started for target: %s", target.Name),
	})
}

// UpdateTarget ویرایش تنظیمات یک تارگت
func UpdateTarget(c *fiber.Ctx) error {
	id := c.Params("id")

	// 1. یافتن تارگت
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	// 2. پارس کردن بادی درخواست
	req := new(dto.UpdateTargetRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid body"})
	}

	// 3. اعمال تغییرات (فقط اگر مقدار ارسال شده باشد)
	if req.Name != "" {
		target.Name = req.Name
	}
	if req.Description != "" {
		target.Description = req.Description
	}
	if req.Frequency != nil {
		target.Frequency = *req.Frequency
	}
	if req.InScope != nil {
		target.InScope = *req.InScope
	}
	if len(req.Modules) > 0 {
		bytes, _ := json.Marshal(req.Modules)
		target.ScanModules = string(bytes)
	}

	// 4. ذخیره
	if err := database.DB.Save(&target).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update target"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Target updated successfully",
		"data":    target,
	})
}

// DeleteTarget حذف کامل تارگت و تمام داده‌های وابسته (Cascade Delete دستی)
func DeleteTarget(c *fiber.Ctx) error {
	id := c.Params("id")

	// استفاده از تراکنش برای اطمینان از حذف کامل
	err := database.DB.Transaction(func(tx *gorm.DB) error {
		// 1. چک کردن وجود تارگت
		var target models.Target
		if err := tx.First(&target, id).Error; err != nil {
			return err // اگر پیدا نشد، تراکنش لغو میشه
		}

		// 2. حذف تاریخچه‌ها (AssetHistory) - چون به Asset وابسته‌اند
		// کوئری: حذف تاریخچه‌هایی که AssetID آن‌ها مربوط به این TargetID است
		if err := tx.Exec("DELETE FROM asset_histories WHERE asset_id IN (SELECT id FROM assets WHERE target_id = ?)", id).Error; err != nil {
			return err
		}

		// 3. حذف دارایی‌ها (Assets)
		if err := tx.Exec("DELETE FROM assets WHERE target_id = ?", id).Error; err != nil {
			return err
		}

		// 4. حذف خود تارگت (Target)
		// استفاده از Unscoped برای حذف فیزیکی (نه Soft Delete)
		if err := tx.Unscoped().Delete(&target).Error; err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Failed to delete target (DB Error)",
			"details": err.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Target and all associated data deleted successfully",
	})
}

// StopScan درخواست توقف اسکن را ثبت می‌کند
func StopScan(c *fiber.Ctx) error {
	id := c.Params("id")

	// فقط فیلد stop_requested را true می‌کنیم
	// ورکر خودش در دور بعدی چک می‌کند و متوقف می‌شود
	if err := database.DB.Model(&models.Target{}).Where("id = ?", id).Update("stop_requested", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to stop scan"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Stop signal sent. Scan will halt shortly.",
	})
}
