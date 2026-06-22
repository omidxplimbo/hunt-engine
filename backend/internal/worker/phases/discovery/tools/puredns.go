package tools

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	workerruntime "github.com/omidxplimbo/hunt-engine/backend/internal/worker/runtime"
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

	publicResolversFile, publicResolversCount, publicCleanup, err := preparePureDNSPublicResolversFile(tempDir)
	if err != nil {
		return nil, err
	}
	if publicCleanup {
		defer os.Remove(publicResolversFile)
	}

	trustedResolversFile := filepath.Join(tempDir, "puredns_trusted_resolvers.txt")
	trusted := trustedResolvers()
	if err := utils.WriteSliceToFile(trustedResolversFile, trusted); err != nil {
		return nil, fmt.Errorf("failed to write trusted resolvers file: %v", err)
	}
	defer os.Remove(trustedResolversFile)

	if publicResolversCount <= len(trusted) {
		log.Printf("PureDNS public resolver list has only %d resolver(s); large bruteforce runs may be slow. Set PUREDNS_RESOLVERS_FILE to a larger resolver list.\n", publicResolversCount)
	}

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

		outputFile := filepath.Join(tempDir, fmt.Sprintf("puredns_%06d_output.txt", idx+1))
		_ = os.Remove(outputFile)

		args := []string{
			"bruteforce",
			wlPath,
			rootDomain,
			"-r", publicResolversFile,
			"--resolvers-trusted", trustedResolversFile,
			"--quiet",
			"--write", outputFile,
		}

		startedAt := time.Now()
		log.Printf(
			"🔍 Running PureDNS wordlist %d/%d for %s: wordlist=%s size_bytes=%d public_resolvers=%d trusted_resolvers=%d output=%s command=%s\n",
			idx+1,
			len(validWordlists),
			rootDomain,
			wlPath,
			fileSizeBytes(wlPath),
			publicResolversCount,
			len(trusted),
			outputFile,
			"puredns "+strings.Join(args, " "),
		)

		err := workerruntime.RunCommandNoCaptureWithKillSwitch(
			ctx.TargetID,
			func(uint) { ctx.heartbeat() },
			func(uint) bool { return ctx.stopped() },
			"puredns",
			args...,
		)
		duration := time.Since(startedAt)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return nil, err
			}
			log.Printf("⚠️ PureDNS failed for wordlist %s after %s: %v\n", wlPath, duration, err)
			continue
		}

		ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS PARSING WORDLIST %d/%d: %s", idx+1, len(validWordlists), baseName))
		ctx.heartbeat()

		subdomains := parsePureDNSSubdomainsFromFile(ctx, outputFile, rootDomain)
		_ = os.Remove(outputFile)
		if len(subdomains) == 0 {
			log.Printf("ℹ️ PureDNS wordlist %s produced no candidates for %s in %s\n", wlPath, rootDomain, duration)
			continue
		}

		if shouldValidatePureDNSWithDNSX() {
			ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS OPTIONAL DNSX VALIDATION WORDLIST %d/%d: %s", idx+1, len(validWordlists), baseName))
			ctx.heartbeat()

			wordlistResults := resolvePureDNSSubdomains(ctx, tempDir, subdomains, idx+1)
			mergePureDNSLiveResults(results, wordlistResults)
		} else {
			mergePureDNSResolvedHosts(results, subdomains)
		}

		log.Printf(
			"✅ PureDNS wordlist %d/%d completed for %s in %s: candidates=%d resolved_total=%d post_dnsx_validation=%t\n",
			idx+1,
			len(validWordlists),
			rootDomain,
			duration,
			len(subdomains),
			len(results),
			shouldValidatePureDNSWithDNSX(),
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

func preparePureDNSPublicResolversFile(tempDir string) (string, int, bool, error) {
	if path := strings.TrimSpace(os.Getenv("PUREDNS_RESOLVERS_FILE")); path != "" {
		count, err := countNonEmptyLines(path)
		if err != nil {
			return "", 0, false, fmt.Errorf("failed to read PUREDNS_RESOLVERS_FILE %s: %v", path, err)
		}
		if count == 0 {
			return "", 0, false, fmt.Errorf("PUREDNS_RESOLVERS_FILE %s is empty", path)
		}
		return path, count, false, nil
	}

	resolvers := pureDNSPublicResolvers()
	if len(resolvers) == 0 {
		resolvers = trustedResolvers()
	}

	path := filepath.Join(tempDir, "puredns_public_resolvers.txt")
	if err := utils.WriteSliceToFile(path, resolvers); err != nil {
		return "", 0, true, fmt.Errorf("failed to write public resolvers file: %v", err)
	}
	return path, len(resolvers), true, nil
}

func pureDNSPublicResolvers() []string {
	resolversStr := os.Getenv("PUREDNS_RESOLVERS")
	parts := strings.Split(resolversStr, ",")
	out := make([]string, 0, len(parts))
	seen := make(map[string]bool)
	for _, r := range parts {
		r = strings.TrimSpace(r)
		if r == "" || seen[r] {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

func countNonEmptyLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	if err := scanner.Err(); err != nil {
		return count, err
	}
	return count, nil
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

func fileSizeBytes(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func shouldValidatePureDNSWithDNSX() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("PUREDNS_POST_DNSX_VALIDATE")))
	return v == "1" || v == "true" || v == "yes"
}

func mergePureDNSResolvedHosts(dst map[string][]string, hosts []string) {
	for _, host := range hosts {
		host = strings.TrimSpace(host)
		if host == "" {
			continue
		}
		if _, exists := dst[host]; !exists {
			dst[host] = []string{}
		}
	}
}

func parsePureDNSSubdomainsFromFile(ctx Context, path, rootDomain string) []string {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("⚠️ Failed to open PureDNS output file %s: %v\n", path, err)
		return []string{}
	}
	defer file.Close()

	seen := make(map[string]bool)
	subdomains := make([]string, 0)
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		normalized := ctx.normalize(strings.TrimSpace(scanner.Text()), rootDomain)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		subdomains = append(subdomains, normalized)
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ Failed reading PureDNS output file %s: %v\n", path, err)
	}

	return subdomains
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
