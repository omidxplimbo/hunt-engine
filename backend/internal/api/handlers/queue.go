package handlers

import (
	"context"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

// QueueItem represents an item in the queue
type QueueItem struct {
	Index   int    `json:"index"`
	Payload string `json:"payload"` // raw payload: "module:targetID:domain"
}

// GetQueueList returns all items in the queue
func GetQueueList(c *fiber.Ctx) error {
	items, err := redisq.Client.LRange(context.Background(), redisq.QueueName, 0, -1).Result()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch queue"})
	}

	queueItems := make([]QueueItem, len(items))
	for i, item := range items {
		queueItems[i] = QueueItem{
			Index:   i,
			Payload: item,
		}
	}

	return c.JSON(queueItems)
}

// RemoveFromQueue removes a specific item by index
// Note: Redis List doesn't support removing by index directly easily without concurrency issues if not careful.
// LREM removes by value. If we have duplicates, it might remove the wrong one, but task payloads should be unique per target.
// But users might re-queue the same target.
// A safer way for index removal is LSET to a special value then LREM.
func RemoveFromQueue(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid index"})
	}

	// Optimistic approach: Get the value at index, then remove one occurrence of it.
	// OR: use Lua script for atomic removal by index.
	// For simplicity, let's use the unique ID approach (UUID) but we store "module:id:domain".
	// Since "module:id:domain" is fairly unique (unless same target queued twice for same module), we try LREM.
	// Actually, the user asked to remove by "target".
	
	// Let's implement removal by index using a placeholder.
	// 1. LSET key index "__DELETED__"
	// 2. LREM key 0 "__DELETED__"
	
	ctx := context.Background()
	
	// Verify index is within range
	qLen, err := redisq.Client.LLen(ctx, redisq.QueueName).Result()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Redis error"})
	}
	if int64(index) >= qLen || index < 0 {
		return c.Status(400).JSON(fiber.Map{"error": "Index out of range"})
	}

	// Retrieve the item payload before deleting to sync target status
	val, err := redisq.Client.LIndex(ctx, redisq.QueueName, int64(index)).Result()
	if err == nil {
		// Payload format: "module:targetID:domain"
		parts := strings.Split(val, ":")
		if len(parts) >= 2 {
			targetID := parts[1]
			// Revert target status to READY (IDLE)
			database.DB.Model(&models.Target{}).Where("id = ?", targetID).Updates(map[string]interface{}{
				"status":        "READY",
				"current_phase": "IDLE",
			})
		}
	}

	// Mark as deleted
	err = redisq.Client.LSet(ctx, redisq.QueueName, int64(index), "__DELETED__").Err()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to mark item"})
	}

	// Remove
	err = redisq.Client.LRem(ctx, redisq.QueueName, 0, "__DELETED__").Err()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to remove item"})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

// MoveItemToTop moves an item from index to the front (0)
func MoveItemToTop(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid index"})
	}

	ctx := context.Background()
	
	// Get value
	val, err := redisq.Client.LIndex(ctx, redisq.QueueName, int64(index)).Result()
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
	}

	// Remove (mark delete trick)
	err = redisq.Client.LSet(ctx, redisq.QueueName, int64(index), "__DELETED__").Err()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to mark item"})
	}
	redisq.Client.LRem(ctx, redisq.QueueName, 0, "__DELETED__")

	// Push to left (front)
	err = redisq.Client.LPush(ctx, redisq.QueueName, val).Err()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to push to front"})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

// MoveItemToBottom moves an item to the end
func MoveItemToBottom(c *fiber.Ctx) error {
	indexStr := c.Params("index")
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid index"})
	}

	ctx := context.Background()
	
	// Get value
	val, err := redisq.Client.LIndex(ctx, redisq.QueueName, int64(index)).Result()
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Item not found"})
	}

	// Remove
	redisq.Client.LSet(ctx, redisq.QueueName, int64(index), "__DELETED__")
	redisq.Client.LRem(ctx, redisq.QueueName, 0, "__DELETED__")

	// Push to right (back)
	err = redisq.Client.RPush(ctx, redisq.QueueName, val).Err()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to push to back"})
	}

	return c.JSON(fiber.Map{"status": "success"})
}

// ClearQueue removes all items
func ClearQueue(c *fiber.Ctx) error {
	// Reset all QUEUED targets to READY in database to prevent zombie states
	database.DB.Model(&models.Target{}).Where("status = ?", "QUEUED").Updates(map[string]interface{}{
		"status":        "READY",
		"current_phase": "IDLE",
	})

	err := redisq.Client.Del(context.Background(), redisq.QueueName).Err()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to clear queue"})
	}
	return c.JSON(fiber.Map{"status": "success"})
}
