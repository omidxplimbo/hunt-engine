package scheduler

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

// بازه چک کردن دیتابیس (هر ۱ دقیقه چک می‌کند که نوبت کسی شده یا نه)
const CheckInterval = 1 * time.Minute

// Start تابع اصلی که باید در main.go به صورت go scheduler.Start() صدا زده شود
func Start() {
	log.Println("⏰ Scheduler started. Checking for due targets every minute...")

	ticker := time.NewTicker(CheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		scheduleDueTargets()
	}
}

func scheduleDueTargets() {
	var targets []models.Target

	// 👇 فقط تارگت‌هایی رو می‌گیریم که فعالن، فرکانس دارن و وضعیتشون READY هست
	// اینطوری تارگت‌های در حال اسکن اصلا انتخاب نمیشن
	err := database.DB.Where("in_scope = ? AND frequency > ? AND status = ?", true, 0, "READY").Find(&targets).Error
	if err != nil {
		log.Printf("❌ Scheduler DB Error: %v\n", err)
		return
	}

	for _, target := range targets {
		shouldScan := false

		if target.LastScanAt == nil {
			shouldScan = true
		} else {
			elapsed := time.Since(*target.LastScanAt)
			waitDuration := time.Duration(target.Frequency) * time.Minute
			if elapsed >= waitDuration {
				shouldScan = true
			}
		}

		if shouldScan {
			triggerScan(target)
		}
	}
}

func triggerScan(t models.Target) {
	var modules []string
	json.Unmarshal([]byte(t.ScanModules), &modules)

	if len(modules) == 0 {
		return
	}
	firstJob := modules[0]

	// 👇👇👇 قفل کردن تارگت (تغییر وضعیت به SCANNING)
	// این کار رو قبل از ارسال به صف انجام میدیم
	tx := database.DB.Model(&models.Target{}).Where("id = ?", t.ID).Update("status", "SCANNING")
	if tx.Error != nil {
		log.Printf("❌ Failed to lock target %s: %v\n", t.RootDomain, tx.Error)
		return
	}

	taskPayload := fmt.Sprintf("%s:%d:%s", firstJob, t.ID, t.RootDomain)
	err := redisq.Client.RPush(redisq.Ctx, redisq.QueueName, taskPayload).Err()

	if err != nil {
		log.Printf("⚠️ Scheduler failed to enqueue %s: %v\n", t.RootDomain, err)
		// اگر ارسال به صف شکست خورد، باید قفل رو باز کنیم (Rollback دستی)
		database.DB.Model(&models.Target{}).Where("id = ?", t.ID).Update("status", "READY")
		return
	}

	log.Printf("⏰ Scheduler Triggered Scan for: %s (Frequency: %d min)\n", t.RootDomain, t.Frequency)
}
