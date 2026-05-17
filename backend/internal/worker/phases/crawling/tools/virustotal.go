package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func RunVirusTotal(ctx Context, subdomains []string) map[string]string {
	results := make(map[string]string)

	apiKey := strings.TrimSpace(os.Getenv("VIRUSTOTAL_API_KEY"))
	if apiKey == "" {
		log.Println("⏩ Skipping VirusTotal collection: VIRUSTOTAL_API_KEY is not set")
		return results
	}

	client := &http.Client{Timeout: 30 * time.Second}

	log.Printf("🔎 Starting VT Collection for %d subdomains.\n", len(subdomains))

	for _, domain := range subdomains {
		if ctx.CheckStop(ctx.TargetID) {
			return results
		}

		domain = strings.TrimSpace(domain)
		if domain == "" {
			continue
		}

		requestURL := fmt.Sprintf(
			"https://www.virustotal.com/vtapi/v2/domain/report?apikey=%s&domain=%s",
			url.QueryEscape(apiKey),
			url.QueryEscape(domain),
		)

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			log.Printf("⚠️ Failed to create VT request for %s: %v\n", domain, err)
			continue
		}

		req.Header.Set(
			"User-Agent",
			"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 "+
				"(KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36",
		)

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("⚠️ VirusTotal API error for %s: %v\n", domain, err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNoContent {
				log.Printf("⚠️ VirusTotal rate limit exceeded for %s. Slowing down.\n", domain)
			} else if resp.StatusCode != http.StatusNotFound {
				log.Printf("⚠️ VirusTotal API status %d for %s\n", resp.StatusCode, domain)
			}

			resp.Body.Close()
			continue
		}

		var vtResp struct {
			DetectedURLs   []interface{} `json:"detected_urls"`
			UndetectedURLs []interface{} `json:"undetected_urls"`
		}

		if err := json.NewDecoder(resp.Body).Decode(&vtResp); err != nil {
			log.Printf("⚠️ Failed to decode VT response for %s: %v\n", domain, err)
			resp.Body.Close()
			continue
		}

		resp.Body.Close()

		foundCount := 0

		processURLs := func(list []interface{}) {
			for _, item := range list {
				arr, ok := item.([]interface{})
				if !ok || len(arr) == 0 {
					continue
				}

				u, ok := arr[0].(string)
				if !ok {
					continue
				}

				u = strings.TrimSpace(u)
				if u == "" {
					continue
				}

				if _, exists := results[u]; exists {
					continue
				}

				results[u] = "virustotal"
				foundCount++
			}
		}

		processURLs(vtResp.DetectedURLs)
		processURLs(vtResp.UndetectedURLs)

		if foundCount > 0 {
			log.Printf("✅ VirusTotal found %d new URLs for %s\n", foundCount, domain)
		}

		time.Sleep(15 * time.Second)
	}

	return results
}
