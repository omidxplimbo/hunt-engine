package reportagent

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
	ModelName          = "deterministic-report-v1"
	Methodology        = "report-agent-v1-human-validation-required"
	MaxFindingsLoaded  = 80
	MaxAgentRunsLoaded = 5
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

type localAgentRun struct {
	ID         uint           `json:"id"`
	AgentType  string         `json:"agent_type"`
	Model      string         `json:"model"`
	OutputJSON datatypes.JSON `gorm:"column:output_json" json:"output_json"`
	CreatedAt  time.Time      `json:"created_at"`
}

type reportCandidate struct {
	Title                    string   `json:"title"`
	ValidationStatus         string   `json:"validation_status"`
	Confidence               string   `json:"confidence"`
	SeverityLanguage         string   `json:"severity_language"`
	WeaknessClassification   []string `json:"weakness_classification"`
	Summary                  string   `json:"summary"`
	Scope                    string   `json:"scope"`
	ImpactHypothesis         string   `json:"impact_hypothesis"`
	StepsToReproduceDraft    []string `json:"steps_to_reproduce_draft"`
	EvidenceNeeded           []string `json:"evidence_needed"`
	ValidationChecklist      []string `json:"validation_checklist"`
	SuggestedFix             []string `json:"suggested_fix"`
	PlatformSafeWording      []string `json:"platform_safe_wording"`
	RelatedFindingIDs        []uint   `json:"related_finding_ids"`
	AffectedAssets           []string `json:"affected_assets"`
	HumanValidationRequired  bool     `json:"human_validation_required"`
	DoNotSubmitAutomatically bool     `json:"do_not_submit_automatically"`
}

type output struct {
	AgentType           string          `json:"agent_type"`
	Methodology         string          `json:"methodology"`
	Model               string          `json:"model"`
	GeneratedAt         string          `json:"generated_at"`
	PolicyStatus        string          `json:"policy_status"`
	Guardrails          []string        `json:"guardrails"`
	ReportCandidate     reportCandidate `json:"report_candidate"`
	ImpactHypothesis    string          `json:"impact_hypothesis"`
	EvidenceNeeded      []string        `json:"evidence_needed"`
	ValidationChecklist []string        `json:"validation_checklist"`
	PlatformSafeWording []string        `json:"platform_safe_wording"`
	PolicySafetyNotes   []string        `json:"policy_safety_notes"`
	Assumptions         []string        `json:"assumptions"`
	Limitations         []string        `json:"limitations"`
	SourceContext       map[string]any  `json:"source_context"`
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

	findings, err := loadFindings(input.TargetID)
	if err != nil {
		return nil, err
	}

	priorRuns, err := loadPriorAgentRuns(input.TargetID)
	if err != nil {
		return nil, err
	}

	policyStatus := policyStatusFor(hasPolicy, policy)

	inputSnapshot := map[string]any{
		"target_id":        target.ID,
		"root_domain":      target.RootDomain,
		"findings_count":   len(findings),
		"prior_runs_count": len(priorRuns),
		"has_policy":       hasPolicy,
		"policy_status":    policyStatus,
		"policy":           policy,
		"methodology":      Methodology,
	}

	inputJSON, err := marshalJSON(inputSnapshot)
	if err != nil {
		return nil, err
	}

	out := buildOutput(target, policy, hasPolicy, policyStatus, findings, priorRuns)

	outputJSON, err := marshalJSON(out)
	if err != nil {
		return nil, err
	}

	completed := time.Now().UTC()

	row := models.AgentRun{
		TargetID:        target.ID,
		CreatedByUserID: input.CreatedByUserID,
		AgentType:       models.AgentRunTypeReport,
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

func loadFindings(targetID uint) ([]localFinding, error) {
	rows := make([]localFinding, 0)

	err := database.DB.
		Table("findings").
		Select("id, title, severity, category, source_tool, evidence, evidence_json, status, created_at").
		Where("target_id = ? AND deleted_at IS NULL", targetID).
		Order(`
			CASE LOWER(severity)
				WHEN 'critical' THEN 1
				WHEN 'high' THEN 2
				WHEN 'medium' THEN 3
				WHEN 'low' THEN 4
				ELSE 5
			END,
			created_at DESC,
			id DESC
		`).
		Limit(MaxFindingsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load findings: %w", err)
	}

	return rows, nil
}

func loadPriorAgentRuns(targetID uint) ([]localAgentRun, error) {
	rows := make([]localAgentRun, 0)

	err := database.DB.
		Table("agent_runs").
		Select("id, agent_type, model, output_json, created_at").
		Where("target_id = ? AND status = ? AND agent_type IN ?", targetID, models.AgentRunStatusCompleted, []string{models.AgentRunTypeSummary, models.AgentRunTypeTriage}).
		Order("created_at desc, id desc").
		Limit(MaxAgentRunsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load prior agent runs: %w", err)
	}

	return rows, nil
}

func buildOutput(target models.Target, policy localPolicy, hasPolicy bool, policyStatus string, findings []localFinding, priorRuns []localAgentRun) output {
	primary := choosePrimaryFinding(findings)
	related := relatedFindings(findings, primary)
	assets := affectedAssets(related)
	ids := relatedFindingIDs(related)

	candidate := buildReportCandidate(target, policy, hasPolicy, primary, related, ids, assets)

	sourceContext := map[string]any{
		"findings_sampled":    len(findings),
		"prior_agent_runs":    summarizePriorRuns(priorRuns),
		"primary_finding_id":  primary.ID,
		"related_finding_ids": ids,
	}

	return output{
		AgentType:    models.AgentRunTypeReport,
		Methodology:  Methodology,
		Model:        ModelName,
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		PolicyStatus: policyStatus,
		Guardrails: []string{
			"Report Agent output is a draft only.",
			"Human validation is required before using this as a customer or bug bounty report.",
			"Do not auto-submit reports.",
			"Do not claim confirmed impact unless the evidence is manually reproduced.",
			"Deterministic findings, severity, risk score, and policy enforcement remain authoritative.",
		},
		ReportCandidate:     candidate,
		ImpactHypothesis:    candidate.ImpactHypothesis,
		EvidenceNeeded:      candidate.EvidenceNeeded,
		ValidationChecklist: candidate.ValidationChecklist,
		PlatformSafeWording: candidate.PlatformSafeWording,
		PolicySafetyNotes:   buildPolicyNotes(policy, hasPolicy),
		Assumptions: []string{
			"The draft is generated only from stored Hunt Engine data.",
			"Stored findings may include false positives or stale results.",
			"Scope and authorization must be verified before testing or reporting.",
		},
		Limitations: []string{
			"This agent does not verify exploitability.",
			"This agent does not execute proof-of-concept payloads.",
			"This agent does not submit reports to any platform.",
		},
		SourceContext: sourceContext,
	}
}

func choosePrimaryFinding(findings []localFinding) localFinding {
	if len(findings) == 0 {
		return localFinding{
			Title:      "Potential attack surface issue requires manual validation",
			Severity:   "informational",
			Category:   "manual-review",
			SourceTool: "hunt-engine",
		}
	}

	for _, f := range findings {
		status := strings.ToLower(strings.TrimSpace(f.Status))
		if status == "false_positive" || status == "fixed" || status == "resolved" {
			continue
		}
		return f
	}

	return findings[0]
}

func relatedFindings(findings []localFinding, primary localFinding) []localFinding {
	out := []localFinding{}
	seen := map[uint]bool{}

	add := func(f localFinding) {
		if f.ID != 0 && seen[f.ID] {
			return
		}
		if f.ID != 0 {
			seen[f.ID] = true
		}
		out = append(out, f)
	}

	add(primary)

	for _, f := range findings {
		if len(out) >= 6 {
			break
		}
		if f.ID == primary.ID {
			continue
		}
		if strings.EqualFold(f.Category, primary.Category) || strings.EqualFold(f.SourceTool, primary.SourceTool) || strings.EqualFold(f.Severity, primary.Severity) {
			add(f)
		}
	}

	return out
}

func buildReportCandidate(target models.Target, policy localPolicy, hasPolicy bool, primary localFinding, related []localFinding, relatedIDs []uint, assets []string) reportCandidate {
	title := safeTitle(primary, target.RootDomain)
	classifications := weaknessClassification(primary)
	summary := fmt.Sprintf(
		"Hunt Engine identified a candidate issue on %s related to %s. This is a draft report candidate and requires manual validation before submission.",
		target.RootDomain,
		nonEmpty(primary.Category, "the observed finding"),
	)

	scope := "Scope must be manually verified before testing or reporting."
	if hasPolicy {
		scope = "Target policy exists. Verify the affected asset against in-scope and out-of-scope rules before using this draft."
		if strings.TrimSpace(policy.ProgramURL) != "" {
			scope += " Program URL: " + strings.TrimSpace(policy.ProgramURL)
		}
	}

	impact := impactHypothesis(primary)
	evidence := evidenceNeeded(primary)
	validation := validationChecklist(primary)
	wording := platformSafeWording(primary)
	fix := suggestedFix(primary)

	return reportCandidate{
		Title:                    title,
		ValidationStatus:         "needs_manual_validation",
		Confidence:               confidenceFor(primary),
		SeverityLanguage:         severityLanguage(primary),
		WeaknessClassification:   classifications,
		Summary:                  summary,
		Scope:                    scope,
		ImpactHypothesis:         impact,
		StepsToReproduceDraft:    reproductionDraft(primary, assets),
		EvidenceNeeded:           evidence,
		ValidationChecklist:      validation,
		SuggestedFix:             fix,
		PlatformSafeWording:      wording,
		RelatedFindingIDs:        relatedIDs,
		AffectedAssets:           assets,
		HumanValidationRequired:  true,
		DoNotSubmitAutomatically: true,
	}
}

func safeTitle(f localFinding, rootDomain string) string {
	title := strings.TrimSpace(f.Title)
	if title == "" {
		title = strings.TrimSpace(f.Category)
	}
	if title == "" {
		title = "Manual validation required"
	}

	return fmt.Sprintf("[Draft] %s on %s", title, rootDomain)
}

func impactHypothesis(f localFinding) string {
	category := strings.ToLower(strings.TrimSpace(f.Category))
	tool := strings.ToLower(strings.TrimSpace(f.SourceTool))
	title := strings.ToLower(strings.TrimSpace(f.Title))

	switch {
	case strings.Contains(category, "takeover") || strings.Contains(title, "takeover") || strings.Contains(tool, "takeover"):
		return "If validated, this may allow an attacker to claim or control a dangling subdomain and serve attacker-controlled content under the target's domain."
	case strings.Contains(category, "js") || strings.Contains(tool, "js"):
		return "If validated, the JavaScript signal may expose sensitive endpoints, source maps, or client-side behavior useful for further security testing."
	case strings.Contains(category, "exposed") || strings.Contains(title, "admin") || strings.Contains(title, "login"):
		return "If validated and improperly protected, exposed administrative or authentication interfaces may increase attack surface and enable targeted testing."
	case strings.Contains(tool, "nuclei"):
		return "If validated, the template match may indicate a reproducible misconfiguration or vulnerable exposure depending on response context and affected asset criticality."
	default:
		return "If validated, this issue may increase the target's attack surface or indicate a misconfiguration requiring remediation."
	}
}

func evidenceNeeded(f localFinding) []string {
	out := []string{
		"Current affected asset URL or hostname.",
		"Timestamped request and response evidence.",
		"Screenshot or terminal output showing the observed behavior.",
		"Confirmation that the asset is in scope.",
		"Manual reproduction notes from a safe, non-destructive validation.",
	}

	if len(f.EvidenceJSON) > 2 {
		out = append(out, "Structured evidence_json fields mapped to the reproduction steps.")
	}
	if strings.TrimSpace(f.Evidence) != "" {
		out = append(out, "Legacy evidence text reviewed and converted into clear proof.")
	}

	return out
}

func validationChecklist(f localFinding) []string {
	out := []string{
		"Verify the affected asset is in scope and not explicitly excluded.",
		"Reproduce the behavior manually from a clean session.",
		"Confirm the finding is still present at the time of reporting.",
		"Determine whether authentication, rate limits, WAF/CDN, or environment constraints affect impact.",
		"Collect minimal evidence necessary to prove impact without destructive testing.",
	}

	category := strings.ToLower(f.Category)
	if strings.Contains(category, "takeover") {
		out = append(out, "Validate DNS/CNAME chain and provider-specific takeover conditions.")
	}
	if strings.Contains(category, "js") {
		out = append(out, "Review JavaScript source manually and confirm whether any endpoint or sink is security relevant.")
	}

	return out
}

func reproductionDraft(f localFinding, assets []string) []string {
	asset := "the affected asset"
	if len(assets) > 0 {
		asset = assets[0]
	}

	return []string{
		"Open or query " + asset + " using a safe, non-destructive request.",
		"Observe the behavior described by finding #" + fmt.Sprintf("%d", f.ID) + ".",
		"Capture the relevant response, header, DNS, or page evidence.",
		"Explain why the behavior has security impact in the target's context.",
	}
}

func platformSafeWording(f localFinding) []string {
	return []string{
		"This is a draft report candidate generated from stored scanner evidence.",
		"Manual validation is required before submission.",
		"Based on current evidence, the issue appears to be a candidate for further review rather than a confirmed exploit.",
		"Severity should be adjusted only after exploitability, scope, and impact are validated.",
	}
}

func suggestedFix(f localFinding) []string {
	category := strings.ToLower(strings.TrimSpace(f.Category))
	title := strings.ToLower(strings.TrimSpace(f.Title))

	switch {
	case strings.Contains(category, "takeover") || strings.Contains(title, "takeover"):
		return []string{
			"Remove dangling DNS records or reconfigure them to point to active owned resources.",
			"Continuously monitor DNS records for orphaned provider references.",
		}
	case strings.Contains(category, "js"):
		return []string{
			"Remove sensitive data from client-side JavaScript and source maps.",
			"Restrict or protect sensitive endpoints discovered from JavaScript.",
			"Deploy appropriate CSP and asset hosting controls where relevant.",
		}
	case strings.Contains(title, "admin") || strings.Contains(title, "login") || strings.Contains(category, "exposed"):
		return []string{
			"Restrict administrative interfaces by VPN, SSO, IP allowlist, or private network controls.",
			"Enforce strong authentication, MFA, rate limiting, and monitoring.",
		}
	default:
		return []string{
			"Review the affected configuration and reduce unnecessary exposure.",
			"Validate remediation with a follow-up scan and manual verification.",
		}
	}
}

func weaknessClassification(f localFinding) []string {
	out := []string{}
	category := strings.ToLower(strings.TrimSpace(f.Category))
	title := strings.ToLower(strings.TrimSpace(f.Title))

	switch {
	case strings.Contains(category, "takeover") || strings.Contains(title, "takeover"):
		out = append(out, "CWE-350 Reliance on Reverse DNS Resolution for a Security-Critical Action", "Subdomain Takeover Candidate")
	case strings.Contains(category, "js"):
		out = append(out, "Information Exposure Candidate", "Client-Side Attack Surface")
	case strings.Contains(title, "admin") || strings.Contains(title, "login"):
		out = append(out, "Exposed Administrative Interface Candidate")
	default:
		out = append(out, "Security Misconfiguration Candidate")
	}

	return out
}

func affectedAssets(findings []localFinding) []string {
	set := map[string]bool{}

	for _, f := range findings {
		for _, v := range extractEvidenceStrings(f.EvidenceJSON) {
			if looksLikeAsset(v) {
				set[v] = true
			}
		}
	}

	out := []string{}
	for v := range set {
		out = append(out, v)
	}
	sort.Strings(out)

	if len(out) > 10 {
		return out[:10]
	}
	return out
}

func extractEvidenceStrings(raw datatypes.JSON) []string {
	if len(raw) == 0 {
		return nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil
	}

	out := []string{}
	var walk func(any)
	walk = func(v any) {
		switch t := v.(type) {
		case string:
			out = append(out, strings.TrimSpace(t))
		case []any:
			for _, item := range t {
				walk(item)
			}
		case map[string]any:
			for _, item := range t {
				walk(item)
			}
		}
	}

	walk(value)
	return cleanStrings(out)
}

func looksLikeAsset(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.Contains(value, " ") {
		return false
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		return true
	}
	return strings.Contains(value, ".") && !strings.Contains(value, "@")
}

func relatedFindingIDs(findings []localFinding) []uint {
	out := []uint{}
	for _, f := range findings {
		if f.ID != 0 {
			out = append(out, f.ID)
		}
	}
	return out
}

func confidenceFor(f localFinding) string {
	if len(f.EvidenceJSON) > 2 {
		return "medium"
	}
	if strings.TrimSpace(f.Evidence) != "" {
		return "low-medium"
	}
	return "low"
}

func severityLanguage(f localFinding) string {
	sev := strings.ToLower(strings.TrimSpace(f.Severity))
	switch sev {
	case "critical", "high":
		return "Use cautious severity wording until exploitability and business impact are manually confirmed."
	case "medium":
		return "Moderate severity language may be appropriate only after validating impact and scope."
	default:
		return "Use low-confidence wording unless additional evidence proves higher impact."
	}
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

func buildPolicyNotes(policy localPolicy, hasPolicy bool) []string {
	if !hasPolicy {
		return []string{
			"No target policy exists yet. Verify scope manually before using this report draft.",
			"Do not run active validation or submit a report without explicit scope confirmation.",
		}
	}

	out := []string{"Target policy exists and was considered while drafting report language."}
	if strings.TrimSpace(policy.PlatformName) != "" {
		out = append(out, "Platform: "+policy.PlatformName)
	}
	if strings.TrimSpace(policy.MaxTestIntensity) != "" {
		out = append(out, "Max test intensity: "+policy.MaxTestIntensity)
	}
	if strings.TrimSpace(policy.SafeTestingNotes) != "" {
		out = append(out, "Safe testing notes: "+policy.SafeTestingNotes)
	}
	if strings.TrimSpace(policy.ReportingPreferences) != "" {
		out = append(out, "Reporting preferences: "+policy.ReportingPreferences)
	}
	return out
}

func summarizePriorRuns(rows []localAgentRun) []map[string]any {
	out := []map[string]any{}

	for _, row := range rows {
		out = append(out, map[string]any{
			"id":         row.ID,
			"agent_type": row.AgentType,
			"model":      row.Model,
			"created_at": row.CreatedAt.Format(time.RFC3339),
		})
	}

	return out
}

func nonEmpty(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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

func marshalJSON(v any) (datatypes.JSON, error) {
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
