package models

import (
	"time"

	"gorm.io/gorm"
)

// TargetScanState stores persistent scan progress/checkpoints per target.
// This enables safe resume after crashes or user pause without repeating completed steps.
type TargetScanState struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint `gorm:"not null;uniqueIndex" json:"target_id"`

	// RunID changes when the user explicitly restarts a scan from scratch.
	RunID string `gorm:"size:64;not null;default:''" json:"run_id"`

	// Status: RUNNING | PAUSED | FAILED | COMPLETED
	Status string `gorm:"size:32;not null;default:'PAUSED';index" json:"status"`

	// Module: DISCOVERY | PROBING | CRAWLING
	CurrentModule string `gorm:"size:32;not null;default:'';index" json:"current_module"`
	// Step examples: PASSIVE_ENUM, ALTERX, PUREDNS, DNSX, SAVE, CDNCHECK, NMAP, HTTPX_BATCH, WAYBACK, GAU, WAYMORE, KATANA, FILTER_SAVE
	CurrentStep string `gorm:"size:64;not null;default:''" json:"current_step"`

	// CompletedSteps is a JSON array of step keys, e.g. ["DISCOVERY:PASSIVE_ENUM","DISCOVERY:DNSX",...]
	CompletedSteps string `gorm:"type:jsonb;default:'[]'" json:"completed_steps"`

	// Meta stores module-specific progress (e.g., probing offsets) as JSON.
	Meta string `gorm:"type:jsonb;default:'{}'" json:"meta"`

	HeartbeatAt *time.Time `gorm:"index" json:"heartbeat_at"`
	LastError   string     `gorm:"type:text" json:"last_error"`
}


