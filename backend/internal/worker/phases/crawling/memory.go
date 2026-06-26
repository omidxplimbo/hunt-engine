package crawling

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strconv"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type crawlURLSourceCount struct {
	Source string
	Count  int64
}

func memoryJSON(v interface{}, fallback string) datatypes.JSON {
	b, err := json.Marshal(v)
	if err != nil {
		return datatypes.JSON([]byte(fallback))
	}
	return datatypes.JSON(b)
}

func crawlingMemoryHash(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func ingestCrawlingMemory(targetID uint, rootDomain string) {
	var target models.Target
	if err := database.DB.First(&target, targetID).Error; err != nil {
		log.Printf("⚠️ Failed to fetch target for crawling memory ingest target=%d: %v", targetID, err)
		return
	}

	var totalURLs int64
	_ = database.DB.Model(&models.FoundURL{}).
		Where("target_id = ?", targetID).
		Count(&totalURLs).Error

	sourceCounts := make([]crawlURLSourceCount, 0)
	_ = database.DB.Model(&models.FoundURL{}).
		Select("source, COUNT(*) AS count").
		Where("target_id = ?", targetID).
		Group("source").
		Order("count DESC").
		Scan(&sourceCounts).Error

	urls := make([]models.FoundURL, 0)
	_ = database.DB.
		Where("target_id = ?", targetID).
		Order("created_at DESC, occurrence_count DESC, id DESC").
		Limit(25).
		Find(&urls).Error

	sourceMap := map[string]int64{}
	lines := []string{
		"Crawling completed URL inventory:",
		"Root domain: " + rootDomain,
		"Total URLs: " + strconv.FormatInt(totalURLs, 10),
		"Sources:",
	}

	for _, row := range sourceCounts {
		source := strings.TrimSpace(row.Source)
		if source == "" {
			source = "unknown"
		}
		sourceMap[source] = row.Count
		lines = append(lines, "- "+source+": "+strconv.FormatInt(row.Count, 10))
	}

	lines = append(lines, "Representative URLs:")
	sampleURLs := make([]string, 0, len(urls))
	for _, u := range urls {
		value := strings.TrimSpace(u.Value)
		if value == "" {
			continue
		}
		sampleURLs = append(sampleURLs, value)
		lines = append(lines, "- "+value+" source="+u.Source+" occurrences="+strconv.Itoa(u.OccurrenceCount))
	}

	if totalURLs == 0 {
		lines = append(lines, "No URLs were collected. Operator should consider rerunning crawling with additional tools or validating live assets first.")
	} else {
		lines = append(lines, "Operator value: crawled URLs are available for endpoint review, parameter discovery, OWASP validation planning, JS intelligence, and controlled follow-up tests.")
	}

	ownerKey := featureflags.OwnerKeyForUserID(target.CreatedByUserID)
	summary := "Crawling completed for " + rootDomain + " with " + strconv.FormatInt(totalURLs, 10) + " collected URLs."
	item := models.TargetMemoryItem{
		UserID:     target.CreatedByUserID,
		OwnerKey:   ownerKey,
		TargetID:   targetID,
		SourceType: models.TargetMemorySourceURL,
		MemoryType: models.TargetMemoryTypeEndpointNote,
		Title:      "Crawling URL inventory summary",
		Content:    strings.Join(lines, "\n"),
		Summary:    summary,
		Tags:       memoryJSON([]string{"crawling", "urls", "endpoints", "operator", "attack_surface"}, "[]"),
		Importance: 86,
		Confidence: 86,
		SourceHash: crawlingMemoryHash("crawling_url_inventory", rootDomain, strconv.FormatInt(totalURLs, 10)),
		Metadata: memoryJSON(map[string]interface{}{
			"ingest_version": "crawling-memory-ingest-v1",
			"event":          "crawling_completed",
			"total_urls":     totalURLs,
			"sources":        sourceMap,
			"sample_urls":    sampleURLs,
			"operator_value": "new crawled URLs available for endpoint review, parameter discovery, OWASP validation planning, JS intelligence, and controlled follow-up tests",
		}, "{}"),
	}

	var existing models.TargetMemoryItem
	err := database.DB.
		Where("target_id = ? AND owner_key = ? AND source_hash = ?", targetID, ownerKey, item.SourceHash).
		First(&existing).Error

	if err == nil {
		item.ID = existing.ID
		if updateErr := database.DB.Model(&existing).Updates(map[string]interface{}{
			"title":       item.Title,
			"content":     item.Content,
			"summary":     item.Summary,
			"tags":        item.Tags,
			"importance":  item.Importance,
			"confidence":  item.Confidence,
			"metadata":    item.Metadata,
			"source_type": item.SourceType,
			"memory_type": item.MemoryType,
		}).Error; updateErr != nil {
			log.Printf("⚠️ Failed to update crawling memory item target=%d: %v", targetID, updateErr)
			return
		}
		log.Printf("🧠 Updated crawling memory item target=%d urls=%d", targetID, totalURLs)
		return
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		log.Printf("⚠️ Failed to lookup crawling memory item target=%d: %v", targetID, err)
		return
	}

	if err := database.DB.Create(&item).Error; err != nil {
		log.Printf("⚠️ Failed to create crawling memory item target=%d: %v", targetID, err)
		return
	}

	log.Printf("🧠 Created crawling memory item target=%d urls=%d", targetID, totalURLs)
}
