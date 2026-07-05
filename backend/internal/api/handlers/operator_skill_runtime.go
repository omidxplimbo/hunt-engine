package handlers

import (
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"gorm.io/gorm"
)

const (
	parameterInventorySkillSlug   = "parameter_inventory"
	httpEvidenceAnalysisSkillSlug = "http_evidence_analysis"
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

	notImplemented := make([]map[string]interface{}, 0)
	for slug, runIDs := range selectedBySlug {
		if slug == parameterInventorySkillSlug || slug == httpEvidenceAnalysisSkillSlug {
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
