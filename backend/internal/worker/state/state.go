package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"gorm.io/gorm"
)

const stopCheckCacheTTL = time.Second

type stopCheckCacheEntry struct {
	checkedAt time.Time
	stopped   bool
}

var stopCheckCache sync.Map

func clearStopCheckCache(targetID uint) {
	stopCheckCache.Delete(targetID)
}

func CheckStopRequest(targetID uint) bool {
	now := time.Now()

	if cached, ok := stopCheckCache.Load(targetID); ok {
		entry, ok := cached.(stopCheckCacheEntry)
		if ok && now.Sub(entry.checkedAt) < stopCheckCacheTTL {
			return entry.stopped
		}
	}

	var t models.Target
	if err := database.DB.Select("stop_requested").First(&t, targetID).Error; err != nil {
		stopCheckCache.Store(targetID, stopCheckCacheEntry{checkedAt: now, stopped: false})
		return false
	}

	if t.StopRequested {
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"status":         "PAUSED",
			"current_phase":  "PAUSED BY USER",
			"stop_requested": false,
			"last_scan_at":   &now,
		})
		MarkPaused(targetID, "paused by user")
		stopCheckCache.Store(targetID, stopCheckCacheEntry{checkedAt: now, stopped: true})
		return true
	}

	stopCheckCache.Store(targetID, stopCheckCacheEntry{checkedAt: now, stopped: false})
	return false
}

func UpdateTargetPhase(targetID uint, phase string) {
	database.DB.Model(&models.Target{}).Where("id = ?", targetID).Update("current_phase", phase)
}

func ResetForNewRun(targetID uint) error {
	clearStopCheckCache(targetID)
	now := time.Now()
	runID := fmt.Sprintf("run_%d", now.UnixNano())

	return database.DB.Transaction(func(tx *gorm.DB) error {
		var existing models.TargetScanState
		err := tx.Where("target_id = ?", targetID).First(&existing).Error

		if errors.Is(err, gorm.ErrRecordNotFound) {
			st := models.TargetScanState{
				TargetID:       targetID,
				RunID:          runID,
				Status:         "QUEUED",
				CurrentModule:  "",
				CurrentStep:    "",
				CompletedSteps: "[]",
				Meta:           "{}",
				HeartbeatAt:    &now,
				LastError:      "",
			}
			return tx.Create(&st).Error
		}

		if err != nil {
			return err
		}

		return tx.Model(&models.TargetScanState{}).
			Where("target_id = ?", targetID).
			Updates(map[string]interface{}{
				"run_id":          runID,
				"status":          "QUEUED",
				"current_module":  "",
				"current_step":    "",
				"completed_steps": "[]",
				"meta":            "{}",
				"heartbeat_at":    &now,
				"last_error":      "",
			}).Error
	})
}

func EnsureScanState(targetID uint) (*models.TargetScanState, error) {
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

func completedSet(completedJSON string) map[string]bool {
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

func completedList(set map[string]bool) []string {
	var keys []string
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func stepKey(module, step string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(module), strings.TrimSpace(step))
}

func MarkRunning(targetID uint, module, step string) {
	clearStopCheckCache(targetID)
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

func Heartbeat(targetID uint) {
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ? AND status = ?", targetID, "RUNNING").
		Update("heartbeat_at", &now).Error
}

func MarkPaused(targetID uint, reason string) {
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":       "PAUSED",
			"heartbeat_at": &now,
			"last_error":   reason,
		}).Error
}

func MarkFailed(targetID uint, errMsg string) {
	failedPhase := "FAILED"
	if errMsg != "" {
		failedPhase = "FAILED: " + errMsg
	}

	_ = database.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.TargetScanState{}).
			Where("target_id = ?", targetID).
			Updates(map[string]interface{}{
				"status":       "FAILED",
				"last_error":   errMsg,
				"heartbeat_at": time.Now(),
			}).Error; err != nil {
			return err
		}

		return tx.Model(&models.Target{}).
			Where("id = ?", targetID).
			Updates(map[string]interface{}{
				"status":         "FAILED",
				"current_phase":  failedPhase,
				"stop_requested": false,
				"updated_at":     time.Now(),
			}).Error
	})
}

func MarkStepDone(targetID uint, module, step string) {
	st, err := EnsureScanState(targetID)
	if err != nil {
		return
	}

	set := completedSet(st.CompletedSteps)
	set[stepKey(module, step)] = true
	keys := completedList(set)
	b, _ := json.Marshal(keys)
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"completed_steps": string(b),
			"heartbeat_at":    &now,
		}).Error
}

func IsStepDone(targetID uint, module, step string) bool {
	st, err := EnsureScanState(targetID)
	if err != nil {
		return false
	}

	set := completedSet(st.CompletedSteps)
	return set[stepKey(module, step)]
}

func GetMeta(targetID uint) map[string]interface{} {
	st, err := EnsureScanState(targetID)
	if err != nil {
		return map[string]interface{}{}
	}

	out := map[string]interface{}{}
	if st.Meta != "" {
		_ = json.Unmarshal([]byte(st.Meta), &out)
	}

	return out
}

func SetMeta(targetID uint, meta map[string]interface{}) {
	b, _ := json.Marshal(meta)
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"meta":         string(b),
			"heartbeat_at": &now,
		}).Error
}

func TriggerNextModule(targetID uint, rootDomain, currentModule string, cleanupTargetTempDir func(uint)) {
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		log.Printf("⚠️ Chaining failed: target %d not found after %s: %v\n", targetID, currentModule, err)
		MarkFailed(targetID, fmt.Sprintf("target not found while chaining from %s: %v", currentModule, err))
		return
	}

	var modules []string
	if err := json.Unmarshal([]byte(target.ScanModules), &modules); err != nil {
		errMsg := fmt.Sprintf("invalid scan module list while chaining from %s: %v", currentModule, err)
		log.Printf("❌ %s\n", errMsg)
		MarkFailed(targetID, errMsg)
		_ = database.DB.Model(&models.Target{}).
			Where("id = ?", targetID).
			Updates(map[string]interface{}{
				"status":         "PAUSED",
				"current_phase":  "FAILED: INVALID MODULE LIST",
				"stop_requested": false,
			}).Error
		return
	}

	currentIndex := -1
	for i, m := range modules {
		if strings.TrimSpace(m) == currentModule {
			currentIndex = i
			break
		}
	}

	if currentIndex != -1 && currentIndex+1 < len(modules) {
		nextModule := strings.TrimSpace(modules[currentIndex+1])
		if nextModule == "" {
			errMsg := fmt.Sprintf("empty next module after %s", currentModule)
			log.Printf("❌ %s\n", errMsg)
			MarkFailed(targetID, errMsg)
			_ = database.DB.Model(&models.Target{}).
				Where("id = ?", targetID).
				Updates(map[string]interface{}{
					"status":         "PAUSED",
					"current_phase":  "FAILED: EMPTY NEXT MODULE",
					"stop_requested": false,
				}).Error
			return
		}

		log.Printf(" Chaining: Phase '%s' done. Starting '%s'\n", currentModule, nextModule)

		// Update status to QUEUED so it does not count towards concurrency limit while waiting.
		if err := database.DB.Model(&models.Target{}).
			Where("id = ?", targetID).
			Updates(map[string]interface{}{"status": "QUEUED", "stop_requested": false}).Error; err != nil {
			errMsg := fmt.Sprintf("failed to mark target queued for next module %s: %v", nextModule, err)
			log.Printf("❌ %s\n", errMsg)
			MarkFailed(targetID, errMsg)
			return
		}

		payload := fmt.Sprintf("%s:%d:%s", nextModule, targetID, rootDomain)
		queueName := redisq.QueueNameForUser(target.CreatedByUserID)
		if err := redisq.Client.RPush(context.Background(), queueName, payload).Err(); err != nil {
			errMsg := fmt.Sprintf("failed to enqueue next module %s for target %d: %v", nextModule, targetID, err)
			log.Printf("❌ %s\n", errMsg)
			MarkFailed(targetID, errMsg)
			_ = database.DB.Model(&models.Target{}).
				Where("id = ?", targetID).
				Updates(map[string]interface{}{
					"status":         "PAUSED",
					"current_phase":  "FAILED: QUEUE NEXT MODULE",
					"stop_requested": false,
				}).Error
			return
		}

		return
	}

	log.Printf(" Chain Complete for %s.\n", rootDomain)
	now := time.Now()
	if err := database.DB.Model(&models.Target{}).
		Where("id = ?", targetID).
		Updates(map[string]interface{}{
			"last_scan_at":  &now,
			"status":        "READY",
			"scan_count":    gorm.Expr("scan_count + 1"),
			"current_phase": "IDLE",
		}).Error; err != nil {
		errMsg := fmt.Sprintf("failed to finalize target after completed chain: %v", err)
		log.Printf("❌ %s\n", errMsg)
		MarkFailed(targetID, errMsg)
		return
	}

	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":         "COMPLETED",
			"current_module": "",
			"current_step":   "",
			"heartbeat_at":   &now,
			"last_error":     "",
		}).Error

	cleanupTargetTempDir(targetID)
}
