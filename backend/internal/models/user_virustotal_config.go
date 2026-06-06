package models

import "time"

type UserVirusTotalConfig struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	UserID   *uint  `gorm:"index" json:"user_id"`
	OwnerKey string `gorm:"uniqueIndex;not null;size:80" json:"owner_key"`

	APIKey string `gorm:"type:text" json:"-"`

	Enabled bool `gorm:"default:false" json:"enabled"`
}
