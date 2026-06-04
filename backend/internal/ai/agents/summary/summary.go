package summary

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	ModelName         = "deterministic-summary-v1"
	Methodology       = "summary-agent-v1-policy-aware-advisory"
	MaxAssetsLoaded   = 200
	MaxFindingsLoaded = 200
	MaxURLsLoaded     = 200
)

type Input struct {
	TargetID        uint
	CreatedByUserID *uint
}

type localPolicy struct {
	PlatformName            string         `json:"platform_name"`
	ProgramURL              string         `json:"program_url"`
	InScopePatterns         datatypes.JSON `json:"in_scope_patterns"`
	OutOfScopePatterns      datatypes.JSON `json:"out_of_scope_patterns"`
	AllowedTestTypes        datatypes.JSON `json:"allowed_test_types"`
	DisallowedTestTypes     datatypes.JSON `json:"disallowed_test_types"`
	MaxTestIntensity        string         `json:"max_test_intensity"`
	AuthRequired            bool           `json:"auth_required"`
	SafeTestingNotes        string         `json:"safe_testing_notes"`
	ReportingPreferences    string         `json:"reporting_preferences"`
	BusinessContext         string         `json:"business_context"`
	AssetCriticalityDefault string         `json:"asset_criticality_default"`
}

type localAsset struct {
	ID           uint           `json:"id"`
	Value        string         `json:"value"`
	IsLive       bool           `json:"is_live"`
	StatusCode   int            `json:"status_code"`
	Title        string         `json:"title"`
	WebServer    string         `json:"web_server"`
	CDNName      string         `gorm:"column:cdn_name" json:"cdn_name"`
	Wafcheck     bool           `json:"wafcheck"`
	WafcheckName string         `json:"wafcheck_name"`
	Technologies datatypes.JSON `json:"technologies"`
	OpenPorts    datatypes.JSON `json:"open_ports"`
	Sources      datatypes.JSON `json:"sources"`
	CreatedAt    time.Time      `json:"created_at"`
}

type localFinding struct {
	ID           uint           `json:"id"`
	Title        string         `json:"title"`
	Severity     string         `json:"severity"`
	Category     string         `json:"category"`
	SourceTool   string         `gorm:"column:source_tool" json:"source_tool"`
	Evidence     string         `json:"evidence"`
	EvidenceJSON datatypes.JSON `gorm:"column:evidence_json" json:"evidence_json"`
	Status       string         `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
}

type localURL struct {
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}

type interestingAsset struct {
	Value         string   `json:"value"`
	Reason        []string `json:"reason"`
	InterestScore int      `json:"interest_score"`
	StatusCode    int      `json:"status_code"`
	IsLive        bool     `json:"is_live"`
	Technologies  []string `json:"technologies"`
	OpenPorts     []string `json:"open_ports"`
}

type coverageSummary struct {
	AssetsSampled     int      `json:"assets_sampled"`
	LiveAssetsSampled int      `json:"live_assets_sampled"`
	FindingsSampled   int      `json:"findings_sampled"`
	URLsSampled       int      `json:"urls_sampled"`
	TotalAssets       int64    `json:"total_assets"`
	TotalLiveAssets   int64    `json:"total_live_assets"`
	TotalFindings     int64    `json:"total_findings"`
	TotalURLs         int64    `json:"total_urls"`
	SourceBuckets     []string `json:"source_buckets"`
	TechnologyBuckets []string `json:"technology_buckets"`
	SeverityBuckets   []string `json:"severity_buckets"`
	CoverageGaps      []string `json:"coverage_gaps"`
}

type output struct {
	AgentType             string             `json:"agent_type"`
	Methodology           string             `json:"methodology"`
	Model                 string             `json:"model"`
	GeneratedAt           string             `json:"generated_at"`
	PolicyStatus          string             `json:"policy_status"`
	Guardrails            []string           `json:"guardrails"`
	AttackSurfaceSummary  string             `json:"attack_surface_summary"`
	MostInterestingAssets []interestingAsset `json:"most_interesting_assets"`
	CoverageSummary       coverageSummary    `json:"coverage_summary"`
	RiskNarrative         string             `json:"risk_narrative"`
	WhatToTestNext        []string           `json:"what_to_test_next"`
	PolicySafetyNotes     []string           `json:"policy_safety_notes"`
	Assumptions           []string           `json:"assumptions"`
	Limitations           []string           `json:"limitations"`
}

func Run(input Input) (*models.AgentRun, error) {
	if input.TargetID == 0 {
		return nil, fmt.Errorf("target id is required")
	}

	started := time.Now().UTC()

	var target models.Target
	if err := database.DB.First(&target, input.TargetID).Error; err != nil {
		return nil, fmt.Errorf("load target: %w", err)
	}

	policy, hasPolicy, err := loadPolicy(input.TargetID)
	if err != nil {
		return nil, err
	}

	assets, err := loadAssets(input.TargetID)
	if err != nil {
		return nil, err
	}

	findings, err := loadFindings(input.TargetID)
	if err != nil {
		return nil, err
	}

	urls, err := loadURLs(input.TargetID)
	if err != nil {
		return nil, err
	}

	totalAssets, totalLiveAssets, totalFindings, totalURLs := countTotals(input.TargetID)
	policyStatus := policyStatusFor(hasPolicy, policy)

	inputSnapshot := map[string]interface{}{
		"target_id":         target.ID,
		"root_domain":       target.RootDomain,
		"assets_sampled":    len(assets),
		"findings_sampled":  len(findings),
		"urls_sampled":      len(urls),
		"total_assets":      totalAssets,
		"total_live_assets": totalLiveAssets,
		"total_findings":    totalFindings,
		"total_urls":        totalURLs,
		"has_policy":        hasPolicy,
		"policy_status":     policyStatus,
		"policy":            policy,
		"methodology":       Methodology,
	}

	inputJSON, err := marshalJSON(inputSnapshot)
	if err != nil {
		return nil, err
	}

	out := buildOutput(target, policy, hasPolicy, policyStatus, assets, findings, urls, totalAssets, totalLiveAssets, totalFindings, totalURLs)

	outputJSON, err := marshalJSON(out)
	if err != nil {
		return nil, err
	}

	completed := time.Now().UTC()

	row := models.AgentRun{
		TargetID:        target.ID,
		CreatedByUserID: input.CreatedByUserID,
		AgentType:       models.AgentRunTypeSummary,
		Provider:        "local",
		Model:           ModelName,
		Status:          models.AgentRunStatusCompleted,
		Source:          models.AgentRunSourceLocal,
		PolicyStatus:    policyStatus,
		InputDigest:     digestJSON(inputJSON),
		InputJSON:       inputJSON,
		OutputJSON:      outputJSON,
		StartedAt:       &started,
		CompletedAt:     &completed,
	}

	if err := database.DB.Create(&row).Error; err != nil {
		return nil, fmt.Errorf("create agent run: %w", err)
	}

	return &row, nil
}

func loadPolicy(targetID uint) (localPolicy, bool, error) {
	var row localPolicy
	err := database.DB.
		Table("target_policies").
		Where("target_id = ?", targetID).
		First(&row).Error

	if err == nil {
		return row, true, nil
	}

	if err == gorm.ErrRecordNotFound {
		return row, false, nil
	}

	return row, false, fmt.Errorf("load target policy: %w", err)
}

func loadAssets(targetID uint) ([]localAsset, error) {
	rows := make([]localAsset, 0)

	err := database.DB.
		Model(&models.Asset{}).
		Select("id, value, is_live, status_code, title, web_server, cdn_name, wafcheck, wafcheck_name, technologies, open_ports, sources, created_at").
		Where("target_id = ?", targetID).
		Order("is_live desc, status_code desc, id desc").
		Limit(MaxAssetsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load assets: %w", err)
	}

	return rows, nil
}

func loadFindings(targetID uint) ([]localFinding, error) {
	rows := make([]localFinding, 0)

	err := database.DB.
		Table("findings").
		Select("id, title, severity, category, source_tool, evidence, evidence_json, status, created_at").
		Where("target_id = ? AND deleted_at IS NULL", targetID).
		Order("created_at desc, id desc").
		Limit(MaxFindingsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load findings: %w", err)
	}

	return rows, nil
}

func loadURLs(targetID uint) ([]localURL, error) {
	rows := make([]localURL, 0)

	err := database.DB.
		Table("found_urls").
		Select("value, source, created_at").
		Where("target_id = ?", targetID).
		Order("created_at desc").
		Limit(MaxURLsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load urls: %w", err)
	}

	return rows, nil
}

func countTotals(targetID uint) (int64, int64, int64, int64) {
	var totalAssets int64
	var totalLiveAssets int64
	var totalFindings int64
	var totalURLs int64

	_ = database.DB.Model(&models.Asset{}).Where("target_id = ?", targetID).Count(&totalAssets).Error
	_ = database.DB.Model(&models.Asset{}).Where("target_id = ? AND is_live = ?", targetID, true).Count(&totalLiveAssets).Error
	_ = database.DB.Table("findings").Where("target_id = ? AND deleted_at IS NULL", targetID).Count(&totalFindings).Error
	_ = database.DB.Table("found_urls").Where("target_id = ?", targetID).Count(&totalURLs).Error

	return totalAssets, totalLiveAssets, totalFindings, totalURLs
}

func buildOutput(target models.Target, policy localPolicy, hasPolicy bool, policyStatus string, assets []localAsset, findings []localFinding, urls []localURL, totalAssets, totalLiveAssets, totalFindings, totalURLs int64) output {
	interesting := mostInterestingAssets(assets)
	coverage := buildCoverageSummary(assets, findings, urls, totalAssets, totalLiveAssets, totalFindings, totalURLs)
	policyNotes := buildPolicyNotes(policy, hasPolicy)

	attackSurface := fmt.Sprintf(
		"%s currently has %d discovered assets, %d live assets, %d stored findings, and %d discovered URLs. The summary agent reviewed sampled assets, findings, and URLs to describe the attack surface without changing deterministic risk or severity.",
		target.RootDomain,
		totalAssets,
		totalLiveAssets,
		totalFindings,
		totalURLs,
	)

	riskNarrative := riskNarrativeFor(findings, totalFindings)

	return output{
		AgentType:    models.AgentRunTypeSummary,
		Methodology:  Methodology,
		Model:        ModelName,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		PolicyStatus: policyStatus,
		Guardrails: []string{
			"Agent output is advisory only.",
			"Deterministic findings, severity, risk score, and policy enforcement remain authoritative.",
			"Coverage gaps are treated as scan-improvement tasks, not vulnerabilities.",
			"No active exploitation or out-of-scope testing is performed by this agent.",
		},
		AttackSurfaceSummary:  attackSurface,
		MostInterestingAssets: interesting,
		CoverageSummary:       coverage,
		RiskNarrative:         riskNarrative,
		WhatToTestNext:        whatToTestNext(interesting, findings, hasPolicy),
		PolicySafetyNotes:     policyNotes,
		Assumptions: []string{
			"The summary is based only on data already stored in Hunt Engine.",
			"Sampled assets/findings/URLs may not include every record on very large targets.",
			"Live/dead status depends on the latest completed scan data.",
		},
		Limitations: []string{
			"This agent does not confirm vulnerabilities.",
			"This agent does not submit reports or execute tests.",
			"Manual validation is required before bug bounty reporting or active testing.",
		},
	}
}

func mostInterestingAssets(assets []localAsset) []interestingAsset {
	out := make([]interestingAsset, 0)

	for _, asset := range assets {
		score := 0
		reasons := []string{}

		if asset.IsLive {
			score += 25
			reasons = append(reasons, "live asset")
		}

		if asset.StatusCode >= 200 && asset.StatusCode < 400 {
			score += 15
			reasons = append(reasons, fmt.Sprintf("HTTP status %d", asset.StatusCode))
		} else if asset.StatusCode >= 400 && asset.StatusCode < 500 {
			score += 8
			reasons = append(reasons, fmt.Sprintf("client-visible status %d", asset.StatusCode))
		} else if asset.StatusCode >= 500 {
			score += 12
			reasons = append(reasons, fmt.Sprintf("server error status %d", asset.StatusCode))
		}

		if strings.TrimSpace(asset.Title) != "" {
			score += 8
			reasons = append(reasons, "has page title")
		}

		ports := jsonStringArray(asset.OpenPorts)
		if len(ports) > 0 {
			score += 15 + minInt(len(ports)*2, 10)
			reasons = append(reasons, fmt.Sprintf("%d open port entries", len(ports)))
		}

		techs := jsonStringArray(asset.Technologies)
		if len(techs) > 0 {
			score += 10 + minInt(len(techs), 10)
			reasons = append(reasons, "technology fingerprint available")
		}

		if strings.TrimSpace(asset.CDNName) != "" || asset.Wafcheck {
			score += 4
			reasons = append(reasons, "edge/CDN/WAF signal available")
		}

		valueLower := strings.ToLower(asset.Value)
		for _, marker := range []string{"admin", "login", "api", "dev", "stage", "staging", "test", "internal", "sso", "auth"} {
			if strings.Contains(valueLower, marker) {
				score += 8
				reasons = append(reasons, "interesting hostname keyword: "+marker)
				break
			}
		}

		if score <= 0 {
			continue
		}

		out = append(out, interestingAsset{
			Value:         asset.Value,
			Reason:        uniqueStrings(reasons),
			InterestScore: score,
			StatusCode:    asset.StatusCode,
			IsLive:        asset.IsLive,
			Technologies:  techs,
			OpenPorts:     ports,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].InterestScore == out[j].InterestScore {
			return out[i].Value < out[j].Value
		}
		return out[i].InterestScore > out[j].InterestScore
	})

	if len(out) > 10 {
		return out[:10]
	}

	return out
}

func buildCoverageSummary(assets []localAsset, findings []localFinding, urls []localURL, totalAssets, totalLiveAssets, totalFindings, totalURLs int64) coverageSummary {
	sourceSet := map[string]bool{}
	techSet := map[string]bool{}
	severityCount := map[string]int{}

	for _, asset := range assets {
		for _, source := range jsonStringArray(asset.Sources) {
			sourceSet[source] = true
		}
		for _, tech := range jsonStringArray(asset.Technologies) {
			techSet[tech] = true
		}
	}

	for _, finding := range findings {
		sev := strings.ToLower(strings.TrimSpace(finding.Severity))
		if sev == "" {
			sev = "unknown"
		}
		severityCount[sev]++
	}

	liveSampled := 0
	for _, asset := range assets {
		if asset.IsLive {
			liveSampled++
		}
	}

	gaps := []string{}
	if totalAssets == 0 {
		gaps = append(gaps, "No assets are stored for this target yet.")
	}
	if totalLiveAssets == 0 {
		gaps = append(gaps, "No live DNS/HTTP assets are currently confirmed.")
	}
	if totalURLs == 0 {
		gaps = append(gaps, "No crawled or archived URLs are stored yet.")
	}
	if totalFindings == 0 {
		gaps = append(gaps, "No findings are stored yet; this is not evidence of no risk.")
	}
	if len(sourceSet) == 0 {
		gaps = append(gaps, "No source diversity is visible in sampled asset data.")
	}

	return coverageSummary{
		AssetsSampled:     len(assets),
		LiveAssetsSampled: liveSampled,
		FindingsSampled:   len(findings),
		URLsSampled:       len(urls),
		TotalAssets:       totalAssets,
		TotalLiveAssets:   totalLiveAssets,
		TotalFindings:     totalFindings,
		TotalURLs:         totalURLs,
		SourceBuckets:     sortedKeys(sourceSet),
		TechnologyBuckets: topN(sortedKeys(techSet), 20),
		SeverityBuckets:   severityBuckets(severityCount),
		CoverageGaps:      gaps,
	}
}

func riskNarrativeFor(findings []localFinding, totalFindings int64) string {
	if totalFindings == 0 {
		return "No stored findings were available for risk narration. This should be treated as lack of evidence, not proof of safety."
	}

	severityCount := map[string]int{}
	categories := map[string]bool{}
	sources := map[string]bool{}

	for _, finding := range findings {
		sev := strings.ToLower(strings.TrimSpace(finding.Severity))
		if sev == "" {
			sev = "unknown"
		}
		severityCount[sev]++

		if c := strings.TrimSpace(finding.Category); c != "" {
			categories[c] = true
		}
		if s := strings.TrimSpace(finding.SourceTool); s != "" {
			sources[s] = true
		}
	}

	criticalHigh := severityCount["critical"] + severityCount["high"]
	if criticalHigh > 0 {
		return fmt.Sprintf("The sampled findings include %d critical/high entries across %d categories and %d tools. Prioritize manual validation of evidence-rich findings before reporting or active testing.", criticalHigh, len(categories), len(sources))
	}

	medium := severityCount["medium"]
	if medium > 0 {
		return fmt.Sprintf("The sampled findings are mostly moderate/low signal, with %d medium entries. Treat these as validation leads and avoid inflating severity without exploitability and impact evidence.", medium)
	}

	return "The sampled findings appear low-signal or informational. Focus on improving coverage and validating evidence-rich leads rather than treating coverage gaps as vulnerabilities."
}

func whatToTestNext(assets []interestingAsset, findings []localFinding, hasPolicy bool) []string {
	out := []string{}

	if !hasPolicy {
		out = append(out, "Define Target Policy before any active or higher-intensity testing.")
	}

	if len(assets) > 0 {
		out = append(out, "Review the most interesting live assets for authentication surfaces, admin panels, API exposure, and unusual ports.")
	}

	if len(findings) > 0 {
		out = append(out, "Validate top findings against evidence_json and current live responses before using them in a report.")
	}

	hasJS := false
	hasTakeover := false
	hasNuclei := false
	for _, f := range findings {
		tool := strings.ToLower(f.SourceTool)
		cat := strings.ToLower(f.Category)
		if strings.Contains(tool, "js") || strings.Contains(cat, "js") {
			hasJS = true
		}
		if strings.Contains(tool, "takeover") || strings.Contains(cat, "takeover") {
			hasTakeover = true
		}
		if strings.Contains(tool, "nuclei") {
			hasNuclei = true
		}
	}

	if hasJS {
		out = append(out, "Review JavaScript-derived endpoints for sensitive paths, source maps, and candidate client-side sinks.")
	}
	if hasTakeover {
		out = append(out, "Manually validate takeover candidates with provider-specific DNS/CNAME evidence.")
	}
	if hasNuclei {
		out = append(out, "Re-check Nuclei findings manually and map them to affected assets, impact, and reproduction evidence.")
	}

	if len(out) == 0 {
		out = append(out, "Run or complete Discovery, Probing, and Crawling to improve attack surface coverage.")
	}

	return uniqueStrings(out)
}

func buildPolicyNotes(policy localPolicy, hasPolicy bool) []string {
	if !hasPolicy {
		return []string{
			"No target policy exists yet. Keep recommendations passive/manual until scope rules are defined.",
			"Do not run active tests without explicit scope and intensity constraints.",
		}
	}

	out := []string{"Target policy exists and was considered by the summary agent."}
	if strings.TrimSpace(policy.MaxTestIntensity) != "" {
		out = append(out, "Max test intensity: "+policy.MaxTestIntensity)
	}
	if strings.TrimSpace(policy.SafeTestingNotes) != "" {
		out = append(out, "Safe testing notes: "+policy.SafeTestingNotes)
	}
	if strings.TrimSpace(policy.ReportingPreferences) != "" {
		out = append(out, "Reporting preferences: "+policy.ReportingPreferences)
	}
	if policy.AuthRequired {
		out = append(out, "Policy indicates authentication is required for some testing.")
	}
	return out
}

func policyStatusFor(hasPolicy bool, policy localPolicy) string {
	if !hasPolicy {
		return models.AgentRunPolicyStatusUnknown
	}

	if len(policy.OutOfScopePatterns) > 2 || strings.TrimSpace(policy.MaxTestIntensity) != "" {
		return models.AgentRunPolicyStatusAllowed
	}

	return models.AgentRunPolicyStatusWarning
}

func jsonStringArray(raw datatypes.JSON) []string {
	if len(raw) == 0 {
		return nil
	}

	var arr []string
	if err := json.Unmarshal(raw, &arr); err == nil {
		return cleanStrings(arr)
	}

	var generic []interface{}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}

	out := make([]string, 0, len(generic))
	for _, item := range generic {
		switch v := item.(type) {
		case string:
			out = append(out, v)
		case map[string]interface{}:
			if value, ok := v["value"].(string); ok {
				out = append(out, value)
			} else if value, ok := v["name"].(string); ok {
				out = append(out, value)
			} else if value, ok := v["port"].(string); ok {
				out = append(out, value)
			} else if port, ok := v["port"].(float64); ok {
				out = append(out, fmt.Sprintf("%.0f", port))
			}
		}
	}

	return cleanStrings(out)
}

func cleanStrings(values []string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]bool{}

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}

	sort.Strings(out)
	return out
}

func uniqueStrings(values []string) []string {
	return cleanStrings(values)
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for key := range m {
		key = strings.TrimSpace(key)
		if key != "" {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

func severityBuckets(counts map[string]int) []string {
	order := []string{"critical", "high", "medium", "low", "info", "informational", "unknown"}
	out := []string{}

	seen := map[string]bool{}
	for _, key := range order {
		if n := counts[key]; n > 0 {
			out = append(out, fmt.Sprintf("%s:%d", key, n))
			seen[key] = true
		}
	}

	extra := []string{}
	for key, n := range counts {
		if seen[key] || n <= 0 {
			continue
		}
		extra = append(extra, fmt.Sprintf("%s:%d", key, n))
	}
	sort.Strings(extra)
	out = append(out, extra...)

	return out
}

func topN(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func marshalJSON(v interface{}) (datatypes.JSON, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return datatypes.JSON(b), nil
}

func digestJSON(raw datatypes.JSON) string {
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
