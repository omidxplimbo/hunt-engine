package passive

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

func RunCrtsh(rootDomain string) []string {
	url1 := fmt.Sprintf("https://crt.sh/?q=%%25.%s&output=json", rootDomain)
	url2 := fmt.Sprintf("https://crt.sh/?q=%s&output=json", rootDomain)

	var allResults []string

	log.Printf(" Querying crt.sh API (wildcard): %s\n", url1)
	if data, err := fetchCrtshData(url1); err == nil {
		parsed := parseCrtshJSON(data, rootDomain)
		allResults = append(allResults, parsed...)
		log.Printf("✅ crt.sh wildcard query returned %d domains\n", len(parsed))
	} else {
		log.Printf("⚠️ crt.sh wildcard query failed: %v\n", err)
	}

	log.Printf(" Querying crt.sh API (exact): %s\n", url2)
	if data, err := fetchCrtshData(url2); err == nil {
		parsed := parseCrtshJSON(data, rootDomain)
		allResults = append(allResults, parsed...)
		log.Printf("✅ crt.sh exact query returned %d domains\n", len(parsed))
	} else {
		log.Printf("⚠️ crt.sh exact query failed: %v\n", err)
	}

	uniqueMap := make(map[string]bool)
	var finalResults []string
	for _, res := range allResults {
		res = strings.ToLower(strings.TrimSpace(res))
		if normalized := NormalizeSubdomain(res, rootDomain); normalized != "" && !uniqueMap[normalized] {
			uniqueMap[normalized] = true
			finalResults = append(finalResults, normalized)
		}
	}

	log.Printf(" CRT.SH total unique domains: %d\n", len(finalResults))
	return finalResults
}

func fetchCrtshData(url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("crt.sh API returned status %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

func parseCrtshJSON(data []byte, rootDomain string) []string {
	var results []string
	var jsonData []map[string]interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return results
	}

	for _, item := range jsonData {
		if cn, ok := item["common_name"].(string); ok && cn != "" {
			cn = strings.TrimPrefix(cn, "*.")
			cn = strings.TrimPrefix(cn, "\\*.")
			results = append(results, cn)
		}

		if nv, ok := item["name_value"].(string); ok && nv != "" {
			for _, line := range strings.Split(nv, "\n") {
				line = strings.TrimSpace(line)
				if line != "" {
					line = strings.TrimPrefix(line, "*.")
					results = append(results, line)
				}
			}
		}
	}

	var filtered []string
	for _, res := range results {
		if normalized := NormalizeSubdomain(res, rootDomain); normalized != "" {
			filtered = append(filtered, normalized)
		}
	}

	return filtered
}
