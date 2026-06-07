package tools

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type vtV2DomainReport struct {
	ResponseCode int `json:"response_code"`
	DetectedURLs []struct {
		URL       string `json:"url"`
		Positives int    `json:"positives"`
		Total     int    `json:"total"`
		ScanDate  string `json:"scan_date"`
	} `json:"detected_urls"`
	UndetectedURLs []interface{} `json:"undetected_urls"`
	VerboseMsg     string        `json:"verbose_msg"`
}

func vtNormalizeDomain(value string) string {
	value = strings.TrimSpace(value)
	value = strings.TrimPrefix(value, "http://")
	value = strings.TrimPrefix(value, "https://")
	if slash := strings.Index(value, "/"); slash >= 0 {
		value = value[:slash]
	}
	return strings.TrimSpace(value)
}

func vtExtractUndetectedURL(item interface{}) string {
	switch v := item.(type) {
	case []interface{}:
		if len(v) == 0 {
			return ""
		}
		u, _ := v[0].(string)
		return strings.TrimSpace(u)
	case []string:
		if len(v) == 0 {
			return ""
		}
		return strings.TrimSpace(v[0])
	case map[string]interface{}:
		for _, key := range []string{"url", "URL"} {
			if raw, ok := v[key].(string); ok {
				return strings.TrimSpace(raw)
			}
		}
	}

	return ""
}

func RunVirusTotal(ctx Context, subdomains []string) map[string]string {
	results := make(map[string]string)

	apiKey := strings.TrimSpace(ctx.VirusTotalAPIKey)
	if apiKey == "" {
		log.Println("⏩ Skipping VirusTotal collection: account VirusTotal API key is not configured or disabled")
		return results
	}

	client := &http.Client{Timeout: 30 * time.Second}

	log.Printf("🔎 Starting VirusTotal v2 URL collection for %d domains.\n", len(subdomains))

	seenDomains := make(map[string]bool)
	for _, rawDomain := range subdomains {
		if ctx.CheckStop(ctx.TargetID) {
			return results
		}

		domain := vtNormalizeDomain(rawDomain)
		if domain == "" || seenDomains[domain] {
			continue
		}
		seenDomains[domain] = true

		requestURL := fmt.Sprintf(
			"https://virustotal.com/vtapi/v2/domain/report?apikey=%s&domain=%s",
			url.QueryEscape(apiKey),
			url.QueryEscape(domain),
		)

		req, err := http.NewRequest("GET", requestURL, nil)
		if err != nil {
			log.Printf("⚠️ Failed to create VirusTotal request for %s: %v\n", domain, err)
			continue
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Accept", "application/json")

		resp, err := client.Do(req)
		if err != nil {
			log.Printf("⚠️ VirusTotal API error for %s: %v\n", domain, err)
			continue
		}

		if resp.StatusCode == http.StatusNoContent {
			resp.Body.Close()
			log.Printf("⚠️ VirusTotal rate limit reached while querying %s; stopping VirusTotal collection for this run\n", domain)
			break
		}

		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			resp.Body.Close()
			log.Printf("⚠️ VirusTotal authentication/permission failed with status %d for %s; check account API key\n", resp.StatusCode, domain)
			break
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			log.Printf("⚠️ VirusTotal API status %d for %s\n", resp.StatusCode, domain)
			continue
		}

		var vtResp vtV2DomainReport
		if err := json.NewDecoder(resp.Body).Decode(&vtResp); err != nil {
			resp.Body.Close()
			log.Printf("⚠️ Failed to decode VirusTotal response for %s: %v\n", domain, err)
			continue
		}
		resp.Body.Close()

		if vtResp.ResponseCode == 0 {
			log.Printf("ℹ️ VirusTotal has no dataset entry for %s: %s\n", domain, vtResp.VerboseMsg)
			time.Sleep(15 * time.Second)
			continue
		}

		foundCount := 0

		for _, item := range vtResp.DetectedURLs {
			u := strings.TrimSpace(item.URL)
			if u == "" {
				continue
			}
			if _, exists := results[u]; exists {
				continue
			}
			results[u] = "virustotal"
			foundCount++
		}

		for _, item := range vtResp.UndetectedURLs {
			u := vtExtractUndetectedURL(item)
			if u == "" {
				continue
			}
			if _, exists := results[u]; exists {
				continue
			}
			results[u] = "virustotal"
			foundCount++
		}

		log.Printf("✅ VirusTotal found %d URLs for %s\n", foundCount, domain)

		// Conservative pacing for community keys.
		time.Sleep(15 * time.Second)
	}

	log.Printf("✅ VirusTotal completed with %d URLs", len(results))
	return results
}
