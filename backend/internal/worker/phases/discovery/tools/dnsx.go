package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
)

const DefaultDNSXBatchSize = 5000

type DNSXResult struct {
	Host string   `json:"host"`
	A    []string `json:"a"`
}

func RunDNSX(ctx Context, inputFile, outputFile string) (map[string][]string, error) {
	if err := ctx.ensureCommandRunner(); err != nil {
		return nil, err
	}

	results := make(map[string][]string)

	domains, err := utils.ReadSliceFromFile(inputFile)
	if err != nil {
		return nil, err
	}
	if len(domains) == 0 {
		return results, nil
	}

	batchSize := dnsxBatchSize()
	totalBatches := (len(domains) + batchSize - 1) / batchSize
	baseDir := filepath.Dir(outputFile)

	log.Printf(" Starting DNSX validation for %d domains in %d batches (batch size: %d)\n", len(domains), totalBatches, batchSize)

	for start := 0; start < len(domains); start += batchSize {
		if ctx.stopped() {
			return nil, fmt.Errorf("process killed by user request")
		}

		end := start + batchSize
		if end > len(domains) {
			end = len(domains)
		}

		batchNo := (start / batchSize) + 1
		batchInputFile := filepath.Join(baseDir, fmt.Sprintf("dnsx_batch_%06d_input.txt", batchNo))
		batchOutputFile := filepath.Join(baseDir, fmt.Sprintf("dnsx_batch_%06d_output.json", batchNo))

		if err := utils.WriteSliceToFile(batchInputFile, domains[start:end]); err != nil {
			return nil, err
		}
		_ = os.Remove(batchOutputFile)

		log.Printf(" DNSX batch %d/%d: validating %d domains (%d/%d)\n", batchNo, totalBatches, end-start, end, len(domains))

		_, err := ctx.RunCommand(
			ctx.TargetID,
			"dnsx",
			"-l", batchInputFile,
			"-json",
			"-o", batchOutputFile,
			"-silent",
			"-a",
			"-resp",
			"-threads", "50",
		)

		_ = os.Remove(batchInputFile)
		if err != nil {
			_ = os.Remove(batchOutputFile)
			return nil, err
		}

		if err := appendDNSXResults(results, batchOutputFile); err != nil {
			_ = os.Remove(batchOutputFile)
			return nil, err
		}

		_ = os.Remove(batchOutputFile)
	}

	log.Printf("✅ DNSX validation completed: %d live domains resolved from %d candidates\n", len(results), len(domains))
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
		if err := json.Unmarshal([]byte(scanner.Text()), &res); err == nil && res.Host != "" {
			results[res.Host] = res.A
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
