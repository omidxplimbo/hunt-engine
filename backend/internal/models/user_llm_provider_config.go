package models

import (
	"time"

	"gorm.io/gorm"
)

// UserLLMProviderConfig stores account-scoped LLM provider settings.
// Non-admin users get owner_key=user:<id>. Admins share owner_key=admin.
type UserLLMProviderConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	OwnerKey string `gorm:"size:80;not null;uniqueIndex:idx_llm_owner_provider,priority:1;index" json:"owner_key"`
	Provider string `gorm:"size:50;not null;uniqueIndex:idx_llm_owner_provider,priority:2" json:"provider"`

	DisplayName  string `gorm:"size:120" json:"display_name"`
	APIKey       string `gorm:"type:text" json:"-"`
	BaseURL      string `gorm:"size:512" json:"base_url"`
	DefaultModel string `gorm:"size:160" json:"default_model"`

	Enabled   bool `gorm:"default:true;index" json:"enabled"`
	IsDefault bool `gorm:"default:false;index" json:"is_default"`
}
