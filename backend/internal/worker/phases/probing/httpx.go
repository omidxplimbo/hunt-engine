package probing

import (
	"bufio"
	"encoding/json"
	workertypes "github.com/omidxplimbo/hunt-engine/backend/internal/worker/types"
	"os"
)

// HTTPXResult mirrors the JSONL output fields currently used by the worker.
type HTTPXResult = workertypes.HTTPXResult

func readHTTPXResults(filename string) (map[string]HTTPXResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	results := make(map[string]HTTPXResult)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		var result HTTPXResult
		if err := json.Unmarshal([]byte(line), &result); err == nil {
			if result.Input != "" {
				result.RawJSON = line
				results[result.Input] = result
			}
		}
	}

	return results, scanner.Err()
}
