package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// AuditLog stores durable activity records for important user/system actions.
// It is intentionally generic so target events, AI events, report downloads,
// findings triage, and future template approvals can share one log table.
type AuditLog struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	ActorUserID *uint  `gorm:"index" json:"actor_user_id"`
	ActorUser   *User  `gorm:"foreignKey:ActorUserID" json:"-"`
	ActorName   string `gorm:"size:100" json:"actor_name"`

	Action     string `gorm:"size:128;not null;index" json:"action"`
	EntityType string `gorm:"size:64;not null;index" json:"entity_type"`
	EntityID   *uint  `gorm:"index" json:"entity_id"`

	TargetID *uint   `gorm:"index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	IPAddress string `gorm:"size:64" json:"ip_address"`
	UserAgent string `gorm:"size:512" json:"user_agent"`

	Metadata datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`
}
