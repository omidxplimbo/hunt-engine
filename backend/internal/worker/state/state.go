package state

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"gorm.io/gorm"
)

func CheckStopRequest(targetID uint) bool {
	var t models.Target
	if err := database.DB.Select("stop_requested").First(&t, targetID).Error; err != nil {
		return false
	}

	if t.StopRequested {
		now := time.Now()
		database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
			"status":         "PAUSED",
			"current_phase":  "PAUSED BY USER",
			"stop_requested": false,
			"last_scan_at":   &now,
		})
		MarkPaused(targetID, "paused by user")
		return true
	}

	return false
}

func UpdateTargetPhase(targetID uint, phase string) {
	database.DB.Model(&models.Target{}).Where("id = ?", targetID).Update("current_phase", phase)
}

func ResetForNewRun(targetID uint) error {
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
	now := time.Now()
	_ = database.DB.Model(&models.TargetScanState{}).
		Where("target_id = ?", targetID).
		Updates(map[string]interface{}{
			"status":       "FAILED",
			"heartbeat_at": &now,
			"last_error":   errMsg,
		}).Error
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

		// Update status to QUEUED so it doesn't count towards concurrency limit while waiting.
		database.DB.Model(&models.Target{}).
			Where("id = ?", targetID).
			Updates(map[string]interface{}{"status": "QUEUED", "stop_requested": false})

		payload := fmt.Sprintf("%s:%d:%s", nextModule, targetID, rootDomain)
		redisq.Client.RPush(context.Background(), redisq.QueueNameForUser(target.CreatedByUserID), payload)
		return
	}

	log.Printf("🏁 Chain Complete for %s.\n", rootDomain)
	now := time.Now()
	database.DB.Model(&models.Target{}).
		Where("id = ?", targetID).
		Updates(map[string]interface{}{
			"last_scan_at":  &now,
			"status":        "READY",
			"scan_count":    gorm.Expr("scan_count + 1"),
			"current_phase": "IDLE",
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

	cleanupTargetTempDir(targetID)
}
