package featureflags

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	configsvc "github.com/omidxplimbo/hunt-engine/backend/internal/platform/config"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

const (
	KeyTargetPDFReport        = "feature.target_pdf_report"
	KeyAIAnalysis             = "feature.ai_analysis"
	KeyLLMAssistedAnalysis    = "feature.llm_assisted_analysis"
	KeyAIRecommendations      = "feature.ai_recommendations"
	KeyAINucleiTemplateDrafts = "feature.ai_nuclei_template_drafts"
)

const cacheTTL = 60 * time.Second

type Definition struct {
	Key         string `json:"key"`
	Default     bool   `json:"default"`
	Description string `json:"description"`
}

var definitions = []Definition{
	{
		Key:         KeyTargetPDFReport,
		Default:     true,
		Description: "Enable target-level downloadable PDF reports.",
	},
	{
		Key:         KeyAIAnalysis,
		Default:     true,
		Description: "Enable deterministic target analysis generation and analysis reads.",
	},
	{
		Key:         KeyLLMAssistedAnalysis,
		Default:     true,
		Description: "Enable optional LLM-assisted analysis narrative. Deterministic scoring remains authoritative.",
	},
	{
		Key:         KeyAIRecommendations,
		Default:     true,
		Description: "Enable deterministic evidence-based target recommendations.",
	},
	{
		Key:         KeyAINucleiTemplateDrafts,
		Default:     false,
		Description: "Enable AI-generated Nuclei template draft workflows. Disabled by default.",
	},
}

func All() []Definition {
	out := make([]Definition, len(definitions))
	copy(out, definitions)
	return out
}

func IsKnownKey(key string) bool {
	_, ok := defaultFor(key)
	return ok
}

func DefaultValue(key string) bool {
	value, _ := defaultFor(key)
	return value
}

func BoolString(value bool) string {
	if value {
		return "true"
	}
	return "false"
}

func IsEnabled(key string) bool {
	defaultValue, ok := defaultFor(key)
	if !ok {
		return false
	}

	if redisq.Client != nil {
		if raw, err := redisq.Client.Get(redisq.Ctx, cacheKey(key)).Result(); err == nil {
			return parseBool(raw, defaultValue)
		}
	}

	value := configsvc.GetBool(key, defaultValue)

	if redisq.Client != nil {
		_ = redisq.Client.Set(redisq.Ctx, cacheKey(key), BoolString(value), cacheTTL).Err()
	}

	return value
}

func DisabledResponse(c *fiber.Ctx, key string) error {
	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		"status":  "error",
		"message": "Feature is disabled",
		"feature": key,
	})
}

func Require(c *fiber.Ctx, key string) error {
	if IsEnabled(key) {
		return nil
	}

	return DisabledResponse(c, key)
}

func Invalidate(key string) {
	if redisq.Client == nil {
		return
	}

	_ = redisq.Client.Del(redisq.Ctx, cacheKey(key)).Err()
}

func InvalidateAll() {
	if redisq.Client == nil {
		return
	}

	keys := make([]string, 0, len(definitions))
	for _, def := range definitions {
		keys = append(keys, cacheKey(def.Key))
	}

	if len(keys) > 0 {
		_ = redisq.Client.Del(redisq.Ctx, keys...).Err()
	}
}

func defaultFor(key string) (bool, bool) {
	key = strings.TrimSpace(key)
	for _, def := range definitions {
		if def.Key == key {
			return def.Default, true
		}
	}
	return false, false
}

func cacheKey(key string) string {
	return "hunt:feature_flags:" + strings.TrimSpace(key)
}

func parseBool(raw string, defaultValue bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return defaultValue
	}
}
