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

// Start موتور اصلی کارگر را روشن می‌کند.
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

// =================================================================
// PHASE 1: Discovery Implementation
// =================================================================
func runDiscoveryPhase(targetID uint, rootDomain string) {
	log.Printf("🚀 Starting PHASE 1 (DISCOVERY) for: %s (ID: %d)\n", rootDomain, targetID)
	updateTargetPhase(targetID, "PHASE 1: STARTED")
	startTime := time.Now()

	// 1. Passive
	updateTargetPhase(targetID, "PHASE 1: PASSIVE ENUM")
	passiveResults := runPassiveCollection(rootDomain)
	if checkStopRequest(targetID) {
		return
	} // 👈 چک کردن

	// 2. Mutation
	updateTargetPhase(targetID, "PHASE 1: MUTATION")
	writeSliceToFile(allFoundFile, passiveResults)
	mutatedResults, err := runAlterx(allFoundFile, rootDomain)
	if checkStopRequest(targetID) {
		return
	} // 👈 چک کردن
	if err != nil {
		log.Printf("❌ Alterx failed: %v. Proceeding without mutations.\n", err)
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

	// 5. Validation (DNSX)
	updateTargetPhase(targetID, "PHASE 1: DNSX VALIDATION")
	// استفاده از dnsx برای گرفتن همزمان زنده‌ها و IPها
	dnsxResults, err := runDnsx(allFoundFile)
	if err != nil {
		log.Printf("❌ Dnsx failed: %v\n", err)
		return
	}
	log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains with IPs.\n", len(dnsxResults))

	// 6. Saving (Optimized)
	updateTargetPhase(targetID, "PHASE 1: SAVING RESULTS")
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
	updateTargetPhase(targetID, "PHASE 2: INITIALIZING")
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
	totalBatches := int(totalLive)/batchSize + 1
	currentBatch := 1

	for {
		if checkStopRequest(targetID) {
			return // خروج اضطراری
		}
		statusMsg := fmt.Sprintf("PHASE 2: BATCH %d/%d", currentBatch, totalBatches)
		updateTargetPhase(targetID, statusMsg)

		// خواندن دسته از دیتابیس
		var batchAssets []models.Asset
		err := database.DB.Where("target_id = ? AND is_live = true", targetID).
			Order("id").
			Limit(batchSize).
			Offset(offset).
			Find(&batchAssets).Error

		if err != nil || len(batchAssets) == 0 {
			break
		}

		log.Printf("🔄 Processing batch: Offset %d, Size %d...\n", offset, len(batchAssets))

		var hosts []string
		for _, asset := range batchAssets {
			hosts = append(hosts, asset.Value)
		}
		if err := writeSliceToFile(probingInputFile, hosts); err != nil {
			log.Printf("❌ Error writing batch input file: %v\n", err)
			break
		}

		cmd := exec.Command("httpx",
			"-l", probingInputFile,
			"-json",
			"-o", probingOutputFile,
			"-silent",
			"-threads", "50",
			"-follow-redirects",
		)
		if err := cmd.Run(); err != nil {
			log.Printf("⚠️ Httpx batch finished with issues: %v\n", err)
		}

		httpxResults, err := readHttpxResults(probingOutputFile)
		if err != nil {
			log.Printf("❌ Error reading batch httpx output: %v\n", err)
		} else {
			// 👇👇👇 بهینه‌سازی: پاس دادن batchAssets به تابع آپدیت
			// این کار باعث میشه دیگه نیازی به SELECT دوباره نباشه
			updateAssetsWithDiff(targetID, batchAssets, httpxResults)
			processedCount += len(batchAssets)
		}

		offset += batchSize
		currentBatch++
	}

	log.Printf("🏁 PHASE 2 finished. Total processed: %d in %s.\n", processedCount, time.Since(startTime))
	triggerNextModule(targetID, rootDomain, "PROBING")
}

// ==========================================
// Smart Storage & Notification Logic (Optimized)
// ==========================================

// saveDiscoveryResultsToDB (نسخه بهینه شده با Batch Fetching)
func saveDiscoveryResultsToDB(target models.Target, masterList []string, liveResults map[string][]string) {
	log.Printf("💾 Saving/Updating assets (Scan Count: %d)...", target.ScanCount)

	isFirstRun := target.ScanCount == 0
	countNew := 0
	countUpdated := 0

	// تبدیل masterList به دسته‌های ۱۰۰۰ تایی برای کاهش فشار به رم و دیتابیس
	chunkSize := 1000
	for i := 0; i < len(masterList); i += chunkSize {
		end := i + chunkSize
		if end > len(masterList) {
			end = len(masterList)
		}
		chunk := masterList[i:end]

		// 1. واکشی تمام رکوردهای موجود برای این دسته (Batch Fetch)
		// به جای ۱۰۰۰ تا سلکت تکی، یک سلکت با IN می‌زنیم
		var existingAssets []models.Asset
		database.DB.Where("target_id = ? AND value IN ?", target.ID, chunk).Find(&existingAssets)

		// ساخت مپ برای دسترسی سریع
		existingMap := make(map[string]*models.Asset)
		for j := range existingAssets {
			existingMap[existingAssets[j].Value] = &existingAssets[j]
		}

		// لیست برای درج گروهی (Bulk Insert)
		var toInsert []models.Asset

		for _, val := range chunk {
			ips, found := liveResults[val]
			hasIPs := found && len(ips) > 0
			isLive := hasIPs

			dnsxIPJSON := "[]"
			if hasIPs {
				bytes, _ := json.Marshal(ips)
				dnsxIPJSON = string(bytes)
			}

			if existing, ok := existingMap[val]; ok {
				// --- دارایی قدیمی (آپدیت) ---
				// برای آپدیت‌ها از تراکنش تکی استفاده می‌کنیم چون لاجیک شرطی دارد
				// (GORM آپدیت بالک شرطی پیچیده را سخت ساپورت می‌کند، اما تعداد آپدیت‌ها معمولاً کمتر از اینسرت‌هاست)

				if existing.IsLive != isLive {
					now := time.Now()
					logChange(database.DB, existing.ID, "is_live", strconv.FormatBool(existing.IsLive), strconv.FormatBool(isLive))

					if isLive {
						telegram.SendChangeAlert(target.RootDomain, val, "is_live", "false", "true")
					}

					database.DB.Model(existing).Updates(map[string]interface{}{
						"is_live":        isLive,
						"is_new":         true,
						"last_change_at": &now,
						"dnsx_ip":        dnsxIPJSON,
					})
					countUpdated++
				} else if isLive && existing.DnsxIP != dnsxIPJSON {
					database.DB.Model(existing).Update("dnsx_ip", dnsxIPJSON)
				}

			} else {
				// --- دارایی جدید (اضافه به لیست درج گروهی) ---
				newAsset := models.Asset{
					TargetID:     target.ID,
					Value:        val,
					Type:         "subdomain",
					IsNew:        true,
					IsLive:       isLive,
					Technologies: "[]",
					HostIP:       "[]",
					RawHttpx:     "{}",
					DnsxIP:       dnsxIPJSON,
				}
				toInsert = append(toInsert, newAsset)

				if !isFirstRun && isLive {
					telegram.SendNewAssetAlert(target.RootDomain, val)
				}
				countNew++
			}
		}

		// 2. درج گروهی رکوردهای جدید (Bulk Insert)
		if len(toInsert) > 0 {
			// استفاده از CreateInBatches برای سرعت بالا
			if err := database.DB.CreateInBatches(toInsert, 500).Error; err != nil {
				log.Printf("❌ Bulk insert failed: %v\n", err)
			}
		}
	}

	log.Printf("✅ DB Sync Complete. New: %d, Status Changed: %d.\n", countNew, countUpdated)
}

// updateAssetsWithDiff (بهینه شده: استفاده از داده‌های از قبل خوانده شده)
func updateAssetsWithDiff(targetID uint, batchAssets []models.Asset, results map[string]HttpxResult) {
	// گرفتن نام تارگت
	var targetName string
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err == nil {
		targetName = target.RootDomain
	}

	// ساخت مپ از دارایی‌های موجود در بچ (برای دسترسی سریع بدون دیتابیس)
	assetsMap := make(map[string]*models.Asset)
	for i := range batchAssets {
		assetsMap[batchAssets[i].Value] = &batchAssets[i]
	}

	countUpdated := 0
	countChanges := 0

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for hostInput, result := range results {
			// 👇👇👇 بهینه‌سازی: به جای کوئری زدن، از مپ استفاده می‌کنیم
			asset, exists := assetsMap[hostInput]
			if !exists {
				// این حالت نباید پیش بیاد چون httpx روی همین لیست اجرا شده
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

			// مقایسه‌ها (با استفاده از آبجکت asset که در حافظه داریم)
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
				countChanges++
			}

			// آماده‌سازی مقادیر آپدیت
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

			// ذخیره تغییرات
			if err := tx.Save(asset).Error; err != nil {
				return err
			}
			countUpdated++
		}
		return nil
	})

	if err != nil {
		log.Printf("❌ Transaction failed: %v\n", err)
	} else {
		log.Printf("✅ Smart Update Complete. Updated: %d, Changes Detected: %d.\n", countUpdated, countChanges)
	}
}

// ==========================================
// Helper Functions
// ==========================================

func updateTargetPhase(targetID uint, phase string) {
	database.DB.Model(&models.Target{}).Where("id = ?", targetID).Update("current_phase", phase)
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
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"last_scan_at":  &now,
			"status":        "READY",
			"scan_count":    gorm.Expr("scan_count + 1"),
			"current_phase": "IDLE",
		})
	}
}

func logChange(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error {
	if oldVal == newVal {
		return nil
	}
	history := models.AssetHistory{
		AssetID: assetID, FieldName: field, OldValue: oldVal, NewValue: newVal, CreatedAt: time.Now(),
	}
	if err := tx.Create(&history).Error; err != nil {
		log.Printf("❌ Failed to create history log: %v\n", err)
		return err
	}
	return nil
}

// DnsxResult struct
type DnsxResult struct {
	Host string   `json:"host"`
	A    []string `json:"a"`
}

// runDnsx: Optimized DNS resolution using dnsx
func runDnsx(inputFile string) (map[string][]string, error) {
	const outputFile = "/tmp/dnsx_output.json"

	// دستور dnsx با ریزالورهای پیش‌فرض خودش (که خیلی خوبن)
	// اگر بخوایم ریزالور اختصاصی بدیم، باید فایلش رو بسازیم
	// فعلا از دیفالت استفاده می‌کنیم که دردسر فایل نداره
	cmd := exec.Command("dnsx",
		"-l", inputFile,
		"-json",
		"-o", outputFile,
		"-silent",
		"-a",
		"-resp",
		"-threads", "50",
	)

	log.Printf("Executing dnsx command...\n")
	if err := cmd.Run(); err != nil {
		log.Printf("⚠️ dnsx finished with issue: %v\n", err)
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

// checkStopRequest بررسی می‌کند آیا کاربر درخواست توقف داده است؟
// اگر بله، وضعیت را ریست می‌کند و true برمی‌گرداند
func checkStopRequest(targetID uint) bool {
	var t models.Target
	database.DB.Select("stop_requested").First(&t, targetID)

	if t.StopRequested {
		log.Printf("bw🛑 Stop requested for target %d. Aborting job...\n", targetID)

		// ریست کردن وضعیت تارگت
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"status":         "READY",
			"current_phase":  "STOPPED BY USER",
			"stop_requested": false, // ریست کردن پرچم برای دفعه بعد
		})
		return true
	}
	return false
}
