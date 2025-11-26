package database

import (
	"fmt"
	"log"
	"os"

	// 👇👇👇 نکته مهم: این خط رو چک کن که با اسم ماژول تو توی go.mod یکی باشه
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

	// 1. خواندن اطلاعات اتصال از متغیرهای محیطی (که در docker-compose ست کردیم)
	dbHost := os.Getenv("DB_HOST")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := os.Getenv("DB_NAME")
	// dbPort := os.Getenv("DB_PORT") // پیشفرض پستگرس 5432 هست

	// ساختن رشته اتصال (Data Source Name - DSN)
	dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=5432 sslmode=disable TimeZone=UTC",
		dbHost, dbUser, dbPassword, dbName)

	log.Println("Connecting to PostgreSQL database...")

	// 2. تلاش برای اتصال با استفاده از درایور پستگرس
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		// تنظیم لاگر برای دیدن کوئری‌های SQL در کنسول (برای دیباگ عالیه)
		Logger: logger.Default.LogMode(logger.Info),
	})

	if err != nil {
		log.Fatal("❌ Failed to connect to the database! \n", err)
	}

	log.Println("✅ Connected to PostgreSQL successfully!")

	// 3. Auto Migration (جادوی GORM)
	// این دستور مدل‌ها رو می‌گیره و جدول‌هاشون رو توی دیتابیس می‌سازه یا آپدیت می‌کنه
	log.Println("Running auto-migrations...")
	err = DB.AutoMigrate(
		&models.Target{},
		&models.Asset{},
	)

	if err != nil {
		log.Fatal("❌ Auto-migration failed! \n", err)
	}

	log.Println("✅ Auto-migration completed successfully! Tables are ready.")
}
