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

	if len(wordlists) == 0 {
		return map[string][]string{}, nil
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

	combinedWordlistFile := filepath.Join(tempDir, "puredns_combined_wordlist.txt")
	allWords := readPureDNSWordlists(wordlists)
	if len(allWords) == 0 {
		log.Printf("⚠️ No words found in wordlists, skipping puredns\n")
		return map[string][]string{}, nil
	}

	if err := utils.WriteSliceToFile(combinedWordlistFile, allWords); err != nil {
		return nil, fmt.Errorf("failed to write combined wordlist: %v", err)
	}

	log.Printf(" Running puredns bruteforce with %d words for %s...\n", len(allWords), rootDomain)

	output, err := ctx.RunCommand(
		ctx.TargetID,
		"puredns",
		"bruteforce",
		combinedWordlistFile,
		rootDomain,
		"-r", resolversFile,
		"--trusted-only",
		"--quiet",
	)
	if err != nil {
		if err.Error() == "process killed by user request" {
			return nil, err
		}
		log.Printf("⚠️ Puredns error: %v\n", err)
		return map[string][]string{}, nil
	}

	subdomains := parsePureDNSSubdomains(ctx, string(output), rootDomain)
	results := resolvePureDNSSubdomains(ctx, tempDir, subdomains)

	log.Printf("✅ Puredns found %d live subdomains for %s\n", len(results), rootDomain)

	_ = os.Remove(combinedWordlistFile)
	_ = os.Remove(resolversFile)

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

func readPureDNSWordlists(wordlists []string) []string {
	wordSet := make(map[string]bool)
	var allWords []string

	for _, wlPath := range wordlists {
		if _, err := os.Stat(wlPath); os.IsNotExist(err) {
			log.Printf("⚠️ Wordlist not found: %s, skipping...\n", wlPath)
			continue
		}

		content, err := os.ReadFile(wlPath)
		if err != nil {
			log.Printf("⚠️ Failed to read wordlist %s: %v, skipping...\n", wlPath, err)
			continue
		}

		for _, line := range strings.Split(string(content), "\n") {
			word := strings.TrimSpace(line)
			if word != "" && !strings.HasPrefix(word, "#") && !wordSet[word] {
				wordSet[word] = true
				allWords = append(allWords, word)
			}
		}
	}

	return allWords
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

func resolvePureDNSSubdomains(ctx Context, tempDir string, subdomains []string) map[string][]string {
	results := make(map[string][]string)
	if len(subdomains) == 0 {
		return results
	}

	purednsInputFile := filepath.Join(tempDir, "puredns_dnsx_input.txt")
	if err := utils.WriteSliceToFile(purednsInputFile, subdomains); err != nil {
		log.Printf("⚠️ Failed to write puredns input for dnsx: %v\n", err)
		for _, subdomain := range subdomains {
			results[subdomain] = []string{}
		}
		return results
	}

	purednsDNSXOutputFile := filepath.Join(tempDir, "puredns_dnsx_output.json")
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
