package crawling

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/cache"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/redisq"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/telegram"
)

func SaveURLs(targetID uint, rootDomain string, urls map[string]string, silent bool) {
	log.Printf(" Processing %d collected URLs...", len(urls))

	ignoredExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".svg", ".bmp", ".ico", ".webp",
		".woff", ".woff2", ".ttf", ".eot", ".otf", ".css",
	}

	var newURLs []models.FoundURL
	countSkippedCache := 0
	countSkippedExt := 0
	countUpdatedSource := 0
	redisKey := fmt.Sprintf("target:%d:crawled_urls", targetID)

	for u, source := range urls {
		cleanURL := strings.Split(u, "?")[0]
		isIgnored := false
		for _, ext := range ignoredExts {
			if strings.HasSuffix(strings.ToLower(cleanURL), ext) {
				isIgnored = true
				break
			}
		}
		if isIgnored {
			countSkippedExt++
			continue
		}

		var existingURL models.FoundURL
		result := database.DB.Where("target_id = ? AND value = ?", targetID, u).First(&existingURL)
		if result.Error == nil {
			currentSources := strings.Split(existingURL.Source, ", ")
			newSource := source
			sourceExists := false
			for _, s := range currentSources {
				if s == newSource {
					sourceExists = true
					break
				}
			}

			if !sourceExists {
				existingURL.Source = existingURL.Source + ", " + newSource
				database.DB.Save(&existingURL)
				countUpdatedSource++
			}
			countSkippedCache++
			continue
		}

		newURLs = append(newURLs, models.FoundURL{
			TargetID: targetID,
			Value:    u,
			Source:   source,
		})
		redisq.Client.SAdd(context.Background(), redisKey, u)

		if !silent {
			telegram.SendNewURLAlert(rootDomain, u, source)
		}
	}

	if len(newURLs) > 0 {
		batchSize := 500
		for i := 0; i < len(newURLs); i += batchSize {
			end := i + batchSize
			if end > len(newURLs) {
				end = len(newURLs)
			}
			if err := database.DB.CreateInBatches(newURLs[i:end], batchSize).Error; err != nil {
				log.Printf("⚠️ DB Insert Warning: %v\n", err)
			}
		}
		cache.IncrementTargetVersion(targetID)
		log.Printf("✅ Saved %d NEW URLs to DB.", len(newURLs))
	} else {
		log.Println("✅ No new URLs to save.")
	}

	if countUpdatedSource > 0 {
		log.Printf(" Updated sources for %d existing URLs.", countUpdatedSource)
	}
	log.Printf(" Stats: Total Found: %d | Ignored (Ext): %d | Exists/Skipped: %d | New: %d", len(urls), countSkippedExt, countSkippedCache, len(newURLs))
}
