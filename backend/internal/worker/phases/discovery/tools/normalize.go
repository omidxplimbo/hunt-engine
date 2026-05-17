package tools

import "strings"

// NormalizeSubdomain trims, lowercases, removes a leading www label, and
// keeps only proper subdomains of rootDomain.
func NormalizeSubdomain(subdomain, rootDomain string) string {
	subdomain = strings.TrimSpace(subdomain)
	if subdomain == "" {
		return ""
	}

	subdomainLower := strings.ToLower(subdomain)
	rootDomainLower := strings.ToLower(strings.TrimSpace(rootDomain))
	if rootDomainLower == "" {
		return ""
	}

	subdomainLower = strings.TrimPrefix(subdomainLower, "www.")

	if subdomainLower == rootDomainLower {
		return ""
	}

	if !strings.HasSuffix(subdomainLower, "."+rootDomainLower) {
		return ""
	}

	return subdomainLower
}
