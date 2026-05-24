package models

import "time"

type UserTelegramConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID   *uint  `gorm:"index" json:"user_id"`
	OwnerKey string `gorm:"uniqueIndex;not null;size:80" json:"owner_key"`

	BotToken string `gorm:"type:text" json:"-"`
	ChatID   string `gorm:"type:text" json:"-"`

	Enabled                     bool   `gorm:"default:false" json:"enabled"`
	EnabledEvents               string `gorm:"type:text" json:"enabled_events"`
	FreshAssetScreenshotEnabled bool   `gorm:"default:false" json:"fresh_asset_screenshot_enabled"`
}
