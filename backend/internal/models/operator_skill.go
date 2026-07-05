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

	OperatorSkillScopeBuiltin = "builtin"
	OperatorSkillScopeUser    = "user"
	OperatorSkillScopeProject = "project"
	OperatorSkillScopeTarget  = "target"
	OperatorSkillScopeGlobal  = "global"

	OperatorSkillOriginBuiltin = "builtin"
	OperatorSkillOriginUser    = "user"
	OperatorSkillOriginTeam    = "team"

	OperatorSkillTypeAdvisory         = "advisory"
	OperatorSkillTypePlanning         = "planning"
	OperatorSkillTypeAnalysis         = "analysis"
	OperatorSkillTypeActiveValidation = "active_validation"
	OperatorSkillTypeExploitRuntime   = "exploit_runtime"
	OperatorSkillTypeChain            = "chain"

	OperatorSkillRuntimeBackendNone             = "none"
	OperatorSkillRuntimeBackendInternalHTTP     = "internal_http_runtime"
	OperatorSkillRuntimeBackendBrowser          = "browser_runtime"
	OperatorSkillRuntimeBackendShellRunner      = "shell_runner"
	OperatorSkillRuntimeBackendToolRunner       = "tool_runner"
	OperatorSkillRuntimeBackendCustomScript     = "custom_script_runner"
	OperatorSkillRuntimeBackendPayloadGenerator = "payload_generator"
	OperatorSkillRuntimeBackendBruteForce       = "brute_force_runner"

	OperatorSkillPermissionScopeAwareAuthorized = "scope_aware_authorized"
	OperatorSkillPermissionManualApproval       = "manual_approval"
	OperatorSkillPermissionAssistedAutopilot    = "assisted_autopilot"
	OperatorSkillPermissionAuthorizedAutonomous = "authorized_autonomous"
	OperatorSkillPermissionLabDestructive       = "lab_destructive_allowed"

	OperatorLearningScopeTarget       = "target"
	OperatorLearningScopeProject      = "project"
	OperatorLearningScopeUserGlobal   = "user_global"
	OperatorLearningScopeOrganization = "organization_global"

	OperatorLearningSourceUserConfirmed     = "user_confirmed"
	OperatorLearningSourceSkillResult       = "skill_result"
	OperatorLearningSourceOperatorInference = "operator_inference"

	OperatorLearningStatusActive     = "active"
	OperatorLearningStatusDisabled   = "disabled"
	OperatorLearningStatusSuperseded = "superseded"

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

	// OwnerKey is empty for built-in global skills. User/team-custom skills use
	// owner-scoped keys so personal/team methodology can be reused safely.
	OwnerKey        string `gorm:"size:80;index" json:"owner_key"`
	CreatedByUserID *uint  `gorm:"index" json:"created_by_user_id"`

	// Scope/origin/type/runtime fields allow v3.15+ user-programmable skills to
	// coexist with built-in skills. They intentionally support real authorized
	// execution capabilities such as shell/tool workflows, HTTP fuzzing, payload
	// execution, bounded brute-force, state-changing validation, and exploit
	// chaining when scope, authorization, policy, budget, stop conditions, and
	// audit requirements allow them.
	Scope          string `gorm:"size:40;not null;default:'builtin';index" json:"scope"`
	Origin         string `gorm:"size:40;not null;default:'builtin';index" json:"origin"`
	SkillType      string `gorm:"size:80;not null;default:'planning';index" json:"skill_type"`
	RuntimeBackend string `gorm:"size:100;not null;default:'none';index" json:"runtime_backend"`
	ProjectKey     string `gorm:"size:120;index" json:"project_key"`
	TargetID       *uint  `gorm:"index" json:"target_id"`

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

	RequiredContext           datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"required_context"`
	TriggerSignals            datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"trigger_signals"`
	SupportedActions          datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"supported_actions"`
	SuccessCriteria           datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"success_criteria"`
	FailureCriteria           datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"failure_criteria"`
	MemoryPolicy              datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"memory_policy"`
	ExecutionProfile          datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"execution_profile"`
	AuthorizationRequirements datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"authorization_requirements"`
	BudgetDefaults            datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"budget_defaults"`
	StopConditions            datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"stop_conditions"`
	UserLearningPolicy        datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"user_learning_policy"`
	CustomDefinition          datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"custom_definition"`
	Metadata                  datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

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

	SkillScope     string `gorm:"size:40;index" json:"skill_scope"`
	SkillOrigin    string `gorm:"size:40;index" json:"skill_origin"`
	SkillType      string `gorm:"size:80;index" json:"skill_type"`
	RuntimeBackend string `gorm:"size:100;index" json:"runtime_backend"`
	PermissionMode string `gorm:"size:80;index" json:"permission_mode"`

	Objective            string         `gorm:"type:text" json:"objective"`
	SelectedBecause      string         `gorm:"type:text" json:"selected_because"`
	InputJSON            datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON           datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	ExecutionProfile     datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"execution_profile"`
	AuthorizationContext datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"authorization_context"`
	Budget               datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"budget"`
	StopConditions       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"stop_conditions"`
	ResultSummary        string         `gorm:"type:text" json:"result_summary"`
	NextStep             string         `gorm:"type:text" json:"next_step"`
	Metadata             datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

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

// OperatorLearningRecord stores user/project/target-scoped methodology learned
// from explicit user instruction, skill results, or operator-confirmed lessons.
// It is separate from target memory so reusable personal methodology can guide
// future skill selection without leaking raw target-specific evidence.
type OperatorLearningRecord struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	UserID   uint   `gorm:"not null;index" json:"user_id"`
	OwnerKey string `gorm:"size:80;index" json:"owner_key"`

	Scope      string `gorm:"size:40;not null;default:'user_global';index" json:"scope"`
	Source     string `gorm:"size:80;not null;default:'user_confirmed';index" json:"source"`
	Status     string `gorm:"size:40;not null;default:'active';index" json:"status"`
	ProjectKey string `gorm:"size:120;index" json:"project_key"`
	TargetID   *uint  `gorm:"index" json:"target_id"`

	Title   string `gorm:"size:255;not null" json:"title"`
	Summary string `gorm:"type:text" json:"summary"`
	Content string `gorm:"type:text" json:"content"`

	BugClass  string `gorm:"size:120;index" json:"bug_class"`
	SkillSlug string `gorm:"size:160;index" json:"skill_slug"`

	AppliesTo      datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"applies_to"`
	TriggerSignals datatypes.JSON `gorm:"type:jsonb;default:'[]'" json:"trigger_signals"`
	Methodology    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"methodology"`
	Constraints    datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"constraints"`
	ExecutionHints datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"execution_hints"`
	EvidenceJSON   datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"evidence_json"`
	Metadata       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"metadata"`

	Confidence int        `gorm:"not null;default:80;index" json:"confidence"`
	UseCount   int        `gorm:"not null;default:0" json:"use_count"`
	LastUsedAt *time.Time `gorm:"index" json:"last_used_at"`

	PromotedFromObservationID *uint `gorm:"index" json:"promoted_from_observation_id"`
}
