package engine

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

type lockMeta struct {
	PID       int   `json:"pid"`
	CreatedAt int64 `json:"created_at"`
}

const staleLockAfter = 30 * time.Minute

// AcquireTargetLock prevents multiple worker jobs from mutating the same target
// workspace/checkpoint files at the same time.
func AcquireTargetLock(targetID uint) (func(), bool) {
	lockDir := filepath.Join(utils.WorkerTempRoot, "locks")
	_ = os.MkdirAll(lockDir, 0o755)

	lockPath := filepath.Join(lockDir, fmt.Sprintf("target_%d.lock", targetID))
	metaPath := filepath.Join(lockPath, "meta.json")

	tryAcquire := func() (func(), bool) {
		if err := os.Mkdir(lockPath, 0o755); err != nil {
			return func() {}, false
		}

		meta := lockMeta{PID: os.Getpid(), CreatedAt: time.Now().Unix()}
		if payload, err := json.Marshal(meta); err == nil {
			_ = os.WriteFile(metaPath, payload, 0o644)
		}

		return func() {
			_ = os.RemoveAll(lockPath)
		}, true
	}

	if release, ok := tryAcquire(); ok {
		return release, true
	}

	createdAt := lockCreatedAt(lockPath, metaPath)
	if !createdAt.IsZero() && time.Since(createdAt) > staleLockAfter {
		log.Printf(" Removing stale lock for target %d (age: %s)\n", targetID, time.Since(createdAt))
		_ = os.RemoveAll(lockPath)
		if release, ok := tryAcquire(); ok {
			return release, true
		}
	}

	return func() {}, false
}

func lockCreatedAt(lockPath, metaPath string) time.Time {
	if payload, err := os.ReadFile(metaPath); err == nil {
		var meta lockMeta
		if json.Unmarshal(payload, &meta) == nil && meta.CreatedAt > 0 {
			return time.Unix(meta.CreatedAt, 0)
		}
	}

	if info, err := os.Stat(lockPath); err == nil {
		return info.ModTime()
	}

	return time.Time{}
}

// ClearAllLocks removes stale target locks on worker startup.
func ClearAllLocks() {
	lockDir := filepath.Join(utils.WorkerTempRoot, "locks")
	if err := os.RemoveAll(lockDir); err != nil {
		log.Printf("⚠️ Failed to clear worker locks: %v\n", err)
	}
	_ = os.MkdirAll(lockDir, 0o755)
}
