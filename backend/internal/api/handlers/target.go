package handlers

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
		"DISCOVERY": 1, "PROBING": 2, "CRAWLING": 3}

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

func normalizeRootDomain(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	v = strings.TrimPrefix(v, "http://")
	v = strings.TrimPrefix(v, "https://")
	v = strings.TrimPrefix(v, "*.")

	if idx := strings.IndexAny(v, "/?#"); idx >= 0 {
		v = v[:idx]
	}
	if idx := strings.Index(v, ":"); idx >= 0 {
		v = v[:idx]
	}

	return strings.Trim(v, ". ")
}

func normalizeNucleiProfile(value string) string {
	v := strings.ToLower(strings.TrimSpace(value))
	switch v {
	case "safe", "exposure", "misconfig", "cves-light", "custom", "full":
		return v
	default:
		return "safe"
	}
}

func isDuplicateTargetError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "idx_targets_owner_root_domain") ||
		strings.Contains(msg, "idx_targets_root_domain") ||
		strings.Contains(msg, "duplicate key")
}

// CreateTarget هندلر ساخت یک هدف جدید
func CreateTarget(c *fiber.Ctx) error {
	req := new(dto.CreateTargetRequest)
	if err := c.BodyParser(req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req.RootDomain = normalizeRootDomain(req.RootDomain)
	if req.RootDomain == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error", "message": "Root domain is required"})
	}

	var existingTarget models.Target
	lookupErr := database.DB.
		Where("created_by_user_id = ? AND root_domain = ?", uid, req.RootDomain).
		First(&existingTarget).Error

	if lookupErr == nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{
			"status": "error", "message": "Target already exists for this user"})
	}
	if lookupErr != nil && !stderrors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error", "message": "Failed to check existing target", "error": lookupErr.Error()})
	}

	// 👇 مرتب‌سازی ماژول‌ها قبل از ذخیره
	if len(req.Modules) > 0 {
		req.Modules = sortScanModules(req.Modules)
	} else {
		req.Modules = []string{"DISCOVERY", "PROBING", "CRAWLING"}
	}

	modulesJSON, _ := json.Marshal(req.Modules)

	target := models.Target{
		CreatedByUserID: uid, Name: req.Name, RootDomain: req.RootDomain, Description: req.Description, InScope: true, Frequency: req.Frequency, ScanModules: string(modulesJSON), Status: "QUEUED", CurrentPhase: "QUEUED", UseAlterx: true, UseWaymore: false, UsePortscan: false, UseCero: false, UseCrtsh: false, UsePuredns: false, UseAbusedb: false, UseAmass: false, UseNuclei: false, NucleiProfile: "safe", PurednsWordlists: "[]"}

	if req.UseAlterx != nil {
		target.UseAlterx = *req.UseAlterx
	}
	if req.UseWaymore != nil {
		target.UseWaymore = *req.UseWaymore
	}
	if req.UsePortscan != nil {
		target.UsePortscan = *req.UsePortscan
	}
	if req.UseCero != nil {
		target.UseCero = *req.UseCero
	}
	if req.UseCrtsh != nil {
		target.UseCrtsh = *req.UseCrtsh
	}
	if req.UseAbusedb != nil {
		target.UseAbusedb = *req.UseAbusedb
	}
	if req.UseAmass != nil {
		target.UseAmass = *req.UseAmass
	}
	if req.UsePuredns != nil {
		target.UsePuredns = *req.UsePuredns
	}
	if req.UseNuclei != nil {
		target.UseNuclei = *req.UseNuclei
	}
	if strings.TrimSpace(req.NucleiProfile) != "" {
		target.NucleiProfile = normalizeNucleiProfile(req.NucleiProfile)
	}
	if len(req.PurednsWordlists) > 0 {
		wordlistsJSON, _ := json.Marshal(req.PurednsWordlists)
		target.PurednsWordlists = string(wordlistsJSON)
	}

	if err := database.DB.Create(&target).Error; err != nil {
		if isDuplicateTargetError(err) {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{
				"status": "error", "message": "Target already exists for this user", "error": err.Error()})
		}

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Could not save target", "error": err.Error()})
	}

	// شروع اولین ماژول (چون مرتب شده، مطمئنیم درسته)
	firstModule := req.Modules[0]
	taskPayload := fmt.Sprintf("%s:%d:%s", firstModule, target.ID, target.RootDomain)

	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueNameForUser(target.CreatedByUserID), taskPayload).Err(); err != nil {
		database.DB.Model(&target).Update("status", "READY")
		log.Printf("⚠️ Failed to enqueue task: %v\n", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue task", "error": err.Error()})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"status": "success", "message": "Target created & started!", "data": target})
}

// UpdateTarget ویرایش تارگت (جایی که باگ اصلی بود)
func UpdateTarget(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
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
	if req.UsePortscan != nil {
		target.UsePortscan = *req.UsePortscan
	}
	if req.UseCero != nil {
		target.UseCero = *req.UseCero
	}
	if req.UseCrtsh != nil {
		target.UseCrtsh = *req.UseCrtsh
	}
	if req.UsePuredns != nil {
		target.UsePuredns = *req.UsePuredns
	}
	if req.UseAbusedb != nil {
		target.UseAbusedb = *req.UseAbusedb
	}
	if req.UseAmass != nil {
		target.UseAmass = *req.UseAmass
	}
	if req.UseNuclei != nil {
		target.UseNuclei = *req.UseNuclei
	}
	if strings.TrimSpace(req.NucleiProfile) != "" {
		target.NucleiProfile = normalizeNucleiProfile(req.NucleiProfile)
	}
	if len(req.PurednsWordlists) > 0 {
		wordlistsJSON, _ := json.Marshal(req.PurednsWordlists)
		target.PurednsWordlists = string(wordlistsJSON)
	} else if req.PurednsWordlists != nil && len(req.PurednsWordlists) == 0 {
		// اگر لیست خالی باشد، آن را به [] تنظیم می‌کنیم
		target.PurednsWordlists = "[]"
	}

	// 👇👇👇 فیکس اصلی اینجاست: مرتب‌سازی لیست ماژول‌ها قبل از آپدیت دیتابیس
	if len(req.Modules) > 0 {
		sortedModules := sortScanModules(req.Modules)
		bytes, _ := json.Marshal(sortedModules)
		target.ScanModules = string(bytes)
	}

	if err := database.DB.Save(target).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to update target"})
	}

	// پاک کردن کش لیست تارگت‌ها برای اعمال تغییرات در فرانت
	// (اگر سیستم کشینگ دارید، اینجا باید کلیدهای مربوطه را Invalidate کنید)

	return c.JSON(fiber.Map{
		"status": "success", "message": "Target updated successfully", "data": target})
}

// StartDiscovery هندلر شروع مجدد اسکن (هوشمند)
func StartDiscovery(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
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
		database.DB.Model(target).Update("scan_modules", string(sortedJSON))
	}

	// --- Load scan state (if any) and decide whether this is a NEW RUN or a RESUME ---
	var st models.TargetScanState
	stErr := database.DB.Where("target_id = ?", target.ID).First(&st).Error

	// If the target is marked SCANNING or QUEUED, only allow "resume" when heartbeat is stale (crash).
	if target.Status == "SCANNING" || target.Status == "QUEUED" {
		if stErr == nil && st.HeartbeatAt != nil && time.Since(*st.HeartbeatAt) < 2*time.Minute {
			return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Target is already being scanned or queued."})
		}
		// stale heartbeat -> allow user to re-queue and continue from checkpoint
	}

	// If user presses EXECUTE while READY, we treat it as a new run and reset checkpoints.
	isNewRun := target.Status == "READY"
	if isNewRun {
		newRunID := fmt.Sprintf("run_%d", time.Now().UnixNano())
		if stErr != nil {
			_ = database.DB.Create(&models.TargetScanState{
				TargetID: target.ID, RunID: newRunID, Status: "PAUSED", CompletedSteps: "[]", Meta: "{}", CurrentModule: "", CurrentStep: "", HeartbeatAt: nil, LastError: ""}).Error
		} else {
			_ = database.DB.Model(&models.TargetScanState{}).
				Where("target_id = ?", target.ID).
				Updates(map[string]interface{}{
					"run_id": newRunID, "status": "PAUSED", "completed_steps": "[]", "meta": "{}", "current_module": "", "current_step": "", "heartbeat_at": nil, "last_error": ""}).Error
		}
		// reset target "paused by user" etc
		_ = database.DB.Model(target).Update("current_phase", "QUEUED: STARTING...").Error
	}

	// Choose resume module: first module in order that doesn't have <MODULE>:DONE in scan state.
	resumeModule := modules[0]
	// Fresh runs already reset completed_steps in DB above. Do not use the old
	// in-memory st value that was loaded before the reset, otherwise a READY target
	// can skip DISCOVERY and start directly at PROBING.
	if !isNewRun && stErr == nil && st.CompletedSteps != "" && st.CompletedSteps != "[]" {
		var completed []string
		if json.Unmarshal([]byte(st.CompletedSteps), &completed) == nil {
			set := make(map[string]bool)
			for _, k := range completed {
				set[strings.TrimSpace(k)] = true
			}
			for _, m := range modules {
				if !set[fmt.Sprintf("%s:DONE", m)] {
					resumeModule = m
					break
				}
			}
		}
	}

	database.DB.Model(target).Updates(map[string]interface{}{
		"status": "QUEUED", "current_phase": fmt.Sprintf("QUEUED: STARTING %s...", resumeModule), "stop_requested": false})

	taskPayload := fmt.Sprintf("%s:%d:%s", resumeModule, target.ID, target.RootDomain)

	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueNameForUser(target.CreatedByUserID), taskPayload).Err(); err != nil {
		database.DB.Model(&target).Update("status", "READY")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue job"})
	}

	return c.JSON(fiber.Map{
		"status": "success", "message": fmt.Sprintf("Scan started. Next phase: %s", resumeModule)})
}

// ResumeTargetScan is an alias for StartDiscovery (kept for clarity in UI/API).
func ResumeTargetScan(c *fiber.Ctx) error {
	return StartDiscovery(c)
}

// GetTargetScanState returns persistent checkpoint/progress for a target.
func GetTargetScanState(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	var st models.TargetScanState
	if err := database.DB.Where("target_id = ?", target.ID).First(&st).Error; err != nil {
		return c.JSON(fiber.Map{"status": "success", "data": fiber.Map{"exists": false}})
	}

	return c.JSON(fiber.Map{
		"status": "success", "data": st})
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

	// Optional filter: only targets that have at least one asset with open ports
	withPortsOnly := c.Query("with_ports") == "true"

	scopedDB, uid, err := scopedTargetsDB(c)
	if err != nil {
		return err
	}

	cacheKey := fmt.Sprintf("targets:list:u:%d:r:%s:o:%d:l:%d:with_ports:%t", uid, currentUserRole(c), offset, limit, withPortsOnly)

	var cachedResponse fiber.Map
	if cache.GetCache(cacheKey, &cachedResponse) {
		c.Set("X-Cache", "HIT")
		return c.JSON(cachedResponse)
	}

	var targets []models.Target
	db := scopedDB.Preload("CreatedByUser").Order("created_at desc")
	if withPortsOnly {
		// open_ports is jsonb (default '{}'); we consider "has ports" when it's a non-empty JSON object
		db = db.Where(`
			EXISTS (
				SELECT 1
				FROM assets a
				WHERE a.target_id = targets.id
				  AND a.open_ports IS NOT NULL
				  AND a.open_ports <> '{}'::jsonb
				  AND a.open_ports <> 'null'::jsonb
			)
		`)
	}

	result := db.Limit(limit).Offset(offset).Find(&targets)
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
	countDB := scopedDB
	if withPortsOnly {
		countDB = countDB.Where(`
			EXISTS (
				SELECT 1
				FROM assets a
				WHERE a.target_id = targets.id
				  AND a.open_ports IS NOT NULL
				  AND a.open_ports <> '{}'::jsonb
				  AND a.open_ports <> 'null'::jsonb
			)
		`)
	}
	countDB.Count(&totalTargets)

	response := fiber.Map{
		"status": "success", "data": targetResponses, "count": len(targetResponses), "total": totalTargets}

	cache.SetCacheWithTTL(cacheKey, response, 5*time.Second)
	c.Set("X-Cache", "MISS")
	return c.JSON(response)
}

// GetTargetDetails جزئیات یک تارگت خاص را برمی‌گرداند
func GetTargetDetails(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	var count int64
	database.DB.Model(&models.Asset{}).Where("target_id = ?", target.ID).Count(&count)

	return c.JSON(fiber.Map{
		"status": "success", "data": toTargetResponse(*target, count)})
}

// GetTargetAssets لیست دارایی‌ها رو با فیلتر، صفحه‌بندی، کشینگ و مرتب‌سازی برمی‌گردونه
func GetTargetAssets(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	// Access control: target باید متعلق به کاربر باشد (مگر admin)
	if _, err := getAccessibleTarget(c, targetID); err != nil {
		return err
	}

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
	hasPorts := c.Query("has_ports")
	noCdn := c.Query("no_cdn")
	hasCdn := c.Query("has_cdn")
	hasWaf := c.Query("has_waf")
	hasCloud := c.Query("has_cloud")
	statusCodeFilter := strings.TrimSpace(c.Query("status_code")) // e.g. "200" or "2xx"

	// 👇 Providers filter (array or comma-separated): sources=subfinder&sources=crtsh OR sources=subfinder,crtsh
	var sources []string
	// Try multi query params first
	if c.Context() != nil && c.Context().QueryArgs() != nil {
		if multi := c.Context().QueryArgs().PeekMulti("sources"); len(multi) > 0 {
			for _, v := range multi {
				s := strings.TrimSpace(string(v))
				if s != "" {
					sources = append(sources, s)
				}
			}
		}
	}
	// Fallback: comma-separated
	if len(sources) == 0 {
		if raw := strings.TrimSpace(c.Query("sources")); raw != "" {
			for _, part := range strings.Split(raw, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					sources = append(sources, part)
				}
			}
		}
	}
	// normalize + dedupe
	if len(sources) > 0 {
		seen := make(map[string]bool)
		var cleaned []string
		for _, s := range sources {
			s = strings.ToLower(strings.TrimSpace(s))
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			cleaned = append(cleaned, s)
		}
		sort.Strings(cleaned)
		sources = cleaned
	}

	// Sorting Params
	sortBy := c.Query("sort_by", "value")
	order := c.Query("order", "asc")

	validSortFields := map[string]bool{
		"value": true, "status_code": true, "content_length": true, "title": true, "created_at": true}
	if !validSortFields[sortBy] {
		sortBy = "value"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, order)

	sourcesKey := ""
	if len(sources) > 0 {
		sourcesKey = strings.Join(sources, ",")
	}
	filtersKey := fmt.Sprintf("l:%s|n:%s|s:%s|h:%s|d:%s|p:%s|no_cdn:%s|cdn:%s|waf:%s|cloud:%s|sc:%s|src:%s|sb:%s|o:%s", isLive, isNew, search, hasHttpx, dnsOnly, hasPorts, noCdn, hasCdn, hasWaf, hasCloud, statusCodeFilter, sourcesKey, sortBy, order)
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

	// Status code filter:
	// - exact: status_code=200
	// - class: status_code=2xx (or 3xx/4xx/5xx)
	if statusCodeFilter != "" {
		raw := strings.ToLower(statusCodeFilter)
		if len(raw) == 3 && raw[1:] == "xx" && raw[0] >= '1' && raw[0] <= '5' {
			start := int(raw[0]-'0') * 100
			db = db.Where("status_code >= ? AND status_code < ?", start, start+100)
		} else {
			if n, err := strconv.Atoi(raw); err == nil {
				db = db.Where("status_code = ?", n)
			}
		}
	}

	// Providers (sources) filter: match ANY selected provider (OR)
	// sources is stored as jsonb array (e.g. ["subfinder","crtsh"])
	if len(sources) > 0 {
		var conds []string
		var args []interface{}
		for _, s := range sources {
			b, _ := json.Marshal([]string{s})
			conds = append(conds, "sources @> ?::jsonb")
			args = append(args, string(b))
		}
		db = db.Where("("+strings.Join(conds, " OR ")+")", args...)
	}

	if hasHttpx == "true" {
		db = db.Where("jsonb_array_length(host_ip) > 0")
	}

	if dnsOnly == "true" {
		db = db.Where("jsonb_array_length(dnsx_ip) > 0 AND jsonb_array_length(host_ip) = 0")
	}

	if hasPorts == "true" {
		// open_ports: jsonb object per IP, default '{}'.
		// "Has ports" یعنی حداقل برای یکی از IPها آرایه پورت‌ها خالی نباشه.
		db = db.Where(`
			jsonb_typeof(open_ports) = 'object'
			AND EXISTS (
				SELECT 1
				FROM jsonb_each(open_ports) e
				WHERE jsonb_typeof(e.value) = 'array'
				  AND jsonb_array_length(e.value) > 0
			)
		`)
	}

	if hasCdn == "true" {
		db = db.Where("( (cdn_name IS NOT NULL AND cdn_name != '' AND cdn_name != 'null') OR cdncheck = true OR (cdncheck_name IS NOT NULL AND cdncheck_name != '' AND cdncheck_name != 'null') )")
	}
	if hasWaf == "true" {
		db = db.Where("( wafcheck = true OR (wafcheck_name IS NOT NULL AND wafcheck_name != '' AND wafcheck_name != 'null') )")
	}
	if hasCloud == "true" {
		db = db.Where("( cloudcheck = true OR (cloudcheck_name IS NOT NULL AND cloudcheck_name != '' AND cloudcheck_name != 'null') )")
	}

	if noCdn == "true" {
		// No CDN یعنی هم httpx CDN تشخیص نداده، هم cdncheck چیزی پیدا نکرده
		db = db.
			Where("(cdn_name IS NULL OR cdn_name = '' OR cdn_name = 'null')").
			Where("cdncheck = ?", false).
			Where("(cdncheck_name IS NULL OR cdncheck_name = '' OR cdncheck_name = 'null')")
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
		"status": "success", "data": assetResponses, "page_count": len(assetResponses), "total_count": totalCount}

	cache.SetCache(cacheKey, response)
	c.Set("X-Cache", "MISS")
	return c.JSON(response)
}

// StartProbing هندلر شروع دستی فاز ۲
func StartProbing(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	if target.Status == "SCANNING" || target.Status == "QUEUED" {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "Target is already being scanned or queued."})
	}

	database.DB.Model(target).Updates(map[string]interface{}{
		"status": "QUEUED", "current_phase": "QUEUED: STARTING PHASE 2...", "stop_requested": false})

	taskPayload := fmt.Sprintf("PROBING:%d:%s", target.ID, target.RootDomain)
	if err := redisq.Client.RPush(redisq.Ctx, redisq.QueueNameForUser(target.CreatedByUserID), taskPayload).Err(); err != nil {
		database.DB.Model(target).Update("status", "READY")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to enqueue job"})
	}

	return c.JSON(fiber.Map{
		"status": "success", "message": fmt.Sprintf("Phase 2 started for target: %s", target.Name)})
}

// DeleteTarget حذف کامل تارگت
func DeleteTarget(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}
	admin := isAdmin(c)

	err = database.DB.Transaction(func(tx *gorm.DB) error {
		var target models.Target
		q := tx.Model(&models.Target{})
		if !admin {
			q = q.Where("created_by_user_id = ?", uid)
		}
		if err := q.First(&target, targetID).Error; err != nil {
			return err
		}

		if err := tx.Exec("DELETE FROM asset_histories WHERE asset_id IN (SELECT id FROM assets WHERE target_id = ?)", targetID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM assets WHERE target_id = ?", targetID).Error; err != nil {
			return err
		}
		if err := tx.Exec("DELETE FROM found_urls WHERE target_id = ?", targetID).Error; err != nil {
			return err
		}

		if err := tx.Unscoped().Delete(&target).Error; err != nil {
			return err
		}
		return nil
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error", "message": "Failed to delete target (DB Error)", "details": err.Error()})
	}

	return c.JSON(fiber.Map{
		"status": "success", "message": "Target and all associated data deleted successfully"})
}

// StopScan درخواست توقف اسکن
func StopScan(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	_, _ = fmt.Sscanf(id, "%d", &targetID)
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}
	if err := database.DB.Model(target).Update("stop_requested", true).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to stop scan"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Stop signal sent. Scan will halt shortly."})
}

// GetTargetURLs لیست URLهای کراول شده را برمی‌گرداند
func GetTargetURLs(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	// Access control: target باید متعلق به کاربر باشد (مگر admin)
	if _, err := getAccessibleTarget(c, targetID); err != nil {
		return err
	}

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
		"value": true, "source": true, "created_at": true}
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
		sourceList := strings.Split(sources, ",")
		if len(sourceList) > 0 {
			var conditions []string
			var args []interface{}
			for _, s := range sourceList {
				conditions = append(conditions, "source LIKE ?")
				args = append(args, "%"+s+"%")
			}
			db = db.Where(strings.Join(conditions, " OR "), args...)
		}
	}

	var totalCount int64
	db.Count(&totalCount)

	var urls []models.FoundURL
	db.Order(orderClause).Limit(limit).Offset(offset).Find(&urls)

	response := make([]dto.FoundURLResponse, len(urls))
	for i, u := range urls {
		response[i] = dto.FoundURLResponse{
			ID: u.ID, Value: u.Value, Source: u.Source, CreatedAt: u.CreatedAt}
	}

	return c.JSON(fiber.Map{
		"status": "success", "data": response, "total_count": totalCount, "page": (offset / limit) + 1})
}

// ... (Helper Functions DTO - اینا رو بذار همونطور که هستن بمونن یا اگه خواستی کپی کن از فایل قبلی)

func targetOwnerUsername(t models.Target) string {
	if strings.TrimSpace(t.CreatedByUser.Username) != "" {
		return t.CreatedByUser.Username
	}

	if t.CreatedByUserID == 0 {
		return ""
	}

	var user models.User
	if err := database.DB.Select("username").First(&user, t.CreatedByUserID).Error; err != nil {
		return ""
	}

	return user.Username
}

func toTargetResponse(t models.Target, assetCount int64) dto.TargetResponse {
	return dto.TargetResponse{
		OwnerUsername: targetOwnerUsername(t), CreatedByUserID: t.CreatedByUserID, ID: t.ID, Name: t.Name, RootDomain: t.RootDomain, Description: t.Description, InScope: t.InScope, CreatedAt: t.CreatedAt, AssetCount: assetCount, Frequency: t.Frequency, LastScanAt: t.LastScanAt, Status: t.Status, CurrentPhase: t.CurrentPhase, UseAlterx: t.UseAlterx, UseWaymore: t.UseWaymore, UsePortscan: t.UsePortscan, UseCero: t.UseCero, UseCrtsh: t.UseCrtsh, UsePuredns: t.UsePuredns, UseAbusedb: t.UseAbusedb,
		UseAmass: t.UseAmass, UseNuclei: t.UseNuclei, NucleiProfile: t.NucleiProfile, PurednsWordlists: parseJSONToInterface(t.PurednsWordlists), ScanModules: t.ScanModules}
}

// parseJSONToInterface پارس کردن JSON string به interface{}
func parseJSONToInterface(jsonStr string) interface{} {
	if jsonStr == "" || jsonStr == "[]" || jsonStr == "null" {
		return []string{}
	}
	var result interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return []string{}
	}
	return result
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
	var openPorts interface{}
	if a.OpenPorts != "" && a.OpenPorts != "{}" && a.OpenPorts != "null" {
		_ = json.Unmarshal([]byte(a.OpenPorts), &openPorts)
	}
	var sources interface{}
	if a.Sources != "" && a.Sources != "[]" && a.Sources != "null" {
		_ = json.Unmarshal([]byte(a.Sources), &sources)
	} else {
		sources = []string{} // پیش‌فرض: آرایه خالی
	}

	return dto.AssetResponse{
		ID: a.ID, Value: a.Value, Type: a.Type, IsNew: a.IsNew, IsLive: a.IsLive, CreatedAt: a.CreatedAt, FinalURL: a.FinalURL, StatusCode: a.StatusCode, Title: a.Title, ContentLength: a.ContentLength, HostIP: a.HostIP, DnsxIP: a.DnsxIP, WebServer: a.WebServer, CDNName: a.CDNName, Cdncheck: a.Cdncheck, CdncheckName: a.CdncheckName, Wafcheck: a.Wafcheck, WafcheckName: a.WafcheckName, Cloudcheck: a.Cloudcheck, CloudcheckName: a.CloudcheckName, Technologies: techs, ResponseTime: a.ResponseTimeMs, OpenPorts: openPorts, RawHttpx: rawHttpx, Sources: sources}
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
	scopedDB, _, err := scopedTargetsDB(c)
	if err != nil {
		return err
	}

	if len(req.TargetIDs) > 0 {
		// Export تارگت‌های انتخاب شده
		err = scopedDB.Where("id IN ?", req.TargetIDs).Find(&targets).Error
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error", "message": "Failed to fetch targets", "error": err.Error()})
		}
		if len(targets) == 0 {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "error", "message": "No targets found"})
		}
		// اگر بخشی از IDها قابل دسترسی نبودند، به صورت 404 برگردون
		if len(targets) != len(req.TargetIDs) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"status": "error", "message": "Some targets not found"})
		}
	} else {
		// Export همه تارگت‌ها
		err = scopedDB.Find(&targets).Error
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"status": "error", "message": "Failed to fetch targets", "error": err.Error()})
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
				Value: a.Value, Type: a.Type, IsNew: a.IsNew, IsLive: a.IsLive, FinalURL: a.FinalURL, StatusCode: a.StatusCode, Title: a.Title, ContentLength: a.ContentLength, HostIP: a.HostIP, DnsxIP: a.DnsxIP, WebServer: a.WebServer, CDNName: a.CDNName, Technologies: a.Technologies, BodyHash: a.BodyHash, HeaderHash: a.HeaderHash, ResponseTimeMs: a.ResponseTimeMs, RawHttpx: a.RawHttpx, OpenPorts: a.OpenPorts, CreatedAt: a.CreatedAt.Format(time.RFC3339)}
		}

		// دریافت URLs
		var urls []models.FoundURL
		database.DB.Where("target_id = ?", t.ID).Find(&urls)
		urlItems := make([]dto.URLExportItem, len(urls))
		for j, u := range urls {
			urlItems[j] = dto.URLExportItem{
				Value: u.Value, Source: u.Source, CreatedAt: u.CreatedAt.Format(time.RFC3339)}
		}

		var wordlists []string
		if t.PurednsWordlists != "" && t.PurednsWordlists != "[]" {
			_ = json.Unmarshal([]byte(t.PurednsWordlists), &wordlists)
		}

		exportItems[i] = dto.TargetExportItem{
			Name: t.Name, RootDomain: t.RootDomain, Description: t.Description, InScope: t.InScope, Frequency: t.Frequency, Modules: modules, UseAlterx: t.UseAlterx, UseWaymore: t.UseWaymore, UsePortscan: t.UsePortscan, UseCero: t.UseCero, UseCrtsh: t.UseCrtsh, UsePuredns: t.UsePuredns, UseAbusedb: t.UseAbusedb,
			UseAmass: t.UseAmass, UseNuclei: t.UseNuclei, NucleiProfile: t.NucleiProfile, PurednsWordlists: wordlists, Assets: assetItems, URLs: urlItems}
	}

	exportData := dto.TargetExportData{
		Version: "1.0", ExportDate: time.Now().Format(time.RFC3339), Targets: exportItems}

	// تنظیم هدر برای دانلود فایل
	c.Set("Content-Type", "application/json")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"targets_export_%s.json\"", time.Now().Format("20060102_150405")))

	return c.JSON(exportData)
}

// ImportTarget هندلر Import تارگت‌ها از فایل JSON
func ImportTarget(c *fiber.Ctx) error {
	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	req := new(dto.ImportTargetRequest)
	if err := c.BodyParser(req); err != nil {
		log.Printf("❌ Import error - BodyParser failed: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error", "message": "Invalid request body", "error": err.Error()})
	}

	// اعتبارسنجی نسخه فرمت
	if req.Data.Version != "1.0" {
		log.Printf("❌ Import error - Unsupported version: %s\n", req.Data.Version)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status": "error", "message": fmt.Sprintf("Unsupported export format version: %s (expected: 1.0)", req.Data.Version)})
	}

	log.Printf("📥 Importing %d target(s)...\n", len(req.Data.Targets))

	var created []models.Target
	var skipped []string
	var errors []string

	for _, item := range req.Data.Targets {
		rootDomain := normalizeRootDomain(item.RootDomain)
		if rootDomain == "" {
			errors = append(errors, "Target root_domain is empty")
			continue
		}

		// Check duplicate only inside the current user's tenant scope.
		var existing models.Target
		lookupErr := database.DB.
			Where("created_by_user_id = ? AND root_domain = ?", uid, rootDomain).
			First(&existing).Error

		if lookupErr == nil {
			if req.SkipExisting {
				skipped = append(skipped, rootDomain)
				continue
			}
			errors = append(errors, fmt.Sprintf("Target with root_domain '%s' already exists for this user", rootDomain))
			continue
		}
		if lookupErr != nil && !stderrors.Is(lookupErr, gorm.ErrRecordNotFound) {
			errors = append(errors, fmt.Sprintf("Failed to check target '%s': %v", rootDomain, lookupErr))
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
			CreatedByUserID: uid, Name: item.Name, RootDomain: rootDomain, Description: item.Description, InScope: item.InScope, Frequency: item.Frequency, ScanModules: string(modulesJSON), Status: "READY", // تارگت‌های import شده به صورت READY هستند (نه SCANNING)
			CurrentPhase: "IDLE", UseAlterx: item.UseAlterx, UseWaymore: item.UseWaymore, UsePortscan: item.UsePortscan, UseCero: item.UseCero, UseCrtsh: item.UseCrtsh, UsePuredns: item.UsePuredns, UseAbusedb: item.UseAbusedb, UseAmass: item.UseAmass}
		// تنظیم PurednsWordlists
		if len(item.PurednsWordlists) > 0 {
			wordlistsJSON, _ := json.Marshal(item.PurednsWordlists)
			target.PurednsWordlists = string(wordlistsJSON)
		} else {
			target.PurednsWordlists = "[]"
		}

		if err := database.DB.Create(&target).Error; err != nil {
			if isDuplicateTargetError(err) {
				errors = append(errors, fmt.Sprintf("Target with root_domain '%s' already exists for this user", rootDomain))
			} else {
				errors = append(errors, fmt.Sprintf("Failed to create target '%s': %v", rootDomain, err))
			}
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
					TargetID: target.ID, Value: aItem.Value, Type: aItem.Type, IsNew: aItem.IsNew, IsLive: aItem.IsLive, FinalURL: aItem.FinalURL, StatusCode: aItem.StatusCode, Title: aItem.Title, ContentLength: aItem.ContentLength, HostIP: aItem.HostIP, DnsxIP: aItem.DnsxIP, WebServer: aItem.WebServer, CDNName: aItem.CDNName, Technologies: aItem.Technologies, BodyHash: aItem.BodyHash, HeaderHash: aItem.HeaderHash, ResponseTimeMs: aItem.ResponseTimeMs, RawHttpx: aItem.RawHttpx, OpenPorts: aItem.OpenPorts, CreatedAt: createdAt})
			}
			// استفاده از CreateInBatches با ON CONFLICT برای جلوگیری از duplicate
			if len(assets) > 0 {
				// استفاده از ON CONFLICT DO NOTHING برای skip کردن duplicate ها
				if err := database.DB.Clauses(clause.OnConflict{
					Columns: []clause.Column{
						{Name: "target_id"}, {Name: "value"}}, DoNothing: true}).CreateInBatches(assets, 500).Error; err != nil {
					log.Printf("⚠️ Warning: Failed to import some assets for target '%s': %v\n", rootDomain, err)
					errors = append(errors, fmt.Sprintf("Failed to import assets for '%s': %v", rootDomain, err))
				} else {
					log.Printf("✅ Imported %d assets for target '%s'\n", len(assets), rootDomain)
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
					TargetID: target.ID, Value: uItem.Value, Source: uItem.Source, CreatedAt: createdAt})
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
						log.Printf("⚠️ Warning: Failed to import some URLs for target '%s': %v\n", rootDomain, err)
						errors = append(errors, fmt.Sprintf("Failed to import URLs for '%s': %v", rootDomain, err))
					} else {
						log.Printf("✅ Imported %d URLs for target '%s' (%d skipped as duplicates)\n", len(newURLs), rootDomain, len(urls)-len(newURLs))
					}
				} else if len(urls) > 0 {
					log.Printf("ℹ️ All URLs for target '%s' already exist, skipping\n", rootDomain)
				}
			}
		}

		created = append(created, target)
	}

	// پاک کردن کش
	cache.ClearCache()

	return c.JSON(fiber.Map{
		"status": "success", "message": fmt.Sprintf("Import completed: %d created, %d skipped, %d errors", len(created), len(skipped), len(errors)), "data": fiber.Map{
			"created": created, "skipped": skipped, "errors": errors}})
}

// ExportTargetIPs هندلر Export تمام IPهای منحصر به فرد یک تارگت به صورت فایل txt
func ExportTargetIPs(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	// دریافت اطلاعات تارگت برای root_domain
	target, err := getAccessibleTarget(c, targetID)
	if err != nil {
		return err
	}

	// دریافت assets این تارگت (فقط اونهایی که پشت CDN نیستند)
	var assets []models.Asset
	// معیار "پشت CDN بودن":
	// - اگر cdn_name مقدار داشته باشد (به‌جز '' و 'null') یعنی httpx CDN را تشخیص داده
	// - یا اگر cdncheck=true / cdncheck_name پر باشد یعنی ابزار cdncheck CDN را تشخیص داده
	if err := database.DB.
		Where("target_id = ?", targetID).
		Where("(cdn_name IS NULL OR cdn_name = '' OR cdn_name = 'null')").
		Where("cdncheck = ?", false).
		Find(&assets).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status": "error", "message": "Failed to fetch assets", "error": err.Error()})
	}

	// جمع‌آوری تمام IPهای منحصر به فرد
	ipSet := make(map[string]bool)

	for _, asset := range assets {
		// فیلتر ایمن اضافه: اگر به هر دلیلی asset پشت CDN بود، IPهاش رو اکسپورت نکن
		cdnName := strings.TrimSpace(strings.ToLower(asset.CDNName))
		if (cdnName != "" && cdnName != "null") || asset.Cdncheck || strings.TrimSpace(asset.CdncheckName) != "" {
			continue
		}

		// IPهای DNSX
		if asset.DnsxIP != "" && asset.DnsxIP != "[]" {
			var dnsxIPs []string
			if err := json.Unmarshal([]byte(asset.DnsxIP), &dnsxIPs); err == nil {
				for _, ip := range dnsxIPs {
					if ip != "" && strings.TrimSpace(ip) != "" {
						ipSet[strings.TrimSpace(ip)] = true
					}
				}
			}
		}

		// IPهای HTTPX
		if asset.HostIP != "" && asset.HostIP != "[]" {
			var httpxIPs []string
			if err := json.Unmarshal([]byte(asset.HostIP), &httpxIPs); err == nil {
				for _, ip := range httpxIPs {
					if ip != "" && strings.TrimSpace(ip) != "" {
						ipSet[strings.TrimSpace(ip)] = true
					}
				}
			}
		}
	}

	// تبدیل set به slice و مرتب‌سازی
	ipList := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ipList = append(ipList, ip)
	}
	sort.Strings(ipList)

	// ساخت محتوای فایل (هر IP در یک خط)
	content := strings.Join(ipList, "\n")

	// تنظیم هدر برای دانلود فایل txt
	c.Set("Content-Type", "text/plain; charset=utf-8")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s_ips.txt\"", strings.ReplaceAll(target.RootDomain, ".", "_")))

	return c.SendString(content)
}

// GetWordlists لیست وردلیست‌های موجود را برمی‌گرداند
func GetWordlists(c *fiber.Ctx) error {
	wordlists := []map[string]string{}

	// وردلیست‌های پیش‌فرض در /wordlists
	defaultWordlists := []string{
		"/wordlists/common.txt", "/wordlists/params.txt"}

	// وردلیست‌های سفارشی در /wordlists/custom
	customWordlistsDir := "/wordlists/custom"

	// بررسی وردلیست‌های پیش‌فرض
	for _, wl := range defaultWordlists {
		if _, err := os.Stat(wl); err == nil {
			wordlists = append(wordlists, map[string]string{
				"path": wl, "name": filepath.Base(wl), "type": "default"})
		}
	}

	// بررسی وردلیست‌های سفارشی
	if entries, err := os.ReadDir(customWordlistsDir); err == nil {
		for _, entry := range entries {
			if !entry.IsDir() {
				fullPath := filepath.Join(customWordlistsDir, entry.Name())
				wordlists = append(wordlists, map[string]string{
					"path": fullPath, "name": entry.Name(), "type": "custom"})
			}
		}
	}

	return c.JSON(fiber.Map{
		"status": "success", "data": wordlists})
}

// DownloadTargetAssets downloads the filtered assets as a text file
func DownloadTargetAssets(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	// Access control
	if _, err := getAccessibleTarget(c, targetID); err != nil {
		return err
	}

	// Filters
	isLive := c.Query("is_live")
	isNew := c.Query("is_new")
	search := c.Query("search")
	hasHttpx := c.Query("has_httpx")
	dnsOnly := c.Query("dns_only")
	hasPorts := c.Query("has_ports")
	noCdn := c.Query("no_cdn")
	hasCdn := c.Query("has_cdn")
	hasWaf := c.Query("has_waf")
	hasCloud := c.Query("has_cloud")
	statusCodeFilter := strings.TrimSpace(c.Query("status_code"))

	var sources []string
	if c.Context() != nil && c.Context().QueryArgs() != nil {
		if multi := c.Context().QueryArgs().PeekMulti("sources"); len(multi) > 0 {
			for _, v := range multi {
				s := strings.TrimSpace(string(v))
				if s != "" {
					sources = append(sources, s)
				}
			}
		}
	}
	if len(sources) == 0 {
		if raw := strings.TrimSpace(c.Query("sources")); raw != "" {
			for _, part := range strings.Split(raw, ",") {
				part = strings.TrimSpace(part)
				if part != "" {
					sources = append(sources, part)
				}
			}
		}
	}

	db := database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID)

	if isLive != "" {
		db = db.Where("is_live = ?", isLive == "true")
	}
	if isNew != "" {
		db = db.Where("is_new = ?", isNew == "true")
	}
	if search != "" {
		db = db.Where("value LIKE ?", "%"+search+"%")
	}

	if statusCodeFilter != "" {
		raw := strings.ToLower(statusCodeFilter)
		if len(raw) == 3 && raw[1:] == "xx" && raw[0] >= '1' && raw[0] <= '5' {
			start := int(raw[0]-'0') * 100
			db = db.Where("status_code >= ? AND status_code < ?", start, start+100)
		} else {
			if n, err := strconv.Atoi(raw); err == nil {
				db = db.Where("status_code = ?", n)
			}
		}
	}

	if len(sources) > 0 {
		var conds []string
		var args []interface{}
		for _, s := range sources {
			b, _ := json.Marshal([]string{s})
			conds = append(conds, "sources @> ?::jsonb")
			args = append(args, string(b))
		}
		db = db.Where("("+strings.Join(conds, " OR ")+")", args...)
	}

	if hasHttpx == "true" {
		db = db.Where("jsonb_array_length(host_ip) > 0")
	}

	if dnsOnly == "true" {
		db = db.Where("jsonb_array_length(dnsx_ip) > 0 AND jsonb_array_length(host_ip) = 0")
	}

	if hasPorts == "true" {
		db = db.Where(`
jsonb_typeof(open_ports) = 'object'
AND EXISTS (
SELECT 1
FROM jsonb_each(open_ports) e
WHERE jsonb_typeof(e.value) = 'array'
  AND jsonb_array_length(e.value) > 0
			)
		`)
	}

	if hasCdn == "true" {
		db = db.Where("( (cdn_name IS NOT NULL AND cdn_name != '' AND cdn_name != 'null') OR cdncheck = true OR (cdncheck_name IS NOT NULL AND cdncheck_name != '' AND cdncheck_name != 'null') )")
	}
	if hasWaf == "true" {
		db = db.Where("( wafcheck = true OR (wafcheck_name IS NOT NULL AND wafcheck_name != '' AND wafcheck_name != 'null') )")
	}
	if hasCloud == "true" {
		db = db.Where("( cloudcheck = true OR (cloudcheck_name IS NOT NULL AND cloudcheck_name != '' AND cloudcheck_name != 'null') )")
	}

	if noCdn == "true" {
		db = db.
			Where("(cdn_name IS NULL OR cdn_name = '' OR cdn_name = 'null')").
			Where("cdncheck = ?", false).
			Where("(cdncheck_name IS NULL OR cdncheck_name = '' OR cdncheck_name = 'null')")
	}

	// Sorting
	sortBy := c.Query("sort_by", "value")
	order := c.Query("order", "asc")
	validSortFields := map[string]bool{
		"value": true, "status_code": true, "content_length": true, "title": true, "created_at": true}
	if !validSortFields[sortBy] {
		sortBy = "value"
	}
	if order != "asc" && order != "desc" {
		order = "asc"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, order)

	var results []string
	if err := db.Order(orderClause).Pluck("value", &results).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	content := strings.Join(results, "\n")
	c.Set("Content-Type", "text/plain")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"assets_%d.txt\"", targetID))
	return c.SendString(content)
}

// DownloadTargetURLs downloads the filtered URLs as a text file
func DownloadTargetURLs(c *fiber.Ctx) error {
	id := c.Params("id")
	targetID := uint(0)
	fmt.Sscanf(id, "%d", &targetID)

	if _, err := getAccessibleTarget(c, targetID); err != nil {
		return err
	}

	search := c.Query("search")
	onlyJS := c.Query("only_js")
	sources := c.Query("sources")

	db := database.DB.Model(&models.FoundURL{}).Where("target_id = ?", targetID)

	if search != "" {
		db = db.Where("value LIKE ?", "%"+search+"%")
	}

	if onlyJS == "true" {
		db = db.Where("value LIKE ?", "%.js")
	}

	if sources != "" {
		sourceList := strings.Split(sources, ",")
		if len(sourceList) > 0 {
			var conditions []string
			var args []interface{}
			for _, s := range sourceList {
				conditions = append(conditions, "source LIKE ?")
				args = append(args, "%"+s+"%")
			}
			db = db.Where(strings.Join(conditions, " OR "), args...)
		}
	}

	sortBy := c.Query("sort_by", "created_at")
	order := c.Query("order", "desc")
	validSortFields := map[string]bool{
		"value": true, "source": true, "created_at": true}
	if !validSortFields[sortBy] {
		sortBy = "created_at"
	}
	if order != "asc" && order != "desc" {
		order = "desc"
	}
	orderClause := fmt.Sprintf("%s %s", sortBy, order)

	var results []string
	if err := db.Order(orderClause).Pluck("value", &results).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	content := strings.Join(results, "\n")
	c.Set("Content-Type", "text/plain")
	c.Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"urls_%d.txt\"", targetID))
	return c.SendString(content)
}
