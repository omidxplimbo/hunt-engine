package dto

import "time"

// TargetResponse ساختار خروجی برای اطلاعات تارگت
// شامل تعداد کل دارایی‌ها هم میشه
type TargetResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	RootDomain  string    `json:"root_domain"`
	Description string    `json:"description"`
	InScope     bool      `json:"in_scope"`
	CreatedAt   time.Time `json:"created_at"`
	AssetCount  int64     `json:"asset_count"` // تعداد کل دارایی‌های یافت شده
}

// AssetResponse ساختار خروجی برای دارایی‌ها (ساب‌دامین‌ها)
type AssetResponse struct {
	ID        uint      `json:"id"`
	Value     string    `json:"value"` // مثلا sub.example.com
	Type      string    `json:"type"`
	IsNew     bool      `json:"is_new"`
	IsLive    bool      `json:"is_live"` // مهم‌ترین فیلد برای فیلتر کردن
	CreatedAt time.Time `json:"created_at"`
	// فیلدهای فاز بعدی رو فعلا نمی‌فرستیم تا پاسخ سبک باشه
}
