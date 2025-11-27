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

// ظرفیت صف پیام‌ها (مثلاً ۲۰۰۰ پیام در صف می‌مانند تا ارسال شوند)
const QueueSize = 2000

// Rate Limit: تلگرام حدود ۲۰ پیام در دقیقه اجازه می‌دهد.
// ما ایمن عمل می‌کنیم: هر ۳ ثانیه یک پیام.
const RateLimit = 3 * time.Second

// کانال برای نگهداری پیام‌ها (این همان صف ماست)
var messageQueue = make(chan string, QueueSize)

// Init Notification System
// این تابع باید یکبار در main.go صدا زده شود تا ورکر تلگرام روشن شود
func Init() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		log.Println("⚠️ Telegram credentials not found. Notification system disabled.")
		return
	}

	// روشن کردن پردازشگر پس‌زمینه (Worker)
	go processQueue()
	log.Println("🚀 Telegram Notification Worker started (Rate Limited).")
}

// processQueue حلقه بی‌پایانی که پیام‌ها را یکی‌یکی برداشته و ارسال می‌کند
func processQueue() {
	// استفاده از Ticker برای کنترل دقیق زمان‌بندی
	ticker := time.NewTicker(RateLimit)
	defer ticker.Stop()

	for msg := range messageQueue {
		// منتظر می‌مانیم تا تیکر اجازه دهد (هر ۳ ثانیه یکبار)
		<-ticker.C

		// حالا پیام را واقعاً به API می‌فرستیم
		performHTTPRequest(msg)
	}
}

// SendMessage پیام را به صف اضافه می‌کند (Non-blocking)
// این تابع دیگر مستقیماً به تلگرام وصل نمی‌شود، فقط پیام را در صف می‌اندازد
func SendMessage(text string) {
	// چک می‌کنیم صف پر نشده باشد تا برنامه قفل نکند
	select {
	case messageQueue <- text:
		// پیام با موفقیت به صف رفت
	default:
		log.Println("❌ Telegram Queue is FULL! Dropping notification to prevent blocking.")
	}
}

// performHTTPRequest کار واقعی ارسال به سرور تلگرام را انجام می‌دهد
func performHTTPRequest(text string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	reqBody, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown",
	})

	client := http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(reqBody))

	if err != nil {
		log.Printf("❌ Failed to send Telegram message: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("❌ Telegram API error: Status %d\n", resp.StatusCode)
		// در سیستم‌های پیشرفته‌تر، اینجا می‌توانیم منطق Retry داشته باشیم
	}
}

// SendChangeAlert فرمت پیام تغییر (بدون تغییر نسبت به قبل، فقط SendMessage را صدا می‌زند)
func SendChangeAlert(targetDomain, assetValue, field, oldVal, newVal string) {
	emoji := "🔄"
	if field == "status_code" {
		emoji = "🚦"
	}
	if field == "host_ip" {
		emoji = "🌍"
	}
	if field == "technologies" {
		emoji = "🛠"
	}
	if field == "title" {
		emoji = "📑"
	}
	if field == "is_live" {
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
