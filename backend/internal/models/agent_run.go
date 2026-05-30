package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	AgentRunTypeTriage       = "triage"
	AgentRunTypeSummary      = "summary"
	AgentRunTypeReport       = "report"
	AgentRunTypePolicyReview = "policy_review"

	AgentRunStatusPending   = "pending"
	AgentRunStatusRunning   = "running"
	AgentRunStatusCompleted = "completed"
	AgentRunStatusFailed    = "failed"
	AgentRunStatusSkipped   = "skipped"

	AgentRunSourceLocal = "local"
	AgentRunSourceLLM   = "llm"
	AgentRunSourceUser  = "user"

	AgentRunPolicyStatusAllowed = "allowed"
	AgentRunPolicyStatusWarning = "warning"
	AgentRunPolicyStatusBlocked = "blocked"
	AgentRunPolicyStatusUnknown = "unknown"
)

// AgentRun stores durable execution records for AI/manual analysis agents.
// Deterministic product guardrails remain authoritative; agent output is advisory.
type AgentRun struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	AgentType    string `gorm:"size:64;not null;index" json:"agent_type"`
	Provider     string `gorm:"size:64;index" json:"provider"`
	Model        string `gorm:"size:128" json:"model"`
	Status       string `gorm:"size:32;not null;default:'pending';index" json:"status"`
	Source       string `gorm:"size:64;not null;default:'local';index" json:"source"`
	PolicyStatus string `gorm:"size:64;not null;default:'unknown';index" json:"policy_status"`

	InputDigest  string         `gorm:"size:128;index" json:"input_digest"`
	InputJSON    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	ErrorMessage string         `gorm:"type:text" json:"error_message"`

	StartedAt   *time.Time `gorm:"index" json:"started_at"`
	CompletedAt *time.Time `gorm:"index" json:"completed_at"`
}
