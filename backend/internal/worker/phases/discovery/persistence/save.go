package persistence

import (
	"encoding/json"
	"log"
	"sort"
	"strconv"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
	"gorm.io/gorm"
)

// SaveContext contains callbacks that still belong to the worker state/history layer.
type SaveContext struct {
	LogChange func(tx *gorm.DB, assetID uint, field, oldVal, newVal string) error
}

func mergeSources(existingSourcesJSON string, newSources []string) string {
	var existingSources []string
	if existingSourcesJSON != "" && existingSourcesJSON != "[]" {
		_ = json.Unmarshal([]byte(existingSourcesJSON), &existingSources)
	}

	sourceMap := make(map[string]bool)
	for _, source := range existingSources {
		sourceMap[source] = true
	}
	for _, source := range newSources {
		sourceMap[source] = true
	}

	merged := make([]string, 0, len(sourceMap))
	for source := range sourceMap {
		merged = append(merged, source)
	}
	sort.Strings(merged)

	bytes, _ := json.Marshal(merged)
	return string(bytes)
}

// SaveDiscoveryResultsToDB persists Discovery output and emits change notifications.
func SaveDiscoveryResultsToDB(ctx SaveContext, target models.Target, masterList []string, liveResults map[string][]string, sourcesMap map[string][]string) {
	log.Printf(" Saving/Updating assets (Scan Count: %d)...", target.ScanCount)

	isFirstRun := target.ScanCount == 0
	countNew := 0
	countUpdated := 0
	chunkSize := 1000

	for i := 0; i < len(masterList); i += chunkSize {
		end := i + chunkSize
		if end > len(masterList) {
			end = len(masterList)
		}

		chunk := masterList[i:end]

		var existingAssets []models.Asset
		database.DB.Where("target_id = ? AND value IN ?", target.ID, chunk).Find(&existingAssets)

		existingMap := make(map[string]*models.Asset)
		for i := range existingAssets {
			existingMap[existingAssets[i].Value] = &existingAssets[i]
		}

		var toInsert []models.Asset

		for _, value := range chunk {
			ips, foundInDNSX := liveResults[value]
			hasIPs := foundInDNSX && len(ips) > 0
			isLive := hasIPs

			dnsxIPJSON := "[]"
			if hasIPs {
				bytes, _ := json.Marshal(ips)
				dnsxIPJSON = string(bytes)
			}

			newSources := sourcesMap[value]
			sourcesJSON := "[]"
			if len(newSources) > 0 {
				bytes, _ := json.Marshal(newSources)
				sourcesJSON = string(bytes)
			}

			if existing, ok := existingMap[value]; ok {
				mergedSourcesJSON := mergeSources(existing.Sources, newSources)
				updateData := make(map[string]interface{})
				needsUpdate := false

				if existing.IsLive != isLive {
					now := time.Now()
					if ctx.LogChange != nil {
						_ = ctx.LogChange(database.DB, existing.ID, "is_live", strconv.FormatBool(existing.IsLive), strconv.FormatBool(isLive))
					}

					if isLive && !existing.IsLive {
						if !isFirstRun {
							telegram.SendNewAssetAlert(target.RootDomain, value)
						}
					} else if !isLive {
						telegram.SendChangeAlert(target.RootDomain, value, "is_live", "true", "false")
					}

					updateData["is_live"] = isLive
					updateData["is_new"] = true
					updateData["last_change_at"] = &now
					updateData["dnsx_ip"] = dnsxIPJSON
					needsUpdate = true
				} else if isLive && existing.DnsxIP != dnsxIPJSON {
					updateData["dnsx_ip"] = dnsxIPJSON
					needsUpdate = true
				}

				if existing.Sources != mergedSourcesJSON {
					updateData["sources"] = mergedSourcesJSON
					needsUpdate = true
				}

				if needsUpdate {
					database.DB.Model(existing).Updates(updateData)
					countUpdated++
				}

				continue
			}

			newAsset := models.Asset{
				TargetID:     target.ID,
				Value:        value,
				Type:         "subdomain",
				IsNew:        true,
				IsLive:       isLive,
				Technologies: "[]",
				HostIP:       "[]",
				RawHttpx:     "{}",
				DnsxIP:       dnsxIPJSON,
				Sources:      sourcesJSON,
			}

			toInsert = append(toInsert, newAsset)

			if !isFirstRun && isLive {
				telegram.SendNewAssetAlert(target.RootDomain, value)
			}

			countNew++
		}

		if len(toInsert) > 0 {
			if err := database.DB.CreateInBatches(toInsert, 500).Error; err != nil {
				log.Printf("❌ Bulk insert failed: %v\n", err)
			}
		}
	}

	if countNew > 0 || countUpdated > 0 {
		cache.IncrementTargetVersion(target.ID)
	}

	log.Printf("✅ DB Sync Complete. New: %d, Status Changed: %d.\n", countNew, countUpdated)
}
