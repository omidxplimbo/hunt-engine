package jsintel

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	SourceToolJSIntel = "js-intel"
	StepJSIntel       = "JS_INTELLIGENCE"

	moduleCrawling = "CRAWLING"
)

type Context struct {
	TargetID uint

	RootDomain string

	CheckStop func(targetID uint) bool

	UpdateTargetPhase func(targetID uint, phase string)

	ScanIsStepDone func(targetID uint, module, step string) bool

	ScanMarkRunning func(targetID uint, module, step string)

	ScanMarkStepDone func(targetID uint, module, step string)
}

type jsURL struct {
	ID      uint
	AssetID *uint
	Value   string
	Source  string
}

type fetchMeta struct {
	StatusCode  int
	ContentType string
	BytesRead   int
	Truncated   bool
}

type secretPattern struct {
	Name        string
	Severity    string
	Description string
	Pattern     *regexp.Regexp
}

type detectedSignal struct {
	Category       string
	SignalType     string
	Severity       string
	Title          string
	Description    string
	Recommendation string
	Evidence       map[string]interface{}
}

var secretPatterns = []secretPattern{
	{
		Name:        "AWS Access Key",
		Severity:    models.FindingSeverityHigh,
		Description: "A value matching the AWS access key ID format was observed in a JavaScript file.",
		Pattern:     regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	},
	{
		Name:        "Google API Key",
		Severity:    models.FindingSeverityMedium,
		Description: "A value matching the Google API key format was observed in a JavaScript file.",
		Pattern:     regexp.MustCompile(`AIza[0-9A-Za-z\-_]{35}`),
	},
	{
		Name:        "GitHub Token",
		Severity:    models.FindingSeverityHigh,
		Description: "A value matching a GitHub token format was observed in a JavaScript file.",
		Pattern:     regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{36,255}`),
	},
	{
		Name:        "Slack Token",
		Severity:    models.FindingSeverityHigh,
		Description: "A value matching a Slack token format was observed in a JavaScript file.",
		Pattern:     regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	},
	{
		Name:        "Stripe Live Secret Key",
		Severity:    models.FindingSeverityHigh,
		Description: "A value matching a Stripe live secret key format was observed in a JavaScript file.",
		Pattern:     regexp.MustCompile(`sk_live_[0-9a-zA-Z]{16,}`),
	},
	{
		Name:        "JWT",
		Severity:    models.FindingSeverityMedium,
		Description: "A JWT-like token was observed in a JavaScript file.",
		Pattern:     regexp.MustCompile(`eyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}`),
	},
}

var (
	sourceMapRegex = regexp.MustCompile(`(?m)//[#@]\s*sourceMappingURL=([^\s]+)`)
	endpointRegex  = regexp.MustCompile(`["']((?:/|https?://)[A-Za-z0-9_\-./?&=%:#]+)["']`)
	scriptSrcRegex = regexp.MustCompile(`(?i)<script[^>]+src=["']([^"']+)["']`)
)

func Run(ctx Context) error {
	if ctx.TargetID == 0 {
		return nil
	}

	if ctx.CheckStop != nil && ctx.CheckStop(ctx.TargetID) {
		return fmt.Errorf("process killed by user request")
	}

	if ctx.ScanIsStepDone != nil && ctx.ScanIsStepDone(ctx.TargetID, moduleCrawling, StepJSIntel) {
		log.Printf("⏩ Resume: skipping JS intelligence for target %d\n", ctx.TargetID)
		return nil
	}

	if ctx.UpdateTargetPhase != nil {
		ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 3: JS INTELLIGENCE")
	}

	if ctx.ScanMarkRunning != nil {
		ctx.ScanMarkRunning(ctx.TargetID, moduleCrawling, StepJSIntel)
	}

	client := &http.Client{Timeout: httpTimeout()}
	maxBytes := maxDownloadBytes()

	candidates, err := loadJSURLs(ctx.TargetID, ctx.RootDomain)
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		assetCandidates, err := discoverJSURLsFromAssets(ctx.TargetID, ctx.RootDomain, client, maxBytes)
		if err != nil {
			log.Printf("⚠️ JS intelligence asset discovery failed for target %d: %v\n", ctx.TargetID, err)
		} else {
			candidates = append(candidates, assetCandidates...)
		}
	}
	now := time.Now()
	seen := make(map[string]struct{})

	for _, candidate := range candidates {
		if ctx.CheckStop != nil && ctx.CheckStop(ctx.TargetID) {
			return fmt.Errorf("process killed by user request")
		}

		body, meta, err := fetchJS(client, candidate.Value, maxBytes)
		if err != nil {
			log.Printf("⚠️ JS intelligence fetch failed for target %d url %s: %v\n", ctx.TargetID, candidate.Value, err)
			continue
		}

		signals := analyze(candidate, meta, body, ctx.RootDomain)

		for _, signal := range signals {
			fingerprint := fingerprintForSignal(ctx.TargetID, candidate, signal)

			if err := upsertFinding(ctx.TargetID, fingerprint, candidate, signal, meta, now); err != nil {
				log.Printf("⚠️ Failed to upsert JS intelligence finding for target %d url %s: %v\n", ctx.TargetID, candidate.Value, err)
				continue
			}

			seen[fingerprint] = struct{}{}
		}
	}

	if err := markMissingFindingsFixed(ctx.TargetID, seen, now); err != nil {
		return err
	}

	if ctx.ScanMarkStepDone != nil {
		ctx.ScanMarkStepDone(ctx.TargetID, moduleCrawling, StepJSIntel)
	}

	log.Printf("✅ JS intelligence complete for target %d: %d active findings from %d JS candidates\n", ctx.TargetID, len(seen), len(candidates))
	return nil
}

func loadJSURLs(targetID uint, rootDomain string) ([]jsURL, error) {
	var found []models.FoundURL

	if err := database.DB.
		Where("target_id = ?", targetID).
		Order("id").
		Find(&found).Error; err != nil {
		return nil, fmt.Errorf("load found urls for JS intelligence: %w", err)
	}

	limit := maxURLs()
	seen := make(map[string]struct{})
	out := make([]jsURL, 0)

	for _, item := range found {
		raw := strings.TrimSpace(item.Value)
		if raw == "" || !isLikelyJSURL(raw) || !isAllowedJSURL(raw, rootDomain) {
			continue
		}

		key := normalizeURLKey(raw)
		if key == "" {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}

		out = append(out, jsURL{
			ID:      item.ID,
			AssetID: item.AssetID,
			Value:   raw,
			Source:  item.Source,
		})

		if limit > 0 && len(out) >= limit {
			break
		}
	}

	return out, nil
}

func discoverJSURLsFromAssets(targetID uint, rootDomain string, client *http.Client, maxBytes int64) ([]jsURL, error) {
	var assets []models.Asset

	if err := database.DB.
		Where("target_id = ? AND is_live = ?", targetID, true).
		Order("id").
		Limit(maxHTMLPages()).
		Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("load live assets for JS intelligence: %w", err)
	}

	seen := make(map[string]struct{})
	out := make([]jsURL, 0)
	limit := maxURLs()

	for _, asset := range assets {
		pageURL := asset.FinalURL
		if strings.TrimSpace(pageURL) == "" {
			pageURL = "https://" + strings.TrimSpace(asset.Value)
		}

		if !isHTTPURL(pageURL) {
			continue
		}

		body, _, err := fetchText(client, pageURL, maxBytes)
		if err != nil {
			log.Printf("⚠️ JS intelligence page fetch failed for target %d url %s: %v\n", targetID, pageURL, err)
			continue
		}

		scripts := extractScriptURLs(pageURL, body, rootDomain)
		for _, scriptURL := range scripts {
			key := normalizeURLKey(scriptURL)
			if key == "" {
				continue
			}

			if _, ok := seen[key]; ok {
				continue
			}

			seen[key] = struct{}{}

			out = append(out, jsURL{
				ID:      0,
				AssetID: &asset.ID,
				Value:   scriptURL,
				Source:  "js-intel-html",
			})

			if limit > 0 && len(out) >= limit {
				return out, nil
			}
		}
	}

	return out, nil
}

func extractScriptURLs(pageURL string, body string, rootDomain string) []string {
	base, err := url.Parse(pageURL)
	if err != nil {
		return nil
	}

	matches := scriptSrcRegex.FindAllStringSubmatch(body, -1)
	out := make([]string, 0)
	seen := make(map[string]struct{})

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		raw := strings.TrimSpace(match[1])
		if raw == "" {
			continue
		}

		parsed, err := url.Parse(raw)
		if err != nil {
			continue
		}

		resolved := base.ResolveReference(parsed).String()
		if !isLikelyJSURL(resolved) || !isAllowedJSURL(resolved, rootDomain) {
			continue
		}

		key := normalizeURLKey(resolved)
		if key == "" {
			continue
		}

		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		out = append(out, resolved)
	}

	return out
}

func isAllowedJSURL(raw string, rootDomain string) bool {
	rootDomain = strings.TrimSpace(strings.ToLower(rootDomain))
	if rootDomain == "" {
		return true
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}

	host := strings.TrimSpace(strings.ToLower(parsed.Hostname()))
	if host == "" {
		return false
	}

	return host == rootDomain || strings.HasSuffix(host, "."+rootDomain)
}

func isLikelyJSURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return false
	}

	path := strings.ToLower(parsed.Path)

	return strings.HasSuffix(path, ".js") ||
		strings.HasSuffix(path, ".mjs") ||
		strings.HasSuffix(path, ".js.map") ||
		strings.HasSuffix(path, ".map")
}

func normalizeURLKey(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}

	parsed.Fragment = ""

	return parsed.String()
}

func fetchText(client *http.Client, raw string, maxBytes int64) (string, fetchMeta, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return "", fetchMeta{}, err
	}

	req.Header.Set("User-Agent", "HuntEngine-JSIntel/1.0")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/javascript,text/javascript,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fetchMeta{}, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBytes+1)

	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fetchMeta{}, err
	}

	truncated := int64(len(body)) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}

	meta := fetchMeta{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		BytesRead:   len(body),
		Truncated:   truncated,
	}

	if resp.StatusCode >= 500 {
		return "", meta, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return string(body), meta, nil
}

func isHTTPURL(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}

	return parsed.Scheme == "http" || parsed.Scheme == "https"
}

func fetchJS(client *http.Client, raw string, maxBytes int64) (string, fetchMeta, error) {
	req, err := http.NewRequest(http.MethodGet, raw, nil)
	if err != nil {
		return "", fetchMeta{}, err
	}

	req.Header.Set("User-Agent", "HuntEngine-JSIntel/1.0")
	req.Header.Set("Accept", "application/javascript,text/javascript,*/*;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return "", fetchMeta{}, err
	}
	defer resp.Body.Close()

	limited := io.LimitReader(resp.Body, maxBytes+1)

	body, err := io.ReadAll(limited)
	if err != nil {
		return "", fetchMeta{}, err
	}

	truncated := int64(len(body)) > maxBytes
	if truncated {
		body = body[:maxBytes]
	}

	meta := fetchMeta{
		StatusCode:  resp.StatusCode,
		ContentType: resp.Header.Get("Content-Type"),
		BytesRead:   len(body),
		Truncated:   truncated,
	}

	if resp.StatusCode >= 500 {
		return "", meta, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return string(body), meta, nil
}

func analyze(candidate jsURL, meta fetchMeta, body string, rootDomain string) []detectedSignal {
	signals := make([]detectedSignal, 0)

	for _, pattern := range secretPatterns {
		matches := uniqueLimited(pattern.Pattern.FindAllString(body, -1), 10)
		if len(matches) == 0 {
			continue
		}

		redacted := make([]string, 0, len(matches))
		for _, match := range matches {
			redacted = append(redacted, redact(match))
		}

		signals = append(signals, detectedSignal{
			Category:       "js-secret",
			SignalType:     pattern.Name,
			Severity:       pattern.Severity,
			Title:          fmt.Sprintf("Potential JavaScript secret exposure: %s", pattern.Name),
			Description:    pattern.Description + " The value is redacted in evidence and must be manually verified before remediation.",
			Recommendation: "Review the JavaScript source, rotate the exposed credential if valid, move secrets server-side, and verify whether the key has exploitable permissions.",
			Evidence: map[string]interface{}{
				"secret_type":      pattern.Name,
				"redacted_matches": redacted,
				"matches_count":    len(matches),
			},
		})
	}

	if sourceMap := detectSourceMap(candidate.Value, body); sourceMap != "" {
		signals = append(signals, detectedSignal{
			Category:       "js-source-map",
			SignalType:     "source-map",
			Severity:       models.FindingSeverityLow,
			Title:          "JavaScript source map reference observed",
			Description:    "A JavaScript file references a source map or appears to be a source map. Source maps can expose original source structure, comments, routes, and internal implementation details.",
			Recommendation: "Confirm whether the source map is intentionally public. If not required in production, disable source map publication or restrict access.",
			Evidence: map[string]interface{}{
				"source_map": sourceMap,
			},
		})
	}

	endpoints := extractInterestingEndpoints(body, rootDomain)
	if len(endpoints) > 0 {
		signals = append(signals, detectedSignal{
			Category:       "js-endpoints",
			SignalType:     "interesting-endpoints",
			Severity:       models.FindingSeverityInfo,
			Title:          "JavaScript exposes interesting endpoints",
			Description:    "The JavaScript file references API, authentication, admin, debug, or internal-looking endpoints that may be useful for manual security review.",
			Recommendation: "Review the discovered endpoints for authorization, sensitive data exposure, debug behavior, and missing access controls.",
			Evidence: map[string]interface{}{
				"endpoints":       endpoints,
				"endpoints_count": len(endpoints),
			},
		})
	}

	return signals
}

func detectSourceMap(jsURL string, body string) string {
	parsed, err := url.Parse(jsURL)
	if err == nil {
		path := strings.ToLower(parsed.Path)
		if strings.HasSuffix(path, ".map") || strings.HasSuffix(path, ".js.map") {
			return jsURL
		}
	}

	matches := sourceMapRegex.FindStringSubmatch(body)
	if len(matches) < 2 {
		return ""
	}

	return strings.TrimSpace(matches[1])
}

func extractInterestingEndpoints(body string, rootDomain string) []string {
	matches := endpointRegex.FindAllStringSubmatch(body, -1)

	seen := make(map[string]struct{})
	endpoints := make([]string, 0)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}

		value := strings.TrimSpace(match[1])
		if value == "" || !isInterestingEndpoint(value, rootDomain) {
			continue
		}

		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		endpoints = append(endpoints, value)

		if len(endpoints) >= 50 {
			break
		}
	}

	sort.Strings(endpoints)
	return endpoints
}

func isInterestingEndpoint(value string, rootDomain string) bool {
	lower := strings.ToLower(value)

	keywords := []string{
		"/api",
		"/graphql",
		"/rest",
		"/auth",
		"/oauth",
		"/login",
		"/admin",
		"/internal",
		"/private",
		"/config",
		"/debug",
		"/v1/",
		"/v2/",
		"/v3/",
	}

	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		parsed, err := url.Parse(lower)
		if err != nil {
			return false
		}

		if rootDomain != "" && !strings.HasSuffix(parsed.Hostname(), strings.ToLower(rootDomain)) {
			return false
		}
	}

	for _, keyword := range keywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	return false
}

func upsertFinding(targetID uint, fingerprint string, candidate jsURL, signal detectedSignal, meta fetchMeta, now time.Time) error {
	var urlID *uint
	if candidate.ID > 0 {
		id := candidate.ID
		urlID = &id
	}

	evidence := map[string]interface{}{
		"js_url":       candidate.Value,
		"url_id":       urlID,
		"asset_id":     candidate.AssetID,
		"source":       candidate.Source,
		"status_code":  meta.StatusCode,
		"content_type": meta.ContentType,
		"bytes_read":   meta.BytesRead,
		"truncated":    meta.Truncated,
		"signal_type":  signal.SignalType,
		"category":     signal.Category,
	}

	for key, value := range signal.Evidence {
		evidence[key] = value
	}

	evidenceBytes, err := json.Marshal(evidence)
	if err != nil {
		return err
	}

	var existing models.Finding

	err = database.DB.
		Where("target_id = ? AND fingerprint = ?", targetID, fingerprint).
		First(&existing).Error

	updates := map[string]interface{}{
		"title":          signal.Title,
		"description":    signal.Description,
		"severity":       signal.Severity,
		"category":       signal.Category,
		"source_tool":    SourceToolJSIntel,
		"evidence":       string(evidenceBytes),
		"evidence_json":  datatypes.JSON(evidenceBytes),
		"recommendation": signal.Recommendation,
		"last_seen":      now,
		"url_id":         urlID,
		"asset_id":       candidate.AssetID,
	}

	if err == nil {
		if existing.Status == models.FindingStatusFixed {
			updates["status"] = models.FindingStatusOpen
		}

		return database.DB.Model(&existing).Updates(updates).Error
	}

	if err != nil && err != gorm.ErrRecordNotFound {
		return err
	}

	finding := models.Finding{
		TargetID:       targetID,
		AssetID:        candidate.AssetID,
		URLID:          urlID,
		Title:          signal.Title,
		Description:    signal.Description,
		Severity:       signal.Severity,
		Category:       signal.Category,
		SourceTool:     SourceToolJSIntel,
		Evidence:       string(evidenceBytes),
		EvidenceJSON:   datatypes.JSON(evidenceBytes),
		Recommendation: signal.Recommendation,
		Status:         models.FindingStatusOpen,
		Fingerprint:    fingerprint,
		FirstSeen:      now,
		LastSeen:       now,
	}

	return database.DB.Create(&finding).Error
}

func markMissingFindingsFixed(targetID uint, seen map[string]struct{}, now time.Time) error {
	var existing []models.Finding

	if err := database.DB.
		Where("target_id = ? AND source_tool = ? AND status <> ?", targetID, SourceToolJSIntel, models.FindingStatusFixed).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing JS intelligence findings: %w", err)
	}

	for _, finding := range existing {
		if finding.Fingerprint == "" {
			continue
		}

		if _, ok := seen[finding.Fingerprint]; ok {
			continue
		}

		if err := database.DB.Model(&finding).Updates(map[string]interface{}{
			"status":    models.FindingStatusFixed,
			"last_seen": now,
		}).Error; err != nil {
			return fmt.Errorf("mark stale JS intelligence finding fixed: %w", err)
		}
	}

	return nil
}

func fingerprintForSignal(targetID uint, candidate jsURL, signal detectedSignal) string {
	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf("js-intel:v1:%d:%s:%s:%s", targetID, candidate.Value, signal.Category, signal.SignalType)))
	return hex.EncodeToString(h.Sum(nil))
}

func uniqueLimited(values []string, limit int) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}

		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		out = append(out, value)

		if limit > 0 && len(out) >= limit {
			break
		}
	}

	return out
}

func redact(value string) string {
	value = strings.TrimSpace(value)

	if len(value) <= 10 {
		return "***"
	}

	return value[:4] + "..." + value[len(value)-4:]
}

func maxHTMLPages() int {
	raw := strings.TrimSpace(os.Getenv("JS_INTEL_MAX_HTML_PAGES"))
	if raw == "" {
		return 50
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 50
	}

	return value
}

func maxURLs() int {
	raw := strings.TrimSpace(os.Getenv("JS_INTEL_MAX_URLS"))
	if raw == "" {
		return 200
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 200
	}

	return value
}

func maxDownloadBytes() int64 {
	raw := strings.TrimSpace(os.Getenv("JS_INTEL_MAX_BYTES"))
	if raw == "" {
		return 1048576
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 1048576
	}

	return value
}

func httpTimeout() time.Duration {
	raw := strings.TrimSpace(os.Getenv("JS_INTEL_HTTP_TIMEOUT_SECONDS"))
	if raw == "" {
		return 10 * time.Second
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 10 * time.Second
	}

	return time.Duration(value) * time.Second
}
