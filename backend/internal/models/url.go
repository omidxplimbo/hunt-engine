package models

import (
	"time"

	"gorm.io/gorm"
)

type FoundURL struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint `gorm:"not null;index" json:"target_id"`
	// اگر بدانیم این URL متعلق به کدام ساب‌دامین است، اینجا ذخیره می‌کنیم (اختیاری)
	AssetID *uint `gorm:"index" json:"asset_id"`

	// آدرس پیدا شده (یونیک بودن در سطح تارگت برای جلوگیری از تکرار)
	Value string `gorm:"type:text;not null" json:"value"`

	// ابزاری که این را پیدا کرده (مثلاً: "wayback,gau")
	Source string `gorm:"size:100" json:"source"`
}
