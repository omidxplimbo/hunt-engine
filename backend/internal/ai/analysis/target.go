package analysis

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ProviderLocalDeterministic = "local"
	ModelTargetDeterministicV1 = "deterministic-target-v1"
	PromptTargetV1             = "target-analysis-v1"
)

type TargetAnalysisInput struct {
	TargetID        uint
	CreatedByUserID *uint
}

type countRow struct {
	Key   string `gorm:"column:key"`
	Count int64  `gorm:"column:count"`
}

type topFinding struct {
	ID         uint      `json:"id"`
	Severity   string    `json:"severity"`
	Status     string    `json:"status"`
	Title      string    `json:"title"`
	Category   string    `json:"category"`
	SourceTool string    `json:"source_tool"`
	LastSeen   time.Time `json:"last_seen"`
}

func GenerateTargetAnalysis(input TargetAnalysisInput) (*models.AIAnalysis, error) {
	if input.TargetID == 0 {
		return nil, fmt.Errorf("target_id is required")
	}

	var target models.Target
	if err := database.DB.First(&target, input.TargetID).Error; err != nil {
		return nil, fmt.Errorf("load target: %w", err)
	}

	started := time.Now().UTC()

	snapshot, err := buildTargetSnapshot(target)
	if err != nil {
		return nil, err
	}

	snapshotJSON, _ := json.Marshal(snapshot)
	sum := sha1.Sum(snapshotJSON)
	inputDigest := hex.EncodeToString(sum[:])

	output := buildTargetAnalysisOutput(snapshot)
	outputJSON, _ := json.Marshal(output)

	title := fmt.Sprintf("Target analysis: %s", target.RootDomain)
	summary, _ := output["summary"].(string)

	row := models.AIAnalysis{
		TargetID:        target.ID,
		CreatedByUserID: input.CreatedByUserID,
		Scope:           models.AIAnalysisScopeTarget,
		Source:          "system",
		Provider:        ProviderLocalDeterministic,
		Model:           ModelTargetDeterministicV1,
		PromptVersion:   PromptTargetV1,
		Status:          models.AIAnalysisStatusCompleted,
		Title:           title,
		Summary:         summary,
		InputDigest:     inputDigest,
		InputJSON:       datatypes.JSON(snapshotJSON),
		OutputJSON:      datatypes.JSON(outputJSON),
		StartedAt:       &started,
		CompletedAt:     ptrTime(time.Now().UTC()),
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create ai analysis: %w", err)
	}

	return &row, nil
}

func buildTargetSnapshot(target models.Target) (map[string]interface{}, error) {
	assetCounts, err := collectAssetCounts(target.ID)
	if err != nil {
		return nil, err
	}

	urlCounts, err := collectURLCounts(target.ID)
	if err != nil {
		return nil, err
	}

	findingCounts, err := collectFindingCounts(target.ID)
	if err != nil {
		return nil, err
	}

	scanState, err := collectScanState(target.ID)
	if err != nil {
		return nil, err
	}

	topFindings, err := collectTopFindings(target.ID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"target": map[string]interface{}{
			"id":             target.ID,
			"name":           target.Name,
			"root_domain":    target.RootDomain,
			"description":    target.Description,
			"status":         target.Status,
			"current_phase":  target.CurrentPhase,
			"scan_count":     target.ScanCount,
			"last_scan_at":   target.LastScanAt,
			"scan_modules":   parseStringList(target.ScanModules),
			"use_nuclei":     target.UseNuclei,
			"nuclei_profile": target.NucleiProfile,
			"use_portscan":   target.UsePortscan,
			"use_waymore":    target.UseWaymore,
			"use_amass":      target.UseAmass,
			"use_puredns":    target.UsePuredns,
		},
		"assets":       assetCounts,
		"urls":         urlCounts,
		"findings":     findingCounts,
		"scan_state":   scanState,
		"top_findings": topFindings,
		"generated_at": time.Now().UTC(),
	}, nil
}

func collectAssetCounts(targetID uint) (map[string]int64, error) {
	base := database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID)

	out := map[string]int64{}
	queries := map[string]*gorm.DB{
		"total":      base.Session(&gorm.Session{}),
		"live":       base.Session(&gorm.Session{}).Where("is_live = ?", true),
		"dead":       base.Session(&gorm.Session{}).Where("is_live = ?", false),
		"web":        base.Session(&gorm.Session{}).Where("status_code > 0 OR final_url <> ''"),
		"dns_only":   base.Session(&gorm.Session{}).Where("is_live = ? AND status_code = 0 AND final_url = ''", true),
		"with_cdn":   base.Session(&gorm.Session{}).Where("cdn_name <> '' OR cdncheck = ?", true),
		"with_waf":   base.Session(&gorm.Session{}).Where("wafcheck = ?", true),
		"with_cloud": base.Session(&gorm.Session{}).Where("cloudcheck = ?", true),
		"with_ports": base.Session(&gorm.Session{}).Where("open_ports IS NOT NULL AND open_ports::text <> '{}' AND open_ports::text <> 'null' AND open_ports::text <> ''"),
	}

	for key, query := range queries {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return nil, fmt.Errorf("count assets %s: %w", key, err)
		}
		out[key] = count
	}

	return out, nil
}

func collectURLCounts(targetID uint) (map[string]int64, error) {
	base := database.DB.Model(&models.FoundURL{}).Where("target_id = ?", targetID)

	out := map[string]int64{}
	queries := map[string]*gorm.DB{
		"total": base.Session(&gorm.Session{}),
		"js":    base.Session(&gorm.Session{}).Where("lower(value) ~ ?", `\.(js|mjs)(\?|$)`),
		"maps":  base.Session(&gorm.Session{}).Where("lower(value) ~ ?", `\.map(\?|$)`),
	}

	for key, query := range queries {
		var count int64
		if err := query.Count(&count).Error; err != nil {
			return nil, fmt.Errorf("count urls %s: %w", key, err)
		}
		out[key] = count
	}

	bySource, err := countGrouped(base.Session(&gorm.Session{}), "source")
	if err != nil {
		return nil, fmt.Errorf("count urls by source: %w", err)
	}

	for key, count := range bySource {
		out["source_"+safeKey(key)] = count
	}

	return out, nil
}

func collectFindingCounts(targetID uint) (map[string]interface{}, error) {
	base := database.DB.Model(&models.Finding{}).Where("target_id = ?", targetID)
	active := base.Session(&gorm.Session{}).Where("status NOT IN ?", []string{
		models.FindingStatusFixed,
		models.FindingStatusFalsePositive,
	})

	total := int64(0)
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count findings total: %w", err)
	}

	activeTotal := int64(0)
	if err := active.Session(&gorm.Session{}).Count(&activeTotal).Error; err != nil {
		return nil, fmt.Errorf("count active findings total: %w", err)
	}

	bySeverity, err := countGrouped(base.Session(&gorm.Session{}), "severity")
	if err != nil {
		return nil, err
	}

	activeBySeverity, err := countGrouped(active.Session(&gorm.Session{}), "severity")
	if err != nil {
		return nil, err
	}

	byStatus, err := countGrouped(base.Session(&gorm.Session{}), "status")
	if err != nil {
		return nil, err
	}

	bySource, err := countGrouped(base.Session(&gorm.Session{}), "source_tool")
	if err != nil {
		return nil, err
	}

	byCategory, err := countGrouped(base.Session(&gorm.Session{}), "category")
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"total":              total,
		"active_total":       activeTotal,
		"by_severity":        bySeverity,
		"active_by_severity": activeBySeverity,
		"by_status":          byStatus,
		"by_source":          bySource,
		"by_category":        byCategory,
	}, nil
}

func collectScanState(targetID uint) (map[string]interface{}, error) {
	var state models.TargetScanState
	err := database.DB.Where("target_id = ?", targetID).First(&state).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return map[string]interface{}{
				"exists": false,
			}, nil
		}
		return nil, fmt.Errorf("load scan state: %w", err)
	}

	return map[string]interface{}{
		"exists":          true,
		"status":          state.Status,
		"current_module":  state.CurrentModule,
		"current_step":    state.CurrentStep,
		"completed_steps": parseStringList(state.CompletedSteps),
		"meta":            parseJSONObject(state.Meta),
		"last_error":      state.LastError,
		"heartbeat_at":    state.HeartbeatAt,
		"updated_at":      state.UpdatedAt,
	}, nil
}

func collectTopFindings(targetID uint) ([]topFinding, error) {
	var rows []models.Finding
	if err := database.DB.
		Where("target_id = ? AND status NOT IN ?", targetID, []string{
			models.FindingStatusFixed,
			models.FindingStatusFalsePositive,
		}).
		Order(`CASE severity
			WHEN 'critical' THEN 5
			WHEN 'high' THEN 4
			WHEN 'medium' THEN 3
			WHEN 'low' THEN 2
			WHEN 'info' THEN 1
			ELSE 0
		END DESC, last_seen DESC, id DESC`).
		Limit(10).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("load top findings: %w", err)
	}

	out := make([]topFinding, 0, len(rows))
	for _, f := range rows {
		out = append(out, topFinding{
			ID:         f.ID,
			Severity:   f.Severity,
			Status:     f.Status,
			Title:      f.Title,
			Category:   f.Category,
			SourceTool: f.SourceTool,
			LastSeen:   f.LastSeen,
		})
	}

	return out, nil
}

func buildTargetAnalysisOutput(snapshot map[string]interface{}) map[string]interface{} {
	target := mapValue(snapshot["target"])
	assets := int64Map(snapshot["assets"])
	urls := int64Map(snapshot["urls"])
	findings := mapValue(snapshot["findings"])
	activeBySeverity := int64Map(findings["active_by_severity"])
	bySource := int64Map(findings["by_source"])
	byCategory := int64Map(findings["by_category"])

	riskScore := calculateRiskScore(activeBySeverity)
	riskLevel := riskLevelForScore(riskScore)

	exposureBuckets := buildExposureBuckets(activeBySeverity, bySource, byCategory, assets, urls)
	coverageGaps := buildCoverageGaps(assets, urls, bySource)
	nextActions := buildNextActions(riskScore, activeBySeverity, bySource, byCategory, assets, urls)

	rootDomain := fmt.Sprint(target["root_domain"])
	summary := fmt.Sprintf(
		"Deterministic analysis for %s: %d assets (%d live), %d URLs, and %d active findings. Current risk is %s (%d/100).",
		rootDomain,
		assets["total"],
		assets["live"],
		urls["total"],
		toInt64(findings["active_total"]),
		riskLevel,
		riskScore,
	)

	return map[string]interface{}{
		"summary":          summary,
		"risk_score":       riskScore,
		"risk_level":       riskLevel,
		"exposure_buckets": exposureBuckets,
		"coverage_gaps":    coverageGaps,
		"next_actions":     nextActions,
		"counts": map[string]interface{}{
			"assets":   assets,
			"urls":     urls,
			"findings": findings,
		},
		"top_findings": snapshot["top_findings"],
	}
}

func calculateRiskScore(activeBySeverity map[string]int64) int {
	score := 0
	score += int(activeBySeverity[models.FindingSeverityCritical]) * 35
	score += int(activeBySeverity[models.FindingSeverityHigh]) * 20
	score += int(activeBySeverity[models.FindingSeverityMedium]) * 8
	score += int(activeBySeverity[models.FindingSeverityLow]) * 3
	score += int(activeBySeverity[models.FindingSeverityInfo]) * 1

	if score > 100 {
		return 100
	}
	return score
}

func riskLevelForScore(score int) string {
	switch {
	case score >= 85:
		return "critical"
	case score >= 60:
		return "high"
	case score >= 30:
		return "medium"
	case score > 0:
		return "low"
	default:
		return "informational"
	}
}

func buildExposureBuckets(activeBySeverity, bySource, byCategory, assets, urls map[string]int64) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)

	add := func(name, severity, description string, count int64) {
		if count <= 0 {
			return
		}
		out = append(out, map[string]interface{}{
			"name":        name,
			"severity":    severity,
			"count":       count,
			"description": description,
		})
	}

	add("Critical/High findings", "high", "Active findings with critical or high severity need manual validation first.", activeBySeverity["critical"]+activeBySeverity["high"])
	add("Subdomain takeover candidates", "high", "Takeover detector reported assets with third-party CNAME takeover indicators.", byCategory["subdomain-takeover"])
	add("JavaScript secrets", "high", "JavaScript intelligence observed token or secret-like patterns.", byCategory["js-secret"])
	add("JavaScript endpoints", "info", "JavaScript intelligence exposed interesting client-side endpoints.", byCategory["js-endpoints"])
	add("Nuclei findings", "medium", "Nuclei generated normalized security findings.", bySource["nuclei"])
	add("Live assets", "info", "Live assets observed during DNS/probing phases.", assets["live"])
	add("Crawled URLs", "info", "URLs collected from passive and active crawling sources.", urls["total"])

	return out
}

func buildCoverageGaps(assets, urls, bySource map[string]int64) []string {
	gaps := make([]string, 0)

	if urls["total"] == 0 {
		gaps = append(gaps, "No crawled URLs are stored for this target; crawling coverage may be incomplete.")
	}
	if urls["js"] == 0 {
		gaps = append(gaps, "No JavaScript URLs were collected; JS intelligence coverage may be limited.")
	}
	if bySource["nuclei"] == 0 {
		gaps = append(gaps, "No Nuclei findings are present; verify whether Nuclei was enabled and completed.")
	}
	if assets["with_ports"] == 0 {
		gaps = append(gaps, "No open port data is present; port-scan coverage may be disabled or empty.")
	}
	if assets["live"] == 0 && assets["total"] > 0 {
		gaps = append(gaps, "Assets exist but none are currently marked live.")
	}
	if len(gaps) == 0 {
		gaps = append(gaps, "No major coverage gaps detected from stored data.")
	}

	return gaps
}

func buildNextActions(riskScore int, activeBySeverity, bySource, byCategory, assets, urls map[string]int64) []string {
	actions := make([]string, 0)

	if activeBySeverity["critical"]+activeBySeverity["high"] > 0 {
		actions = append(actions, "Manually validate all active critical/high findings and prioritize confirmed exploitable issues.")
	}
	if byCategory["subdomain-takeover"] > 0 {
		actions = append(actions, "Review takeover candidates, confirm CNAME ownership status, and remove stale DNS records.")
	}
	if byCategory["js-secret"] > 0 {
		actions = append(actions, "Validate JavaScript secret findings and rotate exposed credentials if confirmed.")
	}
	if byCategory["js-endpoints"] > 0 {
		actions = append(actions, "Review discovered JavaScript endpoints for hidden APIs, debug routes, and authorization gaps.")
	}
	if urls["total"] == 0 {
		actions = append(actions, "Run or re-run crawling modules to improve URL and JavaScript coverage.")
	}
	if assets["live"] > 0 && bySource["nuclei"] == 0 {
		actions = append(actions, "Run Nuclei with an appropriate profile against live assets.")
	}
	if riskScore == 0 {
		actions = append(actions, "Continue scheduled monitoring and compare future scans for new assets or drift.")
	}

	return actions
}

func countGrouped(db *gorm.DB, field string) (map[string]int64, error) {
	rows := make([]countRow, 0)
	if err := db.
		Select(field + " AS key, COUNT(*) AS count").
		Where(field + " <> ''").
		Group(field).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]int64, len(rows))
	for _, row := range rows {
		out[row.Key] = row.Count
	}
	return out, nil
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return []string{}
	}

	var arr []string
	if err := json.Unmarshal([]byte(raw), &arr); err == nil {
		return arr
	}

	return []string{}
}

func parseJSONObject(raw string) map[string]interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return map[string]interface{}{}
	}

	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		return obj
	}

	return map[string]interface{}{}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

func safeKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, ",", "_")
	value = strings.ReplaceAll(value, "-", "_")
	return value
}

func mapValue(v interface{}) map[string]interface{} {
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func int64Map(v interface{}) map[string]int64 {
	if m, ok := v.(map[string]int64); ok {
		return m
	}

	out := map[string]int64{}
	if m, ok := v.(map[string]interface{}); ok {
		for key, value := range m {
			out[key] = toInt64(value)
		}
	}

	return out
}

func toInt64(v interface{}) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case uint:
		return int64(n)
	case uint64:
		return int64(n)
	case float64:
		return int64(n)
	case json.Number:
		i, _ := n.Int64()
		return i
	default:
		return 0
	}
}
