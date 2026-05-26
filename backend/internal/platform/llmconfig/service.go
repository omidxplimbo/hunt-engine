package llmconfig

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

const cacheTTL = 10 * time.Minute

func cacheKey(ownerKey string) string {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" {
		ownerKey = "unknown"
	}
	return fmt.Sprintf("llm:provider-config:%s", ownerKey)
}

// GetProvidersForOwner returns account-scoped LLM provider configs.
// It caches full internal rows, including API keys, in Redis.
// API handlers must still sanitize API keys before returning data to frontend.
func GetProvidersForOwner(ownerKey string) ([]models.UserLLMProviderConfig, error) {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" {
		return nil, fmt.Errorf("owner key is required")
	}

	key := cacheKey(ownerKey)

	if redisq.Client != nil {
		raw, err := redisq.Client.Get(redisq.Ctx, key).Result()
		if err == nil && strings.TrimSpace(raw) != "" {
			var rows []models.UserLLMProviderConfig
			if jsonErr := json.Unmarshal([]byte(raw), &rows); jsonErr == nil {
				return rows, nil
			} else {
				log.Printf("⚠️ Failed to decode LLM provider cache for %s: %v", ownerKey, jsonErr)
			}
		} else if err != nil && err != redis.Nil {
			log.Printf("⚠️ Failed to read LLM provider cache for %s: %v", ownerKey, err)
		}
	}

	var rows []models.UserLLMProviderConfig
	if err := database.DB.
		Where("owner_key = ?", ownerKey).
		Order("is_default DESC, provider ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	if redisq.Client != nil {
		if payload, err := json.Marshal(rows); err == nil {
			if err := redisq.Client.Set(redisq.Ctx, key, payload, cacheTTL).Err(); err != nil {
				log.Printf("⚠️ Failed to write LLM provider cache for %s: %v", ownerKey, err)
			}
		}
	}

	return rows, nil
}

func InvalidateOwner(ownerKey string) {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" || redisq.Client == nil {
		return
	}

	if err := redisq.Client.Del(redisq.Ctx, cacheKey(ownerKey)).Err(); err != nil {
		log.Printf("⚠️ Failed to invalidate LLM provider cache for %s: %v", ownerKey, err)
	}
}
