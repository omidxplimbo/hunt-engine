package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

func RunVirusTotal(ctx Context, subdomains []string) map[string]string {
	results := make(map[string]string)
	apiKey := "183e25c8551f61932c61c190f8d2fc4667b82e954ada72ccef145cb4075a005e" // TODO: Move to config
	client := &http.Client{Timeout: 30 * time.Second}

	log.Printf(" Starting VT Collection for %d subdomains.", len(subdomains))
	for _, domain := range subdomains {
		if ctx.CheckStop(ctx.TargetID) {
			return results
		}

		url := fmt.Sprintf("https://virustotal.com/vtapi/v2/domain/report?apikey=%s&domain=%s", apiKey, domain)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			log.Printf("⚠️ Failed to create VT request for %s: %v\n", domain, err)
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("⚠️ VirusTotal API error for %s: %v\n", domain, err)
			continue
		}

		if resp.StatusCode != 200 {
			if resp.StatusCode == 204 {
				log.Printf("⚠️ VirusTotal Rate Limit Exceeded (204) for %s. Slowing down.\n", domain)
			} else if resp.StatusCode != 404 {
				log.Printf("⚠️ VirusTotal API status %d for %s\n", resp.StatusCode, domain)
			}
			resp.Body.Close()
			continue
		}

		var vtResp struct {
			DetectedUrls   []interface{} `json:"detected_urls"`
			UndetectedUrls []interface{} `json:"undetected_urls"`
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
				if arr, ok := item.([]interface{}); ok && len(arr) > 0 {
					if u, ok := arr[0].(string); ok && u != "" {
						u = strings.TrimSpace(u)
						if _, exists := results[u]; !exists {
							results[u] = "virustotal"
							foundCount++
						}
					}
				}
			}
		}

		processURLs(vtResp.DetectedUrls)
		processURLs(vtResp.UndetectedUrls)

		if foundCount > 0 {
			log.Printf(" VirusTotal found %d new URLs for %s\n", foundCount, domain)
		}

		time.Sleep(15 * time.Second)
	}

	return results
}
