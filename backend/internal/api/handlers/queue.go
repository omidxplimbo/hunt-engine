package handlers

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

type QueueItem struct {
	Index         int    `json:"index"`
	Payload       string `json:"payload"`
	QueueKey      string `json:"queue_key"`
	Module        string `json:"module"`
	TargetID      uint   `json:"target_id"`
	RootDomain    string `json:"root_domain"`
	TargetName    string `json:"target_name"`
	OwnerUsername string `json:"owner_username"`
}

func parseQueuePayload(payload string) (string, uint, string, bool) {
	parts := strings.SplitN(payload, ":", 3)
	if len(parts) < 3 {
		return "", 0, "", false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return "", 0, "", false
	}
	return parts[0], uint(id), parts[2], true
}

func getCurrentQueueKey(c *fiber.Ctx) (string, uint, error) {
	uid, err := currentUserID(c)
	if err != nil {
		return "", 0, err
	}
	return redisq.QueueNameForUser(uid), uid, nil
}

func targetBelongsToUser(targetID, userID uint) (*models.Target, error) {
	var target models.Target
	if err := database.DB.Where("id = ? AND created_by_user_id = ?", targetID, userID).First(&target).Error; err != nil {
		return nil, err
	}
	return &target, nil
}

func ownerUsername(userID uint) string {
	var u models.User
	if err := database.DB.Select("username").First(&u, userID).Error; err != nil {
		return ""
	}
	return u.Username
}

func GetQueueList(c *fiber.Ctx) error {
	ctx := context.Background()
	key, uid, err := getCurrentQueueKey(c)
	if err != nil {
		return err
	}

	items, err := redisq.Client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch queue"})
	}

	queueItems := make([]QueueItem, 0, len(items))
	for i, item := range items {
		module, targetID, rootDomain, ok := parseQueuePayload(item)
		if !ok {
			continue
		}
		target, err := targetBelongsToUser(targetID, uid)
		if err != nil {
			continue
		}
		queueItems = append(queueItems, QueueItem{
			Index:         i,
			Payload:       item,
			QueueKey:      key,
			Module:        module,
			TargetID:      targetID,
			RootDomain:    rootDomain,
			TargetName:    target.Name,
			OwnerUsername: ownerUsername(target.CreatedByUserID),
		})
	}

	return c.JSON(queueItems)
}

func RemoveFromQueue(c *fiber.Ctx) error {
	index, err := strconv.Atoi(c.Params("index"))
	if err != nil || index < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid index"})
	}

	ctx := context.Background()
	key, uid, err := getCurrentQueueKey(c)
	if err != nil {
		return err
	}

	val, err := redisq.Client.LIndex(ctx, key, int64(index)).Result()
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Queue item not found"})
	}

	_, targetID, _, ok := parseQueuePayload(val)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid queue payload"})
	}
	if _, err := targetBelongsToUser(targetID, uid); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Queue item not found"})
	}

	placeholder := fmt.Sprintf("__DELETED__:%d:%d", uid, index)
	if err := redisq.Client.LSet(ctx, key, int64(index), placeholder).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to mark item"})
	}
	if err := redisq.Client.LRem(ctx, key, 1, placeholder).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove item"})
	}

	database.DB.Model(&models.Target{}).
		Where("id = ? AND created_by_user_id = ? AND status = ?", targetID, uid, "QUEUED").
		Updates(map[string]interface{}{"status": "READY", "current_phase": "IDLE"})

	return c.JSON(fiber.Map{"status": "success", "message": "Queue item removed"})
}

func MoveItemToTop(c *fiber.Ctx) error {
	return moveItem(c, true)
}

func MoveItemToBottom(c *fiber.Ctx) error {
	return moveItem(c, false)
}

func moveItem(c *fiber.Ctx, toTop bool) error {
	index, err := strconv.Atoi(c.Params("index"))
	if err != nil || index < 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid index"})
	}
	ctx := context.Background()
	key, uid, err := getCurrentQueueKey(c)
	if err != nil {
		return err
	}

	val, err := redisq.Client.LIndex(ctx, key, int64(index)).Result()
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Queue item not found"})
	}
	_, targetID, _, ok := parseQueuePayload(val)
	if !ok {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid queue payload"})
	}
	if _, err := targetBelongsToUser(targetID, uid); err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Queue item not found"})
	}

	placeholder := fmt.Sprintf("__MOVED__:%d:%d", uid, index)
	if err := redisq.Client.LSet(ctx, key, int64(index), placeholder).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to mark item"})
	}
	if err := redisq.Client.LRem(ctx, key, 1, placeholder).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to remove item"})
	}
	if toTop {
		err = redisq.Client.LPush(ctx, key, val).Err()
	} else {
		err = redisq.Client.RPush(ctx, key, val).Err()
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update queue"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Queue priority updated"})
}

func ClearQueue(c *fiber.Ctx) error {
	ctx := context.Background()
	key, uid, err := getCurrentQueueKey(c)
	if err != nil {
		return err
	}

	items, err := redisq.Client.LRange(ctx, key, 0, -1).Result()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch queue"})
	}
	for _, item := range items {
		_, targetID, _, ok := parseQueuePayload(item)
		if !ok {
			continue
		}
		database.DB.Model(&models.Target{}).
			Where("id = ? AND created_by_user_id = ? AND status = ?", targetID, uid, "QUEUED").
			Updates(map[string]interface{}{"status": "READY", "current_phase": "IDLE"})
	}

	if err := redisq.Client.Del(ctx, key).Err(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to clear queue"})
	}
	return c.JSON(fiber.Map{"status": "success", "message": "Queue cleared"})
}
