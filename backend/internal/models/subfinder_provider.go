package models

import (
	"time"

	"gorm.io/gorm"
)

// SubfinderProviderConfig stores user-scoped provider credentials/config entries for subfinder.
//
// Each provider maps to a JSON array that will be emitted into subfinder's provider-config.yaml:
//   provider: [ ...entries... ]
//
// Most providers accept a simple API key string (entries = ["API_KEY"]), while some support
// more complex objects (entries = [{"username":"...","key":"..."}]).
type SubfinderProviderConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"index;not null;uniqueIndex:idx_user_provider" json:"user_id"`
	Provider string `gorm:"size:64;not null;uniqueIndex:idx_user_provider" json:"provider"`

	// JSON array of entries for this provider. Default empty array.
	Entries string `gorm:"type:jsonb;default:'[]'" json:"entries"`
}


