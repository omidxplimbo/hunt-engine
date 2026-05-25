package analysis

import (
	"fmt"
	"math"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
)

const CommercialMethodologyVersion = "target-risk-v2-commercial-guardrails"

type commercialAssessment struct {
	RiskScore           int                      `json:"risk_score"`
	RiskLevel           string                   `json:"risk_level"`
	ConfidenceScore     int                      `json:"confidence_score"`
	ExposureScore       int                      `json:"exposure_score"`
	CoverageScore       int                      `json:"coverage_score"`
	FindingQualityScore int                      `json:"finding_quality_score"`
	MethodologyVersion  string                   `json:"methodology_version"`
	RiskDrivers         []map[string]interface{} `json:"risk_drivers"`
	Assumptions         []string                 `json:"assumptions"`
	Limitations         []string                 `json:"limitations"`
	PrioritizedFindings []map[string]interface{} `json:"prioritized_findings"`
}

func buildCommercialTargetAnalysisOutput(snapshot map[string]interface{}) map[string]interface{} {
	target := mapValue(snapshot["target"])
	assets := int64Map(snapshot["assets"])
	urls := int64Map(snapshot["urls"])
	findings := mapValue(snapshot["findings"])
	activeBySeverity := int64Map(findings["active_by_severity"])
	bySource := int64Map(findings["by_source"])
	byCategory := int64Map(findings["by_category"])

	assessment := assessCommercialRisk(activeBySeverity, bySource, byCategory, assets, urls, snapshot["top_findings"])

	exposureBuckets := buildCommercialExposureBuckets(activeBySeverity, bySource, byCategory, assets, urls)
	coverageGaps := buildCommercialCoverageGaps(assets, urls, bySource)
	nextActions := buildCommercialNextActions(assessment, activeBySeverity, bySource, byCategory, assets, urls)

	rootDomain := fmt.Sprint(target["root_domain"])
	activeTotal := toInt64(findings["active_total"])

	summary := fmt.Sprintf(
		"Commercial-grade deterministic analysis for %s: %d assets (%d live), %d URLs, and %d active findings. Current evidence-based risk is %s (%d/100), with confidence %d/100 and coverage %d/100.",
		rootDomain,
		assets["total"],
		assets["live"],
		urls["total"],
		activeTotal,
		assessment.RiskLevel,
		assessment.RiskScore,
		assessment.ConfidenceScore,
		assessment.CoverageScore,
	)

	return map[string]interface{}{
		"summary":               summary,
		"executive_summary":     summary,
		"methodology_version":   assessment.MethodologyVersion,
		"risk_score":            assessment.RiskScore,
		"risk_level":            assessment.RiskLevel,
		"confidence_score":      assessment.ConfidenceScore,
		"exposure_score":        assessment.ExposureScore,
		"coverage_score":        assessment.CoverageScore,
		"finding_quality_score": assessment.FindingQualityScore,
		"risk": map[string]interface{}{
			"score":                 assessment.RiskScore,
			"level":                 assessment.RiskLevel,
			"confidence_score":      assessment.ConfidenceScore,
			"exposure_score":        assessment.ExposureScore,
			"coverage_score":        assessment.CoverageScore,
			"finding_quality_score": assessment.FindingQualityScore,
			"drivers":               assessment.RiskDrivers,
		},
		"methodology": map[string]interface{}{
			"version": CommercialMethodologyVersion,
			"principles": []string{
				"Coverage gaps are tracked separately and do not directly increase security risk.",
				"Reconnaissance-only signals do not become critical solely because they are numerous.",
				"Critical risk requires strong evidence such as confirmed high/critical findings, takeover indicators, exposed secrets, exposed sensitive services, or validated scanner evidence.",
				"AI-generated text must remain downstream of deterministic evidence and risk guardrails.",
			},
			"risk_inputs": []string{
				"active finding severity",
				"finding source reliability",
				"finding category exploitability",
				"asset exposure",
				"available scan coverage",
				"finding quality and confidence",
			},
		},
		"risk_drivers":         assessment.RiskDrivers,
		"exposure_buckets":     exposureBuckets,
		"coverage_gaps":        coverageGaps,
		"next_actions":         nextActions,
		"assumptions":          assessment.Assumptions,
		"limitations":          assessment.Limitations,
		"prioritized_findings": assessment.PrioritizedFindings,
		"counts": map[string]interface{}{
			"assets":   assets,
			"urls":     urls,
			"findings": findings,
		},
		"top_findings": snapshot["top_findings"],
	}
}

func assessCommercialRisk(activeBySeverity, bySource, byCategory, assets, urls map[string]int64, topFindings interface{}) commercialAssessment {
	drivers := make([]map[string]interface{}, 0)

	addDriver := func(name string, contribution int, severity string, count int64, rationale string) {
		if contribution <= 0 || count <= 0 {
			return
		}
		drivers = append(drivers, map[string]interface{}{
			"name":         name,
			"contribution": contribution,
			"severity":     severity,
			"count":        count,
			"rationale":    rationale,
		})
	}

	critical := activeBySeverity[models.FindingSeverityCritical]
	high := activeBySeverity[models.FindingSeverityHigh]
	medium := activeBySeverity[models.FindingSeverityMedium]
	low := activeBySeverity[models.FindingSeverityLow]

	score := 0

	cCritical := cappedInt(int(critical)*30, 70)
	cHigh := cappedInt(int(high)*16, 60)
	cMedium := cappedInt(int(medium)*4, 28)
	cLow := cappedInt(int(low)*1, 8)

	score += cCritical
	score += cHigh
	score += cMedium
	score += cLow

	addDriver("Critical severity findings", cCritical, "critical", critical, "Critical findings are weighted heavily but capped to avoid uncontrolled score inflation.")
	addDriver("High severity findings", cHigh, "high", high, "High severity findings materially affect risk when active and not marked fixed or false positive.")
	addDriver("Medium severity findings", cMedium, "medium", medium, "Medium findings contribute to risk but cannot independently create critical risk.")
	addDriver("Low severity findings", cLow, "low", low, "Low findings provide context with a small capped contribution.")

	categoryWeights := []struct {
		category string
		name     string
		weight   int
		cap      int
		severity string
		reason   string
	}{
		{"subdomain-takeover", "Subdomain takeover candidates", 18, 45, "high", "Potential takeover requires urgent manual validation because confirmed cases can become account or content compromise."},
		{"js-secret", "JavaScript secret exposure", 16, 40, "high", "Secret-like material in client-side JavaScript can represent credential exposure if confirmed."},
		{"exposed-vcs", "Exposed version control artefacts", 14, 35, "high", "Exposed VCS paths may leak source code, credentials, or internal history."},
		{"exposed-config", "Exposed configuration artefacts", 12, 30, "high", "Exposed configuration files can leak operational or credential material."},
		{"exposed-service", "Exposed sensitive services", 10, 30, "medium", "Sensitive services exposed to the internet increase attack surface and require access-control validation."},
		{"exposed-interface", "Exposed administrative/login interfaces", 3, 12, "low", "Login/admin indicators are triage signals, not vulnerabilities by themselves."},
		{"server-error", "Server error responses", 1, 4, "low", "5xx responses may indicate instability or debug exposure, but need manual confirmation."},
	}

	for _, item := range categoryWeights {
		count := byCategory[item.category]
		contribution := cappedInt(int(count)*item.weight, item.cap)
		score += contribution
		addDriver(item.name, contribution, item.severity, count, item.reason)
	}

	nucleiContribution := cappedInt(int(bySource["nuclei"])*5, 20)
	score += nucleiContribution
	addDriver("Nuclei validated findings", nucleiContribution, "medium", bySource["nuclei"], "Nuclei findings add confidence when backed by structured evidence and template metadata.")

	strongSignal := critical > 0 ||
		high > 0 ||
		byCategory["subdomain-takeover"] > 0 ||
		byCategory["js-secret"] > 0 ||
		byCategory["exposed-vcs"] > 0 ||
		byCategory["exposed-config"] > 0 ||
		byCategory["exposed-service"] > 0

	if !strongSignal && score > 39 {
		score = 39
		addDriver("Guardrail cap applied", 0, "info", 1, "No strong exploitable signal was present, so risk was capped below high severity.")
	}

	if critical == 0 && high == 0 && score > 64 {
		score = 64
		addDriver("No critical/high severity cap applied", 0, "info", 1, "No active critical/high finding exists, so risk was capped below critical severity.")
	}

	score = cappedInt(score, 100)

	confidence := calculateCommercialConfidence(activeBySeverity, bySource, byCategory)
	exposure := calculateCommercialExposure(assets)
	coverage := calculateCommercialCoverage(assets, urls, bySource)
	quality := calculateFindingQuality(activeBySeverity, bySource, byCategory)

	return commercialAssessment{
		RiskScore:           score,
		RiskLevel:           commercialRiskLevel(score),
		ConfidenceScore:     confidence,
		ExposureScore:       exposure,
		CoverageScore:       coverage,
		FindingQualityScore: quality,
		MethodologyVersion:  CommercialMethodologyVersion,
		RiskDrivers:         drivers,
		Assumptions: []string{
			"Risk is calculated from stored Hunt Engine evidence at analysis time.",
			"Findings marked fixed or false positive are excluded from active severity counts.",
			"Business criticality is not yet configured, so this version uses technical exposure and evidence confidence only.",
		},
		Limitations: []string{
			"Coverage gaps indicate scan completeness issues and are not treated as vulnerabilities.",
			"Reconnaissance findings such as JavaScript endpoints require manual validation before remediation work.",
			"Secret-like values, takeover candidates, and scanner findings must be manually validated before customer-facing claims.",
			"External LLM reasoning is not used in this deterministic analysis version.",
		},
		PrioritizedFindings: buildPrioritizedFindingList(topFindings, byCategory),
	}
}

func calculateCommercialConfidence(activeBySeverity, bySource, byCategory map[string]int64) int {
	score := 35

	if bySource["nuclei"] > 0 {
		score += 20
	}
	if bySource["takeover"] > 0 {
		score += 20
	}
	if bySource["js-intel"] > 0 {
		score += 10
	}
	if bySource["builtin"] > 0 {
		score += 5
	}
	if byCategory["js-secret"] > 0 || byCategory["subdomain-takeover"] > 0 {
		score += 10
	}
	if activeBySeverity[models.FindingSeverityCritical]+activeBySeverity[models.FindingSeverityHigh] > 0 {
		score += 10
	}

	return cappedInt(score, 100)
}

func calculateCommercialExposure(assets map[string]int64) int {
	total := assets["total"]
	live := assets["live"]
	web := assets["web"]
	withPorts := assets["with_ports"]

	score := 0
	score += scaledContribution(live, total, 35)
	score += scaledContribution(web, total, 25)
	score += cappedInt(int(math.Log(float64(maxInt64(withPorts, 0))+1)*12), 25)

	if assets["with_cdn"] > 0 {
		score -= 5
	}
	if score < 0 {
		score = 0
	}
	return cappedInt(score, 100)
}

func calculateCommercialCoverage(assets, urls, bySource map[string]int64) int {
	score := 20

	if assets["total"] > 0 {
		score += 20
	}
	if assets["live"] > 0 {
		score += 15
	}
	if urls["total"] > 0 {
		score += 15
	}
	if urls["js"] > 0 {
		score += 10
	}
	if bySource["nuclei"] > 0 {
		score += 15
	}
	if assets["with_ports"] > 0 {
		score += 5
	}

	return cappedInt(score, 100)
}

func calculateFindingQuality(activeBySeverity, bySource, byCategory map[string]int64) int {
	total := int64(0)
	for _, count := range activeBySeverity {
		total += count
	}

	if total == 0 {
		return 0
	}

	score := 30
	if bySource["nuclei"] > 0 {
		score += 20
	}
	if bySource["takeover"] > 0 {
		score += 20
	}
	if bySource["js-intel"] > 0 {
		score += 10
	}
	if byCategory["exposed-interface"] > 0 || byCategory["server-error"] > 0 {
		score -= 5
	}
	if byCategory["js-endpoints"] > 0 && total == byCategory["js-endpoints"] {
		score -= 10
	}

	return cappedInt(score, 100)
}

func commercialRiskLevel(score int) string {
	switch {
	case score >= 85:
		return "critical"
	case score >= 65:
		return "high"
	case score >= 35:
		return "medium"
	case score >= 10:
		return "low"
	default:
		return "informational"
	}
}

func buildCommercialExposureBuckets(activeBySeverity, bySource, byCategory, assets, urls map[string]int64) []map[string]interface{} {
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

	add("Confirmed high-priority findings", "high", "Active critical/high findings that require manual validation and prioritization.", activeBySeverity["critical"]+activeBySeverity["high"])
	add("Takeover candidates", "high", "Potential takeover candidates require CNAME/provider validation before customer-facing claims.", byCategory["subdomain-takeover"])
	add("Client-side secret candidates", "high", "Secret-like values in JavaScript require validation and potential credential rotation.", byCategory["js-secret"])
	add("Sensitive exposed artefacts", "high", "Version-control or configuration artefacts may expose implementation or credential data.", byCategory["exposed-vcs"]+byCategory["exposed-config"])
	add("Exposed sensitive services", "medium", "Open services associated with infrastructure access should be reviewed for exposure and authentication.", byCategory["exposed-service"])
	add("Administrative/login surfaces", "low", "Admin/login indicators are attack-surface context unless paired with access-control weakness.", byCategory["exposed-interface"])
	add("Reconnaissance context", "info", "JavaScript endpoints and crawled URLs support further testing but are not vulnerabilities by themselves.", byCategory["js-endpoints"]+urls["total"])
	add("Live attack surface", "info", "Live assets discovered during DNS and probing phases.", assets["live"])
	add("Nuclei coverage", "info", "Normalized Nuclei findings are present and should be reviewed by severity and template evidence.", bySource["nuclei"])

	return out
}

func buildCommercialCoverageGaps(assets, urls, bySource map[string]int64) []string {
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

func buildCommercialNextActions(assessment commercialAssessment, activeBySeverity, bySource, byCategory, assets, urls map[string]int64) []string {
	actions := make([]string, 0)

	if activeBySeverity["critical"]+activeBySeverity["high"] > 0 {
		actions = append(actions, "Manually validate active critical/high findings and confirm exploitability before customer-facing severity claims.")
	}
	if byCategory["subdomain-takeover"] > 0 {
		actions = append(actions, "Validate takeover candidates by checking DNS CNAMEs, provider ownership state, and safe proof of control.")
	}
	if byCategory["js-secret"] > 0 {
		actions = append(actions, "Validate JavaScript secret candidates; rotate credentials only after confirming token validity and scope.")
	}
	if byCategory["exposed-vcs"]+byCategory["exposed-config"] > 0 {
		actions = append(actions, "Review exposed configuration/version-control artefacts and verify whether sensitive material is accessible.")
	}
	if byCategory["exposed-service"] > 0 {
		actions = append(actions, "Confirm exposed sensitive services are intentional, authenticated, and restricted to trusted networks where possible.")
	}
	if urls["total"] == 0 {
		actions = append(actions, "Run or re-run crawling modules to improve URL and JavaScript coverage.")
	}
	if assets["live"] > 0 && bySource["nuclei"] == 0 {
		actions = append(actions, "Run Nuclei with an appropriate commercial-safe profile against live assets.")
	}
	if assessment.RiskScore < 10 {
		actions = append(actions, "Continue scheduled monitoring and compare future scans for new assets, exposure drift, and newly introduced findings.")
	}

	return actions
}

func buildPrioritizedFindingList(topFindings interface{}, byCategory map[string]int64) []map[string]interface{} {
	out := make([]map[string]interface{}, 0)

	items, ok := topFindings.([]topFinding)
	if !ok {
		return out
	}

	for _, finding := range items {
		priorityScore := priorityScoreForFinding(finding, byCategory)
		out = append(out, map[string]interface{}{
			"id":             finding.ID,
			"title":          finding.Title,
			"severity":       finding.Severity,
			"status":         finding.Status,
			"category":       finding.Category,
			"source_tool":    finding.SourceTool,
			"priority_score": priorityScore,
			"priority":       priorityLevel(priorityScore),
			"rationale":      priorityRationale(finding),
			"last_seen":      finding.LastSeen,
		})
	}

	return out
}

func priorityScoreForFinding(f topFinding, byCategory map[string]int64) int {
	score := 0

	switch strings.ToLower(f.Severity) {
	case models.FindingSeverityCritical:
		score += 80
	case models.FindingSeverityHigh:
		score += 65
	case models.FindingSeverityMedium:
		score += 40
	case models.FindingSeverityLow:
		score += 20
	default:
		score += 5
	}

	switch strings.ToLower(f.Category) {
	case "subdomain-takeover", "js-secret":
		score += 15
	case "exposed-vcs", "exposed-config":
		score += 12
	case "exposed-service":
		score += 10
	case "exposed-interface":
		score += 3
	case "js-endpoints":
		score -= 10
	}

	switch strings.ToLower(f.SourceTool) {
	case "nuclei", "takeover":
		score += 10
	case "js-intel":
		score += 5
	case "builtin":
		score -= 3
	}

	_ = byCategory
	return cappedInt(score, 100)
}

func priorityLevel(score int) string {
	switch {
	case score >= 80:
		return "urgent"
	case score >= 60:
		return "high"
	case score >= 35:
		return "medium"
	case score >= 15:
		return "low"
	default:
		return "informational"
	}
}

func priorityRationale(f topFinding) string {
	category := strings.ToLower(f.Category)
	source := strings.ToLower(f.SourceTool)

	switch category {
	case "subdomain-takeover":
		return "Potential takeover indicators require direct DNS/provider validation before remediation."
	case "js-secret":
		return "Secret-like material requires validation, scope review, and potential credential rotation."
	case "exposed-vcs", "exposed-config":
		return "Exposed artefacts may reveal sensitive implementation or operational details."
	case "exposed-service":
		return "Sensitive service exposure increases attack surface and requires access-control review."
	case "js-endpoints":
		return "Endpoint discovery is reconnaissance context and should feed manual API testing."
	default:
		if source == "nuclei" {
			return "Scanner finding should be reviewed using template evidence and reproducibility."
		}
		return "Finding requires manual validation before customer-facing classification."
	}
}

func scaledContribution(part, total int64, maxScore int) int {
	if total <= 0 || part <= 0 {
		return 0
	}
	ratio := float64(part) / float64(total)
	return cappedInt(int(math.Round(ratio*float64(maxScore))), maxScore)
}

func maxInt64(v, floor int64) int64 {
	if v < floor {
		return floor
	}
	return v
}
