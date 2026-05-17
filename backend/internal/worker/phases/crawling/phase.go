package crawling

import (
	"log"
	"os"
	"path/filepath"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/crawling/tools"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

func Run(ctx Context) {
	if ctx.CheckStop(ctx.TargetID) {
		return
	}

	tempDir, username, err := utils.GetTargetTempDir(ctx.TargetID)
	if err != nil {
		log.Printf("❌ Failed to init temp dir for target %d: %v\n", ctx.TargetID, err)
		return
	}
	log.Printf("️ Temp workspace: %s (owner: %s)\n", tempDir, username)

	var target models.Target
	if err := database.DB.First(&target, ctx.TargetID).Error; err != nil {
		log.Printf("❌ Failed to fetch target config in Crawling Phase: %v\n", err)
		return
	}

	log.Printf(" Starting PHASE 3 (CRAWLING) for target ID: %d\n", ctx.TargetID)
	ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: FETCHING ASSETS")
	_, _ = ctx.EnsureScanState(ctx.TargetID)

	if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "DONE") {
		log.Printf("⏩ Resume: CRAWLING already completed. Chaining next module.\n")
		ctx.TriggerNextModule(ctx.TargetID, ctx.RootDomain, "CRAWLING")
		return
	}

	var liveAssets []string
	database.DB.Model(&models.Asset{}).
		Where("target_id = ? AND is_live = true", ctx.TargetID).
		Pluck("value", &liveAssets)

	crawlingAssetsFile := filepath.Join(tempDir, "crawling_assets.txt")
	if len(liveAssets) > 0 {
		if err := utils.WriteSliceToFile(crawlingAssetsFile, liveAssets); err != nil {
			log.Printf("❌ Failed to write crawling assets input: %v\n", err)
			return
		}
	}

	crawlingRootFile := filepath.Join(tempDir, "crawling_root.txt")
	if err := utils.WriteSliceToFile(crawlingRootFile, []string{ctx.RootDomain}); err != nil {
		log.Printf("❌ Failed to write crawling root input: %v\n", err)
		return
	}

	toolCtx := ctx.toolContext(tempDir)

	if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "WAYBACK") {
		log.Printf("⏩ Resume: skipping WAYBACK\n")
	} else {
		if ctx.CheckStop(ctx.TargetID) {
			return
		}
		ctx.ScanMarkRunning(ctx.TargetID, "CRAWLING", "WAYBACK")
		ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: RUNNING WAYBACK")
		urls := tools.RunWaybackURLs(toolCtx, crawlingRootFile)
		ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: SAVING WAYBACK")
		SaveURLs(ctx.TargetID, ctx.RootDomain, urls, true)
		ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "WAYBACK")
	}

	if len(liveAssets) > 0 {
		if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "GAU") {
			log.Printf("⏩ Resume: skipping GAU\n")
		} else {
			if ctx.CheckStop(ctx.TargetID) {
				return
			}
			ctx.ScanMarkRunning(ctx.TargetID, "CRAWLING", "GAU")
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: RUNNING GAU")
			urls := tools.RunGAU(toolCtx, crawlingAssetsFile)
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: SAVING GAU")
			SaveURLs(ctx.TargetID, ctx.RootDomain, urls, true)
			ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "GAU")
		}
	} else {
		ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "GAU")
	}

	if target.UseWaymore {
		if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "WAYMORE") {
			log.Printf("⏩ Resume: skipping WAYMORE\n")
		} else {
			if ctx.CheckStop(ctx.TargetID) {
				return
			}
			ctx.ScanMarkRunning(ctx.TargetID, "CRAWLING", "WAYMORE")
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: RUNNING WAYMORE")
			urls := tools.RunWaymore(toolCtx)
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: SAVING WAYMORE")
			SaveURLs(ctx.TargetID, ctx.RootDomain, urls, true)
			ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "WAYMORE")
		}
	} else {
		log.Println("⏩ Skipping Waymore (Disabled in settings)")
		ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "WAYMORE")
	}

	if len(liveAssets) > 0 {
		if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "KATANA") {
			log.Printf("⏩ Resume: skipping KATANA\n")
		} else {
			if ctx.CheckStop(ctx.TargetID) {
				return
			}
			ctx.ScanMarkRunning(ctx.TargetID, "CRAWLING", "KATANA")
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: RUNNING KATANA")
			urls := tools.RunKatana(toolCtx, crawlingAssetsFile)
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: SAVING KATANA")
			SaveURLs(ctx.TargetID, ctx.RootDomain, urls, true)
			ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "KATANA")
		}
	} else {
		ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "KATANA")
	}

	if len(liveAssets) > 0 {
		if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "VIRUSTOTAL") {
			log.Printf("⏩ Resume: skipping VIRUSTOTAL\n")
		} else {
			if ctx.CheckStop(ctx.TargetID) {
				return
			}
			ctx.ScanMarkRunning(ctx.TargetID, "CRAWLING", "VIRUSTOTAL")
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: RUNNING VIRUSTOTAL")
			log.Printf(" Running VirusTotal URL discovery for %d live assets...\n", len(liveAssets))
			urls := tools.RunVirusTotal(toolCtx, liveAssets)
			ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: SAVING VIRUSTOTAL")
			SaveURLs(ctx.TargetID, ctx.RootDomain, urls, true)
			ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "VIRUSTOTAL")
		}
	} else {
		ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "VIRUSTOTAL")
	}

	if ctx.ScanIsStepDone(ctx.TargetID, "CRAWLING", "FILTER_SAVE") {
		log.Printf("⏩ Resume: skipping FILTER/SAVE\n")
	} else {
		if ctx.CheckStop(ctx.TargetID) {
			return
		}
		ctx.ScanMarkRunning(ctx.TargetID, "CRAWLING", "FILTER_SAVE")
		ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: FILTERING & SAVING")
		ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "FILTER_SAVE")
	}

	_ = os.Remove(crawlingAssetsFile)
	_ = os.Remove(crawlingRootFile)

	log.Printf(" PHASE 3 finished for %s.\n", ctx.RootDomain)
	ctx.ScanMarkStepDone(ctx.TargetID, "CRAWLING", "DONE")
	ctx.TriggerNextModule(ctx.TargetID, ctx.RootDomain, "CRAWLING")
}
