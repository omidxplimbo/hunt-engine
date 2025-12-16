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
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
	"gorm.io/gorm"
)

// مسیرهای فایل‌های موقت (بهتر است داینامیک باشند، اما فعلا برای سازگاری نگه داشته شده‌اند)
// نکته: در توابع جدید از نام‌های داینامیک استفاده کردیم
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

// DnsxResult ساختار خروجی JSON ابزار dnsx
type DnsxResult struct {
	Host string   `json:"host"`
	A    []string `json:"a"`
}

// Start موتور اصلی کارگر را روشن می‌کند.
func Start() {
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
	case "CRAWLING":
		runCrawlingPhase(targetID, rootDomain)
	default:
		log.Printf("⚠️ Unknown job type: %s\n", jobType)
	}
}

// =================================================================
// PHASE 1: Discovery Implementation
// =================================================================
func runDiscoveryPhase(targetID uint, rootDomain string) {
	log.Printf("🚀 Starting PHASE 1 (DISCOVERY) for: %s (ID: %d)\n", rootDomain, targetID)

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

	// 2. Mutation (Alterx)
	var mutatedResults []string
	if targetConf.UseAlterx {
		if checkStopRequest(targetID) {
			return
		}
		updateTargetPhase(targetID, "PHASE 1: MUTATION (ALTERX)")
		writeSliceToFile(allFoundFile, passiveResults)

		var err error
		mutatedResults, err = runAlterx(targetID, allFoundFile, rootDomain)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return
			}
			log.Printf("❌ Alterx failed: %v. Proceeding without mutations.\n", err)
			mutatedResults = []string{}
		}
	} else {
		log.Printf("⏩ Skipping Alterx (Mutation) for %s based on target config.\n", rootDomain)
	}

	// 3. Merge & History
	var existingAssets []string
	database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID).Pluck("value", &existingAssets)

	masterList := mergeUnique(passiveResults, mutatedResults)
	masterList = mergeUnique(masterList, existingAssets)
	writeSliceToFile(allFoundFile, masterList)
	log.Printf("📊 Master list created with %d potential subdomains. Starting validation (DNSX)...\n", len(masterList))

	// 4. Validation (DNSX)
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: DNSX VALIDATION")

	dnsxResults, err := runDnsx(targetID, allFoundFile)
	if err != nil {
		if err.Error() == "process killed by user request" {
			return
		}
		log.Printf("❌ Dnsx failed: %v\n", err)
		return
	}
	log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains with IPs.\n", len(dnsxResults))

	// 5. Saving
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: SAVING RESULTS")
	var target models.Target
	database.DB.First(&target, targetID)
	saveDiscoveryResultsToDB(target, masterList, dnsxResults)

	// 6. CDN Check for all live IPs from DNS validation
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: CDN CHECK")
	log.Printf("🔍 Starting CDN check for all live IPs from DNS validation...\n")
	runCdnCheckForLiveAssets(targetID, dnsxResults)

	log.Printf("🏁 PHASE 1 finished for %s in %s.\n", rootDomain, time.Since(startTime))
	triggerNextModule(targetID, rootDomain, "DISCOVERY")
}

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

// =================================================================
// PHASE 3: Crawling Implementation (FIXED)
// =================================================================
func runCrawlingPhase(targetID uint, rootDomain string) {
	if checkStopRequest(targetID) {
		return
	}

	// 👇 1. دریافت تنظیمات تارگت (برای چک کردن UseWaymore)
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		log.Printf("❌ Failed to fetch target config in Crawling Phase: %v\n", err)
		return
	}

	log.Printf("🚀 Starting PHASE 3 (CRAWLING) for target ID: %d\n", targetID)
	updateTargetPhase(targetID, "PHASE 3: FETCHING ASSETS")

	var liveAssets []string
	database.DB.Model(&models.Asset{}).
		Where("target_id = ? AND is_live = true", targetID).
		Pluck("value", &liveAssets)

	crawlingAssetsFile := fmt.Sprintf("/tmp/crawling_assets_%d.txt", targetID)
	if len(liveAssets) > 0 {
		if err := writeSliceToFile(crawlingAssetsFile, liveAssets); err != nil {
			log.Printf("❌ Failed to write crawling assets input: %v\n", err)
			return
		}
	}

	crawlingRootFile := fmt.Sprintf("/tmp/crawling_root_%d.txt", targetID)
	if err := writeSliceToFile(crawlingRootFile, []string{rootDomain}); err != nil {
		log.Printf("❌ Failed to write crawling root input: %v\n", err)
		return
	}

	urlsMap := make(map[string]string)

	// 1. Waybackurls
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 3: RUNNING WAYBACK")
	runToolAndCollect(targetID, crawlingRootFile, urlsMap, "wayback", "waybackurls")

	// 2. GAU
	if len(liveAssets) > 0 {
		if checkStopRequest(targetID) {
			return
		}
		updateTargetPhase(targetID, "PHASE 3: RUNNING GAU")
		runToolAndCollect(targetID, crawlingAssetsFile, urlsMap, "gau", "gau", "--threads", "10")
	}

	// 👇👇👇 3. Waymore Implementation (بخش اضافه شده)
	if target.UseWaymore {
		if checkStopRequest(targetID) {
			return
		}
		updateTargetPhase(targetID, "PHASE 3: RUNNING WAYMORE")
		log.Printf("🔥 Running Waymore for %s...\n", rootDomain)

		// فایل خروجی موقت برای Waymore
		waymoreOutputFile := fmt.Sprintf("/tmp/waymore_%d.txt", targetID)

		// دستور Waymore:
		// -i: دامنه ورودی
		// -mode U: فقط URLها را برگردان
		// -n: ساب‌دامین‌های جدید را اسکن نکن (فقط آرشیو)
		// -oU: مسیر فایل خروجی URLها
		_, err := runCommandWithKillSwitch(targetID, "waymore", "-i", rootDomain, "-mode", "U", "-n", "-oU", waymoreOutputFile)

		if err != nil {
			log.Printf("⚠️ Waymore execution failed: %v\n", err)
		} else {
			// خواندن فایل خروجی Waymore و اضافه کردن به لیست
			content, err := ioutil.ReadFile(waymoreOutputFile)
			if err == nil {
				lines := strings.Split(string(content), "\n")
				for _, line := range lines {
					u := strings.TrimSpace(line)
					if u != "" {
						if _, exists := urlsMap[u]; !exists {
							urlsMap[u] = "waymore"
						}
					}
				}
				log.Printf("✅ Waymore found %d URLs.\n", len(lines))
			} else {
				log.Printf("⚠️ Could not read Waymore output file: %v\n", err)
			}
			// پاک کردن فایل موقت Waymore
			os.Remove(waymoreOutputFile)
		}
	} else {
		log.Println("⏩ Skipping Waymore (Disabled in settings)")
	}
	// 👆👆👆 پایان بخش اضافه شده

	// 4. Katana
	if len(liveAssets) > 0 {
		if checkStopRequest(targetID) {
			return
		}
		updateTargetPhase(targetID, "PHASE 3: RUNNING KATANA")
		runToolAndCollect(targetID, crawlingAssetsFile, urlsMap, "katana", "katana", "-list", crawlingAssetsFile, "-jc", "-kf", "-silent", "-c", "10")
	}

	// Filtering & Saving
	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 3: FILTERING & SAVING")

	os.Remove(crawlingAssetsFile)
	os.Remove(crawlingRootFile)

	saveCrawledURLs(targetID, rootDomain, urlsMap, true)

	log.Printf("🏁 PHASE 3 finished for %s.\n", rootDomain)
	triggerNextModule(targetID, rootDomain, "CRAWLING")
}

func runToolAndCollect(targetID uint, inputFile string, results map[string]string, sourceLabel string, cmdName string, cmdArgs ...string) {
	var output []byte
	var err error

	if cmdName == "waybackurls" || cmdName == "gau" {
		catCmd := exec.Command("cat", inputFile)
		toolCmd := exec.Command(cmdName, cmdArgs...)

		toolCmd.Stdin, _ = catCmd.StdoutPipe()
		var outBuf bytes.Buffer
		toolCmd.Stdout = &outBuf

		_ = toolCmd.Start()
		_ = catCmd.Run()
		err = toolCmd.Wait()
		output = outBuf.Bytes()
	} else {
		output, err = runCommandWithKillSwitch(targetID, cmdName, cmdArgs...)
	}

	if err != nil {
		log.Printf("⚠️ Tool %s failed or killed: %v\n", cmdName, err)
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		u := strings.TrimSpace(scanner.Text())
		if u == "" {
			continue
		}
		if _, exists := results[u]; !exists {
			results[u] = sourceLabel
		}
	}
}

// saveCrawledURLs (UPDATED) اضافه شدن پارامتر silent
func saveCrawledURLs(targetID uint, rootDomain string, urls map[string]string, silent bool) {
	log.Printf("💾 Processing %d collected URLs...", len(urls))

	ignoredExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".bmp", ".ico", ".webp",
		".woff", ".woff2", ".ttf", ".eot", ".otf",
		".css",
	}

	var newUrls []models.FoundURL
	countSkippedCache := 0
	countSkippedExt := 0

	// کلید ردیس برای جلوگیری از تکرار در دفعات بعد (Continuous Monitoring)
	redisKey := fmt.Sprintf("target:%d:crawled_urls", targetID)

	for u, source := range urls {
		cleanURL := strings.Split(u, "?")[0]
		isIgnored := false
		for _, ext := range ignoredExts {
			if strings.HasSuffix(strings.ToLower(cleanURL), ext) {
				isIgnored = true
				break
			}
		}
		if isIgnored {
			countSkippedExt++
			continue
		}

		// 👇 چک کردن کش ردیس (Deduplication)
		exists, err := redisq.Client.SIsMember(context.Background(), redisKey, u).Result()
		if err == nil && exists {
			countSkippedCache++
			continue
		}

		newUrls = append(newUrls, models.FoundURL{
			TargetID: targetID,
			Value:    u,
			Source:   source,
		})

		// 👇 افزودن به کش ردیس
		redisq.Client.SAdd(context.Background(), redisKey, u)

		// 👇 ارسال نوتیفیکیشن فقط اگر silent نباشد
		if !silent {
			telegram.SendNewURLAlert(rootDomain, u, source)
		}
	}

	if len(newUrls) > 0 {
		batchSize := 500
		for i := 0; i < len(newUrls); i += batchSize {
			end := i + batchSize
			if end > len(newUrls) {
				end = len(newUrls)
			}
			if err := database.DB.CreateInBatches(newUrls[i:end], batchSize).Error; err != nil {
				log.Printf("⚠️ DB Insert Warning: %v\n", err)
			}
		}
		// کش تارگت را می‌سوزانیم تا دیتای جدید در لیست‌ها دیده شود
		cache.IncrementTargetVersion(targetID)
		log.Printf("✅ Saved %d NEW URLs to DB.", len(newUrls))
	} else {
		log.Println("✅ No new URLs to save.")
	}

	log.Printf("📊 Stats: Total Found: %d | Ignored (Ext): %d | Cached (Skip): %d | New: %d",
		len(urls), countSkippedExt, countSkippedCache, len(newUrls))
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

	// کش را می‌سوزانیم چون Asset جدید اضافه یا آپدیت شده
	if countNew > 0 || countUpdated > 0 {
		cache.IncrementTargetVersion(target.ID)
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
		// کش را می‌سوزانیم
		if countUpdated > 0 {
			cache.IncrementTargetVersion(targetID)
		}
		log.Printf("✅ Smart Update Complete. Updated: %d, Changes Detected: %d.\n", countUpdated, countChanges)
	}
}

// ... (Helper Functions)
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

// CdnCheckResult ساختار نتیجه cdncheck
type CdnCheckResult struct {
	IP      string `json:"ip"`
	IsCDN   bool   `json:"cdn"`
	CDNName string `json:"cdn_name"`
}

// runCdnCheckForLiveAssets چک کردن CDN برای همه IPهای لایو که dnsx resolve کرده
func runCdnCheckForLiveAssets(targetID uint, dnsxResults map[string][]string) {
	if len(dnsxResults) == 0 {
		log.Printf("ℹ️ No live assets found for CDN check.\n")
		return
	}

	log.Printf("🔍 Starting CDN check for all live IPs from DNS validation...\n")

	// جمع‌آوری تمام IPهای منحصر به فرد از dnsxResults
	ipSet := make(map[string]bool)
	ipToAssets := make(map[string][]string) // IP -> []AssetValue (subdomain)

	for subdomain, ips := range dnsxResults {
		for _, ip := range ips {
			if ip != "" {
				ipSet[ip] = true
				ipToAssets[ip] = append(ipToAssets[ip], subdomain)
			}
		}
	}

	if len(ipSet) == 0 {
		log.Printf("ℹ️ No IPs found for CDN check.\n")
		return
	}

	// تبدیل set به slice برای cdncheck
	ipList := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ipList = append(ipList, ip)
	}

	log.Printf("🔍 Running cdncheck on %d unique IPs from %d live subdomains...\n", len(ipList), len(dnsxResults))

	// اجرای cdncheck
	cdnResults := runCdnCheck(targetID, ipList)
	if len(cdnResults) == 0 {
		log.Printf("⚠️ No CDN check results returned.\n")
		return
	}

	// پیدا کردن assets از دیتابیس بر اساس subdomain
	var allAssets []models.Asset
	database.DB.Where("target_id = ? AND is_live = ?", targetID, true).Find(&allAssets)

	assetMap := make(map[string]*models.Asset) // subdomain -> Asset
	for i := range allAssets {
		assetMap[allAssets[i].Value] = &allAssets[i]
	}

	// به‌روزرسانی assets با اطلاعات CDN
	// برای هر asset، اگر حداقل یکی از IPهایش پشت CDN باشد، cdncheck و cdncheck_name را ذخیره می‌کنیم
	updatedCount := 0

	for subdomain, ips := range dnsxResults {
		asset, exists := assetMap[subdomain]
		if !exists {
			continue
		}

		hasCDN := false
		cdnName := ""

		// چک کردن همه IPهای این asset
		for _, ip := range ips {
			if cdnInfo, found := cdnResults[ip]; found && cdnInfo.IsCDN {
				hasCDN = true
				if cdnInfo.CDNName != "" && cdnName == "" {
					cdnName = cdnInfo.CDNName
				}
			}
		}

		// به‌روزرسانی فیلدهای cdncheck و cdncheck_name
		updates := make(map[string]interface{})
		if asset.Cdncheck != hasCDN {
			updates["cdncheck"] = hasCDN
		}
		if hasCDN && asset.CdncheckName != cdnName {
			updates["cdncheck_name"] = cdnName
		} else if !hasCDN && asset.CdncheckName != "" {
			updates["cdncheck_name"] = ""
		}

		if len(updates) > 0 {
			database.DB.Model(asset).Updates(updates)
			updatedCount++
		}
	}

	log.Printf("✅ CDN check completed. Updated %d assets with cdncheck information.\n", updatedCount)
}

// runCdnCheck اجرای cdncheck روی لیست IPها
func runCdnCheck(targetID uint, ips []string) map[string]CdnCheckResult {
	if len(ips) == 0 {
		return make(map[string]CdnCheckResult)
	}

	const inputFile = "/tmp/cdncheck_input.txt"

	// نوشتن IPها در فایل
	if err := writeSliceToFile(inputFile, ips); err != nil {
		log.Printf("❌ Error writing cdncheck input file: %v\n", err)
		return make(map[string]CdnCheckResult)
	}

	// اجرای cdncheck با فلگ‌های صحیح: -i برای input file و -j برای JSON output به stdout
	log.Printf("🔍 Running cdncheck on %d IPs...\n", len(ips))
	output, err := runCommandWithKillSwitch(targetID, "cdncheck",
		"-i", inputFile,
		"-j",
		"-nc", // no-color برای جلوگیری از ANSI codes در خروجی
	)
	if err != nil {
		if err.Error() != "process killed by user request" {
			log.Printf("⚠️ Cdncheck error: %v\n", err)
		}
		return make(map[string]CdnCheckResult)
	}

	if len(output) == 0 {
		log.Printf("⚠️ Cdncheck returned empty output.\n")
		return make(map[string]CdnCheckResult)
	}

	// خواندن نتایج از stdout
	results := make(map[string]CdnCheckResult)
	lines := strings.Split(string(output), "\n")
	parsedCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// فقط خطوطی که با { شروع می‌شوند JSON هستند (خطوط info/error معمولاً این فرمت را ندارند)
		if !strings.HasPrefix(line, "{") {
			continue
		}

		var rawResult map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rawResult); err != nil {
			// خطا در parse کردن JSON - این خط احتمالاً info message است
			continue
		}

		ip, ok := rawResult["ip"].(string)
		if !ok || ip == "" {
			continue
		}

		result := CdnCheckResult{IP: ip}

		// بررسی فیلد cdn (boolean است در خروجی cdncheck)
		if cdnVal, exists := rawResult["cdn"]; exists {
			switch v := cdnVal.(type) {
			case bool:
				result.IsCDN = v
			case string:
				result.IsCDN = (v == "true" || v == "1" || v == "yes")
			}
		}

		// بررسی فیلد cdn_name
		if cdnName, ok := rawResult["cdn_name"].(string); ok && cdnName != "" {
			result.CDNName = cdnName
		}

		// ذخیره همه نتایج (چه CDN باشد چه نباشد) تا بتوانیم تشخیص دهیم
		results[result.IP] = result
		if result.IsCDN {
			parsedCount++
			log.Printf("  ✓ IP %s: CDN detected - %s\n", result.IP, result.CDNName)
		}
	}

	if parsedCount > 0 {
		log.Printf("✅ Parsed %d CDN results from cdncheck output.\n", parsedCount)
	}

	return results
}
