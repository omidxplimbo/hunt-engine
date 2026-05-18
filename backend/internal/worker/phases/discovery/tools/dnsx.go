package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
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
