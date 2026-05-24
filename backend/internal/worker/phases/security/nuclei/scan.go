package nuclei

import (
	"bufio"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	workerutils "github.com/omidxplimbo/hunt-engine/backend/internal/worker/utils"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	SourceToolNuclei = "nuclei"
	StepSecurityScan = "NUCLEI_SECURITY_ENGINE"
	moduleProbing    = "PROBING"
)

type RunCommandFunc func(targetID uint, name string, args ...string) ([]byte, error)
type RunCommandWithTimeoutFunc func(targetID uint, timeout time.Duration, name string, args ...string) ([]byte, error)

type Context struct {
	TargetID   uint
	RootDomain string

	CheckStop         func(targetID uint) bool
	UpdateTargetPhase func(targetID uint, phase string)
	ScanIsStepDone    func(targetID uint, module, step string) bool
	ScanMarkRunning   func(targetID uint, module, step string)
	ScanMarkStepDone  func(targetID uint, module, step string)

	RunCommand            RunCommandFunc
	RunCommandWithTimeout RunCommandWithTimeoutFunc
}

type resultInfo struct {
	Name        string          `json:"name"`
	Severity    string          `json:"severity"`
	Description string          `json:"description"`
	Remediation string          `json:"remediation"`
	Tags        json.RawMessage `json:"tags"`
}

type resultRow struct {
	TemplateID       string     `json:"template-id"`
	TemplatePath     string     `json:"template-path"`
	MatcherName      string     `json:"matcher-name"`
	Type             string     `json:"type"`
	Host             string     `json:"host"`
	MatchedAt        string     `json:"matched-at"`
	IP               string     `json:"ip"`
	ExtractedResults []string   `json:"extracted-results"`
	Info             resultInfo `json:"info"`
}

func Run(ctx Context) error {
	if ctx.TargetID == 0 {
		return nil
	}
	if ctx.CheckStop != nil && ctx.CheckStop(ctx.TargetID) {
		return fmt.Errorf("process killed by user request")
	}

	var target models.Target
	if err := database.DB.First(&target, ctx.TargetID).Error; err != nil {
		return fmt.Errorf("load target config for nuclei: %w", err)
	}

	if !target.UseNuclei {
		log.Printf("⏩ Skipping Nuclei for target %d (%s): disabled in target config\n", ctx.TargetID, ctx.RootDomain)
		if ctx.ScanMarkStepDone != nil {
			ctx.ScanMarkStepDone(ctx.TargetID, moduleProbing, StepSecurityScan)
		}
		return nil
	}

	if ctx.ScanIsStepDone != nil && ctx.ScanIsStepDone(ctx.TargetID, moduleProbing, StepSecurityScan) {
		log.Printf("⏩ Resume: skipping Nuclei security scan for target %d\n", ctx.TargetID)
		return nil
	}

	if ctx.RunCommand == nil && ctx.RunCommandWithTimeout == nil {
		return fmt.Errorf("nuclei runner has no command executor")
	}

	cfg := LoadConfig()
	if target.CreatedByUserID > 0 {
		cfg.CustomTemplatesDir = filepath.Join(cfg.CustomTemplatesDir, "users", fmt.Sprintf("%d", target.CreatedByUserID))
		if err := syncNucleiTemplateFileCache(cfg.CustomTemplatesDir, target.CreatedByUserID); err != nil {
			log.Printf("⚠️ Failed to sync Nuclei template file cache for user %d: %v\n", target.CreatedByUserID, err)
		}
	}
	profile := target.NucleiProfile
	if strings.TrimSpace(profile) == "" {
		profile = cfg.DefaultProfile
	}
	profile = NormalizeProfile(profile)

	targets, err := loadNucleiTargets(ctx.TargetID)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		log.Printf("⏩ Skipping Nuclei for target %d: no probed live URLs found\n", ctx.TargetID)
		if ctx.ScanMarkStepDone != nil {
			ctx.ScanMarkStepDone(ctx.TargetID, moduleProbing, StepSecurityScan)
		}
		return nil
	}

	tempDir, _, err := workerutils.GetTargetTempDir(ctx.TargetID)
	if err != nil {
		return fmt.Errorf("init nuclei workspace: %w", err)
	}
	inputFile := filepath.Join(tempDir, "nuclei_targets.txt")
	outputFile := filepath.Join(tempDir, "nuclei_output.jsonl")

	if err := writeLines(inputFile, targets); err != nil {
		return fmt.Errorf("write nuclei target list: %w", err)
	}
	_ = os.Remove(outputFile)

	if ctx.UpdateTargetPhase != nil {
		ctx.UpdateTargetPhase(ctx.TargetID, fmt.Sprintf("PHASE 2: NUCLEI SECURITY SCAN (%s)", strings.ToUpper(profile)))
	}
	if ctx.ScanMarkRunning != nil {
		ctx.ScanMarkRunning(ctx.TargetID, moduleProbing, StepSecurityScan)
	}

	args := buildArgs(cfg, profile, inputFile, outputFile)

	log.Printf("🔎 Nuclei debug target %d: targets_file=%s targets_count=%d output_file=%s", target.ID, inputFile, countNonEmptyFileLines(inputFile), outputFile)
	log.Printf("🔎 Nuclei debug target %d: args=%s", target.ID, strings.Join(args, " "))

	if !hasTemplateArgument(args) {
		return fmt.Errorf("nuclei has no templates available: built-in dir %q and custom dir %q contain no YAML templates", cfg.TemplatesDir, cfg.CustomTemplatesDir)
	}

	var output []byte
	if ctx.RunCommandWithTimeout != nil && cfg.Timeout > 0 {
		output, err = ctx.RunCommandWithTimeout(ctx.TargetID, cfg.Timeout, "nuclei", args...)
	} else {
		output, err = ctx.RunCommand(ctx.TargetID, "nuclei", args...)
	}
	if len(output) > 0 {
		log.Printf("Nuclei output for target %d: %s\n", ctx.TargetID, strings.TrimSpace(string(output)))
	}
	if err != nil {
		if _, statErr := os.Stat(outputFile); statErr != nil {
			return fmt.Errorf("nuclei command failed: %w", err)
		}
		log.Printf("⚠️ Nuclei command returned error but output file exists; ingesting partial output: %v\n", err)
	}

	assetIndex, err := loadAssetIndex(ctx.TargetID)
	if err != nil {
		return err
	}
	seen, err := ingestOutput(ctx.TargetID, outputFile, profile, assetIndex)
	if err != nil {
		return err
	}
	if err := markMissingNucleiFindingsFixed(ctx.TargetID, seen); err != nil {
		return err
	}

	if ctx.ScanMarkStepDone != nil {
		ctx.ScanMarkStepDone(ctx.TargetID, moduleProbing, StepSecurityScan)
	}

	log.Printf("🔎 Nuclei debug target %d: output_size=%d output_lines=%d after command", target.ID, fileSizeBytes(outputFile), countNonEmptyFileLines(outputFile))
	log.Printf("✅ Nuclei security scan complete for target %d: %d active findings\n", ctx.TargetID, len(seen))
	return nil
}

func syncNucleiTemplateFileCache(root string, userID uint) error {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" || userID == 0 {
		return nil
	}

	var templates []models.NucleiTemplate
	if err := database.DB.Where("user_id = ? AND enabled = ?", userID, true).Find(&templates).Error; err != nil {
		return fmt.Errorf("load user nuclei templates: %w", err)
	}

	expected := map[string]string{}
	for _, template := range templates {
		placement := strings.ToLower(strings.TrimSpace(template.Placement))
		switch placement {
		case "root", "shared", "safe", "fast", "exposure", "balanced", "misconfig", "cves", "cves-light", "full", "custom":
		default:
			placement = "root"
		}

		name := filepath.Base(template.Name)
		lowerName := strings.ToLower(name)
		if name == "." || name == string(os.PathSeparator) || (!strings.HasSuffix(lowerName, ".yaml") && !strings.HasSuffix(lowerName, ".yml")) {
			continue
		}

		path := filepath.Join(root, name)
		if placement != "root" {
			path = filepath.Join(root, placement, name)
		}

		expected[filepath.Clean(path)] = strings.TrimSpace(template.Content) + "\n"
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}

	for path, content := range expected {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if existing, err := os.ReadFile(path); err == nil && string(existing) == content {
			continue
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			return err
		}
	}

	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}

		lowerName := strings.ToLower(entry.Name())
		if !strings.HasSuffix(lowerName, ".yaml") && !strings.HasSuffix(lowerName, ".yml") {
			return nil
		}

		clean := filepath.Clean(path)
		if _, ok := expected[clean]; !ok {
			_ = os.Remove(clean)
		}

		return nil
	})

	return nil
}

func nucleiRequestTimeoutSeconds() int {
	value := nucleiIntFromEnv("NUCLEI_REQUEST_TIMEOUT_SECONDS", 10)
	if value <= 0 {
		return 10
	}
	return value
}

func nucleiRetries() int {
	value := nucleiIntFromEnv("NUCLEI_RETRIES", 0)
	if value < 0 {
		return 0
	}
	return value
}

func nucleiIntFromEnv(name string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		log.Printf("⚠️ Invalid %s value %q; falling back to %d", name, raw, fallback)
		return fallback
	}

	return value
}

func countNonEmptyFileLines(path string) int {
	file, err := os.Open(path)
	if err != nil {
		return 0
	}
	defer file.Close()

	count := 0
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	for scanner.Scan() {
		if strings.TrimSpace(scanner.Text()) != "" {
			count++
		}
	}
	return count
}

func fileSizeBytes(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func buildArgs(cfg Config, profile, inputFile, outputFile string) []string {
	args := []string{
		"-no-stdin",
		"-l", inputFile,
		"-jsonl",
		"-o", outputFile,
		"-silent",
		"-no-color",
		"-disable-update-check",
		"-rl", fmt.Sprintf("%d", positiveOrDefault(cfg.RateLimit, DefaultRateLimit)),
		"-c", fmt.Sprintf("%d", positiveOrDefault(cfg.Concurrency, DefaultConcurrency)),
		"-bs", fmt.Sprintf("%d", positiveOrDefault(cfg.BulkSize, DefaultBulkSize)),
	}

	requestTimeoutSeconds := nucleiRequestTimeoutSeconds()
	if requestTimeoutSeconds > 0 {
		args = append(args, "-timeout", fmt.Sprintf("%d", requestTimeoutSeconds))
	}
	retries := nucleiRetries()
	if retries >= 0 {
		args = append(args, "-retries", fmt.Sprintf("%d", retries))
	}
	for _, templatePath := range builtinTemplateArgs(cfg.TemplatesDir, profile) {

		args = append(args, "-t", templatePath)

	}

	for _, templatePath := range customTemplateArgs(cfg.CustomTemplatesDir, profile) {
		args = append(args, "-t", templatePath)
	}

	return append(args, profileArgs(profile)...)
}

func hasTemplateArgument(args []string) bool {
	for i, arg := range args {
		if arg != "-t" {
			continue
		}
		if i+1 < len(args) && strings.TrimSpace(args[i+1]) != "" {
			return true
		}
	}

	return false
}

func builtinTemplateArgs(root string, profile string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}

	normalized := NormalizeProfile(profile)

	if normalized == "full" || normalized == "custom" {
		if directoryHasTemplates(root) {
			return []string{root}
		}
		return nil
	}

	candidates := make([]string, 0, 16)
	addDir := func(names ...string) {
		for _, name := range names {
			candidates = append(candidates, filepath.Join(root, name))
		}
	}

	switch normalized {
	case "fast", "exposure":
		addDir(
			"http/exposures",
			"http/exposed-panels",
			"http/default-logins",
			"exposures",
			"exposed-panels",
			"default-logins",
		)
	case "balanced", "misconfig":
		addDir(
			"http/exposures",
			"http/exposed-panels",
			"http/default-logins",
			"http/misconfiguration",
			"http/vulnerabilities/generic",
			"exposures",
			"exposed-panels",
			"default-logins",
			"misconfiguration",
			"vulnerabilities/generic",
		)
	case "cves-light":
		addDir(
			"http/cves",
			"cves",
		)
	default:
		addDir(
			"http/exposures",
			"http/misconfiguration",
			"exposures",
			"misconfiguration",
		)
	}

	out := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		if candidate == "" || candidate == "." {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		if !templatePathHasTemplates(candidate) {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	sort.Strings(out)
	return out
}

func customTemplateArgs(root, profile string) []string {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}

	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil
	}

	normalized := NormalizeProfile(profile)
	candidates := make([]string, 0, 12)
	candidates = append(candidates, rootTemplateFiles(root)...)

	addDir := func(names ...string) {
		for _, name := range names {
			candidates = append(candidates, filepath.Join(root, name))
		}
	}

	// Root YAML files plus shared/safe directories are available to every profile.
	// Profile-specific subdirectories are included only for profiles that should run them.
	addDir("shared", "safe")

	switch normalized {
	case "fast", "exposure":
		addDir("fast", "exposure")
	case "balanced", "misconfig":
		addDir("fast", "exposure", "balanced", "misconfig")
	case "cves-light":
		addDir("cves", "cves-light")
	case "full", "custom":
		addDir("fast", "exposure", "balanced", "misconfig", "cves", "cves-light", "full", "custom")
	default:
		// safe/default keeps root YAML files plus shared/safe custom templates only.
	}

	out := make([]string, 0, len(candidates))
	seen := map[string]struct{}{}
	for _, candidate := range candidates {
		candidate = filepath.Clean(strings.TrimSpace(candidate))
		if candidate == "" || candidate == "." {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		if !templatePathHasTemplates(candidate) {
			continue
		}
		seen[candidate] = struct{}{}
		out = append(out, candidate)
	}

	sort.Strings(out)
	return out
}

func rootTemplateFiles(root string) []string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			out = append(out, filepath.Join(root, entry.Name()))
		}
	}

	sort.Strings(out)
	return out
}

func templatePathHasTemplates(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}

	if !info.IsDir() {
		name := strings.ToLower(info.Name())
		return strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml")
	}

	return directoryHasTemplatesRecursive(path)
}

func directoryHasTemplatesRecursive(dir string) bool {
	found := false
	_ = filepath.WalkDir(dir, func(_ string, entry os.DirEntry, err error) error {
		if err != nil || found {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			found = true
		}
		return nil
	})
	return found
}

func profileArgs(profile string) []string {
	switch NormalizeProfile(profile) {
	case "full":
		return []string{"-severity", "info,low,medium,high,critical"}
	case "custom":
		return []string{"-severity", "info,low,medium,high,critical"}
	case "cves-light":
		return []string{"-tags", "cve", "-severity", "medium,high,critical", "-exclude-tags", "dos,bruteforce,brute-force,intrusive,destructive,fuzz"}
	case "misconfig", "balanced":
		return []string{"-tags", "misconfig,exposure,panel,default-login", "-severity", "medium,high,critical", "-exclude-tags", "dos,bruteforce,brute-force,intrusive,destructive,fuzz"}
	case "exposure", "fast":
		return []string{"-tags", "exposure,panel,default-login", "-severity", "medium,high,critical", "-exclude-tags", "dos,bruteforce,brute-force,intrusive,destructive,fuzz"}
	default:
		return []string{"-severity", "medium,high,critical", "-exclude-tags", "dos,bruteforce,brute-force,intrusive,destructive,fuzz"}
	}
}

func positiveOrDefault(value, fallback int) int {
	if value <= 0 {
		return fallback
	}
	return value
}

func directoryHasTemplates(dir string) bool {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return false
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			return true
		}
		name := strings.ToLower(entry.Name())
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			return true
		}
	}
	return false
}

func loadNucleiTargets(targetID uint) ([]string, error) {
	var assets []models.Asset
	if err := database.DB.Select("id", "value", "final_url", "is_live").
		Where("target_id = ? AND is_live = ?", targetID, true).
		Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("load live assets for nuclei: %w", err)
	}

	seen := make(map[string]struct{})
	for _, asset := range assets {
		for _, candidate := range []string{asset.FinalURL, asset.Value} {
			candidate = normalizeURLTarget(candidate)
			if candidate == "" {
				continue
			}
			seen[candidate] = struct{}{}
		}
	}

	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out, nil
}

func normalizeURLTarget(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return ""
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ""
	}
	parsed.Fragment = ""
	return parsed.String()
}

func writeLines(filename string, lines []string) error {
	return os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}

func loadAssetIndex(targetID uint) (map[string]uint, error) {
	var assets []models.Asset
	if err := database.DB.Select("id", "value", "final_url").Where("target_id = ?", targetID).Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("load asset index for nuclei: %w", err)
	}
	index := make(map[string]uint, len(assets)*2)
	for _, asset := range assets {
		for _, key := range assetKeys(asset) {
			index[key] = asset.ID
		}
	}
	return index, nil
}

func assetKeys(asset models.Asset) []string {
	keys := []string{strings.ToLower(strings.TrimSpace(asset.Value))}
	for _, raw := range []string{asset.FinalURL, asset.Value} {
		if parsed, err := url.Parse(strings.TrimSpace(raw)); err == nil {
			if parsed.Hostname() != "" {
				keys = append(keys, strings.ToLower(parsed.Hostname()))
			}
			if parsed.String() != "" {
				keys = append(keys, strings.ToLower(parsed.String()))
			}
		}
	}
	out := make([]string, 0, len(keys))
	seen := map[string]struct{}{}
	for _, key := range keys {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	return out
}

func ingestOutput(targetID uint, outputFile, profile string, assetIndex map[string]uint) (map[string]struct{}, error) {
	file, err := os.Open(outputFile)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]struct{}{}, nil
		}
		return nil, fmt.Errorf("open nuclei output: %w", err)
	}
	defer file.Close()

	seen := make(map[string]struct{})
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024*1024), 64*1024*1024)
	now := time.Now()
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var row resultRow
		if err := json.Unmarshal([]byte(line), &row); err != nil {
			log.Printf("⚠️ Invalid Nuclei JSONL row at line %d for target %d: %v\n", lineNumber, targetID, err)
			continue
		}
		fingerprint := fingerprintForResult(targetID, row)
		if fingerprint == "" {
			continue
		}
		assetID := findAssetID(row, assetIndex)
		if err := upsertNucleiFinding(targetID, fingerprint, row, assetID, profile, now); err != nil {
			log.Printf("⚠️ Failed to upsert Nuclei finding for target %d template %s: %v\n", targetID, row.TemplateID, err)
			continue
		}
		seen[fingerprint] = struct{}{}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read nuclei output: %w", err)
	}
	return seen, nil
}

func findAssetID(row resultRow, index map[string]uint) *uint {
	for _, raw := range []string{row.MatchedAt, row.Host} {
		for _, key := range resultKeys(raw) {
			if id, ok := index[key]; ok {
				idCopy := id
				return &idCopy
			}
		}
	}
	return nil
}

func resultKeys(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	keys := []string{strings.ToLower(raw)}
	if parsed, err := url.Parse(raw); err == nil {
		if parsed.Hostname() != "" {
			keys = append(keys, strings.ToLower(parsed.Hostname()))
		}
		if parsed.String() != "" {
			keys = append(keys, strings.ToLower(parsed.String()))
		}
	}
	return keys
}

func upsertNucleiFinding(targetID uint, fingerprint string, row resultRow, assetID *uint, profile string, now time.Time) error {
	title := strings.TrimSpace(row.Info.Name)
	if title == "" {
		title = strings.TrimSpace(row.TemplateID)
	}
	if title == "" {
		title = "Nuclei finding"
	}
	if !strings.HasPrefix(strings.ToLower(title), "nuclei") {
		title = "Nuclei: " + title
	}

	description := strings.TrimSpace(row.Info.Description)
	if description == "" {
		description = fmt.Sprintf("Nuclei template %s matched this target.", row.TemplateID)
	}

	recommendation := strings.TrimSpace(row.Info.Remediation)
	if recommendation == "" {
		recommendation = "Review the Nuclei finding manually, confirm exploitability, and remediate according to the affected technology and template guidance."
	}

	evidence := evidenceJSON(row, profile)
	severity := normalizeSeverity(row.Info.Severity)

	var existing models.Finding
	err := database.DB.Where("target_id = ? AND fingerprint = ?", targetID, fingerprint).First(&existing).Error
	updates := map[string]interface{}{
		"title":          title,
		"description":    description,
		"severity":       severity,
		"category":       "nuclei",
		"source_tool":    SourceToolNuclei,
		"evidence":       evidence,
		"evidence_json":  datatypes.JSON([]byte(evidence)),
		"recommendation": recommendation,
		"last_seen":      now,
		"asset_id":       assetID,
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
		TargetID:    targetID,
		AssetID:     assetID,
		Title:       title,
		Description: description,
		Severity:    severity,
		Category:    "nuclei",
		SourceTool:  SourceToolNuclei,
		Evidence:    evidence, EvidenceJSON: datatypes.JSON([]byte(evidence)),
		Recommendation: recommendation,
		Status:         models.FindingStatusOpen,
		Fingerprint:    fingerprint,
		FirstSeen:      now,
		LastSeen:       now,
	}
	return database.DB.Create(&finding).Error
}

func evidenceJSON(row resultRow, profile string) string {
	payload := map[string]interface{}{
		"profile":       profile,
		"template_id":   row.TemplateID,
		"template_path": row.TemplatePath,
		"matcher_name":  row.MatcherName,
		"type":          row.Type,
		"host":          row.Host,
		"matched_at":    row.MatchedAt,
		"ip":            row.IP,
		"severity":      row.Info.Severity,
		"tags":          tagsFromRaw(row.Info.Tags),
	}
	if len(row.ExtractedResults) > 0 {
		limit := len(row.ExtractedResults)
		if limit > 10 {
			limit = 10
		}
		payload["extracted_results"] = row.ExtractedResults[:limit]
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return fmt.Sprintf("template_id=%s matched_at=%s host=%s", row.TemplateID, row.MatchedAt, row.Host)
	}
	return string(b)
}

func tagsFromRaw(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return arr
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		parts := strings.Split(one, ",")
		out := make([]string, 0, len(parts))
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part != "" {
				out = append(out, part)
			}
		}
		return out
	}
	return nil
}

func fingerprintForResult(targetID uint, row resultRow) string {
	templateID := strings.TrimSpace(row.TemplateID)
	if templateID == "" {
		return ""
	}
	subject := strings.TrimSpace(row.MatchedAt)
	if subject == "" {
		subject = strings.TrimSpace(row.Host)
	}
	if subject == "" {
		subject = templateID
	}
	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf("nuclei:v1:%d:%s:%s", targetID, templateID, subject)))
	return hex.EncodeToString(h.Sum(nil))
}

func normalizeSeverity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case models.FindingSeverityCritical:
		return models.FindingSeverityCritical
	case models.FindingSeverityHigh:
		return models.FindingSeverityHigh
	case models.FindingSeverityMedium:
		return models.FindingSeverityMedium
	case models.FindingSeverityLow:
		return models.FindingSeverityLow
	default:
		return models.FindingSeverityInfo
	}
}

func markMissingNucleiFindingsFixed(targetID uint, seen map[string]struct{}) error {
	var existing []models.Finding
	if err := database.DB.
		Where("target_id = ? AND source_tool = ? AND category = ? AND status <> ?", targetID, SourceToolNuclei, "nuclei", models.FindingStatusFixed).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing nuclei findings: %w", err)
	}
	now := time.Now()
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
			return fmt.Errorf("mark stale nuclei finding fixed: %w", err)
		}
	}
	return nil
}
