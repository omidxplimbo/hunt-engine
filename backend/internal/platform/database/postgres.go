package database

import (
	"fmt"
	"log"
	"os"

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
	err = DB.AutoMigrate(&models.Target{}, &models.Asset{}, &models.AssetHistory{}, &models.User{}, &models.FoundURL{})

	if err != nil {
		log.Fatal("❌ Auto-migration failed! \n", err)
	}

	log.Println("✅ Auto-migration completed successfully! Tables are ready.")
}

// CleanupZombieScans تارگت‌هایی که در حالت SCANNING گیر کرده‌اند را آزاد می‌کند
func CleanupZombieScans() {
	log.Println("🧹 Checking for interrupted scans (Zombies)...")

	result := DB.Model(&models.Target{}).
		Where("status = ?", "SCANNING").
		Updates(map[string]interface{}{
			"status":         "READY",
			"current_phase":  "INTERRUPTED (CRASH)",
			"stop_requested": false,
		})

	if result.Error != nil {
		log.Printf("❌ Failed to cleanup zombies: %v\n", result.Error)
	} else if result.RowsAffected > 0 {
		log.Printf("✅ Cleaned up %d interrupted scans. They are now READY.\n", result.RowsAffected)
	} else {
		log.Println("✅ No zombie scans found. System is clean.")
	}
}
