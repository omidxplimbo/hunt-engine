package engine

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	workerstate "github.com/omidxplimbo/hunt-engine/backend/internal/worker/state"
)

// JobHandlers maps worker job types to phase handlers.
type JobHandlers struct {
	Discovery func(targetID uint, rootDomain string)
	Probing   func(targetID uint, rootDomain string)
	Crawling  func(targetID uint, rootDomain string)
}

// LockFunc acquires a target-level lock and returns a release function.
type LockFunc func(targetID uint) (release func(), ok bool)

func failTarget(targetID uint, reason string) {
	if targetID == 0 {
		return
	}

	workerstate.MarkFailed(targetID, reason)
	_ = database.DB.Model(&models.Target{}).
		Where("id = ?", targetID).
		Updates(map[string]interface{}{
			"status":         "PAUSED",
			"current_phase":  "FAILED: CHECK WORKER LOGS",
			"stop_requested": false,
		}).Error
}

func parseJobPayload(payload string) (jobType string, targetID uint, rootDomain string, err error) {
	parts := strings.SplitN(strings.TrimSpace(payload), ":", 3)
	if len(parts) < 3 {
		return "", 0, "", fmt.Errorf("invalid job payload format: %s", payload)
	}

	jobType = strings.ToUpper(strings.TrimSpace(parts[0]))
	rootDomain = strings.TrimSpace(parts[2])

	parsedID, parseErr := strconv.ParseUint(strings.TrimSpace(parts[1]), 10, 64)
	if parseErr != nil || parsedID == 0 {
		return jobType, 0, rootDomain, fmt.Errorf("invalid target id in job payload %q: %w", payload, parseErr)
	}

	if jobType == "" || rootDomain == "" {
		return jobType, uint(parsedID), rootDomain, fmt.Errorf("invalid empty job fields in payload: %s", payload)
	}

	return jobType, uint(parsedID), rootDomain, nil
}

// ProcessJob parses a queued payload, acquires the target lock, and dispatches it
// to the correct phase handler. Invalid jobs, unknown job types, and panics are
// marked as failed so targets do not remain stuck in QUEUED or SCANNING.
func ProcessJob(payload string, acquireTargetLock LockFunc, handlers JobHandlers) {
	jobType, targetID, rootDomain, err := parseJobPayload(payload)
	if err != nil {
		log.Printf("⚠️ %v\n", err)
		failTarget(targetID, err.Error())
		return
	}

	log.Printf("Worker received job type: %s for target ID: %d\n", jobType, targetID)

	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("panic in job dispatcher for target %d: %v", targetID, r)
			log.Printf("CRITICAL PANIC in job dispatcher for target %d: %v\n", targetID, r)
			failTarget(targetID, errMsg)
		}
	}()

	releaseLock, ok := acquireTargetLock(targetID)
	if !ok {
		log.Printf("⏳ Target %d is already being processed. Waiting for lock...\n", targetID)
		for {
			time.Sleep(500 * time.Millisecond)
			releaseLock, ok = acquireTargetLock(targetID)
			if ok {
				break
			}
		}
	}
	defer releaseLock()

	switch jobType {
	case "DISCOVERY":
		if handlers.Discovery == nil {
			failTarget(targetID, "missing DISCOVERY handler")
			return
		}
		handlers.Discovery(targetID, rootDomain)
	case "PROBING":
		if handlers.Probing == nil {
			failTarget(targetID, "missing PROBING handler")
			return
		}
		handlers.Probing(targetID, rootDomain)
	case "CRAWLING":
		if handlers.Crawling == nil {
			failTarget(targetID, "missing CRAWLING handler")
			return
		}
		handlers.Crawling(targetID, rootDomain)
	default:
		errMsg := fmt.Sprintf("unknown job type: %s", jobType)
		log.Printf("⚠️ %s\n", errMsg)
		failTarget(targetID, errMsg)
	}
}
