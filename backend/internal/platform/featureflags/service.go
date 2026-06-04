package featureflags

import (
	"fmt"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	configsvc "github.com/omidxplimbo/hunt-engine/backend/internal/platform/config"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

const (
	KeyTargetPolicy           = "feature.target_policy"
	KeyTargetPDFReport        = "feature.target_pdf_report"
	KeyAIAnalysis             = "feature.ai_analysis"
	KeyLLMAssistedAnalysis    = "feature.llm_assisted_analysis"
	KeyAIRecommendations      = "feature.ai_recommendations"
	KeyAINucleiTemplateDrafts = "feature.ai_nuclei_template_drafts"
	KeyAgentRuns              = "feature.agent_runs"
	KeyAITriageAgent          = "feature.ai_triage_agent"
	KeyAISummaryAgent         = "feature.ai_summary_agent"
	KeyAIReportAgent          = "feature.ai_report_agent"
	KeyAgentActions           = "feature.agent_actions"

	KeyAgentChat  = "feature.agent_chat"
	StateInherit  = "inherit"
	StateEnabled  = "enabled"
	StateDisabled = "disabled"
)

const cacheTTL = 60 * time.Second

type Definition struct {
	Key         string `json:"key"`
	Default     bool   `json:"default"`
	Description string `json:"description"`
}

type AccountFeatureFlag struct {
	Key         string `json:"key"`
	Description string `json:"description"`
	Default     bool   `json:"default"`

	GlobalValue bool   `json:"global_value"`
	State       string `json:"state"`
	Effective   bool   `json:"effective"`
	Source      string `json:"source"`

	Scope    string `json:"scope"`
	OwnerKey string `json:"owner_key"`
}

var definitions = []Definition{
	{
		Key:         KeyTargetPolicy,
		Default:     true,
		Description: "Enable target policy configuration used by AI agents, reporting, and future policy-aware workflows.",
	},
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
	{
		Key:         KeyAgentRuns,
		Default:     true,
		Description: "Enable target-scoped AI/manual agent run history.",
	},
	{
		Key:         KeyAITriageAgent,
		Default:     true,
		Description: "Enable AI triage agent workflows. Agent output is advisory and policy-aware.",
	},
	{
		Key:         KeyAISummaryAgent,
		Default:     true,
		Description: "Enable AI summary agent workflows. Agent output is advisory and policy-aware.",
	},
	{
		Key:         KeyAIReportAgent,
		Default:     true,
		Description: "Enable AI report agent workflows. Report drafts require human validation.",
	},
	{
		Key:         KeyAgentActions,
		Default:     true,
		Description: "Enable proposed/approved/rejected agent actions for human-approved workflows.",
	},
	{
		Key:         KeyAgentChat,
		Default:     true,
		Description: "Enable target-scoped conversational chat/command agent sessions.",
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

func NormalizeState(state string) string {
	switch strings.ToLower(strings.TrimSpace(state)) {
	case "", StateInherit:
		return StateInherit
	case "true", "1", "yes", "on", StateEnabled:
		return StateEnabled
	case "false", "0", "no", "off", StateDisabled:
		return StateDisabled
	default:
		return ""
	}
}

func OwnerKeyForUser(user models.User) (string, string, *uint) {
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		return "admin", "admin-shared", nil
	}

	uid := user.ID
	return fmt.Sprintf("user:%d", user.ID), "user", &uid
}

func OwnerKeyForUserID(userID uint) string {
	if userID == 0 {
		return "admin"
	}

	var user models.User
	if err := database.DB.First(&user, userID).Error; err == nil {
		ownerKey, _, _ := OwnerKeyForUser(user)
		return ownerKey
	}

	return fmt.Sprintf("user:%d", userID)
}

func IsEnabled(key string) bool {
	return IsEnabledForOwner("", key)
}

func IsEnabledForOwner(ownerKey string, key string) bool {
	defaultValue, ok := defaultFor(key)
	if !ok {
		return false
	}

	ownerKey = strings.TrimSpace(ownerKey)
	key = strings.TrimSpace(key)

	if ownerKey != "" && redisq.Client != nil {
		if raw, err := redisq.Client.Get(redisq.Ctx, cacheKeyForOwner(ownerKey, key)).Result(); err == nil {
			return parseBool(raw, defaultValue)
		}
	}

	if ownerKey == "" && redisq.Client != nil {
		if raw, err := redisq.Client.Get(redisq.Ctx, cacheKey(key)).Result(); err == nil {
			return parseBool(raw, defaultValue)
		}
	}

	globalValue := configsvc.GetBool(key, defaultValue)
	value := globalValue

	if ownerKey != "" {
		var row models.UserFeatureFlagConfig
		if err := database.DB.
			Where("owner_key = ? AND feature_key = ?", ownerKey, key).
			First(&row).Error; err == nil {
			switch NormalizeState(row.State) {
			case StateEnabled:
				value = true
			case StateDisabled:
				value = false
			}
		}
	}

	if redisq.Client != nil {
		if ownerKey != "" {
			_ = redisq.Client.Set(redisq.Ctx, cacheKeyForOwner(ownerKey, key), BoolString(value), cacheTTL).Err()
		} else {
			_ = redisq.Client.Set(redisq.Ctx, cacheKey(key), BoolString(value), cacheTTL).Err()
		}
	}

	return value
}

func FlagsForOwner(ownerKey string, scope string) ([]AccountFeatureFlag, error) {
	ownerKey = strings.TrimSpace(ownerKey)

	rows := make([]models.UserFeatureFlagConfig, 0)
	if ownerKey != "" {
		if err := database.DB.
			Where("owner_key = ?", ownerKey).
			Find(&rows).Error; err != nil {
			return nil, err
		}
	}

	overrideByKey := make(map[string]string, len(rows))
	for _, row := range rows {
		state := NormalizeState(row.State)
		if state == "" {
			state = StateInherit
		}
		overrideByKey[row.FeatureKey] = state
	}

	out := make([]AccountFeatureFlag, 0, len(definitions))
	for _, def := range definitions {
		globalValue := configsvc.GetBool(def.Key, def.Default)
		state := overrideByKey[def.Key]
		if state == "" {
			state = StateInherit
		}

		effective := globalValue
		source := "global"

		switch state {
		case StateEnabled:
			effective = true
			source = "account"
		case StateDisabled:
			effective = false
			source = "account"
		default:
			state = StateInherit
		}

		out = append(out, AccountFeatureFlag{
			Key:         def.Key,
			Description: def.Description,
			Default:     def.Default,
			GlobalValue: globalValue,
			State:       state,
			Effective:   effective,
			Source:      source,
			Scope:       scope,
			OwnerKey:    ownerKey,
		})
	}

	return out, nil
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

func RequireForOwner(c *fiber.Ctx, ownerKey string, key string) error {
	if IsEnabledForOwner(ownerKey, key) {
		return nil
	}

	return DisabledResponse(c, key)
}

func Invalidate(key string) {
	if redisq.Client == nil {
		return
	}

	key = strings.TrimSpace(key)
	keys := []string{cacheKey(key)}

	if ownerKeys, err := redisq.Client.Keys(redisq.Ctx, "hunt:feature_flags:*:"+key).Result(); err == nil {
		keys = append(keys, ownerKeys...)
	}

	if len(keys) > 0 {
		_ = redisq.Client.Del(redisq.Ctx, keys...).Err()
	}
}

func InvalidateOwner(ownerKey string) {
	if redisq.Client == nil {
		return
	}

	ownerKey = strings.TrimSpace(ownerKey)
	if ownerKey == "" {
		return
	}

	keys, err := redisq.Client.Keys(redisq.Ctx, "hunt:feature_flags:"+ownerKey+":*").Result()
	if err == nil && len(keys) > 0 {
		_ = redisq.Client.Del(redisq.Ctx, keys...).Err()
	}
}

func InvalidateAll() {
	if redisq.Client == nil {
		return
	}

	keys, err := redisq.Client.Keys(redisq.Ctx, "hunt:feature_flags:*").Result()
	if err == nil && len(keys) > 0 {
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

func cacheKeyForOwner(ownerKey string, key string) string {
	return "hunt:feature_flags:" + strings.TrimSpace(ownerKey) + ":" + strings.TrimSpace(key)
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
