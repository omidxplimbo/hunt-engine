package database

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB متغیری هست که هندل اتصال به دیتابیس رو نگه میداره و در کل برنامه قابل دسترسه
var DB *gorm.DB

// Connect وظیفه اتصال به دیتابیس و انجام مایگریشن رو داره
func Connect() {
	var err error

	// 1. خواندن اطلاعات اتصال از متغیرهای محیطی
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")

	// ساختن رشته اتصال (Data Source Name - DSN)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=UTC",
		dbHost, dbUser, dbPassword, dbName)

	log.Println("Connecting to PostgreSQL database...")

	// 2. تلاش برای اتصال با استفاده از درایور پستگرس
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("❌ Failed to connect to the database! \n", err)
	}

	log.Println("✅ Connected to PostgreSQL successfully!")

	// 3. Auto Migration (جادوی GORM)
	// 👇 مدل FoundURL اضافه شد
	log.Println("Running auto-migrations...")
	err = DB.AutoMigrate(
		&models.Target{},
		&models.Asset{},
		&models.AssetHistory{},
		&models.User{},
		&models.FoundURL{},
		&models.SubfinderProviderConfig{},
		&models.TargetScanState{},
		&models.SystemConfig{},
		&models.UserTelegramConfig{},
		&models.UserLLMProviderConfig{},
		&models.UserFeatureFlagConfig{},
		&models.Finding{},
		&models.AIAnalysis{},
		&models.AIRecommendation{},
		&models.AuditLog{},
		&models.NucleiTemplate{},
	)

	if err != nil {
		log.Fatal("❌ Auto-migration failed! \n", err)
	}

	// Hardening defaults (idempotent best-effort)
	_ = DB.Exec("ALTER TABLE users ALTER COLUMN role SET DEFAULT 'viewer'").Error
	_ = DB.Exec("UPDATE users SET role = 'viewer' WHERE role IS NULL OR role = ''").Error
	_ = DB.Exec("ALTER TABLE users ALTER COLUMN is_active SET DEFAULT true").Error
	_ = DB.Exec("UPDATE users SET is_active = true WHERE is_active IS NULL").Error
	_ = DB.Exec("ALTER TABLE users ADD COLUMN IF NOT EXISTS max_concurrent_scans integer DEFAULT 1").Error
	_ = DB.Exec("UPDATE users SET max_concurrent_scans = 1 WHERE max_concurrent_scans IS NULL OR max_concurrent_scans < 1").Error

	if err := migrateTenantScopedUniqueIndexes(DB); err != nil {
		log.Fatal("❌ Tenant-scoped index migration failed! \n", err)
	}

	// v3.4.0 structured finding evidence foundation.
	// AutoMigrate adds the model field for new databases; these idempotent statements
	// harden upgrades from older releases and preserve legacy text evidence.
	_ = DB.Exec("ALTER TABLE findings ADD COLUMN IF NOT EXISTS evidence_json jsonb DEFAULT '{}'::jsonb").Error
	_ = DB.Exec("UPDATE findings SET evidence_json = jsonb_build_object('text', evidence) WHERE (evidence_json IS NULL OR evidence_json = '{}'::jsonb) AND evidence IS NOT NULL AND btrim(evidence) <> ''").Error

	log.Println("✅ Auto-migration completed successfully! Tables are ready.")
}

// migrateTenantScopedUniqueIndexes converts legacy global uniqueness into tenant-scoped uniqueness.
//
// AutoMigrate creates missing indexes, but it does not remove obsolete indexes. Older versions
// created global unique indexes on targets.root_domain and assets.value, which blocked two users
// from creating/scanning the same domain independently. This migration is idempotent and safe to
// run on every startup.
func migrateTenantScopedUniqueIndexes(db *gorm.DB) error {
	statements := []string{
		// Create the new multi-tenant uniqueness rules first.
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_targets_owner_root_domain ON targets (created_by_user_id, root_domain)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_assets_target_value ON assets (target_id, value)`,

		// Drop legacy global uniqueness rules. Depending on how they were created,
		// PostgreSQL may expose them as constraints, indexes, or both.
		`ALTER TABLE targets DROP CONSTRAINT IF EXISTS idx_targets_root_domain`,
		`DROP INDEX IF EXISTS idx_targets_root_domain`,
		`ALTER TABLE assets DROP CONSTRAINT IF EXISTS idx_assets_value`,
		`DROP INDEX IF EXISTS idx_assets_value`,
	}

	for _, stmt := range statements {
		if err := db.Exec(stmt).Error; err != nil {
			return fmt.Errorf("failed to run tenant-scoped index migration statement %q: %w", stmt, err)
		}
	}

	return nil
}

// CleanupZombieScans تارگت‌هایی که در حالت SCANNING گیر کرده‌اند را آزاد می‌کند
func CleanupZombieScans() {
	log.Println("🧹 Checking for interrupted scans (Zombies)...")

	result := DB.Model(&models.Target{}).
		Where("status = ?", "SCANNING").
		Updates(map[string]interface{}{
			"status":         "PAUSED",
			"current_phase":  "INTERRUPTED (CRASH) - RESUMABLE",
			"stop_requested": false,
		})

	// Also mark scan-state as paused best-effort (so UI/API can safely resume).
	now := time.Now().UTC()
	_ = DB.Model(&models.TargetScanState{}).
		Where("status = ?", "RUNNING").
		Updates(map[string]interface{}{
			"status":       "PAUSED",
			"heartbeat_at": &now,
			"last_error":   "interrupted (crash)",
		}).Error

	if result.Error != nil {
		log.Printf("❌ Failed to cleanup zombies: %v\n", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("✅ Cleaned up %d interrupted scans. They are now PAUSED (resumable).\n", result.RowsAffected)
	} else {
		log.Println("✅ No zombie scans found. System is clean.")
	}
}

// CleanupZombieQueuedItems resets targets stuck in QUEUED state if they are not in the provided Redis list
func CleanupZombieQueuedItems(activePayloads []string) {
	log.Println("🧹 Checking for zombie QUEUED items...")

	var queuedTargets []models.Target
	if err := DB.Where("status = ?", "QUEUED").Find(&queuedTargets).Error; err != nil {
		log.Printf("⚠️ Failed to fetch queued targets: %v\n", err)
		return
	}

	if len(queuedTargets) == 0 {
		return
	}

	// Create a map of active target IDs in Redis
	// Payload format: "module:targetID:domain"
	activeMap := make(map[uint]bool)
	for _, p := range activePayloads {
		var module, domain string
		// Try parsing. Assuming format "module:id:domain"
		// We use Sscanf or Split
		// Let's use Split as it is safer if domain contains ':' (though domain shouldn't)
		// Actually, standard format used in handlers is "%s:%d:%s"
		var idInt int
		n, _ := fmt.Sscanf(p, "%s:%d:%s", &module, &idInt, &domain)
		// Sscanf might fail if domain has spaces etc, but domain is root domain.
		// Let's rely on middle part being ID.
		if n < 2 {
			// Try manual split
			// Just finding the ID is enough
			// TODO: Robust parsing
			continue
		}
		activeMap[uint(idInt)] = true
	}

	// Double check with simpler parsing if Sscanf failed for some
	// Or just better parsing:
	for _, p := range activePayloads {
		// module:id:domain
		// Find first and second colon
		first := -1
		second := -1
		for i, c := range p {
			if c == ':' {
				if first == -1 {
					first = i
				} else {
					second = i
					break
				}
			}
		}
		if first != -1 && second != -1 {
			idStr := p[first+1 : second]
			var id uint64
			fmt.Sscanf(idStr, "%d", &id)
			if id > 0 {
				activeMap[uint(id)] = true
			}
		}
	}

	count := 0
	for _, t := range queuedTargets {
		if !activeMap[t.ID] {
			// This target is QUEUED in DB but not in Redis -> Zombie
			DB.Model(&t).Updates(map[string]interface{}{
				"status":        "READY",
				"current_phase": "IDLE",
			})
			count++
		}
	}

	if count > 0 {
		log.Printf("✅ Fixed %d zombie QUEUED targets (reset to READY).\n", count)
	} else {
		log.Println("✅ Queue integrity check passed.")
	}
}
