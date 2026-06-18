package handlers

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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
