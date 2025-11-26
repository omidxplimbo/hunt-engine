package redisq

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/redis/go-redis/v9"
)

// Client متغیری سراسری برای دسترسی به ردیس در کل برنامه
var Client *redis.Client

// Ctx کانتکست پس‌زمینه برای درخواست‌های ردیس
var Ctx = context.Background()

// QueueName اسم صفی که کارهای ریکان رو توش می‌ریزیم
const QueueName = "discovery_tasks"

// Connect اتصال به ردیس را برقرار می‌کند
func Connect() {
	redisHost := os.Getenv("REDIS_HOST")
	redisPort := os.Getenv("REDIS_PORT")

	addr := fmt.Sprintf("%s:%s", redisHost, redisPort)

	log.Println("Connecting to Redis...")

	Client = redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: "", // فعلا پسورد نداریم
		DB:       0,  // دیتابیس پیشفرض
	})

	// تست اتصال با دستور PING
	_, err := Client.Ping(Ctx).Result()
	if err != nil {
		log.Fatal("❌ Failed to connect to Redis! \n", err)
	}

	log.Println("✅ Connected to Redis successfully!")
}
