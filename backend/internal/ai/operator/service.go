package operator

import (
	"context"
	"fmt"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/llmclient"
)

func GeneratePlan(ctx context.Context, req PlanRequest) (*Plan, error) {
	cfg, err := llmclient.ResolveDefault(req.OwnerKey)
	if err != nil {
		return nil, err
	}
	return GeneratePlanWithConfig(ctx, cfg, req)
}

func GeneratePlanWithConfig(ctx context.Context, cfg *llmclient.Config, req PlanRequest) (*Plan, error) {
	if cfg == nil {
		return nil, fmt.Errorf("LLM config is required")
	}
	req.UserMessage = strings.TrimSpace(req.UserMessage)
	if req.UserMessage == "" {
		return nil, fmt.Errorf("user message is required")
	}

	payload := map[string]interface{}{
		"task":                    "Create a safe, approval-gated pentest operator plan for this target chat request.",
		"operator_prompt_version": PromptVersion,
		"user_message":            req.UserMessage,
		"target": map[string]interface{}{
			"target_id": req.TargetID,
			"user_id":   req.UserID,
			"owner_key": req.OwnerKey,
		},
		"target_context": req.TargetContext,
		"memory_context": req.MemoryContext,
		"allowed_action_types": []string{
			models.AgentActionTypeRunCrawling,
			models.AgentActionTypeRunNucleiProfile,
			models.AgentActionTypeGenerateNucleiDraft,
			models.AgentActionTypeRunJSIntelligence,
			models.AgentActionTypeRunSafeBugTests,
			models.AgentActionTypeReviewBugTestResults,
			models.AgentActionTypePromoteBugTestResults,
			models.AgentActionTypeInspectBugPatterns,
			models.AgentActionTypeInspectBugPayloads,
			models.AgentActionTypeDeepScanAsset,
			models.AgentActionTypeReviewEndpoint,
			models.AgentActionTypeGeneratePayload,
			models.AgentActionTypeRunOWASPChecklist,
			models.AgentActionTypeValidateFinding,
			models.AgentActionTypeProposeSeverityChange,
			models.AgentActionTypeGenerateReport,
		},
	}

	raw, err := llmclient.GenerateJSON(ctx, cfg, llmclient.JSONRequest{
		SystemPrompt:    SystemPrompt(),
		UserPayload:     payload,
		Temperature:     0.2,
		MaxTokens:       1400,
		ResponseVersion: PromptVersion,
	})
	if err != nil {
		return nil, err
	}

	plan := sanitizePlan(raw)
	plan.LLMAssisted = true
	plan.LLMProvider = cfg.Provider
	plan.LLMModel = cfg.Model
	plan.OperatorPromptVersion = PromptVersion

	if plan.ResponseSummary == "" {
		plan.ResponseSummary = "I prepared an approval-gated operator plan from the available target context and memory."
	}
	if len(plan.Guardrails) == 0 {
		plan.Guardrails = defaultGuardrails()
	}

	return plan, nil
}

func sanitizePlan(raw map[string]interface{}) *Plan {
	plan := &Plan{
		Mode:       cleanString(raw["mode"]),
		Guardrails: cleanStringSlice(raw["guardrails"]),
	}
	if plan.Mode == "" {
		plan.Mode = "llm_operator_plan"
	}

	plan.ResponseSummary = cleanString(raw["response_summary"])
	plan.ClarifyingQuestions = cleanStringSlice(raw["clarifying_questions"])
	plan.ReasoningSummary = cleanString(raw["reasoning_summary"])
	plan.RecommendedNextSteps = cleanStringSlice(raw["recommended_next_steps"])
	plan.MemoryNotes = cleanStringSlice(raw["memory_notes"])

	if items, ok := raw["actions"].([]interface{}); ok {
		for _, item := range items {
			obj, ok := item.(map[string]interface{})
			if !ok {
				continue
			}
			action := ActionProposal{
				ActionType:       cleanString(obj["action_type"]),
				Title:            cleanString(obj["title"]),
				Description:      cleanString(obj["description"]),
				RiskLevel:        cleanString(obj["risk_level"]),
				SafetyLevel:      cleanInt(obj["safety_level"]),
				TestLevel:        cleanInt(obj["test_level"]),
				RequiresApproval: true,
				InputJSON:        cleanMap(obj["input_json"]),
				Reason:           cleanString(obj["reason"]),
			}
			if action.ActionType == "" || action.Title == "" {
				continue
			}
			if action.RiskLevel == "" {
				action.RiskLevel = models.AgentActionRiskLow
			}
			if action.InputJSON == nil {
				action.InputJSON = map[string]interface{}{}
			}
			action.InputJSON["source"] = "llm_operator"
			action.InputJSON["operator_prompt_version"] = PromptVersion
			plan.Actions = append(plan.Actions, action)
		}
	}

	return plan
}

func defaultGuardrails() []string {
	return []string{
		"target-scoped and owner-scoped context only",
		"planning and proposal only; no direct execution in chat",
		"active/risky actions require policy checks and approval",
		"destructive, out-of-scope, brute-force, and data-exfiltration actions remain blocked",
	}
}

func cleanString(value interface{}) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return ""
	}
	return text
}

func cleanStringSlice(value interface{}) []string {
	items, ok := value.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := cleanString(item); text != "" {
			out = append(out, text)
		}
	}
	return out
}

func cleanMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	if obj, ok := value.(map[string]interface{}); ok {
		return obj
	}
	return map[string]interface{}{}
}

func cleanInt(value interface{}) int {
	switch v := value.(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	default:
		return 0
	}
}
