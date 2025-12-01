package worker

import (
	"bufio"
	"bytes"
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
	dnsxOutputFile    = "/tmp/dnsx_output.json"
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

// Start موتور اصلی کارگر را روشن می‌کند.
func Start() {
	// 👇👇👇 چاپ بنر اختصاصی تیم Mustache
	RED := "\033[0;31m"
	NC := "\033[0m" // No Color

	fmt.Println()
	fmt.Println(RED + "+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-" + NC)
	fmt.Println(RED + "                                                                                  ")
	fmt.Println(RED + "`7MMM.     ,MMF'                      mm                        MM                ")
	fmt.Println(RED + "  MMMb    dPMM                        MM                        MM                ")
	fmt.Println(RED + "  M YM   ,M MM `7MM  `7MM  ,pP\"Ybd mmMMmm  ,6\"Yb.    ,p6\"bo   MMpMMMb.    .gP\"Ya  ")
	fmt.Println(RED + "  M  Mb  M' MM   MM    MM  8I   `\"   MM   8) MM 6M  'OO       MM    MM   ,M'   Yb ")
	fmt.Println(RED + "  M  YM.P'  MM   MM    MM  `YMMMa.   MM    ,pm9MM    8M       MM    MM   8M\"\"\"\"\"\" ")
	fmt.Println(RED + "  M  `YM'   MM   MM    MM  L.   I8   MM   8M   MM    YM.    , MM    MM   YM.    , ")
	fmt.Println(RED + ".JML. `'  .JMML. `Mbod\"YML.M9mmmP'   `Mbmo`Moo9^Yo.  YMbmd' .JMML  JMML.  `Mbmmd' ")
	fmt.Println(RED + "                                                                                  " + NC)
	fmt.Println()
	fmt.Println(RED + "             ...:~!!~:..........^~~^....^~~^..........:~!!~:...")
	fmt.Println(RED + "             .!PBGP5PG5^.....:?PB##BGJJPB##BP?:.....^YGP5PGBP!.")
	fmt.Println(RED + "             J#B!:...^PG...:?B################B?:...GP^...:!B#J")
	fmt.Println(RED + "             ##7 .....!7..7G####################G7: 7!..... !##")
	fmt.Println(RED + "             P&Y.......^?G###########YY###########G?^.......Y&P")
	fmt.Println(RED + "             ^5#GJ7!7JPB##########BY~..~YB##########BPJ7!7JG#5^")
	fmt.Println(RED + "             ..~YG##&&&&####BBG5?!:......:!?5GBB####&&&&##GY!..")
	fmt.Println(RED + "             ....:^~!7777!!~^:................:^~!!7777!~^:....")
	fmt.Println()
	fmt.Println()

	fmt.Printf("%s%s%s\n", RED, " 🔍 Automated Reconnaissance System", NC)
	fmt.Printf("%s%s%s\n", RED, " 👨‍💻 Recon And Watch Tower Tool From Mustache Team 👨‍💻", NC)
	fmt.Printf("%s%s%s\n", RED, " 📅 "+time.Now().Format("2006-01-02 15:04:05"), NC)
	fmt.Println(RED + "+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-+-" + NC)
	fmt.Println()

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

	// 👇 ابتدا اطلاعات تارگت را می‌گیریم تا کانفیگ را چک کنیم
	var targetConf models.Target
	if err := database.DB.First(&targetConf, targetID).Error; err != nil {
		log.Printf("❌ Failed to fetch target config: %v\n", err)
		return
	}

	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: STARTED")
	startTime := time.Now()

	// 1. Passive
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: PASSIVE ENUM")
	passiveResults := runPassiveCollection(targetID, rootDomain)

	// 2. Mutation (Alterx) - 👇👇👇 شرط حیاتی برای چک کردن تنظیمات کاربر
	var mutatedResults []string

	// فقط اگر UseAlterx روشن باشد، این بخش اجرا می‌شود
	if targetConf.UseAlterx {
		if checkStopRequest(targetID) {
			return
		}
		updateTargetPhase(targetID, "PHASE 1: MUTATION (ALTERX)")
		writeSliceToFile(allFoundFile, passiveResults)

		var err error
		// اجرای Alterx با قابلیت Stop
		mutatedResults, err = runAlterx(targetID, allFoundFile, rootDomain)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return
			}
			log.Printf("❌ Alterx failed: %v. Proceeding without mutations.\n", err)
			mutatedResults = []string{}
		}
	} else {
		// اگر خاموش بود، لاگ می‌زنیم و رد می‌شویم
		log.Printf("⏩ Skipping Alterx (Mutation) for %s based on target config.\n", rootDomain)
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
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: DNSX VALIDATION")

	// 👇 استفاده از تابع ایمن
	dnsxResults, err := runDnsx(targetID, allFoundFile)
	if err != nil {
		if err.Error() == "process killed by user request" {
			return
		}
		log.Printf("❌ Dnsx failed: %v\n", err)
		return
	}
	log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains with IPs.\n", len(dnsxResults))

	// 6. Saving
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: SAVING RESULTS")
	var target models.Target
	database.DB.First(&target, targetID)
	saveDiscoveryResultsToDB(target, masterList, dnsxResults)

	log.Printf("🏁 PHASE 1 finished for %s in %s.\n", rootDomain, time.Since(startTime))
	triggerNextModule(targetID, rootDomain, "DISCOVERY")
}

// ... (بقیه توابع فایل، همان نسخه‌های قبلی و کامل هستند که در پیام‌های قبل داشتید. برای جلوگیری از تکرار زیاد، فقط تابع اصلی که باگ داشت را در بالا گذاشتم اما شما فایل کامل را از پاسخ قبلی کپی کنید و فقط تابع runDiscoveryPhase را با این نسخه عوض کنید، یا اگر می‌خواهید، من دوباره کل فایل را بگذارم؟)
// (برای اطمینان، این پایین کل فایل را دوباره می‌گذارم)

// =================================================================
// PHASE 2: Probing Implementation
// =================================================================
func runProbingPhase(targetID uint, rootDomain string) {
	if checkStopRequest(targetID) {
		return
	}

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
			return
		}

		statusMsg := fmt.Sprintf("PHASE 2: BATCH %d/%d", currentBatch, totalBatches)
		updateTargetPhase(targetID, statusMsg)

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

		_, err = runCommandWithKillSwitch(targetID, "httpx",
			"-l", probingInputFile,
			"-json",
			"-o", probingOutputFile,
			"-silent",
			"-threads", "50",
			"-follow-redirects",
		)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return
			}
			log.Printf("⚠️ Httpx batch finished with issues: %v\n", err)
		}

		httpxResults, err := readHttpxResults(probingOutputFile)
		if err != nil {
			log.Printf("❌ Error reading batch httpx output: %v\n", err)
		} else {
			UpdateAssetsWithDiff(targetID, httpxResults)
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

func saveDiscoveryResultsToDB(target models.Target, masterList []string, liveResults map[string][]string) {
	log.Printf("💾 Saving/Updating assets (Scan Count: %d)...", target.ScanCount)

	isFirstRun := target.ScanCount == 0
	countNew := 0
	countUpdated := 0

	chunkSize := 1000
	for i := 0; i < len(masterList); i += chunkSize {
		end := i + chunkSize
		if end > len(masterList) {
			end = len(masterList)
		}
		chunk := masterList[i:end]

		var existingAssets []models.Asset
		database.DB.Where("target_id = ? AND value IN ?", target.ID, chunk).Find(&existingAssets)

		existingMap := make(map[string]*models.Asset)
		for j := range existingAssets {
			existingMap[existingAssets[j].Value] = &existingAssets[j]
		}

		var toInsert []models.Asset

		for _, val := range chunk {
			ips, foundInDnsx := liveResults[val]
			hasIPs := foundInDnsx && len(ips) > 0
			isLive := hasIPs

			dnsxIPJSON := "[]"
			if hasIPs {
				bytes, _ := json.Marshal(ips)
				dnsxIPJSON = string(bytes)
			}

			if existing, ok := existingMap[val]; ok {
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

		if len(toInsert) > 0 {
			if err := database.DB.CreateInBatches(toInsert, 500).Error; err != nil {
				log.Printf("❌ Bulk insert failed: %v\n", err)
			}
		}
	}
	log.Printf("✅ DB Sync Complete. New: %d, Status Changed: %d.\n", countNew, countUpdated)
}

func UpdateAssetsWithDiff(targetID uint, results map[string]HttpxResult) {
	var targetName string
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err == nil {
		targetName = target.RootDomain
	}

	countUpdated := 0
	countChanges := 0

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for hostInput, result := range results {
			var asset models.Asset
			if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("value = ? AND target_id = ?", hostInput, targetID).First(&asset).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					asset = models.Asset{
						TargetID: targetID, Value: hostInput, Type: "subdomain", IsNew: true, IsLive: true,
						Technologies: "[]", HostIP: "[]", RawHttpx: "{}", DnsxIP: "[]",
					}
					if err := tx.Create(&asset).Error; err != nil {
						return err
					}
					telegram.SendNewAssetAlert(targetName, hostInput)
				} else {
					continue
				}
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
				countChanges++
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
// Helper Functions (Core Logic)
// ==========================================

func runCommandWithKillSwitch(targetID uint, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command %s: %w", name, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			return outBuf.Bytes(), err
		case <-ticker.C:
			if checkStopRequest(targetID) {
				log.Printf("🛑 Kill switch activated for target %d. Killing process %s...", targetID, name)
				if err := cmd.Process.Kill(); err != nil {
					log.Printf("⚠️ Failed to kill process: %v", err)
				}
				return nil, fmt.Errorf("process killed by user request")
			}
		}
	}
}

func checkStopRequest(targetID uint) bool {
	var t models.Target
	if err := database.DB.Select("stop_requested").First(&t, targetID).Error; err != nil {
		return false
	}
	if t.StopRequested {
		now := time.Now()
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"status": "PAUSED", "current_phase": "PAUSED BY USER", "stop_requested": false, "last_scan_at": &now,
		})
		return true
	}
	return false
}

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
			"last_scan_at": &now, "status": "READY", "scan_count": gorm.Expr("scan_count + 1"), "current_phase": "IDLE",
		})
	}
}

func logChange(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error {
	if oldVal == newVal {
		return nil
	}
	history := models.AssetHistory{AssetID: assetID, FieldName: field, OldValue: oldVal, NewValue: newVal, CreatedAt: time.Now()}
	return tx.Create(&history).Error
}

func runDnsx(targetID uint, inputFile string) (map[string][]string, error) {
	const outputFile = "/tmp/dnsx_output.json"
	_, err := runCommandWithKillSwitch(targetID, "dnsx", "-l", inputFile, "-json", "-o", outputFile, "-silent", "-a", "-resp", "-threads", "50")
	if err != nil {
		return nil, err
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

func runPassiveCollection(targetID uint, domain string) []string {
	var wg sync.WaitGroup
	results := make(chan string, 50000)
	runTool := func(name string, args ...string) {
		defer wg.Done()
		output, err := runCommandWithKillSwitch(targetID, name, args...)
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				results <- strings.TrimSpace(line)
			}
		} else {
			log.Printf("❌ %s error/killed: %v\n", name, err)
		}
	}
	wg.Add(2)
	go runTool("subfinder", "-d", domain, "-silent", "-all")
	go runTool("assetfinder", "--subs-only", domain)
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

func runAlterx(targetID uint, inputFile, rootDomain string) ([]string, error) {
	output, err := runCommandWithKillSwitch(targetID, "alterx", "-l", inputFile, "-silent")
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
