package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/datatypes"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

type createTargetMemoryRequest struct {
	MemoryType string                 `json:"memory_type"`
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Summary    string                 `json:"summary"`
	Tags       []string               `json:"tags"`
	Importance *int                   `json:"importance"`
	Confidence *int                   `json:"confidence"`
	Metadata   map[string]interface{} `json:"metadata"`
}

func memoryJSON(value interface{}, fallback string) datatypes.JSON {
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte(fallback))
	}
	return datatypes.JSON(b)
}

func memoryHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		h.Write([]byte(part))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func clampMemoryScore(v int) int {
	if v < 0 {
		return 0
	}
	if v > 100 {
		return 100
	}
	return v
}

func chunkMemoryContent(content string, maxRunes int) []string {
	content = strings.TrimSpace(content)
	if content == "" {
		return []string{}
	}
	if maxRunes <= 0 {
		maxRunes = 1800
	}

	runes := []rune(content)
	out := make([]string, 0)
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		out = append(out, strings.TrimSpace(string(runes[start:end])))
	}
	return out
}

func createMemoryChunks(item models.TargetMemoryItem) error {
	chunks := chunkMemoryContent(item.Content, 1800)

	for idx, chunk := range chunks {
		chunkRow := models.TargetMemoryChunk{
			UserID:       item.UserID,
			TargetID:     item.TargetID,
			MemoryItemID: item.ID,
			ChunkIndex:   idx,
			ChunkText:    chunk,
			TokenCount:   len([]rune(chunk)) / 4,
			SourceHash:   memoryHash(item.SourceHash, chunk),
			Metadata: memoryJSON(map[string]interface{}{
				"chunking_version": "target-memory-local-chunker-v1",
				"embedding_ready":  true,
				"embedding_set":    false,
			}, "{}"),
		}
		if err := database.DB.Create(&chunkRow).Error; err != nil {
			return err
		}

		event := models.TargetMemoryEvent{
			UserID:        item.UserID,
			TargetID:      item.TargetID,
			EventType:     models.TargetMemoryEventChunked,
			SourceType:    item.SourceType,
			SourceID:      item.SourceID,
			MemoryItemID:  &item.ID,
			MemoryChunkID: &chunkRow.ID,
			AfterJSON: memoryJSON(map[string]interface{}{
				"chunk_index": idx,
				"token_count": chunkRow.TokenCount,
			}, "{}"),
			Metadata: memoryJSON(map[string]interface{}{
				"chunking_version": "target-memory-local-chunker-v1",
			}, "{}"),
		}
		_ = database.DB.Create(&event).Error
	}

	return nil
}

func targetMemoryOwner(c *fiber.Ctx, targetID uint) (*models.User, *models.Target, error) {
	user, err := currentUser(c)
	if err != nil {
		return nil, nil, err
	}

	var target models.Target
	if err := database.DB.Where("id = ? AND created_by_user_id = ?", targetID, user.ID).First(&target).Error; err != nil {
		return nil, nil, fiber.NewError(fiber.StatusNotFound, "target not found")
	}

	return user, &target, nil
}

func ListTargetMemory(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, _, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	limit := c.QueryInt("limit", 50)
	if limit <= 0 || limit > 200 {
		limit = 50
	}

	memoryType := strings.TrimSpace(c.Query("memory_type"))
	sourceType := strings.TrimSpace(c.Query("source_type"))

	q := database.DB.
		Where("target_id = ? AND user_id = ?", targetID, user.ID).
		Order("importance DESC, updated_at DESC").
		Limit(limit)

	if memoryType != "" {
		q = q.Where("memory_type = ?", memoryType)
	}
	if sourceType != "" {
		q = q.Where("source_type = ?", sourceType)
	}

	items := make([]models.TargetMemoryItem, 0)
	if err := q.Find(&items).Error; err != nil {
		return err
	}

	var total int64
	_ = database.DB.Model(&models.TargetMemoryItem{}).
		Where("target_id = ? AND user_id = ?", targetID, user.ID).
		Count(&total).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"items": items,
			"total": total,
			"limit": limit,
		},
	})
}

func CreateTargetMemory(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, _, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	var req createTargetMemoryRequest
	if err := c.BodyParser(&req); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid request body")
	}

	req.Title = strings.TrimSpace(req.Title)
	req.Content = strings.TrimSpace(req.Content)
	req.Summary = strings.TrimSpace(req.Summary)
	req.MemoryType = strings.TrimSpace(req.MemoryType)

	if req.Title == "" {
		return fiber.NewError(fiber.StatusBadRequest, "title is required")
	}
	if req.Content == "" {
		return fiber.NewError(fiber.StatusBadRequest, "content is required")
	}
	if req.MemoryType == "" {
		req.MemoryType = models.TargetMemoryTypeUserDecision
	}

	importance := 50
	confidence := 50
	if req.Importance != nil {
		importance = clampMemoryScore(*req.Importance)
	}
	if req.Confidence != nil {
		confidence = clampMemoryScore(*req.Confidence)
	}

	item := models.TargetMemoryItem{
		UserID:     user.ID,
		TargetID:   targetID,
		SourceType: models.TargetMemorySourceUserNote,
		MemoryType: req.MemoryType,
		Title:      req.Title,
		Content:    req.Content,
		Summary:    req.Summary,
		Tags:       memoryJSON(req.Tags, "[]"),
		Importance: importance,
		Confidence: confidence,
		SourceHash: memoryHash(req.Title, req.Content, req.MemoryType),
		Metadata: memoryJSON(map[string]interface{}{
			"source":          "manual",
			"created_from_ui": true,
			"operator_ready":  true,
			"user_metadata":   req.Metadata,
		}, "{}"),
	}

	if err := database.DB.Create(&item).Error; err != nil {
		return err
	}

	event := models.TargetMemoryEvent{
		UserID:       user.ID,
		TargetID:     targetID,
		EventType:    models.TargetMemoryEventCreated,
		SourceType:   item.SourceType,
		SourceID:     item.SourceID,
		MemoryItemID: &item.ID,
		AfterJSON: memoryJSON(map[string]interface{}{
			"id":          item.ID,
			"title":       item.Title,
			"memory_type": item.MemoryType,
			"importance":  item.Importance,
			"confidence":  item.Confidence,
		}, "{}"),
		Metadata: memoryJSON(map[string]interface{}{
			"created_by": "target-memory-api-v1",
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	if err := createMemoryChunks(item); err != nil {
		return err
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"status": "ok",
		"data":   item,
	})
}

func GetTargetMemory(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	memoryID, err := parseUintParam(c, "memory_id")
	if err != nil {
		return err
	}

	user, _, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	var item models.TargetMemoryItem
	if err := database.DB.
		Where("id = ? AND target_id = ? AND user_id = ?", memoryID, targetID, user.ID).
		First(&item).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "memory item not found")
	}

	chunks := make([]models.TargetMemoryChunk, 0)
	_ = database.DB.
		Where("memory_item_id = ? AND target_id = ? AND user_id = ?", item.ID, targetID, user.ID).
		Order("chunk_index ASC").
		Find(&chunks).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"item":   item,
			"chunks": chunks,
		},
	})
}

func DeleteTargetMemory(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	memoryID, err := parseUintParam(c, "memory_id")
	if err != nil {
		return err
	}

	user, _, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	var item models.TargetMemoryItem
	if err := database.DB.
		Where("id = ? AND target_id = ? AND user_id = ?", memoryID, targetID, user.ID).
		First(&item).Error; err != nil {
		return fiber.NewError(fiber.StatusNotFound, "memory item not found")
	}

	_ = database.DB.
		Where("memory_item_id = ? AND target_id = ? AND user_id = ?", item.ID, targetID, user.ID).
		Delete(&models.TargetMemoryChunk{}).Error

	if err := database.DB.Delete(&item).Error; err != nil {
		return err
	}

	event := models.TargetMemoryEvent{
		UserID:       user.ID,
		TargetID:     targetID,
		EventType:    "memory.deleted",
		SourceType:   item.SourceType,
		SourceID:     item.SourceID,
		MemoryItemID: &item.ID,
		BeforeJSON: memoryJSON(map[string]interface{}{
			"id":          item.ID,
			"title":       item.Title,
			"memory_type": item.MemoryType,
		}, "{}"),
		Metadata: memoryJSON(map[string]interface{}{
			"deleted_by": "target-memory-api-v1",
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"deleted": true,
			"id":      item.ID,
		},
	})
}

func memoryItemBrief(item models.TargetMemoryItem) map[string]interface{} {
	text := strings.TrimSpace(item.Summary)
	if text == "" {
		text = strings.TrimSpace(item.Content)
	}
	runes := []rune(text)
	if len(runes) > 420 {
		text = string(runes[:420]) + "..."
	}

	return map[string]interface{}{
		"id":          item.ID,
		"memory_type": item.MemoryType,
		"source_type": item.SourceType,
		"source_id":   item.SourceID,
		"title":       item.Title,
		"summary":     text,
		"importance":  item.Importance,
		"confidence":  item.Confidence,
		"tags":        item.Tags,
		"updated_at":  item.UpdatedAt,
	}
}

func estimateContextTokens(value string) int {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	return len([]rune(value)) / 4
}

func GetTargetMemoryContext(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, target, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	maxItems := c.QueryInt("max_items", 18)
	if maxItems <= 0 || maxItems > 50 {
		maxItems = 18
	}

	tokenBudget := c.QueryInt("token_budget", 1800)
	if tokenBudget <= 300 || tokenBudget > 6000 {
		tokenBudget = 1800
	}

	important := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND user_id = ?", targetID, user.ID).
		Order("importance DESC, confidence DESC, updated_at DESC").
		Limit(maxItems).
		Find(&important).Error

	recent := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND user_id = ?", targetID, user.ID).
		Order("updated_at DESC").
		Limit(8).
		Find(&recent).Error

	policies := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND user_id = ? AND memory_type = ?", targetID, user.ID, models.TargetMemoryTypePolicyConstraint).
		Order("importance DESC, updated_at DESC").
		Limit(8).
		Find(&policies).Error

	failedTests := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND user_id = ? AND memory_type = ?", targetID, user.ID, models.TargetMemoryTypeFailedTest).
		Order("updated_at DESC").
		Limit(8).
		Find(&failedTests).Error

	successfulTests := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND user_id = ? AND memory_type = ?", targetID, user.ID, models.TargetMemoryTypeSuccessfulTest).
		Order("updated_at DESC").
		Limit(8).
		Find(&successfulTests).Error

	findingEvidence := make([]models.TargetMemoryItem, 0)
	_ = database.DB.
		Where("target_id = ? AND user_id = ? AND memory_type = ?", targetID, user.ID, models.TargetMemoryTypeFindingEvidence).
		Order("importance DESC, updated_at DESC").
		Limit(8).
		Find(&findingEvidence).Error

	typeCounts := make([]struct {
		MemoryType string `json:"memory_type"`
		Count      int64  `json:"count"`
	}, 0)
	_ = database.DB.Model(&models.TargetMemoryItem{}).
		Select("memory_type, COUNT(*) as count").
		Where("target_id = ? AND user_id = ?", targetID, user.ID).
		Group("memory_type").
		Order("count DESC").
		Scan(&typeCounts).Error

	importantBrief := make([]map[string]interface{}, 0, len(important))
	for _, item := range important {
		importantBrief = append(importantBrief, memoryItemBrief(item))
	}

	recentBrief := make([]map[string]interface{}, 0, len(recent))
	for _, item := range recent {
		recentBrief = append(recentBrief, memoryItemBrief(item))
	}

	policyBrief := make([]map[string]interface{}, 0, len(policies))
	for _, item := range policies {
		policyBrief = append(policyBrief, memoryItemBrief(item))
	}

	failedBrief := make([]map[string]interface{}, 0, len(failedTests))
	for _, item := range failedTests {
		failedBrief = append(failedBrief, memoryItemBrief(item))
	}

	successfulBrief := make([]map[string]interface{}, 0, len(successfulTests))
	for _, item := range successfulTests {
		successfulBrief = append(successfulBrief, memoryItemBrief(item))
	}

	evidenceBrief := make([]map[string]interface{}, 0, len(findingEvidence))
	for _, item := range findingEvidence {
		evidenceBrief = append(evidenceBrief, memoryItemBrief(item))
	}

	compactSummary := map[string]interface{}{
		"target_id":         target.ID,
		"target_name":       target.Name,
		"root_domain":       target.RootDomain,
		"status":            target.Status,
		"current_phase":     target.CurrentPhase,
		"memory_item_count": len(important),
		"operator_ready":    true,
		"context_version":   "target-memory-context-v1",
		"context_intent":    "Provide compact target memory for the RAG-backed LLM Pentest Operator.",
		"guardrail_summary": "Use this memory only for authorized target-scoped testing. Active/risky execution still requires policy checks and approval.",
	}

	contextTextParts := []string{
		target.Name,
		target.RootDomain,
		target.Status,
		target.CurrentPhase,
	}
	for _, item := range important {
		if item.Summary != "" {
			contextTextParts = append(contextTextParts, item.Summary)
		} else {
			contextTextParts = append(contextTextParts, item.Content)
		}
	}
	estimatedTokens := estimateContextTokens(strings.Join(contextTextParts, "\n"))

	event := models.TargetMemoryEvent{
		UserID:    user.ID,
		TargetID:  targetID,
		EventType: models.TargetMemoryEventUsed,
		AfterJSON: memoryJSON(map[string]interface{}{
			"context_version":  "target-memory-context-v1",
			"max_items":        maxItems,
			"token_budget":     tokenBudget,
			"estimated_tokens": estimatedTokens,
		}, "{}"),
		Metadata: memoryJSON(map[string]interface{}{
			"used_by": "target-memory-context-api-v1",
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"context_version":    "target-memory-context-v1",
			"compact_summary":    compactSummary,
			"important_memories": importantBrief,
			"recent_memories":    recentBrief,
			"policy_constraints": policyBrief,
			"failed_tests":       failedBrief,
			"successful_tests":   successfulBrief,
			"finding_evidence":   evidenceBrief,
			"type_counts":        typeCounts,
			"token_budget": fiber.Map{
				"requested":         tokenBudget,
				"estimated":         estimatedTokens,
				"within_budget":     estimatedTokens <= tokenBudget,
				"estimator":         "runes_div_4",
				"retrieval_limited": len(important) >= maxItems,
			},
			"operator_guardrails": []string{
				"Context is target-scoped and owner-scoped.",
				"Context is for planning and reasoning, not direct execution.",
				"Active/risky tests require policy checks and approval.",
				"Do not use memory for out-of-scope targets.",
			},
		},
	})
}

type ingestTargetMemoryRequest struct {
	IncludeAssets         *bool `json:"include_assets"`
	IncludeURLs           *bool `json:"include_urls"`
	IncludeFindings       *bool `json:"include_findings"`
	IncludeBugTestResults *bool `json:"include_bug_test_results"`
}

func boolDefault(value *bool, fallback bool) bool {
	if value == nil {
		return fallback
	}
	return *value
}

func upsertTargetMemoryItem(item models.TargetMemoryItem) (models.TargetMemoryItem, bool, error) {
	var existing models.TargetMemoryItem
	err := database.DB.
		Where("target_id = ? AND user_id = ? AND source_hash = ?", item.TargetID, item.UserID, item.SourceHash).
		First(&existing).Error

	if err == nil {
		existing.SourceType = item.SourceType
		existing.SourceID = item.SourceID
		existing.MemoryType = item.MemoryType
		existing.Title = item.Title
		existing.Content = item.Content
		existing.Summary = item.Summary
		existing.Tags = item.Tags
		existing.Importance = item.Importance
		existing.Confidence = item.Confidence
		existing.Metadata = item.Metadata

		if saveErr := database.DB.Save(&existing).Error; saveErr != nil {
			return existing, false, saveErr
		}

		_ = database.DB.
			Where("memory_item_id = ? AND target_id = ? AND user_id = ?", existing.ID, existing.TargetID, existing.UserID).
			Delete(&models.TargetMemoryChunk{}).Error

		if chunkErr := createMemoryChunks(existing); chunkErr != nil {
			return existing, false, chunkErr
		}

		event := models.TargetMemoryEvent{
			UserID:       existing.UserID,
			TargetID:     existing.TargetID,
			EventType:    models.TargetMemoryEventUpdated,
			SourceType:   existing.SourceType,
			SourceID:     existing.SourceID,
			MemoryItemID: &existing.ID,
			AfterJSON: memoryJSON(map[string]interface{}{
				"id":          existing.ID,
				"title":       existing.Title,
				"memory_type": existing.MemoryType,
				"source_hash": existing.SourceHash,
			}, "{}"),
			Metadata: memoryJSON(map[string]interface{}{
				"updated_by": "target-memory-ingest-v1",
			}, "{}"),
		}
		_ = database.DB.Create(&event).Error

		return existing, false, nil
	}

	if err := database.DB.Create(&item).Error; err != nil {
		return item, false, err
	}

	event := models.TargetMemoryEvent{
		UserID:       item.UserID,
		TargetID:     item.TargetID,
		EventType:    models.TargetMemoryEventCreated,
		SourceType:   item.SourceType,
		SourceID:     item.SourceID,
		MemoryItemID: &item.ID,
		AfterJSON: memoryJSON(map[string]interface{}{
			"id":          item.ID,
			"title":       item.Title,
			"memory_type": item.MemoryType,
			"source_hash": item.SourceHash,
		}, "{}"),
		Metadata: memoryJSON(map[string]interface{}{
			"created_by": "target-memory-ingest-v1",
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	if err := createMemoryChunks(item); err != nil {
		return item, true, err
	}

	return item, true, nil
}

func IngestTargetMemory(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, target, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	var req ingestTargetMemoryRequest
	_ = c.BodyParser(&req)

	includeAssets := boolDefault(req.IncludeAssets, true)
	includeURLs := boolDefault(req.IncludeURLs, true)
	includeFindings := boolDefault(req.IncludeFindings, true)
	includeBugTestResults := boolDefault(req.IncludeBugTestResults, true)

	created := 0
	updated := 0
	items := make([]models.TargetMemoryItem, 0)

	targetContent := strings.Join([]string{
		"Target overview:",
		"Name: " + target.Name,
		"Root domain: " + target.RootDomain,
		"Description: " + target.Description,
		"Status: " + target.Status,
		"Current phase: " + target.CurrentPhase,
		"Scan modules: " + target.ScanModules,
		"In scope: " + strconv.FormatBool(target.InScope),
		"Tool flags:",
		"- alterx=" + strconv.FormatBool(target.UseAlterx),
		"- gau=" + strconv.FormatBool(target.UseGau),
		"- katana=" + strconv.FormatBool(target.UseKatana),
		"- virustotal=" + strconv.FormatBool(target.UseVirusTotal),
		"- nuclei=" + strconv.FormatBool(target.UseNuclei),
		"- portscan=" + strconv.FormatBool(target.UsePortscan),
		"This memory item is a compact target overview for the RAG-backed LLM Pentest Operator.",
	}, "\n")

	targetItem := models.TargetMemoryItem{
		UserID:     user.ID,
		TargetID:   targetID,
		SourceType: models.TargetMemorySourceTarget,
		SourceID:   &target.ID,
		MemoryType: models.TargetMemoryTypeOverview,
		Title:      "Target overview: " + target.Name,
		Content:    targetContent,
		Summary:    "Target " + target.Name + " on " + target.RootDomain + " is " + target.Status + " / " + target.CurrentPhase + ".",
		Tags:       memoryJSON([]string{"target", "overview", "operator"}, "[]"),
		Importance: 90,
		Confidence: 90,
		SourceHash: memoryHash("target_overview", target.RootDomain, target.Name, target.Status, target.CurrentPhase),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version": "target-memory-ingest-v1",
			"root_domain":    target.RootDomain,
			"status":         target.Status,
			"current_phase":  target.CurrentPhase,
			"in_scope":       target.InScope,
		}, "{}"),
	}

	row, wasCreated, err := upsertTargetMemoryItem(targetItem)
	if err != nil {
		return err
	}
	if wasCreated {
		created++
	} else {
		updated++
	}
	items = append(items, row)

	if includeAssets {
		var totalAssets int64
		var liveAssets int64
		_ = database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID).Count(&totalAssets).Error
		_ = database.DB.Model(&models.Asset{}).Where("target_id = ? AND is_live = ?", targetID, true).Count(&liveAssets).Error

		topAssets := make([]models.Asset, 0)
		_ = database.DB.
			Where("target_id = ? AND is_live = ?", targetID, true).
			Order("status_code DESC, id DESC").
			Limit(15).
			Find(&topAssets).Error

		lines := []string{
			"Live asset summary:",
			"Total assets: " + strconv.FormatInt(totalAssets, 10),
			"Live assets: " + strconv.FormatInt(liveAssets, 10),
			"Representative live assets:",
		}
		for _, asset := range topAssets {
			line := "- " + asset.Value +
				" status=" + strconv.Itoa(asset.StatusCode) +
				" final_url=" + asset.FinalURL +
				" title=" + asset.Title +
				" web_server=" + asset.WebServer +
				" tech=" + asset.Technologies
			lines = append(lines, line)
		}

		item := models.TargetMemoryItem{
			UserID:     user.ID,
			TargetID:   targetID,
			SourceType: models.TargetMemorySourceAsset,
			MemoryType: models.TargetMemoryTypeAttackSurface,
			Title:      "Live asset summary",
			Content:    strings.Join(lines, "\n"),
			Summary:    "Target has " + strconv.FormatInt(liveAssets, 10) + " live assets out of " + strconv.FormatInt(totalAssets, 10) + " discovered assets.",
			Tags:       memoryJSON([]string{"assets", "subdomains", "attack_surface", "operator"}, "[]"),
			Importance: 85,
			Confidence: 85,
			SourceHash: memoryHash("asset_summary", target.RootDomain, strconv.FormatInt(totalAssets, 10), strconv.FormatInt(liveAssets, 10)),
			Metadata: memoryJSON(map[string]interface{}{
				"ingest_version": "target-memory-ingest-v1",
				"total_assets":   totalAssets,
				"live_assets":    liveAssets,
			}, "{}"),
		}

		row, wasCreated, err := upsertTargetMemoryItem(item)
		if err != nil {
			return err
		}
		if wasCreated {
			created++
		} else {
			updated++
		}
		items = append(items, row)
	}

	if includeURLs {
		var totalURLs int64
		_ = database.DB.Model(&models.FoundURL{}).Where("target_id = ?", targetID).Count(&totalURLs).Error

		urls := make([]models.FoundURL, 0)
		_ = database.DB.
			Where("target_id = ?", targetID).
			Order("occurrence_count DESC, id DESC").
			Limit(20).
			Find(&urls).Error

		lines := []string{
			"URL inventory summary:",
			"Total URLs: " + strconv.FormatInt(totalURLs, 10),
			"Representative URLs:",
		}
		for _, u := range urls {
			lines = append(lines, "- "+u.Value+" source="+u.Source+" occurrences="+strconv.Itoa(u.OccurrenceCount))
		}

		item := models.TargetMemoryItem{
			UserID:     user.ID,
			TargetID:   targetID,
			SourceType: models.TargetMemorySourceURL,
			MemoryType: models.TargetMemoryTypeEndpointNote,
			Title:      "URL inventory summary",
			Content:    strings.Join(lines, "\n"),
			Summary:    "Target has " + strconv.FormatInt(totalURLs, 10) + " collected URLs for endpoint, parameter, and crawl planning.",
			Tags:       memoryJSON([]string{"urls", "endpoints", "parameters", "operator"}, "[]"),
			Importance: 82,
			Confidence: 82,
			SourceHash: memoryHash("url_summary", target.RootDomain, strconv.FormatInt(totalURLs, 10)),
			Metadata: memoryJSON(map[string]interface{}{
				"ingest_version": "target-memory-ingest-v1",
				"total_urls":     totalURLs,
			}, "{}"),
		}

		row, wasCreated, err := upsertTargetMemoryItem(item)
		if err != nil {
			return err
		}
		if wasCreated {
			created++
		} else {
			updated++
		}
		items = append(items, row)
	}

	if includeFindings {
		var totalFindings int64
		_ = database.DB.Model(&models.Finding{}).Where("target_id = ?", targetID).Count(&totalFindings).Error

		findings := make([]models.Finding, 0)
		_ = database.DB.
			Where("target_id = ?", targetID).
			Order("severity DESC, id DESC").
			Limit(15).
			Find(&findings).Error

		lines := []string{
			"Finding summary:",
			"Total findings: " + strconv.FormatInt(totalFindings, 10),
			"Representative findings:",
		}
		for _, finding := range findings {
			lines = append(lines, "- "+finding.Title+" severity="+finding.Severity+" category="+finding.Category+" status="+finding.Status+" source="+finding.SourceTool)
		}

		item := models.TargetMemoryItem{
			UserID:     user.ID,
			TargetID:   targetID,
			SourceType: models.TargetMemorySourceFinding,
			MemoryType: models.TargetMemoryTypeFindingEvidence,
			Title:      "Finding summary",
			Content:    strings.Join(lines, "\n"),
			Summary:    "Target has " + strconv.FormatInt(totalFindings, 10) + " findings available for operator triage and evidence review.",
			Tags:       memoryJSON([]string{"findings", "evidence", "triage", "operator"}, "[]"),
			Importance: 78,
			Confidence: 82,
			SourceHash: memoryHash("finding_summary", target.RootDomain, strconv.FormatInt(totalFindings, 10)),
			Metadata: memoryJSON(map[string]interface{}{
				"ingest_version": "target-memory-ingest-v1",
				"total_findings": totalFindings,
			}, "{}"),
		}

		row, wasCreated, err := upsertTargetMemoryItem(item)
		if err != nil {
			return err
		}
		if wasCreated {
			created++
		} else {
			updated++
		}
		items = append(items, row)
	}

	if includeBugTestResults {
		var totalResults int64
		_ = database.DB.Model(&models.BugTestResult{}).Where("target_id = ?", targetID).Count(&totalResults).Error

		results := make([]models.BugTestResult, 0)
		_ = database.DB.
			Where("target_id = ?", targetID).
			Order("id DESC").
			Limit(15).
			Find(&results).Error

		lines := []string{
			"Bug test result summary:",
			"Total bug test results: " + strconv.FormatInt(totalResults, 10),
			"Recent bug test results:",
		}
		for _, result := range results {
			lines = append(lines, "- "+result.TestName+" type="+result.BugType+" status="+result.Status+" confidence="+result.Confidence+" severity_hint="+result.SeverityHint+" pattern_key="+result.PatternKey)
		}

		item := models.TargetMemoryItem{
			UserID:     user.ID,
			TargetID:   targetID,
			SourceType: models.TargetMemorySourceBugTestResult,
			MemoryType: models.TargetMemoryTypeTestResult,
			Title:      "Bug test result summary",
			Content:    strings.Join(lines, "\n"),
			Summary:    "Target has " + strconv.FormatInt(totalResults, 10) + " bug test results available for iterative operator planning.",
			Tags:       memoryJSON([]string{"bug_tests", "test_results", "operator"}, "[]"),
			Importance: 80,
			Confidence: 82,
			SourceHash: memoryHash("bug_test_result_summary", target.RootDomain, strconv.FormatInt(totalResults, 10)),
			Metadata: memoryJSON(map[string]interface{}{
				"ingest_version":    "target-memory-ingest-v1",
				"total_bug_results": totalResults,
			}, "{}"),
		}

		row, wasCreated, err := upsertTargetMemoryItem(item)
		if err != nil {
			return err
		}
		if wasCreated {
			created++
		} else {
			updated++
		}
		items = append(items, row)
	}

	event := models.TargetMemoryEvent{
		UserID:    user.ID,
		TargetID:  targetID,
		EventType: "memory.ingested",
		AfterJSON: memoryJSON(map[string]interface{}{
			"created": created,
			"updated": updated,
			"total":   len(items),
		}, "{}"),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version": "target-memory-ingest-v1",
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"ingest_version": "target-memory-ingest-v1",
			"created":        created,
			"updated":        updated,
			"items":          items,
		},
	})
}

func normalizeMemoryObjective(objective string) string {
	objective = strings.ToLower(strings.TrimSpace(objective))
	switch objective {
	case "all", "all_bugs", "all-bugs", "bugs", "vuln", "vulns", "vulnerability", "vulnerability_discovery", "vulnerability-discovery", "full", "full_scan", "full-scan":
		return "all_bugs"
	case "xss", "cross_site_scripting", "cross-site-scripting":
		return "xss"
	case "redirect", "open_redirect", "open-redirect":
		return "open_redirect"
	case "cors":
		return "cors"
	case "headers", "security_headers", "security-headers":
		return "security_headers"
	case "js", "javascript", "js_review", "js-review":
		return "js_review"
	case "auth", "authentication", "authorization":
		return "auth"
	case "access", "access_control", "access-control", "idor", "bola", "bfl":
		return "access_control"
	case "api", "api_security", "api-security":
		return "api"
	case "ssrf":
		return "ssrf"
	case "sqli", "sql", "sql_injection", "sql-injection":
		return "sqli"
	case "nosqli", "nosql", "nosql_injection", "nosql-injection":
		return "nosqli"
	case "ssti", "template_injection", "template-injection":
		return "ssti"
	case "path", "path_traversal", "path-traversal", "lfi", "rfi", "file_inclusion", "file-inclusion":
		return "path_traversal"
	case "upload", "file_upload", "file-upload":
		return "file_upload"
	case "csrf":
		return "csrf"
	case "clickjacking", "click_jacking":
		return "clickjacking"
	case "takeover", "subdomain_takeover", "subdomain-takeover":
		return "takeover"
	case "secret", "secrets", "exposed_secrets", "exposed-secrets":
		return "secrets"
	case "cloud", "storage", "cloud_misconfig", "cloud-misconfig":
		return "cloud"
	case "cache", "cache_poisoning", "cache-poisoning":
		return "cache_poisoning"
	case "rate", "rate_limit", "rate-limit", "ratelimit":
		return "rate_limit"
	case "session", "cookie", "cookies", "session_cookie", "session-cookie":
		return "session"
	case "logic", "business_logic", "business-logic":
		return "business_logic"
	case "misconfig", "misconfiguration", "config":
		return "misconfiguration"
	case "cve", "dependency", "dependencies", "dependency_cve", "dependency-cve":
		return "cve"
	case "crawl", "crawl_more", "crawl-more", "recon", "discovery":
		return "crawl_more"
	case "report", "reporting":
		return "report"
	default:
		return "general"
	}
}

func objectiveMemoryTerms(objective string) []string {
	switch objective {
	case "all_bugs":
		return []string{"finding", "evidence", "bug", "vulnerability", "endpoint", "parameter", "auth", "api", "token", "secret", "redirect", "xss", "cors", "header", "upload", "path", "ssrf", "sql", "idor", "access", "session", "cookie", "rate", "takeover", "misconfig"}
	case "xss":
		return []string{"xss", "cross-site", "script", "payload", "reflection", "parameter", "query", "dom", "sink", "url", "endpoint"}
	case "open_redirect":
		return []string{"redirect", "url=", "next", "return", "continue", "dest", "destination", "callback", "redirect_uri", "location"}
	case "cors":
		return []string{"cors", "origin", "access-control", "allow-origin", "credentials", "header"}
	case "security_headers":
		return []string{"header", "csp", "content-security-policy", "hsts", "x-frame", "x-content-type", "referrer-policy"}
	case "js_review":
		return []string{"javascript", "js", ".js", "source", "sink", "dom", "endpoint", "api", "token", "secret"}
	case "auth":
		return []string{"auth", "login", "session", "jwt", "token", "oauth", "sso", "cookie", "password", "account"}
	case "access_control":
		return []string{"idor", "access", "authorization", "permission", "role", "user_id", "account_id", "tenant", "object", "admin", "private"}
	case "api":
		return []string{"api", "graphql", "rest", "json", "endpoint", "token", "authorization", "swagger", "openapi"}
	case "ssrf":
		return []string{"ssrf", "url", "uri", "callback", "webhook", "fetch", "proxy", "import", "remote", "internal"}
	case "sqli":
		return []string{"sql", "sqli", "query", "id=", "search", "filter", "sort", "where", "select", "database"}
	case "nosqli":
		return []string{"nosql", "mongodb", "mongo", "filter", "json", "$ne", "$regex", "query"}
	case "ssti":
		return []string{"ssti", "template", "render", "jinja", "twig", "handlebars", "velocity", "expression"}
	case "path_traversal":
		return []string{"path", "file", "download", "upload", "include", "lfi", "rfi", "traversal", "../", "filename"}
	case "file_upload":
		return []string{"upload", "file", "avatar", "image", "attachment", "multipart", "content-type", "extension"}
	case "csrf":
		return []string{"csrf", "state-changing", "post", "delete", "update", "token", "form"}
	case "clickjacking":
		return []string{"clickjacking", "frame", "iframe", "x-frame-options", "csp", "frame-ancestors"}
	case "takeover":
		return []string{"takeover", "cname", "dangling", "bucket", "github", "heroku", "vercel", "netlify", "azure"}
	case "secrets":
		return []string{"secret", "token", "apikey", "api_key", "password", "credential", "private_key", "jwt", "bearer"}
	case "cloud":
		return []string{"cloud", "s3", "bucket", "storage", "azure", "gcp", "aws", "blob", "public"}
	case "cache_poisoning":
		return []string{"cache", "poison", "vary", "host", "x-forwarded", "cdn", "akamai", "cloudflare"}
	case "rate_limit":
		return []string{"rate", "limit", "throttle", "otp", "password reset", "login", "bruteforce", "enumeration"}
	case "session":
		return []string{"session", "cookie", "jwt", "secure", "httponly", "samesite", "token", "refresh"}
	case "business_logic":
		return []string{"logic", "workflow", "checkout", "price", "coupon", "role", "state", "order", "payment"}
	case "misconfiguration":
		return []string{"misconfig", "debug", "admin", "exposed", "directory", "index", "backup", "config", "env"}
	case "cve":
		return []string{"cve", "version", "dependency", "technology", "server", "framework", "outdated"}
	case "crawl_more":
		return []string{"crawl", "url", "endpoint", "asset", "subdomain", "live", "katana", "gau", "wayback", "virustotal"}
	case "report":
		return []string{"finding", "evidence", "impact", "reproduce", "severity", "recommendation", "report"}
	default:
		return []string{"target", "asset", "url", "finding", "test", "operator", "memory"}
	}
}

func objectiveMemoryTypes(objective string) []string {
	switch objective {
	case "all_bugs":
		return []string{
			models.TargetMemoryTypeOverview,
			models.TargetMemoryTypeAttackSurface,
			models.TargetMemoryTypeEndpointNote,
			models.TargetMemoryTypeFindingEvidence,
			models.TargetMemoryTypeTestResult,
			models.TargetMemoryTypeVulnerabilityHypothesis,
			models.TargetMemoryTypeFailedTest,
			models.TargetMemoryTypeSuccessfulTest,
			models.TargetMemoryTypePolicyConstraint,
			models.TargetMemoryTypeUserDecision,
		}
	case "xss", "open_redirect", "cors", "security_headers", "js_review", "auth", "access_control", "api", "ssrf", "sqli", "nosqli", "ssti", "path_traversal", "file_upload", "csrf", "clickjacking", "takeover", "secrets", "cloud", "cache_poisoning", "rate_limit", "session", "business_logic", "misconfiguration", "cve":
		return []string{
			models.TargetMemoryTypeEndpointNote,
			models.TargetMemoryTypeFindingEvidence,
			models.TargetMemoryTypeTestResult,
			models.TargetMemoryTypeVulnerabilityHypothesis,
			models.TargetMemoryTypeFailedTest,
			models.TargetMemoryTypeSuccessfulTest,
			models.TargetMemoryTypeAttackSurface,
			models.TargetMemoryTypePolicyConstraint,
			models.TargetMemoryTypeUserDecision,
		}
	case "crawl_more":
		return []string{
			models.TargetMemoryTypeAttackSurface,
			models.TargetMemoryTypeEndpointNote,
			models.TargetMemoryTypeOverview,
			models.TargetMemoryTypePolicyConstraint,
			models.TargetMemoryTypeUserDecision,
		}
	case "report":
		return []string{
			models.TargetMemoryTypeFindingEvidence,
			models.TargetMemoryTypeSuccessfulTest,
			models.TargetMemoryTypeTestResult,
			models.TargetMemoryTypeUserDecision,
			models.TargetMemoryTypePolicyConstraint,
		}
	default:
		return []string{}
	}
}

func objectiveRecommendedAction(objective string) string {
	switch objective {
	case "all_bugs":
		return "build_prioritized_owasp_test_plan_from_full_attack_surface"
	case "xss":
		return "review_parameters_or_propose_safe_xss_tests"
	case "open_redirect":
		return "review_redirect_parameters_or_propose_safe_open_redirect_tests"
	case "cors":
		return "review_cors_candidates_or_run_safe_cors_checks"
	case "security_headers":
		return "run_safe_security_header_checks"
	case "js_review":
		return "run_js_intelligence_or_review_js_findings"
	case "auth":
		return "ask_clarifying_question_before_auth_testing"
	case "access_control":
		return "ask_for_authorized_test_accounts_before_access_control_testing"
	case "api":
		return "review_api_endpoints_and_auth_context"
	case "ssrf":
		return "review_ssrf_candidates_and_require_approval_before_active_testing"
	case "sqli", "nosqli":
		return "review_injection_candidates_and_require_approval_before_active_testing"
	case "ssti":
		return "review_template_candidates_and_require_approval_before_active_testing"
	case "path_traversal":
		return "review_file_path_candidates_and_require_approval_before_active_testing"
	case "file_upload":
		return "ask_for_upload_scope_and_allowed_file_types"
	case "csrf":
		return "review_state_changing_endpoints"
	case "clickjacking":
		return "check_frame_protection_headers"
	case "takeover":
		return "review_takeover_candidates"
	case "secrets":
		return "review_exposed_secret_candidates"
	case "cloud":
		return "review_cloud_storage_exposure_candidates"
	case "cache_poisoning":
		return "review_cache_key_and_header_behavior_candidates"
	case "rate_limit":
		return "ask_for_rate_limit_testing_policy_before_active_testing"
	case "session":
		return "review_cookie_and_session_security"
	case "business_logic":
		return "ask_clarifying_question_for_business_logic_workflow"
	case "misconfiguration":
		return "review_exposed_admin_debug_config_surfaces"
	case "cve":
		return "review_detected_technologies_for_known_exposures"
	case "crawl_more":
		return "propose_policy_checked_crawling"
	case "report":
		return "draft_report_from_validated_evidence"
	default:
		return "retrieve_more_context_or_ask_clarifying_question"
	}
}

func memoryTypeWhere(memoryTypes []string) (string, []interface{}) {
	if len(memoryTypes) == 0 {
		return "", nil
	}

	placeholders := make([]string, 0, len(memoryTypes))
	args := make([]interface{}, 0, len(memoryTypes))
	for _, mt := range memoryTypes {
		placeholders = append(placeholders, "?")
		args = append(args, mt)
	}

	return "memory_type IN (" + strings.Join(placeholders, ",") + ")", args
}

func RetrieveTargetMemory(c *fiber.Ctx) error {
	targetID, err := parseUintParam(c, "id")
	if err != nil {
		return err
	}

	user, target, err := targetMemoryOwner(c, targetID)
	if err != nil {
		return err
	}

	objective := normalizeMemoryObjective(c.Query("objective", "all_bugs"))

	tokenBudget := c.QueryInt("token_budget", 2200)
	if tokenBudget <= 300 || tokenBudget > 10000 {
		tokenBudget = 2200
	}

	limit := c.QueryInt("limit", 16)
	if limit <= 0 || limit > 80 {
		limit = 16
	}

	chunkLimit := c.QueryInt("chunk_limit", 12)
	if chunkLimit <= 0 || chunkLimit > 60 {
		chunkLimit = 12
	}

	terms := objectiveMemoryTerms(objective)
	memoryTypes := objectiveMemoryTypes(objective)

	items := make([]models.TargetMemoryItem, 0)
	q := database.DB.Where("target_id = ? AND user_id = ?", targetID, user.ID)

	if where, args := memoryTypeWhere(memoryTypes); where != "" {
		q = q.Where(where, args...)
	}

	_ = q.Order("importance DESC, confidence DESC, updated_at DESC").
		Limit(limit).
		Find(&items).Error

	if len(items) < limit {
		extra := make([]models.TargetMemoryItem, 0)
		_ = database.DB.
			Where("target_id = ? AND user_id = ?", targetID, user.ID).
			Order("updated_at DESC").
			Limit(limit - len(items)).
			Find(&extra).Error

		seen := map[uint]bool{}
		for _, item := range items {
			seen[item.ID] = true
		}
		for _, item := range extra {
			if !seen[item.ID] {
				items = append(items, item)
				seen[item.ID] = true
			}
		}
	}

	itemIDs := make([]uint, 0, len(items))
	itemBriefs := make([]map[string]interface{}, 0, len(items))
	for _, item := range items {
		itemIDs = append(itemIDs, item.ID)
		itemBriefs = append(itemBriefs, memoryItemBrief(item))
	}

	chunks := make([]models.TargetMemoryChunk, 0)
	if len(itemIDs) > 0 {
		_ = database.DB.
			Where("target_id = ? AND user_id = ? AND memory_item_id IN ?", targetID, user.ID, itemIDs).
			Order("chunk_index ASC").
			Limit(chunkLimit).
			Find(&chunks).Error
	}

	chunkBriefs := make([]map[string]interface{}, 0, len(chunks))
	contextParts := []string{target.Name, target.RootDomain, objective}
	for _, chunk := range chunks {
		text := strings.TrimSpace(chunk.ChunkText)
		runes := []rune(text)
		if len(runes) > 700 {
			text = string(runes[:700]) + "..."
		}

		chunkBriefs = append(chunkBriefs, map[string]interface{}{
			"id":             chunk.ID,
			"memory_item_id": chunk.MemoryItemID,
			"chunk_index":    chunk.ChunkIndex,
			"chunk_text":     text,
			"token_count":    chunk.TokenCount,
			"source_hash":    chunk.SourceHash,
			"metadata":       chunk.Metadata,
			"created_at":     chunk.CreatedAt,
			"updated_at":     chunk.UpdatedAt,
		})
		contextParts = append(contextParts, text)
	}

	relevantURLs := make([]models.FoundURL, 0)
	urlQuery := database.DB.Where("target_id = ?", targetID)
	if objective != "general" && len(terms) > 0 {
		likeParts := make([]string, 0, len(terms))
		args := make([]interface{}, 0, len(terms))
		for _, term := range terms {
			likeParts = append(likeParts, "LOWER(value) LIKE ?")
			args = append(args, "%"+strings.ToLower(term)+"%")
		}
		urlQuery = urlQuery.Where(strings.Join(likeParts, " OR "), args...)
	}
	_ = urlQuery.Order("occurrence_count DESC, id DESC").Limit(20).Find(&relevantURLs).Error

	urlBriefs := make([]map[string]interface{}, 0, len(relevantURLs))
	for _, u := range relevantURLs {
		urlBriefs = append(urlBriefs, map[string]interface{}{
			"id":               u.ID,
			"value":            u.Value,
			"canonical_value":  u.CanonicalValue,
			"occurrence_count": u.OccurrenceCount,
			"source":           u.Source,
			"asset_id":         u.AssetID,
		})
		contextParts = append(contextParts, u.Value)
	}

	relevantResults := make([]models.BugTestResult, 0)
	resultQuery := database.DB.Where("target_id = ?", targetID)
	if objective != "general" && objective != "all_bugs" {
		resultQuery = resultQuery.Where(
			"LOWER(bug_type) LIKE ? OR LOWER(test_name) LIKE ? OR LOWER(pattern_key) LIKE ?",
			"%"+strings.ReplaceAll(objective, "_", "%")+"%",
			"%"+strings.ReplaceAll(objective, "_", "%")+"%",
			"%"+strings.ReplaceAll(objective, "_", "%")+"%",
		)
	}
	_ = resultQuery.Order("id DESC").Limit(20).Find(&relevantResults).Error

	resultBriefs := make([]map[string]interface{}, 0, len(relevantResults))
	for _, r := range relevantResults {
		resultBriefs = append(resultBriefs, map[string]interface{}{
			"id":            r.ID,
			"run_id":        r.RunID,
			"bug_type":      r.BugType,
			"test_name":     r.TestName,
			"status":        r.Status,
			"confidence":    r.Confidence,
			"severity_hint": r.SeverityHint,
			"pattern_key":   r.PatternKey,
			"asset_id":      r.AssetID,
			"url_id":        r.URLID,
			"finding_id":    r.FindingID,
			"tags":          r.Tags,
			"owasp_refs":    r.OWASPRefs,
		})
		contextParts = append(contextParts, r.TestName+" "+r.BugType+" "+r.Status+" "+r.Confidence)
	}

	relevantFindings := make([]models.Finding, 0)
	findingQuery := database.DB.Where("target_id = ?", targetID)
	if objective != "general" && objective != "all_bugs" && len(terms) > 0 {
		likeParts := make([]string, 0, len(terms))
		args := make([]interface{}, 0, len(terms)*3)
		for _, term := range terms {
			like := "%" + strings.ToLower(term) + "%"
			likeParts = append(likeParts, "(LOWER(title) LIKE ? OR LOWER(description) LIKE ? OR LOWER(category) LIKE ?)")
			args = append(args, like, like, like)
		}
		findingQuery = findingQuery.Where(strings.Join(likeParts, " OR "), args...)
	}
	_ = findingQuery.Order("severity DESC, id DESC").Limit(20).Find(&relevantFindings).Error

	findingBriefs := make([]map[string]interface{}, 0, len(relevantFindings))
	for _, f := range relevantFindings {
		findingBriefs = append(findingBriefs, map[string]interface{}{
			"id":          f.ID,
			"title":       f.Title,
			"severity":    f.Severity,
			"category":    f.Category,
			"status":      f.Status,
			"source_tool": f.SourceTool,
			"asset_id":    f.AssetID,
			"url_id":      f.URLID,
		})
		contextParts = append(contextParts, f.Title+" "+f.Severity+" "+f.Category+" "+f.Status)
	}

	estimatedTokens := estimateContextTokens(strings.Join(contextParts, "\n"))

	missingContext := []string{}
	if len(itemBriefs) == 0 {
		missingContext = append(missingContext, "No target memory found; run automatic memory ingestion first.")
	}
	if (objective == "all_bugs" || objective == "crawl_more") && len(urlBriefs) == 0 {
		missingContext = append(missingContext, "No relevant URLs found; propose crawling or URL discovery.")
	}
	if objective == "js_review" && len(urlBriefs) == 0 {
		missingContext = append(missingContext, "No JS-specific URLs found; propose JS intelligence or crawling.")
	}
	if objective == "auth" || objective == "access_control" || objective == "business_logic" {
		missingContext = append(missingContext, "Interactive clarification or authorized test accounts may be required before meaningful testing.")
	}
	if objective == "ssrf" || objective == "sqli" || objective == "nosqli" || objective == "ssti" || objective == "path_traversal" || objective == "file_upload" {
		missingContext = append(missingContext, "Active validation for this class requires explicit policy checks and approval.")
	}
	if len(missingContext) == 0 {
		missingContext = append(missingContext, "No critical missing context detected from current retrieval.")
	}

	event := models.TargetMemoryEvent{
		UserID:    user.ID,
		TargetID:  targetID,
		EventType: models.TargetMemoryEventUsed,
		AfterJSON: memoryJSON(map[string]interface{}{
			"retrieval_version": "target-memory-retrieval-v1",
			"objective":         objective,
			"token_budget":      tokenBudget,
			"estimated_tokens":  estimatedTokens,
			"items":             len(itemBriefs),
			"chunks":            len(chunkBriefs),
			"urls":              len(urlBriefs),
			"bug_results":       len(resultBriefs),
			"findings":          len(findingBriefs),
		}, "{}"),
		Metadata: memoryJSON(map[string]interface{}{
			"used_by": "target-memory-retrieval-api-v1",
		}, "{}"),
	}
	_ = database.DB.Create(&event).Error

	return c.JSON(fiber.Map{
		"status": "ok",
		"data": fiber.Map{
			"retrieval_version": "target-memory-retrieval-v1",
			"objective":         objective,
			"target": fiber.Map{
				"id":          target.ID,
				"name":        target.Name,
				"root_domain": target.RootDomain,
				"status":      target.Status,
				"phase":       target.CurrentPhase,
			},
			"selected_memories":                itemBriefs,
			"selected_chunks":                  chunkBriefs,
			"relevant_urls":                    urlBriefs,
			"relevant_bug_results":             resultBriefs,
			"relevant_findings":                findingBriefs,
			"suggested_missing_context":        missingContext,
			"recommended_next_operator_action": objectiveRecommendedAction(objective),
			"bug_coverage_note":                "Operator retrieval is broad by design and is not limited to XSS or a small set of bug classes.",
			"token_budget": fiber.Map{
				"requested":     tokenBudget,
				"estimated":     estimatedTokens,
				"within_budget": estimatedTokens <= tokenBudget,
				"estimator":     "runes_div_4",
			},
			"operator_guardrails": []string{
				"Retrieval is target-scoped and owner-scoped.",
				"Use this context for planning, prioritization, and clarifying questions.",
				"Active/risky tests require policy checks and approval.",
				"Additional crawling must stay within target scope and configured rate limits.",
				"Retain low-severity findings when useful for chaining, evidence, or reporting.",
			},
		},
	})
}
