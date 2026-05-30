package models

import (
	"time"

	"gorm.io/gorm"
)

type UserWordlist struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID uint  `gorm:"not null;index;uniqueIndex:idx_user_wordlists_user_display_name,priority:1" json:"user_id"`
	User   *User `gorm:"foreignKey:UserID" json:"-"`

	DisplayName  string `gorm:"size:255;not null;uniqueIndex:idx_user_wordlists_user_display_name,priority:2" json:"display_name"`
	OriginalName string `gorm:"size:255" json:"original_name"`
	StoredName   string `gorm:"size:255;not null" json:"stored_name"`
	Path         string `gorm:"size:1024;not null" json:"path"`

	SizeBytes int64  `gorm:"not null;default:0" json:"size_bytes"`
	SHA256    string `gorm:"size:64;index" json:"sha256"`
	Lines     int64  `gorm:"not null;default:0" json:"lines"`

	Source    string `gorm:"size:32;not null;default:'upload';index" json:"source"`
	SourceURL string `gorm:"size:2048" json:"source_url"`

	StoragePath string `gorm:"size:1024;not null" json:"storage_path"`

	LastUsedAt *time.Time `gorm:"index" json:"last_used_at"`
}
