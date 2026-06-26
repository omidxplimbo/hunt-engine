package tools

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/featureflags"
	workerruntime "github.com/omidxplimbo/hunt-engine/backend/internal/worker/runtime"
	workerstate "github.com/omidxplimbo/hunt-engine/backend/internal/worker/state"
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

	publicResolversFile, publicResolversCount, publicCleanup, err := preparePureDNSPublicResolversFile(tempDir, ctx.TargetID)
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
		massdnsOutputFile := filepath.Join(tempDir, fmt.Sprintf("puredns_%06d_massdns.txt", idx+1))
		_ = os.Remove(outputFile)

		totalLines, totalLinesErr := countNonEmptyLines(wlPath)
		if totalLinesErr != nil {
			log.Printf("⚠️ Failed to count PureDNS wordlist lines for %s: %v\n", wlPath, totalLinesErr)
			totalLines = 0
		}

		args := []string{
			"bruteforce",
			wlPath,
			rootDomain,
			"-r", publicResolversFile,
			"--resolvers-trusted", trustedResolversFile,
			"--write", outputFile,
			"--write-massdns", massdnsOutputFile,
		}

		startedAt := time.Now()
		log.Printf(
			"🔍 Running PureDNS wordlist %d/%d for %s: wordlist=%s size_bytes=%d lines=%d public_resolvers=%d trusted_resolvers=%d output=%s command=%s\n",
			idx+1,
			len(validWordlists),
			rootDomain,
			wlPath,
			fileSizeBytes(wlPath),
			totalLines,
			publicResolversCount,
			len(trusted),
			outputFile,
			"puredns "+strings.Join(args, " "),
		)

		progressBuffer := ""
		updatePureDNSProgressInitial(
			ctx.TargetID,
			rootDomain,
			wlPath,
			idx+1,
			len(validWordlists),
			totalLines,
			publicResolversCount,
			len(trusted),
			publicResolversFile,
			trustedResolversFile,
			startedAt,
		)

		err := workerruntime.RunCommandWithProgressAndKillSwitch(
			ctx.TargetID,
			func(uint) { ctx.heartbeat() },
			func(uint) bool { return ctx.stopped() },
			func(chunk []byte) {
				progressBuffer = appendPureDNSProgressBuffer(progressBuffer, chunk)
				updatePureDNSProgressFromOutput(
					ctx.TargetID,
					rootDomain,
					wlPath,
					idx+1,
					len(validWordlists),
					totalLines,
					publicResolversCount,
					len(trusted),
					publicResolversFile,
					trustedResolversFile,
					startedAt,
					progressBuffer,
				)
			},
			"puredns",
			args...,
		)
		duration := time.Since(startedAt)
		updatePureDNSProgressCompleted(ctx.TargetID, rootDomain, wlPath, idx+1, len(validWordlists), totalLines, duration, err)
		if err != nil {
			if err.Error() == "process killed by user request" {
				return nil, err
			}
			log.Printf("⚠️ PureDNS failed for wordlist %s after %s: %v\n", wlPath, duration, err)
			continue
		}

		ctx.updatePhase(fmt.Sprintf("PHASE 1: PUREDNS PARSING WORDLIST %d/%d: %s", idx+1, len(validWordlists), baseName))
		ctx.heartbeat()

		wordlistResults := parsePureDNSResolvedResultsFromFile(ctx, massdnsOutputFile, rootDomain)
		_ = os.Remove(outputFile)
		_ = os.Remove(massdnsOutputFile)
		if len(wordlistResults) == 0 {
			log.Printf("ℹ️ PureDNS wordlist %s produced no resolved host/IP results for %s in %s; skipping host-only results to avoid storing dead PureDNS assets\n", wlPath, rootDomain, duration)
			continue
		}

		mergePureDNSLiveResults(results, wordlistResults)

		log.Printf(
			"✅ PureDNS wordlist %d/%d completed for %s in %s: resolved_with_ips=%d resolved_total=%d post_dnsx_validation=false\n",
			idx+1,
			len(validWordlists),
			rootDomain,
			duration,
			len(wordlistResults),
			len(results),
		)
	}

	updatePureDNSProgressFinal(ctx.TargetID, rootDomain, len(validWordlists), len(results))
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

func preparePureDNSPublicResolversFile(tempDir string, targetID uint) (string, int, bool, error) {
	if resolvers, ok := targetOwnerPureDNSResolvers(targetID); ok {
		path := filepath.Join(tempDir, "puredns_user_public_resolvers.txt")
		if err := utils.WriteSliceToFile(path, resolvers); err != nil {
			return "", 0, true, fmt.Errorf("failed to write account PureDNS resolvers file: %v", err)
		}
		log.Printf("PureDNS using account-scoped public resolver list for target_id=%d: resolvers=%d\n", targetID, len(resolvers))
		return path, len(resolvers), true, nil
	}

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

type pureDNSResolverConfigRow struct {
	Enabled       bool   `gorm:"column:enabled"`
	ResolversText string `gorm:"column:resolvers_text"`
	ResolverCount int    `gorm:"column:resolver_count"`
}

func targetOwnerPureDNSResolvers(targetID uint) ([]string, bool) {
	if targetID == 0 {
		return nil, false
	}

	var target struct {
		CreatedByUserID uint `gorm:"column:created_by_user_id"`
	}

	if err := database.DB.Table("targets").
		Select("created_by_user_id").
		Where("id = ?", targetID).
		Scan(&target).Error; err != nil {
		log.Printf("Failed to resolve target owner for PureDNS resolvers target_id=%d: %v\n", targetID, err)
		return nil, false
	}

	if target.CreatedByUserID == 0 {
		return nil, false
	}

	ownerKey := featureflags.OwnerKeyForUserID(target.CreatedByUserID)
	if strings.TrimSpace(ownerKey) == "" {
		return nil, false
	}

	var cfg pureDNSResolverConfigRow
	if err := database.DB.Table("user_pure_dns_resolver_configs").
		Select("enabled", "resolvers_text", "resolver_count").
		Where("owner_key = ? AND enabled = true", ownerKey).
		Scan(&cfg).Error; err != nil {
		log.Printf("Failed to load account PureDNS resolvers for owner_key=%s: %v\n", ownerKey, err)
		return nil, false
	}

	if !cfg.Enabled || strings.TrimSpace(cfg.ResolversText) == "" || cfg.ResolverCount <= 0 {
		return nil, false
	}

	resolvers := normalizePureDNSResolverLines(cfg.ResolversText)
	if len(resolvers) == 0 {
		return nil, false
	}

	return resolvers, true
}

func normalizePureDNSResolverLines(raw string) []string {
	lines := strings.Split(raw, "\n")
	out := make([]string, 0, len(lines))
	seen := make(map[string]bool)

	for _, line := range lines {
		value := strings.TrimSpace(line)
		if value == "" || strings.HasPrefix(value, "#") {
			continue
		}

		if hash := strings.Index(value, "#"); hash >= 0 {
			value = strings.TrimSpace(value[:hash])
		}

		if value == "" || seen[value] {
			continue
		}

		seen[value] = true
		out = append(out, value)
	}

	return out
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

func parsePureDNSResolvedResultsFromFile(ctx Context, path, rootDomain string) map[string][]string {
	file, err := os.Open(path)
	if err != nil {
		log.Printf("⚠️ Failed to open PureDNS output file %s: %v\n", path, err)
		return map[string][]string{}
	}
	defer file.Close()

	results := make(map[string][]string)
	hostOnlyCount := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 1024*1024)

	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}

		host, ips := parsePureDNSResolvedLine(ctx, raw, rootDomain)
		if host == "" {
			continue
		}
		if len(ips) == 0 {
			hostOnlyCount++
			continue
		}

		mergePureDNSLiveResults(results, map[string][]string{host: ips})
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ Failed reading PureDNS output file %s: %v\n", path, err)
	}

	if hostOnlyCount > 0 {
		log.Printf("⚠️ PureDNS output file %s contained %d host-only rows without captured IPs; skipped them to avoid storing PureDNS assets as dead/without DNS IP\n", path, hostOnlyCount)
	}

	return results
}

func parsePureDNSResolvedLine(ctx Context, raw, rootDomain string) (string, []string) {
	fields := strings.Fields(strings.TrimSpace(raw))
	if len(fields) == 0 {
		return "", nil
	}

	host := ""
	ips := make([]string, 0)

	for _, field := range fields {
		clean := strings.Trim(field, "[](),;\"'")
		clean = strings.TrimSuffix(clean, ".")

		if ip := net.ParseIP(clean); ip != nil {
			ips = append(ips, ip.String())
			continue
		}

		if host == "" {
			normalized := ctx.normalize(clean, rootDomain)
			if normalized != "" {
				host = normalized
			}
		}
	}

	if host == "" {
		host = ctx.normalize(fields[0], rootDomain)
	}

	if len(ips) == 0 {
		return host, nil
	}

	seen := make(map[string]bool)
	out := make([]string, 0, len(ips))
	for _, ip := range ips {
		if ip == "" || seen[ip] {
			continue
		}
		seen[ip] = true
		out = append(out, ip)
	}
	sort.Strings(out)

	return host, out
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

func appendPureDNSProgressBuffer(existing string, chunk []byte) string {
	next := existing + string(chunk)
	if len(next) > 32768 {
		next = next[len(next)-32768:]
	}
	return next
}

func updatePureDNSProgressInitial(
	targetID uint,
	rootDomain string,
	wordlistPath string,
	wordlistIndex int,
	wordlistTotal int,
	totalLines int,
	publicResolvers int,
	trustedResolvers int,
	publicResolverFile string,
	trustedResolverFile string,
	startedAt time.Time,
) {
	meta := workerstate.GetMeta(targetID)
	meta["puredns_progress"] = map[string]interface{}{
		"status":                "running",
		"domain":                rootDomain,
		"wordlist_index":        wordlistIndex,
		"wordlist_total":        wordlistTotal,
		"wordlist_path":         wordlistPath,
		"wordlist_name":         filepath.Base(wordlistPath),
		"total_lines":           totalLines,
		"processed_lines":       0,
		"percent":               0,
		"rate_per_second":       0,
		"elapsed_seconds":       0,
		"eta_seconds":           0,
		"public_resolvers":      publicResolvers,
		"trusted_resolvers":     trustedResolvers,
		"public_resolver_file":  publicResolverFile,
		"trusted_resolver_file": trustedResolverFile,
		"updated_at_unix":       time.Now().Unix(),
		"started_at_unix":       startedAt.Unix(),
		"progress_source":       "puredns_cli",
	}
	workerstate.SetMeta(targetID, meta)
}

func updatePureDNSProgressFromOutput(
	targetID uint,
	rootDomain string,
	wordlistPath string,
	wordlistIndex int,
	wordlistTotal int,
	totalLines int,
	publicResolvers int,
	trustedResolvers int,
	publicResolverFile string,
	trustedResolverFile string,
	startedAt time.Time,
	buffer string,
) {
	etaRaw, processed, totalFromCLI, rate, elapsedRaw, ok := latestPureDNSProgressFromBuffer(buffer)
	if !ok {
		return
	}

	elapsedFromCLI := parsePureDNSClockSeconds(elapsedRaw)
	etaSeconds := parsePureDNSClockSeconds(etaRaw)

	total := totalLines
	if totalFromCLI > 0 {
		total = totalFromCLI
	}

	percent := 0.0
	if total > 0 {
		percent = (float64(processed) / float64(total)) * 100
		if percent > 100 {
			percent = 100
		}
	}

	elapsed := int(time.Since(startedAt).Seconds())
	if elapsedFromCLI > 0 {
		elapsed = elapsedFromCLI
	}

	meta := workerstate.GetMeta(targetID)
	meta["puredns_progress"] = map[string]interface{}{
		"status":                "running",
		"domain":                rootDomain,
		"wordlist_index":        wordlistIndex,
		"wordlist_total":        wordlistTotal,
		"wordlist_path":         wordlistPath,
		"wordlist_name":         filepath.Base(wordlistPath),
		"total_lines":           total,
		"processed_lines":       processed,
		"percent":               percent,
		"rate_per_second":       rate,
		"elapsed_seconds":       elapsed,
		"eta_seconds":           etaSeconds,
		"public_resolvers":      publicResolvers,
		"trusted_resolvers":     trustedResolvers,
		"public_resolver_file":  publicResolverFile,
		"trusted_resolver_file": trustedResolverFile,
		"updated_at_unix":       time.Now().Unix(),
		"started_at_unix":       startedAt.Unix(),
		"progress_source":       "puredns_cli",
	}
	workerstate.SetMeta(targetID, meta)
}

func latestPureDNSProgressFromBuffer(buffer string) (string, int, int, float64, string, bool) {
	idx := strings.LastIndex(buffer, "[ETA ")
	if idx < 0 {
		return "", 0, 0, 0, "", false
	}

	segment := buffer[idx:]
	if end := strings.Index(segment, "\n"); end >= 0 {
		segment = segment[:end]
	}
	segment = strings.TrimSpace(segment)

	closeIdx := strings.Index(segment, "]")
	if closeIdx < 0 {
		return "", 0, 0, 0, "", false
	}

	etaRaw := strings.TrimSpace(strings.TrimPrefix(segment[:closeIdx], "[ETA"))
	rest := strings.TrimSpace(segment[closeIdx+1:])

	processed := 0
	total := 0
	for _, field := range strings.Fields(rest) {
		if !strings.Contains(field, "/") {
			continue
		}

		parts := strings.SplitN(field, "/", 2)
		if len(parts) != 2 {
			continue
		}

		left := strings.Trim(parts[0], " |")
		right := strings.Trim(parts[1], " |")
		p, pErr := strconv.Atoi(left)
		t, tErr := strconv.Atoi(right)
		if pErr == nil && tErr == nil && t > 0 {
			processed = p
			total = t
			break
		}
	}

	if total == 0 {
		return "", 0, 0, 0, "", false
	}

	rate := 0.0
	if rateIdx := strings.Index(rest, "rate:"); rateIdx >= 0 {
		ratePart := strings.TrimSpace(rest[rateIdx+len("rate:"):])
		if qpsIdx := strings.Index(ratePart, "qps"); qpsIdx >= 0 {
			ratePart = strings.TrimSpace(ratePart[:qpsIdx])
		}
		if fields := strings.Fields(ratePart); len(fields) > 0 {
			if parsed, err := strconv.ParseFloat(fields[0], 64); err == nil {
				rate = parsed
			}
		}
	}

	elapsedRaw := ""
	if timeIdx := strings.Index(rest, "(time:"); timeIdx >= 0 {
		timePart := strings.TrimSpace(rest[timeIdx+len("(time:"):])
		if close := strings.Index(timePart, ")"); close >= 0 {
			timePart = timePart[:close]
		}
		elapsedRaw = strings.TrimSpace(timePart)
	}

	return etaRaw, processed, total, rate, elapsedRaw, true
}

func parsePureDNSClockSeconds(raw string) int {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0
	}

	parts := strings.Split(raw, ":")
	total := 0
	for _, part := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil {
			return 0
		}
		total = total*60 + n
	}
	return total
}

func updatePureDNSProgressCompleted(
	targetID uint,
	rootDomain string,
	wordlistPath string,
	wordlistIndex int,
	wordlistTotal int,
	totalLines int,
	duration time.Duration,
	runErr error,
) {
	status := "completed"
	if runErr != nil {
		status = "failed"
	}

	meta := workerstate.GetMeta(targetID)

	progress := map[string]interface{}{}
	if existing, ok := meta["puredns_progress"].(map[string]interface{}); ok && existing != nil {
		progress = existing
	}

	progress["status"] = status
	progress["domain"] = rootDomain
	progress["wordlist_index"] = wordlistIndex
	progress["wordlist_total"] = wordlistTotal
	progress["wordlist_path"] = wordlistPath
	progress["wordlist_name"] = filepath.Base(wordlistPath)
	progress["total_lines"] = totalLines
	progress["elapsed_seconds"] = int(duration.Seconds())
	progress["eta_seconds"] = 0
	progress["updated_at_unix"] = time.Now().Unix()

	if runErr != nil {
		progress["error"] = runErr.Error()
	}

	meta["puredns_progress"] = progress
	workerstate.SetMeta(targetID, meta)
}

func updatePureDNSProgressFinal(targetID uint, rootDomain string, wordlistTotal int, resolvedTotal int) {
	meta := workerstate.GetMeta(targetID)
	meta["puredns_progress"] = map[string]interface{}{
		"status":          "done",
		"domain":          rootDomain,
		"wordlist_total":  wordlistTotal,
		"resolved_total":  resolvedTotal,
		"percent":         100,
		"eta_seconds":     0,
		"updated_at_unix": time.Now().Unix(),
		"progress_source": "puredns_cli",
	}
	workerstate.SetMeta(targetID, meta)
}
