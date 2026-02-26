package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

const WorkerTempRoot = "/tmp/hunt-engine"

func GetTargetTempDir(targetID uint) (string, string, error) {
	var t models.Target
	if err := database.DB.Select("id", "root_domain", "created_by_user_id").First(&t, targetID).Error; err != nil {
		return "", "", err
	}

	username := "admin"
	isAdminOwner := true
	if t.CreatedByUserID != 0 {
		var u models.User
		if err := database.DB.Select("id", "username", "role").First(&u, t.CreatedByUserID).Error; err == nil {
			username = u.Username
			isAdminOwner = strings.ToLower(strings.TrimSpace(u.Role)) == "admin"
		}
	}

	baseDir := filepath.Join(WorkerTempRoot, "admin")
	if !isAdminOwner {
		baseDir = filepath.Join(WorkerTempRoot, SanitizePathComponent(username))
	}

	td := filepath.Join(baseDir, targetFolderName(t.RootDomain, targetID))
	if err := os.MkdirAll(td, 0o755); err != nil {
		return "", "", err
	}
	return td, username, nil
}

func SanitizePathComponent(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return "unknown"
	}
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_' || r == '.':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func targetFolderName(rootDomain string, targetID uint) string {
	rootDomain = strings.TrimSpace(rootDomain)
	if rootDomain == "" {
		return fmt.Sprintf("target_%d", targetID)
	}
	// مطابق مثال: test.com => test
	label := strings.Split(rootDomain, ".")[0]
	label = strings.TrimSpace(label)
	if label == "" {
		label = rootDomain
	}
	return SanitizePathComponent(label)
}
