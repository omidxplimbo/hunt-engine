package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ControlledTestRuntimeHTTPProbe            = "http_probe"
	ControlledTestRuntimeOWASPValidation      = "owasp_validation"
	ControlledTestRuntimeAuthRateLimit        = "auth_rate_limit"
	ControlledTestRuntimeControlledBruteForce = "controlled_bruteforce"
	ControlledTestRuntimeExploitValidation    = "exploit_validation"

	ControlledTestRunStatusQueued          = "queued"
	ControlledTestRunStatusRunning         = "running"
	ControlledTestRunStatusCompleted       = "completed"
	ControlledTestRunStatusFailed          = "failed"
	ControlledTestRunStatusBlockedByPolicy = "blocked_by_policy"
	ControlledTestRunStatusCancelled       = "cancelled"

	ControlledTestResultStatusPassed            = "passed"
	ControlledTestResultStatusFailed            = "failed"
	ControlledTestResultStatusVulnerable        = "vulnerable"
	ControlledTestResultStatusInconclusive      = "inconclusive"
	ControlledTestResultStatusBlocked           = "blocked"
	ControlledTestResultStatusNeedsManualReview = "needs_manual_review"

	ControlledTestEventCreated       = "controlled_test.created"
	ControlledTestEventPolicyChecked = "controlled_test.policy_checked"
	ControlledTestEventStarted       = "controlled_test.started"
	ControlledTestEventResultCreated = "controlled_test.result_created"
	ControlledTestEventCompleted     = "controlled_test.completed"
	ControlledTestEventFailed        = "controlled_test.failed"
	ControlledTestEventBlocked       = "controlled_test.blocked"
	ControlledTestEventCancelled     = "controlled_test.cancelled"
)

type ControlledTestRun struct {
	gorm.Model

	UserID        uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey      string `gorm:"size:80;index" json:"owner_key"`
	TargetID      uint   `gorm:"not null;index" json:"target_id"`
	AgentActionID *uint  `gorm:"index" json:"agent_action_id"`

	RuntimeType string `gorm:"size:80;not null;index" json:"runtime_type"`
	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`

	Status       string `gorm:"size:40;not null;default:'queued';index" json:"status"`
	PolicyStatus string `gorm:"size:40;not null;default:'warning';index" json:"policy_status"`

	RiskLevel     string `gorm:"size:40;not null;default:'medium';index" json:"risk_level"`
	SafetyLevel   int    `gorm:"not null;default:0" json:"safety_level"`
	TestLevel     int    `gorm:"not null;default:0" json:"test_level"`
	AutonomyLevel int    `gorm:"not null;default:0" json:"autonomy_level"`

	AttemptCount int `gorm:"not null;default:0" json:"attempt_count"`
	ResultCount  int `gorm:"not null;default:0" json:"result_count"`

	StartedAt   *time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`

	InputJSON       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	PolicyCheckJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"policy_check_json"`
	ErrorMessage    string         `gorm:"type:text" json:"error_message"`

	Target      *Target      `gorm:"foreignKey:TargetID" json:"-"`
	AgentAction *AgentAction `gorm:"foreignKey:AgentActionID" json:"-"`
}

type ControlledTestResult struct {
	gorm.Model

	UserID        uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey      string `gorm:"size:80;index" json:"owner_key"`
	TargetID      uint   `gorm:"not null;index" json:"target_id"`
	RunID         uint   `gorm:"not null;index" json:"run_id"`
	AgentActionID *uint  `gorm:"index" json:"agent_action_id"`
	FindingID     *uint  `gorm:"index" json:"finding_id"`

	RuntimeType string `gorm:"size:80;not null;index" json:"runtime_type"`
	BugType     string `gorm:"size:120;index" json:"bug_type"`
	CheckKey    string `gorm:"size:180;index" json:"check_key"`

	URL    string `gorm:"type:text" json:"url"`
	Method string `gorm:"size:16" json:"method"`

	Status       string `gorm:"size:40;not null;default:'inconclusive';index" json:"status"`
	Confidence   string `gorm:"size:40;not null;default:'low';index" json:"confidence"`
	SeverityHint string `gorm:"size:40;not null;default:'info';index" json:"severity_hint"`

	RequestJSON  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"request_json"`
	ResponseJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"response_json"`
	EvidenceJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"evidence_json"`
	MatcherJSON  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"matcher_json"`
	Metadata     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	Run     *ControlledTestRun `gorm:"foreignKey:RunID" json:"-"`
	Target  *Target            `gorm:"foreignKey:TargetID" json:"-"`
	Finding *Finding           `gorm:"foreignKey:FindingID" json:"-"`
}

type ControlledTestEvent struct {
	gorm.Model

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`
	TargetID uint   `gorm:"not null;index" json:"target_id"`

	RunID    *uint `gorm:"index" json:"run_id"`
	ResultID *uint `gorm:"index" json:"result_id"`

	EventType string `gorm:"size:80;not null;index" json:"event_type"`

	BeforeJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"before_json"`
	AfterJSON  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"after_json"`
	Metadata   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	Target *Target               `gorm:"foreignKey:TargetID" json:"-"`
	Run    *ControlledTestRun    `gorm:"foreignKey:RunID" json:"-"`
	Result *ControlledTestResult `gorm:"foreignKey:ResultID" json:"-"`
}
