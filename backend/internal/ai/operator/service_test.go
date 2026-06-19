package operator

import (
	"strings"
	"testing"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

func TestBuildContextFactsIncludesTargetAndMemory(t *testing.T) {
	facts := buildContextFacts(
		map[string]interface{}{
			"target_name":          "test",
			"root_domain":          "test.com",
			"asset_count":          2135,
			"live_asset_count":     25,
			"url_count":            34,
			"finding_count":        28,
			"active_finding_count": 28,
			"platform_name":        "HackerOne",
			"max_test_intensity":   "safe",
			"auth_required":        false,
		},
		map[string]interface{}{
			"memory_count": 8,
			"memories": []interface{}{
				map[string]interface{}{
					"title":   "Bug test result summary",
					"summary": "Target has 247 bug test results available for iterative planning.",
				},
			},
		},
	)

	joined := strings.Join(facts, " | ")

	required := []string{
		"target_name=test",
		"root_domain=test.com",
		"asset_count=2135",
		"live_asset_count=25",
		"url_count=34",
		"finding_count=28",
		"memory_count=8",
		"Bug test result summary",
	}

	for _, item := range required {
		if !strings.Contains(joined, item) {
			t.Fatalf("expected facts to contain %q, got: %s", item, joined)
		}
	}
}

func TestGroundPlanRewritesGenericSummaryAndAddsEvidenceBasis(t *testing.T) {
	plan := &Plan{
		ResponseSummary: "I will create a prioritized OWASP test plan and propose safe actions.",
		Actions: []ActionProposal{
			{
				ActionType:  models.AgentActionTypeReviewBugTestResults,
				Title:       "Review bug test results",
				InputJSON:   map[string]interface{}{},
				RiskLevel:   models.AgentActionRiskLow,
				SafetyLevel: 0,
				TestLevel:   0,
			},
		},
	}

	facts := []string{
		"target_name=test",
		"asset_count=2135",
		"live_asset_count=25",
		"url_count=34",
		"finding_count=28",
		"memory_count=8",
	}

	groundPlan(plan, facts)

	if !strings.Contains(plan.ResponseSummary, "asset_count=2135") {
		t.Fatalf("expected grounded summary to include asset_count fact, got: %s", plan.ResponseSummary)
	}

	if len(plan.RecommendedNextSteps) == 0 {
		t.Fatalf("expected recommended next steps to be populated")
	}

	if len(plan.Guardrails) == 0 {
		t.Fatalf("expected default guardrails to be populated")
	}

	basis, ok := plan.Actions[0].InputJSON["evidence_basis"].([]string)
	if !ok || len(basis) == 0 {
		t.Fatalf("expected evidence_basis to be added to action input_json, got: %#v", plan.Actions[0].InputJSON["evidence_basis"])
	}
}

func TestSanitizePlanKeepsOnlyAllowedApprovalGatedActions(t *testing.T) {
	raw := map[string]interface{}{
		"mode":             "llm_operator_plan",
		"response_summary": "Review current target evidence.",
		"actions": []interface{}{
			map[string]interface{}{
				"action_type":       models.AgentActionTypeReviewBugTestResults,
				"title":             "Review bug test results",
				"description":       "Review stored candidate results.",
				"requires_approval": false,
				"input_json": map[string]interface{}{
					"objective": "triage",
				},
			},
			map[string]interface{}{
				"action_type": "execute_payload_test",
				"title":       "Execute payload",
			},
			map[string]interface{}{
				"action_type": "unknown_action",
				"title":       "Unknown action",
			},
		},
	}

	plan := sanitizePlan(raw)

	if len(plan.Actions) != 1 {
		t.Fatalf("expected exactly one allowed action, got %d: %#v", len(plan.Actions), plan.Actions)
	}

	action := plan.Actions[0]
	if action.ActionType != models.AgentActionTypeReviewBugTestResults {
		t.Fatalf("unexpected action type: %s", action.ActionType)
	}

	if !action.RequiresApproval {
		t.Fatalf("expected action to be approval-gated regardless of raw LLM value")
	}

	if action.RiskLevel != models.AgentActionRiskLow {
		t.Fatalf("expected missing risk_level to default low, got %q", action.RiskLevel)
	}

	if action.InputJSON["source"] != "llm_operator" {
		t.Fatalf("expected source=llm_operator, got %#v", action.InputJSON["source"])
	}

	if action.InputJSON["operator_prompt_version"] != PromptVersion {
		t.Fatalf("expected operator prompt version in input_json")
	}
}

func TestSystemPromptAllowsControlledApprovedValidationButNotChatExecution(t *testing.T) {
	prompt := SystemPrompt()

	required := []string{
		"Do not execute tests or exploitation inside the chat response itself.",
		"controlled validation or exploitation may be proposed as approval-gated Agent Actions",
		"High-risk actions require explicit approval",
		"Do not claim a vulnerability is confirmed without evidence",
	}

	for _, item := range required {
		if !strings.Contains(prompt, item) {
			t.Fatalf("expected prompt to contain %q", item)
		}
	}

	forbidden := []string{
		"Do not perform real exploitation directly in chat.",
	}

	for _, item := range forbidden {
		if strings.Contains(prompt, item) {
			t.Fatalf("prompt still contains outdated restrictive wording: %q", item)
		}
	}
}
