package models

import (
	"time"

	"gorm.io/gorm"
)

type UserWordlistImportJob struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID uint  `gorm:"not null;index" json:"user_id"`
	User   *User `gorm:"foreignKey:UserID" json:"-"`

	WordlistID *uint         `gorm:"index" json:"wordlist_id,omitempty"`
	Wordlist   *UserWordlist `gorm:"foreignKey:WordlistID" json:"-"`

	DisplayName string `gorm:"size:255;not null" json:"display_name"`
	SourceURL   string `gorm:"size:2048;not null" json:"source_url"`

	Status       string `gorm:"size:32;not null;default:'queued';index" json:"status"`
	ErrorMessage string `gorm:"type:text" json:"error_message,omitempty"`

	BytesDownloaded int64  `gorm:"not null;default:0" json:"bytes_downloaded"`
	SizeBytes       int64  `gorm:"not null;default:0" json:"size_bytes"`
	Lines           int64  `gorm:"not null;default:0" json:"lines"`
	SHA256          string `gorm:"size:64;index" json:"sha256,omitempty"`
	StoragePath     string `gorm:"size:1024" json:"storage_path,omitempty"`

	StartedAt   *time.Time `gorm:"index" json:"started_at,omitempty"`
	CompletedAt *time.Time `gorm:"index" json:"completed_at,omitempty"`
}
