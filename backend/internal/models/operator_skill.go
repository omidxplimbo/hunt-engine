package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	OperatorSkillCategoryRecon             = "recon"
	OperatorSkillCategoryParameterIntel    = "parameter_intelligence"
	OperatorSkillCategoryHTTPAnalysis      = "http_evidence_analysis"
	OperatorSkillCategoryClientSide        = "client_side"
	OperatorSkillCategoryInjection         = "injection"
	OperatorSkillCategoryAccessControl     = "access_control"
	OperatorSkillCategoryNetworkFileCloud  = "network_file_cloud"
	OperatorSkillCategoryBusinessLogic     = "business_logic"
	OperatorSkillCategoryFindingPromotion  = "finding_promotion"
	OperatorSkillCategoryExploitValidation = "exploit_validation"

	OperatorSkillRunStatusPlanned         = "planned"
	OperatorSkillRunStatusRunning         = "running"
	OperatorSkillRunStatusCompleted       = "completed"
	OperatorSkillRunStatusFailed          = "failed"
	OperatorSkillRunStatusBlockedByPolicy = "blocked_by_policy"
	OperatorSkillRunStatusNeedsContext    = "needs_context"
	OperatorSkillRunStatusNeedsApproval   = "needs_approval"
	OperatorSkillRunStatusCancelled       = "cancelled"

	OperatorSkillObservationTypeCandidate   = "candidate"
	OperatorSkillObservationTypeEvidence    = "evidence"
	OperatorSkillObservationTypeLearning    = "learning"
	OperatorSkillObservationTypeFailure     = "failure"
	OperatorSkillObservationTypeNextStep    = "next_step"
	OperatorSkillObservationTypeFindingHint = "finding_hint"
)

// OperatorSkill defines a reusable pentest skill. A skill is not only an advisory
// prompt; it is a repeatable authorized validation/exploitation workflow template
// that can select evidence, request/execute actions, analyze results, and write
// learning back to memory when scope and authorization allow it.
type OperatorSkill struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// OwnerKey is empty for built-in global skills. User/team-custom skills can
	// later use a scoped owner key.
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`

	Name        string `gorm:"size:160;not null" json:"name"`
	Slug        string `gorm:"size:160;not null;uniqueIndex" json:"slug"`
	Version     string `gorm:"size:40;not null;default:'v1'" json:"version"`
	Category    string `gorm:"size:96;not null;index" json:"category"`
	BugClass    string `gorm:"size:120;index" json:"bug_class"`
	Description string `gorm:"type:text" json:"description"`

	DefaultRiskLevel     string `gorm:"size:40;not null;default:'low'" json:"default_risk_level"`
	DefaultSafetyLevel   int    `gorm:"not null;default:1;index" json:"default_safety_level"`
	DefaultTestLevel     int    `gorm:"not null;default:1;index" json:"default_test_level"`
	DefaultAutonomyLevel int    `gorm:"not null;default:1;index" json:"default_autonomy_level"`

	// PermissionMode is descriptive for now. Execution is still decided by target
	// scope, policy, runtime, budget, and explicit authorization when required.
	PermissionMode string `gorm:"size:80;not null;default:'scope_aware_authorized'" json:"permission_mode"`

	RequiredContext  datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"required_context"`
	TriggerSignals   datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"trigger_signals"`
	SupportedActions datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"supported_actions"`
	SuccessCriteria  datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"success_criteria"`
	FailureCriteria  datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"failure_criteria"`
	MemoryPolicy     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"memory_policy"`
	Metadata         datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	IsBuiltIn bool `gorm:"not null;default:false;index" json:"is_builtin"`
	IsEnabled bool `gorm:"not null;default:true;index" json:"is_enabled"`
}

// OperatorSkillRun records one invocation of a skill for a target/session.
type OperatorSkillRun struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`
	TargetID uint   `gorm:"not null;index" json:"target_id"`

	SkillID   *uint  `gorm:"index" json:"skill_id"`
	SkillSlug string `gorm:"size:160;not null;index" json:"skill_slug"`
	SkillName string `gorm:"size:160" json:"skill_name"`

	ChatSessionID *uint `gorm:"index" json:"chat_session_id"`
	ChatMessageID *uint `gorm:"index" json:"chat_message_id"`
	AgentActionID *uint `gorm:"index" json:"agent_action_id"`

	Status        string `gorm:"size:40;not null;default:'planned';index" json:"status"`
	RiskLevel     string `gorm:"size:40;not null;default:'low';index" json:"risk_level"`
	SafetyLevel   int    `gorm:"not null;default:1;index" json:"safety_level"`
	TestLevel     int    `gorm:"not null;default:1;index" json:"test_level"`
	AutonomyLevel int    `gorm:"not null;default:1;index" json:"autonomy_level"`

	Objective       string         `gorm:"type:text" json:"objective"`
	SelectedBecause string         `gorm:"type:text" json:"selected_because"`
	InputJSON       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	ResultSummary   string         `gorm:"type:text" json:"result_summary"`
	NextStep        string         `gorm:"type:text" json:"next_step"`
	Metadata        datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	StartedAt   *time.Time `gorm:"index" json:"started_at"`
	CompletedAt *time.Time `gorm:"index" json:"completed_at"`

	Skill *OperatorSkill `gorm:"foreignKey:SkillID" json:"-"`
}

// OperatorSkillObservation stores evidence, candidates, failures, next steps,
// and learning emitted by a skill run.
type OperatorSkillObservation struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`
	TargetID uint   `gorm:"not null;index" json:"target_id"`

	SkillRunID uint   `gorm:"not null;index" json:"skill_run_id"`
	SkillSlug  string `gorm:"size:160;not null;index" json:"skill_slug"`

	ObservationType string `gorm:"size:80;not null;index" json:"observation_type"`
	Title           string `gorm:"size:255;not null" json:"title"`
	Summary         string `gorm:"type:text" json:"summary"`
	Content         string `gorm:"type:text" json:"content"`

	URL       string `gorm:"type:text" json:"url"`
	ParamName string `gorm:"size:180;index" json:"param_name"`
	BugClass  string `gorm:"size:120;index" json:"bug_class"`

	Confidence int    `gorm:"not null;default:50;index" json:"confidence"`
	Severity   string `gorm:"size:40;index" json:"severity"`
	Status     string `gorm:"size:40;index" json:"status"`

	EvidenceJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"evidence_json"`
	Metadata     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	SkillRun *OperatorSkillRun `gorm:"foreignKey:SkillRunID" json:"-"`
}
