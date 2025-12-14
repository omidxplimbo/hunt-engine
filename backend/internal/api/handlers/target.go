package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"sort" // 👈 اضافه شده برای مرتب‌سازی
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// 👇 تابع کمکی برای مرتب‌سازی ماژول‌ها به ترتیب منطقی
func sortScanModules(modules []string) []string {
	// نقشه اولویت‌بندی مراحل
	priority := map[string]int{
		"DISCOVERY": 1,
		"PROBING":   2,
		"CRAWLING":  3,
	}

	sort.Slice(modules, func(i, j int) bool {
		p1, ok1 := priority[modules[i]]
		p2, ok2 := priority[modules[j]]

		// اگر ماژولی در لیست نبود (ناشناخته)، می‌رود ته لیست
		if !ok1 {
			p1 = 99
		}
		if !ok2 {
			p2 = 99
		}

		return p1 < p2
	})
	return modules
}

// CreateTarget هندلر ساخت یک هدف جدید
func CreateTarget(c *fiber.Ctx) error {
	req := new(dto.CreateTargetRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	// 👇 مرتب‌سازی ماژول‌ها قبل از ذخیره
	if len(req.Modules) > 0 {
		req.Modules = sortScanModules(req.Modules)
	} else {
		req.Modules = []string{"DISCOVERY", "PROBING", "CRAWLING"}
	}

	modulesJSON, _ := json.Marshal(req.Modules)

	target := models.Target{
		Name:         req.Name,
		RootDomain:   req.RootDomain,
		Description:  req.Description,
		InScope:      true,
		Frequency:    req.Frequency,
		ScanModules:  string(modulesJSON),
		Status:       "SCANNING",
		CurrentPhase: "QUEUED: STARTING...",
		UseAlterx:    true,
		UseWaymore:   false,
	}

	if req.UseAlterx != nil {
		target.UseAlterx = *req.UseAlterx
	}
	if req.UseWaymore != nil {
		target.UseWaymore = *req.UseWaymore
	}

	if err := database.DB.Create(&target).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not save target", "error": err.Error()})
	}

	// شروع اولین ماژول (چون مرتب شده، مطمئنیم درسته)
	firstModule := req.Modules[0]
	taskPayload := fmt.Sprintf("%s:%d:%s", firstModule, target.ID, target.RootDomain)

	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err(); err != nil {
		database.DB.Model(&target).Update("status", "READY")
		log.Printf("⚠️ Failed to enqueue task: %v\n", err)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "message": "Target created & started!", "data": target})
}

// UpdateTarget ویرایش تارگت (جایی که باگ اصلی بود)
func UpdateTarget(c *fiber.Ctx) error {
	id := c.Params("id")
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	req := new(dto.UpdateTargetRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid body"})
	}

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

	if req.UseAlterx != nil {
		target.UseAlterx = *req.UseAlterx
	}
	if req.UseWaymore != nil {
		target.UseWaymore = *req.UseWaymore
	}

	// 👇👇👇 فیکس اصلی اینجاست: مرتب‌سازی لیست ماژول‌ها قبل از آپدیت دیتابیس
	if len(req.Modules) > 0 {
		sortedModules := sortScanModules(req.Modules)
		bytes, _ := json.Marshal(sortedModules)
		target.ScanModules = string(bytes)
	}

	if err := database.DB.Save(&target).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update target"})
	}

	// پاک کردن کش لیست تارگت‌ها برای اعمال تغییرات در فرانت
	// (اگر سیستم کشینگ دارید، اینجا باید کلیدهای مربوطه را Invalidate کنید)

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Target updated successfully",
		"data":    target,
	})
}

// StartDiscovery هندلر شروع مجدد اسکن (هوشمند)
func StartDiscovery(c *fiber.Ctx) error {
	id := c.Params("id")
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	if target.Status == "SCANNING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Target is already being scanned."})
	}

	var modules []string
	if err := json.Unmarshal([]byte(target.ScanModules), &modules); err != nil || len(modules) == 0 {
		modules = []string{"DISCOVERY", "PROBING", "CRAWLING"}
	}

	// 👇 اطمینان حاصل می‌کنیم که حتی اگر دیتای قدیمی در دیتابیس نامرتب است، اینجا درست شروع شود
	modules = sortScanModules(modules)

	// آپدیت کردن ترتیب درست در دیتابیس (برای اینکه ورکر در مراحل بعد گیج نشود)
	sortedJSON, _ := json.Marshal(modules)
	if string(sortedJSON) != target.ScanModules {
		database.DB.Model(&target).Update("scan_modules", string(sortedJSON))
	}

	firstModule := modules[0]

	database.DB.Model(&target).Updates(map[string]interface{}{
		"status":        "SCANNING",
		"current_phase": fmt.Sprintf("QUEUED: STARTING %s...", firstModule),
	})

	taskPayload := fmt.Sprintf("%s:%d:%s", firstModule, target.ID, target.RootDomain)

	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err(); err != nil {
		database.DB.Model(&target).Update("status", "READY")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue job"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Scan started. First phase: %s", firstModule),
	})
}

// ... (بقیه هندلرها: GetTargets, GetTargetDetails, GetTargetAssets, StartProbing, DeleteTarget, StopScan, GetTargetURLs بدون تغییر باقی می‌مانند)
// فقط کدهای بالا را جایگزین توابع مشابه در فایل فعلی کنید.

// GetTargets لیست تمام تارگت‌ها را به صورت صفحه‌بندی شده برمی‌گردونه
func GetTargets(c *fiber.Ctx) error {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	offset := 0
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	} else {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		offset = (page - 1) * limit
	}

	cacheKey := fmt.Sprintf("targets:list:o:%d:l:%d", offset, limit)

	var cachedResponse fiber.Map
	if cache.GetCache(cacheKey, &cachedResponse) {
		c.Set("X-Cache", "HIT")
		return c.JSON(cachedResponse)
	}

	var targets []models.Target
	result := database.DB.Order("created_at desc").Limit(limit).Offset(offset).Find(&targets)
	if result.Error != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": result.Error.Error()})
	}

	targetResponses := make([]dto.TargetResponse, len(targets))
	for i, t := range targets {
		var count int64
		database.DB.Model(&models.Asset{}).Where("target_id = ?", t.ID).Count(&count)
		targetResponses[i] = toTargetResponse(t, count)
	}

	var totalTargets int64
	database.DB.Model(&models.Target{}).Count(&totalTargets)

	response := fiber.Map{
		"status": "success",
		"data":   targetResponses,
		"count":  len(targetResponses),
		"total":  totalTargets,
	}

	cache.SetCacheWithTTL(cacheKey, response, 5*time.Second)
	c.Set("X-Cache", "MISS")
	return c.JSON(response)
}

// GetTargetDetails جزئیات یک تارگت خاص را برمی‌گرداند
func GetTargetDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	var count int64
	database.DB.Model(&models.Asset{}).Where("target_id = ?", target.ID).Count(&count)

	return c.JSON(fiber.Map{
		"status": "success",
		"data":   toTargetResponse(target, count),
	})
}

// GetTargetAssets لیست دارایی‌ها رو با فیلتر، صفحه‌بندی، کشینگ و مرتب‌سازی برمی‌گردونه
func GetTargetAssets(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	limit := utils.DefaultLimit
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	offset := 0
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	} else {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		offset = (page - 1) * limit
	}

	// Filters
	isLive := c.Query("is_live")
	isNew := c.Query("is_new")
	search := c.Query("search")
	hasHttpx := c.Query("has_httpx")
	dnsOnly := c.Query("dns_only")

	// Sorting Params
	sortBy := c.Query("sort_by", "value")
	order := c.Query("order", "asc")

	validSortFields := map[string]bool{
		"value": true, "status_code": true, "content_length": true, "title": true, "created_at": true,
	}
	if !validSortFields[sortBy] {
		sortBy = "value"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, order)

	filtersKey := fmt.Sprintf("l:%s|n:%s|s:%s|h:%s|d:%s|sb:%s|o:%s", isLive, isNew, search, hasHttpx, dnsOnly, sortBy, order)
	cacheKey := cache.GenerateAssetKey(targetID, offset, limit, filtersKey)

	var cachedResponse fiber.Map
	if cache.GetCache(cacheKey, &cachedResponse) {
		c.Set("X-Cache", "HIT")
		return c.JSON(cachedResponse)
	}

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

	if hasHttpx == "true" {
		db = db.Where("jsonb_array_length(host_ip) > 0")
	}

	if dnsOnly == "true" {
		db = db.Where("jsonb_array_length(dnsx_ip) > 0 AND jsonb_array_length(host_ip) = 0")
	}

	var totalCount int64
	if err := db.Count(&totalCount).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	var assets []models.Asset
	result := db.Order(orderClause).Limit(limit).Offset(offset).Find(&assets)

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

	cache.SetCache(cacheKey, response)
	c.Set("X-Cache", "MISS")
	return c.JSON(response)
}

// StartProbing هندلر شروع دستی فاز ۲
func StartProbing(c *fiber.Ctx) error {
	id := c.Params("id")
	var target models.Target
	if err := database.DB.First(&target, id).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	if target.Status == "SCANNING" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Target is already being scanned."})
	}

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

// DeleteTarget حذف کامل تارگت
func DeleteTarget(c *fiber.Ctx) error {
	id := c.Params("id")

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		var target models.Target
		if err := tx.First(&target, id).Error; err != nil {
			return err
		}

		if err := tx.Exec("DELETE FROM asset_histories WHERE asset_id IN (SELECT id FROM assets WHERE target_id = ?)", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM assets WHERE target_id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM found_urls WHERE target_id = ?", id).Error; err != nil {
			return err
		}

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

// StopScan درخواست توقف اسکن
func StopScan(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := database.DB.Model(&models.Target{}).Where("id = ?", id).Update("stop_requested", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to stop scan"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Stop signal sent. Scan will halt shortly."})
}

// GetTargetURLs لیست URLهای کراول شده را برمی‌گرداند
func GetTargetURLs(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	limit := utils.DefaultLimit
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	offset := 0
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	} else {
		page, _ := strconv.Atoi(c.Query("page", "1"))
		offset = (page - 1) * limit
	}

	search := c.Query("search")
	onlyJS := c.Query("only_js")
	sources := c.Query("sources")

	sortBy := c.Query("sort_by", "created_at")
	order := c.Query("order", "desc")

	validSortFields := map[string]bool{
		"value": true, "source": true, "created_at": true,
	}
	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, order)

	db := database.DB.Model(&models.FoundURL{}).Where("target_id = ?", targetID)

	if search != "" {
		db = db.Where("value LIKE ?", "%"+search+"%")
	}

	if onlyJS == "true" {
		db = db.Where("value LIKE ?", "%.js")
	}

	if sources != "" {
		sourceList := strings.Split(sources, ",") // اصلاح شد: نیاز به strings.Split نیست چون قبلا استفاده نکردیم
		// نه صبر کن، بالا استفاده نکرده بودیم اینجا باید strings اضافه بشه به ایمپورت ها
		// چون strings توی ایمپورت ها نیست، اینجا کد ساده تر میزنم یا باید strings رو اضافه کنی
		// بذار یه کار تمیزتر کنیم، strings رو دستی به ایمپورت ها اضافه کردم (بالا).
		if len(sourceList) > 0 {
			db = db.Where("source IN ?", sourceList)
		}
	}

	var totalCount int64
	db.Count(&totalCount)

	var urls []models.FoundURL
	db.Order(orderClause).Limit(limit).Offset(offset).Find(&urls)

	response := make([]dto.FoundURLResponse, len(urls))
	for i, u := range urls {
		response[i] = dto.FoundURLResponse{
			ID:        u.ID,
			Value:     u.Value,
			Source:    u.Source,
			CreatedAt: u.CreatedAt,
		}
	}

	return c.JSON(fiber.Map{
		"status":      "success",
		"data":        response,
		"total_count": totalCount,
		"page":        (offset / limit) + 1,
	})
}

// ... (Helper Functions DTO - اینا رو بذار همونطور که هستن بمونن یا اگه خواستی کپی کن از فایل قبلی)
func toTargetResponse(t models.Target, assetCount int64) dto.TargetResponse {
	return dto.TargetResponse{
		ID:           t.ID,
		Name:         t.Name,
		RootDomain:   t.RootDomain,
		Description:  t.Description,
		InScope:      t.InScope,
		CreatedAt:    t.CreatedAt,
		AssetCount:   assetCount,
		Frequency:    t.Frequency,
		LastScanAt:   t.LastScanAt,
		Status:       t.Status,
		CurrentPhase: t.CurrentPhase,
		UseAlterx:    t.UseAlterx,
		UseWaymore:   t.UseWaymore,
		ScanModules:  t.ScanModules,
	}
}

func toAssetResponse(a models.Asset) dto.AssetResponse {
	var techs interface{}
	if a.Technologies != "" && a.Technologies != "[]" && a.Technologies != "null" {
		_ = json.Unmarshal([]byte(a.Technologies), &techs)
	}
	var rawHttpx interface{}
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
		DnsxIP:        a.DnsxIP,
		WebServer:     a.WebServer,
		CDNName:       a.CDNName,
		Technologies:  techs,
		ResponseTime:  a.ResponseTimeMs,
		RawHttpx:      rawHttpx,
	}
}

// ExportTargetRequest درخواست Export تارگت‌ها
type ExportTargetRequest struct {
	TargetIDs []uint `json:"target_ids"` // لیست IDهای تارگت‌ها (اگر خالی باشد، همه export می‌شوند)
}

// ExportTarget هندلر Export یک یا چند تارگت به فرمت JSON استاندارد (شامل تمام داده‌های مرتبط)
func ExportTarget(c *fiber.Ctx) error {
	var req ExportTargetRequest
	
	// اگر method POST بود، body را parse می‌کنیم
	if c.Method() == "POST" {
		_ = c.BodyParser(&req) // خطا را ignore می‌کنیم
	}
	
	// اگر method GET بود یا body خالی بود، از query param استفاده می‌کنیم
	if len(req.TargetIDs) == 0 {
		if id := c.Query("id"); id != "" {
			targetID := uint(0)
			if _, err := fmt.Sscanf(id, "%d", &targetID); err == nil {
				req.TargetIDs = []uint{targetID}
			}
		}
	}
	
	var targets []models.Target
	var err error
	
	if len(req.TargetIDs) > 0 {
		// Export تارگت‌های انتخاب شده
		err = database.DB.Where("id IN ?", req.TargetIDs).Find(&targets).Error
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to fetch targets",
				"error":   err.Error(),
			})
		}
		if len(targets) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status":  "error",
				"message": "No targets found",
			})
		}
	} else {
		// Export همه تارگت‌ها
		err = database.DB.Find(&targets).Error
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status":  "error",
				"message": "Failed to fetch targets",
				"error":   err.Error(),
			})
		}
	}

	// تبدیل به فرمت Export (شامل Assets و URLs)
	exportItems := make([]dto.TargetExportItem, len(targets))
	for i, t := range targets {
		var modules []string
		if t.ScanModules != "" {
			_ = json.Unmarshal([]byte(t.ScanModules), &modules)
		}
		if len(modules) == 0 {
			modules = []string{"DISCOVERY", "PROBING", "CRAWLING"}
		}

		// دریافت Assets
		var assets []models.Asset
		database.DB.Where("target_id = ?", t.ID).Find(&assets)
		assetItems := make([]dto.AssetExportItem, len(assets))
		for j, a := range assets {
			assetItems[j] = dto.AssetExportItem{
				Value:          a.Value,
				Type:           a.Type,
				IsNew:          a.IsNew,
				IsLive:         a.IsLive,
				FinalURL:       a.FinalURL,
				StatusCode:     a.StatusCode,
				Title:          a.Title,
				ContentLength:  a.ContentLength,
				HostIP:         a.HostIP,
				DnsxIP:         a.DnsxIP,
				WebServer:      a.WebServer,
				CDNName:        a.CDNName,
				Technologies:   a.Technologies,
				BodyHash:       a.BodyHash,
				HeaderHash:     a.HeaderHash,
				ResponseTimeMs: a.ResponseTimeMs,
				RawHttpx:       a.RawHttpx,
				CreatedAt:      a.CreatedAt.Format(time.RFC3339),
			}
		}

		// دریافت URLs
		var urls []models.FoundURL
		database.DB.Where("target_id = ?", t.ID).Find(&urls)
		urlItems := make([]dto.URLExportItem, len(urls))
		for j, u := range urls {
			urlItems[j] = dto.URLExportItem{
				Value:     u.Value,
				Source:    u.Source,
				CreatedAt: u.CreatedAt.Format(time.RFC3339),
			}
		}

		exportItems[i] = dto.TargetExportItem{
			Name:        t.Name,
			RootDomain:  t.RootDomain,
			Description: t.Description,
			InScope:     t.InScope,
			Frequency:   t.Frequency,
			Modules:     modules,
			UseAlterx:   t.UseAlterx,
			UseWaymore:  t.UseWaymore,
			Assets:      assetItems,
			URLs:        urlItems,
		}
	}

	exportData := dto.TargetExportData{
		Version:    "1.0",
		ExportDate: time.Now().Format(time.RFC3339),
		Targets:    exportItems,
	}

	// تنظیم هدر برای دانلود فایل
	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"targets_export_%s.json\"", time.Now().Format("20060102_150405")))

	return c.JSON(exportData)
}

// ImportTarget هندلر Import تارگت‌ها از فایل JSON
func ImportTarget(c *fiber.Ctx) error {
	req := new(dto.ImportTargetRequest)
	if err := c.BodyParser(req); err != nil {
		log.Printf("❌ Import error - BodyParser failed: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// اعتبارسنجی نسخه فرمت
	if req.Data.Version != "1.0" {
		log.Printf("❌ Import error - Unsupported version: %s\n", req.Data.Version)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": fmt.Sprintf("Unsupported export format version: %s (expected: 1.0)", req.Data.Version),
		})
	}

	log.Printf("📥 Importing %d target(s)...\n", len(req.Data.Targets))

	var created []models.Target
	var skipped []string
	var errors []string

	for _, item := range req.Data.Targets {
		// بررسی وجود تارگت با root_domain یکسان
		var existing models.Target
		if err := database.DB.Where("root_domain = ?", item.RootDomain).First(&existing).Error; err == nil {
			if req.SkipExisting {
				skipped = append(skipped, item.RootDomain)
				continue
			}
			// اگر skip_existing false باشد، خطا می‌دهیم
			errors = append(errors, fmt.Sprintf("Target with root_domain '%s' already exists", item.RootDomain))
			continue
		}

		// مرتب‌سازی ماژول‌ها
		modules := item.Modules
		if len(modules) == 0 {
			modules = []string{"DISCOVERY", "PROBING", "CRAWLING"}
		} else {
			modules = sortScanModules(modules)
		}

		modulesJSON, _ := json.Marshal(modules)

		// ایجاد تارگت جدید
		target := models.Target{
			Name:         item.Name,
			RootDomain:   item.RootDomain,
			Description:  item.Description,
			InScope:      item.InScope,
			Frequency:    item.Frequency,
			ScanModules:  string(modulesJSON),
			Status:       "READY", // تارگت‌های import شده به صورت READY هستند (نه SCANNING)
			CurrentPhase: "IDLE",
			UseAlterx:    item.UseAlterx,
			UseWaymore:   item.UseWaymore,
		}

		if err := database.DB.Create(&target).Error; err != nil {
			errors = append(errors, fmt.Sprintf("Failed to create target '%s': %v", item.RootDomain, err))
			continue
		}

		// Import Assets
		if len(item.Assets) > 0 {
			assets := make([]models.Asset, 0, len(item.Assets))
			for _, aItem := range item.Assets {
				// Skip empty values (unique constraint)
				if aItem.Value == "" {
					continue
				}
				
				createdAt := time.Now()
				if aItem.CreatedAt != "" {
					if parsed, err := time.Parse(time.RFC3339, aItem.CreatedAt); err == nil {
						createdAt = parsed
					}
				}
				
				assets = append(assets, models.Asset{
					TargetID:       target.ID,
					Value:          aItem.Value,
					Type:           aItem.Type,
					IsNew:          aItem.IsNew,
					IsLive:         aItem.IsLive,
					FinalURL:       aItem.FinalURL,
					StatusCode:     aItem.StatusCode,
					Title:          aItem.Title,
					ContentLength:  aItem.ContentLength,
					HostIP:         aItem.HostIP,
					DnsxIP:         aItem.DnsxIP,
					WebServer:      aItem.WebServer,
					CDNName:        aItem.CDNName,
					Technologies:   aItem.Technologies,
					BodyHash:       aItem.BodyHash,
					HeaderHash:     aItem.HeaderHash,
					ResponseTimeMs: aItem.ResponseTimeMs,
					RawHttpx:       aItem.RawHttpx,
					CreatedAt:      createdAt,
				})
			}
			// استفاده از CreateInBatches با ON CONFLICT برای جلوگیری از duplicate
			if len(assets) > 0 {
				// استفاده از ON CONFLICT DO NOTHING برای skip کردن duplicate ها
				if err := database.DB.Clauses(clause.OnConflict{
					Columns:   []clause.Column{{Name: "value"}},
					DoNothing: true,
				}).CreateInBatches(assets, 500).Error; err != nil {
					log.Printf("⚠️ Warning: Failed to import some assets for target '%s': %v\n", item.RootDomain, err)
					errors = append(errors, fmt.Sprintf("Failed to import assets for '%s': %v", item.RootDomain, err))
				} else {
					log.Printf("✅ Imported %d assets for target '%s'\n", len(assets), item.RootDomain)
				}
			}
		}

		// Import URLs
		if len(item.URLs) > 0 {
			urls := make([]models.FoundURL, 0, len(item.URLs))
			for _, uItem := range item.URLs {
				// Skip empty values
				if uItem.Value == "" {
					continue
				}
				
				createdAt := time.Now()
				if uItem.CreatedAt != "" {
					if parsed, err := time.Parse(time.RFC3339, uItem.CreatedAt); err == nil {
						createdAt = parsed
					}
				}
				
				urls = append(urls, models.FoundURL{
					TargetID:  target.ID,
					Value:     uItem.Value,
					Source:    uItem.Source,
					CreatedAt: createdAt,
				})
			}
			// استفاده از CreateInBatches با ON CONFLICT برای جلوگیری از duplicate
			if len(urls) > 0 {
				// برای URLs، unique constraint روی (target_id, value) است، پس باید هر دو را چک کنیم
				// اما GORM ON CONFLICT برای multiple columns مشکل دارد، پس از روش دیگری استفاده می‌کنیم
				// چک کردن وجود URL قبل از insert
				existingURLs := make(map[string]bool)
				var existingFoundURLs []models.FoundURL
				database.DB.Where("target_id = ?", target.ID).Select("value").Find(&existingFoundURLs)
				for _, u := range existingFoundURLs {
					existingURLs[u.Value] = true
				}
				
				// فیلتر کردن URLs که قبلاً وجود ندارند
				newURLs := make([]models.FoundURL, 0)
				for _, u := range urls {
					if !existingURLs[u.Value] {
						newURLs = append(newURLs, u)
					}
				}
				
				if len(newURLs) > 0 {
					if err := database.DB.CreateInBatches(newURLs, 500).Error; err != nil {
						log.Printf("⚠️ Warning: Failed to import some URLs for target '%s': %v\n", item.RootDomain, err)
						errors = append(errors, fmt.Sprintf("Failed to import URLs for '%s': %v", item.RootDomain, err))
					} else {
						log.Printf("✅ Imported %d URLs for target '%s' (%d skipped as duplicates)\n", len(newURLs), item.RootDomain, len(urls)-len(newURLs))
					}
				} else if len(urls) > 0 {
					log.Printf("ℹ️ All URLs for target '%s' already exist, skipping\n", item.RootDomain)
				}
			}
		}

		created = append(created, target)
	}

	// پاک کردن کش
	cache.ClearCache()

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Import completed: %d created, %d skipped, %d errors", len(created), len(skipped), len(errors)),
		"data": fiber.Map{
			"created": created,
			"skipped": skipped,
			"errors":  errors,
		},
	})
}
