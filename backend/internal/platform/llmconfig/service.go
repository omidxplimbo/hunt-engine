package llmconfig

import (
	"fmt"
	"log"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

func cacheKey(ownerKey string) string {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" {
		ownerKey = "unknown"
	}
	return fmt.Sprintf("llm:provider-config:%s", ownerKey)
}

// GetProvidersForOwner returns account-scoped LLM provider configs.
//
// Important security note:
// UserLLMProviderConfig.APIKey is tagged json:"-" so JSON marshaling drops it.
// Caching this model in Redis causes ResolveDefault to read cached provider rows
// without API keys and incorrectly fall back with "no API key saved".
// Do not cache provider configs that include secrets; read them directly from DB.
func GetProvidersForOwner(ownerKey string) ([]models.UserLLMProviderConfig, error) {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" {
		return nil, fmt.Errorf("owner key is required")
	}

	var rows []models.UserLLMProviderConfig
	if err := database.DB.
		Where("owner_key = ?", ownerKey).
		Order("is_default DESC, provider ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}

	return rows, nil
}

// InvalidateOwner removes legacy/stale provider cache entries.
// Provider configs are intentionally not cached because they include secrets.
func InvalidateOwner(ownerKey string) {
	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" || redisq.Client == nil {
		return
	}

	if err := redisq.Client.Del(redisq.Ctx, cacheKey(ownerKey)).Err(); err != nil {
		log.Printf("⚠️ Failed to invalidate LLM provider cache for %s: %v", ownerKey, err)
	}
}
