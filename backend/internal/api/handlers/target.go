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
	"github.com/omidxplimbo/hunt-engine/backend/internal/utils"
	"gorm.io/gorm"
)

// CreateTarget هندلر ساخت یک هدف جدید است
func CreateTarget(c *fiber.Ctx) error {
	req := new(dto.CreateTargetRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "Invalid request body format",
			"error":   err.Error(),
		})
	}

	modulesJSON := "[]"
	if len(req.Modules) > 0 {
		bytes, _ := json.Marshal(req.Modules)
		modulesJSON = string(bytes)
	} else {
		modulesJSON = "[\"DISCOVERY\", \"PROBING\"]"
	}

	target := models.Target{
		Name:         req.Name,
		RootDomain:   req.RootDomain,
		Description:  req.Description,
		InScope:      true,
		Frequency:    req.Frequency,
		ScanModules:  modulesJSON,
		Status:       "SCANNING",
		CurrentPhase: "QUEUED: STARTING...",
	}

	if req.UseAlterx != nil {
		target.UseAlterx = *req.UseAlterx
	} else {
		target.UseAlterx = false
	}

	if err := database.DB.Create(&target).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "Could not save target to database",
			"error":   err.Error(),
		})
	}

	taskPayload := fmt.Sprintf("DISCOVERY:%d:%s", target.ID, target.RootDomain)
	err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err()
	if err != nil {
		database.DB.Model(&target).Update("status", "READY")
		fmt.Printf("⚠️ Failed to enqueue discovery task for %s: %v\n", target.RootDomain, err)
	} else {
		fmt.Printf("Have enqueued discovery task for: %s [Payload: %s]\n", target.RootDomain, taskPayload)
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status":  "success",
		"message": "Target created successfully AND discovery queued!",
		"data":    target,
	})
}

// GetTargets لیست تمام تارگت‌ها را به صورت صفحه‌بندی شده برمی‌گردونه (با کش ۵ ثانیه)
// GetTargets لیست تمام تارگت‌ها را به صورت صفحه‌بندی شده برمی‌گردونه (با کش ۵ ثانیه)
func GetTargets(c *fiber.Ctx) error {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	// 👇 اصلاح: خواندن offset از کوئری (اولویت با offset است)
	offset := 0
	if o, err := strconv.Atoi(c.Query("offset")); err == nil && o >= 0 {
		offset = o
	} else {
		// اگر offset نبود، از page حساب کن (Fallback)
		page, _ := strconv.Atoi(c.Query("page", "1"))
		offset = (page - 1) * limit
	}

	// 👇 استفاده از offset در کلید کش
	cacheKey := fmt.Sprintf("targets:list:o:%d:l:%d", offset, limit)

	var cachedResponse fiber.Map
	if cache.GetCache(cacheKey, &cachedResponse) {
		c.Set("X-Cache", "HIT")
		return c.JSON(cachedResponse)
	}

	var targets []models.Target
	// استفاده مستقیم از Limit و Offset محاسبه شده
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

	// Offset logic
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
	// 👇 پارامتر جدید
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

	// 👇 آپدیت کلید کش با پارامتر dnsOnly
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
		// شرط: host_ip (نتیجه httpx) خالی نباشد
		db = db.Where("jsonb_array_length(host_ip) > 0")
	}

	// 👇👇👇 شرط جدید: DNS Only
	// یعنی: dnsx_ip داشته باشد ولی host_ip نداشته باشد
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

// StartDiscovery هندلر شروع مجدد فاز ۱
func StartDiscovery(c *fiber.Ctx) error {
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
		"current_phase": "QUEUED: STARTING PHASE 1...",
	})

	taskPayload := fmt.Sprintf("DISCOVERY:%d:%s", target.ID, target.RootDomain)
	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err(); err != nil {
		database.DB.Model(&target).Update("status", "READY")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue job"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": fmt.Sprintf("Phase 1 started for target: %s", target.Name),
	})
}

// UpdateTarget ویرایش تارگت
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
	if len(req.Modules) > 0 {
		bytes, _ := json.Marshal(req.Modules)
		target.ScanModules = string(bytes)
	}

	// 👇👇👇 فیکس نهایی: اعمال تغییر UseAlterx در ویرایش
	if req.UseAlterx != nil {
		target.UseAlterx = *req.UseAlterx
	}

	if err := database.DB.Save(&target).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update target"})
	}

	return c.JSON(fiber.Map{
		"status":  "success",
		"message": "Target updated successfully",
		"data":    target,
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

// ==========================================
// Helper Functions (Model -> DTO Mapper)
// ==========================================
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
