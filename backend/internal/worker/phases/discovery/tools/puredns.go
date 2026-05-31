package tools

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

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

	validWordlists := make([]string, 0, len(wordlists))
	for _, wlPath := range wordlists {
		wlPath = strings.TrimSpace(wlPath)
		if wlPath == "" {
			continue
		}

		info, err := os.Stat(wlPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Printf("⚠️ PureDNS wordlist not found: %s, skipping...\n", wlPath)
				continue
			}
			log.Printf("⚠️ Failed to stat PureDNS wordlist %s: %v, skipping...\n", wlPath, err)
			continue
		}

		if info.IsDir() {
			log.Printf("⚠️ PureDNS wordlist is a directory: %s, skipping...\n", wlPath)
			continue
		}

		validWordlists = append(validWordlists, wlPath)
	}

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
