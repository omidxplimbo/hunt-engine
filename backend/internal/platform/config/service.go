package config

import (
	"strconv"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

// GetMaxConcurrentScans retrieves the maximum number of concurrent scans allowed
func GetMaxConcurrentScans() int {
	var config models.SystemConfig
	if err := database.DB.First(&config, "key = ?", "max_concurrent_scans").Error; err != nil {
		// Default to 2 if not set
		return 2
	}
	val, err := strconv.Atoi(config.Value)
	if err != nil || val <= 0 {
		return 2
	}
	return val
}
