package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// TargetPolicy stores target-specific scope and safety rules used by AI agents,
// reporting, and future approval-gated workflows. It is descriptive policy data;
// it must not directly authorize unsafe execution without deterministic checks.
type TargetPolicy struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;uniqueIndex;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	PlatformName string `gorm:"size:128;index" json:"platform_name"`
	ProgramURL   string `gorm:"size:1024" json:"program_url"`

	InScopePatterns    datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"in_scope_patterns"`
	OutOfScopePatterns datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"out_of_scope_patterns"`

	AllowedTestTypes    datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"allowed_test_types"`
	DisallowedTestTypes datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"disallowed_test_types"`

	MaxTestIntensity string `gorm:"size:32;not null;default:'safe';index" json:"max_test_intensity"`
	RateLimitNotes   string `gorm:"type:text" json:"rate_limit_notes"`
	AuthRequired     bool   `gorm:"default:false" json:"auth_required"`
	SafeTestingNotes string `gorm:"type:text" json:"safe_testing_notes"`

	ReportingPreferences string `gorm:"type:text" json:"reporting_preferences"`
	BusinessContext      string `gorm:"type:text" json:"business_context"`

	AssetCriticalityDefault string `gorm:"size:32;not null;default:'medium';index" json:"asset_criticality_default"`
}
