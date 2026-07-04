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

const parameterInventorySkillSlug = "parameter_inventory"

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

func executeParameterInventorySkillRunsFromChat(db *gorm.DB, uid uint, ownerKey string, target *models.Target, selectedSkillRunIDs []uint) (map[string]interface{}, error) {
	if target == nil || len(selectedSkillRunIDs) == 0 {
		return nil, nil
	}

	runs := make([]models.OperatorSkillRun, 0)
	if err := db.
		Where("id IN ? AND target_id = ? AND user_id = ? AND skill_slug = ?", selectedSkillRunIDs, target.ID, uid, parameterInventorySkillSlug).
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
			primaryBugClass := "parameterized_endpoint"
			if len(candidate.BugClasses) > 0 {
				primaryBugClass = candidate.BugClasses[0]
			}

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
