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
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/config"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/screenshot"
)

const QueueSize = 50000
const RateLimit = 1 * time.Second

type queuedMessage struct {
	Text             string
	PhotoPath        string
	DeletePhotoAfter bool
}

var messageQueue = make(chan queuedMessage, QueueSize)

func Init() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		log.Println("⚠️ Telegram config missing. Notification system disabled.")
		return
	}

	go processQueue()
	log.Println("🚀 Telegram Notification Worker started (No-Drop Mode).")
}

func processQueue() {
	ticker := time.NewTicker(RateLimit)
	defer ticker.Stop()

	for item := range messageQueue {
		<-ticker.C

		if item.PhotoPath != "" {
			performPhotoRequest(item.Text, item.PhotoPath, item.DeletePhotoAfter)
			continue
		}

		performMessageRequest(item.Text)
	}
}

func SendMessage(text string) {
	if !config.TelegramNotificationsEnabled() {
		return
	}

	enqueue(queuedMessage{Text: text})
}

func SendMessageForEvent(event string, text string) {
	if !config.TelegramEventEnabled(event) {
		return
	}

	enqueue(queuedMessage{Text: text})
}

func SendPhotoForEvent(event string, caption string, photoPath string, deleteAfter bool) {
	if !config.TelegramEventEnabled(event) {
		if deleteAfter {
			_ = os.Remove(photoPath)
		}
		return
	}

	enqueue(queuedMessage{
		Text:             caption,
		PhotoPath:        photoPath,
		DeletePhotoAfter: deleteAfter,
	})
}

func enqueue(item queuedMessage) {
	select {
	case messageQueue <- item:
	default:
		log.Println("⚠️ Telegram queue full. Dropping notification.")
		if item.DeletePhotoAfter && item.PhotoPath != "" {
			_ = os.Remove(item.PhotoPath)
		}
	}
}

func performMessageRequest(text string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"chat_id":                  chatID,
		"text":                     text,
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
			log.Println("⚠️ Rate limit hit! Slowing down...")
			time.Sleep(5 * time.Second)
		}
	}
}

func performPhotoRequest(caption string, photoPath string, deleteAfter bool) {
	if deleteAfter {
		defer func() {
			if err := os.Remove(photoPath); err != nil && !os.IsNotExist(err) {
				log.Printf("⚠️ Failed to delete Telegram screenshot %s: %v\n", photoPath, err)
			}
		}()
	}

	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		return
	}

	file, err := os.Open(photoPath)
	if err != nil {
		log.Printf("❌ Failed to open Telegram photo %s: %v\n", photoPath, err)
		return
	}
	defer file.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	_ = writer.WriteField("chat_id", chatID)
	_ = writer.WriteField("caption", caption)
	_ = writer.WriteField("parse_mode", "Markdown")

	part, err := writer.CreateFormFile("photo", filepath.Base(photoPath))
	if err != nil {
		log.Printf("❌ Failed to create Telegram photo form: %v\n", err)
		return
	}

	if _, err := io.Copy(part, file); err != nil {
		log.Printf("❌ Failed to read Telegram photo %s: %v\n", photoPath, err)
		return
	}

	if err := writer.Close(); err != nil {
		log.Printf("❌ Failed to close Telegram photo form: %v\n", err)
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendPhoto", token)

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

func SendChangeAlert(targetDomain, assetValue, field, oldVal, newVal string) {
	event := config.FieldToTelegramEvent(field)
	if !config.TelegramEventEnabled(event) {
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

	SendMessageForEvent(event, msg)
}

func SendNewAssetAlert(targetDomain, assetValue string) {
	SendNewAssetAlertWithScreenshot(0, 0, targetDomain, assetValue, "")
}

func SendNewAssetAlertWithScreenshot(userID uint, targetID uint, targetDomain, assetValue, pageURL string) {
	if !config.TelegramEventEnabled(config.TelegramEventFreshAsset) {
		return
	}

	msg := fmt.Sprintf(
		"🚨 *FRESH ASSET* 🚨\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *Asset:* `%s`\n\n"+
			"⏰ _%s_",
		targetDomain, assetValue, time.Now().Format("15:04:05"),
	)

	if !config.TelegramFreshAssetScreenshotEnabled() || userID == 0 || targetID == 0 {
		SendMessageForEvent(config.TelegramEventFreshAsset, msg)
		return
	}

	go func() {
		photoPath, err := screenshot.CaptureFreshAsset(userID, targetID, assetValue, pageURL)
		if err != nil {
			log.Printf("⚠️ Fresh asset screenshot failed for %s: %v\n", assetValue, err)
			SendMessageForEvent(config.TelegramEventFreshAsset, msg)
			return
		}

		SendPhotoForEvent(config.TelegramEventFreshAsset, msg, photoPath, true)
	}()
}

func SendNewURLAlert(targetDomain, url, source string) {
	if !config.TelegramEventEnabled(config.TelegramEventFreshURL) {
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

	SendMessageForEvent(config.TelegramEventFreshURL, msg)
}
