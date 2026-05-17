package probing

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

// Run executes Phase 2 (Probing) for a target. It intentionally preserves the
// existing worker behavior: batch processing, checkpoint metadata, kill-switch
// handling, and chaining to the next module are all delegated through Context.
func Run(ctx Context) {
	targetID := ctx.TargetID
	rootDomain := ctx.RootDomain

	if ctx.CheckStop(targetID) {
		return
	}

	tempDir, username, err := utils.GetTargetTempDir(targetID)
	if err != nil {
		log.Printf("❌ Failed to init temp dir for target %d: %v\n", targetID, err)
		return
	}
	log.Printf("🗂️ Temp workspace: %s (owner: %s)\n", tempDir, username)

	probingInputFile := filepath.Join(tempDir, "httpx_input.txt")
	probingOutputFile := filepath.Join(tempDir, "httpx_output.json")

	batchSize := ctx.BatchSize
	if batchSize <= 0 {
		batchSize = 500
	}

	log.Printf("🚀 Starting PHASE 2 (PROBING) for target ID: %d (Batch Size: %d)\n", targetID, batchSize)
	ctx.UpdateTargetPhase(targetID, "PHASE 2: INITIALIZING")
	startTime := time.Now()

	ctx.EnsureScanState(targetID)
	if ctx.ScanIsStepDone(targetID, "PROBING", "DONE") {
		log.Printf("⏩ Resume: PROBING already completed. Chaining next module.\n")
		ctx.TriggerNextModule(targetID, rootDomain, "PROBING")
		return
	}

	var totalLive int64
	database.DB.Model(&models.Asset{}).Where("target_id = ? AND is_live = true", targetID).Count(&totalLive)

	if totalLive == 0 {
		log.Println("⚠️ No live assets found. Aborting probing.")
		ctx.TriggerNextModule(targetID, rootDomain, "PROBING")
		return
	}
	log.Printf("✅ Found %d live assets in DB. Starting batch processing...\n", totalLive)

	meta := ctx.ScanGetMeta(targetID)
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
		if ctx.CheckStop(targetID) {
			meta["probing"] = map[string]interface{}{
				"offset":          offset,
				"processed_count": processedCount,
				"batch_size":      batchSize,
				"total_live":      totalLive,
			}
			ctx.ScanSetMeta(targetID, meta)
			return
		}

		statusMsg := fmt.Sprintf("PHASE 2: BATCH %d/%d", currentBatch, totalBatches)
		ctx.UpdateTargetPhase(targetID, statusMsg)
		ctx.ScanMarkRunning(targetID, "PROBING", "HTTPX_BATCH")

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

		if err := utils.WriteSliceToFile(probingInputFile, hosts); err != nil {
			log.Printf("❌ Error writing batch input file: %v\n", err)
			break
		}

		_, err = ctx.RunCommand(targetID, "httpx",
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

		httpxResults, err := readHTTPXResults(probingOutputFile)
		if err != nil {
			log.Printf("❌ Error reading batch httpx output: %v\n", err)
		} else {
			ctx.UpdateAssets(targetID, httpxResults)
			processedCount += len(batchAssets)
		}

		_ = os.Remove(probingOutputFile)
		_ = os.Remove(probingInputFile)

		offset += batchSize
		currentBatch++

		meta["probing"] = map[string]interface{}{
			"offset":          offset,
			"processed_count": processedCount,
			"batch_size":      batchSize,
			"total_live":      totalLive,
		}
		ctx.ScanSetMeta(targetID, meta)
	}

	log.Printf("🏁 PHASE 2 finished. Total processed: %d in %s.\n", processedCount, time.Since(startTime))
	ctx.ScanMarkStepDone(targetID, "PROBING", "DONE")
	ctx.TriggerNextModule(targetID, rootDomain, "PROBING")
}
