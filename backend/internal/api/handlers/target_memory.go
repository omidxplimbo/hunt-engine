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
