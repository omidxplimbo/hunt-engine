package merge

import (
	"sort"

	workerhelpers "github.com/omidxplimbo/hunt-engine/backend/internal/worker/helpers"
)

// Input contains all Discovery candidate streams that participate in the MERGE step.
type Input struct {
	PassiveResults []string
	PassiveSources map[string][]string

	MutatedResults []string
	MutatedSources map[string][]string

	PurednsResults map[string][]string
	PurednsSources map[string][]string

	ExistingAssets []string
}

// Result contains the final candidate list sent to DNSX and the source map
// persisted alongside the merge checkpoint.
type Result struct {
	MasterList []string
	Sources    map[string][]string
}

// Build creates the Discovery master list and merged source map from passive,
// mutation, optional puredns, and existing asset inputs.
func Build(input Input) Result {
	masterList := workerhelpers.MergeUnique(input.PassiveResults, input.MutatedResults)

	purednsSubdomains := make([]string, 0, len(input.PurednsResults))
	for subdomain := range input.PurednsResults {
		purednsSubdomains = append(purednsSubdomains, subdomain)
	}

	masterList = workerhelpers.MergeUnique(masterList, purednsSubdomains)
	masterList = workerhelpers.MergeUnique(masterList, input.ExistingAssets)

	sources := make(map[string][]string)
	MergeSources(sources, input.PassiveSources)
	MergeSources(sources, input.MutatedSources)
	MergeSources(sources, input.PurednsSources)

	return Result{
		MasterList: masterList,
		Sources:    sources,
	}
}

// MergeSources merges src into dst while de-duplicating and sorting source labels.
func MergeSources(dst map[string][]string, src map[string][]string) {
	for subdomain, sourceList := range src {
		if len(sourceList) == 0 {
			continue
		}

		seen := make(map[string]bool)
		for _, source := range dst[subdomain] {
			seen[source] = true
		}
		for _, source := range sourceList {
			seen[source] = true
		}

		merged := make([]string, 0, len(seen))
		for source := range seen {
			merged = append(merged, source)
		}

		sort.Strings(merged)
		dst[subdomain] = merged
	}
}

// MergeLiveResults merges already-resolved live DNS results, de-duplicating IPs per host.
// It is used after DNSX to inject PureDNS results that were resolved earlier and
// should not be revalidated by DNSX.
func MergeLiveResults(base map[string][]string, extra map[string][]string) map[string][]string {
	if base == nil {
		base = make(map[string][]string)
	}

	for host, ips := range extra {
		if host == "" || len(ips) == 0 {
			continue
		}

		seen := make(map[string]bool)
		for _, ip := range base[host] {
			if ip != "" {
				seen[ip] = true
			}
		}
		for _, ip := range ips {
			if ip != "" {
				seen[ip] = true
			}
		}

		merged := make([]string, 0, len(seen))
		for ip := range seen {
			merged = append(merged, ip)
		}
		sort.Strings(merged)
		base[host] = merged
	}

	return base
}
