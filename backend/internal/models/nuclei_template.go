package models

import (
	"time"

	"gorm.io/gorm"
)

// NucleiTemplate stores custom Nuclei templates as the product source of truth.
// The filesystem copy under /data/nuclei/custom is only a runtime cache for the nuclei binary.
type NucleiTemplate struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID uint `gorm:"not null;index;uniqueIndex:idx_nuclei_templates_user_placement_name,priority:1" json:"user_id"`
	User   User `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`

	Name      string `gorm:"size:255;not null;uniqueIndex:idx_nuclei_templates_user_placement_name,priority:3" json:"name"`
	Placement string `gorm:"size:50;not null;default:'fast';uniqueIndex:idx_nuclei_templates_user_placement_name,priority:2" json:"placement"`
	Content   string `gorm:"type:text;not null" json:"content,omitempty"`

	Enabled  bool   `gorm:"default:true;index" json:"enabled"`
	Source   string `gorm:"size:50;not null;default:'manual'" json:"source"`
	Checksum string `gorm:"size:64;index" json:"checksum"`

	IsAIGenerated    bool       `gorm:"default:false" json:"is_ai_generated"`
	RequiresApproval bool       `gorm:"default:false" json:"requires_approval"`
	ApprovedAt       *time.Time `json:"approved_at"`
	ApprovedByUserID *uint      `json:"approved_by_user_id"`

	ValidationStatus string     `gorm:"size:50;not null;default:'unknown'" json:"validation_status"`
	ValidationError  string     `gorm:"type:text" json:"validation_error,omitempty"`
	LastValidatedAt  *time.Time `json:"last_validated_at"`
}
