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

// SendMessage یک پیام متنی ساده به چت آیدی تنظیم شده می‌فرستد
// این تابع غیرهمگام (Async) نیست، اگر می‌خواهی برنامه قفل نکند باید در goroutine صدا زده شود
func SendMessage(text string) {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	chatID := os.Getenv("TELEGRAM_CHAT_ID")

	if token == "" || chatID == "" {
		// اگر تنظیم نشده بود، فقط لاگ می‌زنیم و رد می‌شویم
		log.Println("⚠️ Telegram config missing. Notification skipped.")
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)

	// ساخت بدنه درخواست
	reqBody, _ := json.Marshal(map[string]interface{}{
		"chat_id":    chatID,
		"text":       text,
		"parse_mode": "Markdown", // برای خوشگل‌تر شدن متن
	})

	// ارسال درخواست با تایم‌اوت کم (که اگر تلگرام فیلتر بود برنامه گیر نکند)
	client := http.Client{
		Timeout: 10 * time.Second,
	}

	resp, err := client.Post(apiURL, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("❌ Failed to send Telegram message: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Printf("❌ Telegram API error: Status %d\n", resp.StatusCode)
	} else {
		log.Println("🔔 Telegram notification sent successfully.")
	}
}

// SendChangeAlert یک فرمت اختصاصی برای اعلام تغییرات می‌سازد
func SendChangeAlert(targetDomain, assetValue, field, oldVal, newVal string) {
	// ایموجی‌های مناسب برای هر فیلد
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

	msg := fmt.Sprintf(
		"%s *ASSET CHANGED DETECTED* %s\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *Asset:* `%s`\n"+
			"--------------------------------\n"+
			"🔹 *Field:* `%s`\n"+
			"🔻 *Old:* `%s`\n"+
			"🔺 *New:* `%s`\n"+
			"--------------------------------\n"+
			"⏰ _%s_",
		emoji, emoji,
		targetDomain,
		assetValue,
		field,
		oldVal,
		newVal,
		time.Now().Format("2006-01-02 15:04:05"),
	)

	// ارسال در یک نخ جداگانه تا سرعت پردازش را نگیرد
	go SendMessage(msg)
}

// SendNewAssetAlert فرمت اختصاصی برای دارایی‌های جدید
func SendNewAssetAlert(targetDomain, assetValue string) {
	msg := fmt.Sprintf(
		"🚨 *FRESH ASSET DISCOVERED* 🚨\n\n"+
			"🎯 *Target:* `%s`\n"+
			"🔗 *Asset:* `%s`\n\n"+
			"🔍 _Scanning for details..._\n"+
			"⏰ _%s_",
		targetDomain,
		assetValue,
		time.Now().Format("2006-01-02 15:04:05"),
	)
	go SendMessage(msg)
}
