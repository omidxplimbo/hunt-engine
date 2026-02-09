package worker

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	"gopkg.in/yaml.v3"
	"gorm.io/gorm"
)

const workerTempRoot = "/tmp/hunt-engine"

const DefaultBatchSize = 500

// subfinderProviderKeys is the default set of providers emitted by subfinder v2.11.0
// when it generates /root/.config/subfinder/provider-config.yaml
var subfinderProviderKeys = []string{
	"alienvault",
	"bevigil",
	"bufferover",
	"builtwith",
	"c99",
	"censys",
	"certspotter",
	"chaos",
	"chinaz",
	"digitalyama",
	"dnsdb",
	"dnsdumpster",
	"dnsrepo",
	"domainsproject",
	"driftnet",
	"facebook",
	"fofa",
	"fullhunt",
	"github",
	"intelx",
	"leakix",
	"merklemap",
	"netlas",
	"onyphe",
	"profundis",
	"pugrecon",
	"quake",
	"redhuntlabs",
	"robtex",
	"rsecloud",
	"securitytrails",
	"shodan",
	"threatbook",
	"virustotal",
	"whoisxmlapi",
	"windvane",
	"zoomeyeapi",
}

func sanitizePathComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown"
	}
	return out
}

func targetFolderName(rootDomain string, targetID uint) string {
	rootDomain = strings.TrimSpace(rootDomain)
	if rootDomain == "" {
		return fmt.Sprintf("target_%d", targetID)
	}
	// مطابق مثال: test.com => test
	label := strings.Split(rootDomain, ".")[0]
	label = strings.TrimSpace(label)
	if label == "" {
		label = rootDomain
	}
	return sanitizePathComponent(label)
}

func getTargetTempDir(targetID uint) (string, string, error) {
	var t models.Target
	if err := database.DB.Select("id", "root_domain", "created_by_user_id").First(&t, targetID).Error; err != nil {
		return "", "", err
	}

	username := "admin"
	isAdminOwner := true
	if t.CreatedByUserID != 0 {
		var u models.User
		if err := database.DB.Select("id", "username", "role").First(&u, t.CreatedByUserID).Error; err == nil {
			username = u.Username
			isAdminOwner = strings.ToLower(strings.TrimSpace(u.Role)) == "admin"
		}
	}

	baseDir := filepath.Join(workerTempRoot, "admin")
	if !isAdminOwner {
		baseDir = filepath.Join(workerTempRoot, sanitizePathComponent(username))
	}

	td := filepath.Join(baseDir, targetFolderName(t.RootDomain, targetID))
	if err := os.MkdirAll(td, 0o755); err != nil {
		return "", "", err
	}
	return td, username, nil
}

func getTargetToolTmpDir(targetID uint) string {
	td, _, err := getTargetTempDir(targetID)
	if err != nil || td == "" {
		// fallback to default OS tmp; best-effort
		return ""
	}
	tmpDir := filepath.Join(td, "_tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	return tmpDir
}

func envWithTargetTmp(targetID uint) []string {
	// Inherit existing env, but force TMPDIR/TMP/TEMP into per-target workspace
	env := append([]string{}, os.Environ()...)
	tmpDir := getTargetToolTmpDir(targetID)
	if tmpDir == "" {
		return env
	}
	env = append(env,
		"TMPDIR="+tmpDir,
		"TMP="+tmpDir,
		"TEMP="+tmpDir,
	)
	return env
}

func cleanupTargetTempDir(targetID uint) {
	td, _, err := getTargetTempDir(targetID)
	if err != nil {
		return
	}
	// حذف فقط پوشه‌ی همان target (نه فولدر کل user)
	_ = os.RemoveAll(td)
}

func acquireTargetLock(targetID uint) (func(), bool) {
	lockDir := filepath.Join(workerTempRoot, "locks")
	_ = os.MkdirAll(lockDir, 0o755)
	lockPath := filepath.Join(lockDir, fmt.Sprintf("target_%d.lock", targetID))

	type lockMeta struct {
		PID       int   `json:"pid"`
		CreatedAt int64 `json:"created_at"` // unix seconds
	}

	const staleAfter = 30 * time.Minute
	metaPath := filepath.Join(lockPath, "meta.json")

	tryAcquire := func() (func(), bool) {
		if err := os.Mkdir(lockPath, 0o755); err != nil {
			return func() {}, false
		}
		// best-effort write meta
		m := lockMeta{PID: os.Getpid(), CreatedAt: time.Now().Unix()}
		if b, err := json.Marshal(m); err == nil {
			_ = os.WriteFile(metaPath, b, 0o644)
		}
		return func() { _ = os.RemoveAll(lockPath) }, true
	}

	if release, ok := tryAcquire(); ok {
		return release, true
	}

	// already locked: check if stale (e.g., worker crash left lock behind)
	var createdAt time.Time
	if b, err := os.ReadFile(metaPath); err == nil {
		var m lockMeta
		if json.Unmarshal(b, &m) == nil && m.CreatedAt > 0 {
			createdAt = time.Unix(m.CreatedAt, 0)
		}
	}
	if createdAt.IsZero() {
		if fi, err := os.Stat(lockPath); err == nil {
			createdAt = fi.ModTime()
		}
	}

	if !createdAt.IsZero() && time.Since(createdAt) > staleAfter {
		log.Printf("🧹 Removing stale lock for target %d (age: %s)\n", targetID, time.Since(createdAt))
		_ = os.RemoveAll(lockPath)
		if release, ok := tryAcquire(); ok {
			return release, true
		}
	}

	return func() {}, false
}

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

	// Best-effort cleanup of legacy temp dirs created by tools (e.g. httpxXXXX) under /tmp.
	// With TMPDIR redirected to per-target workspace, new ones should not be created.
	cleanupLegacyToolTmp()

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

func cleanupLegacyToolTmp() {
	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-30 * time.Minute)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Only legacy httpx temp dirs in /tmp root (NOT inside /tmp/hunt-engine)
		if !strings.HasPrefix(name, "httpx") {
			continue
		}
		full := filepath.Join("/tmp", name)
		// safety: do not touch our workspace root
		if strings.HasPrefix(full, workerTempRoot) {
			continue
		}
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().Before(cutoff) {
			_ = os.RemoveAll(full)
		}
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

	// جلوگیری از اجرای همزمان چند job روی یک target (برای جلوگیری از تداخل فایل‌های temp و race در cleanup)
	releaseLock, ok := acquireTargetLock(targetID)
	if !ok {
		log.Printf("⏳ Target %d is already being processed. Waiting for lock...\n", targetID)
		for {
			time.Sleep(500 * time.Millisecond)
			releaseLock, ok = acquireTargetLock(targetID)
			if ok {
				break
			}
		}
	}
	defer releaseLock()

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

	tempDir, username, err := getTargetTempDir(targetID)
	if err != nil {
		log.Printf("❌ Failed to init temp dir for target %d: %v\n", targetID, err)
		return
	}
	log.Printf("🗂️ Temp workspace: %s (owner: %s)\n", tempDir, username)

	allFoundFile := filepath.Join(tempDir, "dnsx_all_found.txt")
	dnsxOutputFile := filepath.Join(tempDir, "dnsx_output.json")
	cdncheckInputFile := filepath.Join(tempDir, "cdncheck_input.txt")
	nmapInputFile := filepath.Join(tempDir, "nmap_input.txt")
	// checkpoint artifacts (persist across crashes so we can resume safely)
	passiveResultsFile := filepath.Join(tempDir, "passive_results.txt")
	passiveSourcesFile := filepath.Join(tempDir, "passive_sources.json")
	alterxResultsFile := filepath.Join(tempDir, "alterx_results.txt")
	alterxSourcesFile := filepath.Join(tempDir, "alterx_sources.json")
	purednsResultsFile := filepath.Join(tempDir, "puredns_results.json")
	allSourcesFile := filepath.Join(tempDir, "all_sources.json")
	dnsxResultsFile := filepath.Join(tempDir, "dnsx_results.json")

	var targetConf models.Target
	if err := database.DB.First(&targetConf, targetID).Error; err != nil {
		log.Printf("❌ Failed to fetch target config: %v\n", err)
		return
	}
	_, _ = ensureScanState(targetID)

	if checkStopRequest(targetID) {
		return
	}
	updateTargetPhase(targetID, "PHASE 1: STARTED")
	startTime := time.Now()

	// 1. Passive
	var passiveResults []string
	passiveSources := make(map[string][]string)
	if scanIsStepDone(targetID, "DISCOVERY", "PASSIVE_ENUM") {
		passiveResults, _ = readSliceFromFile(passiveResultsFile)
		_ = readJSONFromFile(passiveSourcesFile, &passiveSources)
		log.Printf("⏩ Resume: skipping PASSIVE ENUM (loaded %d results from checkpoint)\n", len(passiveResults))
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "PASSIVE_ENUM")
		updateTargetPhase(targetID, "PHASE 1: PASSIVE ENUM")
		passiveResults, passiveSources = runPassiveCollection(targetID, rootDomain)
		_ = writeSliceToFile(passiveResultsFile, passiveResults)
		_ = writeJSONToFile(passiveSourcesFile, passiveSources)
		scanMarkStepDone(targetID, "DISCOVERY", "PASSIVE_ENUM")
	}

	// 2. Mutation (Alterx)
	var mutatedResults []string
	mutatedSources := make(map[string][]string)
	if scanIsStepDone(targetID, "DISCOVERY", "ALTERX") {
		mutatedResults, _ = readSliceFromFile(alterxResultsFile)
		_ = readJSONFromFile(alterxSourcesFile, &mutatedSources)
		log.Printf("⏩ Resume: skipping ALTERX (loaded %d results from checkpoint)\n", len(mutatedResults))
	} else if targetConf.UseAlterx {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "ALTERX")
		updateTargetPhase(targetID, "PHASE 1: MUTATION (ALTERX)")
		_ = writeSliceToFile(allFoundFile, passiveResults)

		var err error
		mutatedResults, err = runAlterx(targetID, allFoundFile, rootDomain)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return
			}
			log.Printf("❌ Alterx failed: %v. Proceeding without mutations.\n", err)
			mutatedResults = []string{}
		} else {
			for _, subdomain := range mutatedResults {
				mutatedSources[subdomain] = []string{"alterx"}
			}
		}
		_ = writeSliceToFile(alterxResultsFile, mutatedResults)
		_ = writeJSONToFile(alterxSourcesFile, mutatedSources)
		scanMarkStepDone(targetID, "DISCOVERY", "ALTERX")
	} else {
		log.Printf("⏩ Skipping Alterx (Mutation) for %s based on target config.\n", rootDomain)
		scanMarkStepDone(targetID, "DISCOVERY", "ALTERX")
	}

	// 2.5. Puredns Bruteforce (فقط subdomain‌های لایو)
	var purednsResults map[string][]string
	purednsSources := make(map[string][]string)
	if scanIsStepDone(targetID, "DISCOVERY", "PUREDNS") {
		purednsResults = map[string][]string{}
		_ = readJSONFromFile(purednsResultsFile, &purednsResults)
		for subdomain := range purednsResults {
			purednsSources[subdomain] = []string{"puredns"}
		}
		log.Printf("⏩ Resume: skipping PUREDNS (loaded %d results from checkpoint)\n", len(purednsResults))
	} else if targetConf.UsePuredns {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "PUREDNS")
		updateTargetPhase(targetID, "PHASE 1: PUREDNS BRUTEFORCE")

		// پارس کردن وردلیست‌های انتخابی
		var wordlists []string
		if targetConf.PurednsWordlists != "" && targetConf.PurednsWordlists != "[]" {
			if err := json.Unmarshal([]byte(targetConf.PurednsWordlists), &wordlists); err != nil {
				log.Printf("⚠️ Failed to parse puredns wordlists: %v\n", err)
				wordlists = []string{}
			}
		}

		purednsResults = map[string][]string{}
		if len(wordlists) > 0 {
			var err error
			purednsResults, err = runPuredns(targetID, rootDomain, wordlists)
			if err != nil {
				if err.Error() == "process killed by user request" {
					return
				}
				log.Printf("❌ Puredns failed: %v. Proceeding without bruteforce results.\n", err)
				purednsResults = map[string][]string{}
			} else {
				for subdomain := range purednsResults {
					purednsSources[subdomain] = []string{"puredns"}
				}
				log.Printf("✅ Puredns found %d live subdomains for %s\n", len(purednsResults), rootDomain)
			}
		} else {
			log.Printf("⏩ Skipping Puredns for %s (no wordlists selected)\n", rootDomain)
		}
		_ = writeJSONToFile(purednsResultsFile, purednsResults)
		scanMarkStepDone(targetID, "DISCOVERY", "PUREDNS")
	} else {
		log.Printf("⏩ Skipping Puredns for %s (disabled in target config)\n", rootDomain)
		purednsResults = map[string][]string{}
		_ = writeJSONToFile(purednsResultsFile, purednsResults)
		scanMarkStepDone(targetID, "DISCOVERY", "PUREDNS")
	}

	// 3. Merge & History (creates master list + allSources map)
	var masterList []string
	allSources := make(map[string][]string)
	if scanIsStepDone(targetID, "DISCOVERY", "MERGE") {
		masterList, _ = readSliceFromFile(allFoundFile)
		_ = readJSONFromFile(allSourcesFile, &allSources)
		log.Printf("⏩ Resume: skipping MERGE (loaded %d items from checkpoint)\n", len(masterList))
	} else {
		scanMarkRunning(targetID, "DISCOVERY", "MERGE")

		var existingAssets []string
		database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID).Pluck("value", &existingAssets)

		masterList = mergeUnique(passiveResults, mutatedResults)
		var purednsSubdomains []string
		for subdomain := range purednsResults {
			purednsSubdomains = append(purednsSubdomains, subdomain)
		}
		masterList = mergeUnique(masterList, purednsSubdomains)
		masterList = mergeUnique(masterList, existingAssets)

		// Merge sources: passive + mutated + puredns
		for subdomain, sources := range passiveSources {
			allSources[subdomain] = sources
		}
		for subdomain, sources := range mutatedSources {
			if existing, ok := allSources[subdomain]; ok {
				sourceMap := make(map[string]bool)
				for _, s := range existing {
					sourceMap[s] = true
				}
				for _, s := range sources {
					sourceMap[s] = true
				}
				var merged []string
				for s := range sourceMap {
					merged = append(merged, s)
				}
				sort.Strings(merged)
				allSources[subdomain] = merged
			} else {
				allSources[subdomain] = sources
			}
		}
		for subdomain, sources := range purednsSources {
			if existing, ok := allSources[subdomain]; ok {
				sourceMap := make(map[string]bool)
				for _, s := range existing {
					sourceMap[s] = true
				}
				for _, s := range sources {
					sourceMap[s] = true
				}
				var merged []string
				for s := range sourceMap {
					merged = append(merged, s)
				}
				sort.Strings(merged)
				allSources[subdomain] = merged
			} else {
				allSources[subdomain] = sources
			}
		}

		_ = writeSliceToFile(allFoundFile, masterList)
		_ = writeJSONToFile(allSourcesFile, allSources)
		scanMarkStepDone(targetID, "DISCOVERY", "MERGE")
	}

	log.Printf("📊 Master list created with %d potential subdomains. Starting validation (DNSX)...\n", len(masterList))

	// 4. Validation (DNSX)
	var dnsxResults map[string][]string
	if scanIsStepDone(targetID, "DISCOVERY", "DNSX") {
		dnsxResults = map[string][]string{}
		_ = readJSONFromFile(dnsxResultsFile, &dnsxResults)
		log.Printf("⏩ Resume: skipping DNSX (loaded %d live subdomains from checkpoint)\n", len(dnsxResults))
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "DNSX")
		updateTargetPhase(targetID, "PHASE 1: DNSX VALIDATION")

		var err error
		dnsxResults, err = runDnsx(targetID, allFoundFile, dnsxOutputFile)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return
			}
			log.Printf("❌ Dnsx failed: %v\n", err)
			scanMarkFailed(targetID, fmt.Sprintf("dnsx failed: %v", err))
			return
		}
		_ = os.Remove(dnsxOutputFile)

		// Merge puredns results (already live) into dnsxResults
		for subdomain, ips := range purednsResults {
			if existingIPs, exists := dnsxResults[subdomain]; exists {
				ipMap := make(map[string]bool)
				for _, ip := range existingIPs {
					ipMap[ip] = true
				}
				for _, ip := range ips {
					ipMap[ip] = true
				}
				var mergedIPs []string
				for ip := range ipMap {
					mergedIPs = append(mergedIPs, ip)
				}
				dnsxResults[subdomain] = mergedIPs
			} else {
				dnsxResults[subdomain] = ips
			}
		}

		_ = writeJSONToFile(dnsxResultsFile, dnsxResults)
		scanMarkStepDone(targetID, "DISCOVERY", "DNSX")
	}

	log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains with IPs (including %d from puredns).\n", len(dnsxResults), len(purednsResults))

	// 5. Saving
	if scanIsStepDone(targetID, "DISCOVERY", "SAVE") {
		log.Printf("⏩ Resume: skipping SAVE RESULTS\n")
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "SAVE")
		updateTargetPhase(targetID, "PHASE 1: SAVING RESULTS")
		var target models.Target
		database.DB.First(&target, targetID)
		saveDiscoveryResultsToDB(target, masterList, dnsxResults, allSources)
		scanMarkStepDone(targetID, "DISCOVERY", "SAVE")
	}

	// 6. CDN Check for all live IPs from DNS validation
	if scanIsStepDone(targetID, "DISCOVERY", "CDNCHECK") {
		log.Printf("⏩ Resume: skipping CDN CHECK\n")
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "CDNCHECK")
		updateTargetPhase(targetID, "PHASE 1: CDN CHECK")
		log.Printf("🔍 Starting CDN check for all live IPs from DNS validation...\n")
		runCdnCheckForLiveAssets(targetID, dnsxResults, cdncheckInputFile)
		_ = os.Remove(cdncheckInputFile)
		scanMarkStepDone(targetID, "DISCOVERY", "CDNCHECK")
	}

	// 7. Optional Port Scan (Nmap) - ONLY: live assets resolved by dnsx AND no CDN
	if scanIsStepDone(targetID, "DISCOVERY", "NMAP") {
		log.Printf("⏩ Resume: skipping NMAP\n")
	} else if targetConf.UsePortscan {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "NMAP")
		updateTargetPhase(targetID, "PHASE 1: PORT SCAN (NMAP)")
		log.Printf("🔎 Starting optional port scan (nmap) for target %d...\n", targetID)
		runPortScanForLiveNoCDNAssets(targetID, nmapInputFile)
		_ = os.Remove(nmapInputFile)
		scanMarkStepDone(targetID, "DISCOVERY", "NMAP")
	} else {
		log.Printf("⏩ Skipping Nmap Port Scan (Disabled in settings)\n")
		scanMarkStepDone(targetID, "DISCOVERY", "NMAP")
	}

	log.Printf("🏁 PHASE 1 finished for %s in %s.\n", rootDomain, time.Since(startTime))
	scanMarkStepDone(targetID, "DISCOVERY", "DONE")
	triggerNextModule(targetID, rootDomain, "DISCOVERY")

	// cleanup big temp file at end of discovery phase
	_ = os.Remove(allFoundFile)
}

// =================================================================
// PHASE 2: Probing Implementation
// =================================================================
func runProbingPhase(targetID uint, rootDomain string) {
	if checkStopRequest(targetID) {
		return
	}

	tempDir, username, err := getTargetTempDir(targetID)
	if err != nil {
		log.Printf("❌ Failed to init temp dir for target %d: %v\n", targetID, err)
		return
	}
	log.Printf("🗂️ Temp workspace: %s (owner: %s)\n", tempDir, username)

	probingInputFile := filepath.Join(tempDir, "httpx_input.txt")
	probingOutputFile := filepath.Join(tempDir, "httpx_output.json")

	batchSize := getBatchSize()
	log.Printf("🚀 Starting PHASE 2 (PROBING) for target ID: %d (Batch Size: %d)\n", targetID, batchSize)
	updateTargetPhase(targetID, "PHASE 2: INITIALIZING")
	startTime := time.Now()

	_, _ = ensureScanState(targetID)
	if scanIsStepDone(targetID, "PROBING", "DONE") {
		log.Printf("⏩ Resume: PROBING already completed. Chaining next module.\n")
		triggerNextModule(targetID, rootDomain, "PROBING")
		return
	}

	var totalLive int64
	database.DB.Model(&models.Asset{}).Where("target_id = ? AND is_live = true", targetID).Count(&totalLive)

	if totalLive == 0 {
		log.Println("⚠️ No live assets found. Aborting probing.")
		triggerNextModule(targetID, rootDomain, "PROBING")
		return
	}
	log.Printf("✅ Found %d live assets in DB. Starting batch processing...\n", totalLive)

	meta := scanGetMeta(targetID)
	processedCount := 0
	offset := 0
	if pm, ok := meta["probing"].(map[string]interface{}); ok {
		if v, ok := pm["offset"].(float64); ok && int(v) > 0 {
			offset = int(v)
		}
		if v, ok := pm["processed_count"].(float64); ok && int(v) > 0 {
			processedCount = int(v)
		}
	}
	totalBatches := int(totalLive)/batchSize + 1
	currentBatch := (offset / batchSize) + 1

	for {
		if checkStopRequest(targetID) {
			// persist checkpoint so resume continues from this offset
			meta["probing"] = map[string]interface{}{
				"offset":          offset,
				"processed_count": processedCount,
				"batch_size":      batchSize,
				"total_live":      totalLive,
			}
			scanSetMeta(targetID, meta)
			return
		}

		statusMsg := fmt.Sprintf("PHASE 2: BATCH %d/%d", currentBatch, totalBatches)
		updateTargetPhase(targetID, statusMsg)
		scanMarkRunning(targetID, "PROBING", "HTTPX_BATCH")

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
			"-tech-detect",
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
		// پاکسازی فایل‌های موقت هر batch
		_ = os.Remove(probingOutputFile)
		_ = os.Remove(probingInputFile)

		offset += batchSize
		currentBatch++

		// persist checkpoint after each batch
		meta["probing"] = map[string]interface{}{
			"offset":          offset,
			"processed_count": processedCount,
			"batch_size":      batchSize,
			"total_live":      totalLive,
		}
		scanSetMeta(targetID, meta)
	}

	log.Printf("🏁 PHASE 2 finished. Total processed: %d in %s.\n", processedCount, time.Since(startTime))
	scanMarkStepDone(targetID, "PROBING", "DONE")
	triggerNextModule(targetID, rootDomain, "PROBING")
}

// =================================================================
// PHASE 3: Crawling Implementation (FIXED)
// =================================================================
func runCrawlingPhase(targetID uint, rootDomain string) {
	if checkStopRequest(targetID) {
		return
	}

	tempDir, username, err := getTargetTempDir(targetID)
	if err != nil {
		log.Printf("❌ Failed to init temp dir for target %d: %v\n", targetID, err)
		return
	}
	log.Printf("🗂️ Temp workspace: %s (owner: %s)\n", tempDir, username)

	// 👇 1. دریافت تنظیمات تارگت (برای چک کردن UseWaymore)
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		log.Printf("❌ Failed to fetch target config in Crawling Phase: %v\n", err)
		return
	}

	log.Printf("🚀 Starting PHASE 3 (CRAWLING) for target ID: %d\n", targetID)
	updateTargetPhase(targetID, "PHASE 3: FETCHING ASSETS")

	_, _ = ensureScanState(targetID)
	if scanIsStepDone(targetID, "CRAWLING", "DONE") {
		log.Printf("⏩ Resume: CRAWLING already completed. Chaining next module.\n")
		triggerNextModule(targetID, rootDomain, "CRAWLING")
		return
	}

	var liveAssets []string
	database.DB.Model(&models.Asset{}).
		Where("target_id = ? AND is_live = true", targetID).
		Pluck("value", &liveAssets)

	crawlingAssetsFile := filepath.Join(tempDir, "crawling_assets.txt")
	if len(liveAssets) > 0 {
		if err := writeSliceToFile(crawlingAssetsFile, liveAssets); err != nil {
			log.Printf("❌ Failed to write crawling assets input: %v\n", err)
			return
		}
	}

	crawlingRootFile := filepath.Join(tempDir, "crawling_root.txt")
	if err := writeSliceToFile(crawlingRootFile, []string{rootDomain}); err != nil {
		log.Printf("❌ Failed to write crawling root input: %v\n", err)
		return
	}

	// 1. Waybackurls
	if scanIsStepDone(targetID, "CRAWLING", "WAYBACK") {
		log.Printf("⏩ Resume: skipping WAYBACK\n")
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "CRAWLING", "WAYBACK")
		updateTargetPhase(targetID, "PHASE 3: RUNNING WAYBACK")
		tmp := make(map[string]string)
		runToolAndCollect(targetID, crawlingRootFile, tmp, "wayback", "waybackurls")
		saveCrawledURLs(targetID, rootDomain, tmp, true)
		scanMarkStepDone(targetID, "CRAWLING", "WAYBACK")
	}

	// 2. GAU
	if len(liveAssets) > 0 {
		if scanIsStepDone(targetID, "CRAWLING", "GAU") {
			log.Printf("⏩ Resume: skipping GAU\n")
		} else {
			if checkStopRequest(targetID) {
				return
			}
			scanMarkRunning(targetID, "CRAWLING", "GAU")
			updateTargetPhase(targetID, "PHASE 3: RUNNING GAU")
			tmp := make(map[string]string)
			runToolAndCollect(targetID, crawlingAssetsFile, tmp, "gau", "gau", "--threads", "10")
			saveCrawledURLs(targetID, rootDomain, tmp, true)
			scanMarkStepDone(targetID, "CRAWLING", "GAU")
		}
	} else {
		scanMarkStepDone(targetID, "CRAWLING", "GAU")
	}

	// 👇👇👇 3. Waymore Implementation (بخش اضافه شده)
	if target.UseWaymore {
		if scanIsStepDone(targetID, "CRAWLING", "WAYMORE") {
			log.Printf("⏩ Resume: skipping WAYMORE\n")
		} else {
			if checkStopRequest(targetID) {
				return
			}
			scanMarkRunning(targetID, "CRAWLING", "WAYMORE")
			updateTargetPhase(targetID, "PHASE 3: RUNNING WAYMORE")
			log.Printf("🔥 Running Waymore for %s...\n", rootDomain)

			// فایل خروجی موقت برای Waymore
			waymoreOutputFile := filepath.Join(tempDir, "waymore.txt")

			_, err := runCommandWithKillSwitch(targetID, "waymore", "-i", rootDomain, "-mode", "U", "-n", "-oU", waymoreOutputFile)

			tmp := make(map[string]string)
			if err != nil {
				log.Printf("⚠️ Waymore execution failed: %v\n", err)
			} else {
				content, err := ioutil.ReadFile(waymoreOutputFile)
				if err == nil {
					lines := strings.Split(string(content), "\n")
					for _, line := range lines {
						u := strings.TrimSpace(line)
						if u != "" {
							tmp[u] = "waymore"
						}
					}
					log.Printf("✅ Waymore found %d URLs.\n", len(lines))
				} else {
					log.Printf("⚠️ Could not read Waymore output file: %v\n", err)
				}
				_ = os.Remove(waymoreOutputFile)
			}
			saveCrawledURLs(targetID, rootDomain, tmp, true)
			scanMarkStepDone(targetID, "CRAWLING", "WAYMORE")
		}
	} else {
		log.Println("⏩ Skipping Waymore (Disabled in settings)")
		scanMarkStepDone(targetID, "CRAWLING", "WAYMORE")
	}
	// 👆👆👆 پایان بخش اضافه شده

	// 4. Katana
	if len(liveAssets) > 0 {
		if scanIsStepDone(targetID, "CRAWLING", "KATANA") {
			log.Printf("⏩ Resume: skipping KATANA\n")
		} else {
			if checkStopRequest(targetID) {
				return
			}
			scanMarkRunning(targetID, "CRAWLING", "KATANA")
			updateTargetPhase(targetID, "PHASE 3: RUNNING KATANA")
			tmp := make(map[string]string)
			runToolAndCollect(targetID, crawlingAssetsFile, tmp, "katana", "katana", "-list", crawlingAssetsFile, "-jc", "-kf", "-silent", "-c", "10")
			saveCrawledURLs(targetID, rootDomain, tmp, true)
			scanMarkStepDone(targetID, "CRAWLING", "KATANA")
		}
	} else {
		scanMarkStepDone(targetID, "CRAWLING", "KATANA")
	}

	// 5. VirusTotal
	if len(liveAssets) > 0 {
		if scanIsStepDone(targetID, "CRAWLING", "VIRUSTOTAL") {
			log.Printf("⏩ Resume: skipping VIRUSTOTAL\n")
		} else {
			if checkStopRequest(targetID) {
				return
			}
			scanMarkRunning(targetID, "CRAWLING", "VIRUSTOTAL")
			updateTargetPhase(targetID, "PHASE 3: RUNNING VIRUSTOTAL")
			log.Printf("🦠 Running VirusTotal URL discovery for %d live assets...\n", len(liveAssets))
			
			tmp := make(map[string]string)
			runVirusTotalCollection(targetID, liveAssets, tmp)
			saveCrawledURLs(targetID, rootDomain, tmp, true)
			
			scanMarkStepDone(targetID, "CRAWLING", "VIRUSTOTAL")
		}
	} else {
		scanMarkStepDone(targetID, "CRAWLING", "VIRUSTOTAL")
	}

	// Filtering & Saving
	if scanIsStepDone(targetID, "CRAWLING", "FILTER_SAVE") {
		log.Printf("⏩ Resume: skipping FILTER/SAVE\n")
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "CRAWLING", "FILTER_SAVE")
		updateTargetPhase(targetID, "PHASE 3: FILTERING & SAVING")
		// URLs were already saved per-tool; this step is mostly a final marker.
		scanMarkStepDone(targetID, "CRAWLING", "FILTER_SAVE")
	}

	_ = os.Remove(crawlingAssetsFile)
	_ = os.Remove(crawlingRootFile)

	log.Printf("🏁 PHASE 3 finished for %s.\n", rootDomain)
	scanMarkStepDone(targetID, "CRAWLING", "DONE")
	triggerNextModule(targetID, rootDomain, "CRAWLING")
}

func runToolAndCollect(targetID uint, inputFile string, results map[string]string, sourceLabel string, cmdName string, cmdArgs ...string) {
	var output []byte
	var err error

	if cmdName == "waybackurls" || cmdName == "gau" {
		catCmd := exec.Command("cat", inputFile)
		toolCmd := exec.Command(cmdName, cmdArgs...)
		catCmd.Env = envWithTargetTmp(targetID)
		toolCmd.Env = envWithTargetTmp(targetID)

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
	countUpdatedSource := 0

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
		// برای هر URL، بررسی می‌کنیم آیا قبلاً در دیتابیس وجود دارد یا خیر تا source را مرج کنیم
		var existingURL models.FoundURL
		result := database.DB.Where("target_id = ? AND value = ?", targetID, u).First(&existingURL)
		
		if result.Error == nil {
			// URL already exists, check/update source
			currentSources := strings.Split(existingURL.Source, ", ")
			newSource := source
			
			// Check if new source is already present
			sourceExists := false
			for _, s := range currentSources {
				if s == newSource {
					sourceExists = true
					break
				}
			}
			
			if !sourceExists {
				// Append new source
				existingURL.Source = existingURL.Source + ", " + newSource
				database.DB.Save(&existingURL)
				countUpdatedSource++
			}
			
			countSkippedCache++
			continue
		}

		// URL is new
		newUrls = append(newUrls, models.FoundURL{
			TargetID: targetID,
			Value:    u,
			Source:   source,
		})

		// 👇 افزودن به کش ردیس (اگرچه منطق بالا دیتابیس را چک می‌کند، ردیس هم برای سرعت خوب است اما اینجا منطق اصلی دیتابیس شد)
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
	
	if countUpdatedSource > 0 {
		log.Printf("🔄 Updated sources for %d existing URLs.", countUpdatedSource)
	}

	log.Printf("📊 Stats: Total Found: %d | Ignored (Ext): %d | Exists/Skipped: %d | New: %d",
		len(urls), countSkippedExt, countSkippedCache, len(newUrls))
}

// ==========================================
// Smart Storage & Notification Logic
// ==========================================

func saveDiscoveryResultsToDB(target models.Target, masterList []string, liveResults map[string][]string, sourcesMap map[string][]string) {
	log.Printf("💾 Saving/Updating assets (Scan Count: %d)...", target.ScanCount)

	isFirstRun := target.ScanCount == 0
	countNew := 0
	countUpdated := 0

	// Helper function برای merge کردن sources
	mergeSources := func(existingSourcesJSON string, newSources []string) string {
		var existingSources []string
		if existingSourcesJSON != "" && existingSourcesJSON != "[]" {
			_ = json.Unmarshal([]byte(existingSourcesJSON), &existingSources)
		}

		// ایجاد map برای unique کردن
		sourceMap := make(map[string]bool)
		for _, s := range existingSources {
			sourceMap[s] = true
		}
		for _, s := range newSources {
			sourceMap[s] = true
		}

		// تبدیل به slice و مرتب کردن
		var merged []string
		for s := range sourceMap {
			merged = append(merged, s)
		}
		sort.Strings(merged)

		bytes, _ := json.Marshal(merged)
		return string(bytes)
	}

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

			// دریافت sources برای این subdomain
			newSources := sourcesMap[val]
			sourcesJSON := "[]"
			if len(newSources) > 0 {
				bytes, _ := json.Marshal(newSources)
				sourcesJSON = string(bytes)
			}

			if existing, ok := existingMap[val]; ok {
				// Merge sources با sources موجود
				mergedSourcesJSON := mergeSources(existing.Sources, newSources)

				updateData := make(map[string]interface{})
				needsUpdate := false

				if existing.IsLive != isLive {
					now := time.Now()
					logChange(database.DB, existing.ID, "is_live", strconv.FormatBool(existing.IsLive), strconv.FormatBool(isLive))

					// اگر ساب‌دامین قبلاً در دیتابیس بود اما لایو نبود و حالا لایو شده، به عنوان fresh asset در نظر می‌گیریم
					if isLive && !existing.IsLive {
						if !isFirstRun {
							telegram.SendNewAssetAlert(target.RootDomain, val)
						}
					} else if !isLive {
						telegram.SendChangeAlert(target.RootDomain, val, "is_live", "true", "false")
					}

					updateData["is_live"] = isLive
					updateData["is_new"] = true
					updateData["last_change_at"] = &now
					updateData["dnsx_ip"] = dnsxIPJSON
					needsUpdate = true
				} else if isLive && existing.DnsxIP != dnsxIPJSON {
					updateData["dnsx_ip"] = dnsxIPJSON
					needsUpdate = true
				}

				// آپدیت sources اگر تغییر کرده
				if existing.Sources != mergedSourcesJSON {
					updateData["sources"] = mergedSourcesJSON
					needsUpdate = true
				}

				if needsUpdate {
					database.DB.Model(existing).Updates(updateData)
					countUpdated++
				}
			} else {
				// ساب‌دامین جدید: فقط اگر لایو باشد (دارای IP از dnsx) به عنوان fresh asset در نظر می‌گیریم
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
					Sources:      sourcesJSON,
				}
				toInsert = append(toInsert, newAsset)

				// فقط برای ساب‌دامین‌های جدید که لایو هستند (دارای IP از dnsx) نوتیف می‌فرستیم
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
	isFirstRun := false
	if err := database.DB.First(&target, targetID).Error; err == nil {
		targetName = target.RootDomain
		isFirstRun = target.ScanCount == 0
	}

	countUpdated := 0
	countChanges := 0

	err := database.DB.Transaction(func(tx *gorm.DB) error {
		for hostInput, result := range results {
			var asset models.Asset
			if err := tx.Set("gorm:query_option", "FOR UPDATE").Where("value = ? AND target_id = ?", hostInput, targetID).First(&asset).Error; err != nil {
				if err == gorm.ErrRecordNotFound {
					// این حالت نادر است چون Phase 2 فقط assets موجود در دیتابیس را پردازش می‌کند
					// اما اگر asset جدیدی پیدا شد و لایو است، فقط در صورتی نوتیف می‌فرستیم که اولین اجرا نباشد
					asset = models.Asset{
						TargetID: targetID, Value: hostInput, Type: "subdomain", IsNew: true, IsLive: true,
						Technologies: "[]", HostIP: "[]", RawHttpx: "{}", DnsxIP: "[]",
					}
					if err := tx.Create(&asset).Error; err != nil {
						return err
					}
					// فقط برای assets جدید که لایو هستند و اولین اجرا نیست، نوتیف می‌فرستیم
					if !isFirstRun && result.StatusCode > 0 {
						telegram.SendNewAssetAlert(targetName, hostInput)
					}
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
	cmd.Env = envWithTargetTmp(targetID)
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
			// keep scan heartbeat fresh so API doesn't treat active scans as stale
			scanHeartbeat(targetID)
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

// runCommandWithKillSwitchCombined مثل runCommandWithKillSwitch ولی stdout/stderr را با هم برمی‌گرداند (برای ابزارهایی مثل nmap)
func runCommandWithKillSwitchCombined(targetID uint, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = envWithTargetTmp(targetID)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &outBuf
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
			// keep scan heartbeat fresh so API doesn't treat active scans as stale
			scanHeartbeat(targetID)
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
		scanMarkPaused(targetID, "paused by user")
		return true
	}
	return false
}

func updateTargetPhase(targetID uint, phase string) {
	database.DB.Model(&models.Target{}).Where("id = ?", targetID).Update("current_phase", phase)
}

// =================================================================
// Persistent Scan Checkpointing (DB-backed)
// =================================================================

func ensureScanState(targetID uint) (*models.TargetScanState, error) {
	var st models.TargetScanState
	err := database.DB.Where("target_id = ?", targetID).First(&st).Error
	if err == nil {
		return &st, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	st = models.TargetScanState{
		TargetID: targetID,
		Status:   "PAUSED",
		RunID:    fmt.Sprintf("run_%d", time.Now().UnixNano()),
	}
	if err := database.DB.Create(&st).Error; err != nil {
		return nil, err
	}
	return &st, nil
}

func scanCompletedSet(completedJSON string) map[string]bool {
	out := make(map[string]bool)
	if completedJSON == "" {
		return out
	}
	var keys []string
	if err := json.Unmarshal([]byte(completedJSON), &keys); err != nil {
		return out
	}
	for _, k := range keys {
		k = strings.TrimSpace(k)
		if k != "" {
			out[k] = true
		}
	}
	return out
}

func scanCompletedList(set map[string]bool) []string {
	var keys []string
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func scanStepKey(module, step string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(module), strings.TrimSpace(step))
}

func scanMarkRunning(targetID uint, module, step string) {
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":         "RUNNING",
			"current_module": module,
			"current_step":   step,
			"heartbeat_at":   &now,
			"last_error":     "",
		}).Error
}

func scanHeartbeat(targetID uint) {
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ? AND status = ?", targetID, "RUNNING").
		Update("heartbeat_at", &now).Error
}

func scanMarkPaused(targetID uint, reason string) {
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":       "PAUSED",
			"heartbeat_at": &now,
			"last_error":   reason,
		}).Error
}

func scanMarkFailed(targetID uint, errMsg string) {
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":       "FAILED",
			"heartbeat_at": &now,
			"last_error":   errMsg,
		}).Error
}

func scanMarkStepDone(targetID uint, module, step string) {
	st, err := ensureScanState(targetID)
	if err != nil {
		return
	}
	set := scanCompletedSet(st.CompletedSteps)
	set[scanStepKey(module, step)] = true
	keys := scanCompletedList(set)
	b, _ := json.Marshal(keys)
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"completed_steps": string(b),
			"heartbeat_at":    &now,
		}).Error
}

func scanIsStepDone(targetID uint, module, step string) bool {
	st, err := ensureScanState(targetID)
	if err != nil {
		return false
	}
	set := scanCompletedSet(st.CompletedSteps)
	return set[scanStepKey(module, step)]
}

func scanGetMeta(targetID uint) map[string]interface{} {
	st, err := ensureScanState(targetID)
	if err != nil {
		return map[string]interface{}{}
	}
	out := map[string]interface{}{}
	if st.Meta != "" {
		_ = json.Unmarshal([]byte(st.Meta), &out)
	}
	return out
}

func scanSetMeta(targetID uint, meta map[string]interface{}) {
	b, _ := json.Marshal(meta)
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"meta":         string(b),
			"heartbeat_at": &now,
		}).Error
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
		_ = database.DB.Model(&models.TargetScanState{}).
			Where("target_id = ?", targetID).
			Updates(map[string]interface{}{
				"status":         "COMPLETED",
				"current_module": "",
				"current_step":   "",
				"heartbeat_at":   &now,
				"last_error":     "",
			}).Error
		// پاکسازی workspace موقت این target بعد از پایان کامل chain
		cleanupTargetTempDir(targetID)
	}
}

func logChange(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error {
	if oldVal == newVal {
		return nil
	}
	history := models.AssetHistory{AssetID: assetID, FieldName: field, OldValue: oldVal, NewValue: newVal, CreatedAt: time.Now()}
	return tx.Create(&history).Error
}

func runDnsx(targetID uint, inputFile, outputFile string) (map[string][]string, error) {
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

// normalizeSubdomain حذف www و normalize کردن subdomain
func normalizeSubdomain(subdomain, rootDomain string) string {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return ""
	}

	// تبدیل به lowercase برای مقایسه
	subdomainLower := strings.ToLower(subdomain)
	rootDomainLower := strings.ToLower(rootDomain)

	// حذف www. از ابتدای subdomain (case-insensitive)
	subdomainLower = strings.TrimPrefix(subdomainLower, "www.")

	// اگر subdomain برابر rootDomain باشد، رد می‌کنیم (فقط subdomain می‌خواهیم)
	if subdomainLower == rootDomainLower {
		return ""
	}

	// اطمینان از اینکه subdomain واقعا زیرمجموعه‌ی rootDomain است (مرز نقطه)
	// مثال valid: api.example.com
	// مثال invalid: api.notexample.com (صرفا به خاطر suffix نباید قبول شود)
	if !strings.HasSuffix(subdomainLower, "."+rootDomainLower) {
		return ""
	}

	return subdomainLower
}

// writeSubfinderProviderConfigFile builds a per-user provider-config.yaml for subfinder and writes it
// into the current target temp directory. Returns empty string if the user has no provider entries.
func writeSubfinderProviderConfigFile(targetID uint, userID uint) (string, error) {
	if userID == 0 {
		return "", nil
	}

	var rows []models.SubfinderProviderConfig
	if err := database.DB.
		Where("user_id = ?", userID).
		Order("provider asc").
		Find(&rows).Error; err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}

	tempDir, _, err := getTargetTempDir(targetID)
	if err != nil {
		return "", err
	}

	// Start from subfinder defaults so unknown providers don't break anything and we keep a stable schema.
	cfg := make(map[string]interface{}, len(subfinderProviderKeys)+len(rows))
	for _, k := range subfinderProviderKeys {
		cfg[k] = []interface{}{}
	}

	for _, r := range rows {
		p := strings.ToLower(strings.TrimSpace(r.Provider))
		if p == "" {
			continue
		}
		var entries []interface{}
		if strings.TrimSpace(r.Entries) != "" {
			_ = json.Unmarshal([]byte(r.Entries), &entries)
		}
		if entries == nil {
			entries = []interface{}{}
		}
		cfg[p] = entries
	}

	yml, err := yaml.Marshal(cfg)
	if err != nil {
		return "", err
	}

	outPath := filepath.Join(tempDir, fmt.Sprintf("subfinder_provider-config_u%d.yaml", userID))
	if err := os.WriteFile(outPath, yml, 0600); err != nil {
		return "", err
	}
	return outPath, nil
}

// runPuredns اجرای puredns برای bruteforce subdomain discovery
// این تابع subdomain‌های لایو (resolve شده) با IP‌هایشان را برمی‌گرداند
// خروجی: map[string][]string که key=subdomain و value=[]IP
func runPuredns(targetID uint, rootDomain string, wordlists []string) (map[string][]string, error) {
	if len(wordlists) == 0 {
		return map[string][]string{}, nil
	}

	tempDir, _, err := getTargetTempDir(targetID)
	if err != nil {
		return nil, fmt.Errorf("failed to get temp dir: %v", err)
	}

	// ساخت فایل resolvers.txt از TRUSTED_RESOLVERS
	resolversFile := filepath.Join(tempDir, "puredns_resolvers.txt")
	resolversStr := os.Getenv("TRUSTED_RESOLVERS")
	if resolversStr == "" {
		resolversStr = "1.1.1.1,8.8.8.8,1.0.0.1,8.8.4.4" // fallback
	}
	resolvers := strings.Split(resolversStr, ",")
	var cleanResolvers []string
	for _, r := range resolvers {
		r = strings.TrimSpace(r)
		if r != "" {
			cleanResolvers = append(cleanResolvers, r)
		}
	}
	if err := writeSliceToFile(resolversFile, cleanResolvers); err != nil {
		return nil, fmt.Errorf("failed to write resolvers file: %v", err)
	}

	// ساخت یک فایل موقت برای ترکیب همه وردلیست‌ها
	combinedWordlistFile := filepath.Join(tempDir, "puredns_combined_wordlist.txt")
	var allWords []string
	wordSet := make(map[string]bool)

	for _, wlPath := range wordlists {
		// بررسی وجود فایل
		if _, err := os.Stat(wlPath); os.IsNotExist(err) {
			log.Printf("⚠️ Wordlist not found: %s, skipping...\n", wlPath)
			continue
		}

		// خواندن وردلیست
		content, err := os.ReadFile(wlPath)
		if err != nil {
			log.Printf("⚠️ Failed to read wordlist %s: %v, skipping...\n", wlPath, err)
			continue
		}

		// پارس کردن خطوط
		for _, line := range strings.Split(string(content), "\n") {
			word := strings.TrimSpace(line)
			if word != "" && !strings.HasPrefix(word, "#") && !wordSet[word] {
				wordSet[word] = true
				allWords = append(allWords, word)
			}
		}
	}

	if len(allWords) == 0 {
		log.Printf("⚠️ No words found in wordlists, skipping puredns\n")
		return map[string][]string{}, nil
	}

	// نوشتن وردلیست ترکیبی
	if err := writeSliceToFile(combinedWordlistFile, allWords); err != nil {
		return nil, fmt.Errorf("failed to write combined wordlist: %v", err)
	}

	log.Printf("🔨 Running puredns bruteforce with %d words for %s...\n", len(allWords), rootDomain)

	// اجرای puredns
	// نکته: -r برای public resolvers است. برای اینکه نیاز به فایل default trusted resolvers نداشته باشیم،
	// از --trusted-only استفاده می‌کنیم تا فقط همین resolvers استفاده شوند.
	// puredns bruteforce wordlist.txt domain.com -r resolvers.txt --trusted-only --quiet
	output, err := runCommandWithKillSwitch(targetID, "puredns", "bruteforce", combinedWordlistFile, rootDomain, "-r", resolversFile, "--trusted-only", "--quiet")
	if err != nil {
		if err.Error() == "process killed by user request" {
			return nil, err
		}
		log.Printf("⚠️ Puredns error: %v\n", err)
		return map[string][]string{}, nil // در صورت خطا، map خالی برمی‌گردانیم
	}

	// پارس کردن خروجی (هر خط یک subdomain لایو است)
	subdomains := []string{}
	for _, line := range strings.Split(string(output), "\n") {
		normalized := normalizeSubdomain(strings.TrimSpace(line), rootDomain)
		if normalized != "" {
			subdomains = append(subdomains, normalized)
		}
	}

	// حالا باید IP‌های این subdomain‌ها را resolve کنیم
	// استفاده از dnsx برای resolve کردن IP‌ها
	results := make(map[string][]string)
	if len(subdomains) > 0 {
		// ساخت فایل موقت برای dnsx
		purednsInputFile := filepath.Join(tempDir, "puredns_dnsx_input.txt")
		if err := writeSliceToFile(purednsInputFile, subdomains); err != nil {
			log.Printf("⚠️ Failed to write puredns input for dnsx: %v\n", err)
			// در صورت خطا، subdomain‌ها را بدون IP برمی‌گردانیم
			for _, subdomain := range subdomains {
				results[subdomain] = []string{} // IP خالی، اما subdomain لایو است
			}
		} else {
			// اجرای dnsx برای resolve کردن IP‌ها
			purednsDnsxOutputFile := filepath.Join(tempDir, "puredns_dnsx_output.json")
			dnsxResults, dnsxErr := runDnsx(targetID, purednsInputFile, purednsDnsxOutputFile)
			if dnsxErr == nil {
				results = dnsxResults
			} else {
				// در صورت خطا، subdomain‌ها را بدون IP برمی‌گردانیم
				for _, subdomain := range subdomains {
					results[subdomain] = []string{} // IP خالی، اما subdomain لایو است
				}
			}
			_ = os.Remove(purednsInputFile)
			_ = os.Remove(purednsDnsxOutputFile)
		}
	}

	log.Printf("✅ Puredns found %d live subdomains for %s\n", len(results), rootDomain)

	// پاکسازی فایل‌های موقت
	_ = os.Remove(combinedWordlistFile)
	_ = os.Remove(resolversFile)

	return results, nil
}

// runVirusTotalCollection collects URLs from VirusTotal API for live subdomains
func runVirusTotalCollection(targetID uint, subdomains []string, results map[string]string) {
	apiKey := "183e25c8551f61932c61c190f8d2fc4667b82e954ada72ccef145cb4075a005e" // TODO: Move to config
	client := &http.Client{Timeout: 30 * time.Second}

	for _, domain := range subdomains {
		if checkStopRequest(targetID) {
			return
		}

		url := fmt.Sprintf("https://virustotal.com/vtapi/v2/domain/report?apikey=%s&domain=%s", apiKey, domain)
		resp, err := client.Get(url)
		if err != nil {
			log.Printf("⚠️ VirusTotal API error for %s: %v\n", domain, err)
			continue
		}

		if resp.StatusCode != 200 {
			// Don't log 404s/204s too noisily, but other errors might be interesting
			if resp.StatusCode != 204 { // 204 = quota exceeded
				log.Printf("⚠️ VirusTotal API status %d for %s\n", resp.StatusCode, domain)
			}
			resp.Body.Close()
			continue
		}

		var vtResp struct {
			DetectedUrls   []interface{} `json:"detected_urls"`
			UndetectedUrls []interface{} `json:"undetected_urls"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&vtResp); err != nil {
			log.Printf("⚠️ Failed to decode VT response for %s: %v\n", domain, err)
			resp.Body.Close()
			continue
		}
		resp.Body.Close()

		// Helper to extract URLs from the nested arrays
		// detected_urls structure: [ [url, sha256, positives, total, date], ... ]
		processUrls := func(list []interface{}) {
			for _, item := range list {
				if arr, ok := item.([]interface{}); ok && len(arr) > 0 {
					if u, ok := arr[0].(string); ok && u != "" {
						u = strings.TrimSpace(u)
						if _, exists := results[u]; !exists {
							results[u] = "virustotal"
						}
					}
				}
			}
		}

		processUrls(vtResp.DetectedUrls)
		processUrls(vtResp.UndetectedUrls)
		
		// Respect rate limits - VT public API has 4 req/min limit usually, but key provided might be premium or standard.
		// Adding a small sleep just in case to be safe, though parallel workers might be better handled globally.
		time.Sleep(500 * time.Millisecond) 
	}
}

// SubdomainSource نگه‌داری subdomain و source آن
type SubdomainSource struct {
	Subdomain string
	Source    string
}

// runPassiveCollection جمع‌آوری اطلاعات غیرفعال (Passive) از منابع مختلف
func runPassiveCollection(targetID uint, domain string) ([]string, map[string][]string) {
	// دریافت تنظیمات target
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		log.Printf("⚠️ Failed to fetch target config: %v\n", err)
		// استفاده از تنظیمات پیش‌فرض
		target.UseCero = false
		target.UseCrtsh = false
	}

	// Build per-owner subfinder provider-config (API keys) if present
	subfinderPC, err := writeSubfinderProviderConfigFile(targetID, target.CreatedByUserID)
	if err != nil {
		log.Printf("⚠️ Failed to build subfinder provider-config (user_id=%d): %v\n", target.CreatedByUserID, err)
		subfinderPC = ""
	}
	if subfinderPC != "" {
		// do not log file contents (API keys)
		log.Printf("🔑 Subfinder provider-config loaded for user_id=%d\n", target.CreatedByUserID)
		defer func() { _ = os.Remove(subfinderPC) }()
	}

	var wg sync.WaitGroup
	results := make(chan SubdomainSource, 50000)

	runTool := func(name string, args ...string) {
		defer wg.Done()
		output, err := runCommandWithKillSwitch(targetID, name, args...)
		if err == nil {
			for _, line := range strings.Split(string(output), "\n") {
				normalized := normalizeSubdomain(strings.TrimSpace(line), domain)
				if normalized != "" {
					results <- SubdomainSource{Subdomain: normalized, Source: name}
				}
			}
		} else {
			log.Printf("❌ %s error/killed: %v\n", name, err)
		}
	}

	// 1. ابزارهای همیشگی (Subfinder, Assetfinder)
	wg.Add(2)
	go func() {
		args := []string{"-d", domain, "-silent", "-all"}
		if subfinderPC != "" {
			args = append(args, "-pc", subfinderPC)
		}
		runTool("subfinder", args...)
	}()
	go runTool("assetfinder", "--subs-only", domain)

	// 2. ابزارهای انتخابی (Cero)
	if target.UseCero {
		wg.Add(1)
		log.Printf("🔐 Starting CERO for %s (SSL certificate scraping)...\n", domain)
		go func() {
			defer wg.Done()
			ceroResults := runCero(targetID, domain)
			log.Printf("✅ CERO found %d domains for %s\n", len(ceroResults), domain)
			for _, res := range ceroResults {
				normalized := normalizeSubdomain(res, domain)
				if normalized != "" {
					results <- SubdomainSource{Subdomain: normalized, Source: "cero"}
				}
			}
		}()
	} else {
		log.Printf("⏩ Skipping CERO for %s (Disabled in settings)\n", domain)
	}

	// 3. ابزارهای انتخابی (Crt.sh)
	if target.UseCrtsh {
		wg.Add(1)
		log.Printf("🔍 Starting CRT.SH API query for %s...\n", domain)
		go func() {
			defer wg.Done()
			crtshResults := runCrtsh(domain)
			log.Printf("✅ CRT.SH found %d domains for %s\n", len(crtshResults), domain)
			for _, res := range crtshResults {
				normalized := normalizeSubdomain(res, domain)
				if normalized != "" {
					results <- SubdomainSource{Subdomain: normalized, Source: "crtsh"}
				}
			}
		}()
	} else {
		log.Printf("⏩ Skipping CRT.SH for %s (Disabled in settings)\n", domain)
	}

	// 4. ابزار اختیاری (AbuseDB)
	if target.UseAbusedb {
		wg.Add(1)
		log.Printf("😈 Starting AbuseDB scraping for %s...\n", domain)
		go func() {
			defer wg.Done()
			abuseResults := runAbuseDB(domain)
			for _, res := range abuseResults {
				normalized := normalizeSubdomain(res, domain)
				if normalized != "" {
					results <- SubdomainSource{Subdomain: normalized, Source: "abusedb"}
				}
			}
		}()
	} else {
		log.Printf("⏩ Skipping AbuseDB for %s (Disabled in settings)\n", domain)
	}

	// بستن کانال وقتی همه تمام شدند
	go func() { wg.Wait(); close(results) }()

	// جمع‌آوری نتایج در Map برای حذف تکراری‌ها و ادغام منابع
	sourcesMap := make(map[string]map[string]bool)
	var finalSlice []string

	for res := range results {
		if res.Subdomain != "" && strings.HasSuffix(res.Subdomain, domain) {
			// اضافه کردن subdomain به لیست
			if _, exists := sourcesMap[res.Subdomain]; !exists {
				sourcesMap[res.Subdomain] = make(map[string]bool)
				finalSlice = append(finalSlice, res.Subdomain)
			}
			// اضافه کردن source
			sourcesMap[res.Subdomain][res.Source] = true
		}
	}

	// تبدیل map[string]bool به []string برای هر ساب‌دامین
	sourcesListMap := make(map[string][]string)
	for subdomain, sourcesSet := range sourcesMap {
		var sources []string
		for source := range sourcesSet {
			sources = append(sources, source)
		}
		sort.Strings(sources) // مرتب کردن برای consistency
		sourcesListMap[subdomain] = sources
	}

	return finalSlice, sourcesListMap
}

// ---------------------------------------------------------
// این تابع را در انتهای فایل runner.go اضافه کنید
// ---------------------------------------------------------

func runAbuseDB(rootDomain string) []string {
	scriptPath := "/root/hunt-engine/backend/scripts/abusedb.sh"
	// اطمینان از وجود فایل
	if _, err := os.Stat(scriptPath); os.IsNotExist(err) {
		log.Printf("❌ AbuseDB script not found at %s\n", scriptPath)
		return []string{}
	}

	cmd := exec.Command("/bin/bash", scriptPath, rootDomain)

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("❌ AbuseDB Script Error: %v\nOutput: %s\n", err, string(output))
		return []string{}
	}

	var results []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			results = append(results, line)
		}
	}

	if len(results) > 0 {
		log.Printf("😈 AbuseDB script found %d subdomains for %s\n", len(results), rootDomain)
	} else {
		log.Printf("⏩ AbuseDB script produced no results for %s\n", rootDomain)
	}

	return results
}

// runCero اجرای ابزار cero برای scrape کردن domain names از SSL certificates
func runCero(targetID uint, domain string) []string {
	output, err := runCommandWithKillSwitch(targetID, "cero", "-d", domain)
	if err != nil {
		log.Printf("⚠️ CERO error/killed: %v\n", err)
		return []string{}
	}

	var results []string
	for _, line := range strings.Split(string(output), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			results = append(results, trimmed)
		}
	}
	if len(results) > 0 {
		log.Printf("🔐 CERO scraped %d domains from SSL certificates\n", len(results))
	}
	return results
}

// runCrtsh استفاده از crt.sh API برای پیدا کردن subdomain ها
func runCrtsh(rootDomain string) []string {
	// ساخت URL برای crt.sh
	url1 := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", rootDomain)
	url2 := fmt.Sprintf("https://crt.sh/?q=%s&output=json", rootDomain)

	var allResults []string

	// درخواست اول: با wildcard
	log.Printf("🔍 Querying crt.sh API (wildcard): %s\n", url1)
	if data, err := fetchCrtshData(url1); err == nil {
		parsed := parseCrtshJSON(data, rootDomain)
		allResults = append(allResults, parsed...)
		log.Printf("✅ crt.sh wildcard query returned %d domains\n", len(parsed))
	} else {
		log.Printf("⚠️ crt.sh wildcard query failed: %v\n", err)
	}

	// درخواست دوم: بدون wildcard
	log.Printf("🔍 Querying crt.sh API (exact): %s\n", url2)
	if data, err := fetchCrtshData(url2); err == nil {
		parsed := parseCrtshJSON(data, rootDomain)
		allResults = append(allResults, parsed...)
		log.Printf("✅ crt.sh exact query returned %d domains\n", len(parsed))
	} else {
		log.Printf("⚠️ crt.sh exact query failed: %v\n", err)
	}

	// Unique کردن نتایج
	uniqueMap := make(map[string]bool)
	var finalResults []string
	for _, res := range allResults {
		res = strings.ToLower(strings.TrimSpace(res))
		if res != "" && !uniqueMap[res] {
			uniqueMap[res] = true
			finalResults = append(finalResults, res)
		}
	}

	log.Printf("🔍 CRT.SH total unique domains: %d\n", len(finalResults))
	return finalResults
}

// fetchCrtshData دریافت داده از crt.sh API
func fetchCrtshData(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crt.sh API returned status %d", resp.StatusCode)
	}

	return ioutil.ReadAll(resp.Body)
}

// parseCrtshJSON پارس کردن JSON از crt.sh و استخراج domain names
func parseCrtshJSON(data []byte, rootDomain string) []string {
	var results []string
	var jsonData []map[string]interface{}

	if err := json.Unmarshal(data, &jsonData); err != nil {
		return results
	}

	for _, item := range jsonData {
		// استخراج common_name
		if cn, ok := item["common_name"].(string); ok && cn != "" {
			// حذف wildcard
			cn = strings.TrimPrefix(cn, "*.")
			cn = strings.TrimPrefix(cn, "\\*.")
			results = append(results, cn)
		}

		// استخراج name_value (می‌تواند یک string یا array باشد)
		if nv, ok := item["name_value"].(string); ok && nv != "" {
			// name_value می‌تواند چند خطی باشد
			for _, line := range strings.Split(nv, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					line = strings.TrimPrefix(line, "*.")
					results = append(results, line)
				}
			}
		}
	}

	// فیلتر کردن بر اساس rootDomain
	var filtered []string
	for _, res := range results {
		res = strings.ToLower(strings.TrimSpace(res))
		// بررسی اینکه با rootDomain تمام می‌شود
		if strings.HasSuffix(res, rootDomain) {
			filtered = append(filtered, res)
		}
	}

	return filtered
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

func readSliceFromFile(filename string) ([]string, error) {
	b, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	var out []string
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if l != "" {
			out = append(out, l)
		}
	}
	return out, nil
}

func writeJSONToFile(filename string, v interface{}) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return os.WriteFile(filename, b, 0o644)
}

func readJSONFromFile(filename string, v interface{}) error {
	b, err := os.ReadFile(filename)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// CdnCheckResult ساختار نتیجه cdncheck
type CdnCheckResult struct {
	IP        string `json:"ip"`
	IsCDN     bool   `json:"cdn"`
	CDNName   string `json:"cdn_name"`
	IsWAF     bool   `json:"waf"`
	WAFName   string `json:"waf_name"`
	IsCloud   bool   `json:"cloud"`
	CloudName string `json:"cloud_name"`
}

// runCdnCheckForLiveAssets چک کردن CDN برای همه IPهای لایو که dnsx resolve کرده
func runCdnCheckForLiveAssets(targetID uint, dnsxResults map[string][]string, inputFile string) {
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
	cdnResults := runCdnCheck(targetID, ipList, inputFile)
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

	// به‌روزرسانی assets با اطلاعات CDN/WAF/CLOUD از cdncheck
	// برای هر asset، اگر حداقل یکی از IPهایش پشت CDN/WAF/CLOUD باشد، فیلدهای مربوطه را ذخیره می‌کنیم
	updatedCount := 0

	for subdomain, ips := range dnsxResults {
		asset, exists := assetMap[subdomain]
		if !exists {
			continue
		}

		hasCDN := false
		cdnName := ""
		hasWAF := false
		wafName := ""
		hasCloud := false
		cloudName := ""

		// چک کردن همه IPهای این asset
		for _, ip := range ips {
			if cdnInfo, found := cdnResults[ip]; found {
				if cdnInfo.IsCDN {
					hasCDN = true
					if cdnInfo.CDNName != "" && cdnName == "" {
						cdnName = cdnInfo.CDNName
					}
				}
				if cdnInfo.IsWAF {
					hasWAF = true
					if cdnInfo.WAFName != "" && wafName == "" {
						wafName = cdnInfo.WAFName
					}
				}
				if cdnInfo.IsCloud {
					hasCloud = true
					if cdnInfo.CloudName != "" && cloudName == "" {
						cloudName = cdnInfo.CloudName
					}
				}
			}
		}

		// به‌روزرسانی فیلدهای cdncheck/cdncheck_name و wafcheck/wafcheck_name و cloudcheck/cloudcheck_name
		updates := make(map[string]interface{})
		if asset.Cdncheck != hasCDN {
			updates["cdncheck"] = hasCDN
		}
		if hasCDN && asset.CdncheckName != cdnName {
			updates["cdncheck_name"] = cdnName
		} else if !hasCDN && asset.CdncheckName != "" {
			updates["cdncheck_name"] = ""
		}

		if asset.Wafcheck != hasWAF {
			updates["wafcheck"] = hasWAF
		}
		if hasWAF && asset.WafcheckName != wafName {
			updates["wafcheck_name"] = wafName
		} else if !hasWAF && asset.WafcheckName != "" {
			updates["wafcheck_name"] = ""
		}

		if asset.Cloudcheck != hasCloud {
			updates["cloudcheck"] = hasCloud
		}
		if hasCloud && asset.CloudcheckName != cloudName {
			updates["cloudcheck_name"] = cloudName
		} else if !hasCloud && asset.CloudcheckName != "" {
			updates["cloudcheck_name"] = ""
		}

		if len(updates) > 0 {
			database.DB.Model(asset).Updates(updates)
			updatedCount++
		}
	}

	log.Printf("✅ CDN check completed. Updated %d assets with cdncheck information.\n", updatedCount)
}

// runCdnCheck اجرای cdncheck روی لیست IPها
func runCdnCheck(targetID uint, ips []string, inputFile string) map[string]CdnCheckResult {
	if len(ips) == 0 {
		return make(map[string]CdnCheckResult)
	}

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
		"-silent",
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

		// merge if we already saw this ip with another detection type
		result := results[ip]
		result.IP = ip

		// CDN
		if cdnVal, exists := rawResult["cdn"]; exists {
			switch v := cdnVal.(type) {
			case bool:
				result.IsCDN = result.IsCDN || v
			case string:
				result.IsCDN = result.IsCDN || (v == "true" || v == "1" || v == "yes")
			}
		}
		if cdnName, ok := rawResult["cdn_name"].(string); ok && cdnName != "" && result.CDNName == "" {
			result.CDNName = cdnName
		}

		// WAF
		if wafVal, exists := rawResult["waf"]; exists {
			switch v := wafVal.(type) {
			case bool:
				result.IsWAF = result.IsWAF || v
			case string:
				result.IsWAF = result.IsWAF || (v == "true" || v == "1" || v == "yes")
			}
		}
		if wafName, ok := rawResult["waf_name"].(string); ok && wafName != "" && result.WAFName == "" {
			result.WAFName = wafName
		}

		// CLOUD
		if cloudVal, exists := rawResult["cloud"]; exists {
			switch v := cloudVal.(type) {
			case bool:
				result.IsCloud = result.IsCloud || v
			case string:
				result.IsCloud = result.IsCloud || (v == "true" || v == "1" || v == "yes")
			}
		}
		if cloudName, ok := rawResult["cloud_name"].(string); ok && cloudName != "" && result.CloudName == "" {
			result.CloudName = cloudName
		}

		// ذخیره همه نتایج (چه CDN باشد چه نباشد) تا بتوانیم تشخیص دهیم
		results[ip] = result
		if result.IsCDN || result.IsWAF || result.IsCloud {
			parsedCount++
		}
	}

	if parsedCount > 0 {
		log.Printf("✅ Parsed %d technology results from cdncheck output.\n", parsedCount)
	}

	return results
}

// =================================================================
// Optional Port Scan (Nmap) - PHASE 1
// =================================================================

// runPortScanForLiveNoCDNAssets فقط روی IPهای DNSX برای assetهای لایو و بدون CDN پورت‌اسکن انجام می‌دهد
// و نتیجه را روی فیلد open_ports هر Asset (به صورت map[ip][]ports) ذخیره می‌کند.
func runPortScanForLiveNoCDNAssets(targetID uint, inputFile string) {
	// معیار بدون CDN (همان منطق no_cdn در API)
	var assets []models.Asset
	if err := database.DB.Model(&models.Asset{}).
		Where("target_id = ?", targetID).
		Where("is_live = ?", true).
		Where("jsonb_array_length(dnsx_ip) > 0").
		Where("(cdn_name IS NULL OR cdn_name = '' OR cdn_name = 'null')").
		Where("cdncheck = ?", false).
		Where("(cdncheck_name IS NULL OR cdncheck_name = '' OR cdncheck_name = 'null')").
		Find(&assets).Error; err != nil {
		log.Printf("❌ Port scan: failed to fetch eligible assets: %v\n", err)
		return
	}

	if len(assets) == 0 {
		log.Printf("ℹ️ Port scan: no eligible assets (live + dnsx + no CDN).\n")
		return
	}

	// collect unique IPs from dnsx_ip
	ipSet := make(map[string]struct{})
	for _, a := range assets {
		if a.DnsxIP == "" || a.DnsxIP == "[]" || a.DnsxIP == "null" {
			continue
		}
		var ips []string
		if err := json.Unmarshal([]byte(a.DnsxIP), &ips); err != nil {
			continue
		}
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip == "" {
				continue
			}
			// فعلاً فقط IPv4
			if strings.Contains(ip, ":") {
				continue
			}
			ipSet[ip] = struct{}{}
		}
	}

	if len(ipSet) == 0 {
		log.Printf("ℹ️ Port scan: no IPv4 IPs extracted from dnsx.\n")
		return
	}

	ipList := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ipList = append(ipList, ip)
	}
	sort.Strings(ipList)

	results, err := runNmapTopPorts(targetID, ipList, inputFile)
	if err != nil {
		if err.Error() == "process killed by user request" {
			return
		}
		log.Printf("⚠️ Port scan: nmap finished with error: %v\n", err)
	}

	updated := 0
	// update assets with per-ip ports mapping
	for i := range assets {
		a := &assets[i]
		var ips []string
		_ = json.Unmarshal([]byte(a.DnsxIP), &ips)
		m := make(map[string][]int)
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip == "" || strings.Contains(ip, ":") {
				continue
			}
			if ports, ok := results[ip]; ok && len(ports) > 0 {
				m[ip] = ports
			}
		}

		// store {} if nothing found
		b, _ := json.Marshal(m)
		openPortsJSON := string(b)
		if openPortsJSON == "" || openPortsJSON == "null" {
			openPortsJSON = "{}"
		}

		if a.OpenPorts != openPortsJSON {
			if err := database.DB.Model(&models.Asset{}).Where("id = ?", a.ID).Update("open_ports", openPortsJSON).Error; err == nil {
				updated++
			}
		}
	}

	if updated > 0 {
		cache.IncrementTargetVersion(targetID)
	}

	log.Printf("✅ Port scan completed. Updated %d assets with open ports.\n", updated)
}

// runNmapTopPorts یک اسکن سریع و بهینه برای top ports انجام می‌دهد و فقط پورت‌های open را برمی‌گرداند.
func runNmapTopPorts(targetID uint, ips []string, inputFile string) (map[string][]int, error) {
	results := make(map[string][]int)
	if len(ips) == 0 {
		return results, nil
	}
	if err := writeSliceToFile(inputFile, ips); err != nil {
		return results, err
	}

	// Nmap command (optimized):
	// -n: no DNS, -Pn: skip host discovery, --open: only open ports
	// --top-ports 1000: fast/common ports, -T4: speed
	// --max-retries 2, --host-timeout 45s: bound runtime, --min-rate 300: push packets
	// -oG -: greppable output to stdout for easy parsing
	out, err := runCommandWithKillSwitchCombined(targetID, "nmap",
		"-n", "-Pn",
		"-sT",
		"--open",
		"--top-ports", "1000",
		"-T4",
		"--max-retries", "2",
		"--host-timeout", "45s",
		"--min-rate", "300",
		"-iL", inputFile,
		"-oG", "-",
	)
	if len(out) > 0 {
		results = parseNmapGreppable(out)
	}
	// nmap may return non-zero if some hosts down; we still keep parsed results
	return results, err
}

func parseNmapGreppable(out []byte) map[string][]int {
	results := make(map[string][]int)
	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if !strings.HasPrefix(line, "Host:") {
			continue
		}
		// Example:
		// Host: 1.2.3.4 ()	Ports: 80/open/tcp//http///, 443/open/tcp//https///
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		ip := parts[1]
		if ip == "" {
			continue
		}
		portsIdx := strings.Index(line, "Ports:")
		if portsIdx == -1 {
			continue
		}
		portsPart := strings.TrimSpace(line[portsIdx+len("Ports:"):])
		if portsPart == "" {
			continue
		}
		// cut off trailing metadata
		if i := strings.Index(portsPart, "Ignored"); i != -1 {
			portsPart = strings.TrimSpace(portsPart[:i])
		}
		if i := strings.Index(portsPart, "OS:"); i != -1 {
			portsPart = strings.TrimSpace(portsPart[:i])
		}
		portItems := strings.Split(portsPart, ",")
		portSet := make(map[int]struct{})
		for _, item := range portItems {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			// "80/open/tcp//http///"
			s := strings.Split(item, "/")
			if len(s) < 2 {
				continue
			}
			if s[1] != "open" {
				continue
			}
			p, err := strconv.Atoi(s[0])
			if err != nil || p <= 0 {
				continue
			}
			portSet[p] = struct{}{}
		}
		if len(portSet) == 0 {
			continue
		}
		ports := make([]int, 0, len(portSet))
		for p := range portSet {
			ports = append(ports, p)
		}
		sort.Ints(ports)
		results[ip] = ports
	}
	return results
}
