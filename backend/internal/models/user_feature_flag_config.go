package models

import (
	"time"

	"gorm.io/gorm"
)

// UserFeatureFlagConfig stores account-scoped feature flag overrides.
// Missing row or state=inherit means: use global SystemConfig/default value.
// Admin users share owner_key=admin. Non-admin users use owner_key=user:<id>.
type UserFeatureFlagConfig struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   *uint  `gorm:"index" json:"user_id"`
	OwnerKey string `gorm:"size:80;not null;uniqueIndex:idx_feature_owner_key,priority:1;index" json:"owner_key"`

	FeatureKey string `gorm:"size:120;not null;uniqueIndex:idx_feature_owner_key,priority:2;index" json:"feature_key"`
	State      string `gorm:"size:16;not null;default:'inherit';index" json:"state"`
}
