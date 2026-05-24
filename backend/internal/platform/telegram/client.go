package telegram

import (
	"bytes"
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
	ownerKey := "admin"

	if ownerUserID > 0 {
		var user models.User
		if err := database.DB.First(&user, ownerUserID).Error; err == nil {
			if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
				ownerKey = "admin"
			} else {
				ownerKey = fmt.Sprintf("user:%d", user.ID)
			}
		} else {
			ownerKey = fmt.Sprintf("user:%d", ownerUserID)
		}
	}

	var row models.UserTelegramConfig
	if err := database.DB.Where("owner_key = ?", ownerKey).First(&row).Error; err != nil {
		return resolvedConfig{}, false
	}

	events := parseEvents(row.EnabledEvents)

	return resolvedConfig{
		OwnerKey:                    ownerKey,
		BotToken:                    row.BotToken,
		ChatID:                      row.ChatID,
		Enabled:                     row.Enabled,
		EnabledEvents:               events,
		FreshAssetScreenshotEnabled: row.FreshAssetScreenshotEnabled,
	}, true
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
