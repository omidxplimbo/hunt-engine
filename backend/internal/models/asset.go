package models

import (
	"time"

	"gorm.io/gorm"
)

// ==========================================
// Target Model
// ==========================================
type Target struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Name        string `gorm:"size:100;not null" json:"name"`
	RootDomain  string `gorm:"size:255;not null;uniqueIndex" json:"root_domain"`
	Description string `json:"description"`
	InScope     bool   `gorm:"default:true" json:"in_scope"`

	// تنظیمات زمان‌بندی (دقیقه)
	Frequency  int        `gorm:"default:0" json:"frequency"`
	LastScanAt *time.Time `json:"last_scan_at"`
	Status     string     `gorm:"size:50;default:'READY'" json:"status"`
	ScanCount  int        `gorm:"default:0" json:"scan_count"`
	// 👇👇👇 فیلد جدید: لیست ماژول‌های انتخابی برای اجرا
	// مثال: ["DISCOVERY", "PROBING"]
	// پیش‌فرض: فقط دیسکاوری
	ScanModules string `gorm:"type:jsonb;default:'[\"DISCOVERY\", \"PROBING\"]'" json:"scan_modules"`

	Assets []Asset `json:"-"`
}

// ==========================================
// Asset Model
// ==========================================
type Asset struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint   `gorm:"not null;index" json:"target_id"`
	Value    string `gorm:"size:255;uniqueIndex;not null" json:"value"`
	Type     string `gorm:"size:50" json:"type"`

	IsNew  bool `gorm:"default:true;index" json:"is_new"`
	IsLive bool `gorm:"default:false;index" json:"is_live"`

	// 👇👇👇 فیلد جدید: آخرین زمانی که یک تغییر مهم در این دارایی کشف شده
	LastChangeAt *time.Time `gorm:"index" json:"last_change_at"`

	// --- فیلدهای کلیدی ---
	FinalURL       string `gorm:"size:512" json:"final_url"`
	StatusCode     int    `gorm:"index" json:"status_code"`
	Title          string `gorm:"size:512" json:"title"`
	ContentLength  int64  `json:"content_length"`
	HostIP         string `gorm:"type:jsonb;default:'[]'" json:"host_ip"` // آرایه IPها
	DnsxIP         string `gorm:"type:jsonb;default:'[]'" json:"dnsx_ip"`
	WebServer      string `gorm:"size:255" json:"web_server"`
	CDNName        string `gorm:"size:100;index" json:"cdn_name"`
	Technologies   string `gorm:"type:jsonb;default:'[]'" json:"technologies"`
	BodyHash       string `gorm:"size:64" json:"body_hash"`
	HeaderHash     string `gorm:"size:64" json:"header_hash"`
	ResponseTimeMs int64  `json:"response_time_ms"`

	// داده خام
	RawHttpx string `gorm:"type:jsonb" json:"raw_httpx"`

	// ارتباط با جدول تاریخچه
	History []AssetHistory `json:"history,omitempty"`
}

// ==========================================
// AssetHistory Model (جدول جدید برای ثبت تغییرات)
// ==========================================
// این جدول دقیقاً می‌گوید چه چیزی، کی و چگونه تغییر کرده است.
type AssetHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"` // زمان کشف تغییر

	AssetID uint   `gorm:"not null;index" json:"asset_id"`
	Asset   *Asset `json:"-"` // ارتباط برای GORM

	// FieldName: نام فیلدی که تغییر کرده (مثلاً "status_code" یا "host_ip")
	FieldName string `gorm:"size:100;not null" json:"field_name"`

	// OldValue: مقدار قبلی (به صورت رشته ذخیره می‌شود)
	OldValue string `gorm:"type:text" json:"old_value"`

	// NewValue: مقدار جدید
	NewValue string `gorm:"type:text" json:"new_value"`
}
