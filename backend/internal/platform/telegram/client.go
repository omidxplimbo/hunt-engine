package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/screenshot"
)

const QueueSize = 50000
const RateLimit = 1 * time.Second

const (
	EventFreshAsset              = "fresh_asset"
	EventAssetChangeIsLive       = "asset_change_is_live"
	EventAssetChangeStatusCode   = "asset_change_status_code"
	EventAssetChangeTitle        = "asset_change_title"
	EventAssetChangeWebServer    = "asset_change_web_server"
	EventAssetChangeTechnologies = "asset_change_technologies"
	EventAssetChangeHostIP       = "asset_change_host_ip"
	EventFreshURL                = "fresh_url"
)

type queuedMessage struct {
	Text             string
	PhotoPath        string
	DeletePhotoAfter bool
	BotToken         string
	ChatID           string
}

type resolvedConfig struct {
	OwnerKey                    string
	BotToken                    string
	ChatID                      string
	Enabled                     bool
	EnabledEvents               map[string]struct{}
	FreshAssetScreenshotEnabled bool
}

type cachedResolvedConfig struct {
	OwnerKey                    string   `json:"owner_key"`
	BotToken                    string   `json:"bot_token"`
	ChatID                      string   `json:"chat_id"`
	Enabled                     bool     `json:"enabled"`
	EnabledEvents               []string `json:"enabled_events"`
	FreshAssetScreenshotEnabled bool     `json:"fresh_asset_screenshot_enabled"`
}

const telegramConfigCacheTTL = 5 * time.Minute

var messageQueue = make(chan queuedMessage, QueueSize)

func Init() {
	go processQueue()
	log.Println("🚀 Telegram Notification Worker started (per-user config mode).")
}

func processQueue() {
	ticker := time.NewTicker(RateLimit)
	defer ticker.Stop()

	for item := range messageQueue {
		<-ticker.C

		if item.PhotoPath != "" {
			performPhotoRequest(item, item.DeletePhotoAfter)
			continue
		}

		performMessageRequest(item)
	}
}

func SendChangeAlert(ownerUserID uint, targetDomain, assetValue, field, oldVal, newVal string) {
	event := FieldToEvent(field)
	cfg, ok := resolveConfigForEvent(ownerUserID, event)
	if !ok {
		return
	}

	emoji := "🔄"
	if field == "status_code" {
		emoji = "🚦"
	} else if field == "is_live" {
		emoji = "🔌"
	}

	msg := fmt.Sprintf(
		"%s *ASSET CHANGED* %s\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *Asset:* `%s`\n"+
			"--------------------------------\n"+
			"🔹 *Field:* `%s`\n"+
			"🔻 *Old:* `%s`\n"+
			"🔺 *New:* `%s`\n"+
			"--------------------------------\n"+
			"⏰ _%s_",
		emoji, emoji, targetDomain, assetValue, field, oldVal, newVal,
		time.Now().Format("15:04:05"),
	)

	enqueue(cfg, queuedMessage{Text: msg})
}

func SendNewAssetAlert(targetDomain, assetValue string) {
	SendNewAssetAlertWithScreenshot(0, 0, targetDomain, assetValue, "")
}

func SendNewAssetAlertWithScreenshot(ownerUserID uint, targetID uint, targetDomain, assetValue, pageURL string) {
	cfg, ok := resolveConfigForEvent(ownerUserID, EventFreshAsset)
	if !ok {
		return
	}

	msg := fmt.Sprintf(
		"🚨 *FRESH ASSET* 🚨\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *Asset:* `%s`\n\n"+
			"⏰ _%s_",
		targetDomain, assetValue, time.Now().Format("15:04:05"),
	)

	if !cfg.FreshAssetScreenshotEnabled || ownerUserID == 0 || targetID == 0 {
		enqueue(cfg, queuedMessage{Text: msg})
		return
	}

	go func() {
		photoPath, err := screenshot.CaptureFreshAsset(ownerUserID, targetID, assetValue, pageURL)
		if err != nil {
			log.Printf("⚠️ Fresh asset screenshot failed for %s: %v\n", assetValue, err)
			enqueue(cfg, queuedMessage{Text: msg})
			return
		}

		enqueue(cfg, queuedMessage{
			Text:             msg,
			PhotoPath:        photoPath,
			DeletePhotoAfter: true,
		})
	}()
}

func SendNewURLAlert(ownerUserID uint, targetDomain, url, source string) {
	cfg, ok := resolveConfigForEvent(ownerUserID, EventFreshURL)
	if !ok {
		return
	}

	msg := fmt.Sprintf(
		"🕷 *FRESH CRAWL URL* 🕷\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *URL:* `%s`\n"+
			"🧭 *Source:* `%s`\n\n"+
			"⏰ _%s_",
		targetDomain, url, source, time.Now().Format("15:04:05"),
	)

	enqueue(cfg, queuedMessage{Text: msg})
}

func FieldToEvent(field string) string {
	switch field {
	case "is_live":
		return EventAssetChangeIsLive
	case "status_code":
		return EventAssetChangeStatusCode
	case "title":
		return EventAssetChangeTitle
	case "web_server":
		return EventAssetChangeWebServer
	case "technologies":
		return EventAssetChangeTechnologies
	case "host_ip":
		return EventAssetChangeHostIP
	default:
		return "asset_change_" + strings.TrimSpace(strings.ToLower(field))
	}
}

func enqueue(cfg resolvedConfig, item queuedMessage) {
	item.BotToken = cfg.BotToken
	item.ChatID = cfg.ChatID

	select {
	case messageQueue <- item:
	default:
		log.Println("⚠️ Telegram queue full. Dropping notification.")
		if item.DeletePhotoAfter && item.PhotoPath != "" {
			_ = os.Remove(item.PhotoPath)
		}
	}
}

func resolveConfigForEvent(ownerUserID uint, event string) (resolvedConfig, bool) {
	cfg, ok := resolveConfig(ownerUserID)
	if !ok || !cfg.Enabled || strings.TrimSpace(cfg.BotToken) == "" || strings.TrimSpace(cfg.ChatID) == "" {
		return cfg, false
	}

	if _, enabled := cfg.EnabledEvents[event]; !enabled {
		return cfg, false
	}

	return cfg, true
}

func resolveConfig(ownerUserID uint) (resolvedConfig, bool) {
	ownerKey := resolveOwnerKey(ownerUserID)
	if ownerKey == "" {
		return resolvedConfig{}, false
	}

	if cfg, ok := getCachedConfig(ownerKey); ok {
		return cfg, true
	}

	var row models.UserTelegramConfig
	if err := database.DB.Where("owner_key = ?", ownerKey).First(&row).Error; err != nil {
		return resolvedConfig{}, false
	}

	cfg := resolvedConfig{
		OwnerKey:                    ownerKey,
		BotToken:                    row.BotToken,
		ChatID:                      row.ChatID,
		Enabled:                     row.Enabled,
		EnabledEvents:               parseEvents(row.EnabledEvents),
		FreshAssetScreenshotEnabled: row.FreshAssetScreenshotEnabled,
	}

	setCachedConfig(cfg)

	return cfg, true
}

func resolveOwnerKey(ownerUserID uint) string {
	if ownerUserID == 0 {
		return "admin"
	}

	cacheKey := telegramOwnerCacheKey(ownerUserID)
	if redisq.Client != nil {
		if value, err := redisq.Client.Get(context.Background(), cacheKey).Result(); err == nil && strings.TrimSpace(value) != "" {
			return value
		}
	}

	ownerKey := fmt.Sprintf("user:%d", ownerUserID)

	var user models.User
	if err := database.DB.Select("id", "role").First(&user, ownerUserID).Error; err == nil {
		if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
			ownerKey = "admin"
		}
	}

	if redisq.Client != nil {
		_ = redisq.Client.Set(context.Background(), cacheKey, ownerKey, telegramConfigCacheTTL).Err()
	}

	return ownerKey
}

func getCachedConfig(ownerKey string) (resolvedConfig, bool) {
	if redisq.Client == nil {
		return resolvedConfig{}, false
	}

	raw, err := redisq.Client.Get(context.Background(), telegramConfigCacheKey(ownerKey)).Result()
	if err != nil || strings.TrimSpace(raw) == "" {
		return resolvedConfig{}, false
	}

	var cached cachedResolvedConfig
	if err := json.Unmarshal([]byte(raw), &cached); err != nil {
		return resolvedConfig{}, false
	}

	return resolvedConfig{
		OwnerKey:                    cached.OwnerKey,
		BotToken:                    cached.BotToken,
		ChatID:                      cached.ChatID,
		Enabled:                     cached.Enabled,
		EnabledEvents:               eventsToSet(cached.EnabledEvents),
		FreshAssetScreenshotEnabled: cached.FreshAssetScreenshotEnabled,
	}, true
}

func setCachedConfig(cfg resolvedConfig) {
	if redisq.Client == nil || strings.TrimSpace(cfg.OwnerKey) == "" {
		return
	}

	cached := cachedResolvedConfig{
		OwnerKey:                    cfg.OwnerKey,
		BotToken:                    cfg.BotToken,
		ChatID:                      cfg.ChatID,
		Enabled:                     cfg.Enabled,
		EnabledEvents:               setToEvents(cfg.EnabledEvents),
		FreshAssetScreenshotEnabled: cfg.FreshAssetScreenshotEnabled,
	}

	data, err := json.Marshal(cached)
	if err != nil {
		return
	}

	_ = redisq.Client.Set(context.Background(), telegramConfigCacheKey(cfg.OwnerKey), string(data), telegramConfigCacheTTL).Err()
}

// InvalidateConfigCache clears cached Telegram notification settings after account-level updates.
func InvalidateConfigCache(ownerKey string, userID *uint) {
	if redisq.Client == nil {
		return
	}

	ctx := context.Background()

	if strings.TrimSpace(ownerKey) != "" {
		_ = redisq.Client.Del(ctx, telegramConfigCacheKey(ownerKey)).Err()
	}

	if userID != nil && *userID > 0 {
		_ = redisq.Client.Del(ctx, telegramOwnerCacheKey(*userID)).Err()
	}
}

func telegramConfigCacheKey(ownerKey string) string {
	return "telegram:config:" + ownerKey
}

func telegramOwnerCacheKey(userID uint) string {
	return fmt.Sprintf("telegram:owner:%d", userID)
}

func eventsToSet(events []string) map[string]struct{} {
	out := map[string]struct{}{}
	for _, event := range events {
		event = strings.TrimSpace(event)
		if event != "" {
			out[event] = struct{}{}
		}
	}
	return out
}

func setToEvents(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for event := range values {
		if strings.TrimSpace(event) != "" {
			out = append(out, event)
		}
	}
	return out
}

func parseEvents(raw string) map[string]struct{} {
	out := map[string]struct{}{}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return out
	}

	var events []string
	if err := json.Unmarshal([]byte(raw), &events); err != nil {
		return out
	}

	for _, event := range events {
		event = strings.TrimSpace(event)
		if event != "" {
			out[event] = struct{}{}
		}
	}

	return out
}

func performMessageRequest(item queuedMessage) {
	if item.BotToken == "" || item.ChatID == "" {
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", item.BotToken)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"chat_id":                  item.ChatID,
		"text":                     item.Text,
		"parse_mode":               "Markdown",
		"disable_web_page_preview": true,
	})

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("❌ Failed to send Telegram message: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Telegram API error: Status %d\n", resp.StatusCode)
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(5 * time.Second)
		}
	}
}

func performPhotoRequest(item queuedMessage, deleteAfter bool) {
	if deleteAfter {
		defer func() {
			if err := os.Remove(item.PhotoPath); err != nil && !os.IsNotExist(err) {
				log.Printf("⚠️ Failed to delete Telegram screenshot %s: %v\n", item.PhotoPath, err)
			}
		}()
	}

	if item.BotToken == "" || item.ChatID == "" {
		return
	}

	file, err := os.Open(item.PhotoPath)
	if err != nil {
		log.Printf("❌ Failed to open Telegram photo %s: %v\n", item.PhotoPath, err)
		return
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("chat_id", item.ChatID)
	_ = writer.WriteField("caption", item.Text)
	_ = writer.WriteField("parse_mode", "Markdown")

	part, err := writer.CreateFormFile("photo", filepath.Base(item.PhotoPath))
	if err != nil {
		log.Printf("❌ Failed to create Telegram photo form: %v\n", err)
		return
	}

	if _, err := io.Copy(part, file); err != nil {
		log.Printf("❌ Failed to read Telegram photo %s: %v\n", item.PhotoPath, err)
		return
	}

	if err := writer.Close(); err != nil {
		log.Printf("❌ Failed to close Telegram photo form: %v\n", err)
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", item.BotToken)

	req, err := http.NewRequest(http.MethodPost, apiURL, &body)
	if err != nil {
		log.Printf("❌ Failed to create Telegram photo request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("❌ Failed to send Telegram photo: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("❌ Telegram photo API error: Status %d\n", resp.StatusCode)
		if resp.StatusCode == http.StatusTooManyRequests {
			time.Sleep(5 * time.Second)
		}
	}
}
