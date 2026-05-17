package passive

import "strings"

// NormalizeSubdomain normalizes a candidate and keeps only real subdomains of rootDomain.
func NormalizeSubdomain(subdomain, rootDomain string) string {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return ""
	}

	subdomainLower := strings.ToLower(subdomain)
	rootDomainLower := strings.ToLower(strings.TrimSpace(rootDomain))

	subdomainLower = strings.TrimPrefix(subdomainLower, "www.")

	if subdomainLower == rootDomainLower {
		return ""
	}

	if !strings.HasSuffix(subdomainLower, "."+rootDomainLower) {
		return ""
	}

	return subdomainLower
}

func normalizeLines(output []byte, rootDomain string) []string {
	seen := make(map[string]bool)
	var out []string

	for _, line := range strings.Split(string(output), "\n") {
		normalized := NormalizeSubdomain(line, rootDomain)
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}

	return out
}
