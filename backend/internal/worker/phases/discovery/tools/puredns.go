package tools

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

func RunPureDNS(ctx Context, rootDomain string, wordlists []string) (map[string][]string, error) {
	if err := ctx.ensureCommandRunner(); err != nil {
		return nil, err
	}

	results := make(map[string][]string)
	if len(wordlists) == 0 {
		return results, nil
	}

	tempDir := ctx.TempDir
	if tempDir == "" {
		var err error
		tempDir, _, err = utils.GetTargetTempDir(ctx.TargetID)
		if err != nil {
			return nil, fmt.Errorf("failed to get temp dir: %v", err)
		}
	}

	resolversFile := filepath.Join(tempDir, "puredns_resolvers.txt")
	resolvers := trustedResolvers()
	if err := utils.WriteSliceToFile(resolversFile, resolvers); err != nil {
		return nil, fmt.Errorf("failed to write resolvers file: %v", err)
	}
	defer os.Remove(resolversFile)

	validWordlists := resolvePureDNSWordlistPaths(ctx, wordlists)

	if len(validWordlists) == 0 {
		log.Printf("⚠️ No valid PureDNS wordlists found for %s, skipping puredns\n", rootDomain)
		return results, nil
	}

	log.Printf("🔍 PureDNS will run %d wordlist(s) sequentially for %s\n", len(validWordlists), rootDomain)

	for idx, wlPath := range validWordlists {
		if ctx.stopped() {
			return nil, fmt.Errorf("process killed by user request")
		}

		baseName := filepath.Base(wlPath)
		ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS BRUTEFORCE WORDLIST %d/%d: %s", idx+1, len(validWordlists), baseName))
		ctx.heartbeat()

		log.Printf("🔍 Running PureDNS wordlist %d/%d for %s: %s\n", idx+1, len(validWordlists), rootDomain, wlPath)

		output, err := ctx.RunCommand(
			ctx.TargetID,
			"puredns",
			"bruteforce",
			wlPath,
			rootDomain,
			"-r", resolversFile,
			"--trusted-only",
			"--quiet",
		)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return nil, err
			}
			log.Printf("⚠️ PureDNS failed for wordlist %s: %v\n", wlPath, err)
			continue
		}

		ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS PARSING WORDLIST %d/%d: %s", idx+1, len(validWordlists), baseName))
		ctx.heartbeat()

		subdomains := parsePureDNSSubdomains(ctx, string(output), rootDomain)
		if len(subdomains) == 0 {
			log.Printf("ℹ️ PureDNS wordlist %s produced no candidates for %s\n", wlPath, rootDomain)
			continue
		}

		ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS DNSX VALIDATION WORDLIST %d/%d: %s", idx+1, len(validWordlists), baseName))
		ctx.heartbeat()

		wordlistResults := resolvePureDNSSubdomains(ctx, tempDir, subdomains, idx+1)
		mergePureDNSLiveResults(results, wordlistResults)

		log.Printf(
			"✅ PureDNS wordlist %d/%d completed for %s: candidates=%d resolved_total=%d\n",
			idx+1,
			len(validWordlists),
			rootDomain,
			len(subdomains),
			len(results),
		)
	}

	ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS DONE (%d RESOLVED)", len(results)))
	ctx.heartbeat()
	log.Printf("✅ PureDNS found %d live subdomains for %s across %d wordlist(s)\n", len(results), rootDomain, len(validWordlists))

	return results, nil
}

type pureDNSWordlistPathRow struct {
	StoragePath string `gorm:"column:storage_path"`
}

func resolvePureDNSWordlistPaths(ctx Context, wordlists []string) []string {
	seen := make(map[string]bool)
	out := make([]string, 0, len(wordlists))

	add := func(raw, resolved string) {
		resolved = strings.TrimSpace(resolved)
		if resolved == "" || seen[resolved] {
			return
		}

		info, err := os.Stat(resolved)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("⚠️ PureDNS wordlist not found: raw=%s resolved=%s, skipping...\n", raw, resolved)
				return
			}
			log.Printf("⚠️ Failed to stat PureDNS wordlist raw=%s resolved=%s: %v, skipping...\n", raw, resolved, err)
			return
		}

		if info.IsDir() {
			log.Printf("⚠️ PureDNS wordlist is a directory: raw=%s resolved=%s, skipping...\n", raw, resolved)
			return
		}

		seen[resolved] = true
		out = append(out, resolved)
	}

	for _, raw := range wordlists {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}

		// System/default wordlists and already-absolute custom storage paths.
		if filepath.IsAbs(raw) {
			add(raw, raw)
			continue
		}

		// Account custom puredns path, e.g. user:1/my-list.txt.
		if strings.HasPrefix(raw, "user:") {
			if resolved := resolveUserPureDNSWordlistToken(raw); resolved != "" {
				add(raw, resolved)
			} else {
				log.Printf("⚠️ Could not resolve PureDNS custom wordlist token: %s\n", raw)
			}
			continue
		}

		// Fallback for older UI/DB values that stored only the display name.
		if resolved := resolveTargetOwnerPureDNSWordlistName(ctx.TargetID, raw); resolved != "" {
			add(raw, resolved)
			continue
		}

		// Final fallback: allow relative paths that exist from current process cwd.
		add(raw, raw)
	}

	return out
}

func resolveUserPureDNSWordlistToken(token string) string {
	value := strings.TrimPrefix(strings.TrimSpace(token), "user:")
	parts := strings.SplitN(value, "/", 2)
	if len(parts) != 2 {
		return ""
	}

	userIDRaw := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if userIDRaw == "" || name == "" {
		return ""
	}

	var userID uint
	if _, err := fmt.Sscanf(userIDRaw, "%d", &userID); err != nil || userID == 0 {
		return ""
	}

	var row pureDNSWordlistPathRow
	err := database.DB.Raw(`
		SELECT storage_path
		FROM user_wordlists
		WHERE deleted_at IS NULL
		  AND user_id = ?
		  AND (name = ? OR storage_path = ? OR storage_path LIKE ?)
		ORDER BY id DESC
		LIMIT 1
	`, userID, name, name, "%/"+name).Scan(&row).Error
	if err != nil {
		log.Printf("⚠️ Failed to resolve PureDNS custom wordlist token %s: %v\n", token, err)
		return ""
	}

	return strings.TrimSpace(row.StoragePath)
}

func resolveTargetOwnerPureDNSWordlistName(targetID uint, name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	var target struct {
		CreatedByUserID uint `gorm:"column:created_by_user_id"`
	}

	if err := database.DB.Table("targets").
		Select("created_by_user_id").
		Where("id = ?", targetID).
		Scan(&target).Error; err != nil {
		log.Printf("⚠️ Failed to resolve target owner for PureDNS wordlist %s: %v\n", name, err)
		return ""
	}

	if target.CreatedByUserID == 0 {
		return ""
	}

	var row pureDNSWordlistPathRow
	err := database.DB.Raw(`
		SELECT storage_path
		FROM user_wordlists
		WHERE deleted_at IS NULL
		  AND user_id = ?
		  AND (name = ? OR storage_path = ? OR storage_path LIKE ?)
		ORDER BY id DESC
		LIMIT 1
	`, target.CreatedByUserID, name, name, "%/"+name).Scan(&row).Error
	if err != nil {
		log.Printf("⚠️ Failed to resolve target-owner PureDNS wordlist %s: %v\n", name, err)
		return ""
	}

	return strings.TrimSpace(row.StoragePath)
}

func trustedResolvers() []string {
	resolversStr := os.Getenv("TRUSTED_RESOLVERS")
	if resolversStr == "" {
		resolversStr = "1.1.1.1,8.8.8.8,1.0.0.1,8.8.4.4"
	}

	parts := strings.Split(resolversStr, ",")
	out := make([]string, 0, len(parts))
	for _, r := range parts {
		r = strings.TrimSpace(r)
		if r != "" {
			out = append(out, r)
		}
	}
	return out
}

func mergePureDNSLiveResults(dst map[string][]string, src map[string][]string) {
	for host, ips := range src {
		host = strings.TrimSpace(host)
		if host == "" || len(ips) == 0 {
			continue
		}

		seen := make(map[string]bool)
		for _, ip := range dst[host] {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				seen[ip] = true
			}
		}
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				seen[ip] = true
			}
		}

		merged := make([]string, 0, len(seen))
		for ip := range seen {
			merged = append(merged, ip)
		}

		dst[host] = merged
	}
}

func parsePureDNSSubdomains(ctx Context, output, rootDomain string) []string {
	subdomains := []string{}

	for _, line := range strings.Split(output, "\n") {
		normalized := ctx.normalize(strings.TrimSpace(line), rootDomain)
		if normalized != "" {
			subdomains = append(subdomains, normalized)
		}
	}

	return subdomains
}

func resolvePureDNSSubdomains(ctx Context, tempDir string, subdomains []string, batchNo int) map[string][]string {
	results := make(map[string][]string)
	if len(subdomains) == 0 {
		return results
	}

	purednsInputFile := filepath.Join(tempDir, fmt.Sprintf("puredns_dnsx_%06d_input.txt", batchNo))
	if err := utils.WriteSliceToFile(purednsInputFile, subdomains); err != nil {
		log.Printf("⚠️ Failed to write puredns input for dnsx: %v\n", err)
		for _, subdomain := range subdomains {
			results[subdomain] = []string{}
		}
		return results
	}

	purednsDNSXOutputFile := filepath.Join(tempDir, fmt.Sprintf("puredns_dnsx_%06d_output.json", batchNo))
	dnsxResults, dnsxErr := RunDNSX(ctx, purednsInputFile, purednsDNSXOutputFile)
	if dnsxErr == nil {
		results = dnsxResults
	} else {
		for _, subdomain := range subdomains {
			results[subdomain] = []string{}
		}
	}

	_ = os.Remove(purednsInputFile)
	_ = os.Remove(purednsDNSXOutputFile)

	return results
}
