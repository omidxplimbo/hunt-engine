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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

// مسیرهای فایل‌های موقت
const (
	// فاز ۱
	allFoundFile = "/tmp/all_found.txt"
	// finalLiveFile = "/tmp/live.txt" // 👈 این دیگه لازم نیست
	// فاز ۲
	probingInputFile  = "/tmp/probing_input.txt"
	probingOutputFile = "/tmp/probing_output.json"
)

// مقادیر پیش‌فرض کانفیگ
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
	startTime := time.Now()

	// 1. Passive
	passiveResults := runPassiveCollection(rootDomain)
	if len(passiveResults) == 0 {
		log.Println("⚠️ No subdomains found passively. Aborting.")
		return
	}

	// 2. Mutation
	writeSliceToFile(allFoundFile, passiveResults)
	mutatedResults, err := runAlterx(allFoundFile, rootDomain)
	if err != nil {
		log.Printf("❌ Alterx failed: %v. Proceeding without mutations.\n", err)
		mutatedResults = []string{}
	}
	masterList := mergeUnique(passiveResults, mutatedResults)
	writeSliceToFile(allFoundFile, masterList)
	log.Printf("📊 Master list created with %d potential subdomains. Starting validation...\n", len(masterList))

	// 3. Validation (Puredns)
	// 👇👇👇 اصلاح فراخوانی (حذف پارامتر دوم)
	liveSubdomains, err := runPuredns(allFoundFile)
	if err != nil {
		log.Printf("❌ Puredns failed critically: %v\n", err)
		liveSubdomains = []string{}
	} else {
		log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains.\n", len(liveSubdomains))
	}

	// 4. Saving
	saveDiscoveryResultsToDB(targetID, masterList, liveSubdomains)

	log.Printf("🏁 PHASE 1 finished for %s in %s.\n", rootDomain, time.Since(startTime))
}

// =================================================================
// PHASE 2: Probing Implementation
// =================================================================
func runProbingPhase(targetID uint, rootDomain string) {
	batchSize := getBatchSize()
	log.Printf("🚀 Starting PHASE 2 (PROBING) for target ID: %d (Batch Size: %d)\n", targetID, batchSize)
	startTime := time.Now()

	var totalLive int64
	database.DB.Model(&models.Asset{}).Where("target_id = ? AND is_live = true", targetID).Count(&totalLive)

	if totalLive == 0 {
		log.Println("⚠️ No live assets found for this target in DB. Aborting probing.")
		return
	}
	log.Printf("✅ Found %d live assets in DB. Starting batch processing...\n", totalLive)

	processedCount := 0
	offset := 0

	for {
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

		// اجرای httpx
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
			updateAssetsInDB(targetID, httpxResults)
			processedCount += len(batchAssets)
		}

		offset += batchSize
		log.Printf("✅ Batch finished. Progress: %d/%d\n", processedCount, totalLive)
	}

	log.Printf("🏁 PHASE 2 (PROBING) finished for target ID %d. Total processed: %d in %s.\n", targetID, processedCount, time.Since(startTime))
}

// ==========================================
// Helper Functions
// ==========================================

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

// runPuredns (نسخه نهایی و اصلاح‌شده - خواندن مستقیم از Stdout)
func runPuredns(inputFile string) ([]string, error) {
	const resolversPath = "/root/trusted_resolvers.txt"

	// حذف --write و استفاده از خروجی استاندارد
	// حذف -q برای اینکه اگر خطایی داشت توی لاگ ببینیم
	cmd := exec.Command("puredns", "resolve", inputFile,
		"-r", resolversPath,
		"--resolvers-trusted", resolversPath,
		"-w", "20", "--rate-limit", "5000",
	)

	log.Printf("Executing puredns command and capturing stdout (resolvers: %s)\n", resolversPath)

	// گرفتن مستقیم خروجی استاندارد (Stdout)
	outputBytes, err := cmd.Output()
	if err != nil {
		// اگر puredns با خطا روبرو بشه، لاگ می‌کنیم ولی خروجی رو هم پردازش می‌کنیم
		log.Printf("⚠️ Puredns command finished with error state: %v\n", err)
	}

	// تبدیل بایت به رشته و جداسازی خطوط
	outputStr := string(outputBytes)
	lines := strings.Split(outputStr, "\n")

	var results []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		// فیلتر کردن خطوط خالی و لاگ‌های احتمالی puredns (که با [ شروع میشن)
		if trimmed != "" && !strings.HasPrefix(trimmed, "[") {
			results = append(results, trimmed)
		}
	}

	log.Printf("✅ Puredns finished. Captured %d live domains from stdout.\n", len(results))
	return results, nil
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

func updateAssetsInDB(targetID uint, results map[string]HttpxResult) {
	log.Printf("💾 Updating DB with probing data for %d assets...", len(results))
	countUpdated := 0

	for hostInput, result := range results {
		techJSON, _ := json.Marshal(result.Technologies)
		if string(techJSON) == "null" || string(techJSON) == "" {
			techJSON = []byte("[]")
		}

		ipsJSON, _ := json.Marshal(result.A)
		if string(ipsJSON) == "null" || string(ipsJSON) == "" {
			ipsJSON = []byte("[]")
		}

		var respTime int64
		timeStr := strings.TrimSuffix(result.ResponseTime, "ms")
		if val, err := strconv.ParseFloat(timeStr, 64); err == nil {
			respTime = int64(val)
		}

		dbResult := database.DB.Model(&models.Asset{}).
			Where("value = ? AND target_id = ?", hostInput, targetID).
			Updates(map[string]interface{}{
				"final_url":        result.URL,
				"status_code":      result.StatusCode,
				"title":            result.Title,
				"content_length":   result.ContentLength,
				"host_ip":          string(ipsJSON),
				"web_server":       result.WebServer,
				"cdn_name":         result.CDNName,
				"technologies":     string(techJSON),
				"body_hash":        result.BodyHash,
				"header_hash":      result.HeaderHash,
				"response_time_ms": respTime,
				"raw_httpx":        result.RawJSON,
			})

		if dbResult.Error != nil {
			log.Printf("⚠️ DB Error updating asset %s: %v\n", hostInput, dbResult.Error)
		} else if dbResult.RowsAffected > 0 {
			countUpdated++
		} else {
			log.Printf("⚠️ Warning: Httpx found result for input '%s', but no matching asset found in DB.\n", hostInput)
		}
	}
	log.Printf("✅ DB Update Complete. Successfully updated %d assets.\n", countUpdated)
}

// ==========================================
// Shared Utility Functions (Phase 1)
// ==========================================
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

func saveDiscoveryResultsToDB(targetID uint, masterList []string, liveList []string) {
	log.Printf("💾 Saving/Updating %d potential assets to DB (Smart Upsert)...", len(masterList))

	liveMap := make(map[string]bool)
	for _, l := range liveList {
		liveMap[l] = true
	}

	countNew := 0
	countUpdated := 0

	for _, val := range masterList {
		isLive := liveMap[val]

		asset := models.Asset{
			TargetID:     targetID,
			Value:        val,
			Type:         "subdomain",
			IsNew:        true,
			IsLive:       isLive,
			Technologies: "[]",
			HostIP:       "[]",
			RawHttpx:     "{}",
		}

		var existingAsset models.Asset
		result := database.DB.Where("value = ? AND target_id = ?", val, targetID).First(&existingAsset)

		if result.Error == nil {
			if existingAsset.IsLive != isLive {
				database.DB.Model(&existingAsset).Updates(map[string]interface{}{
					"is_live": isLive,
					"is_new":  true,
				})
				countUpdated++
			}
		} else {
			database.DB.Create(&asset)
			countNew++
		}
	}
	log.Printf("✅ DB Sync Complete. New: %d, Status Changed: %d.\n", countNew, countUpdated)
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

func readSliceFromFile(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return []string{}, err
	}
	defer file.Close()
	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines, scanner.Err()
}
