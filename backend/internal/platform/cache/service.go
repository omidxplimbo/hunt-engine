package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/redis/go-redis/v9"
)

var ctx = context.Background()

const CacheTTL = 1 * time.Hour

// ... (بقیه توابع قبلی مثل GetTargetVersion و IncrementTargetVersion سر جایشان بمانند) ...

func GetTargetVersion(targetID uint) int64 {
	key := fmt.Sprintf("target:%d:version", targetID)
	val, err := redisq.Client.Get(ctx, key).Int64()
	if err == redis.Nil {
		return 1
	}
	if err != nil {
		return 1
	}
	return val
}

func IncrementTargetVersion(targetID uint) {
	key := fmt.Sprintf("target:%d:version", targetID)
	redisq.Client.Incr(ctx, key)
}

// ... (تابع GetCache قبلی) ...
func GetCache(key string, dest interface{}) bool {
	val, err := redisq.Client.Get(ctx, key).Result()
	if err == redis.Nil || err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(val), dest); err != nil {
		return false
	}
	return true
}

// ... (تابع SetCache قبلی) ...
func SetCache(key string, data interface{}) {
	SetCacheWithTTL(key, data, CacheTTL)
}

// 👇👇👇 تابع جدید: کش کردن با زمان دلخواه
func SetCacheWithTTL(key string, data interface{}, ttl time.Duration) {
	bytes, err := json.Marshal(data)
	if err != nil {
		return
	}
	redisq.Client.Set(ctx, key, bytes, ttl)
}

// ... (GenerateAssetKey هم بماند) ...
// GenerateAssetKey کلید منحصر به فرد برای کش لیست دارایی‌ها می‌سازه
// اصلاح شده: استفاده از offset به جای page برای دقت بالاتر
func GenerateAssetKey(targetID uint, offset, limit int, filters string) string {
	version := GetTargetVersion(targetID)
	// تغییر p (page) به o (offset)
	return fmt.Sprintf("assets:tid:%d:v:%d:o:%d:l:%d:f:%s", targetID, version, offset, limit, filters)
}
