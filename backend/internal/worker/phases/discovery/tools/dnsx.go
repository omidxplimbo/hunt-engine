package tools

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

const DefaultDNSXBatchSize = 5000
const DefaultDNSXThreads = 50

type DNSXResult struct {
	Host string   `json:"host"`
	A    []string `json:"a"`
}

func RunDNSXForList(ctx Context, candidates []string, outputFile string) (map[string][]string, error) {
	if len(candidates) == 0 {
		return map[string][]string{}, nil
	}
	if err := os.MkdirAll(filepath.Dir(outputFile), 0o755); err != nil {
		return nil, err
	}
	inputFile := outputFile + ".input"
	if err := utils.WriteSliceToFile(inputFile, candidates); err != nil {
		return nil, err
	}
	defer os.Remove(inputFile)
	defer os.Remove(outputFile)
	return RunDNSX(ctx, inputFile, outputFile)
}

// DetectWildcardIPs resolves random non-existent hosts under rootDomain and
// returns IPs that indicate wildcard/sinkhole DNS behavior. Callers can then
// filter live DNS results before mutation/save stages.
func DetectWildcardIPs(ctx Context, rootDomain string, outputFile string) (map[string]struct{}, error) {
	rootDomain = strings.Trim(strings.ToLower(rootDomain), ".")
	wildcardIPs := make(map[string]struct{})
	if rootDomain == "" {
		return wildcardIPs, nil
	}

	probeCount := wildcardProbeCount()
	probes := make([]string, 0, probeCount)
	for i := 0; i < probeCount; i++ {
		probes = append(probes, "__hunt-wildcard-"+randomHex(8)+"."+rootDomain)
	}

	ctx.updatePhase("PHASE 1: WILDCARD DNS CHECK")
	ctx.heartbeat()

	results, err := RunDNSXForList(ctx, probes, outputFile)
	if err != nil {
		return wildcardIPs, err
	}

	for _, ips := range results {
		for _, ip := range ips {
			ip = strings.TrimSpace(ip)
			if ip != "" {
				wildcardIPs[ip] = struct{}{}
			}
		}
	}

	if len(wildcardIPs) == 0 {
		log.Printf("✅ Wildcard DNS check for %s: no wildcard IPs detected\n", rootDomain)
		return wildcardIPs, nil
	}

	items := make([]string, 0, len(wildcardIPs))
	for ip := range wildcardIPs {
		items = append(items, ip)
	}
	sort.Strings(items)
	log.Printf("⚠️ Wildcard DNS check for %s detected %d wildcard IP(s): %s\n", rootDomain, len(items), strings.Join(items, ", "))
	return wildcardIPs, nil
}

func randomHex(n int) string {
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return strconv.FormatInt(int64(os.Getpid()), 10) + strconv.Itoa(n)
	}
	return hex.EncodeToString(buf)
}

func wildcardProbeCount() int {
	envStr := os.Getenv("DNS_WILDCARD_PROBE_COUNT")
	if envStr == "" {
		return 5
	}
	value, err := strconv.Atoi(envStr)
	if err != nil || value <= 0 {
		return 5
	}
	if value > 20 {
		return 20
	}
	return value
}

func RunDNSX(ctx Context, inputFile, outputFile string) (map[string][]string, error) {
	if err := ctx.ensureCommandRunner(); err != nil {
		return nil, err
	}

	file, err := os.Open(inputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string][]string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	results := make(map[string][]string)
	batchSize := dnsxBatchSize()
	threads := dnsxThreads()
	baseDir := filepath.Dir(outputFile)

	log.Printf(" Starting streaming DNSX validation from %s (batch size: %d, threads: %d)\n", inputFile, batchSize, threads)

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)

	batch := make([]string, 0, batchSize)
	batchNo := 0
	totalCandidates := 0

	flushBatch := func() error {
		if len(batch) == 0 {
			return nil
		}

		if ctx.stopped() {
			return fmt.Errorf("process killed by user request")
		}

		batchNo++
		batchInputFile := filepath.Join(baseDir, fmt.Sprintf("dnsx_batch_%06d_input.txt", batchNo))
		batchOutputFile := filepath.Join(baseDir, fmt.Sprintf("dnsx_batch_%06d_output.json", batchNo))

		if err := utils.WriteSliceToFile(batchInputFile, batch); err != nil {
			return err
		}
		_ = os.Remove(batchOutputFile)

		totalCandidates += len(batch)
		log.Printf(" DNSX batch %d: validating %d domains (processed candidates: %d)\n", batchNo, len(batch), totalCandidates)

		_, err := ctx.RunCommand(
			ctx.TargetID,
			"dnsx",
			"-l", batchInputFile,
			"-json",
			"-o", batchOutputFile,
			"-silent",
			"-a",
			"-resp",
			"-threads", strconv.Itoa(threads),
		)

		_ = os.Remove(batchInputFile)
		if err != nil {
			_ = os.Remove(batchOutputFile)
			return err
		}

		if err := appendDNSXResults(results, batchOutputFile); err != nil {
			_ = os.Remove(batchOutputFile)
			return err
		}

		_ = os.Remove(batchOutputFile)
		batch = batch[:0]
		return nil
	}

	for scanner.Scan() {
		candidate := strings.TrimSpace(scanner.Text())
		if candidate == "" {
			continue
		}

		batch = append(batch, candidate)
		if len(batch) >= batchSize {
			if err := flushBatch(); err != nil {
				return nil, err
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	if err := flushBatch(); err != nil {
		return nil, err
	}

	log.Printf("✅ DNSX validation completed: %d live domains resolved from %d candidates\n", len(results), totalCandidates)
	return results, nil
}

func appendDNSXResults(results map[string][]string, filename string) error {
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
		var res DNSXResult
		if err := json.Unmarshal([]byte(scanner.Text()), &res); err == nil && res.Host != "" && len(res.A) > 0 {
			cleanIPs := make([]string, 0, len(res.A))
			seen := make(map[string]struct{})
			for _, ip := range res.A {
				ip = strings.TrimSpace(ip)
				if ip == "" {
					continue
				}
				if _, exists := seen[ip]; exists {
					continue
				}
				seen[ip] = struct{}{}
				cleanIPs = append(cleanIPs, ip)
			}
			if len(cleanIPs) > 0 {
				sort.Strings(cleanIPs)
				results[res.Host] = cleanIPs
			}
		}
	}

	return scanner.Err()
}

func dnsxBatchSize() int {
	envStr := os.Getenv("DNSX_BATCH_SIZE")
	if envStr == "" {
		return DefaultDNSXBatchSize
	}

	size, err := strconv.Atoi(envStr)
	if err != nil || size <= 0 {
		return DefaultDNSXBatchSize
	}

	return size
}

func dnsxThreads() int {
	envStr := os.Getenv("DNSX_THREADS")
	if envStr == "" {
		return DefaultDNSXThreads
	}

	threads, err := strconv.Atoi(envStr)
	if err != nil || threads <= 0 {
		return DefaultDNSXThreads
	}

	return threads
}
