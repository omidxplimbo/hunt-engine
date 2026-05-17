package runtime

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

// TargetToolTmpDir returns a per-target temp directory for external tools.
func TargetToolTmpDir(targetID uint) string {
	td, _, err := utils.GetTargetTempDir(targetID)
	if err != nil || td == "" {
		return ""
	}

	tmpDir := filepath.Join(td, "_tmp")
	_ = os.MkdirAll(tmpDir, 0o755)
	return tmpDir
}

// EnvWithTargetTmp inherits the current environment and pins TMPDIR/TMP/TEMP
// to the target workspace so tools do not collide through global /tmp folders.
func EnvWithTargetTmp(targetID uint) []string {
	env := append([]string{}, os.Environ()...)
	tmpDir := TargetToolTmpDir(targetID)
	if tmpDir == "" {
		return env
	}

	env = append(env,
		"TMPDIR="+tmpDir,
		"TMP="+tmpDir,
		"TEMP="+tmpDir,
	)
	return env
}

// CleanupTargetTempDir removes only the selected target workspace.
func CleanupTargetTempDir(targetID uint) {
	td, _, err := utils.GetTargetTempDir(targetID)
	if err != nil {
		return
	}
	_ = os.RemoveAll(td)
}

// CleanupLegacyToolTmp removes stale legacy tool temp directories that were
// created directly under /tmp before per-target TMPDIR isolation existed.
func CleanupLegacyToolTmp() {
	entries, err := os.ReadDir("/tmp")
	if err != nil {
		return
	}

	cutoff := time.Now().Add(-30 * time.Minute)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		name := e.Name()
		if !strings.HasPrefix(name, "httpx") {
			continue
		}

		full := filepath.Join("/tmp", name)
		if strings.HasPrefix(full, utils.WorkerTempRoot) {
			continue
		}

		fi, err := e.Info()
		if err != nil {
			continue
		}

		if fi.ModTime().Before(cutoff) {
			_ = os.RemoveAll(full)
		}
	}
}

// ClearAllLocks removes stale target lock directories on worker startup.
func ClearAllLocks() {
	lockDir := filepath.Join(utils.WorkerTempRoot, "locks")
	if err := os.RemoveAll(lockDir); err != nil {
		log.Printf("⚠️ Failed to clear locks: %v\n", err)
	} else {
		log.Println(" Cleared all stale locks on startup.")
	}
}

// TargetLockPath is intentionally small and exported for future lock refactors.
func TargetLockPath(targetID uint) string {
	return filepath.Join(utils.WorkerTempRoot, "locks", fmt.Sprintf("target_%d.lock", targetID))
}
