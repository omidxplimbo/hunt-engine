package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// نام کاربری باید یکتا باشد
	Username string `gorm:"uniqueIndex;not null;size:50" json:"username"`

	// پسورد هش شده (هیچوقت متن ساده ذخیره نمی‌کنیم)
	// json:"-" باعث می‌شود پسورد هرگز در خروجی API ارسال نشود
	Password string `gorm:"not null" json:"-"`

	Role string `gorm:"default:'viewer'" json:"role"` // admin, viewer

	// Active / Deactive: اگر false باشد کاربر نمی‌تواند وارد سیستم شود و به API دسترسی ندارد
	IsActive           bool `gorm:"default:true" json:"is_active"`
	MaxConcurrentScans int  `gorm:"default:1" json:"max_concurrent_scans"`

	// Per-user PureDNS custom wordlist quotas.
	// Value 0 means default limit for normal users. Admin users are treated as unlimited.
	WordlistMaxFileSizeBytes  int64 `gorm:"default:10485760" json:"wordlist_max_file_size_bytes"`
	WordlistMaxTotalSizeBytes int64 `gorm:"default:104857600" json:"wordlist_max_total_size_bytes"`
}
