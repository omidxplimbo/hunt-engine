package operator

type PlanRequest struct {
	TargetID      uint
	UserID        uint
	OwnerKey      string
	UserMessage   string
	TargetContext map[string]interface{}
	MemoryContext map[string]interface{}
}

type ActionProposal struct {
	ActionType       string                 `json:"action_type"`
	Title            string                 `json:"title"`
	Description      string                 `json:"description"`
	RiskLevel        string                 `json:"risk_level"`
	SafetyLevel      int                    `json:"safety_level"`
	TestLevel        int                    `json:"test_level"`
	RequiresApproval bool                   `json:"requires_approval"`
	InputJSON        map[string]interface{} `json:"input_json"`
	Reason           string                 `json:"reason"`
}

type PentestHypothesis struct {
	Title                    string   `json:"title"`
	BugClass                 string   `json:"bug_class"`
	ImpactPotential          string   `json:"impact_potential"`
	Confidence               string   `json:"confidence"`
	Evidence                 []string `json:"evidence"`
	WhyThisMightBeReal       string   `json:"why_this_might_be_real"`
	MissingEvidence          []string `json:"missing_evidence"`
	RequiredContext          []string `json:"required_context"`
	SafeNextTest             string   `json:"safe_next_test"`
	ControlledValidationPath string   `json:"controlled_validation_path"`
	RiskLevel                string   `json:"risk_level"`
	TestLevel                int      `json:"test_level"`
	RequiresApproval         bool     `json:"requires_approval"`
}

type Plan struct {
	Mode                  string              `json:"mode"`
	ResponseSummary       string              `json:"response_summary"`
	ClarifyingQuestions   []string            `json:"clarifying_questions"`
	ReasoningSummary      string              `json:"reasoning_summary"`
	RecommendedNextSteps  []string            `json:"recommended_next_steps"`
	Actions               []ActionProposal    `json:"actions"`
	Hypotheses            []PentestHypothesis `json:"hypotheses"`
	MemoryNotes           []string            `json:"memory_notes"`
	Guardrails            []string            `json:"guardrails"`
	LLMAssisted           bool                `json:"llm_assisted"`
	LLMProvider           string              `json:"llm_provider"`
	LLMModel              string              `json:"llm_model"`
	FallbackReason        string              `json:"fallback_reason,omitempty"`
	OperatorPromptVersion string              `json:"operator_prompt_version"`
}
