package handlers

import (
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/auditlog"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

type dispatchAgentActionRequest struct {
	DryRun *bool  `json:"dry_run"`
	Note   string `json:"note"`
}

func dispatcherHandlerName(actionType string) string {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case models.AgentActionTypeRunOWASPChecklist:
		return "owasp_checklist_dispatcher"
	case models.AgentActionTypeRunCrawling:
		return "crawling_dispatcher"
	case models.AgentActionTypeRunNucleiProfile:
		return "nuclei_profile_dispatcher"
	case models.AgentActionTypeGenerateNucleiDraft:
		return "nuclei_draft_dispatcher"
	case models.AgentActionTypeRunJSIntelligence:
		return "js_intelligence_dispatcher"
	case models.AgentActionTypeRunSafeBugTests:
		return "safe_bug_test_dispatcher"
	case models.AgentActionTypeDeepScanAsset:
		return "deep_scan_asset_dispatcher"
	case models.AgentActionTypeReviewEndpoint:
		return "endpoint_review_dispatcher"
	case models.AgentActionTypeRunCommandSchema:
		return "command_schema_dispatcher"
	case models.AgentActionTypeGeneratePayload:
		return "payload_generation_dispatcher"
	case models.AgentActionTypeExecutePayloadTest:
		return "payload_execution_dispatcher"
	case models.AgentActionTypeValidateFinding:
		return "finding_validation_dispatcher"
	case models.AgentActionTypeProposeSeverityChange:
		return "severity_review_dispatcher"
	case models.AgentActionTypeApplySeverityChange:
		return "severity_apply_dispatcher"
	case models.AgentActionTypeGenerateReport:
		return "report_generation_dispatcher"
	case models.AgentActionTypeSubmitReport:
		return "report_submission_dispatcher"
	default:
		return "unknown_dispatcher"
	}
}

func isHardBlockedDispatcherAction(action models.AgentAction) bool {
	switch strings.ToLower(strings.TrimSpace(action.ActionType)) {
	case models.AgentActionTypeRunCommandSchema,
		models.AgentActionTypeExecutePayloadTest,
		models.AgentActionTypeApplySeverityChange,
		models.AgentActionTypeSubmitReport:
		return true
	}

	if action.SafetyLevel >= 3 || action.TestLevel >= 3 {
		return true
	}

	if strings.EqualFold(action.RiskLevel, models.AgentActionRiskHigh) ||
		strings.EqualFold(action.RiskLevel, models.AgentActionRiskCritical) {
		return true
	}

	return false
}

func buildDispatchPreview(target models.Target, action models.AgentAction, dryRun bool, note string) map[string]interface{} {
	hardBlocked := isHardBlockedDispatcherAction(action)
	class := actionClass(action.ActionType)

	blockedReasons := make([]string, 0)
	requiredControls := make([]string, 0)

	switch class {
	case "command_execution":
		requiredControls = append(requiredControls, "allow_command_execution")
		blockedReasons = append(blockedReasons, "command execution is disabled in v3.7.0 foundation")
	case "payload_generation":
		requiredControls = append(requiredControls, "allow_payload_generation")
		blockedReasons = append(blockedReasons, "payload generation execution path is disabled in v3.7.0 foundation")
	case "payload_execution":
		requiredControls = append(requiredControls, "allow_payload_execution", "can_run_exploit_validation")
		blockedReasons = append(blockedReasons, "payload execution is blocked until controlled exploit validation is implemented")
	case "severity_apply":
		requiredControls = append(requiredControls, "allow_severity_auto_update")
		blockedReasons = append(blockedReasons, "automatic severity application is disabled")
	case "report_submission":
		requiredControls = append(requiredControls, "allow_report_auto_submit")
		blockedReasons = append(blockedReasons, "automatic report submission is disabled")
	}

	if hardBlocked && len(blockedReasons) == 0 {
		blockedReasons = append(blockedReasons, "action class is hard-blocked from execution by default")
	}

	reason := "dispatcher foundation is active, but real action execution is disabled in this v3.7.0 step"
	if len(blockedReasons) > 0 {
		reason = blockedReasons[0]
	}

	return map[string]interface{}{
		"dispatcher_version": "agent-action-dispatcher-v2",
		"target_id":          target.ID,
		"root_domain":        target.RootDomain,
		"action_id":          action.ID,
		"action_type":        action.ActionType,
		"action_class":       class,
		"handler":            dispatcherHandlerName(action.ActionType),
		"dry_run":            dryRun,
		"note":               strings.TrimSpace(note),
		"status_before":      action.Status,
		"policy_status":      action.PolicyStatus,
		"risk_level":         action.RiskLevel,
		"safety_level":       action.SafetyLevel,
		"test_level":         action.TestLevel,
		"autonomy_level":     action.AutonomyLevel,
		"execution_enabled":  false,
		"executed":           false,
		"hard_blocked":       hardBlocked,
		"blocked_reasons":    blockedReasons,
		"required_controls":  requiredControls,
		"reason":             reason,
		"autonomy_controls": map[string]interface{}{
			"allow_command_execution":    false,
			"allow_payload_generation":   false,
			"allow_payload_execution":    false,
			"allow_severity_auto_update": false,
			"allow_report_auto_submit":   false,
			"can_run_exploit_validation": false,
		},
		"guardrails": []string{
			"v3.7.0 dispatcher does not execute commands, payloads, scans, report submission, or severity changes",
			"approved actions are converted into a dispatcher preview only",
			"future real execution must be feature-flagged, policy-checked, approval-gated, rate-limited, and audited",
			"hard-blocked action classes require explicit future controls before any execution path can be enabled",
			"dispatcher preview is not proof of vulnerability validation or execution",
		},
	}
}

// DispatchTargetAgentAction records a dispatcher preview for an approved action.
// This intentionally does not execute real commands/payloads/scans in v3.7.0.
func DispatchTargetAgentAction(c *fiber.Ctx) error {
	if err := ensureAgentActionsEnabled(c); err != nil {
		return err
	}

	target, row, err := loadAccessibleAgentAction(c)
	if err != nil {
		if fe, ok := err.(*fiber.Error); ok {
			return c.Status(fe.Code).JSON(fiber.Map{"status": "error", "message": fe.Message})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	if row.Status == models.AgentActionStatusBlockedByPolicy || row.PolicyStatus == models.AgentActionPolicyStatusBlocked {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "blocked_by_policy actions cannot be dispatched"})
	}

	if row.Status != models.AgentActionStatusApproved {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"status": "error", "message": "only approved actions can be dispatched"})
	}

	req := new(dispatchAgentActionRequest)
	_ = c.BodyParser(req)

	dryRun := true
	if req.DryRun != nil {
		dryRun = *req.DryRun
	}

	uid, err := currentUserID(c)
	if err != nil {
		return err
	}

	preview := buildDispatchPreview(target, row, dryRun, req.Note)
	now := time.Now().UTC()

	updateFields := map[string]interface{}{
		"output_json":   agentActionJSON(preview),
		"error_message": "execution dispatcher preview recorded; real execution is disabled for this action class",
		"completed_at":  &now,
	}

	if row.ActionType == models.AgentActionTypeRunSafeBugTests && !isHardBlockedDispatcherAction(row) {
		run, resultCount, runErr := CreateBugTestRunFromAgentAction(&target, &row, uid)
		if runErr != nil {
			preview["safe_bug_testing"] = map[string]interface{}{
				"attempted": true,
				"error":     runErr.Error(),
			}
			updateFields["output_json"] = agentActionJSON(preview)
			updateFields["error_message"] = runErr.Error()
		} else {
			preview["execution_enabled"] = true
			preview["executed"] = true
			preview["hard_blocked"] = false
			preview["reason"] = "safe bug testing engine foundation executed passive/stub runner"
			preview["safe_bug_testing"] = map[string]interface{}{
				"attempted":         true,
				"bug_test_run_id":   run.ID,
				"bug_test_status":   run.Status,
				"results_created":   resultCount,
				"active_testing":    false,
				"manual_validation": true,
			}
			updateFields["output_json"] = agentActionJSON(preview)
			updateFields["error_message"] = ""
			updateFields["status"] = models.AgentActionStatusExecuted
			updateFields["executed_at"] = &now
		}
	}

	if err := database.DB.Model(&models.AgentAction{}).
		Where("id = ?", row.ID).
		Updates(updateFields).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error()})
	}

	_ = database.DB.First(&row, row.ID)

	entityID := row.ID
	_ = auditlog.Record(auditlog.Entry{
		ActorUserID: &uid,
		Action:      "target.agent_action.dispatch",
		EntityType:  "agent_action",
		EntityID:    &entityID,
		TargetID:    &target.ID,
		IPAddress:   auditlog.ClientIP(c),
		UserAgent:   auditlog.UserAgent(c),
		Metadata: map[string]interface{}{
			"target_id":         target.ID,
			"root_domain":       target.RootDomain,
			"action_id":         row.ID,
			"action_type":       row.ActionType,
			"handler":           dispatcherHandlerName(row.ActionType),
			"execution_enabled": preview["execution_enabled"],
			"executed":          preview["executed"],
			"dry_run":           dryRun,
			"hard_blocked":      preview["hard_blocked"],
		},
	})

	return c.JSON(fiber.Map{
		"status": "success",
		"data": fiber.Map{
			"action":   row,
			"dispatch": preview,
		},
	})
}
