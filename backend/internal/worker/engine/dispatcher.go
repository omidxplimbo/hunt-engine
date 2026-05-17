package engine

import (
	"fmt"
	"log"
	"strings"
	"time"
)

// JobHandlers maps worker job types to phase handlers.
type JobHandlers struct {
	Discovery func(targetID uint, rootDomain string)
	Probing   func(targetID uint, rootDomain string)
	Crawling  func(targetID uint, rootDomain string)
}

// LockFunc acquires a target-level lock and returns a release function.
type LockFunc func(targetID uint) (release func(), ok bool)

// ProcessJob parses a queued payload, acquires the target lock, and dispatches it
// to the correct phase handler. It intentionally preserves the previous worker
// behavior while moving dispatcher orchestration out of runner.go.
func ProcessJob(payload string, acquireTargetLock LockFunc, handlers JobHandlers) {
	parts := strings.Split(payload, ":")
	if len(parts) < 3 {
		log.Printf("⚠️ Invalid job payload format: %s\n", payload)
		return
	}

	jobType := parts[0]
	targetIDStr := parts[1]
	rootDomain := parts[2]

	var targetID uint
	fmt.Sscanf(targetIDStr, "%d", &targetID)

	log.Printf(" Worker received job type: %s for target ID: %d\n", jobType, targetID)

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

	defer func() {
		if r := recover(); r != nil {
			log.Printf(" CRITICAL PANIC in job dispatcher for target %d: %v\n", targetID, r)
		}
	}()

	switch jobType {
	case "DISCOVERY":
		if handlers.Discovery != nil {
			handlers.Discovery(targetID, rootDomain)
		}
	case "PROBING":
		if handlers.Probing != nil {
			handlers.Probing(targetID, rootDomain)
		}
	case "CRAWLING":
		if handlers.Crawling != nil {
			handlers.Crawling(targetID, rootDomain)
		}
	default:
		log.Printf("⚠️ Unknown job type: %s\n", jobType)
	}
}
