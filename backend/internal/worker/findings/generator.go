package findings

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/gorm"
)

const sourceToolBuiltin = "builtin"

var assetBuiltinCategories = []string{
	"exposed-interface",
	"directory-listing",
	"server-error",
	"exposed-service",
}

var urlBuiltinCategories = []string{
	"sensitive-url",
	"exposed-config",
	"exposed-vcs",
	"api-documentation",
	"debug-endpoint",
	"backup-artifact",
}

type findingCandidate struct {
	AssetID        *uint
	URLID          *uint
	Title          string
	Description    string
	Severity       string
	Category       string
	Evidence       string
	Recommendation string
	FingerprintKey string
	StableSubject  string
}

// GenerateBuiltinFindings runs all built-in generators. It is safe to call after a full scan.
func GenerateBuiltinFindings(targetID uint) error {
	if err := GenerateAssetFindings(targetID); err != nil {
		return err
	}

	return GenerateURLFindings(targetID)
}

// GenerateAssetFindings creates findings from asset-level signals collected by httpx/nmap.
func GenerateAssetFindings(targetID uint) error {
	var assets []models.Asset
	if err := database.DB.Where("target_id = ?", targetID).Find(&assets).Error; err != nil {
		return fmt.Errorf("load target assets: %w", err)
	}

	now := time.Now()
	seen := make(map[string]struct{})

	for _, asset := range assets {
		for _, candidate := range candidatesFromAsset(asset) {
			fingerprint := stableFingerprint(targetID, candidate)

			if err := upsertFinding(targetID, fingerprint, candidate, now); err != nil {
				log.Printf("⚠️ Failed to upsert builtin asset finding for target %d asset %d: %v\n", targetID, asset.ID, err)
				continue
			}

			seen[fingerprint] = struct{}{}
		}
	}

	if err := markMissingBuiltinFindingsFixed(targetID, assetBuiltinCategories, seen, now); err != nil {
		return fmt.Errorf("mark missing builtin asset findings fixed: %w", err)
	}

	log.Printf("✅ Builtin asset findings generated for target %d: %d active candidates\n", targetID, len(seen))

	return nil
}

// GenerateURLFindings creates findings from crawled/archive URL signals.
func GenerateURLFindings(targetID uint) error {
	var urls []models.FoundURL
	if err := database.DB.Where("target_id = ?", targetID).Find(&urls).Error; err != nil {
		return fmt.Errorf("load target URLs: %w", err)
	}

	now := time.Now()
	seen := make(map[string]struct{})

	for _, foundURL := range urls {
		for _, candidate := range candidatesFromURL(foundURL) {
			fingerprint := stableFingerprint(targetID, candidate)

			if err := upsertFinding(targetID, fingerprint, candidate, now); err != nil {
				log.Printf("⚠️ Failed to upsert builtin URL finding for target %d url %d: %v\n", targetID, foundURL.ID, err)
				continue
			}

			seen[fingerprint] = struct{}{}
		}
	}

	if err := markMissingBuiltinFindingsFixed(targetID, urlBuiltinCategories, seen, now); err != nil {
		return fmt.Errorf("mark missing builtin URL findings fixed: %w", err)
	}

	log.Printf("✅ Builtin URL findings generated for target %d: %d active candidates\n", targetID, len(seen))

	return nil
}

func candidatesFromAsset(asset models.Asset) []findingCandidate {
	candidates := make([]findingCandidate, 0)
	assetID := asset.ID

	searchText := strings.ToLower(strings.Join([]string{
		asset.Value,
		asset.FinalURL,
		asset.Title,
		asset.WebServer,
		asset.Technologies,
	}, " "))

	if containsAny(searchText, []string{
		"admin",
		"login",
		"dashboard",
		"cpanel",
		"phpmyadmin",
		"jenkins",
		"grafana",
		"kibana",
	}) {
		candidates = append(candidates, findingCandidate{
			AssetID:        &assetID,
			Title:          "Possible exposed admin or login interface",
			Description:    "The asset metadata suggests an administrative, login, dashboard, or management interface may be exposed.",
			Severity:       models.FindingSeverityMedium,
			Category:       "exposed-interface",
			Evidence:       assetEvidence(asset),
			Recommendation: "Review the endpoint manually and restrict access with authentication, IP allowlisting, or network controls where appropriate.",
			FingerprintKey: "admin-interface",
		})
	}

	if strings.Contains(strings.ToLower(asset.Title), "index of /") || strings.Contains(strings.ToLower(asset.Title), "index of") {
		candidates = append(candidates, findingCandidate{
			AssetID:        &assetID,
			Title:          "Possible directory listing exposed",
			Description:    "The page title indicates that directory listing may be enabled.",
			Severity:       models.FindingSeverityMedium,
			Category:       "directory-listing",
			Evidence:       assetEvidence(asset),
			Recommendation: "Disable directory listing and verify that sensitive files are not exposed.",
			FingerprintKey: "directory-listing",
		})
	}

	if asset.StatusCode >= 500 && asset.StatusCode <= 599 {
		candidates = append(candidates, findingCandidate{
			AssetID:        &assetID,
			Title:          "Server error response observed",
			Description:    "The asset returned a 5xx HTTP response during probing.",
			Severity:       models.FindingSeverityLow,
			Category:       "server-error",
			Evidence:       assetEvidence(asset),
			Recommendation: "Investigate the failing endpoint and check whether it reveals stack traces, debug behavior, or unstable services.",
			FingerprintKey: fmt.Sprintf("server-error-%d", asset.StatusCode),
		})
	}

	for host, ports := range riskyOpenPorts(asset.OpenPorts) {
		for _, port := range ports {
			candidates = append(candidates, findingCandidate{
				AssetID:        &assetID,
				Title:          "Potentially exposed sensitive service",
				Description:    "A port commonly associated with sensitive infrastructure services appears to be open.",
				Severity:       severityForPort(port),
				Category:       "exposed-service",
				Evidence:       fmt.Sprintf("asset=%s host=%s port=%d open_ports=%s", asset.Value, host, port, asset.OpenPorts),
				Recommendation: "Confirm whether the service is intentionally exposed. Restrict access to trusted networks and enforce authentication.",
				FingerprintKey: fmt.Sprintf("open-port-%s-%d", host, port),
			})
		}
	}

	return candidates
}

func candidatesFromURL(foundURL models.FoundURL) []findingCandidate {
	candidates := make([]findingCandidate, 0)
	urlID := foundURL.ID
	assetID := foundURL.AssetID

	canonicalURL := canonicalURLForFinding(foundURL.Value)
	analysisURL := canonicalURL
	if analysisURL == "" {
		analysisURL = foundURL.Value
	}

	parsed, err := url.Parse(analysisURL)
	pathValue := ""
	queryValue := ""
	if err == nil {
		pathValue = strings.ToLower(parsed.Path)
		queryValue = strings.ToLower(parsed.RawQuery)
	}
	if pathValue == "" {
		pathValue = strings.ToLower(analysisURL)
	}

	baseName := strings.ToLower(filepath.Base(pathValue))
	fullLower := strings.ToLower(analysisURL)
	stableSubject := "url:" + analysisURL

	add := func(title, description, severity, category, recommendation, key string) {
		candidates = append(candidates, findingCandidate{
			AssetID:        assetID,
			URLID:          &urlID,
			Title:          title,
			Description:    description,
			Severity:       severity,
			Category:       category,
			Evidence:       urlEvidence(foundURL, analysisURL),
			Recommendation: recommendation,
			FingerprintKey: key,
			StableSubject:  stableSubject,
		})
	}

	if containsAny(pathValue, []string{
		"/admin",
		"/administrator",
		"/login",
		"/signin",
		"/dashboard",
		"/manage",
		"/management",
		"/console",
		"/cpanel",
		"/phpmyadmin",
		"/jenkins",
		"/grafana",
		"/kibana",
	}) {
		add(
			"Sensitive administrative or login URL discovered",
			"A crawled or archived URL appears to reference an administrative, login, or management interface.",
			models.FindingSeverityMedium,
			"sensitive-url",
			"Review whether the endpoint should be publicly reachable and enforce authentication, MFA, or network restrictions.",
			"admin-login-url",
		)
	}

	if containsAny(fullLower, []string{
		"/.env",
		"/.env.",
		"/config.php",
		"/config.json",
		"/config.yml",
		"/settings.py",
		"/web.config",
		"/application.properties",
		"/database.yml",
		"/wp-config.php",
	}) {
		add(
			"Potential exposed configuration file URL discovered",
			"A crawled or archived URL points to a file name commonly associated with application configuration or secrets.",
			models.FindingSeverityHigh,
			"exposed-config",
			"Verify whether the file is reachable. Remove public access and rotate any exposed credentials if confirmed.",
			"config-file-url",
		)
	}

	if containsAny(fullLower, []string{
		"/.git",
		"/.svn",
		"/.hg",
		"/.bzr",
	}) {
		add(
			"Potential exposed version-control path discovered",
			"A crawled or archived URL references a version-control path that should not be publicly accessible.",
			models.FindingSeverityHigh,
			"exposed-vcs",
			"Block access to version-control directories and verify whether repository metadata or source files are exposed.",
			"vcs-path-url",
		)
	}

	if containsAny(pathValue, []string{
		"/swagger",
		"/api-docs",
		"/openapi",
		"/graphql",
		"/graphiql",
		"/redoc",
	}) || containsAny(baseName, []string{"swagger.json", "openapi.json", "openapi.yaml"}) {
		add(
			"API documentation or schema URL discovered",
			"A crawled or archived URL appears to expose API documentation, schema, or an interactive API console.",
			models.FindingSeverityMedium,
			"api-documentation",
			"Review whether API documentation should be public. Remove sensitive examples, internal endpoints, and unauthenticated consoles.",
			"api-doc-url",
		)
	}

	if containsAny(pathValue, []string{
		"/debug",
		"/debug/",
		"/actuator",
		"/metrics",
		"/server-status",
		"/phpinfo",
		"/trace",
	}) || strings.Contains(queryValue, "debug=true") {
		add(
			"Debug or monitoring URL discovered",
			"A crawled or archived URL appears to reference debug, metrics, status, or diagnostic functionality.",
			models.FindingSeverityMedium,
			"debug-endpoint",
			"Verify exposure and restrict diagnostic endpoints to trusted internal networks only.",
			"debug-url",
		)
	}

	if containsAny(fullLower, []string{
		".bak",
		".backup",
		".old",
		".orig",
		".zip",
		".tar",
		".tar.gz",
		".tgz",
		".sql",
		".dump",
	}) {
		add(
			"Potential backup or archive URL discovered",
			"A crawled or archived URL references a file extension commonly used for backups, archives, or database dumps.",
			models.FindingSeverityHigh,
			"backup-artifact",
			"Confirm whether the file is reachable. Remove public access and inspect for sensitive data exposure if confirmed.",
			"backup-artifact-url",
		)
	}

	return candidates
}

func upsertFinding(targetID uint, fingerprint string, candidate findingCandidate, now time.Time) error {
	var existing models.Finding
	err := database.DB.Where("target_id = ? AND fingerprint = ?", targetID, fingerprint).First(&existing).Error

	if err == nil {
		updates := map[string]interface{}{
			"title":          candidate.Title,
			"description":    candidate.Description,
			"severity":       candidate.Severity,
			"category":       candidate.Category,
			"source_tool":    sourceToolBuiltin,
			"evidence":       candidate.Evidence,
			"recommendation": candidate.Recommendation,
			"last_seen":      now,
			"asset_id":       candidate.AssetID,
			"url_id":         candidate.URLID,
		}

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
		URLID:          candidate.URLID,
		Title:          candidate.Title,
		Description:    candidate.Description,
		Severity:       candidate.Severity,
		Category:       candidate.Category,
		SourceTool:     sourceToolBuiltin,
		Evidence:       candidate.Evidence,
		Recommendation: candidate.Recommendation,
		Status:         models.FindingStatusOpen,
		Fingerprint:    fingerprint,
		FirstSeen:      now,
		LastSeen:       now,
	}

	return database.DB.Create(&finding).Error
}

func markMissingBuiltinFindingsFixed(targetID uint, categories []string, seen map[string]struct{}, now time.Time) error {
	var existing []models.Finding
	if err := database.DB.
		Where("target_id = ? AND source_tool = ? AND category IN ? AND status <> ?", targetID, sourceToolBuiltin, categories, models.FindingStatusFixed).
		Find(&existing).Error; err != nil {
		return err
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
			return err
		}
	}

	return nil
}

func stableFingerprint(targetID uint, candidate findingCandidate) string {
	if candidate.StableSubject != "" {
		h := sha1.New()
		_, _ = h.Write([]byte(fmt.Sprintf(
			"builtin:v3:%d:%s:%s:%s:%s",
			targetID,
			candidate.StableSubject,
			candidate.Category,
			candidate.Title,
			candidate.FingerprintKey,
		)))

		return hex.EncodeToString(h.Sum(nil))
	}

	assetID := uint(0)
	if candidate.AssetID != nil {
		assetID = *candidate.AssetID
	}

	urlID := uint(0)
	if candidate.URLID != nil {
		urlID = *candidate.URLID
	}

	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf(
		"builtin:v2:%d:%d:%d:%s:%s:%s",
		targetID,
		assetID,
		urlID,
		candidate.Category,
		candidate.Title,
		candidate.FingerprintKey,
	)))

	return hex.EncodeToString(h.Sum(nil))
}

func assetEvidence(asset models.Asset) string {
	parts := []string{
		fmt.Sprintf("asset=%s", asset.Value),
		fmt.Sprintf("status=%d", asset.StatusCode),
	}

	if asset.FinalURL != "" {
		parts = append(parts, fmt.Sprintf("final_url=%s", asset.FinalURL))
	}

	if asset.Title != "" {
		parts = append(parts, fmt.Sprintf("title=%q", asset.Title))
	}

	if asset.WebServer != "" {
		parts = append(parts, fmt.Sprintf("web_server=%q", asset.WebServer))
	}

	if asset.Technologies != "" && asset.Technologies != "[]" {
		parts = append(parts, fmt.Sprintf("technologies=%s", asset.Technologies))
	}

	return strings.Join(parts, " ")
}

func urlEvidence(foundURL models.FoundURL, canonicalURL string) string {
	parts := []string{
		fmt.Sprintf("canonical_url=%s", canonicalURL),
	}

	if foundURL.Value != "" && foundURL.Value != canonicalURL {
		parts = append(parts, fmt.Sprintf("sample_raw_url=%s", shortenEvidenceValue(foundURL.Value, 240)))
	}

	if foundURL.Source != "" {
		parts = append(parts, fmt.Sprintf("source=%s", foundURL.Source))
	}

	if foundURL.AssetID != nil {
		parts = append(parts, fmt.Sprintf("asset_id=%d", *foundURL.AssetID))
	}

	return strings.Join(parts, " ")
}

func canonicalURLForFinding(rawValue string) string {
	rawValue = strings.TrimSpace(rawValue)
	if rawValue == "" {
		return ""
	}

	parsed, err := url.Parse(rawValue)
	if err != nil {
		return compactRawURL(rawValue)
	}

	parsed.Fragment = ""
	parsed.User = nil

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = canonicalHost(parsed)
	parsed.Path = canonicalPath(parsed.Path)
	parsed.RawQuery = canonicalQuery(parsed.Query())
	parsed.ForceQuery = false

	if parsed.Scheme == "" && parsed.Host == "" {
		result := parsed.Path
		if parsed.RawQuery != "" {
			result += "?" + parsed.RawQuery
		}
		return result
	}

	return parsed.String()
}

func canonicalHost(parsed *url.URL) string {
	hostname := strings.ToLower(parsed.Hostname())
	port := parsed.Port()

	if hostname == "" {
		return strings.ToLower(parsed.Host)
	}

	if port == "" || (parsed.Scheme == "http" && port == "80") || (parsed.Scheme == "https" && port == "443") {
		return hostname
	}

	return net.JoinHostPort(hostname, port)
}

func canonicalPath(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "/"
	}

	cleaned := path.Clean("/" + value)
	if cleaned != "/" {
		cleaned = strings.TrimRight(cleaned, "/")
	}

	return cleaned
}

func canonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}

	keys := make([]string, 0, len(values))
	seen := make(map[string]struct{})

	for key, vals := range values {
		canonicalKey := strings.ToLower(strings.TrimSpace(key))
		if canonicalKey == "" {
			continue
		}

		if isVolatileQueryParam(canonicalKey) || hasVolatileQueryValue(vals) {
			continue
		}

		if _, ok := seen[canonicalKey]; ok {
			continue
		}

		seen[canonicalKey] = struct{}{}
		keys = append(keys, canonicalKey)
	}

	sort.Strings(keys)

	return strings.Join(keys, "&")
}

func isVolatileQueryParam(name string) bool {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return true
	}

	if strings.HasPrefix(name, "utm_") {
		return true
	}

	exact := map[string]struct{}{
		"fbclid":       {},
		"gclid":        {},
		"msclkid":      {},
		"mc_cid":       {},
		"mc_eid":       {},
		"sid":          {},
		"jsessionid":   {},
		"phpsessid":    {},
		"aspsessionid": {},
		"ts":           {},
		"t":            {},
		"sig":          {},
		"cache":        {},
		"cachebuster":  {},
		"cb":           {},
		"rand":         {},
		"random":       {},
		"captcha":      {},
	}
	if _, ok := exact[name]; ok {
		return true
	}

	volatileSubstrings := []string{
		"csrf",
		"nonce",
		"token",
		"session",
		"signature",
		"timestamp",
		"authenticity",
		"anti-forgery",
		"xsrf",
	}

	for _, needle := range volatileSubstrings {
		if strings.Contains(name, needle) {
			return true
		}
	}

	return false
}

func hasVolatileQueryValue(values []string) bool {
	for _, value := range values {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "csrf") || strings.Contains(lower, "nonce") || strings.Contains(lower, "session") || strings.Contains(lower, "token") {
			return true
		}
	}

	return false
}

func compactRawURL(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}

	if idx := strings.Index(value, "#"); idx >= 0 {
		value = value[:idx]
	}

	return value
}

func shortenEvidenceValue(value string, maxLen int) string {
	if len(value) <= maxLen {
		return value
	}

	if maxLen <= 3 {
		return value[:maxLen]
	}

	return value[:maxLen-3] + "..."
}

func containsAny(value string, needles []string) bool {
	value = strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}

	return false
}

func riskyOpenPorts(openPortsJSON string) map[string][]int {
	results := make(map[string][]int)

	openPortsJSON = strings.TrimSpace(openPortsJSON)
	if openPortsJSON == "" || openPortsJSON == "{}" || openPortsJSON == "null" {
		return results
	}

	var raw map[string]interface{}
	if err := json.Unmarshal([]byte(openPortsJSON), &raw); err != nil {
		return results
	}

	for host, value := range raw {
		for _, port := range interfaceToPorts(value) {
			if isRiskyPort(port) {
				results[host] = append(results[host], port)
			}
		}
	}

	return results
}

func interfaceToPorts(value interface{}) []int {
	ports := make([]int, 0)

	switch typed := value.(type) {
	case []interface{}:
		for _, item := range typed {
			switch port := item.(type) {
			case float64:
				ports = append(ports, int(port))
			case int:
				ports = append(ports, port)
			}
		}
	case []int:
		ports = append(ports, typed...)
	case float64:
		ports = append(ports, int(typed))
	case int:
		ports = append(ports, typed)
	}

	return ports
}

func isRiskyPort(port int) bool {
	switch port {
	case 21, 22, 23, 25, 110, 143, 389, 445, 3306, 5432, 5900, 6379, 9200, 9300, 27017, 27018, 27019, 5000, 5601, 8086, 2375, 2376:
		return true
	default:
		return false
	}
}

func severityForPort(port int) string {
	switch port {
	case 6379, 9200, 9300, 27017, 27018, 27019, 2375, 2376:
		return models.FindingSeverityHigh
	case 3306, 5432, 5900, 5601:
		return models.FindingSeverityMedium
	default:
		return models.FindingSeverityLow
	}
}
