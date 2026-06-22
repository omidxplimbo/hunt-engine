package handlers

import (
	"encoding/json"
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
	case models.AgentActionTypeReviewBugTestResults:
		return "review_bug_test_results_dispatcher"
	case models.AgentActionTypePromoteBugTestResults:
		return "promote_bug_test_results_dispatcher"
	case models.AgentActionTypeInspectBugPatterns:
		return "inspect_bug_patterns_dispatcher"
	case models.AgentActionTypeInspectBugPayloads:
		return "inspect_bug_payloads_dispatcher"
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
		blockedReasons = append(blockedReasons, "command execution requires a controlled runtime and is not enabled for this action")
	case "payload_generation":
		requiredControls = append(requiredControls, "allow_payload_generation")
		blockedReasons = append(blockedReasons, "payload generation execution requires a controlled runtime and policy approval")
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

	reason := "dispatcher preview recorded; this action class is not executable yet through the controlled runtime"
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
			"dispatcher preview does not execute commands, payloads, scans, report submission, or severity changes without controlled runtime approval",
			"unsupported action classes are converted into dispatcher previews; supported controlled-runtime actions may execute after policy and approval checks",
			"future real execution must be feature-flagged, policy-checked, approval-gated, rate-limited, and audited",
			"hard-blocked action classes require explicit future controls before any execution path can be enabled",
			"dispatcher preview is not proof of vulnerability validation or execution",
		},
	}
}

// DispatchTargetAgentAction records a dispatcher preview for an approved action.
// This intentionally does not execute real commands/payloads/scans without controlled runtime approval.
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
			preview["reason"] = "safe bug testing engine executed approved safe bug test workflow"

			runOutput := map[string]interface{}{}
			if len(run.OutputJSON) > 0 {
				_ = json.Unmarshal(run.OutputJSON, &runOutput)
			}
			activeTesting, _ := runOutput["active_testing"].(bool)
			safeActiveHeaders, _ := runOutput["safe_active_security_headers_v1"].(bool)

			preview["safe_bug_testing"] = map[string]interface{}{
				"attempted":                       true,
				"bug_test_run_id":                 run.ID,
				"bug_test_status":                 run.Status,
				"results_created":                 resultCount,
				"active_testing":                  activeTesting,
				"safe_active_security_headers_v1": safeActiveHeaders,
				"manual_validation":               true,
			}
			updateFields["output_json"] = agentActionJSON(preview)
			updateFields["error_message"] = ""
			updateFields["status"] = models.AgentActionStatusExecuted
			updateFields["executed_at"] = &now
		}
	}

	if !isHardBlockedDispatcherAction(row) && controlledRuntimeTypeForAgentAction(row.ActionType) != "" {
		run, runErr := CreateControlledTestRunFromAgentAction(&target, &row, uid)
		if runErr != nil {
			preview["controlled_runtime"] = map[string]interface{}{
				"attempted": true,
				"error":     runErr.Error(),
			}
			updateFields["output_json"] = agentActionJSON(preview)
			updateFields["error_message"] = runErr.Error()
		} else {
			preview["execution_enabled"] = true
			preview["executed"] = true
			preview["hard_blocked"] = false
			preview["reason"] = "approved action converted into queued controlled runtime run"
			preview["controlled_runtime"] = map[string]interface{}{
				"attempted":              true,
				"controlled_test_run_id": run.ID,
				"runtime_type":           run.RuntimeType,
				"runtime_status":         run.Status,
				"real_execution_pending": true,
				"evidence_capture":       true,
				"manual_review":          true,
			}
			updateFields["output_json"] = agentActionJSON(preview)
			updateFields["error_message"] = ""
			updateFields["status"] = models.AgentActionStatusExecuted
			updateFields["executed_at"] = &now
		}
	}

	if !isHardBlockedDispatcherAction(row) {
		var bridgeOutput map[string]interface{}

		switch row.ActionType {
		case models.AgentActionTypeReviewBugTestResults:
			bridgeOutput = dispatchReviewBugTestResults(target, row)
		case models.AgentActionTypeInspectBugPatterns:
			bridgeOutput = dispatchInspectBugPatterns(target, row)
		case models.AgentActionTypeInspectBugPayloads:
			bridgeOutput = dispatchInspectBugPayloads(target, row)
		case models.AgentActionTypePromoteBugTestResults:
			bridgeOutput = dispatchPromoteBugTestResults(target, row)
		}

		if bridgeOutput != nil {
			preview["execution_enabled"] = true
			preview["executed"] = true
			preview["hard_blocked"] = false
			preview["reason"] = "agent bug testing bridge executed a safe metadata/control workflow"
			preview["bridge_output"] = bridgeOutput
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

func dispatchReviewBugTestResults(target models.Target, action models.AgentAction) map[string]interface{} {
	var total int64
	var candidates int64
	var needsManual int64
	var validated int64
	var falsePositive int64
	var promoted int64

	_ = database.DB.Model(&models.BugTestResult{}).
		Where("target_id = ?", target.ID).
		Count(&total).Error
	_ = database.DB.Model(&models.BugTestResult{}).
		Where("target_id = ? AND status = ?", target.ID, models.BugTestResultStatusCandidate).
		Count(&candidates).Error
	_ = database.DB.Model(&models.BugTestResult{}).
		Where("target_id = ? AND status = ?", target.ID, models.BugTestResultStatusNeedsManualValidation).
		Count(&needsManual).Error
	_ = database.DB.Model(&models.BugTestResult{}).
		Where("target_id = ? AND status = ?", target.ID, models.BugTestResultStatusValidated).
		Count(&validated).Error
	_ = database.DB.Model(&models.BugTestResult{}).
		Where("target_id = ? AND status = ?", target.ID, models.BugTestResultStatusFalsePositive).
		Count(&falsePositive).Error
	_ = database.DB.Model(&models.BugTestResult{}).
		Where("target_id = ? AND finding_id IS NOT NULL", target.ID).
		Count(&promoted).Error

	var byType []struct {
		BugType string
		Count   int64
	}
	_ = database.DB.Model(&models.BugTestResult{}).
		Select("bug_type, COUNT(*) as count").
		Where("target_id = ?", target.ID).
		Group("bug_type").
		Order("count DESC").
		Scan(&byType).Error

	return map[string]interface{}{
		"review_version":          "bug-test-review-v1",
		"target_id":               target.ID,
		"action_id":               action.ID,
		"total_results":           total,
		"candidate_results":       candidates,
		"needs_manual_validation": needsManual,
		"validated_results":       validated,
		"false_positive_results":  falsePositive,
		"promoted_to_findings":    promoted,
		"by_bug_type":             byType,
		"recommendation":          "Review candidate and needs_manual_validation results, validate evidence manually, then promote only useful candidates to findings.",
		"execution_enabled":       true,
		"executed":                true,
		"active_testing":          false,
		"payload_execution":       false,
		"guardrails": []string{
			"review is metadata-only",
			"no payloads are executed",
			"no exploit validation is performed",
			"finding promotion remains controlled and audited",
		},
	}
}

func dispatchInspectBugPatterns(target models.Target, action models.AgentAction) map[string]interface{} {
	var total int64
	var enabled int64
	var passive int64
	var safeDefault int64
	var packTotal int64
	var packEnabled int64

	_ = database.DB.Model(&models.BugPattern{}).Count(&total).Error
	_ = database.DB.Model(&models.BugPattern{}).Where("enabled = ?", true).Count(&enabled).Error
	_ = database.DB.Model(&models.BugPattern{}).Where("mode = ?", "passive").Count(&passive).Error
	_ = database.DB.Model(&models.BugPattern{}).Where("safe_by_default = ?", true).Count(&safeDefault).Error
	_ = database.DB.Model(&models.BugPatternPack{}).Count(&packTotal).Error
	_ = database.DB.Model(&models.BugPatternPack{}).Where("enabled = ?", true).Count(&packEnabled).Error

	packs := make([]models.BugPatternPack, 0)
	_ = database.DB.Model(&models.BugPatternPack{}).
		Order("source ASC, key ASC").
		Limit(10).
		Find(&packs).Error

	packSummaries := make([]map[string]interface{}, 0, len(packs))
	for _, pack := range packs {
		packSummaries = append(packSummaries, map[string]interface{}{
			"id":                  pack.ID,
			"key":                 pack.Key,
			"name":                pack.Name,
			"source":              pack.Source,
			"version":             pack.Version,
			"trust_level":         pack.TrustLevel,
			"enabled":             pack.Enabled,
			"locked":              pack.Locked,
			"update_mode":         pack.UpdateMode,
			"pattern_count":       pack.PatternCount,
			"safety_score":        pack.SafetyScore,
			"quality_score":       pack.QualityScore,
			"noise_score":         pack.NoiseScore,
			"false_positive_rate": pack.FalsePositiveRate,
			"checksum_present":    pack.Checksum != "",
			"signature_present":   pack.Signature != "",
			"metadata":            pack.Metadata,
		})
	}

	return map[string]interface{}{
		"handler":                "inspect_bug_patterns",
		"inspection_version":     "pattern-pack-inspection-v1",
		"target_id":              target.ID,
		"action_id":              action.ID,
		"total_patterns":         total,
		"enabled_patterns":       enabled,
		"passive_patterns":       passive,
		"safe_by_default":        safeDefault,
		"total_pattern_packs":    packTotal,
		"enabled_pattern_packs":  packEnabled,
		"pattern_pack_summaries": packSummaries,
		"update_plan_foundation": map[string]interface{}{
			"external_update_enabled": false,
			"auto_import_enabled":     false,
			"rollback_ready":          true,
			"requires_approval":       true,
			"v3_9_foundation":         true,
		},
		"recommendation": "Review enabled trusted packs first. Keep external/community packs disabled until checksum, signature, safety score, and policy controls are reviewed.",
		"guardrails": []string{
			"Pattern inspection is metadata-only.",
			"Inspection does not import external feeds.",
			"Inspection does not execute payloads or exploit validation.",
			"Pattern use remains constrained by enabled pack controls, target policy, and approval controls.",
		},
	}
}

func dispatchInspectBugPayloads(target models.Target, action models.AgentAction) map[string]interface{} {
	var total int64
	var enabled int64
	var inert int64
	var safe int64
	var packTotal int64
	var packEnabled int64

	_ = database.DB.Model(&models.BugPayload{}).Count(&total).Error
	_ = database.DB.Model(&models.BugPayload{}).Where("enabled = ?", true).Count(&enabled).Error
	_ = database.DB.Model(&models.BugPayload{}).Where("safety_class = ?", "inert").Count(&inert).Error
	_ = database.DB.Model(&models.BugPayload{}).Where("safety_class = ?", "safe").Count(&safe).Error
	_ = database.DB.Model(&models.BugPayloadPack{}).Count(&packTotal).Error
	_ = database.DB.Model(&models.BugPayloadPack{}).Where("enabled = ?", true).Count(&packEnabled).Error

	packs := make([]models.BugPayloadPack, 0)
	_ = database.DB.Model(&models.BugPayloadPack{}).
		Order("source ASC, key ASC").
		Limit(10).
		Find(&packs).Error

	packSummaries := make([]map[string]interface{}, 0, len(packs))
	for _, pack := range packs {
		packSummaries = append(packSummaries, map[string]interface{}{
			"id":                  pack.ID,
			"key":                 pack.Key,
			"name":                pack.Name,
			"source":              pack.Source,
			"version":             pack.Version,
			"trust_level":         pack.TrustLevel,
			"enabled":             pack.Enabled,
			"locked":              pack.Locked,
			"update_mode":         pack.UpdateMode,
			"payload_count":       pack.PayloadCount,
			"safety_score":        pack.SafetyScore,
			"quality_score":       pack.QualityScore,
			"noise_score":         pack.NoiseScore,
			"false_positive_rate": pack.FalsePositiveRate,
			"checksum_present":    pack.Checksum != "",
			"signature_present":   pack.Signature != "",
			"metadata":            pack.Metadata,
		})
	}

	return map[string]interface{}{
		"handler":                "inspect_bug_payloads",
		"inspection_version":     "payload-pack-inspection-v1",
		"target_id":              target.ID,
		"action_id":              action.ID,
		"total_payloads":         total,
		"enabled_payloads":       enabled,
		"inert_payloads":         inert,
		"safe_payloads":          safe,
		"total_payload_packs":    packTotal,
		"enabled_payload_packs":  packEnabled,
		"payload_pack_summaries": packSummaries,
		"update_plan_foundation": map[string]interface{}{
			"external_update_enabled": false,
			"auto_import_enabled":     false,
			"rollback_ready":          true,
			"requires_approval":       true,
			"payload_execution":       false,
			"v3_9_foundation":         true,
		},
		"recommendation": "Treat payload packs as metadata-only until a safe/approved execution path exists. External packs must be reviewed for safety class, checksum, signature, and policy fit.",
		"guardrails": []string{
			"Payload registry inspection is metadata-only.",
			"Inspection does not import external payload feeds.",
			"Payload execution remains disabled.",
			"Payload use must require target policy, approval controls, and safety class checks.",
		},
	}
}

func dispatchPromoteBugTestResults(target models.Target, action models.AgentAction) map[string]interface{} {
	limit := 5
	var results []models.BugTestResult

	err := database.DB.
		Where("target_id = ? AND finding_id IS NULL AND status IN ?", target.ID, []string{
			models.BugTestResultStatusValidated,
			models.BugTestResultStatusNeedsManualValidation,
		}).
		Order("CASE WHEN status = 'validated' THEN 0 ELSE 1 END, id DESC").
		Limit(limit).
		Find(&results).Error

	if err != nil {
		return map[string]interface{}{
			"promotion_version": "bug-test-promotion-v1",
			"target_id":         target.ID,
			"action_id":         action.ID,
			"execution_enabled": false,
			"executed":          false,
			"error":             err.Error(),
		}
	}

	return map[string]interface{}{
		"promotion_version": "bug-test-promotion-v1",
		"target_id":         target.ID,
		"action_id":         action.ID,
		"execution_enabled": true,
		"executed":          true,
		"promoted_count":    0,
		"eligible_count":    len(results),
		"mode":              "review_only_foundation",
		"recommendation":    "Eligible results were identified, but bulk promotion execution remains review-only in this bridge step. Use individual Promote to Finding or a future explicit bulk endpoint.",
		"guardrails": []string{
			"bulk promotion is review-only in this bridge step",
			"false_positive and ignored results are excluded",
			"manual validation remains required",
			"automatic severity escalation is disabled",
		},
	}
}
