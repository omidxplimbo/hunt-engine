package worker

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/config"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/discovery"
	workerengine "github.com/omidxplimbo/hunt-engine/backend/internal/worker/engine"
	workerfindings "github.com/omidxplimbo/hunt-engine/backend/internal/worker/findings"
	workerhelpers "github.com/omidxplimbo/hunt-engine/backend/internal/worker/helpers"
	crawlingphase "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/crawling"
	discoverymerge "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/discovery/merge"
	passivetools "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/discovery/passive"
	discoverypersist "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/discovery/persistence"
	discoverytools "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/discovery/tools"
	probing "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/probing"
	probingpersist "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/probing/persistence"
	jsintelphase "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/security/jsintel"
	nucleiphase "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/security/nuclei"
	takeoverphase "github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/security/takeover"
	workerruntime "github.com/omidxplimbo/hunt-engine/backend/internal/worker/runtime"
	workerstate "github.com/omidxplimbo/hunt-engine/backend/internal/worker/state"
	workertypes "github.com/omidxplimbo/hunt-engine/backend/internal/worker/types"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
	"gorm.io/gorm"
)

const DefaultBatchSize = 500
const DefaultDNSXBatchSize = 5000

// --- Monitor Logic ---
func GetRunningTasks() []workerruntime.TaskInfo {
	return workerruntime.GetRunningTasks()
}

// HttpxResult ساختار برای پارس کردن خروجی JSON
type HttpxResult = workertypes.HTTPXResult

// DnsxResult ساختار خروجی JSON ابزار dnsx
type DnsxResult = workertypes.DNSXResult

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

	// Clear stale locks from previous crashes/restarts
	workerengine.ClearAllLocks()

	// Best-effort cleanup of legacy temp dirs created by tools (e.g. httpxXXXX) under /tmp.
	// With TMPDIR redirected to per-target workspace, new ones should not be created.
	workerruntime.CleanupLegacyToolTmp()

	for {
		// 1. Check Max Concurrent
		max := config.GetMaxConcurrentScans()
		current := workerengine.CountActiveScans()

		if current >= int64(max) {
			// If max reached, wait a bit before checking again
			time.Sleep(5 * time.Second)
			continue
		}

		// 2. Pop from Redis (with timeout to allow re-checking concurrency)
		payload, err := workerengine.PopNextEligiblePayload(max)
		if err != nil {
			log.Printf("❌ Error selecting Redis job: %v\n", err)
			time.Sleep(5 * time.Second)
			continue
		}
		if payload == "" {
			time.Sleep(1 * time.Second)
			continue
		}

		// CRITICAL FIX: Update status to SCANNING synchronously before spawning
		// the goroutine. This prevents the queue selector from overbooking
		// global or per-user scan slots.
		workerengine.MarkPayloadTargetScanning(payload)

		go processJobDispatcher(payload)
	}
}

func processJobDispatcher(payload string) {
	workerengine.ProcessJob(payload, workerengine.AcquireTargetLock, workerengine.JobHandlers{
		Discovery: runDiscoveryPhase,
		Probing:   runProbingPhase,
		Crawling:  runCrawlingPhase,
	})
}

// =================================================================
// PHASE 1: Discovery Implementation
// =================================================================
func runDiscoveryPhase(targetID uint, rootDomain string) {
	log.Printf("🚀 Starting PHASE 1 (DISCOVERY) for: %s (ID: %d)\n", rootDomain, targetID)

	tempDir, username, err := utils.GetTargetTempDir(targetID)
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
	alterxLiveInputFile := filepath.Join(tempDir, "alterx_live_input.txt")
	alterxDNSXOutputFile := filepath.Join(tempDir, "alterx_dnsx_output.json")
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
		passiveResults, _ = utils.ReadSliceFromFile(passiveResultsFile)
		_ = utils.ReadJSONFromFile(passiveSourcesFile, &passiveSources)
		log.Printf("⏩ Resume: skipping PASSIVE ENUM (loaded %d results from checkpoint)\n", len(passiveResults))
	} else {
		if checkStopRequest(targetID) {
			return
		}
		scanMarkRunning(targetID, "DISCOVERY", "PASSIVE_ENUM")
		updateTargetPhase(targetID, "PHASE 1: PASSIVE ENUM")
		var err error
		passiveResults, passiveSources, err = runPassiveCollection(targetID, rootDomain)
		if err != nil && err.Error() == "process killed by user request" {
			return
		}
		if err := utils.WriteSliceToFile(passiveResultsFile, passiveResults); err != nil {
			log.Printf("❌ Failed to write passive results checkpoint: %v\n", err)
			scanMarkFailed(targetID, fmt.Sprintf("passive checkpoint failed: %v", err))
			return
		}
		if err := utils.WriteJSONToFile(passiveSourcesFile, passiveSources); err != nil {
			log.Printf("❌ Failed to write passive sources checkpoint: %v\n", err)
			scanMarkFailed(targetID, fmt.Sprintf("passive sources checkpoint failed: %v", err))
			return
		}
		scanMarkStepDone(targetID, "DISCOVERY", "PASSIVE_ENUM")
	}

	// 2. Mutation (AlterX) is deferred until after the first DNSX validation.
	// Running AlterX against raw passive results can explode on large targets; it
	// should mutate only live/resolved hosts and then be validated by DNSX again.
	alterxCandidateCount := 0
	mutatedSources := make(map[string][]string)

	// 2.5. Puredns Bruteforce (فقط subdomain‌های لایو)
	var purednsResults map[string][]string
	purednsSources := make(map[string][]string)
	if scanIsStepDone(targetID, "DISCOVERY", "PUREDNS") {
		purednsResults = map[string][]string{}
		_ = utils.ReadJSONFromFile(purednsResultsFile, &purednsResults)
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
		_ = utils.WriteJSONToFile(purednsResultsFile, purednsResults)
		scanMarkStepDone(targetID, "DISCOVERY", "PUREDNS")
	} else {
		log.Printf("⏩ Skipping Puredns for %s (disabled in target config)\n", rootDomain)
		purednsResults = map[string][]string{}
		_ = utils.WriteJSONToFile(purednsResultsFile, purednsResults)
		scanMarkStepDone(targetID, "DISCOVERY", "PUREDNS")
	}

	// 3. Merge & History (writes DNSX candidate file without keeping huge mutation streams in memory)
	allSources := make(map[string][]string)
	var existingAssets []string
	database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID).Pluck("value", &existingAssets)

	mergeCandidateCount := 0
	mergeDone := scanIsStepDone(targetID, "DISCOVERY", "MERGE")
	if mergeDone {
		if _, err := os.Stat(allFoundFile); err != nil {
			log.Printf("⚠️ MERGE checkpoint artifact is missing; rebuilding MERGE before DNSX. err=%v\n", err)
			mergeDone = false
			allSources = make(map[string][]string)
		} else {
			mergeCandidateCount = discoverytools.CountLines(allFoundFile)
			_ = utils.ReadJSONFromFile(allSourcesFile, &allSources)
			log.Printf("⏩ Resume: skipping MERGE (candidate file has %d entries)\n", mergeCandidateCount)
		}
	}

	if !mergeDone {
		scanMarkRunning(targetID, "DISCOVERY", "MERGE")
		discoverymerge.MergeSources(allSources, passiveSources)
		// Do not merge full AlterX candidate sources here. Large AlterX runs can
		// produce millions of unresolved candidates. AlterX source attribution is
		// applied later only for hosts that actually resolve via DNSX.
		discoverymerge.MergeSources(allSources, purednsSources)

		var err error
		mergeCandidateCount, err = discoverymerge.WriteCandidatesToFile(discoverymerge.CandidateFileInput{
			PassiveResults:     passiveResults,
			MutatedResultsFile: alterxResultsFile,
			PurednsResults:     purednsResults,
			ExistingAssets:     existingAssets,
			OutputFile:         allFoundFile,
			CheckStop: func() bool {
				return checkStopRequest(targetID)
			},
		})
		if err != nil {
			if err == os.ErrClosed {
				return
			}
			log.Printf("❌ Failed to write DNSX candidate file: %v\n", err)
			scanMarkFailed(targetID, fmt.Sprintf("merge checkpoint failed: %v", err))
			return
		}

		if err := utils.WriteJSONToFile(allSourcesFile, allSources); err != nil {
			log.Printf("❌ Failed to write merged sources checkpoint: %v\n", err)
			scanMarkFailed(targetID, fmt.Sprintf("merged sources checkpoint failed: %v", err))
			return
		}
		scanMarkStepDone(targetID, "DISCOVERY", "MERGE")
	}

	log.Printf(" Master candidate file created with %d potential subdomain entries. Starting validation (DNSX)...\n", mergeCandidateCount)

	// 4. Validation (DNSX)
	var dnsxResults map[string][]string
	if scanIsStepDone(targetID, "DISCOVERY", "DNSX") {
		dnsxResults = map[string][]string{}
		_ = utils.ReadJSONFromFile(dnsxResultsFile, &dnsxResults)
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

		// Merge puredns results (already live) into dnsxResults.
		dnsxResults = discoverymerge.MergeLiveResults(dnsxResults, purednsResults)

		_ = utils.WriteJSONToFile(dnsxResultsFile, dnsxResults)
		scanMarkStepDone(targetID, "DISCOVERY", "DNSX")
	}

	if targetConf.UseAlterx {
		if scanIsStepDone(targetID, "DISCOVERY", "ALTERX") {
			alterxCandidateCount = discoverytools.CountLines(alterxResultsFile)
			_ = utils.ReadJSONFromFile(alterxSourcesFile, &mutatedSources)
			log.Printf("⏩ Resume: skipping ALTERX (loaded %d live-derived candidates from checkpoint)\n", alterxCandidateCount)
		} else {
			if checkStopRequest(targetID) {
				return
			}
			scanMarkRunning(targetID, "DISCOVERY", "ALTERX")
			updateTargetPhase(targetID, "PHASE 1: MUTATION (ALTERX ON LIVE DNSX HOSTS)")

			liveHosts := make([]string, 0, len(dnsxResults))
			for host := range dnsxResults {
				liveHosts = append(liveHosts, host)
			}
			sort.Strings(liveHosts)

			if len(liveHosts) == 0 {
				log.Printf("⏩ Skipping AlterX for %s: no DNSX-live hosts to mutate.\n", rootDomain)
				_ = utils.WriteSliceToFile(alterxResultsFile, []string{})
			} else {
				if err := utils.WriteSliceToFile(alterxLiveInputFile, liveHosts); err != nil {
					log.Printf("❌ Failed to write AlterX live input: %v\n", err)
					scanMarkFailed(targetID, fmt.Sprintf("alterx live input failed: %v", err))
					return
				}

				var err error
				alterxCandidateCount, err = runAlterx(targetID, alterxLiveInputFile, rootDomain, alterxResultsFile)
				if err != nil {
					if err.Error() == "process killed by user request" {
						return
					}
					log.Printf("❌ AlterX failed on live hosts: %v. Proceeding without mutations.\n", err)
					alterxCandidateCount = 0
					_ = utils.WriteSliceToFile(alterxResultsFile, []string{})
				}
			}

			mutatedSources = map[string][]string{}
			if err := utils.WriteJSONToFile(alterxSourcesFile, mutatedSources); err != nil {
				log.Printf("❌ Failed to write alterx sources checkpoint: %v\n", err)
				scanMarkFailed(targetID, fmt.Sprintf("alterx sources checkpoint failed: %v", err))
				return
			}
			scanMarkStepDone(targetID, "DISCOVERY", "ALTERX")
		}

		if alterxCandidateCount > 0 {
			if scanIsStepDone(targetID, "DISCOVERY", "ALTERX_DNSX") {
				log.Printf("⏩ Resume: skipping ALTERX_DNSX validation\n")
			} else {
				if checkStopRequest(targetID) {
					return
				}
				scanMarkRunning(targetID, "DISCOVERY", "ALTERX_DNSX")
				updateTargetPhase(targetID, "PHASE 1: DNSX VALIDATION FOR ALTERX")
				alterxLiveResults, err := runDnsx(targetID, alterxResultsFile, alterxDNSXOutputFile)
				if err != nil {
					if err.Error() == "process killed by user request" {
						return
					}
					log.Printf("❌ AlterX DNSX validation failed: %v\n", err)
					scanMarkFailed(targetID, fmt.Sprintf("alterx dnsx failed: %v", err))
					return
				}
				_ = os.Remove(alterxDNSXOutputFile)

				dnsxResults = discoverymerge.MergeLiveResults(dnsxResults, alterxLiveResults)

				if err := discoverymerge.MergeFileSourcesForLive(allSources, alterxResultsFile, alterxLiveResults, "alterx"); err != nil {
					log.Printf("⚠️ Failed to apply live-only AlterX sources: %v\n", err)
				}
				_ = utils.WriteJSONToFile(allSourcesFile, allSources)
				_ = utils.WriteJSONToFile(dnsxResultsFile, dnsxResults)
				scanMarkStepDone(targetID, "DISCOVERY", "ALTERX_DNSX")
			}
		}
	}

	log.Printf("✅ [Stage 3] Confirmed %d LIVE subdomains with IPs (including %d from puredns and AlterX-live mutations).\n", len(dnsxResults), len(purednsResults))

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
		saveList := discoverymerge.BuildSaveList(discoverymerge.SaveListInput{
			PassiveResults: passiveResults,
			LiveResults:    dnsxResults,
			PurednsResults: purednsResults,
			ExistingAssets: existingAssets,
		})
		saveDiscoveryResultsToDB(target, saveList, dnsxResults, allSources)
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
		discovery.RunNmapScan(targetID, nmapInputFile, runCommandWithKillSwitchCombined)
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
	probing.Run(probing.Context{
		TargetID:   targetID,
		RootDomain: rootDomain,
		BatchSize:  workerhelpers.GetBatchSize(DefaultBatchSize),

		CheckStop:         checkStopRequest,
		UpdateTargetPhase: updateTargetPhase,
		EnsureScanState: func(targetID uint) {
			_, _ = ensureScanState(targetID)
		},
		ScanIsStepDone:    scanIsStepDone,
		ScanGetMeta:       scanGetMeta,
		ScanSetMeta:       scanSetMeta,
		ScanMarkRunning:   scanMarkRunning,
		ScanMarkStepDone:  scanMarkStepDone,
		TriggerNextModule: triggerNextModule,
		RunCommand:        runCommandWithKillSwitch,
		UpdateAssets: func(targetID uint, results map[string]probing.HTTPXResult) {
			UpdateAssetsWithDiff(targetID, results)
		},
		AfterProbing: func(targetID uint) {
			runPostProbingSecurity(targetID, rootDomain)
		},
	})
}

func runPostProbingSecurity(targetID uint, rootDomain string) {
	if err := workerfindings.GenerateBuiltinFindings(targetID); err != nil {
		log.Printf("⚠️ Failed to generate builtin findings for target %d: %v\n", targetID, err)
	}

	if err := takeoverphase.Run(takeoverphase.Context{
		TargetID:          targetID,
		RootDomain:        rootDomain,
		CheckStop:         checkStopRequest,
		UpdateTargetPhase: updateTargetPhase,
		ScanIsStepDone:    scanIsStepDone,
		ScanMarkRunning:   scanMarkRunning,
		ScanMarkStepDone:  scanMarkStepDone,
	}); err != nil {
		log.Printf("⚠️ Takeover detection failed for target %d: %v\n", targetID, err)
	}

	if err := nucleiphase.Run(nucleiphase.Context{
		TargetID:              targetID,
		RootDomain:            rootDomain,
		CheckStop:             checkStopRequest,
		UpdateTargetPhase:     updateTargetPhase,
		ScanIsStepDone:        scanIsStepDone,
		ScanMarkRunning:       scanMarkRunning,
		ScanMarkStepDone:      scanMarkStepDone,
		RunCommand:            runCommandWithKillSwitch,
		RunCommandWithTimeout: runCommandWithTimeoutAndKillSwitch,
	}); err != nil {
		log.Printf("⚠️ Nuclei security scan failed for target %d: %v\n", targetID, err)
	}
}

// =================================================================
// PHASE 3: Crawling Implementation (FIXED)
// =================================================================
func runCrawlingPhase(targetID uint, rootDomain string) {
	crawlingphase.Run(crawlingphase.Context{
		TargetID:            targetID,
		RootDomain:          rootDomain,
		CheckStop:           checkStopRequest,
		UpdateTargetPhase:   updateTargetPhase,
		EnsureScanState:     ensureScanState,
		ScanIsStepDone:      scanIsStepDone,
		ScanMarkRunning:     scanMarkRunning,
		ScanMarkStepDone:    scanMarkStepDone,
		TriggerNextModule:   triggerNextModule,
		RunCommand:          runCommandWithKillSwitch,
		RunCommandWithStdin: runCommandWithStdinAndKillSwitch,
		AfterCrawling: func(targetID uint) {
			runPostCrawlingSecurity(targetID, rootDomain)
		},
	})
}

func runPostCrawlingSecurity(targetID uint, rootDomain string) {
	if err := jsintelphase.Run(jsintelphase.Context{
		TargetID:          targetID,
		RootDomain:        rootDomain,
		CheckStop:         checkStopRequest,
		UpdateTargetPhase: updateTargetPhase,
		ScanIsStepDone:    scanIsStepDone,
		ScanMarkRunning:   scanMarkRunning,
		ScanMarkStepDone:  scanMarkStepDone,
	}); err != nil {
		log.Printf("⚠️ JS intelligence failed for target %d: %v\n", targetID, err)
	}
}

// saveCrawledURLs (UPDATED) اضافه شدن پارامتر silent

// ==========================================
// Smart Storage & Notification Logic
// ==========================================

func saveDiscoveryResultsToDB(target models.Target, masterList []string, liveResults map[string][]string, sourcesMap map[string][]string) {
	discoverypersist.SaveDiscoveryResultsToDB(discoverypersist.SaveContext{
		LogChange: logChange,
	}, target, masterList, liveResults, sourcesMap)
}

func UpdateAssetsWithDiff(targetID uint, results map[string]HttpxResult) {
	probingpersist.UpdateAssetsWithDiff(probingpersist.Context{
		LogChange: logChange,
	}, targetID, results)
}

// ... (Helper Functions)
func runCommandWithKillSwitch(targetID uint, name string, args ...string) ([]byte, error) {
	return workerruntime.RunCommandWithKillSwitch(targetID, scanHeartbeat, checkStopRequest, name, args...)
}

func runCommandWithStdinAndKillSwitch(targetID uint, stdin io.Reader, name string, args ...string) ([]byte, error) {
	return workerruntime.RunCommandWithStdinAndKillSwitch(targetID, stdin, scanHeartbeat, checkStopRequest, name, args...)
}

func runCommandWithTimeoutAndKillSwitch(targetID uint, timeout time.Duration, name string, args ...string) ([]byte, error) {
	return workerruntime.RunCommandWithTimeoutAndKillSwitch(targetID, timeout, scanHeartbeat, checkStopRequest, name, args...)
}

// runCommandWithKillSwitchCombined مثل runCommandWithKillSwitch ولی stdout/stderr را با هم برمی‌گرداند (برای ابزارهایی مثل nmap)
func runCommandWithKillSwitchCombined(targetID uint, name string, args ...string) ([]byte, error) {
	return workerruntime.RunCommandWithKillSwitchCombined(targetID, scanHeartbeat, checkStopRequest, name, args...)
}

func checkStopRequest(targetID uint) bool {
	return workerstate.CheckStopRequest(targetID)
}

func updateTargetPhase(targetID uint, phase string) {
	workerstate.UpdateTargetPhase(targetID, phase)
}

func ensureScanState(targetID uint) (*models.TargetScanState, error) {
	return workerstate.EnsureScanState(targetID)
}

func scanMarkRunning(targetID uint, module, step string) {
	workerstate.MarkRunning(targetID, module, step)
}

func scanHeartbeat(targetID uint) {
	workerstate.Heartbeat(targetID)
}

func scanMarkPaused(targetID uint, reason string) {
	workerstate.MarkPaused(targetID, reason)
}

func scanMarkFailed(targetID uint, errMsg string) {
	workerstate.MarkFailed(targetID, errMsg)
}

func scanMarkStepDone(targetID uint, module, step string) {
	workerstate.MarkStepDone(targetID, module, step)
}

func scanIsStepDone(targetID uint, module, step string) bool {
	return workerstate.IsStepDone(targetID, module, step)
}

func scanGetMeta(targetID uint) map[string]interface{} {
	return workerstate.GetMeta(targetID)
}

func scanSetMeta(targetID uint, meta map[string]interface{}) {
	workerstate.SetMeta(targetID, meta)
}

func triggerNextModule(targetID uint, rootDomain, currentModule string) {
	workerstate.TriggerNextModule(targetID, rootDomain, currentModule, workerruntime.CleanupTargetTempDir)
}

func logChange(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error {
	if oldVal == newVal {
		return nil
	}
	history := models.AssetHistory{AssetID: assetID, FieldName: field, OldValue: oldVal, NewValue: newVal, CreatedAt: time.Now()}
	return tx.Create(&history).Error
}

func buildDiscoveryToolContext(targetID uint, rootDomain string) discoverytools.Context {
	tempDir, _, _ := utils.GetTargetTempDir(targetID)
	return discoverytools.Context{
		TargetID:           targetID,
		RootDomain:         rootDomain,
		TempDir:            tempDir,
		RunCommand:         runCommandWithKillSwitch,
		CheckStop:          checkStopRequest,
		NormalizeSubdomain: discoverytools.NormalizeSubdomain,
		UpdateTargetPhase:  updateTargetPhase,
		Heartbeat:          scanHeartbeat,
	}
}

func runDnsx(targetID uint, inputFile, outputFile string) (map[string][]string, error) {
	return discoverytools.RunDNSX(buildDiscoveryToolContext(targetID, ""), inputFile, outputFile)
}

// normalizeSubdomain حذف www و normalize کردن subdomain
// runPuredns اجرای puredns برای bruteforce subdomain discovery
// این تابع subdomain‌های لایو (resolve شده) با IP‌هایشان را برمی‌گرداند
// خروجی: map[string][]string که key=subdomain و value=[]IP
func runPuredns(targetID uint, rootDomain string, wordlists []string) (map[string][]string, error) {
	return discoverytools.RunPureDNS(buildDiscoveryToolContext(targetID, rootDomain), rootDomain, wordlists)
}

// runVirusTotalCollection collects URLs from VirusTotal API for live subdomains

// SubdomainSource نگه‌داری subdomain و source آن

// runPassiveCollection جمع‌آوری اطلاعات غیرفعال (Passive) از منابع مختلف
func runPassiveCollection(targetID uint, domain string) ([]string, map[string][]string, error) {
	return passivetools.Collect(passivetools.Context{
		TargetID: targetID,
		Domain:   domain,
		RunCommand: func(name string, args ...string) ([]byte, error) {
			return runCommandWithKillSwitch(targetID, name, args...)
		},
		RunCombinedCommand: func(name string, args ...string) ([]byte, error) {
			return runCommandWithKillSwitchCombined(targetID, name, args...)
		},
		RunCombinedCommandWithTimeout: func(timeout time.Duration, name string, args ...string) ([]byte, error) {
			return workerruntime.RunCommandWithKillSwitchCombinedTimeout(targetID, timeout, scanHeartbeat, checkStopRequest, name, args...)
		},
	})
}

func runAlterx(targetID uint, inputFile, rootDomain, outputFile string) (int, error) {
	return discoverytools.RunAlterx(buildDiscoveryToolContext(targetID, rootDomain), inputFile, rootDomain, outputFile)
}

// runCdnCheckForLiveAssets چک کردن CDN برای همه IPهای لایو که dnsx resolve کرده
func runCdnCheckForLiveAssets(targetID uint, dnsxResults map[string][]string, inputFile string) {
	discoverypersist.RunCDNCheckForLiveAssets(discoverypersist.Context{
		TargetID:   targetID,
		RunCommand: runCommandWithKillSwitch,
	}, dnsxResults, inputFile)
}

// runCdnCheck اجرای cdncheck روی لیست IPها
// =================================================================
// Optional Port Scan (Nmap) - PHASE 1
// =================================================================
