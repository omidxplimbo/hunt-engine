package merge

import (
	"bufio"
	"os"
	"sort"
	"strings"

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

type CandidateFileInput struct {
	PassiveResults     []string
	MutatedResultsFile string
	PurednsResults     map[string][]string
	ExistingAssets     []string
	OutputFile         string
	CheckStop          func() bool
}

type SaveListInput struct {
	PassiveResults []string
	LiveResults    map[string][]string
	PurednsResults map[string][]string
	ExistingAssets []string
}

type FileInput struct {
	PassiveResults []string
	PassiveSources map[string][]string

	MutatedResultsFile string
	MutatedSources     map[string][]string

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

	return Result{MasterList: masterList, Sources: sources}
}

// BuildFromFiles is a memory-safer variant for large mutation outputs. It
// reads Alterx candidates line-by-line instead of requiring the caller to load
// them all into a slice first.
func BuildFromFiles(input FileInput) (Result, error) {
	seen := make(map[string]struct{})
	masterList := make([]string, 0, len(input.PassiveResults)+len(input.ExistingAssets)+len(input.PurednsResults))

	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		masterList = append(masterList, value)
	}

	for _, value := range input.PassiveResults {
		add(value)
	}

	if input.MutatedResultsFile != "" {
		if err := addFile(input.MutatedResultsFile, add); err != nil {
			return Result{}, err
		}
	}

	for subdomain := range input.PurednsResults {
		add(subdomain)
	}

	for _, value := range input.ExistingAssets {
		add(value)
	}

	sources := make(map[string][]string)
	MergeSources(sources, input.PassiveSources)
	MergeSources(sources, input.MutatedSources)
	MergeSources(sources, input.PurednsSources)

	return Result{MasterList: masterList, Sources: sources}, nil
}

func addFile(filename string, add func(string)) error {
	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		add(scanner.Text())
	}

	return scanner.Err()
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

// MergeFileSourcesForLive streams a huge candidate file and adds source labels
// only for hosts that actually resolved. This keeps AlterX source attribution
// without materializing millions of unresolved candidates in memory or JSON.
func MergeFileSourcesForLive(dst map[string][]string, filename string, live map[string][]string, source string) error {
	if filename == "" || len(live) == 0 || source == "" {
		return nil
	}

	file, err := os.Open(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	addSource := func(host string) {
		host = strings.TrimSpace(host)
		if host == "" {
			return
		}
		if _, ok := live[host]; !ok {
			return
		}

		seen := make(map[string]bool)
		for _, existing := range dst[host] {
			existing = strings.TrimSpace(existing)
			if existing != "" {
				seen[existing] = true
			}
		}
		seen[source] = true

		merged := make([]string, 0, len(seen))
		for item := range seen {
			merged = append(merged, item)
		}
		sort.Strings(merged)
		dst[host] = merged
	}

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	for scanner.Scan() {
		addSource(scanner.Text())
	}

	return scanner.Err()
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

// WriteCandidatesToFile writes DNSX candidates to disk without materializing the
// full candidate set in memory. It intentionally allows duplicates from huge
// mutation streams so large scans remain memory-stable; DNSX validation is still
// performed over the complete candidate stream.
func WriteCandidatesToFile(input CandidateFileInput) (int, error) {
	out, err := os.Create(input.OutputFile)
	if err != nil {
		return 0, err
	}
	defer out.Close()

	writer := bufio.NewWriterSize(out, 1024*1024)
	defer writer.Flush()

	count := 0
	writeOne := func(value string) error {
		if input.CheckStop != nil && input.CheckStop() {
			return os.ErrClosed
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return nil
		}
		if _, err := writer.WriteString(value + "\n"); err != nil {
			return err
		}
		count++
		return nil
	}

	for _, value := range input.PassiveResults {
		if err := writeOne(value); err != nil {
			return count, err
		}
	}

	if input.MutatedResultsFile != "" {
		file, err := os.Open(input.MutatedResultsFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return count, err
			}
		} else {
			scanner := bufio.NewScanner(file)
			scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
			for scanner.Scan() {
				if err := writeOne(scanner.Text()); err != nil {
					_ = file.Close()
					return count, err
				}
			}
			if err := scanner.Err(); err != nil {
				_ = file.Close()
				return count, err
			}
			_ = file.Close()
		}
	}

	for subdomain := range input.PurednsResults {
		if err := writeOne(subdomain); err != nil {
			return count, err
		}
	}

	for _, value := range input.ExistingAssets {
		if err := writeOne(value); err != nil {
			return count, err
		}
	}

	_ = writer.Flush()
	return count, nil
}

// BuildSaveList returns the bounded asset list persisted after DNS validation.
// Huge Alterx-only dead candidates are intentionally not persisted as dead assets;
// they are still fully validated by DNSX and live results are saved.
func BuildSaveList(input SaveListInput) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(input.PassiveResults)+len(input.LiveResults)+len(input.ExistingAssets))
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	for _, value := range input.PassiveResults {
		add(value)
	}
	for value := range input.LiveResults {
		add(value)
	}
	for value := range input.PurednsResults {
		add(value)
	}
	for _, value := range input.ExistingAssets {
		add(value)
	}
	return out
}
