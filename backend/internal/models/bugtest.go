package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	BugTestRunStatusQueued          = "queued"
	BugTestRunStatusRunning         = "running"
	BugTestRunStatusCompleted       = "completed"
	BugTestRunStatusFailed          = "failed"
	BugTestRunStatusBlockedByPolicy = "blocked_by_policy"

	BugTestProfilePassive        = "passive"
	BugTestProfileSafe           = "safe"
	BugTestProfileBalanced       = "balanced"
	BugTestProfileManualApproved = "manual_approved"

	BugTestResultStatusCandidate             = "candidate"
	BugTestResultStatusNeedsManualValidation = "needs_manual_validation"
	BugTestResultStatusPassed                = "passed"
	BugTestResultStatusFailed                = "failed"
	BugTestResultStatusBlocked               = "blocked"
	BugTestResultStatusInconclusive          = "inconclusive"

	BugTypeXSS             = "xss"
	BugTypeCORS            = "cors"
	BugTypeOpenRedirect    = "open_redirect"
	BugTypeSecurityHeaders = "security_headers"
	BugTypeAPI             = "api"
	BugTypeUnknown         = "unknown"
)

type BugTestRun struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	AgentActionID *uint        `gorm:"index" json:"agent_action_id"`
	AgentAction   *AgentAction `gorm:"foreignKey:AgentActionID" json:"-"`

	Profile      string `gorm:"size:64;not null;default:'passive';index" json:"profile"`
	Status       string `gorm:"size:64;not null;default:'queued';index" json:"status"`
	PolicyStatus string `gorm:"size:64;not null;default:'unknown';index" json:"policy_status"`

	SafetyLevel int `gorm:"not null;default:0;index" json:"safety_level"`
	TestLevel   int `gorm:"not null;default:0;index" json:"test_level"`

	BugTypes  datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"bug_types"`
	OWASPRefs datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"owasp_refs"`

	InputJSON       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	PolicyCheckJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"policy_check_json"`

	ErrorMessage string `gorm:"type:text" json:"error_message"`

	StartedAt   *time.Time `gorm:"index" json:"started_at"`
	CompletedAt *time.Time `gorm:"index" json:"completed_at"`
}

type BugTestResult struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	RunID uint        `gorm:"not null;index" json:"run_id"`
	Run   *BugTestRun `gorm:"foreignKey:RunID" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	AssetID *uint  `gorm:"index" json:"asset_id"`
	Asset   *Asset `gorm:"foreignKey:AssetID" json:"-"`

	URLID *uint     `gorm:"index" json:"url_id"`
	URL   *FoundURL `gorm:"foreignKey:URLID" json:"-"`

	FindingID *uint    `gorm:"index" json:"finding_id"`
	Finding   *Finding `gorm:"foreignKey:FindingID" json:"-"`

	PatternID  *uint       `gorm:"index" json:"pattern_id"`
	Pattern    *BugPattern `gorm:"foreignKey:PatternID" json:"-"`
	PatternKey string      `gorm:"size:160;index" json:"pattern_key"`

	BugType      string `gorm:"size:96;not null;index" json:"bug_type"`
	TestName     string `gorm:"size:160;not null;index" json:"test_name"`
	Status       string `gorm:"size:64;not null;default:'candidate';index" json:"status"`
	Confidence   string `gorm:"size:64;not null;default:'low';index" json:"confidence"`
	SeverityHint string `gorm:"size:32;not null;default:'info';index" json:"severity_hint"`

	EvidenceJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"evidence_json"`
	OWASPRefs    datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"owasp_refs"`
	Tags         datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tags"`
}

// BugPattern stores safe bug testing pattern metadata.
// v3.8.0 uses a local seeded registry. Feed/update/signing controls belong to v3.9.0.
type BugPattern struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	Key         string `gorm:"size:160;not null;uniqueIndex" json:"key"`
	Name        string `gorm:"size:255;not null" json:"name"`
	Description string `gorm:"type:text" json:"description"`

	BugType           string `gorm:"size:96;not null;index" json:"bug_type"`
	SeverityHint      string `gorm:"size:32;not null;default:'info';index" json:"severity_hint"`
	ConfidenceDefault string `gorm:"size:64;not null;default:'low';index" json:"confidence_default"`

	TestLevel   int `gorm:"not null;default:0;index" json:"test_level"`
	SafetyLevel int `gorm:"not null;default:0;index" json:"safety_level"`

	Mode             string `gorm:"size:32;not null;default:'passive';index" json:"mode"`
	SafeByDefault    bool   `gorm:"not null;default:true;index" json:"safe_by_default"`
	RequiresApproval bool   `gorm:"not null;default:false;index" json:"requires_approval"`
	Enabled          bool   `gorm:"not null;default:true;index" json:"enabled"`

	Source  string `gorm:"size:64;not null;default:'core';index" json:"source"`
	Version string `gorm:"size:64;not null;default:'v1'" json:"version"`

	MatcherJSON        datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"matcher_json"`
	EvidenceSchemaJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"evidence_schema_json"`
	OWASPRefs          datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"owasp_refs"`
	Tags               datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"tags"`
}
