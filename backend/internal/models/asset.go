package models

import (
	"time"

	"gorm.io/gorm"
)

// Target represents a scope or a company defined by the user.
type Target struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Name        string         `gorm:"size:255;not null" json:"name"`
	RootDomain  string         `gorm:"size:255;uniqueIndex;not null" json:"root_domain"`
	Description string         `gorm:"type:text" json:"description"`
	InScope     bool           `gorm:"default:true" json:"in_scope"`
	Assets      []Asset        `gorm:"foreignKey:TargetID" json:"assets,omitempty"`
}

// Asset represents a discovered host (subdomain, IP, etc.).
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

	// --- فیلدهای کلیدی استخراج شده (برای جستجوی سریع) ---
	FinalURL       string `gorm:"size:512" json:"final_url"`
	StatusCode     int    `gorm:"index" json:"status_code"`
	Title          string `gorm:"size:512" json:"title"`
	ContentLength  int64  `json:"content_length"`
	HostIP         string `gorm:"type:jsonb;default:'[]'" json:"host_ip"` // آرایه A records
	WebServer      string `gorm:"size:255" json:"web_server"`
	CDNName        string `gorm:"size:100;index" json:"cdn_name"`
	Technologies   string `gorm:"type:jsonb;default:'[]'" json:"technologies"`
	BodyHash       string `gorm:"size:64" json:"body_hash"`
	HeaderHash     string `gorm:"size:64" json:"header_hash"`
	ResponseTimeMs int64  `json:"response_time_ms"`

	// 👇👇👇 فیلد جدید: ذخیره کل خروجی خام httpx
	// این فیلد حاوی تمام جزئیات (cname, port, scheme, ...) خواهد بود
	RawHttpx string `gorm:"type:jsonb" json:"raw_httpx"`
}
