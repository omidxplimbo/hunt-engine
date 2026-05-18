package crawling

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"log"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

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
	now := time.Now()
	countSkippedCache := 0
	countSkippedExt := 0
	countUpdatedSource := 0
	countMergedCanonical := 0
	redisKey := fmt.Sprintf("target:%d:crawled_urls", targetID)

	for rawURL, source := range urls {
		rawURL = strings.TrimSpace(rawURL)
		if rawURL == "" {
			continue
		}

		cleanURL := strings.Split(rawURL, "?")[0]
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

		canonicalValue, canonicalHash := canonicalizeCrawledURL(rawURL)
		if canonicalValue == "" || canonicalHash == "" {
			canonicalValue = rawURL
			canonicalHash = hashCanonicalURL(rawURL)
		}

		var existingURL models.FoundURL
		result := database.DB.Where("target_id = ? AND canonical_hash = ?", targetID, canonicalHash).First(&existingURL)

		// Backward compatibility for URLs saved before canonical_hash existed.
		if result.Error != nil {
			result = database.DB.Where("target_id = ? AND value = ?", targetID, rawURL).First(&existingURL)
		}

		if result.Error == nil {
			changed := false

			if mergeURLSource(&existingURL, source) {
				countUpdatedSource++
				changed = true
			}

			if existingURL.CanonicalValue == "" {
				existingURL.CanonicalValue = canonicalValue
				changed = true
			}

			if existingURL.CanonicalHash == "" {
				existingURL.CanonicalHash = canonicalHash
				changed = true
			}

			existingURL.OccurrenceCount++
			existingURL.LastSeen = &now
			changed = true

			if changed {
				database.DB.Save(&existingURL)
			}

			countSkippedCache++
			countMergedCanonical++
			redisq.Client.SAdd(context.Background(), redisKey, canonicalValue)
			continue
		}

		newURLs = append(newURLs, models.FoundURL{
			TargetID:        targetID,
			Value:           rawURL,
			CanonicalValue:  canonicalValue,
			CanonicalHash:   canonicalHash,
			OccurrenceCount: 1,
			LastSeen:        &now,
			Source:          source,
		})

		redisq.Client.SAdd(context.Background(), redisKey, canonicalValue)

		if !silent {
			telegram.SendNewURLAlert(rootDomain, rawURL, source)
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
		log.Printf("✅ Saved %d NEW canonical URLs to DB.", len(newURLs))
	} else {
		log.Println("✅ No new URLs to save.")
	}

	if countUpdatedSource > 0 {
		log.Printf(" Updated sources for %d existing URLs.", countUpdatedSource)
	}

	log.Printf(
		" Stats: Total Found: %d | Ignored (Ext): %d | Exists/Merged: %d | Canonical Merges: %d | New: %d",
		len(urls),
		countSkippedExt,
		countSkippedCache,
		countMergedCanonical,
		len(newURLs),
	)
}

func mergeURLSource(existingURL *models.FoundURL, source string) bool {
	source = strings.TrimSpace(source)
	if source == "" {
		return false
	}

	currentSources := strings.Split(existingURL.Source, ",")
	for _, current := range currentSources {
		if strings.TrimSpace(current) == source {
			return false
		}
	}

	if strings.TrimSpace(existingURL.Source) == "" {
		existingURL.Source = source
	} else {
		existingURL.Source = existingURL.Source + ", " + source
	}

	return true
}

func canonicalizeCrawledURL(rawURL string) (string, string) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL, hashCanonicalURL(rawURL)
	}

	if parsed.Scheme == "" || parsed.Host == "" {
		return rawURL, hashCanonicalURL(rawURL)
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Fragment = ""

	cleanPath := path.Clean(parsed.EscapedPath())
	if cleanPath == "." || cleanPath == "" {
		cleanPath = "/"
	}

	if cleanPath != "/" {
		cleanPath = strings.TrimRight(cleanPath, "/")
		if cleanPath == "" {
			cleanPath = "/"
		}
	}

	queryKeys := canonicalQueryKeys(parsed.Query())
	canonical := parsed.Scheme + "://" + parsed.Host + cleanPath
	if len(queryKeys) > 0 {
		canonical += "?" + strings.Join(queryKeys, "&")
	}

	return canonical, hashCanonicalURL(canonical)
}

func canonicalQueryKeys(values url.Values) []string {
	seen := make(map[string]struct{})
	keys := make([]string, 0, len(values))

	for key := range values {
		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		if normalizedKey == "" || isVolatileURLParam(normalizedKey) {
			continue
		}

		if _, exists := seen[normalizedKey]; exists {
			continue
		}

		seen[normalizedKey] = struct{}{}
		keys = append(keys, url.QueryEscape(normalizedKey))
	}

	sort.Strings(keys)

	return keys
}

func isVolatileURLParam(key string) bool {
	volatileExact := map[string]struct{}{
		"fbclid":  {},
		"gclid":   {},
		"msclkid": {},
		"_":       {},
	}

	if _, ok := volatileExact[key]; ok {
		return true
	}

	volatileContains := []string{
		"csrf",
		"xsrf",
		"nonce",
		"token",
		"session",
		"sid",
		"signature",
		"sig",
		"timestamp",
		"cachebuster",
		"captcha",
		"rand",
		"random",
		"utm_",
		"org.apache.catalina.filters.csrf_nonce",
	}

	for _, marker := range volatileContains {
		if strings.Contains(key, marker) {
			return true
		}
	}

	// Very short time/cache keys are common noise, but avoid removing meaningful names like "title".
	if key == "ts" || key == "t" || key == "time" || key == "cache" {
		return true
	}

	return false
}

func hashCanonicalURL(value string) string {
	h := sha1.New()
	_, _ = h.Write([]byte(value))
	return hex.EncodeToString(h.Sum(nil))
}
