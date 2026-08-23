package models

import (
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ActionStartTwoAccountTest = "START_TWO_ACCOUNT_TEST"
	ActionAnalyzeIDOR          = "ANALYZE_IDOR"
	ActionSwitchIdentity       = "SWITCH_IDENTITY"
	AgentActionStatusProposed        = "proposed"
	AgentActionStatusApproved        = "approved"
	AgentActionStatusRejected        = "rejected"
	AgentActionStatusExecuted        = "executed"
	AgentActionStatusFailed          = "failed"
	AgentActionStatusBlockedByPolicy = "blocked_by_policy"

	AgentActionPolicyStatusAllowed = "allowed"
	AgentActionPolicyStatusWarning = "warning"
	AgentActionPolicyStatusBlocked = "blocked"
	AgentActionPolicyStatusUnknown = "unknown"

	AgentActionRiskLow      = "low"
	AgentActionRiskMedium   = "medium"
	AgentActionRiskHigh     = "high"
	AgentActionRiskCritical = "critical"

	AgentActionTypeRunCrawling           = "run_crawling"
	AgentActionTypeRunNucleiProfile      = "run_nuclei_profile"
	AgentActionTypeGenerateNucleiDraft   = "generate_nuclei_draft"
	AgentActionTypeRunJSIntelligence     = "run_js_intelligence"
	AgentActionTypeRunSafeBugTests       = "run_safe_bug_tests"
	AgentActionTypeReviewBugTestResults  = "review_bug_test_results"
	AgentActionTypePromoteBugTestResults = "promote_bug_test_results"
	AgentActionTypeInspectBugPatterns    = "inspect_bug_patterns"
	AgentActionTypeInspectBugPayloads    = "inspect_bug_payloads"
	AgentActionTypeDeepScanAsset         = "deep_scan_asset"
	AgentActionTypeReviewEndpoint        = "review_endpoint"
	AgentActionTypeRunCommandSchema      = "run_command_schema"
	AgentActionTypeGeneratePayload       = "generate_payload"
	AgentActionTypeExecutePayloadTest    = "execute_payload_test"
	AgentActionTypeRunOWASPChecklist     = "run_owasp_checklist"
	AgentActionTypeValidateFinding       = "validate_finding"
	AgentActionTypeProposeSeverityChange = "propose_severity_change"
	AgentActionTypeApplySeverityChange   = "apply_severity_change"
	AgentActionTypeGenerateReport        = "generate_report"
	AgentActionTypeSubmitReport          = "submit_report"
)

// AgentAction stores proposed/approved/rejected/executed workflow actions.
// Execution must remain policy-aware, scoped, approval-gated, audited, and safe-by-default.
type AgentAction struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint    `gorm:"not null;index" json:"target_id"`
	OwnerKey string  `gorm:"size:80;index" json:"owner_key"`
	Target   *Target `gorm:"foreignKey:TargetID" json:"-"`

	AgentRunID *uint     `gorm:"index" json:"agent_run_id"`
	AgentRun   *AgentRun `gorm:"foreignKey:AgentRunID" json:"-"`

	CreatedByUserID *uint `gorm:"index" json:"created_by_user_id"`
	CreatedByUser   *User `gorm:"foreignKey:CreatedByUserID" json:"-"`

	ApprovedByUserID *uint `gorm:"index" json:"approved_by_user_id"`
	ApprovedByUser   *User `gorm:"foreignKey:ApprovedByUserID" json:"-"`

	ActionType  string `gorm:"size:96;not null;index" json:"action_type"`
	Title       string `gorm:"size:255;not null" json:"title"`
	Description string `gorm:"type:text" json:"description"`

	Status       string `gorm:"size:32;not null;default:'proposed';index" json:"status"`
	PolicyStatus string `gorm:"size:64;not null;default:'unknown';index" json:"policy_status"`
	RiskLevel    string `gorm:"size:32;not null;default:'low';index" json:"risk_level"`

	// SafetyLevel/TestLevel follow the roadmap levels:
	// 0 passive, 1 safe active, 2 controlled active, 3 human-approved/exploit validation.
	SafetyLevel int `gorm:"not null;default:0;index" json:"safety_level"`
	TestLevel   int `gorm:"not null;default:0;index" json:"test_level"`

	AutonomyLevel    int  `gorm:"not null;default:0;index" json:"autonomy_level"`
	RequestedByAgent bool `gorm:"not null;default:true;index" json:"requested_by_agent"`
	RequiresApproval bool `gorm:"not null;default:true;index" json:"requires_approval"`

	InputJSON       datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"input_json"`
	OutputJSON      datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"output_json"`
	PolicyCheckJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"policy_check_json"`

	ErrorMessage string `gorm:"type:text" json:"error_message"`
	RejectReason string `gorm:"type:text" json:"reject_reason"`

	ApprovedAt  *time.Time `gorm:"index" json:"approved_at"`
	RejectedAt  *time.Time `gorm:"index" json:"rejected_at"`
	ExecutedAt  *time.Time `gorm:"index" json:"executed_at"`
	CompletedAt *time.Time `gorm:"index" json:"completed_at"`
}

const (
	AgentActionApprovalDecisionRejected = "rejected"
)

// AgentActionApproval stores human decisions for an agent action.
type AgentActionApproval struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	ActionID uint         `gorm:"not null;index" json:"action_id"`
	Action   *AgentAction `gorm:"foreignKey:ActionID" json:"-"`

	UserID uint  `gorm:"not null;index" json:"user_id"`
	User   *User `gorm:"foreignKey:UserID" json:"-"`

	Decision string `gorm:"size:32;not null;index" json:"decision"`
	Reason   string `gorm:"type:text" json:"reason"`

	SnapshotJSON datatypes.JSON `gorm:"type:jsonb;default:'{}'" json:"snapshot_json"`
}
