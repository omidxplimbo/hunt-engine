package telegram

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"
)

// 👇 افزایش ظرفیت صف به ۵۰ هزار پیام
const QueueSize = 50000

// 👇 سرعت ارسال (۱ پیام در ثانیه)
const RateLimit = 1 * time.Second

// کانال بافر دار
var messageQueue = make(chan string, QueueSize)

func Init() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		log.Println("⚠️ Telegram config missing. Notification system disabled.")
		return
	}

	// روشن کردن پردازشگر پس‌زمینه
	go processQueue()
	log.Println("🚀 Telegram Notification Worker started (No-Drop Mode).")
}

func processQueue() {
	ticker := time.NewTicker(RateLimit)
	defer ticker.Stop()

	for msg := range messageQueue {
		// صبر برای تیکر (Rate Limit)
		<-ticker.C

		// ارسال واقعی
		performHTTPRequest(msg)
	}
}

func SendMessage(text string) {
	messageQueue <- text
}

func performHTTPRequest(text string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
		// جلوگیری از پیش‌نمایش لینک‌ها برای تمیزتر شدن چت
		"disable_web_page_preview": true,
	})

	client := http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(reqBody))

	if err != nil {
		log.Printf("❌ Failed to send Telegram message: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("❌ Telegram API error: Status %d\n", resp.StatusCode)
		if resp.StatusCode == 429 {
			log.Println("⚠️ Rate limit hit! Slowing down...")
			time.Sleep(5 * time.Second)
		}
	}
}

// SendChangeAlert فرمت پیام تغییر
func SendChangeAlert(targetDomain, assetValue, field, oldVal, newVal string) {
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
	SendMessage(msg)
}

// SendNewAssetAlert فرمت پیام دارایی جدید
func SendNewAssetAlert(targetDomain, assetValue string) {
	msg := fmt.Sprintf(
		"🚨 *FRESH ASSET* 🚨\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *Asset:* `%s`\n\n"+
			"⏰ _%s_",
		targetDomain, assetValue, time.Now().Format("15:04:05"),
	)
	SendMessage(msg)
}

// 👇👇👇 تابع جدید برای URLهای پیدا شده
func SendNewURLAlert(targetDomain, url, source string) {
	msg := fmt.Sprintf(
		"🕷 *FRESH CRAWL URL* 🕷\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *URL:* `%s`\n"+
			"lz *Source:* `%s`\n\n"+
			"⏰ _%s_",
		targetDomain, url, source, time.Now().Format("15:04:05"),
	)
	SendMessage(msg)
}
