package persistence

import (
	"encoding/json"
	"log"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	workertypes "github.com/omidxplimbo/hunt-engine/backend/internal/worker/types"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

// Context contains the runtime dependencies needed by Discovery persistence helpers.
type Context struct {
	TargetID   uint
	RunCommand func(targetID uint, name string, args ...string) ([]byte, error)
}

// RunCDNCheckForLiveAssets enriches live Discovery assets with CDN/WAF/Cloud metadata.
func RunCDNCheckForLiveAssets(ctx Context, dnsxResults map[string][]string, inputFile string) {
	if len(dnsxResults) == 0 {
		log.Printf("ℹ️ No live assets found for CDN check.\n")
		return
	}

	log.Printf("🔍 Starting CDN check for all live IPs from DNS validation...\n")

	ipSet := make(map[string]bool)
	for _, ips := range dnsxResults {
		for _, ip := range ips {
			if ip != "" {
				ipSet[ip] = true
			}
		}
	}

	if len(ipSet) == 0 {
		log.Printf("ℹ️ No IPs found for CDN check.\n")
		return
	}

	ipList := make([]string, 0, len(ipSet))
	for ip := range ipSet {
		ipList = append(ipList, ip)
	}

	log.Printf("🔍 Running cdncheck on %d unique IPs from %d live subdomains...\n", len(ipList), len(dnsxResults))

	cdnResults := RunCDNCheck(ctx, ipList, inputFile)
	if len(cdnResults) == 0 {
		log.Printf("⚠️ No CDN check results returned.\n")
		return
	}

	var allAssets []models.Asset
	database.DB.Where("target_id = ? AND is_live = ?", ctx.TargetID, true).Find(&allAssets)

	assetMap := make(map[string]*models.Asset)
	for i := range allAssets {
		assetMap[allAssets[i].Value] = &allAssets[i]
	}

	updatedCount := 0

	for subdomain, ips := range dnsxResults {
		asset, exists := assetMap[subdomain]
		if !exists {
			continue
		}

		hasCDN := false
		cdnName := ""
		hasWAF := false
		wafName := ""
		hasCloud := false
		cloudName := ""

		for _, ip := range ips {
			if cdnInfo, found := cdnResults[ip]; found {
				if cdnInfo.IsCDN {
					hasCDN = true
					if cdnInfo.CDNName != "" && cdnName == "" {
						cdnName = cdnInfo.CDNName
					}
				}
				if cdnInfo.IsWAF {
					hasWAF = true
					if cdnInfo.WAFName != "" && wafName == "" {
						wafName = cdnInfo.WAFName
					}
				}
				if cdnInfo.IsCloud {
					hasCloud = true
					if cdnInfo.CloudName != "" && cloudName == "" {
						cloudName = cdnInfo.CloudName
					}
				}
			}
		}

		updates := make(map[string]interface{})
		if asset.Cdncheck != hasCDN {
			updates["cdncheck"] = hasCDN
		}
		if hasCDN && asset.CdncheckName != cdnName {
			updates["cdncheck_name"] = cdnName
		} else if !hasCDN && asset.CdncheckName != "" {
			updates["cdncheck_name"] = ""
		}

		if asset.Wafcheck != hasWAF {
			updates["wafcheck"] = hasWAF
		}
		if hasWAF && asset.WafcheckName != wafName {
			updates["wafcheck_name"] = wafName
		} else if !hasWAF && asset.WafcheckName != "" {
			updates["wafcheck_name"] = ""
		}

		if asset.Cloudcheck != hasCloud {
			updates["cloudcheck"] = hasCloud
		}
		if hasCloud && asset.CloudcheckName != cloudName {
			updates["cloudcheck_name"] = cloudName
		} else if !hasCloud && asset.CloudcheckName != "" {
			updates["cloudcheck_name"] = ""
		}

		if len(updates) > 0 {
			database.DB.Model(asset).Updates(updates)
			updatedCount++
		}
	}

	log.Printf("✅ CDN check completed. Updated %d assets with cdncheck information.\n", updatedCount)
}

// RunCDNCheck executes cdncheck on a list of IPs and parses JSONL stdout.
func RunCDNCheck(ctx Context, ips []string, inputFile string) map[string]workertypes.CDNCheckResult {
	if len(ips) == 0 {
		return make(map[string]workertypes.CDNCheckResult)
	}

	if err := utils.WriteSliceToFile(inputFile, ips); err != nil {
		log.Printf("❌ Error writing cdncheck input file: %v\n", err)
		return make(map[string]workertypes.CDNCheckResult)
	}

	log.Printf("🔍 Running cdncheck on %d IPs...\n", len(ips))
	output, err := ctx.RunCommand(ctx.TargetID, "cdncheck",
		"-i", inputFile,
		"-j",
		"-silent",
		"-nc",
	)
	if err != nil {
		if err.Error() != "process killed by user request" {
			log.Printf("⚠️ Cdncheck error: %v\n", err)
		}
		return make(map[string]workertypes.CDNCheckResult)
	}

	if len(output) == 0 {
		log.Printf("⚠️ Cdncheck returned empty output.\n")
		return make(map[string]workertypes.CDNCheckResult)
	}

	results := make(map[string]workertypes.CDNCheckResult)
	lines := strings.Split(string(output), "\n")
	parsedCount := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "{") {
			continue
		}

		var rawResult map[string]interface{}
		if err := json.Unmarshal([]byte(line), &rawResult); err != nil {
			continue
		}

		ip, ok := rawResult["ip"].(string)
		if !ok || ip == "" {
			continue
		}

		result := results[ip]
		result.IP = ip

		if cdnVal, exists := rawResult["cdn"]; exists {
			switch v := cdnVal.(type) {
			case bool:
				result.IsCDN = result.IsCDN || v
			case string:
				result.IsCDN = result.IsCDN || (v == "true" || v == "1" || v == "yes")
			}
		}
		if cdnName, ok := rawResult["cdn_name"].(string); ok && cdnName != "" && result.CDNName == "" {
			result.CDNName = cdnName
		}

		if wafVal, exists := rawResult["waf"]; exists {
			switch v := wafVal.(type) {
			case bool:
				result.IsWAF = result.IsWAF || v
			case string:
				result.IsWAF = result.IsWAF || (v == "true" || v == "1" || v == "yes")
			}
		}
		if wafName, ok := rawResult["waf_name"].(string); ok && wafName != "" && result.WAFName == "" {
			result.WAFName = wafName
		}

		if cloudVal, exists := rawResult["cloud"]; exists {
			switch v := cloudVal.(type) {
			case bool:
				result.IsCloud = result.IsCloud || v
			case string:
				result.IsCloud = result.IsCloud || (v == "true" || v == "1" || v == "yes")
			}
		}
		if cloudName, ok := rawResult["cloud_name"].(string); ok && cloudName != "" && result.CloudName == "" {
			result.CloudName = cloudName
		}

		results[ip] = result
		if result.IsCDN || result.IsWAF || result.IsCloud {
			parsedCount++
		}
	}

	if parsedCount > 0 {
		log.Printf("✅ Parsed %d technology results from cdncheck output.\n", parsedCount)
	}

	return results
}
