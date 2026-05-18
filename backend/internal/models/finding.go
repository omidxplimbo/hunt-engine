package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	FindingSeverityInfo     = "info"
	FindingSeverityLow      = "low"
	FindingSeverityMedium   = "medium"
	FindingSeverityHigh     = "high"
	FindingSeverityCritical = "critical"
)

const (
	FindingStatusOpen          = "open"
	FindingStatusAccepted      = "accepted"
	FindingStatusFalsePositive = "false_positive"
	FindingStatusFixed         = "fixed"
)

// Finding represents a normalized, actionable security or reconnaissance signal.
// It is intentionally independent from any single tool so future sources such as
// Nuclei, takeover detection, JS intelligence, screenshots, or manual review can
// all write to the same table.
type Finding struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	AssetID *uint  `gorm:"index" json:"asset_id"`
	Asset   *Asset `gorm:"foreignKey:AssetID" json:"-"`

	URLID *uint     `gorm:"index" json:"url_id"`
	URL   *FoundURL `gorm:"foreignKey:URLID" json:"-"`

	Title           string     `gorm:"size:255;not null;index" json:"title"`
	Description     string     `gorm:"type:text" json:"description"`
	Severity        string     `gorm:"size:20;not null;default:'info';index" json:"severity"`
	Category        string     `gorm:"size:100;index" json:"category"`
	SourceTool      string     `gorm:"size:100;index" json:"source_tool"`
	Evidence        string     `gorm:"type:text" json:"evidence"`
	Recommendation  string     `gorm:"type:text" json:"recommendation"`
	Status          string     `gorm:"size:30;not null;default:'open';index" json:"status"`
	TriageNote      string     `gorm:"type:text" json:"triage_note"`
	TriagedAt       *time.Time `gorm:"index" json:"triaged_at"`
	TriagedByUserID *uint      `gorm:"index" json:"triaged_by_user_id"`

	// Fingerprint is used by later generators to de-duplicate findings across scans.
	// It is indexed but not unique yet because existing/manual records can be created
	// before generators assign stable fingerprints.
	Fingerprint string `gorm:"size:255;index" json:"fingerprint"`

	FirstSeen time.Time `gorm:"index" json:"first_seen"`
	LastSeen  time.Time `gorm:"index" json:"last_seen"`
}
