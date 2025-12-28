package models

import (
	"time"

	"gorm.io/gorm"
)

// Finding represents an actionable security finding tied to a target (and optionally an asset/url).
// It is designed to support triage workflows (bug bounty + enterprise ASM).
type Finding struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint `gorm:"not null;index;uniqueIndex:idx_target_finding_hash" json:"target_id"`

	// Optional links (best-effort)
	AssetID *uint `gorm:"index" json:"asset_id"`
	URLID   *uint `gorm:"index" json:"url_id"` // references FoundURL.ID

	// Core fields
	Title    string `gorm:"type:text;not null" json:"title"`
	Type     string `gorm:"size:64;index" json:"type"`         // e.g. nuclei, manual
	Severity string `gorm:"size:16;index" json:"severity"`     // info|low|medium|high|critical
	Status   string `gorm:"size:16;index;default:'open'" json:"status"` // open|triaged|duplicate|fixed|wontfix

	// Dedup
	Hash string `gorm:"size:64;not null;index;uniqueIndex:idx_target_finding_hash" json:"hash"`

	// Nuclei mapping (optional)
	TemplateID   string `gorm:"size:128;index" json:"template_id"`
	MatcherName  string `gorm:"size:128" json:"matcher_name"`
	MatchedAt    string `gorm:"size:512;index" json:"matched_at"`
	Host         string `gorm:"size:512;index" json:"host"`
	CurlCommand  string `gorm:"type:text" json:"curl_command"`

	// Additional metadata
	Tags       string `gorm:"type:jsonb;default:'[]'" json:"tags"`     // JSON array of strings
	Evidence   string `gorm:"type:jsonb;default:'{}'" json:"evidence"` // JSON object
	Notes      string `gorm:"type:text" json:"notes"`
	ReportedTo string `gorm:"size:128" json:"reported_to"`
}


