package config

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

const (
	KeyMaxConcurrentScans                 = "max_concurrent_scans"
	KeyTelegramNotificationsEnabled       = "telegram_notifications_enabled"
	KeyTelegramNotificationEvents         = "telegram_notification_events"
	KeyTelegramFreshAssetScreenshot       = "telegram_fresh_asset_screenshot_enabled"

	TelegramEventFreshAsset              = "fresh_asset"
	TelegramEventAssetChangeIsLive       = "asset_change_is_live"
	TelegramEventAssetChangeStatusCode   = "asset_change_status_code"
	TelegramEventAssetChangeTitle        = "asset_change_title"
	TelegramEventAssetChangeWebServer    = "asset_change_web_server"
	TelegramEventAssetChangeTechnologies = "asset_change_technologies"
	TelegramEventAssetChangeHostIP       = "asset_change_host_ip"
	TelegramEventFreshURL                = "fresh_url"
)

var DefaultTelegramNotificationEvents = []string{
	TelegramEventFreshAsset,
	TelegramEventAssetChangeIsLive,
	TelegramEventAssetChangeStatusCode,
	TelegramEventAssetChangeTitle,
	TelegramEventAssetChangeWebServer,
	TelegramEventAssetChangeTechnologies,
	TelegramEventAssetChangeHostIP,
	TelegramEventFreshURL,
}

// GetMaxConcurrentScans retrieves the maximum number of concurrent scans allowed.
func GetMaxConcurrentScans() int {
	val, ok := GetInt(KeyMaxConcurrentScans)
	if !ok || val <= 0 {
		return 2
	}

	return val
}

func GetString(key string) (string, bool) {
	var cfg models.SystemConfig
	if err := database.DB.First(&cfg, "key = ?", key).Error; err != nil {
		return "", false
	}

	return cfg.Value, true
}

func GetBool(key string, defaultValue bool) bool {
	raw, ok := GetString(key)
	if !ok {
		return defaultValue
	}

	raw = strings.TrimSpace(strings.ToLower(raw))
	switch raw {
	case "1", "true", "yes", "on", "enabled":
		return true
	case "0", "false", "no", "off", "disabled":
		return false
	default:
		return defaultValue
	}
}

func GetInt(key string) (int, bool) {
	raw, ok := GetString(key)
	if !ok {
		return 0, false
	}

	value, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil {
		return 0, false
	}

	return value, true
}

func GetStringList(key string, defaultValue []string) []string {
	raw, ok := GetString(key)
	if !ok || strings.TrimSpace(raw) == "" {
		return append([]string{}, defaultValue...)
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return normalizeStringList(values)
	}

	return normalizeStringList(strings.Split(raw, ","))
}

func TelegramNotificationsEnabled() bool {
	return GetBool(KeyTelegramNotificationsEnabled, true)
}

func TelegramFreshAssetScreenshotEnabled() bool {
	return GetBool(KeyTelegramFreshAssetScreenshot, false)
}

func TelegramEventEnabled(event string) bool {
	event = strings.TrimSpace(event)
	if event == "" {
		return false
	}

	if !TelegramNotificationsEnabled() {
		return false
	}

	enabled := GetStringList(KeyTelegramNotificationEvents, DefaultTelegramNotificationEvents)
	for _, candidate := range enabled {
		if candidate == event {
			return true
		}
	}

	return false
}

func FieldToTelegramEvent(field string) string {
	switch field {
	case "is_live":
		return TelegramEventAssetChangeIsLive
	case "status_code":
		return TelegramEventAssetChangeStatusCode
	case "title":
		return TelegramEventAssetChangeTitle
	case "web_server":
		return TelegramEventAssetChangeWebServer
	case "technologies":
		return TelegramEventAssetChangeTechnologies
	case "host_ip":
		return TelegramEventAssetChangeHostIP
	default:
		return "asset_change_" + strings.TrimSpace(strings.ToLower(field))
	}
}

func normalizeStringList(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		out = append(out, value)
	}

	return out
}
