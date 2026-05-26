package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	AIAnalysisScopeTarget  = "target"
	AIAnalysisScopeFinding = "finding"
	AIAnalysisScopeAsset   = "asset"
	AIAnalysisScopeURL     = "url"

	AIAnalysisStatusPending   = "pending"
	AIAnalysisStatusRunning   = "running"
	AIAnalysisStatusCompleted = "completed"
	AIAnalysisStatusFailed    = "failed"
	AIAnalysisStatusSkipped   = "skipped"

	AIRecommendationStatusOpen          = "open"
	AIRecommendationStatusAccepted      = "accepted"
	AIRecommendationStatusRejected      = "rejected"
	AIRecommendationStatusImplemented   = "implemented"
	AIRecommendationStatusFalsePositive = "false_positive"
)

// AIAnalysis stores normalized AI analysis output for targets, findings, assets, or URLs.
// v3.5.0 starts with the data layer only; later workers/providers will populate this table.
type AIAnalysis struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	FindingID *uint    `gorm:"index" json:"finding_id"`
	Finding   *Finding `gorm:"foreignKey:FindingID" json:"-"`

	AssetID *uint  `gorm:"index" json:"asset_id"`
	Asset   *Asset `gorm:"foreignKey:AssetID" json:"-"`

	URLID *uint     `gorm:"index" json:"url_id"`
	URL   *FoundURL `gorm:"foreignKey:URLID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	Scope         string `gorm:"size:32;not null;default:'target';index" json:"scope"`
	Source        string `gorm:"size:64;not null;default:'system';index" json:"source"`
	Provider      string `gorm:"size:64" json:"provider"`
	Model         string `gorm:"size:128" json:"model"`
	PromptVersion string `gorm:"size:64" json:"prompt_version"`

	Status       string         `gorm:"size:32;not null;default:'pending';index" json:"status"`
	Title        string         `gorm:"size:255" json:"title"`
	Summary      string         `gorm:"type:text" json:"summary"`
	InputDigest  string         `gorm:"size:128;index" json:"input_digest"`
	InputJSON    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	ErrorMessage string         `gorm:"type:text" json:"error_message"`

	StartedAt   *time.Time `gorm:"index" json:"started_at"`
	CompletedAt *time.Time `gorm:"index" json:"completed_at"`
}

// AIRecommendation stores actionable recommendations generated from analysis.
// Initial v3.5.0 implementation is storage/API foundation; generation comes later.
type AIRecommendation struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	FindingID *uint    `gorm:"index" json:"finding_id"`
	Finding   *Finding `gorm:"foreignKey:FindingID" json:"-"`

	AnalysisID *uint       `gorm:"index" json:"analysis_id"`
	Analysis   *AIAnalysis `gorm:"foreignKey:AnalysisID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	Source             string `gorm:"size:64;not null;default:'ai';index" json:"source"`
	RecommendationType string `gorm:"size:64;index" json:"recommendation_type"`
	Priority           string `gorm:"size:32;not null;default:'medium';index" json:"priority"`
	Confidence         string `gorm:"size:32;not null;default:'medium';index" json:"confidence"`
	Status             string `gorm:"size:32;not null;default:'open';index" json:"status"`

	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`
	Rationale   string `gorm:"type:text" json:"rationale"`

	EvidenceJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"evidence_json"`
	ActionJSON   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"action_json"`

	AcceptedAt       *time.Time `gorm:"index" json:"accepted_at"`
	AcceptedByUserID *uint      `gorm:"index" json:"accepted_by_user_id"`
}
