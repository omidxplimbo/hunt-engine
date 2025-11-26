package worker

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

const (
	resolversFile = "/tmp/resolvers.txt"
	allFoundFile  = "/tmp/all_found.txt"
	finalLiveFile = "/tmp/live.txt"
)

// Start موتور اصلی کارگر را روشن می‌کند.
func Start() {
	log.Println("👷 Worker started. Waiting for jobs...", redisq.QueueName)
	if err := downloadResolvers(); err != nil {
		log.Printf("⚠️ Warning: Could not download fresh resolvers. %v\n", err)
	}

	for {
		result, err := redisq.Client.BLPop(context.Background(), 0*time.Second, redisq.QueueName).Result()
		if err != nil {
			log.Printf("❌ Error popping from Redis: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		log.Printf("👷 Worker picked up job: %s\n", result[1])
		processJob(result[1])
	}
}

// processJob مدیر اجرایی پایپ‌لاین است
func processJob(payload string) {
	parts := strings.Split(payload, ":")
	if len(parts) != 2 {
		return
	}
	var targetID uint
	fmt.Sscanf(parts[0], "%d", &targetID)
	rootDomain := parts[1]

	log.Printf("🚀 Starting FULL RECON for: %s (ID: %d)\n", rootDomain, targetID)
	startTime := time.Now()

	// 1. Passive Collection
	log.Println("📡 [Stage 1] Passive Collection...")
	passiveResults := runPassiveCollection(rootDomain)
	log.Printf("✅ [Stage 1] Found %d passive subdomains.\n", len(passiveResults))

	if len(passiveResults) == 0 {
		log.Println("⚠️ No subdomains found passively. Aborting.")
		return
	}

	// 2. Mutation (Alterx)
	log.Println("🧬 [Stage 2] Mutation (Alterx)...")
	// برای آلترکس باید ورودی رو توی فایل بنویسیم
	writeSliceToFile(allFoundFile, passiveResults)

	mutatedResults, err := runAlterx(allFoundFile, rootDomain)
	if err != nil {
		log.Printf("❌ [Stage 2] Alterx failed: %v. Proceeding without mutations.\n", err)
		mutatedResults = []string{}
	} else {
		log.Printf("✅ [Stage 2] Generated %d mutations.\n", len(mutatedResults))
	}

	// ادغام نتایج پسیو و جهش‌یافته در یک لیست اصلی
	masterList := mergeUnique(passiveResults, mutatedResults)
	log.Printf("📊 Total unique potential subdomains found: %d\n", len(masterList))

	// نوشتن لیست نهایی برای puredns
	writeSliceToFile(allFoundFile, masterList)

	// 3. Validation (Puredns)
	log.Println("🎯 [Stage 3] Active Validation (Puredns)...")
	liveSubdomains, err := runPuredns(allFoundFile, finalLiveFile, resolversFile)
	if err != nil {
		log.Printf("❌ [Stage 3] Puredns failed critically: %v\n", err)
		// حتی اگر puredns فیل شد، ما باید نتایج پسیو رو ذخیره کنیم (به عنوان غیر زنده)
		liveSubdomains = []string{}
	} else {
		log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains.\n", len(liveSubdomains))
	}

	// 4. Saving EVERYTHING (Smart Upsert)
	// ما کل masterList رو ذخیره می‌کنیم، و از liveSubdomains برای تعیین وضعیت استفاده می‌کنیم
	saveResultsToDB(targetID, masterList, liveSubdomains)

	log.Printf("🏁 Pipeline finished for %s in %s.\n", rootDomain, time.Since(startTime))
}

// ================= Helper Functions =================

func downloadResolvers() error {
	log.Println("🔄 Downloading fresh DNS resolvers...")
	resp, err := http.Get("https://raw.githubusercontent.com/proabiral/dns-validator/master/resolvers.txt")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(resolversFile, body, 0644)
}

func runPassiveCollection(domain string) []string {
	var wg sync.WaitGroup
	results := make(chan string, 50000) // بافر بزرگتر

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
		// فیلتر کردن دامنه‌های نامرتبط و تکراری
		if res != "" && strings.HasSuffix(res, domain) && !uniqueMap[res] {
			uniqueMap[res] = true
			finalSlice = append(finalSlice, res)
		}
	}
	return finalSlice
}

// runAlterx حالا خروجی رو برمی‌گردونه به جای نوشتن در فایل
func runAlterx(inputFile, rootDomain string) ([]string, error) {
	// alterx از stdin می‌خونه. ما فایل رو می‌خونیم و پایپ می‌کنیم بهش
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
		// مطمئن میشیم که alterx دامنه‌هایی خارج از اسکوپ نساخته باشه
		if trimmed != "" && strings.HasSuffix(trimmed, rootDomain) {
			results = append(results, trimmed)
		}
	}
	return results, nil
}

func runPuredns(inputFile, outputFile, resolvers string) ([]string, error) {
	cmd := exec.Command("puredns", "resolve", inputFile, "-r", resolvers, "--write", outputFile, "--resolvers-trusted", resolvers, "-q", "-w", "20")
	if err := cmd.Run(); err != nil {
		// Puredns اگر هیچ دامنه‌ای زنده نباشه هم exit code 1 میده گاهی.
		// پس خطا رو لاگ می‌کنیم ولی فایل خروجی رو چک می‌کنیم.
		log.Printf("⚠️ Puredns finished with potential issues: %v\n", err)
	}
	return readSliceFromFile(outputFile)
}

// saveResultsToDB منطق جدید ذخیره‌سازی
func saveResultsToDB(targetID uint, masterList []string, liveList []string) {
	log.Printf("💾 Saving/Updating %d potential assets to DB...", len(masterList))

	// تبدیل لیست زنده‌ها به مپ برای جستجوی سریع (O(1))
	liveMap := make(map[string]bool)
	for _, l := range liveList {
		liveMap[l] = true
	}

	countNew := 0
	countUpdated := 0

	for _, val := range masterList {
		isLive := liveMap[val] // چک می‌کنیم آیا این دامنه توی لیست زنده‌ها هست؟

		asset := models.Asset{
			TargetID:     targetID,
			Value:        val,
			Type:         "subdomain",
			IsNew:        true,
			IsLive:       isLive, // وضعیت واقعی رو ست می‌کنیم
			Technologies: "[]",
		}

		// تلاش برای پیدا کردن رکورد موجود
		var existingAsset models.Asset
		result := database.DB.Where("value = ? AND target_id = ?", val, targetID).First(&existingAsset)

		if result.Error == nil {
			// رکورد قبلاً وجود داشته. چک می‌کنیم وضعیتش عوض شده؟
			if existingAsset.IsLive != isLive {
				// وضعیت تغییر کرده! (مثلا قبلا مرده بود الان زنده شده)
				database.DB.Model(&existingAsset).Updates(map[string]interface{}{
					"is_live": isLive,
					"is_new":  true, // دوباره مارکش می‌کنیم به عنوان جدید که توی داشبورد دیده بشه
				})
				countUpdated++
			}
		} else {
			// رکورد جدید است
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
