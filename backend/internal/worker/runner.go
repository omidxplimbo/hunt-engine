package worker

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
	"gorm.io/gorm"
)

// مسیرهای فایل‌های موقت
const (
	allFoundFile      = "/tmp/all_found.txt"
	dnsxOutputFile    = "/tmp/dnsx_output.json"
	probingInputFile  = "/tmp/probing_input.txt"
	probingOutputFile = "/tmp/probing_output.json"
)

const DefaultBatchSize = 500

// HttpxResult ساختار برای پارس کردن خروجی JSON
type HttpxResult struct {
	Input         string   `json:"input"`
	URL           string   `json:"url"`
	StatusCode    int      `json:"status_code"`
	Title         string   `json:"title"`
	ContentLength int64    `json:"content_length"`
	A             []string `json:"a"`
	WebServer     string   `json:"webserver"`
	CDNName       string   `json:"cdn_name"`
	Technologies  []string `json:"tech"`
	BodyHash      string   `json:"body_hash"`
	HeaderHash    string   `json:"header_hash"`
	ResponseTime  string   `json:"time"`
	RawJSON       string   `json:"-"`
}

// DnsxResult ساختار خروجی JSON ابزار dnsx
type DnsxResult struct {
	Host string   `json:"host"`
	A    []string `json:"a"`
}

func Start() {
	log.Println("👷 Worker started. Waiting for jobs...", redisq.QueueName)

	for {
		result, err := redisq.Client.BLPop(context.Background(), 0*time.Second, redisq.QueueName).Result()
		if err != nil {
			log.Printf("❌ Error popping from Redis: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		go processJobDispatcher(result[1])
	}
}

func processJobDispatcher(payload string) {
	parts := strings.Split(payload, ":")
	if len(parts) < 3 {
		log.Printf("⚠️ Invalid job payload format: %s\n", payload)
		return
	}

	jobType := parts[0]
	targetIDStr := parts[1]
	rootDomain := parts[2]

	var targetID uint
	fmt.Sscanf(targetIDStr, "%d", &targetID)

	log.Printf("👷 Worker received job type: %s for target ID: %d\n", jobType, targetID)

	switch jobType {
	case "DISCOVERY":
		runDiscoveryPhase(targetID, rootDomain)
	case "PROBING":
		runProbingPhase(targetID, rootDomain)
	default:
		log.Printf("⚠️ Unknown job type: %s\n", jobType)
	}
}

// تابع کمکی برای آپدیت سریع وضعیت در دیتابیس (برای نمایش در فرانت)
func updateTargetPhase(targetID uint, phase string) {
	database.DB.Model(&models.Target{}).Where("id = ?", targetID).Update("current_phase", phase)
}

// =================================================================
// PHASE 1: Discovery Implementation
// =================================================================
func runDiscoveryPhase(targetID uint, rootDomain string) {
	log.Printf("🚀 Starting PHASE 1 (DISCOVERY) for: %s (ID: %d)\n", rootDomain, targetID)
	updateTargetPhase(targetID, "PHASE 1: STARTED") // 👈 آپدیت وضعیت
	startTime := time.Now()

	// 1. Passive
	updateTargetPhase(targetID, "PHASE 1: PASSIVE ENUM") // 👈 آپدیت وضعیت
	passiveResults := runPassiveCollection(rootDomain)

	// 2. Mutation
	updateTargetPhase(targetID, "PHASE 1: MUTATION") // 👈 آپدیت وضعیت
	writeSliceToFile(allFoundFile, passiveResults)
	mutatedResults, err := runAlterx(allFoundFile, rootDomain)
	if err != nil {
		log.Printf("❌ Alterx failed: %v.\n", err)
		mutatedResults = []string{}
	}

	// 3. History Injection
	var existingAssets []string
	database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID).Pluck("value", &existingAssets)

	// 4. Merge
	masterList := mergeUnique(passiveResults, mutatedResults)
	masterList = mergeUnique(masterList, existingAssets)
	writeSliceToFile(allFoundFile, masterList)
	log.Printf("📊 Master list created with %d potential subdomains. Starting validation (DNSX)...\n", len(masterList))

	// 5. Validation & IP Resolution (DNSX)
	updateTargetPhase(targetID, "PHASE 1: DNSX VALIDATION") // 👈 آپدیت وضعیت
	dnsxResults, err := runDnsx(allFoundFile, dnsxOutputFile)
	if err != nil {
		log.Printf("❌ Dnsx failed: %v\n", err)
		return
	}
	log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains with IPs.\n", len(dnsxResults))

	// 6. Saving
	updateTargetPhase(targetID, "PHASE 1: SAVING RESULTS") // 👈 آپدیت وضعیت
	var target models.Target
	database.DB.First(&target, targetID)
	saveDiscoveryResultsToDB(target, masterList, dnsxResults)

	log.Printf("🏁 PHASE 1 finished for %s in %s.\n", rootDomain, time.Since(startTime))
	triggerNextModule(targetID, rootDomain, "DISCOVERY")
}

// =================================================================
// PHASE 2: Probing Implementation
// =================================================================
func runProbingPhase(targetID uint, rootDomain string) {
	batchSize := getBatchSize()
	log.Printf("🚀 Starting PHASE 2 (PROBING) for target ID: %d (Batch Size: %d)\n", targetID, batchSize)
	updateTargetPhase(targetID, "PHASE 2: INITIALIZING") // 👈 آپدیت وضعیت
	startTime := time.Now()

	var totalLive int64
	database.DB.Model(&models.Asset{}).Where("target_id = ? AND is_live = true", targetID).Count(&totalLive)

	if totalLive == 0 {
		log.Println("⚠️ No live assets found. Aborting probing.")
		triggerNextModule(targetID, rootDomain, "PROBING")
		return
	}
	log.Printf("✅ Found %d live assets in DB. Starting batch processing...\n", totalLive)

	processedCount := 0
	offset := 0

	// محاسبه تعداد کل بچ‌ها برای نمایش
	totalBatches := int(totalLive)/batchSize + 1
	currentBatch := 1

	for {
		// گزارش پیشرفت دقیق
		statusMsg := fmt.Sprintf("PHASE 2: BATCH %d/%d", currentBatch, totalBatches)
		updateTargetPhase(targetID, statusMsg) // 👈 آپدیت وضعیت

		var batchAssets []models.Asset
		err := database.DB.Where("target_id = ? AND is_live = true", targetID).Order("id").Limit(batchSize).Offset(offset).Find(&batchAssets).Error
		if err != nil || len(batchAssets) == 0 {
			break
		}

		log.Printf("🔄 Processing batch: Offset %d, Size %d...\n", offset, len(batchAssets))
		var hosts []string
		for _, asset := range batchAssets {
			hosts = append(hosts, asset.Value)
		}
		if err := writeSliceToFile(probingInputFile, hosts); err != nil {
			break
		}

		cmd := exec.Command("httpx", "-l", probingInputFile, "-json", "-o", probingOutputFile, "-silent", "-threads", "50", "-follow-redirects")
		cmd.Run()

		httpxResults, err := readHttpxResults(probingOutputFile)
		if err == nil {
			updateAssetsWithDiff(targetID, httpxResults)
			processedCount += len(batchAssets)
		}
		offset += batchSize
		currentBatch++
	}

	log.Printf("🏁 PHASE 2 finished. Total processed: %d in %s.\n", processedCount, time.Since(startTime))
	triggerNextModule(targetID, rootDomain, "PROBING")
}

// ==========================================
// Smart Storage & Notification Logic
// ==========================================

// saveDiscoveryResultsToDB (نسخه اصلاح‌شده: شرط سخت‌گیرانه برای IP و Live)
func saveDiscoveryResultsToDB(target models.Target, masterList []string, liveResults map[string][]string) {
	log.Printf("💾 Saving/Updating assets (Scan Count: %d)...", target.ScanCount)

	isFirstRun := target.ScanCount == 0
	countNew := 0
	countUpdated := 0

	for _, val := range masterList {
		ips, found := liveResults[val]

		// 👇👇👇 فیکس باگ ۱: فقط اگر IP وجود داشت، زنده محسوب می‌شود
		hasIPs := found && len(ips) > 0
		isLive := hasIPs // وضعیت لایو وابسته به داشتن IP است

		// آماده‌سازی JSON برای ذخیره
		dnsxIPJSON := "[]"
		if hasIPs {
			bytes, _ := json.Marshal(ips)
			dnsxIPJSON = string(bytes)
		}

		var existingAsset models.Asset
		result := database.DB.Where("value = ? AND target_id = ?", val, target.ID).First(&existingAsset)

		if result.Error == nil {
			// --- دارایی قدیمی ---
			// چک تغییر وضعیت
			if existingAsset.IsLive != isLive {
				now := time.Now()
				logChange(database.DB, existingAsset.ID, "is_live", strconv.FormatBool(existingAsset.IsLive), strconv.FormatBool(isLive))

				// اگر زنده شد -> خبر بده
				if isLive {
					telegram.SendChangeAlert(target.RootDomain, val, "is_live", "false", "true")
				}

				database.DB.Model(&existingAsset).Updates(map[string]interface{}{
					"is_live":        isLive,
					"is_new":         true,
					"last_change_at": &now,
					"dnsx_ip":        dnsxIPJSON, // آپدیت IP
				})
				countUpdated++
			} else if isLive && existingAsset.DnsxIP != dnsxIPJSON {
				// اگر زنده بود و فقط IP عوض شد -> آپدیت کن
				database.DB.Model(&existingAsset).Update("dnsx_ip", dnsxIPJSON)
			}
		} else {
			// --- دارایی جدید ---
			// پیش‌فرض is_live را false می‌گیریم مگر اینکه شرط بالا true شده باشد
			asset := models.Asset{
				TargetID:     target.ID,
				Value:        val,
				Type:         "subdomain",
				IsNew:        true,
				IsLive:       isLive, // 👈 مقدار صحیح محاسبه شده
				Technologies: "[]",
				HostIP:       "[]",
				RawHttpx:     "{}",
				DnsxIP:       dnsxIPJSON, // 👈 ذخیره IP
			}
			database.DB.Create(&asset)

			// نوتیفیکیشن فقط برای زنده‌های جدید (غیر از بار اول)
			if !isFirstRun && isLive {
				telegram.SendNewAssetAlert(target.RootDomain, val)
			}
			countNew++
		}
	}
	log.Printf("✅ DB Sync Complete. New: %d, Status Changed: %d.\n", countNew, countUpdated)
}

func updateAssetsWithDiff(targetID uint, results map[string]HttpxResult) {
	var targetName string
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err == nil {
		targetName = target.RootDomain
	}

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for hostInput, result := range results {
			var asset models.Asset
			if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("value = ? AND target_id = ?", hostInput, targetID).First(&asset).Error; err != nil {
				continue
			}

			isFirstProbing := asset.RawHttpx == "" || asset.RawHttpx == "{}"
			hasChanged := false
			now := time.Now()

			checkAndNotify := func(field, oldVal, newVal string) error {
				if oldVal != newVal {
					if err := logChange(tx, asset.ID, field, oldVal, newVal); err != nil {
						return err
					}
					if !isFirstProbing {
						telegram.SendChangeAlert(targetName, hostInput, field, oldVal, newVal)
					}
					hasChanged = true
				}
				return nil
			}

			if err := checkAndNotify("status_code", strconv.Itoa(asset.StatusCode), strconv.Itoa(result.StatusCode)); err != nil {
				return err
			}

			oldTitle := shortenString(asset.Title, 50)
			newTitle := shortenString(result.Title, 50)
			if err := checkAndNotify("title", oldTitle, newTitle); err != nil {
				return err
			}

			if err := checkAndNotify("web_server", asset.WebServer, result.WebServer); err != nil {
				return err
			}

			newTechJSON := marshalJSONOrDefault(result.Technologies, "[]")
			newIPsJSON := marshalJSONOrDefault(result.A, "[]")

			if !areJSONArraysEqual(asset.Technologies, newTechJSON) {
				if err := checkAndNotify("technologies", asset.Technologies, newTechJSON); err != nil {
					return err
				}
			}
			if !areJSONArraysEqual(asset.HostIP, newIPsJSON) {
				if err := checkAndNotify("host_ip", asset.HostIP, newIPsJSON); err != nil {
					return err
				}
			}

			if hasChanged {
				asset.LastChangeAt = &now
			}

			asset.FinalURL = result.URL
			asset.StatusCode = result.StatusCode
			asset.Title = result.Title
			asset.ContentLength = result.ContentLength
			asset.HostIP = newIPsJSON
			asset.WebServer = result.WebServer
			asset.CDNName = result.CDNName
			asset.Technologies = newTechJSON
			asset.BodyHash = result.BodyHash
			asset.HeaderHash = result.HeaderHash
			asset.RawHttpx = result.RawJSON
			asset.ResponseTimeMs = parseResponseTime(result.ResponseTime)

			if err := tx.Save(&asset).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log.Printf("❌ Transaction failed: %v\n", err)
	}
}

func triggerNextModule(targetID uint, rootDomain, currentModule string) {
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Update("status", "READY")
		return
	}

	var modules []string
	json.Unmarshal([]byte(target.ScanModules), &modules)

	currentIndex := -1
	for i, m := range modules {
		if m == currentModule {
			currentIndex = i
			break
		}
	}

	if currentIndex != -1 && currentIndex+1 < len(modules) {
		nextModule := modules[currentIndex+1]
		log.Printf("🔗 Chaining: Phase '%s' done. Starting '%s'\n", currentModule, nextModule)
		payload := fmt.Sprintf("%s:%d:%s", nextModule, targetID, rootDomain)
		redisq.Client.RPush(context.Background(), redisq.QueueName, payload)
	} else {
		log.Printf("🏁 Chain Complete for %s.\n", rootDomain)
		now := time.Now()
		// 👇 وضعیت را به READY برمی‌گردانیم و فاز فعلی را IDLE می‌کنیم
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"last_scan_at":  &now,
			"status":        "READY",
			"scan_count":    gorm.Expr("scan_count + 1"),
			"current_phase": "IDLE",
		})
	}
}

// --- Helper Functions ---

func logChange(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error {
	if oldVal == newVal {
		return nil
	}
	history := models.AssetHistory{AssetID: assetID, FieldName: field, OldValue: oldVal, NewValue: newVal, CreatedAt: time.Now()}
	if err := tx.Create(&history).Error; err != nil {
		return err
	}
	return nil
}

func runDnsx(inputFile, outputFile string) (map[string][]string, error) {
	cmd := exec.Command("dnsx", "-l", inputFile, "-json", "-o", outputFile, "-silent", "-a", "-resp", "-threads", "50")
	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ dnsx issue: %v\n", err)
	}

	results := make(map[string][]string)
	file, err := os.Open(outputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return results, nil
		}
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var res DnsxResult
		if err := json.Unmarshal([]byte(scanner.Text()), &res); err == nil && res.Host != "" {
			results[res.Host] = res.A
		}
	}
	return results, scanner.Err()
}

func areJSONArraysEqual(json1, json2 string) bool {
	var arr1, arr2 []string
	_ = json.Unmarshal([]byte(json1), &arr1)
	_ = json.Unmarshal([]byte(json2), &arr2)
	if len(arr1) != len(arr2) {
		return false
	}
	sort.Strings(arr1)
	sort.Strings(arr2)
	return reflect.DeepEqual(arr1, arr2)
}
func marshalJSONOrDefault(v interface{}, defaultVal string) string {
	if reflect.ValueOf(v).Kind() == reflect.Slice && reflect.ValueOf(v).Len() == 0 {
		return defaultVal
	}
	jsonBytes, err := json.Marshal(v)
	if err != nil || string(jsonBytes) == "null" || string(jsonBytes) == "" {
		return defaultVal
	}
	return string(jsonBytes)
}
func parseResponseTime(timeStr string) int64 {
	str := strings.TrimSuffix(timeStr, "ms")
	if val, err := strconv.ParseFloat(str, 64); err == nil {
		return int64(val)
	}
	return 0
}
func shortenString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) > maxLen {
		return string(runes[:maxLen]) + "..."
	}
	return s
}
func getBatchSize() int {
	envStr := os.Getenv("PROBE_BATCH_SIZE")
	if envStr == "" {
		return DefaultBatchSize
	}
	size, err := strconv.Atoi(envStr)
	if err != nil || size <= 0 {
		return DefaultBatchSize
	}
	return size
}
func readHttpxResults(filename string) (map[string]HttpxResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	results := make(map[string]HttpxResult)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var result HttpxResult
		if err := json.Unmarshal([]byte(line), &result); err == nil {
			if result.Input != "" {
				result.RawJSON = line
				results[result.Input] = result
			}
		}
	}
	return results, scanner.Err()
}
func runPassiveCollection(domain string) []string {
	var wg sync.WaitGroup
	results := make(chan string, 50000)
	tools := []struct {
		name string
		cmd  *exec.Cmd
	}{
		{"subfinder", exec.Command("subfinder", "-d", domain, "-silent", "-all")},
		{"assetfinder", exec.Command("assetfinder", "--subs-only", domain)},
	}
	for _, tool := range tools {
		wg.Add(1)
		go func(t struct {
			name string
			cmd  *exec.Cmd
		}) {
			defer wg.Done()
			output, err := t.cmd.CombinedOutput()
			if err == nil {
				for _, line := range strings.Split(string(output), "\n") {
					results <- strings.TrimSpace(line)
				}
			} else {
				log.Printf("❌ %s error: %v\n", t.name, err)
			}
		}(tool)
	}
	go func() { wg.Wait(); close(results) }()
	uniqueMap := make(map[string]bool)
	var finalSlice []string
	for res := range results {
		if res != "" && strings.HasSuffix(res, domain) && !uniqueMap[res] {
			uniqueMap[res] = true
			finalSlice = append(finalSlice, res)
		}
	}
	return finalSlice
}
func runAlterx(inputFile, rootDomain string) ([]string, error) {
	cmd := exec.Command("alterx", "-silent")
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	go func() {
		defer stdin.Close()
		file, _ := os.Open(inputFile)
		defer file.Close()
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			fmt.Fprintln(stdin, scanner.Text())
		}
	}()
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	var results []string
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && strings.HasSuffix(trimmed, rootDomain) {
			results = append(results, trimmed)
		}
	}
	return results, nil
}
func mergeUnique(slice1, slice2 []string) []string {
	uniqueMap := make(map[string]bool)
	for _, v := range slice1 {
		uniqueMap[v] = true
	}
	for _, v := range slice2 {
		uniqueMap[v] = true
	}
	final := make([]string, 0, len(uniqueMap))
	for k := range uniqueMap {
		final = append(final, k)
	}
	return final
}
func writeSliceToFile(filename string, data []string) error {
	content := strings.Join(data, "\n")
	return ioutil.WriteFile(filename, []byte(content), 0644)
}
