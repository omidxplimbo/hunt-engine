package handlers

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"gorm.io/gorm"
)

const (
	parameterInventorySkillSlug       = "parameter_inventory"
	httpEvidenceAnalysisSkillSlug     = "http_evidence_analysis"
	authContextNeededSkillSlug        = "auth_context_needed"
	jsAuditSkillSlug                  = "js_audit"
	xssReflectionSkillSlug            = "xss_reflection"
	openRedirectSkillSlug             = "open_redirect"
	pathTraversalBaselineSkillSlug    = "path_traversal_baseline"
	xssReflectionContextSkillSlug     = "xss_reflection_context"
	openRedirectChainSkillSlug        = "open_redirect_chain"
	pathTraversalFileReadBaselineSlug = "path_traversal_file_read_baseline"
)

type parameterInventoryCandidate struct {
	Name       string
	URLs       []string
	Sources    []string
	BugClasses []string
	Reasons    []string
	Score      int
}

func uniqueStrings(values []string, max int) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
		if max > 0 && len(out) >= max {
			break
		}
	}
	return out
}

func parameterBugClassHints(param string) ([]string, []string, int) {
	p := strings.ToLower(strings.TrimSpace(param))
	classes := []string{"parameterized_endpoint"}
	reasons := []string{"query parameter discovered in target URL inventory"}
	score := 25

	add := func(class string, reason string, delta int) {
		for _, existing := range classes {
			if existing == class {
				return
			}
		}
		classes = append(classes, class)
		reasons = append(reasons, reason)
		score += delta
	}

	switch {
	case p == "q" || p == "s" || strings.Contains(p, "search") || strings.Contains(p, "query") || strings.Contains(p, "keyword"):
		add("xss", "search-like parameter may reflect into HTML/DOM contexts", 25)
		add("sqli", "search/query parameter can indicate server-side filtering or lookup behavior", 15)
	case strings.Contains(p, "redirect") || strings.Contains(p, "return") || strings.Contains(p, "next") || strings.Contains(p, "url") || strings.Contains(p, "callback"):
		add("open_redirect", "redirect/url-like parameter may influence Location or navigation flow", 30)
		add("ssrf_candidate", "url-like parameter may later need SSRF sink analysis if server-side fetch behavior exists", 10)
	case strings.Contains(p, "id") || strings.Contains(p, "user") || strings.Contains(p, "account") || strings.Contains(p, "org") || strings.Contains(p, "tenant") || strings.Contains(p, "order"):
		add("idor_bola", "identifier-like parameter may cross authorization or ownership boundaries", 30)
		add("api_authorization", "object identifier can support access-control validation when authenticated context exists", 15)
	case strings.Contains(p, "file") || strings.Contains(p, "path") || strings.Contains(p, "dir") || strings.Contains(p, "download") || strings.Contains(p, "template") || strings.Contains(p, "include"):
		add("path_traversal_file_read", "file/path-like parameter may influence filesystem or template lookup behavior", 35)
	case strings.Contains(p, "page") || strings.Contains(p, "view") || strings.Contains(p, "template"):
		add("template_injection", "template/view-like parameter may affect rendering or include behavior", 15)
		add("path_traversal_file_read", "template/view-like parameter may map to file/include behavior", 15)
	case strings.Contains(p, "sort") || strings.Contains(p, "filter") || strings.Contains(p, "where") || strings.Contains(p, "select"):
		add("sqli", "filter/sort-like parameter may influence backend query construction", 25)
		add("nosqli", "filter-like parameter can indicate API query object behavior", 10)
	}

	if len(classes) == 1 {
		add("injection_candidate", "generic parameter should remain available for later class-specific validation", 5)
	}

	if score > 100 {
		score = 100
	}
	return classes, reasons, score
}

func addParameterInventoryCandidate(candidates map[string]*parameterInventoryCandidate, paramName string, rawURL string, source string) {
	name := strings.TrimSpace(paramName)
	if name == "" {
		return
	}

	key := strings.ToLower(name)
	candidate, ok := candidates[key]
	if !ok {
		classes, reasons, score := parameterBugClassHints(name)
		candidate = &parameterInventoryCandidate{
			Name:       name,
			BugClasses: classes,
			Reasons:    reasons,
			Score:      score,
		}
		candidates[key] = candidate
	}

	candidate.URLs = uniqueStrings(append(candidate.URLs, rawURL), 8)
	candidate.Sources = uniqueStrings(append(candidate.Sources, source), 8)

	occurrenceBonus := len(candidate.URLs) * 3
	if occurrenceBonus > 15 {
		occurrenceBonus = 15
	}
	if candidate.Score+occurrenceBonus <= 100 {
		candidate.Score += occurrenceBonus
	}
}

func primaryParameterBugClass(classes []string) string {
	for _, class := range classes {
		c := strings.TrimSpace(class)
		if c == "" || c == "parameterized_endpoint" || c == "injection_candidate" {
			continue
		}
		return c
	}

	for _, class := range classes {
		c := strings.TrimSpace(class)
		if c != "" {
			return c
		}
	}

	return "parameterized_endpoint"
}

func buildParameterInventoryCandidates(foundURLs []models.FoundURL) []parameterInventoryCandidate {
	candidates := map[string]*parameterInventoryCandidate{}

	for _, row := range foundURLs {
		raw := strings.TrimSpace(row.Value)
		if raw == "" {
			continue
		}

		parsed, err := url.Parse(raw)
		if err != nil || parsed == nil {
			continue
		}

		values := parsed.Query()
		if len(values) == 0 {
			continue
		}

		for name := range values {
			addParameterInventoryCandidate(candidates, name, raw, row.Source)
		}
	}

	out := make([]parameterInventoryCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		candidate.URLs = uniqueStrings(candidate.URLs, 8)
		candidate.Sources = uniqueStrings(candidate.Sources, 8)
		candidate.BugClasses = uniqueStrings(candidate.BugClasses, 8)
		candidate.Reasons = uniqueStrings(candidate.Reasons, 8)
		out = append(out, *candidate)
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
		}
		return out[i].Score > out[j].Score
	})

	if len(out) > 50 {
		out = out[:50]
	}

	return out
}

func parameterInventoryMemoryContent(foundURLCount int, candidates []parameterInventoryCandidate) (string, string) {
	lines := []string{
		"Operator skill: parameter_inventory",
		fmt.Sprintf("Sampled found_urls: %d", foundURLCount),
		fmt.Sprintf("Parameter candidates: %d", len(candidates)),
	}

	if len(candidates) == 0 {
		lines = append(lines,
			"Learning: No parameterized URLs were available in the sampled found_urls inventory for this target at execution time.",
			"Operator hint: prioritize crawling, URL discovery, JS/API route mining, and authenticated browsing before class-specific parameter validation.",
		)

		return strings.Join(lines, "\n"), "Parameter inventory found no query parameters in the sampled URL inventory."
	}

	lines = append(lines,
		"Learning: Parameterized URLs exist and should guide authorized class-specific validation.",
		"Top parameter candidates:",
	)

	limit := len(candidates)
	if limit > 10 {
		limit = 10
	}

	summaryParts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		candidate := candidates[i]
		bugClasses := strings.Join(candidate.BugClasses, ", ")
		if bugClasses == "" {
			bugClasses = "parameterized_endpoint"
		}

		exampleURL := ""
		if len(candidate.URLs) > 0 {
			exampleURL = candidate.URLs[0]
		}

		lines = append(lines, fmt.Sprintf(
			"- %s: score=%d candidate_bug_classes=%s examples=%d first_url=%s",
			candidate.Name,
			candidate.Score,
			bugClasses,
			len(candidate.URLs),
			exampleURL,
		))
		summaryParts = append(summaryParts, fmt.Sprintf("%s[%s]", candidate.Name, bugClasses))
	}

	lines = append(lines,
		"Next operator hint: use these parameter candidates to select controlled validation skills such as xss_reflection, open_redirect, path_traversal_baseline, auth_context_needed, IDOR/BOLA workflows, or SQLi/NoSQLi strategy when authorization and context allow.",
	)

	return strings.Join(lines, "\n"), "Parameter inventory found candidates: " + strings.Join(summaryParts, "; ")
}

func persistParameterInventorySkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, foundURLCount int, candidates []parameterInventoryCandidate, observationIDs []uint) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := parameterInventoryMemoryContent(foundURLCount, candidates)

	sourceType := models.TargetMemorySourceEvidence
	sourceID := &run.ID
	if run.ChatMessageID != nil && *run.ChatMessageID > 0 {
		sourceType = models.TargetMemorySourceChatMessage
		sourceID = run.ChatMessageID
	}

	importance := 45
	confidence := 65
	if len(candidates) > 0 {
		importance = 75
		confidence = 80
	}

	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: sourceType,
		SourceID:   sourceID,
		MemoryType: models.TargetMemoryTypeParameterNote,
		Title:      "Operator parameter inventory: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"parameter_inventory",
			"parameters",
			"attack_surface",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: importance,
		Confidence: confidence,
		SourceHash: memoryHash(
			"operator_skill_parameter_inventory_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":         "operator-skill-parameter-inventory-memory-v1",
			"skill":                  parameterInventorySkillSlug,
			"skill_run_id":           run.ID,
			"chat_session_id":        run.ChatSessionID,
			"chat_message_id":        run.ChatMessageID,
			"found_url_sample_count": foundURLCount,
			"parameter_count":        len(candidates),
			"observation_count":      len(observationIDs),
			"observation_ids":        observationIDs,
			"operator_ready":         true,
			"next_operator_hint":     "Use parameter inventory memory to prioritize authorized class-specific validation and avoid blind/redundant testing.",
		}, "{}"),
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

func httpEvidenceObservationSeverity(result models.ControlledTestResult) string {
	hint := strings.ToLower(strings.TrimSpace(result.SeverityHint))
	switch hint {
	case models.FindingSeverityCritical, models.FindingSeverityHigh, models.FindingSeverityMedium, models.FindingSeverityLow, models.FindingSeverityInfo:
		return hint
	}

	switch result.Status {
	case models.ControlledTestResultStatusVulnerable:
		return models.FindingSeverityHigh
	case models.ControlledTestResultStatusPassed:
		return models.FindingSeverityInfo
	case models.ControlledTestResultStatusBlocked, models.ControlledTestResultStatusInconclusive, models.ControlledTestResultStatusNeedsManualReview:
		return models.FindingSeverityInfo
	case models.ControlledTestResultStatusFailed:
		return models.FindingSeverityLow
	default:
		return models.FindingSeverityInfo
	}
}

func httpEvidenceObservationConfidence(result models.ControlledTestResult) int {
	switch result.Status {
	case models.ControlledTestResultStatusVulnerable:
		return 90
	case models.ControlledTestResultStatusPassed:
		return 75
	case models.ControlledTestResultStatusNeedsManualReview:
		return 65
	case models.ControlledTestResultStatusFailed:
		return 60
	case models.ControlledTestResultStatusInconclusive:
		return 50
	case models.ControlledTestResultStatusBlocked:
		return 45
	default:
		return 50
	}
}

func httpEvidenceLearning(result models.ControlledTestResult) string {
	status := strings.ToLower(strings.TrimSpace(result.Status))
	switch status {
	case models.ControlledTestResultStatusVulnerable:
		return "Controlled runtime produced vulnerability-marked evidence. Promote only if evidence quality, scope, and impact are confirmed."
	case models.ControlledTestResultStatusPassed:
		return "Controlled runtime produced a positive baseline. This can support follow-up class-specific validation when endpoint behavior and parameters justify it."
	case models.ControlledTestResultStatusBlocked:
		return "Controlled runtime was blocked/challenged. Treat this as environmental or access-control evidence, not proof of a vulnerability."
	case models.ControlledTestResultStatusInconclusive:
		return "Controlled runtime evidence is inconclusive. Avoid claiming impact; choose better context, authenticated state, browser-aware validation, or alternate endpoints."
	case models.ControlledTestResultStatusNeedsManualReview:
		return "Controlled runtime needs manual review before escalation. Human/operator inspection should decide whether controlled validation is warranted."
	case models.ControlledTestResultStatusFailed:
		return "Controlled runtime failed. This is useful failure evidence and should prevent blind retesting without changing the method or context."
	default:
		return "Controlled runtime evidence should be used as context for the next authorized validation decision."
	}
}

func httpEvidenceAnalysisMemoryContent(results []models.ControlledTestResult, statusCounts map[string]int) (string, string) {
	lines := []string{
		"Operator skill: http_evidence_analysis",
		fmt.Sprintf("Controlled runtime results sampled: %d", len(results)),
	}

	if len(results) == 0 {
		lines = append(lines,
			"Learning: No controlled runtime evidence was available for this target at execution time.",
			"Operator hint: run low-risk baseline probes or approved controlled validations before relying on HTTP evidence analysis.",
		)
		return strings.Join(lines, "\n"), "HTTP evidence analysis found no controlled runtime results for this target."
	}

	statusKeys := make([]string, 0, len(statusCounts))
	for key := range statusCounts {
		statusKeys = append(statusKeys, key)
	}
	sort.Strings(statusKeys)

	lines = append(lines, "Status counts:")
	countParts := make([]string, 0, len(statusKeys))
	for _, key := range statusKeys {
		lines = append(lines, fmt.Sprintf("- %s: %d", key, statusCounts[key]))
		countParts = append(countParts, fmt.Sprintf("%s=%d", key, statusCounts[key]))
	}

	lines = append(lines, "Recent evidence highlights:")
	limit := len(results)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		result := results[i]
		lines = append(lines, fmt.Sprintf(
			"- result_id=%d status=%s runtime=%s method=%s url=%s learning=%s",
			result.ID,
			result.Status,
			result.RuntimeType,
			result.Method,
			result.URL,
			httpEvidenceLearning(result),
		))
	}

	lines = append(lines,
		"Next operator hint: use controlled HTTP evidence to avoid overclaiming blocked/inconclusive responses and to choose the next authorized validation skill based on endpoint behavior.",
	)

	return strings.Join(lines, "\n"), "HTTP evidence analysis summarized controlled runtime status counts: " + strings.Join(countParts, ", ")
}

func persistHTTPEvidenceAnalysisSkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, results []models.ControlledTestResult, statusCounts map[string]int, observationIDs []uint) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := httpEvidenceAnalysisMemoryContent(results, statusCounts)

	importance := 50
	confidence := 65
	if len(results) > 0 {
		importance = 70
		confidence = 75
	}
	if statusCounts[models.ControlledTestResultStatusVulnerable] > 0 || statusCounts[models.ControlledTestResultStatusNeedsManualReview] > 0 {
		importance = 85
		confidence = 80
	}

	sourceID := run.ID
	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceEvidence,
		SourceID:   &sourceID,
		MemoryType: models.TargetMemoryTypeTestResult,
		Title:      "Operator HTTP evidence analysis: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"http_evidence_analysis",
			"controlled_runtime",
			"http_probe",
			"evidence_analysis",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: importance,
		Confidence: confidence,
		SourceHash: memoryHash(
			"operator_skill_http_evidence_analysis_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":     "operator-skill-http-evidence-analysis-memory-v1",
			"skill":              httpEvidenceAnalysisSkillSlug,
			"skill_run_id":       run.ID,
			"chat_session_id":    run.ChatSessionID,
			"chat_message_id":    run.ChatMessageID,
			"result_count":       len(results),
			"status_counts":      statusCounts,
			"observation_count":  len(observationIDs),
			"observation_ids":    observationIDs,
			"operator_ready":     true,
			"next_operator_hint": "Use controlled runtime evidence memory to choose authorized validation paths and avoid overclaiming blocked/inconclusive evidence.",
		}, "{}"),
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

func executeHTTPEvidenceAnalysisSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, httpEvidenceAnalysisSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	results := make([]models.ControlledTestResult, 0)
	if err := db.
		Where("target_id = ? AND user_id = ? AND owner_key = ?", target.ID, uid, ownerKey).
		Order("created_at DESC, id DESC").
		Limit(100).
		Find(&results).Error; err != nil {
		return nil, err
	}

	statusCounts := map[string]int{}
	for _, result := range results {
		status := strings.TrimSpace(result.Status)
		if status == "" {
			status = "unknown"
		}
		statusCounts[status]++
	}

	runSummaries := make([]map[string]interface{}, 0, len(runs))
	now := time.Now().UTC()

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0)
		limit := len(results)
		if limit > 30 {
			limit = 30
		}

		for i := 0; i < limit; i++ {
			result := results[i]
			severity := httpEvidenceObservationSeverity(result)
			confidence := httpEvidenceObservationConfidence(result)
			learning := httpEvidenceLearning(result)

			titleURL := result.URL
			if titleURL == "" {
				titleURL = fmt.Sprintf("controlled-result-%d", result.ID)
			}

			observationType := models.OperatorSkillObservationTypeEvidence
			if result.Status == models.ControlledTestResultStatusFailed || result.Status == models.ControlledTestResultStatusBlocked || result.Status == models.ControlledTestResultStatusInconclusive {
				observationType = models.OperatorSkillObservationTypeLearning
			}

			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       httpEvidenceAnalysisSkillSlug,
				ObservationType: observationType,
				Title:           fmt.Sprintf("HTTP evidence result: %s %s", strings.ToUpper(result.Status), titleURL),
				Summary:         fmt.Sprintf("Controlled runtime result %d produced status=%s runtime=%s severity_hint=%s confidence=%s.", result.ID, result.Status, result.RuntimeType, result.SeverityHint, result.Confidence),
				Content:         learning,
				URL:             result.URL,
				BugClass:        result.BugType,
				Confidence:      confidence,
				Severity:        severity,
				Status:          result.Status,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"controlled_result_id": result.ID,
					"controlled_run_id":    result.RunID,
					"agent_action_id":      result.AgentActionID,
					"runtime_type":         result.RuntimeType,
					"bug_type":             result.BugType,
					"check_key":            result.CheckKey,
					"url":                  result.URL,
					"method":               result.Method,
					"status":               result.Status,
					"confidence":           result.Confidence,
					"severity_hint":        result.SeverityHint,
					"learning":             learning,
					"request_json":         result.RequestJSON,
					"response_json":        result.ResponseJSON,
					"evidence_json":        result.EvidenceJSON,
					"matcher_json":         result.MatcherJSON,
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "http-evidence-analysis-skill-v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "http-evidence-analysis-observation-v1",
				}),
			}

			if observation.BugClass == "" {
				observation.BugClass = "http_evidence"
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		}

		memoryItemID, memoryWasCreated, memoryErr := persistHTTPEvidenceAnalysisSkillMemory(uid, ownerKey, target, run, results, statusCounts, createdObservationIDs)

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("HTTP evidence analysis completed: %d controlled runtime result(s) analyzed and %d observation(s) created.", len(results), len(createdObservationIDs))
		nextStep := "Use HTTP evidence observations to choose the next authorized validation skill. Do not promote blocked or inconclusive evidence as vulnerability proof."

		output := map[string]interface{}{
			"status":             status,
			"skill":              httpEvidenceAnalysisSkillSlug,
			"result_count":       len(results),
			"status_counts":      statusCounts,
			"observation_count":  len(createdObservationIDs),
			"observation_ids":    createdObservationIDs,
			"runtime_version":    "http-evidence-analysis-skill-runtime-v1",
			"memory_ingested":    memoryErr == nil && memoryItemID > 0,
			"memory_item_id":     memoryItemID,
			"memory_was_created": memoryWasCreated,
			"next_recommended_skills": []string{
				"xss_reflection",
				"open_redirect",
				"path_traversal_baseline",
				"auth_context_needed",
			},
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":       run.ID,
			"status":             status,
			"result_count":       len(results),
			"status_counts":      statusCounts,
			"observation_count":  len(createdObservationIDs),
			"observation_ids":    createdObservationIDs,
			"memory_ingested":    memoryErr == nil && memoryItemID > 0,
			"memory_item_id":     memoryItemID,
			"memory_was_created": memoryWasCreated,
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	return map[string]interface{}{
		"status":        "completed",
		"runs":          runSummaries,
		"run_count":     len(runSummaries),
		"result_count":  len(results),
		"status_counts": statusCounts,
	}, nil
}

func authContextNeededContent(target *models.Target) (string, string) {
	root := "target"
	if target != nil && strings.TrimSpace(target.RootDomain) != "" {
		root = target.RootDomain
	}

	lines := []string{
		"Operator skill: auth_context_needed",
		"Purpose: collect the minimum authorized context required for real access-control, IDOR/BOLA/BFLA, API authorization, session/JWT/OAuth, and business-logic validation.",
		"Target: " + root,
		"",
		"Required context before meaningful validation:",
		"- In-scope authenticated test account credentials or session cookies/tokens.",
		"- Preferably two accounts with different ownership boundaries for IDOR/BOLA comparison.",
		"- Role/tenant/org context if the application has multiple roles, organizations, teams, or subscriptions.",
		"- Explicit authorization for account-to-account authorization checks and any state-changing workflows.",
		"- Known sensitive object identifiers, API endpoints, account/profile/order/resource URLs, or workflows to prioritize.",
		"- Stop conditions and rate limits for controlled validation.",
		"",
		"Operator hint: do not claim IDOR/API authorization impact without comparing authorized and unauthorized behavior under approved authenticated context.",
	}

	summary := "Authenticated context and preferably a second test account are needed before real IDOR/API authorization/business-logic validation."
	return strings.Join(lines, "\n"), summary
}

func persistAuthContextNeededSkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, observationIDs []uint) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := authContextNeededContent(target)
	sourceID := run.ID

	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceAgentAction,
		SourceID:   &sourceID,
		MemoryType: models.TargetMemoryTypePolicyConstraint,
		Title:      "Operator auth context needed: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"auth_context_needed",
			"access_control",
			"idor",
			"api_authorization",
			"authenticated_testing",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: 85,
		Confidence: 85,
		SourceHash: memoryHash(
			"operator_skill_auth_context_needed_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":     "operator-skill-auth-context-needed-memory-v1",
			"skill":              authContextNeededSkillSlug,
			"skill_run_id":       run.ID,
			"chat_session_id":    run.ChatSessionID,
			"chat_message_id":    run.ChatMessageID,
			"observation_count":  len(observationIDs),
			"observation_ids":    observationIDs,
			"operator_ready":     true,
			"needs_user_input":   true,
			"next_operator_hint": "Ask the user for authenticated context, second test account, authorization boundaries, and stop conditions before access-control validation.",
		}, "{}"),
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

type jsAuditCandidate struct {
	URL        string
	Kind       string
	BugClasses []string
	Reason     string
	Source     string
	Score      int
}

func classifyJSAuditURL(rawURL string, source string) (jsAuditCandidate, bool) {
	value := strings.TrimSpace(rawURL)
	if value == "" {
		return jsAuditCandidate{}, false
	}

	lower := strings.ToLower(value)
	parsed, _ := url.Parse(value)
	pathValue := strings.ToLower(parsed.Path)

	candidate := jsAuditCandidate{
		URL:        value,
		Source:     strings.TrimSpace(source),
		BugClasses: []string{"client_side_attack_surface"},
		Score:      35,
	}

	addClass := func(class string) {
		for _, existing := range candidate.BugClasses {
			if existing == class {
				return
			}
		}
		candidate.BugClasses = append(candidate.BugClasses, class)
	}

	switch {
	case strings.HasSuffix(pathValue, ".js") || strings.Contains(lower, ".js?"):
		candidate.Kind = "javascript_asset"
		candidate.Reason = "JavaScript asset discovered in URL inventory; useful for endpoint mining, secrets review, source map checks, and DOM sink discovery."
		addClass("js_review")
		addClass("dom_xss")
		addClass("exposed_secrets")
		candidate.Score = 75
	case strings.HasSuffix(pathValue, ".map") || strings.Contains(lower, ".js.map"):
		candidate.Kind = "sourcemap_candidate"
		candidate.Reason = "Source map-like asset discovered; source maps may expose original client code, routes, comments, or sensitive implementation details."
		addClass("source_map_exposure")
		addClass("exposed_secrets")
		candidate.Score = 80
	case strings.Contains(pathValue, "/api/") || strings.Contains(pathValue, "/ajax/") || strings.Contains(pathValue, "/rest/") || strings.Contains(pathValue, "/graphql") || strings.Contains(pathValue, "/v1/") || strings.Contains(pathValue, "/v2/") || strings.Contains(pathValue, "/v3/"):
		candidate.Kind = "api_like_route"
		candidate.Reason = "API-like route discovered in URL inventory; useful for parameter mining, authz testing, IDOR/BOLA hypotheses, and schema discovery."
		addClass("api_authorization")
		addClass("idor_bola")
		addClass("parameterized_endpoint")
		candidate.Score = 70
	case strings.Contains(pathValue, "/app") || strings.Contains(pathValue, "/dashboard") || strings.Contains(pathValue, "/admin") || strings.Contains(pathValue, "/account") || strings.Contains(pathValue, "/profile") || strings.Contains(pathValue, "/user"):
		candidate.Kind = "frontend_route"
		candidate.Reason = "Frontend/application route discovered; useful for authenticated workflow mapping, DOM route review, and access-control context planning."
		addClass("access_control")
		addClass("business_logic")
		candidate.Score = 55
	default:
		return jsAuditCandidate{}, false
	}

	return candidate, true
}

func buildJSAuditCandidates(foundURLs []models.FoundURL) []jsAuditCandidate {
	seen := map[string]bool{}
	out := make([]jsAuditCandidate, 0)

	for _, foundURL := range foundURLs {
		candidate, ok := classifyJSAuditURL(foundURL.Value, foundURL.Source)
		if !ok {
			continue
		}

		key := candidate.Kind + "\x00" + candidate.URL
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, candidate)

		if len(out) >= 50 {
			break
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Score == out[j].Score {
			return out[i].URL < out[j].URL
		}
		return out[i].Score > out[j].Score
	})

	return out
}

func jsAuditMemoryContent(target *models.Target, candidates []jsAuditCandidate, sampledURLCount int) (string, string) {
	root := "target"
	if target != nil && strings.TrimSpace(target.RootDomain) != "" {
		root = target.RootDomain
	}

	if len(candidates) == 0 {
		content := strings.Join([]string{
			"Operator skill: js_audit",
			"Target: " + root,
			fmt.Sprintf("Sampled URL inventory: %d", sampledURLCount),
			"",
			"No JavaScript/API-like candidates were found in the sampled URL inventory.",
			"Learning: the operator needs better crawling, JS collection, browser-aware discovery, or imported JS resources before DOM/API/client-side validation can be meaningful.",
		}, "\n")
		return content, "JS audit found no JavaScript/API-like candidates in the sampled URL inventory."
	}

	lines := []string{
		"Operator skill: js_audit",
		"Target: " + root,
		fmt.Sprintf("Sampled URL inventory: %d", sampledURLCount),
		fmt.Sprintf("JS/API candidates: %d", len(candidates)),
		"",
		"Top candidates:",
	}

	for idx, candidate := range candidates {
		if idx >= 15 {
			break
		}
		lines = append(lines, fmt.Sprintf("- [%s] %s", candidate.Kind, candidate.URL))
		lines = append(lines, "  "+candidate.Reason)
	}

	return strings.Join(lines, "\n"), fmt.Sprintf("JS audit found %d JavaScript/API-like candidate(s) for client-side and API route analysis.", len(candidates))
}

func persistJSAuditSkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, candidates []jsAuditCandidate, observationIDs []uint, sampledURLCount int) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := jsAuditMemoryContent(target, candidates, sampledURLCount)
	sourceID := run.ID

	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceAgentAction,
		SourceID:   &sourceID,
		MemoryType: models.TargetMemoryTypeAttackSurface,
		Title:      "Operator JS audit: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"js_audit",
			"javascript",
			"client_side",
			"api_routes",
			"attack_surface",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: 70,
		Confidence: 70,
		SourceHash: memoryHash(
			"operator_skill_js_audit_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":      "operator-skill-js-audit-memory-v1",
			"skill":               jsAuditSkillSlug,
			"skill_run_id":        run.ID,
			"chat_session_id":     run.ChatSessionID,
			"chat_message_id":     run.ChatMessageID,
			"sampled_url_count":   sampledURLCount,
			"candidate_count":     len(candidates),
			"observation_count":   len(observationIDs),
			"observation_ids":     observationIDs,
			"next_operator_hint":  "Use JS/API candidates for endpoint mining, DOM sink review, secrets review, and authorized API/access-control validation planning.",
			"runtime_scope":       "inventory_only_no_new_crawl",
			"requires_browser_v2": len(candidates) == 0,
		}, "{}"),
	}

	if len(candidates) == 0 {
		item.Importance = 55
		item.Confidence = 65
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

type xssReflectionCandidate struct {
	ParamName  string
	URL        string
	Source     string
	Reason     string
	Confidence int
}

func isXSSRelevantParam(name string) bool {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return false
	}

	return p == "q" ||
		p == "s" ||
		p == "search" ||
		p == "query" ||
		p == "keyword" ||
		p == "term" ||
		p == "text" ||
		p == "message" ||
		p == "comment" ||
		p == "title" ||
		p == "name" ||
		p == "return" ||
		p == "next" ||
		p == "url" ||
		p == "callback" ||
		strings.Contains(p, "search") ||
		strings.Contains(p, "query") ||
		strings.Contains(p, "keyword") ||
		strings.Contains(p, "redirect") ||
		strings.Contains(p, "callback")
}

func buildXSSReflectionCandidates(foundURLs []models.FoundURL, parameterObservations []models.OperatorSkillObservation) []xssReflectionCandidate {
	seen := map[string]bool{}
	out := make([]xssReflectionCandidate, 0)

	add := func(candidate xssReflectionCandidate) {
		candidate.ParamName = strings.TrimSpace(candidate.ParamName)
		candidate.URL = strings.TrimSpace(candidate.URL)
		if candidate.ParamName == "" && candidate.URL == "" {
			return
		}
		key := strings.ToLower(candidate.ParamName) + "\x00" + candidate.URL
		if seen[key] {
			return
		}
		seen[key] = true
		if candidate.Confidence <= 0 {
			candidate.Confidence = 60
		}
		out = append(out, candidate)
	}

	for _, observation := range parameterObservations {
		if !strings.EqualFold(observation.BugClass, "xss") && !isXSSRelevantParam(observation.ParamName) {
			continue
		}

		reason := observation.Summary
		if strings.TrimSpace(reason) == "" {
			reason = "Parameter inventory observation indicates a reflection-relevant parameter candidate."
		}

		add(xssReflectionCandidate{
			ParamName:  observation.ParamName,
			URL:        observation.URL,
			Source:     "parameter_inventory_observation",
			Reason:     reason,
			Confidence: observation.Confidence,
		})
	}

	for _, foundURL := range foundURLs {
		value := strings.TrimSpace(foundURL.Value)
		if value == "" || !strings.Contains(value, "?") {
			continue
		}

		parsed, err := url.Parse(value)
		if err != nil {
			continue
		}

		for paramName := range parsed.Query() {
			if !isXSSRelevantParam(paramName) {
				continue
			}

			add(xssReflectionCandidate{
				ParamName:  paramName,
				URL:        value,
				Source:     "found_url_query_parameter",
				Reason:     "URL inventory contains an XSS/reflection-relevant query parameter. Runtime v1 only records a candidate and does not execute payloads.",
				Confidence: 65,
			})
		}

		if len(out) >= 50 {
			break
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			if out[i].ParamName == out[j].ParamName {
				return out[i].URL < out[j].URL
			}
			return out[i].ParamName < out[j].ParamName
		}
		return out[i].Confidence > out[j].Confidence
	})

	if len(out) > 50 {
		out = out[:50]
	}

	return out
}

func xssReflectionMemoryContent(target *models.Target, candidates []xssReflectionCandidate, sampledURLCount int) (string, string) {
	root := "target"
	if target != nil && strings.TrimSpace(target.RootDomain) != "" {
		root = target.RootDomain
	}

	if len(candidates) == 0 {
		content := strings.Join([]string{
			"Operator skill: xss_reflection",
			"Target: " + root,
			fmt.Sprintf("Sampled URL inventory: %d", sampledURLCount),
			"",
			"No XSS/reflection parameter candidates were found from current parameter inventory or found URLs.",
			"Learning: the operator needs richer crawling, parameter inventory, JS/API route mining, or prior reflection evidence before controlled XSS validation can be meaningful.",
			"Execution note: runtime v1 does not execute payloads.",
		}, "\n")
		return content, "XSS reflection planning found no candidate parameters from current inventory."
	}

	lines := []string{
		"Operator skill: xss_reflection",
		"Target: " + root,
		fmt.Sprintf("Candidate count: %d", len(candidates)),
		"",
		"Top reflection candidates:",
	}

	for idx, candidate := range candidates {
		if idx >= 15 {
			break
		}
		lines = append(lines, fmt.Sprintf("- param=%s url=%s", candidate.ParamName, candidate.URL))
		lines = append(lines, "  "+candidate.Reason)
	}

	lines = append(lines, "")
	lines = append(lines, "Execution note: runtime v1 records XSS reflection candidates only. Actual reflection probing and payload validation must be handled by controlled runtime with policy/scope/rate-limit controls.")

	return strings.Join(lines, "\n"), fmt.Sprintf("XSS reflection planning found %d candidate parameter(s) for controlled validation planning.", len(candidates))
}

func persistXSSReflectionSkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, candidates []xssReflectionCandidate, observationIDs []uint, sampledURLCount int) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := xssReflectionMemoryContent(target, candidates, sampledURLCount)
	sourceID := run.ID

	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceAgentAction,
		SourceID:   &sourceID,
		MemoryType: models.TargetMemoryTypeVulnerabilityHypothesis,
		Title:      "Operator XSS reflection plan: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"xss_reflection",
			"xss",
			"reflection",
			"parameter_inventory",
			"vulnerability_hypothesis",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: 65,
		Confidence: 70,
		SourceHash: memoryHash(
			"operator_skill_xss_reflection_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":     "operator-skill-xss-reflection-memory-v1",
			"skill":              xssReflectionSkillSlug,
			"skill_run_id":       run.ID,
			"chat_session_id":    run.ChatSessionID,
			"chat_message_id":    run.ChatMessageID,
			"sampled_url_count":  sampledURLCount,
			"candidate_count":    len(candidates),
			"observation_count":  len(observationIDs),
			"observation_ids":    observationIDs,
			"runtime_scope":      "planning_only_no_payload_execution",
			"next_operator_hint": "Use controlled runtime for reflection/context probing only when scope and policy allow.",
		}, "{}"),
	}

	if len(candidates) == 0 {
		item.Importance = 50
		item.Confidence = 60
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

type openRedirectCandidate struct {
	ParamName  string
	URL        string
	Source     string
	Reason     string
	Confidence int
}

func isOpenRedirectRelevantParam(name string) bool {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return false
	}

	return p == "url" ||
		p == "uri" ||
		p == "u" ||
		p == "next" ||
		p == "return" ||
		p == "return_url" ||
		p == "redirect" ||
		p == "redirect_url" ||
		p == "redirect_uri" ||
		p == "redir" ||
		p == "callback" ||
		p == "continue" ||
		p == "destination" ||
		p == "dest" ||
		p == "target" ||
		p == "goto" ||
		p == "to" ||
		strings.Contains(p, "redirect") ||
		strings.Contains(p, "return") ||
		strings.Contains(p, "callback") ||
		strings.Contains(p, "next") ||
		strings.Contains(p, "url") ||
		strings.Contains(p, "uri")
}

func buildOpenRedirectCandidates(foundURLs []models.FoundURL, parameterObservations []models.OperatorSkillObservation) []openRedirectCandidate {
	seen := map[string]bool{}
	out := make([]openRedirectCandidate, 0)

	add := func(candidate openRedirectCandidate) {
		candidate.ParamName = strings.TrimSpace(candidate.ParamName)
		candidate.URL = strings.TrimSpace(candidate.URL)
		if candidate.ParamName == "" && candidate.URL == "" {
			return
		}

		key := strings.ToLower(candidate.ParamName) + "\x00" + candidate.URL
		if seen[key] {
			return
		}

		seen[key] = true
		if candidate.Confidence <= 0 {
			candidate.Confidence = 65
		}
		out = append(out, candidate)
	}

	for _, observation := range parameterObservations {
		if !strings.EqualFold(observation.BugClass, "open_redirect") && !isOpenRedirectRelevantParam(observation.ParamName) {
			continue
		}

		reason := observation.Summary
		if strings.TrimSpace(reason) == "" {
			reason = "Parameter inventory observation indicates a redirect/url-like parameter candidate."
		}

		add(openRedirectCandidate{
			ParamName:  observation.ParamName,
			URL:        observation.URL,
			Source:     "parameter_inventory_observation",
			Reason:     reason,
			Confidence: observation.Confidence,
		})
	}

	for _, foundURL := range foundURLs {
		value := strings.TrimSpace(foundURL.Value)
		if value == "" || !strings.Contains(value, "?") {
			continue
		}

		parsed, err := url.Parse(value)
		if err != nil {
			continue
		}

		for paramName := range parsed.Query() {
			if !isOpenRedirectRelevantParam(paramName) {
				continue
			}

			add(openRedirectCandidate{
				ParamName:  paramName,
				URL:        value,
				Source:     "found_url_query_parameter",
				Reason:     "URL inventory contains a redirect/url-like query parameter. Runtime v1 only records a candidate and does not execute redirect validation.",
				Confidence: 70,
			})
		}

		if len(out) >= 50 {
			break
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			if out[i].ParamName == out[j].ParamName {
				return out[i].URL < out[j].URL
			}
			return out[i].ParamName < out[j].ParamName
		}
		return out[i].Confidence > out[j].Confidence
	})

	if len(out) > 50 {
		out = out[:50]
	}

	return out
}

func openRedirectMemoryContent(target *models.Target, candidates []openRedirectCandidate, sampledURLCount int) (string, string) {
	root := "target"
	if target != nil && strings.TrimSpace(target.RootDomain) != "" {
		root = target.RootDomain
	}

	if len(candidates) == 0 {
		content := strings.Join([]string{
			"Operator skill: open_redirect",
			"Target: " + root,
			fmt.Sprintf("Sampled URL inventory: %d", sampledURLCount),
			"",
			"No open redirect candidate parameters were found from current parameter inventory or found URLs.",
			"Learning: the operator needs richer crawling, parameter inventory, login/logout flow mapping, OAuth/callback route discovery, or redirect-like URL evidence before controlled open redirect validation can be meaningful.",
			"Execution note: runtime v1 does not execute redirect payloads or external validation.",
		}, "\n")
		return content, "Open redirect planning found no redirect/url-like candidate parameters from current inventory."
	}

	lines := []string{
		"Operator skill: open_redirect",
		"Target: " + root,
		fmt.Sprintf("Candidate count: %d", len(candidates)),
		"",
		"Top redirect candidates:",
	}

	for idx, candidate := range candidates {
		if idx >= 15 {
			break
		}
		lines = append(lines, fmt.Sprintf("- param=%s url=%s", candidate.ParamName, candidate.URL))
		lines = append(lines, "  "+candidate.Reason)
	}

	lines = append(lines, "")
	lines = append(lines, "Execution note: runtime v1 records open redirect candidates only. Actual redirect behavior validation must be handled by controlled runtime with policy/scope/rate-limit controls.")

	return strings.Join(lines, "\n"), fmt.Sprintf("Open redirect planning found %d candidate parameter(s) for controlled validation planning.", len(candidates))
}

func persistOpenRedirectSkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, candidates []openRedirectCandidate, observationIDs []uint, sampledURLCount int) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := openRedirectMemoryContent(target, candidates, sampledURLCount)
	sourceID := run.ID

	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceAgentAction,
		SourceID:   &sourceID,
		MemoryType: models.TargetMemoryTypeVulnerabilityHypothesis,
		Title:      "Operator open redirect plan: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"open_redirect",
			"redirect",
			"url_parameter",
			"parameter_inventory",
			"vulnerability_hypothesis",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: 65,
		Confidence: 70,
		SourceHash: memoryHash(
			"operator_skill_open_redirect_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":     "operator-skill-open-redirect-memory-v1",
			"skill":              openRedirectSkillSlug,
			"skill_run_id":       run.ID,
			"chat_session_id":    run.ChatSessionID,
			"chat_message_id":    run.ChatMessageID,
			"sampled_url_count":  sampledURLCount,
			"candidate_count":    len(candidates),
			"observation_count":  len(observationIDs),
			"observation_ids":    observationIDs,
			"runtime_scope":      "planning_only_no_redirect_validation",
			"next_operator_hint": "Use controlled runtime for redirect behavior validation only when scope and policy allow.",
		}, "{}"),
	}

	if len(candidates) == 0 {
		item.Importance = 50
		item.Confidence = 60
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

type pathTraversalCandidate struct {
	ParamName  string
	URL        string
	Source     string
	Reason     string
	Confidence int
}

func isPathTraversalRelevantParam(name string) bool {
	p := strings.ToLower(strings.TrimSpace(name))
	if p == "" {
		return false
	}

	return p == "file" ||
		p == "filename" ||
		p == "path" ||
		p == "filepath" ||
		p == "dir" ||
		p == "directory" ||
		p == "folder" ||
		p == "download" ||
		p == "template" ||
		p == "tpl" ||
		p == "view" ||
		p == "page" ||
		p == "include" ||
		p == "load" ||
		p == "resource" ||
		p == "document" ||
		p == "doc" ||
		p == "export" ||
		p == "import" ||
		strings.Contains(p, "file") ||
		strings.Contains(p, "path") ||
		strings.HasPrefix(p, "dir_") ||
		strings.HasSuffix(p, "_dir") ||
		strings.Contains(p, "directory") ||
		strings.Contains(p, "download") ||
		strings.Contains(p, "template") ||
		strings.Contains(p, "include") ||
		strings.Contains(p, "folder")
}

func buildPathTraversalCandidates(foundURLs []models.FoundURL, parameterObservations []models.OperatorSkillObservation) []pathTraversalCandidate {
	seen := map[string]bool{}
	out := make([]pathTraversalCandidate, 0)

	add := func(candidate pathTraversalCandidate) {
		candidate.ParamName = strings.TrimSpace(candidate.ParamName)
		candidate.URL = strings.TrimSpace(candidate.URL)
		if candidate.ParamName == "" && candidate.URL == "" {
			return
		}

		key := strings.ToLower(candidate.ParamName) + "\x00" + candidate.URL
		if seen[key] {
			return
		}

		seen[key] = true
		if candidate.Confidence <= 0 {
			candidate.Confidence = 65
		}
		out = append(out, candidate)
	}

	for _, observation := range parameterObservations {
		if !strings.EqualFold(observation.BugClass, "path_traversal_file_read") && !strings.EqualFold(observation.BugClass, "path_traversal") && !isPathTraversalRelevantParam(observation.ParamName) {
			continue
		}

		reason := observation.Summary
		if strings.TrimSpace(reason) == "" {
			reason = "Parameter inventory observation indicates a file/path/template-like parameter candidate."
		}

		add(pathTraversalCandidate{
			ParamName:  observation.ParamName,
			URL:        observation.URL,
			Source:     "parameter_inventory_observation",
			Reason:     reason,
			Confidence: observation.Confidence,
		})
	}

	for _, foundURL := range foundURLs {
		value := strings.TrimSpace(foundURL.Value)
		if value == "" || !strings.Contains(value, "?") {
			continue
		}

		parsed, err := url.Parse(value)
		if err != nil {
			continue
		}

		for paramName := range parsed.Query() {
			if !isPathTraversalRelevantParam(paramName) {
				continue
			}

			add(pathTraversalCandidate{
				ParamName:  paramName,
				URL:        value,
				Source:     "found_url_query_parameter",
				Reason:     "URL inventory contains a file/path/template-like query parameter. Runtime v1 only records a candidate and does not execute traversal or file-read validation.",
				Confidence: 70,
			})
		}

		if len(out) >= 50 {
			break
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Confidence == out[j].Confidence {
			if out[i].ParamName == out[j].ParamName {
				return out[i].URL < out[j].URL
			}
			return out[i].ParamName < out[j].ParamName
		}
		return out[i].Confidence > out[j].Confidence
	})

	if len(out) > 50 {
		out = out[:50]
	}

	return out
}

func pathTraversalMemoryContent(target *models.Target, candidates []pathTraversalCandidate, sampledURLCount int) (string, string) {
	root := "target"
	if target != nil && strings.TrimSpace(target.RootDomain) != "" {
		root = target.RootDomain
	}

	if len(candidates) == 0 {
		content := strings.Join([]string{
			"Operator skill: path_traversal_baseline",
			"Target: " + root,
			fmt.Sprintf("Sampled URL inventory: %d", sampledURLCount),
			"",
			"No path traversal/file-read candidate parameters were found from current parameter inventory or found URLs.",
			"Learning: the operator needs richer crawling, download/export route discovery, file/template parameter inventory, or upload/download workflow mapping before controlled file-read validation can be meaningful.",
			"Execution note: runtime v1 does not execute traversal payloads or file-read validation.",
		}, "\n")
		return content, "Path traversal planning found no file/path-like candidate parameters from current inventory."
	}

	lines := []string{
		"Operator skill: path_traversal_baseline",
		"Target: " + root,
		fmt.Sprintf("Candidate count: %d", len(candidates)),
		"",
		"Top file/path candidates:",
	}

	for idx, candidate := range candidates {
		if idx >= 15 {
			break
		}
		lines = append(lines, fmt.Sprintf("- param=%s url=%s", candidate.ParamName, candidate.URL))
		lines = append(lines, "  "+candidate.Reason)
	}

	lines = append(lines, "")
	lines = append(lines, "Execution note: runtime v1 records path traversal/file-read candidates only. Actual file-read behavior validation must be handled by controlled runtime with policy/scope/rate-limit controls.")

	return strings.Join(lines, "\n"), fmt.Sprintf("Path traversal planning found %d candidate parameter(s) for controlled validation planning.", len(candidates))
}

func persistPathTraversalSkillMemory(uid uint, ownerKey string, target *models.Target, run models.OperatorSkillRun, candidates []pathTraversalCandidate, observationIDs []uint, sampledURLCount int) (uint, bool, error) {
	if target == nil {
		return 0, false, nil
	}

	content, summary := pathTraversalMemoryContent(target, candidates, sampledURLCount)
	sourceID := run.ID

	item := models.TargetMemoryItem{
		UserID:     uid,
		OwnerKey:   ownerKey,
		TargetID:   target.ID,
		SourceType: models.TargetMemorySourceAgentAction,
		SourceID:   &sourceID,
		MemoryType: models.TargetMemoryTypeVulnerabilityHypothesis,
		Title:      "Operator path traversal plan: " + target.RootDomain,
		Content:    content,
		Summary:    summary,
		Tags: memoryJSON([]string{
			"operator_skill",
			"path_traversal_baseline",
			"path_traversal",
			"file_read",
			"download",
			"template",
			"parameter_inventory",
			"vulnerability_hypothesis",
			"authorized_validation",
			"rag",
			"operator",
		}, "[]"),
		Importance: 65,
		Confidence: 70,
		SourceHash: memoryHash(
			"operator_skill_path_traversal_latest",
			fmt.Sprint(target.ID),
			ownerKey,
		),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version":     "operator-skill-path-traversal-memory-v1",
			"skill":              pathTraversalBaselineSkillSlug,
			"skill_run_id":       run.ID,
			"chat_session_id":    run.ChatSessionID,
			"chat_message_id":    run.ChatMessageID,
			"sampled_url_count":  sampledURLCount,
			"candidate_count":    len(candidates),
			"observation_count":  len(observationIDs),
			"observation_ids":    observationIDs,
			"runtime_scope":      "planning_only_no_file_read_validation",
			"next_operator_hint": "Use controlled runtime for file-read/path traversal behavior validation only when scope and policy allow.",
		}, "{}"),
	}

	if len(candidates) == 0 {
		item.Importance = 50
		item.Confidence = 60
	}

	row, wasCreated, err := upsertTargetMemoryItem(item)
	if err != nil {
		return 0, false, err
	}

	return row.ID, wasCreated, nil
}

func executePathTraversalSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, pathTraversalBaselineSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	parameterObservations := make([]models.OperatorSkillObservation, 0)
	_ = db.
		Where("target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ? AND observation_type = ?",
			target.ID,
			uid,
			ownerKey,
			parameterInventorySkillSlug,
			models.OperatorSkillObservationTypeCandidate,
		).
		Order("created_at DESC, id DESC").
		Limit(200).
		Find(&parameterObservations).Error

	candidates := buildPathTraversalCandidates(foundURLs, parameterObservations)
	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0, len(candidates))

		if len(candidates) == 0 {
			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       pathTraversalBaselineSkillSlug,
				ObservationType: models.OperatorSkillObservationTypeLearning,
				Title:           "Path traversal planning needs file/path-like parameters",
				Summary:         "No file/path/template/download candidate parameters were found from current inventory.",
				Content:         "Learning: improve crawling, download/export route discovery, file/template parameter inventory, or upload/download workflow mapping before controlled path traversal/file-read validation. Runtime v1 does not execute traversal payloads.",
				BugClass:        "path_traversal_file_read",
				Confidence:      60,
				Severity:        models.FindingSeverityInfo,
				Status:          models.OperatorSkillRunStatusCompleted,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"sampled_url_count":           len(foundURLs),
					"parameter_observation_count": len(parameterObservations),
					"candidate_count":             0,
					"skill":                       pathTraversalBaselineSkillSlug,
					"runtime_scope":               "planning_only_no_file_read_validation",
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "path-traversal-skill-v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "path-traversal-learning-v1",
				}),
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		} else {
			for _, candidate := range candidates {
				observation := models.OperatorSkillObservation{
					UserID:          uid,
					OwnerKey:        ownerKey,
					TargetID:        target.ID,
					SkillRunID:      run.ID,
					SkillSlug:       pathTraversalBaselineSkillSlug,
					ObservationType: models.OperatorSkillObservationTypeCandidate,
					Title:           fmt.Sprintf("Path traversal candidate: %s", candidate.ParamName),
					Summary:         fmt.Sprintf("Parameter %q is a candidate for controlled file-read/path traversal validation.", candidate.ParamName),
					Content:         candidate.Reason + "\nRuntime v1 does not execute traversal or file-read validation.",
					URL:             candidate.URL,
					ParamName:       candidate.ParamName,
					BugClass:        "path_traversal_file_read",
					Confidence:      candidate.Confidence,
					Severity:        models.FindingSeverityInfo,
					Status:          "candidate",
					EvidenceJSON: chatJSON(map[string]interface{}{
						"parameter":     candidate.ParamName,
						"url":           candidate.URL,
						"source":        candidate.Source,
						"reason":        candidate.Reason,
						"confidence":    candidate.Confidence,
						"skill":         pathTraversalBaselineSkillSlug,
						"runtime_scope": "planning_only_no_file_read_validation",
					}),
					Metadata: chatJSON(map[string]interface{}{
						"created_by":          "path-traversal-skill-v1",
						"operator_skill_run":  run.ID,
						"target_id":           target.ID,
						"observation_version": "path-traversal-candidate-v1",
					}),
				}

				if err := db.Create(&observation).Error; err != nil {
					return nil, err
				}
				createdObservationIDs = append(createdObservationIDs, observation.ID)
			}
		}

		memoryItemID, memoryWasCreated, memoryErr := persistPathTraversalSkillMemory(uid, ownerKey, target, run, candidates, createdObservationIDs, len(foundURLs))

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("Path traversal planning reviewed %d URL(s), %d parameter observation(s), and found %d candidate(s).", len(foundURLs), len(parameterObservations), len(candidates))
		nextStep := "Use controlled runtime for file-read/path traversal behavior validation only when scope and policy allow."
		if len(candidates) == 0 {
			nextStep = "Improve URL discovery, download/export route discovery, file/template parameter inventory, or workflow mapping before controlled file-read/path traversal validation."
		}

		output := map[string]interface{}{
			"status":                      status,
			"skill":                       pathTraversalBaselineSkillSlug,
			"runtime_version":             "path-traversal-skill-runtime-v1",
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"memory_ingested":             memoryErr == nil && memoryItemID > 0,
			"memory_item_id":              memoryItemID,
			"memory_was_created":          memoryWasCreated,
			"runtime_scope":               "planning_only_no_file_read_validation",
			"next_recommended_skills": []string{
				"path_traversal_probe",
				"http_evidence_analysis",
			},
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":                run.ID,
			"status":                      status,
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"memory_ingested":             memoryErr == nil && memoryItemID > 0,
			"memory_item_id":              memoryItemID,
			"memory_was_created":          memoryWasCreated,
			"runtime_scope":               "planning_only_no_file_read_validation",
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	return map[string]interface{}{
		"status":          models.OperatorSkillRunStatusCompleted,
		"runs":            runSummaries,
		"run_count":       len(runSummaries),
		"candidate_count": len(candidates),
		"runtime_scope":   "planning_only_no_file_read_validation",
	}, nil
}

func executeOpenRedirectSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, openRedirectSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	parameterObservations := make([]models.OperatorSkillObservation, 0)
	_ = db.
		Where("target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ? AND observation_type = ?",
			target.ID,
			uid,
			ownerKey,
			parameterInventorySkillSlug,
			models.OperatorSkillObservationTypeCandidate,
		).
		Order("created_at DESC, id DESC").
		Limit(200).
		Find(&parameterObservations).Error

	candidates := buildOpenRedirectCandidates(foundURLs, parameterObservations)
	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0, len(candidates))

		if len(candidates) == 0 {
			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       openRedirectSkillSlug,
				ObservationType: models.OperatorSkillObservationTypeLearning,
				Title:           "Open redirect planning needs redirect/url-like parameters",
				Summary:         "No open redirect candidate parameters were found from current inventory.",
				Content:         "Learning: improve crawling, parameter inventory, login/logout flow mapping, OAuth/callback route discovery, or redirect-like URL evidence before controlled open redirect validation. Runtime v1 does not execute redirect payloads.",
				BugClass:        "open_redirect",
				Confidence:      60,
				Severity:        models.FindingSeverityInfo,
				Status:          models.OperatorSkillRunStatusCompleted,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"sampled_url_count":           len(foundURLs),
					"parameter_observation_count": len(parameterObservations),
					"candidate_count":             0,
					"skill":                       openRedirectSkillSlug,
					"runtime_scope":               "planning_only_no_redirect_validation",
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "open-redirect-skill-v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "open-redirect-learning-v1",
				}),
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		} else {
			for _, candidate := range candidates {
				observation := models.OperatorSkillObservation{
					UserID:          uid,
					OwnerKey:        ownerKey,
					TargetID:        target.ID,
					SkillRunID:      run.ID,
					SkillSlug:       openRedirectSkillSlug,
					ObservationType: models.OperatorSkillObservationTypeCandidate,
					Title:           fmt.Sprintf("Open redirect candidate: %s", candidate.ParamName),
					Summary:         fmt.Sprintf("Parameter %q is a candidate for controlled redirect behavior validation.", candidate.ParamName),
					Content:         candidate.Reason + "\nRuntime v1 does not execute redirect validation.",
					URL:             candidate.URL,
					ParamName:       candidate.ParamName,
					BugClass:        "open_redirect",
					Confidence:      candidate.Confidence,
					Severity:        models.FindingSeverityInfo,
					Status:          "candidate",
					EvidenceJSON: chatJSON(map[string]interface{}{
						"parameter":     candidate.ParamName,
						"url":           candidate.URL,
						"source":        candidate.Source,
						"reason":        candidate.Reason,
						"confidence":    candidate.Confidence,
						"skill":         openRedirectSkillSlug,
						"runtime_scope": "planning_only_no_redirect_validation",
					}),
					Metadata: chatJSON(map[string]interface{}{
						"created_by":          "open-redirect-skill-v1",
						"operator_skill_run":  run.ID,
						"target_id":           target.ID,
						"observation_version": "open-redirect-candidate-v1",
					}),
				}

				if err := db.Create(&observation).Error; err != nil {
					return nil, err
				}
				createdObservationIDs = append(createdObservationIDs, observation.ID)
			}
		}

		memoryItemID, memoryWasCreated, memoryErr := persistOpenRedirectSkillMemory(uid, ownerKey, target, run, candidates, createdObservationIDs, len(foundURLs))

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("Open redirect planning reviewed %d URL(s), %d parameter observation(s), and found %d candidate(s).", len(foundURLs), len(parameterObservations), len(candidates))
		nextStep := "Use controlled runtime for redirect behavior validation only when scope and policy allow."
		if len(candidates) == 0 {
			nextStep = "Improve URL discovery, parameter inventory, login/logout flow mapping, OAuth/callback discovery, or redirect-like evidence before controlled redirect validation."
		}

		output := map[string]interface{}{
			"status":                      status,
			"skill":                       openRedirectSkillSlug,
			"runtime_version":             "open-redirect-skill-runtime-v1",
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"memory_ingested":             memoryErr == nil && memoryItemID > 0,
			"memory_item_id":              memoryItemID,
			"memory_was_created":          memoryWasCreated,
			"runtime_scope":               "planning_only_no_redirect_validation",
			"next_recommended_skills": []string{
				"open_redirect_probe",
				"http_evidence_analysis",
			},
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":                run.ID,
			"status":                      status,
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"memory_ingested":             memoryErr == nil && memoryItemID > 0,
			"memory_item_id":              memoryItemID,
			"memory_was_created":          memoryWasCreated,
			"runtime_scope":               "planning_only_no_redirect_validation",
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	return map[string]interface{}{
		"status":          models.OperatorSkillRunStatusCompleted,
		"runs":            runSummaries,
		"run_count":       len(runSummaries),
		"candidate_count": len(candidates),
		"runtime_scope":   "planning_only_no_redirect_validation",
	}, nil
}

func executeXSSReflectionSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, xssReflectionSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	parameterObservations := make([]models.OperatorSkillObservation, 0)
	_ = db.
		Where("target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ? AND observation_type = ?",
			target.ID,
			uid,
			ownerKey,
			parameterInventorySkillSlug,
			models.OperatorSkillObservationTypeCandidate,
		).
		Order("created_at DESC, id DESC").
		Limit(200).
		Find(&parameterObservations).Error

	candidates := buildXSSReflectionCandidates(foundURLs, parameterObservations)
	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0, len(candidates))

		if len(candidates) == 0 {
			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       xssReflectionSkillSlug,
				ObservationType: models.OperatorSkillObservationTypeLearning,
				Title:           "XSS reflection planning needs parameter candidates",
				Summary:         "No XSS/reflection parameter candidates were found from current inventory.",
				Content:         "Learning: improve crawling, parameter inventory, JS/API route mining, or reflection evidence before controlled XSS validation. Runtime v1 does not execute payloads.",
				BugClass:        "xss",
				Confidence:      60,
				Severity:        models.FindingSeverityInfo,
				Status:          models.OperatorSkillRunStatusCompleted,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"sampled_url_count":           len(foundURLs),
					"parameter_observation_count": len(parameterObservations),
					"candidate_count":             0,
					"skill":                       xssReflectionSkillSlug,
					"runtime_scope":               "planning_only_no_payload_execution",
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "xss-reflection-skill-v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "xss-reflection-learning-v1",
				}),
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		} else {
			for _, candidate := range candidates {
				observation := models.OperatorSkillObservation{
					UserID:          uid,
					OwnerKey:        ownerKey,
					TargetID:        target.ID,
					SkillRunID:      run.ID,
					SkillSlug:       xssReflectionSkillSlug,
					ObservationType: models.OperatorSkillObservationTypeCandidate,
					Title:           fmt.Sprintf("XSS reflection candidate: %s", candidate.ParamName),
					Summary:         fmt.Sprintf("Parameter %q is a candidate for controlled reflection/context validation.", candidate.ParamName),
					Content:         candidate.Reason + "\nRuntime v1 does not execute payloads.",
					URL:             candidate.URL,
					ParamName:       candidate.ParamName,
					BugClass:        "xss",
					Confidence:      candidate.Confidence,
					Severity:        models.FindingSeverityInfo,
					Status:          "candidate",
					EvidenceJSON: chatJSON(map[string]interface{}{
						"parameter":     candidate.ParamName,
						"url":           candidate.URL,
						"source":        candidate.Source,
						"reason":        candidate.Reason,
						"confidence":    candidate.Confidence,
						"skill":         xssReflectionSkillSlug,
						"runtime_scope": "planning_only_no_payload_execution",
					}),
					Metadata: chatJSON(map[string]interface{}{
						"created_by":          "xss-reflection-skill-v1",
						"operator_skill_run":  run.ID,
						"target_id":           target.ID,
						"observation_version": "xss-reflection-candidate-v1",
					}),
				}

				if err := db.Create(&observation).Error; err != nil {
					return nil, err
				}
				createdObservationIDs = append(createdObservationIDs, observation.ID)
			}
		}

		memoryItemID, memoryWasCreated, memoryErr := persistXSSReflectionSkillMemory(uid, ownerKey, target, run, candidates, createdObservationIDs, len(foundURLs))

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("XSS reflection planning reviewed %d URL(s), %d parameter observation(s), and found %d candidate(s).", len(foundURLs), len(parameterObservations), len(candidates))
		nextStep := "Use controlled runtime for reflection/context probing only when scope and policy allow."
		if len(candidates) == 0 {
			nextStep = "Improve URL discovery, parameter inventory, JS/API route mining, or reflection evidence before controlled XSS validation."
		}

		output := map[string]interface{}{
			"status":                      status,
			"skill":                       xssReflectionSkillSlug,
			"runtime_version":             "xss-reflection-skill-runtime-v1",
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"memory_ingested":             memoryErr == nil && memoryItemID > 0,
			"memory_item_id":              memoryItemID,
			"memory_was_created":          memoryWasCreated,
			"runtime_scope":               "planning_only_no_payload_execution",
			"next_recommended_skills": []string{
				"xss_reflection_probe",
				"dom_xss_audit",
				"http_evidence_analysis",
			},
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":                run.ID,
			"status":                      status,
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"memory_ingested":             memoryErr == nil && memoryItemID > 0,
			"memory_item_id":              memoryItemID,
			"memory_was_created":          memoryWasCreated,
			"runtime_scope":               "planning_only_no_payload_execution",
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	return map[string]interface{}{
		"status":          models.OperatorSkillRunStatusCompleted,
		"runs":            runSummaries,
		"run_count":       len(runSummaries),
		"candidate_count": len(candidates),
		"runtime_scope":   "planning_only_no_payload_execution",
	}, nil
}

func executeJSAuditSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, jsAuditSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	candidates := buildJSAuditCandidates(foundURLs)
	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0, len(candidates))

		if len(candidates) == 0 {
			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       jsAuditSkillSlug,
				ObservationType: models.OperatorSkillObservationTypeLearning,
				Title:           "JS audit needs URL/JS inventory",
				Summary:         "No JavaScript/API-like candidates were found in the sampled URL inventory.",
				Content:         "Learning: run or improve crawling, JS collection, browser-aware discovery, or JS intelligence before client-side/API route validation.",
				BugClass:        "js_review",
				Confidence:      65,
				Severity:        models.FindingSeverityInfo,
				Status:          models.OperatorSkillRunStatusCompleted,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"sampled_url_count": len(foundURLs),
					"candidate_count":   0,
					"skill":             jsAuditSkillSlug,
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "js-audit-skill-v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "js-audit-learning-v1",
				}),
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		} else {
			for _, candidate := range candidates {
				observation := models.OperatorSkillObservation{
					UserID:          uid,
					OwnerKey:        ownerKey,
					TargetID:        target.ID,
					SkillRunID:      run.ID,
					SkillSlug:       jsAuditSkillSlug,
					ObservationType: models.OperatorSkillObservationTypeCandidate,
					Title:           fmt.Sprintf("JS/API candidate: %s", candidate.Kind),
					Summary:         fmt.Sprintf("%s candidate discovered for JS/API/client-side analysis: %s", candidate.Kind, candidate.URL),
					Content:         candidate.Reason,
					URL:             candidate.URL,
					BugClass:        candidate.BugClasses[0],
					Confidence:      candidate.Score,
					Severity:        models.FindingSeverityInfo,
					Status:          "candidate",
					EvidenceJSON: chatJSON(map[string]interface{}{
						"url":                   candidate.URL,
						"kind":                  candidate.Kind,
						"candidate_bug_classes": candidate.BugClasses,
						"reason":                candidate.Reason,
						"source":                candidate.Source,
						"score":                 candidate.Score,
						"skill":                 jsAuditSkillSlug,
					}),
					Metadata: chatJSON(map[string]interface{}{
						"created_by":          "js-audit-skill-v1",
						"operator_skill_run":  run.ID,
						"target_id":           target.ID,
						"observation_version": "js-audit-candidate-v1",
					}),
				}

				if err := db.Create(&observation).Error; err != nil {
					return nil, err
				}
				createdObservationIDs = append(createdObservationIDs, observation.ID)
			}
		}

		memoryItemID, memoryWasCreated, memoryErr := persistJSAuditSkillMemory(uid, ownerKey, target, run, candidates, createdObservationIDs, len(foundURLs))

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("JS audit reviewed %d sampled URL(s) and found %d JS/API-like candidate(s).", len(foundURLs), len(candidates))
		nextStep := "Use JS/API candidates for endpoint mining, DOM sink review, secrets review, and authorized API/access-control validation planning."
		if len(candidates) == 0 {
			nextStep = "Improve crawling, JS collection, browser-aware discovery, or JS intelligence before relying on JS audit results."
		}

		jsAssetCount := 0
		sourceMapCount := 0
		apiLikeCount := 0
		frontendRouteCount := 0
		for _, candidate := range candidates {
			switch candidate.Kind {
			case "javascript_asset":
				jsAssetCount++
			case "sourcemap_candidate":
				sourceMapCount++
			case "api_like_route":
				apiLikeCount++
			case "frontend_route":
				frontendRouteCount++
			}
		}

		output := map[string]interface{}{
			"status":               status,
			"skill":                jsAuditSkillSlug,
			"runtime_version":      "js-audit-skill-runtime-v1",
			"sampled_url_count":    len(foundURLs),
			"candidate_count":      len(candidates),
			"js_asset_count":       jsAssetCount,
			"sourcemap_count":      sourceMapCount,
			"api_like_route_count": apiLikeCount,
			"frontend_route_count": frontendRouteCount,
			"observation_count":    len(createdObservationIDs),
			"observation_ids":      createdObservationIDs,
			"memory_ingested":      memoryErr == nil && memoryItemID > 0,
			"memory_item_id":       memoryItemID,
			"memory_was_created":   memoryWasCreated,
			"next_recommended_skills": []string{
				"dom_xss_audit",
				"parameter_inventory",
				"auth_context_needed",
				"http_evidence_analysis",
			},
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":         run.ID,
			"status":               status,
			"sampled_url_count":    len(foundURLs),
			"candidate_count":      len(candidates),
			"js_asset_count":       jsAssetCount,
			"sourcemap_count":      sourceMapCount,
			"api_like_route_count": apiLikeCount,
			"frontend_route_count": frontendRouteCount,
			"observation_count":    len(createdObservationIDs),
			"observation_ids":      createdObservationIDs,
			"memory_ingested":      memoryErr == nil && memoryItemID > 0,
			"memory_item_id":       memoryItemID,
			"memory_was_created":   memoryWasCreated,
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	totalCandidates := len(candidates)
	return map[string]interface{}{
		"status":          models.OperatorSkillRunStatusCompleted,
		"runs":            runSummaries,
		"run_count":       len(runSummaries),
		"candidate_count": totalCandidates,
	}, nil
}

func executeAuthContextNeededSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, authContextNeededSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	runSummaries := make([]map[string]interface{}, 0, len(runs))
	now := time.Now().UTC()

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		content, summary := authContextNeededContent(target)
		observation := models.OperatorSkillObservation{
			UserID:          uid,
			OwnerKey:        ownerKey,
			TargetID:        target.ID,
			SkillRunID:      run.ID,
			SkillSlug:       authContextNeededSkillSlug,
			ObservationType: models.OperatorSkillObservationTypeNextStep,
			Title:           "Authenticated context required for access-control validation",
			Summary:         summary,
			Content:         content,
			BugClass:        "access_control",
			Confidence:      90,
			Severity:        models.FindingSeverityInfo,
			Status:          models.OperatorSkillRunStatusNeedsContext,
			EvidenceJSON: chatJSON(map[string]interface{}{
				"required_context": []string{
					"authenticated session or test credentials",
					"second test account where possible",
					"role/tenant/org boundaries",
					"authorization for account-to-account checks",
					"target workflows and sensitive object identifiers",
					"rate limits and stop conditions",
				},
				"skill": authContextNeededSkillSlug,
			}),
			Metadata: chatJSON(map[string]interface{}{
				"created_by":          "auth-context-needed-skill-v1",
				"operator_skill_run":  run.ID,
				"target_id":           target.ID,
				"observation_version": "auth-context-needed-observation-v1",
			}),
		}

		if err := db.Create(&observation).Error; err != nil {
			return nil, err
		}

		observationIDs := []uint{observation.ID}
		memoryItemID, memoryWasCreated, memoryErr := persistAuthContextNeededSkillMemory(uid, ownerKey, target, run, observationIDs)

		status := models.OperatorSkillRunStatusNeedsContext
		resultSummary := "Auth context is required before real access-control, IDOR/BOLA, API authorization, session, or business-logic validation."
		nextStep := "Ask the user for an authenticated session or test credentials, preferably two accounts with different ownership boundaries, plus explicit authorization and stop conditions."

		output := map[string]interface{}{
			"status":            status,
			"skill":             authContextNeededSkillSlug,
			"observation_count": 1,
			"observation_ids":   observationIDs,
			"runtime_version":   "auth-context-needed-skill-runtime-v1",
			"needs_context":     true,
			"required_context": []string{
				"authenticated session or test credentials",
				"second test account where possible",
				"role/tenant/org boundaries",
				"authorization for account-to-account checks",
				"target workflows and sensitive object identifiers",
				"rate limits and stop conditions",
			},
			"memory_ingested":    memoryErr == nil && memoryItemID > 0,
			"memory_item_id":     memoryItemID,
			"memory_was_created": memoryWasCreated,
			"next_recommended_skills": []string{
				"idor_authz_probe",
				"api_authorization",
				"auth_session_analysis",
			},
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":       run.ID,
			"status":             status,
			"needs_context":      true,
			"observation_count":  1,
			"observation_ids":    observationIDs,
			"memory_ingested":    memoryErr == nil && memoryItemID > 0,
			"memory_item_id":     memoryItemID,
			"memory_was_created": memoryWasCreated,
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	return map[string]interface{}{
		"status":        models.OperatorSkillRunStatusNeedsContext,
		"runs":          runSummaries,
		"run_count":     len(runSummaries),
		"needs_context": true,
	}, nil
}

func executeSelectedOperatorSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	output := map[string]interface{}{}

	if target == nil || len(selectedSkillRunIDs) == 0 {
		output["status"] = "no_selected_skill_runs"
		output["executed_count"] = 0
		output["dispatcher_version"] = "operator-skill-runtime-dispatcher-v1"
		return output, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ?", selectedSkillRunIDs, target.ID, uid, ownerKey).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return output, err
	}

	selectedBySlug := map[string][]uint{}
	for _, run := range runs {
		slug := strings.ToLower(strings.TrimSpace(run.SkillSlug))
		if slug == "" {
			continue
		}
		selectedBySlug[slug] = append(selectedBySlug[slug], run.ID)
	}

	executedCount := 0
	plannedCount := 0
	blockedCount := 0

	if runIDs := selectedBySlug[parameterInventorySkillSlug]; len(runIDs) > 0 {
		parameterOutput, err := executeParameterInventorySkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["parameter_inventory_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if parameterOutput != nil {
			output["parameter_inventory"] = parameterOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[httpEvidenceAnalysisSkillSlug]; len(runIDs) > 0 {
		httpEvidenceOutput, err := executeHTTPEvidenceAnalysisSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["http_evidence_analysis_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if httpEvidenceOutput != nil {
			output["http_evidence_analysis"] = httpEvidenceOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[authContextNeededSkillSlug]; len(runIDs) > 0 {
		authContextOutput, err := executeAuthContextNeededSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["auth_context_needed_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if authContextOutput != nil {
			output["auth_context_needed"] = authContextOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[jsAuditSkillSlug]; len(runIDs) > 0 {
		jsAuditOutput, err := executeJSAuditSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["js_audit_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if jsAuditOutput != nil {
			output["js_audit"] = jsAuditOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[xssReflectionSkillSlug]; len(runIDs) > 0 {
		xssOutput, err := executeXSSReflectionSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["xss_reflection_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if xssOutput != nil {
			output["xss_reflection"] = xssOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[openRedirectSkillSlug]; len(runIDs) > 0 {
		openRedirectOutput, err := executeOpenRedirectSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["open_redirect_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if openRedirectOutput != nil {
			output["open_redirect"] = openRedirectOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[pathTraversalBaselineSkillSlug]; len(runIDs) > 0 {
		pathTraversalOutput, err := executePathTraversalSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["path_traversal_baseline_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if pathTraversalOutput != nil {
			output["path_traversal_baseline"] = pathTraversalOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[xssReflectionContextSkillSlug]; len(runIDs) > 0 {
		xssContextOutput, err := executeXSSReflectionContextSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["xss_reflection_context_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if xssContextOutput != nil {
			output["xss_reflection_context"] = xssContextOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[openRedirectChainSkillSlug]; len(runIDs) > 0 {
		openRedirectChainOutput, err := executeOpenRedirectChainSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["open_redirect_chain_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if openRedirectChainOutput != nil {
			output["open_redirect_chain"] = openRedirectChainOutput
			executedCount += len(runIDs)
		}
	}

	if runIDs := selectedBySlug[pathTraversalFileReadBaselineSlug]; len(runIDs) > 0 {
		pathFileReadOutput, err := executePathTraversalFileReadBaselineSkillRunsFromChat(db, uid, ownerKey, target, runIDs)
		if err != nil {
			output["path_traversal_file_read_baseline_error"] = err.Error()
			blockedCount += len(runIDs)
		} else if pathFileReadOutput != nil {
			output["path_traversal_file_read_baseline"] = pathFileReadOutput
			executedCount += len(runIDs)
		}
	}

	notImplemented := make([]map[string]interface{}, 0)
	for slug, runIDs := range selectedBySlug {
		if slug == parameterInventorySkillSlug || slug == httpEvidenceAnalysisSkillSlug || slug == authContextNeededSkillSlug || slug == jsAuditSkillSlug || slug == xssReflectionSkillSlug || slug == openRedirectSkillSlug || slug == pathTraversalBaselineSkillSlug || slug == xssReflectionContextSkillSlug || slug == openRedirectChainSkillSlug || slug == pathTraversalFileReadBaselineSlug {
			continue
		}

		plannedCount += len(runIDs)
		notImplemented = append(notImplemented, map[string]interface{}{
			"skill_slug": slug,
			"run_ids":    runIDs,
			"status":     "planned",
			"reason":     "Skill runtime is registered but not implemented in dispatcher v1.",
		})
	}

	output["status"] = "completed"
	output["dispatcher_version"] = "operator-skill-runtime-dispatcher-v1"
	output["selected_run_count"] = len(runs)
	output["executed_count"] = executedCount
	output["planned_count"] = plannedCount
	output["blocked_count"] = blockedCount
	output["not_implemented"] = notImplemented

	return output, nil
}

func controlledValidationMarker(prefix string, runID uint) string {
	base := strings.TrimSpace(prefix)
	if base == "" {
		base = "hunt"
	}
	return fmt.Sprintf("%s_v3151_%d", base, runID)
}

type controlledHTTPProbeResult struct {
	URL          string
	StatusCode   int
	Blocked      bool
	Inconclusive bool
	Reflected    bool
	ContentType  string
	CSP          string
	Location     string
	BodySample   string
	HeaderSample map[string]string
	Error        string
}

func controlledGETProbe(rawURL string, timeout time.Duration) controlledHTTPProbeResult {
	result := controlledHTTPProbeResult{
		URL:          rawURL,
		HeaderSample: map[string]string{},
	}

	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		result.Error = err.Error()
		result.Inconclusive = true
		return result
	}

	req.Header.Set("User-Agent", "HuntEngine-Authorized-Validation/3.15.1")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/json,text/plain,*/*;q=0.8")

	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		result.Error = err.Error()
		result.Inconclusive = true
		return result
	}
	defer resp.Body.Close()

	result.StatusCode = resp.StatusCode
	result.ContentType = resp.Header.Get("Content-Type")
	result.CSP = resp.Header.Get("Content-Security-Policy")
	result.Location = resp.Header.Get("Location")

	for _, name := range []string{
		"Content-Type",
		"Content-Security-Policy",
		"X-Content-Type-Options",
		"X-Frame-Options",
		"Location",
		"Cache-Control",
		"Vary",
		"Set-Cookie",
		"Server",
		"CF-Cache-Status",
		"X-Cache",
	} {
		if value := strings.TrimSpace(resp.Header.Get(name)); value != "" {
			result.HeaderSample[name] = value
		}
	}

	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		result.Blocked = true
		result.Inconclusive = true
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
	result.BodySample = string(body)

	return result
}

func buildProbeURLWithParam(rawURL string, paramName string, marker string) (string, bool) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil || parsed == nil {
		return "", false
	}

	paramName = strings.TrimSpace(paramName)
	if paramName == "" {
		return "", false
	}

	q := parsed.Query()
	if _, ok := q[paramName]; !ok {
		return "", false
	}
	q.Set(paramName, marker)
	parsed.RawQuery = q.Encode()

	return parsed.String(), true
}

func roughReflectionContext(body string, marker string, contentType string) string {
	if marker == "" || body == "" {
		return "none"
	}

	lowerContentType := strings.ToLower(contentType)
	lowerBody := strings.ToLower(body)
	idx := strings.Index(body, marker)
	if idx < 0 {
		return "none"
	}

	if strings.Contains(lowerContentType, "application/json") {
		return "json"
	}
	if strings.Contains(lowerContentType, "javascript") {
		return "javascript"
	}
	if strings.Contains(lowerContentType, "text/plain") {
		return "text"
	}

	start := idx - 80
	if start < 0 {
		start = 0
	}
	end := idx + len(marker) + 80
	if end > len(body) {
		end = len(body)
	}
	window := strings.ToLower(body[start:end])

	switch {
	case strings.Contains(window, "<script"):
		return "script"
	case strings.Contains(window, "href=\"") || strings.Contains(window, "src=\"") || strings.Contains(window, "value=\"") || strings.Contains(window, "='") || strings.Contains(window, "=\""):
		return "html_attribute"
	case strings.Contains(lowerBody, "<html") || strings.Contains(lowerBody, "<body") || strings.Contains(lowerBody, "<div") || strings.Contains(lowerBody, "<span"):
		return "html_text"
	default:
		return "unknown_reflection"
	}
}

func executeXSSReflectionContextSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, xssReflectionContextSkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	parameterObservations := make([]models.OperatorSkillObservation, 0)
	_ = db.
		Where("target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ? AND observation_type = ?",
			target.ID,
			uid,
			ownerKey,
			parameterInventorySkillSlug,
			models.OperatorSkillObservationTypeCandidate,
		).
		Order("created_at DESC, id DESC").
		Limit(200).
		Find(&parameterObservations).Error

	candidates := buildXSSReflectionCandidates(foundURLs, parameterObservations)
	if len(candidates) > 10 {
		candidates = candidates[:10]
	}

	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		marker := controlledValidationMarker("hunt_xss_reflection_context", run.ID)
		createdObservationIDs := make([]uint, 0)
		probeResults := make([]map[string]interface{}, 0)
		reflectionCount := 0
		blockedCount := 0
		inconclusiveCount := 0

		if len(candidates) == 0 {
			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       xssReflectionContextSkillSlug,
				ObservationType: models.OperatorSkillObservationTypeLearning,
				Title:           "XSS reflection context validation needs candidates",
				Summary:         "No XSS/reflection candidates were available for controlled marker probing.",
				Content:         "Learning: improve crawling, parameter inventory, JS/API route mining, or reflection evidence before controlled XSS context validation.",
				BugClass:        "xss",
				Confidence:      60,
				Severity:        models.FindingSeverityInfo,
				Status:          models.OperatorSkillRunStatusCompleted,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"sampled_url_count":           len(foundURLs),
					"parameter_observation_count": len(parameterObservations),
					"candidate_count":             0,
					"skill":                       xssReflectionContextSkillSlug,
					"runtime_scope":               "controlled_marker_reflection_probe_no_exploit_payload",
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "xss-reflection-context-runtime-v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "xss-reflection-context-learning-v1",
				}),
			}
			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		} else {
			for _, candidate := range candidates {
				probeURL, ok := buildProbeURLWithParam(candidate.URL, candidate.ParamName, marker)
				if !ok {
					continue
				}

				probe := controlledGETProbe(probeURL, 8*time.Second)
				probe.Reflected = strings.Contains(probe.BodySample, marker)

				contextClass := roughReflectionContext(probe.BodySample, marker, probe.ContentType)
				if probe.Reflected {
					reflectionCount++
				}
				if probe.Blocked {
					blockedCount++
				}
				if probe.Inconclusive {
					inconclusiveCount++
				}

				status := "candidate"
				observationType := models.OperatorSkillObservationTypeCandidate
				severity := models.FindingSeverityInfo
				confidence := candidate.Confidence
				if confidence <= 0 {
					confidence = 65
				}

				if probe.Reflected && !probe.Inconclusive {
					status = "evidence"
					observationType = models.OperatorSkillObservationTypeEvidence
					severity = models.FindingSeverityLow
					confidence += 10
					if confidence > 90 {
						confidence = 90
					}
				}

				if probe.Inconclusive {
					status = "inconclusive"
					observationType = models.OperatorSkillObservationTypeLearning
					if confidence > 65 {
						confidence = 65
					}
				}

				title := fmt.Sprintf("XSS reflection context probe: %s", candidate.ParamName)
				summary := fmt.Sprintf("Controlled marker probe checked parameter %q for reflection context.", candidate.ParamName)
				if probe.Reflected {
					summary = fmt.Sprintf("Controlled marker reflected for parameter %q in %s context.", candidate.ParamName, contextClass)
				}
				if probe.Inconclusive {
					summary = fmt.Sprintf("Controlled marker probe for parameter %q was inconclusive or blocked.", candidate.ParamName)
				}

				observation := models.OperatorSkillObservation{
					UserID:          uid,
					OwnerKey:        ownerKey,
					TargetID:        target.ID,
					SkillRunID:      run.ID,
					SkillSlug:       xssReflectionContextSkillSlug,
					ObservationType: observationType,
					Title:           title,
					Summary:         summary,
					Content:         "Runtime scope: controlled_marker_reflection_probe_no_exploit_payload. This probe uses an inert marker value and does not execute browser or exploit payloads.",
					URL:             probeURL,
					ParamName:       candidate.ParamName,
					BugClass:        "xss",
					Confidence:      confidence,
					Severity:        severity,
					Status:          status,
					EvidenceJSON: chatJSON(map[string]interface{}{
						"original_url":     candidate.URL,
						"probe_url":        probeURL,
						"parameter":        candidate.ParamName,
						"marker":           marker,
						"status_code":      probe.StatusCode,
						"blocked":          probe.Blocked,
						"inconclusive":     probe.Inconclusive,
						"reflected":        probe.Reflected,
						"context":          contextClass,
						"content_type":     probe.ContentType,
						"csp":              probe.CSP,
						"location":         probe.Location,
						"header_sample":    probe.HeaderSample,
						"error":            probe.Error,
						"runtime_scope":    "controlled_marker_reflection_probe_no_exploit_payload",
						"execution_level":  2,
						"destructive":      false,
						"state_changing":   false,
						"browser_executed": false,
					}),
					Metadata: chatJSON(map[string]interface{}{
						"created_by":          "xss-reflection-context-runtime-v1",
						"operator_skill_run":  run.ID,
						"target_id":           target.ID,
						"observation_version": "xss-reflection-context-probe-v1",
					}),
				}

				if err := db.Create(&observation).Error; err != nil {
					return nil, err
				}
				createdObservationIDs = append(createdObservationIDs, observation.ID)

				probeResults = append(probeResults, map[string]interface{}{
					"param_name":   candidate.ParamName,
					"original_url": candidate.URL,
					"probe_url":    probeURL,
					"status_code":  probe.StatusCode,
					"blocked":      probe.Blocked,
					"inconclusive": probe.Inconclusive,
					"reflected":    probe.Reflected,
					"context":      contextClass,
					"content_type": probe.ContentType,
					"csp_present":  strings.TrimSpace(probe.CSP) != "",
				})
			}
		}

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("XSS reflection context validation probed %d candidate(s), found %d reflection(s), %d blocked/inconclusive result(s).", len(probeResults), reflectionCount, blockedCount+inconclusiveCount)
		nextStep := "If marker reflection exists, use approved browser/context-aware validation later; do not promote exploitability without payload/context proof."

		output := map[string]interface{}{
			"status":                      status,
			"skill":                       xssReflectionContextSkillSlug,
			"runtime_version":             "xss-reflection-context-runtime-v1",
			"runtime_scope":               "controlled_marker_reflection_probe_no_exploit_payload",
			"execution_level":             2,
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"probe_count":                 len(probeResults),
			"reflection_count":            reflectionCount,
			"blocked_count":               blockedCount,
			"inconclusive_count":          inconclusiveCount,
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"probe_results":               probeResults,
			"memory_ingested":             false,
			"memory_scope":                "operator_skill_observations_only",
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":       run.ID,
			"status":             status,
			"candidate_count":    len(candidates),
			"probe_count":        len(probeResults),
			"reflection_count":   reflectionCount,
			"blocked_count":      blockedCount,
			"inconclusive_count": inconclusiveCount,
			"observation_count":  len(createdObservationIDs),
			"observation_ids":    createdObservationIDs,
			"runtime_scope":      "controlled_marker_reflection_probe_no_exploit_payload",
		})
	}

	return map[string]interface{}{
		"status":           models.OperatorSkillRunStatusCompleted,
		"runs":             runSummaries,
		"run_count":        len(runSummaries),
		"runtime_scope":    "controlled_marker_reflection_probe_no_exploit_payload",
		"execution_level":  2,
		"validation_class": "xss_reflection_context",
	}, nil
}

func executeOpenRedirectChainSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	return executeCandidateClassificationAliasSkillRunsFromChat(
		db,
		uid,
		ownerKey,
		target,
		selectedSkillRunIDs,
		openRedirectChainSkillSlug,
		"open_redirect",
		"open-redirect-chain-runtime-v1",
		"chain_candidate_classification_no_external_redirect_validation",
		"Open redirect chain",
		"Parameter is a candidate for chain-aware redirect validation planning. No external redirect validation is executed in this runtime.",
		"Use controlled redirect behavior validation only when scope and policy allow.",
		func(foundURLs []models.FoundURL, parameterObservations []models.OperatorSkillObservation) []map[string]interface{} {
			candidates := buildOpenRedirectCandidates(foundURLs, parameterObservations)
			out := make([]map[string]interface{}, 0, len(candidates))
			for _, candidate := range candidates {
				out = append(out, map[string]interface{}{
					"param_name": candidate.ParamName,
					"url":        candidate.URL,
					"source":     candidate.Source,
					"reason":     candidate.Reason,
					"confidence": candidate.Confidence,
				})
			}
			return out
		},
	)
}

func executePathTraversalFileReadBaselineSkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	return executeCandidateClassificationAliasSkillRunsFromChat(
		db,
		uid,
		ownerKey,
		target,
		selectedSkillRunIDs,
		pathTraversalFileReadBaselineSlug,
		"path_traversal_file_read",
		"path-traversal-file-read-baseline-runtime-v1",
		"candidate_classification_no_file_read_payload_execution",
		"Path traversal/file-read baseline",
		"Parameter is a candidate for controlled file-read/path traversal baseline validation planning. No file-read payload is executed in this runtime.",
		"Use controlled file-read/path traversal validation only when scope and policy allow.",
		func(foundURLs []models.FoundURL, parameterObservations []models.OperatorSkillObservation) []map[string]interface{} {
			candidates := buildPathTraversalCandidates(foundURLs, parameterObservations)
			out := make([]map[string]interface{}, 0, len(candidates))
			for _, candidate := range candidates {
				out = append(out, map[string]interface{}{
					"param_name": candidate.ParamName,
					"url":        candidate.URL,
					"source":     candidate.Source,
					"reason":     candidate.Reason,
					"confidence": candidate.Confidence,
				})
			}
			return out
		},
	)
}

func executeCandidateClassificationAliasSkillRunsFromChat(
	db *gorm.DB,
	uid uint,
	ownerKey string,
	target *models.Target,
	selectedSkillRunIDs []uint,
	skillSlug string,
	bugClass string,
	runtimeVersion string,
	runtimeScope string,
	titlePrefix string,
	candidateSummary string,
	nextStep string,
	buildCandidates func([]models.FoundURL, []models.OperatorSkillObservation) []map[string]interface{},
) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, skillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	parameterObservations := make([]models.OperatorSkillObservation, 0)
	_ = db.
		Where("target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ? AND observation_type = ?",
			target.ID,
			uid,
			ownerKey,
			parameterInventorySkillSlug,
			models.OperatorSkillObservationTypeCandidate,
		).
		Order("created_at DESC, id DESC").
		Limit(200).
		Find(&parameterObservations).Error

	candidates := buildCandidates(foundURLs, parameterObservations)
	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0, len(candidates))

		if len(candidates) == 0 {
			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       skillSlug,
				ObservationType: models.OperatorSkillObservationTypeLearning,
				Title:           titlePrefix + " needs candidate evidence",
				Summary:         "No matching parameter candidates were found from current inventory.",
				Content:         "Learning: improve crawling, parameter inventory, JS/API route mining, workflow mapping, or baseline HTTP evidence before this validation path can be meaningful. Runtime scope: " + runtimeScope + ".",
				BugClass:        bugClass,
				Confidence:      60,
				Severity:        models.FindingSeverityInfo,
				Status:          models.OperatorSkillRunStatusCompleted,
				EvidenceJSON: chatJSON(map[string]interface{}{
					"sampled_url_count":           len(foundURLs),
					"parameter_observation_count": len(parameterObservations),
					"candidate_count":             0,
					"skill":                       skillSlug,
					"runtime_scope":               runtimeScope,
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          runtimeVersion,
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": runtimeVersion + "-learning",
				}),
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		} else {
			for _, candidate := range candidates {
				paramName, _ := candidate["param_name"].(string)
				rawURL, _ := candidate["url"].(string)
				source, _ := candidate["source"].(string)
				reason, _ := candidate["reason"].(string)
				confidence := 65
				if rawConfidence, ok := candidate["confidence"].(int); ok && rawConfidence > 0 {
					confidence = rawConfidence
				}

				observation := models.OperatorSkillObservation{
					UserID:          uid,
					OwnerKey:        ownerKey,
					TargetID:        target.ID,
					SkillRunID:      run.ID,
					SkillSlug:       skillSlug,
					ObservationType: models.OperatorSkillObservationTypeCandidate,
					Title:           fmt.Sprintf("%s candidate: %s", titlePrefix, paramName),
					Summary:         candidateSummary,
					Content:         reason + "\nRuntime scope: " + runtimeScope + ".",
					URL:             rawURL,
					ParamName:       paramName,
					BugClass:        bugClass,
					Confidence:      confidence,
					Severity:        models.FindingSeverityInfo,
					Status:          "candidate",
					EvidenceJSON: chatJSON(map[string]interface{}{
						"parameter":     paramName,
						"url":           rawURL,
						"source":        source,
						"reason":        reason,
						"confidence":    confidence,
						"skill":         skillSlug,
						"runtime_scope": runtimeScope,
					}),
					Metadata: chatJSON(map[string]interface{}{
						"created_by":          runtimeVersion,
						"operator_skill_run":  run.ID,
						"target_id":           target.ID,
						"observation_version": runtimeVersion + "-candidate",
					}),
				}

				if err := db.Create(&observation).Error; err != nil {
					return nil, err
				}
				createdObservationIDs = append(createdObservationIDs, observation.ID)
			}
		}

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("%s reviewed %d URL(s), %d parameter observation(s), and found %d candidate(s).", titlePrefix, len(foundURLs), len(parameterObservations), len(candidates))

		output := map[string]interface{}{
			"status":                      status,
			"skill":                       skillSlug,
			"runtime_version":             runtimeVersion,
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"runtime_scope":               runtimeScope,
			"memory_ingested":             false,
			"memory_scope":                "operator_skill_observations_only",
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":                run.ID,
			"status":                      status,
			"sampled_url_count":           len(foundURLs),
			"parameter_observation_count": len(parameterObservations),
			"candidate_count":             len(candidates),
			"observation_count":           len(createdObservationIDs),
			"observation_ids":             createdObservationIDs,
			"runtime_scope":               runtimeScope,
		})
	}

	return map[string]interface{}{
		"status":          models.OperatorSkillRunStatusCompleted,
		"runs":            runSummaries,
		"run_count":       len(runSummaries),
		"candidate_count": len(candidates),
		"runtime_scope":   runtimeScope,
	}, nil
}

func executeParameterInventorySkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND owner_key = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, ownerKey, parameterInventorySkillSlug).
		Order("id ASC").
		Find(&runs).Error; err != nil {
		return nil, err
	}

	if len(runs) == 0 {
		return nil, nil
	}

	foundURLs := make([]models.FoundURL, 0)
	if err := db.
		Where("target_id = ?", target.ID).
		Order("last_seen DESC NULLS LAST, id DESC").
		Limit(1000).
		Find(&foundURLs).Error; err != nil {
		return nil, err
	}

	candidates := buildParameterInventoryCandidates(foundURLs)
	now := time.Now().UTC()
	runSummaries := make([]map[string]interface{}, 0, len(runs))

	for _, run := range runs {
		startedAt := now
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":     models.OperatorSkillRunStatusRunning,
				"started_at": &startedAt,
				"updated_at": now,
			}).Error; err != nil {
			return nil, err
		}

		createdObservationIDs := make([]uint, 0, len(candidates))
		for _, candidate := range candidates {
			primaryBugClass := primaryParameterBugClass(candidate.BugClasses)

			exampleURL := ""
			if len(candidate.URLs) > 0 {
				exampleURL = candidate.URLs[0]
			}

			observation := models.OperatorSkillObservation{
				UserID:          uid,
				OwnerKey:        ownerKey,
				TargetID:        target.ID,
				SkillRunID:      run.ID,
				SkillSlug:       parameterInventorySkillSlug,
				ObservationType: models.OperatorSkillObservationTypeCandidate,
				Title:           fmt.Sprintf("Parameter candidate: %s", candidate.Name),
				Summary:         fmt.Sprintf("Parameter %q appears in %d sampled URL(s) and maps to candidate bug classes: %s.", candidate.Name, len(candidate.URLs), strings.Join(candidate.BugClasses, ", ")),
				Content:         strings.Join(candidate.Reasons, "\n"),
				URL:             exampleURL,
				ParamName:       candidate.Name,
				BugClass:        primaryBugClass,
				Confidence:      candidate.Score,
				Severity:        models.FindingSeverityInfo,
				Status:          "candidate",
				EvidenceJSON: chatJSON(map[string]interface{}{
					"parameter":             candidate.Name,
					"candidate_bug_classes": candidate.BugClasses,
					"reasons":               candidate.Reasons,
					"sample_urls":           candidate.URLs,
					"sources":               candidate.Sources,
					"score":                 candidate.Score,
					"skill":                 parameterInventorySkillSlug,
				}),
				Metadata: chatJSON(map[string]interface{}{
					"created_by":          "parameter_inventory_skill_v1",
					"operator_skill_run":  run.ID,
					"target_id":           target.ID,
					"observation_version": "parameter-inventory-observation-v1",
				}),
			}

			if err := db.Create(&observation).Error; err != nil {
				return nil, err
			}
			createdObservationIDs = append(createdObservationIDs, observation.ID)
		}

		status := models.OperatorSkillRunStatusCompleted
		resultSummary := fmt.Sprintf("Parameter inventory completed: %d parameter candidate(s) extracted from %d sampled URL(s).", len(candidates), len(foundURLs))
		nextStep := "Use parameter observations to select class-specific validation skills such as xss_reflection, open_redirect, idor_bola, sqli/nosqli strategy, or path_traversal_baseline."

		memoryItemID, memoryWasCreated, memoryErr := persistParameterInventorySkillMemory(uid, ownerKey, target, run, len(foundURLs), candidates, createdObservationIDs)

		output := map[string]interface{}{
			"status":                  status,
			"skill":                   parameterInventorySkillSlug,
			"found_url_sample_count":  len(foundURLs),
			"parameter_count":         len(candidates),
			"observation_count":       len(createdObservationIDs),
			"observation_ids":         createdObservationIDs,
			"top_parameters":          candidates,
			"runtime_version":         "parameter-inventory-skill-runtime-v1",
			"next_recommended_skills": []string{"xss_reflection", "open_redirect", "path_traversal_baseline", "auth_context_needed"},
			"memory_ingested":         memoryErr == nil && memoryItemID > 0,
			"memory_item_id":          memoryItemID,
			"memory_was_created":      memoryWasCreated,
		}

		if memoryErr != nil {
			output["memory_ingest_error"] = memoryErr.Error()
		}

		completedAt := time.Now().UTC()
		if err := db.Model(&models.OperatorSkillRun{}).
			Where("id = ?", run.ID).
			Updates(map[string]interface{}{
				"status":         status,
				"output_json":    chatJSON(output),
				"result_summary": resultSummary,
				"next_step":      nextStep,
				"completed_at":   &completedAt,
				"updated_at":     completedAt,
			}).Error; err != nil {
			return nil, err
		}

		runSummaries = append(runSummaries, map[string]interface{}{
			"skill_run_id":       run.ID,
			"status":             status,
			"parameter_count":    len(candidates),
			"observation_count":  len(createdObservationIDs),
			"observation_ids":    createdObservationIDs,
			"top_parameters":     candidates,
			"memory_ingested":    memoryErr == nil && memoryItemID > 0,
			"memory_item_id":     memoryItemID,
			"memory_was_created": memoryWasCreated,
			"memory_ingest_error": func() string {
				if memoryErr != nil {
					return memoryErr.Error()
				}
				return ""
			}(),
		})
	}

	return map[string]interface{}{
		"status":           "completed",
		"runs":             runSummaries,
		"run_count":        len(runSummaries),
		"parameter_count":  len(candidates),
		"found_url_sample": len(foundURLs),
	}, nil
}
