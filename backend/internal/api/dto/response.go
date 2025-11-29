package dto

import "time"

// TargetResponse ساختار خروجی برای اطلاعات تارگت
type TargetResponse struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	RootDomain  string    `json:"root_domain"`
	Description string    `json:"description"`
	InScope     bool      `json:"in_scope"`
	CreatedAt   time.Time `json:"created_at"`
	AssetCount  int64     `json:"asset_count"`
}

// AssetResponse ساختار خروجی برای دارایی‌ها
type AssetResponse struct {
	ID        uint      `json:"id"`
	Value     string    `json:"value"`
	Type      string    `json:"type"`
	IsNew     bool      `json:"is_new"`
	IsLive    bool      `json:"is_live"`
	CreatedAt time.Time `json:"created_at"`

	FinalURL      string      `json:"final_url"`
	StatusCode    int         `json:"status_code"`
	Title         string      `json:"title"`
	ContentLength int64       `json:"content_length"`
	HostIP        string      `json:"host_ip"`
	DnsxIP        string      `json:"dnsx_ip"`
	WebServer     string      `json:"web_server"`
	CDNName       string      `json:"cdn_name"`
	Technologies  interface{} `json:"technologies"`
	ResponseTime  int64       `json:"response_time_ms"`

	// 👇👇👇 فیلد جدید برای نمایش کل خروجی خام httpx
	// از نوع interface{} استفاده می‌کنیم تا JSON تو در تو رو درست نشون بده
	// omitempty باعث میشه اگه داده‌ای نبود، این فیلد کلا ارسال نشه تا حجم کم بمونه
	RawHttpx interface{} `json:"raw_httpx,omitempty"`
}
