package passive

import (
	"fmt"
	"log"
	"os"
	"sort"
	"sync"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	legacydiscovery "github.com/omidxplimbo/hunt-engine/backend/internal/worker/discovery"
)

// Collect runs all enabled passive discovery tools and returns a unique subdomain list
// plus per-subdomain source attribution.
func Collect(ctx Context) ([]string, map[string][]string, error) {
	var target models.Target
	if err := database.DB.First(&target, ctx.TargetID).Error; err != nil {
		log.Printf("⚠️ Failed to fetch target config: %v\n", err)
		target.UseCero = false
		target.UseCrtsh = false
		target.UseAbusedb = false
	}

	subfinderProviderConfig, err := legacydiscovery.WriteSubfinderProviderConfigFile(ctx.TargetID, target.CreatedByUserID)
	if err != nil {
		log.Printf("⚠️ Failed to build subfinder provider-config (user_id=%d): %v\n", target.CreatedByUserID, err)
		subfinderProviderConfig = ""
	}
	if subfinderProviderConfig != "" {
		log.Printf(" Subfinder provider-config loaded for user_id=%d\n", target.CreatedByUserID)
		defer func() { _ = os.Remove(subfinderProviderConfig) }()
	}

	var wg sync.WaitGroup
	results := make(chan SourceResult, 50000)

	emit := func(source string, values []string) {
		for _, value := range values {
			normalized := NormalizeSubdomain(value, ctx.Domain)
			if normalized != "" {
				results <- SourceResult{Subdomain: normalized, Source: source}
			}
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		emit("subfinder", RunSubfinder(ctx, subfinderProviderConfig))
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		emit("assetfinder", RunAssetfinder(ctx))
	}()

	if target.UseCero {
		wg.Add(1)
		log.Printf(" Starting CERO for %s (SSL certificate scraping)...\n", ctx.Domain)
		go func() {
			defer wg.Done()
			ceroResults := RunCero(ctx)
			log.Printf("✅ CERO found %d domains for %s\n", len(ceroResults), ctx.Domain)
			emit("cero", ceroResults)
		}()
	} else {
		log.Printf("⏩ Skipping CERO for %s (Disabled in settings)\n", ctx.Domain)
	}

	if target.UseCrtsh {
		wg.Add(1)
		log.Printf(" Starting CRT.SH API query for %s...\n", ctx.Domain)
		go func() {
			defer wg.Done()
			crtshResults := RunCrtsh(ctx.Domain)
			log.Printf("✅ CRT.SH found %d domains for %s\n", len(crtshResults), ctx.Domain)
			emit("crtsh", crtshResults)
		}()
	} else {
		log.Printf("⏩ Skipping CRT.SH for %s (Disabled in settings)\n", ctx.Domain)
	}

	if target.UseAbusedb {
		wg.Add(1)
		log.Printf(" Starting AbuseDB scraping for %s...\n", ctx.Domain)
		go func() {
			defer wg.Done()
			emit("abusedb", RunAbuseDB(ctx))
		}()
	} else {
		log.Printf("⏩ Skipping AbuseDB for %s (Disabled in settings)\n", ctx.Domain)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	sourcesMap := make(map[string]map[string]bool)
	var finalSlice []string

	for res := range results {
		if res.Subdomain == "" || !isSubdomainOf(res.Subdomain, ctx.Domain) {
			continue
		}

		if _, exists := sourcesMap[res.Subdomain]; !exists {
			sourcesMap[res.Subdomain] = make(map[string]bool)
			finalSlice = append(finalSlice, res.Subdomain)
		}
		sourcesMap[res.Subdomain][res.Source] = true
	}

	sourcesListMap := make(map[string][]string)
	for subdomain, sourcesSet := range sourcesMap {
		var sources []string
		for source := range sourcesSet {
			sources = append(sources, source)
		}
		sort.Strings(sources)
		sourcesListMap[subdomain] = sources
	}

	var statusTarget models.Target
	database.DB.Select("status").First(&statusTarget, ctx.TargetID)
	if statusTarget.Status == "PAUSED" {
		return finalSlice, sourcesListMap, fmt.Errorf("process killed by user request")
	}

	return finalSlice, sourcesListMap, nil
}

func isSubdomainOf(subdomain, rootDomain string) bool {
	return NormalizeSubdomain(subdomain, rootDomain) != ""
}
