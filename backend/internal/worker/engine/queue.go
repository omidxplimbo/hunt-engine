package engine

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
)

const roundRobinCursorKey = redisq.QueueName + ":round_robin_cursor"

// ParsePayloadTargetID extracts the target id from queue payloads formatted as
// MODULE:targetID:rootDomain.
func ParsePayloadTargetID(payload string) (uint, bool) {
	parts := strings.SplitN(payload, ":", 3)
	if len(parts) < 3 {
		return 0, false
	}

	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}

	return uint(id), true
}

// CountActiveScans returns the number of globally active scan targets.
func CountActiveScans() int64 {
	var count int64
	database.DB.Model(&models.Target{}).
		Where("status = ?", "SCANNING").
		Count(&count)
	return count
}

// CountActiveScansForUser returns active scans owned by the given user.
func CountActiveScansForUser(userID uint) int64 {
	var count int64
	database.DB.Model(&models.Target{}).
		Where("created_by_user_id = ? AND status = ?", userID, "SCANNING").
		Count(&count)
	return count
}

// UserHasFreeScanSlot enforces per-user scan concurrency for regular users.
// Admin users are only constrained by the global engine cap.
func UserHasFreeScanSlot(user models.User) bool {
	if strings.ToLower(strings.TrimSpace(user.Role)) == "admin" {
		return true
	}

	limit := user.MaxConcurrentScans
	if limit < 1 {
		limit = 1
	}

	return CountActiveScansForUser(user.ID) < int64(limit)
}

// RemoveQueuePayloadAt removes a queue item only if the payload at index still
// matches the item selected by the scheduler.
func RemoveQueuePayloadAt(ctx context.Context, key string, index int, payload string) error {
	placeholder := fmt.Sprintf("__RUNNING__:%d:%d", time.Now().UnixNano(), index)

	current, err := redisq.Client.LIndex(ctx, key, int64(index)).Result()
	if err != nil {
		return err
	}
	if current != payload {
		return fmt.Errorf("queue changed while selecting job")
	}

	if err := redisq.Client.LSet(ctx, key, int64(index), placeholder).Err(); err != nil {
		return err
	}

	return redisq.Client.LRem(ctx, key, 1, placeholder).Err()
}

func queueKeysWithLegacy(ctx context.Context) ([]string, error) {
	keys, err := redisq.UserQueueKeys(ctx)
	if err != nil {
		return nil, err
	}

	// Legacy fallback: drain old global queue if it still has jobs from before
	// the per-user queue migration.
	if n, _ := redisq.Client.LLen(ctx, redisq.QueueName).Result(); n > 0 {
		keys = append(keys, redisq.QueueName)
	}

	return keys, nil
}

func getRoundRobinCursor(ctx context.Context, keyCount int) int {
	if keyCount <= 0 {
		return 0
	}

	raw, err := redisq.Client.Get(ctx, roundRobinCursorKey).Int()
	if err != nil {
		return 0
	}

	if raw < 0 {
		raw = 0
	}

	return raw % keyCount
}

func setRoundRobinCursor(ctx context.Context, keyCount int, selectedIndex int) {
	if keyCount <= 0 {
		return
	}

	next := (selectedIndex + 1) % keyCount
	_ = redisq.Client.Set(ctx, roundRobinCursorKey, next, 0).Err()
}

func orderedQueueKeys(keys []string, start int) []struct {
	Index int
	Key   string
} {
	ordered := make([]struct {
		Index int
		Key   string
	}, 0, len(keys))

	if len(keys) == 0 {
		return ordered
	}

	start = start % len(keys)
	for offset := 0; offset < len(keys); offset++ {
		idx := (start + offset) % len(keys)
		ordered = append(ordered, struct {
			Index int
			Key   string
		}{
			Index: idx,
			Key:   keys[idx],
		})
	}

	return ordered
}

// PopNextEligiblePayload returns one runnable payload using fair round-robin
// selection across per-user queues. Each call starts from the queue after the
// last successfully selected queue, so busy users do not permanently starve
// later user queues.
func PopNextEligiblePayload(maxGlobal int) (string, error) {
	ctx := context.Background()

	keys, err := queueKeysWithLegacy(ctx)
	if err != nil {
		return "", err
	}
	if len(keys) == 0 {
		return "", nil
	}

	start := getRoundRobinCursor(ctx, len(keys))
	for _, queuedKey := range orderedQueueKeys(keys, start) {
		items, err := redisq.Client.LRange(ctx, queuedKey.Key, 0, -1).Result()
		if err != nil {
			return "", err
		}

		for idx, payload := range items {
			targetID, ok := ParsePayloadTargetID(payload)
			if !ok {
				_ = RemoveQueuePayloadAt(ctx, queuedKey.Key, idx, payload)
				continue
			}

			var target models.Target
			if err := database.DB.First(&target, targetID).Error; err != nil {
				_ = RemoveQueuePayloadAt(ctx, queuedKey.Key, idx, payload)
				continue
			}

			if target.Status != "QUEUED" {
				_ = RemoveQueuePayloadAt(ctx, queuedKey.Key, idx, payload)
				continue
			}

			var owner models.User
			if err := database.DB.First(&owner, target.CreatedByUserID).Error; err != nil {
				continue
			}

			if !UserHasFreeScanSlot(owner) {
				break
			}

			if CountActiveScans() >= int64(maxGlobal) {
				return "", nil
			}

			if err := RemoveQueuePayloadAt(ctx, queuedKey.Key, idx, payload); err != nil {
				return "", err
			}

			setRoundRobinCursor(ctx, len(keys), queuedKey.Index)
			return payload, nil
		}
	}

	return "", nil
}

// MarkPayloadTargetScanning updates the target status before the worker goroutine
// starts so the scheduler cannot overbook global or per-user slots.
func MarkPayloadTargetScanning(payload string) {
	targetID, ok := ParsePayloadTargetID(payload)
	if !ok {
		return
	}

	database.DB.Model(&models.Target{}).
		Where("id = ?", targetID).
		Update("status", "SCANNING")
}
