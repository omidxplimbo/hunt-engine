package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// HuntEvidence persists a single piece of evidence produced by the hunter
// agent loop, workers, or validators.
type HuntEvidence struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`
	TargetID uint   `gorm:"not null;index" json:"target_id"`

	AgentID string `gorm:"size:64;index" json:"agent_id"` // which agent/worker produced it

	TestType  string `gorm:"size:48;not null;index" json:"test_type"` // xss, sqli, idor, ssrf...
	Target    string `gorm:"size:512" json:"target"`
	Parameter string `gorm:"size:128" json:"parameter"`
	Payload   string `gorm:"type:text" json:"payload"`

	Status     string  `gorm:"size:24;not null;default:'candidate';index" json:"status"` // candidate/testing/confirmed/false_positive
	Confidence float64 `gorm:"not null;default:0" json:"confidence"`                     // 0.0 - 1.0
	Severity   string  `gorm:"size:16" json:"severity"`                                  // info/low/medium/high/critical
	PoC        string  `gorm:"type:text" json:"poc,omitempty"`
	Notes      string  `gorm:"type:text" json:"notes,omitempty"`

	// Full structured evidence (HTTPResult / validator output) as JSONB
	Data datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"data"`
}

func (HuntEvidence) TableName() string { return "hunt_evidence" }
