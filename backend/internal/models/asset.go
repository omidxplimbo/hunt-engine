package models

import (
	"time"

	"gorm.io/gorm"
)

// Target represents a scope or a company defined by the user.
// مثال: نام: "Company X", دامنه ریشه: "company.com"
type Target struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	RootDomain  string         `gorm:"size:255;uniqueIndex;not null" json:"root_domain"`
	Description string         `gorm:"type:text" json:"description"`
	InScope     bool           `gorm:"default:true" json:"in_scope"`

	// ارتباط یک-به-چند: یک تارگت می‌تواند چندین دارایی داشته باشد
	Assets []Asset `gorm:"foreignKey:TargetID" json:"assets,omitempty"`
}

// Asset represents a discovered host (subdomain, IP, etc.).
// این جدول اصلی ماست که نتایج ریکان اینجا ذخیره میشه.
type Asset struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// کلید خارجی برای ارتباط با جدول Target
	TargetID uint `gorm:"not null;index" json:"target_id"`

	// اطلاعات پایه
	Value string `gorm:"size:255;uniqueIndex;not null" json:"value"` // مثال: sub.example.com یا 1.2.3.4
	Type  string `gorm:"size:50" json:"type"`                        // subdomain, ip, cloud_storage

	// اطلاعات وضعیت (Probe Data) - فاز ۲
	IsLive         bool   `gorm:"default:false;index" json:"is_live"`
	StatusCode     int    `gorm:"index" json:"status_code"` // 200, 403, 404...
	Title          string `gorm:"size:512" json:"title"`
	ContentLength  int64  `json:"content_length"`
	ResponseTimeMs int64  `json:"response_time_ms"`

	// برای تشخیص تغییرات (Change Detection) - فاز ۳
	BodyHash   string `gorm:"size:64" json:"body_hash"` // SHA256 محتوای صفحه
	HeaderHash string `gorm:"size:64" json:"header_hash"`

	// اطلاعات زیرساختی
	IPAddress    string `gorm:"size:45" json:"ip_address"` // IPv4 or IPv6
	CDNProvider  string `gorm:"size:100" json:"cdn_provider"`
	Technologies string `gorm:"type:jsonb" json:"technologies"` // ذخیره لیست تکنولوژی‌ها به صورت JSON

	// فلگ‌های مهم برای باگ هانتینگ
	IsNew bool `gorm:"default:true;index" json:"is_new"` // آیا تازه کشف شده؟
}
