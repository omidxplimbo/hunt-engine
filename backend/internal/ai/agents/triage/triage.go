package triage

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
	ModelName         = "deterministic-triage-v1"
	Methodology      = "triage-agent-v1-policy-aware-advisory"
	MaxFindingsLoaded = 200
	MaxAssetsLoaded   = 100
)

type Input struct {
	TargetID        uint
	CreatedByUserID *uint
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

type localAsset struct {
	Value        string         `json:"value"`
	IsLive       bool           `json:"is_live"`
	StatusCode   int            `json:"status_code"`
	Title        string         `json:"title"`
	CDNName      string         `gorm:"column:cdn_name" json:"cdn_name"`
	Wafcheck     bool           `json:"wafcheck"`
	WafcheckName string         `json:"wafcheck_name"`
	Technologies datatypes.JSON `json:"technologies"`
	OpenPorts    datatypes.JSON `json:"open_ports"`
}

type localPolicy struct {
	PlatformName            string         `json:"platform_name"`
	ProgramURL              string         `json:"program_url"`
	InScopePatterns          datatypes.JSON `json:"in_scope_patterns"`
	OutOfScopePatterns       datatypes.JSON `json:"out_of_scope_patterns"`
	AllowedTestTypes         datatypes.JSON `json:"allowed_test_types"`
	DisallowedTestTypes      datatypes.JSON `json:"disallowed_test_types"`
	MaxTestIntensity         string         `json:"max_test_intensity"`
	AuthRequired            bool           `json:"auth_required"`
	SafeTestingNotes         string         `json:"safe_testing_notes"`
	ReportingPreferences    string         `json:"reporting_preferences"`
	BusinessContext         string         `json:"business_context"`
	AssetCriticalityDefault string         `json:"asset_criticality_default"`
}

type triageFinding struct {
	FindingID            uint     `json:"finding_id"`
	Title                string   `json:"title"`
	Severity             string   `json:"severity"`
	Category             string   `json:"category"`
	SourceTool           string   `json:"source_tool"`
	InterestScore        int      `json:"interest_score"`
	BugBountyValue        string   `json:"bug_bounty_value"`
	WhyInteresting       []string `json:"why_interesting"`
	EvidenceSummary      []string `json:"evidence_summary"`
	ManualValidationHint []string `json:"manual_validation_hint"`
	FalsePositiveRisk    string   `json:"false_positive_risk"`
}

type manualTest struct {
	TestType      string   `json:"test_type"`
	Priority      string   `json:"priority"`
	Why           string   `json:"why"`
	SafetyLevel   string   `json:"safety_level"`
	PolicyNotes   []string `json:"policy_notes"`
	Validation    []string `json:"validation"`
}

type output struct {
	AgentType              string          `json:"agent_type"`
	Methodology            string          `json:"methodology"`
	Model                  string          `json:"model"`
	GeneratedAt            string          `json:"generated_at"`
	AdvisoryPriority       string          `json:"advisory_priority"`
	PolicyStatus           string          `json:"policy_status"`
	Guardrails             []string        `json:"guardrails"`
	Summary                string          `json:"summary"`
	TopInterestingFindings []triageFinding `json:"top_interesting_findings"`
	RecommendedManualTests []manualTest     `json:"recommended_manual_tests"`
	ManualValidationSteps  []string        `json:"manual_validation_steps"`
	FalsePositiveRisks     []string        `json:"false_positive_risks"`
	BugBountyValueNotes     []string        `json:"bug_bounty_value_notes"`
	PolicySafetyNotes      []string        `json:"policy_safety_notes"`
	CoverageNotes           []string        `json:"coverage_notes"`
	Assumptions             []string        `json:"assumptions"`
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

	assets, err := loadAssets(input.TargetID)
	if err != nil {
		return nil, err
	}

	urlCount := countFoundURLs(input.TargetID)

	policyStatus := policyStatusFor(hasPolicy, policy)
	inputSnapshot := map[string]interface{}{
		"target_id":       target.ID,
		"root_domain":     target.RootDomain,
		"findings_count":  len(findings),
		"assets_count":    len(assets),
		"found_url_count": urlCount,
		"has_policy":      hasPolicy,
		"policy_status":   policyStatus,
		"policy":          policy,
		"methodology":     Methodology,
	}

	inputJSON, err := marshalJSON(inputSnapshot)
	if err != nil {
		return nil, err
	}

	out := buildOutput(target, policy, hasPolicy, policyStatus, findings, assets, urlCount)

	outputJSON, err := marshalJSON(out)
	if err != nil {
		return nil, err
	}

	completed := time.Now().UTC()

	row := models.AgentRun{
		TargetID:        target.ID,
		CreatedByUserID: input.CreatedByUserID,
		AgentType:       models.AgentRunTypeTriage,
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
		Order("created_at desc, id desc").
		Limit(MaxFindingsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load findings: %w", err)
	}

	return rows, nil
}

func loadAssets(targetID uint) ([]localAsset, error) {
	rows := make([]localAsset, 0)

	err := database.DB.
		Model(&models.Asset{}).
		Select("value, is_live, status_code, title, cdn_name, wafcheck, wafcheck_name, technologies, open_ports").
		Where("target_id = ?", targetID).
		Order("is_live desc, status_code desc, value asc").
		Limit(MaxAssetsLoaded).
		Scan(&rows).Error

	if err != nil {
		return nil, fmt.Errorf("load assets: %w", err)
	}

	return rows, nil
}

func countFoundURLs(targetID uint) int64 {
	var total int64
	_ = database.DB.Table("found_urls").Where("target_id = ?", targetID).Count(&total).Error
	return total
}

func buildOutput(target models.Target, policy localPolicy, hasPolicy bool, policyStatus string, findings []localFinding, assets []localAsset, urlCount int64) output {
	top := rankFindings(findings)
	tests := recommendedTests(top, policy, hasPolicy)

	priority := "low"
	if len(top) > 0 {
		switch {
		case top[0].InterestScore >= 90:
			priority = "high"
		case top[0].InterestScore >= 60:
			priority = "medium"
		}
	}

	policyNotes := []string{}
	if hasPolicy {
		policyNotes = append(policyNotes, "Target policy exists and was considered before recommending manual tests.")
		if strings.TrimSpace(policy.MaxTestIntensity) != "" {
			policyNotes = append(policyNotes, "Max test intensity: "+policy.MaxTestIntensity)
		}
		if strings.TrimSpace(policy.SafeTestingNotes) != "" {
			policyNotes = append(policyNotes, "Safe testing notes: "+policy.SafeTestingNotes)
		}
	} else {
		policyNotes = append(policyNotes, "No target policy exists yet. Keep tests passive/manual until scope rules are defined.")
	}

	summary := fmt.Sprintf(
		"Triage reviewed %d findings, %d sampled assets, and %d discovered URLs for %s. Output is advisory and does not modify deterministic risk or finding severity.",
		len(findings),
		len(assets),
		urlCount,
		target.RootDomain,
	)

	return output{
		AgentType:        models.AgentRunTypeTriage,
		Methodology:      Methodology,
		Model:            ModelName,
		GeneratedAt:      time.Now().UTC().Format(time.RFC3339),
		AdvisoryPriority: priority,
		PolicyStatus:     policyStatus,
		Guardrails: []string{
			"Agent output is advisory only.",
			"Deterministic findings, severity, risk score, and policy enforcement remain authoritative.",
			"No active exploitation, destructive testing, authentication bypass, or out-of-scope testing is performed by this agent.",
			"Recommended tests require human validation and must respect target policy.",
		},
		Summary:                summary,
		TopInterestingFindings: top,
		RecommendedManualTests: tests,
		ManualValidationSteps: []string{
			"Review top findings against stored evidence_json before taking action.",
			"Confirm affected asset and scope status in Target Policy.",
			"Validate manually in a safe, non-destructive way.",
			"Preserve request/response evidence before preparing any report.",
			"Do not submit or escalate without human verification.",
		},
		FalsePositiveRisks: []string{
			"Reconnaissance-only findings may indicate exposure but not confirmed vulnerability.",
			"Nuclei/template findings should be validated against live evidence and response context.",
			"Takeover-like signals require DNS/CNAME and provider-specific validation.",
			"JavaScript endpoint discovery is usually a lead, not a confirmed vulnerability.",
		},
		BugBountyValueNotes: []string{
			"High-value candidates usually combine exploitability, sensitive asset exposure, clear impact, and reproducible evidence.",
			"Findings with structured evidence and direct affected URLs/assets are better report candidates.",
			"Coverage gaps are useful for hunting strategy but should not be reported as vulnerabilities.",
		},
		PolicySafetyNotes: policyNotes,
		CoverageNotes: []string{
			fmt.Sprintf("Sampled live/known assets: %d", len(assets)),
			fmt.Sprintf("Discovered URL count: %d", urlCount),
			"Coverage notes must not inflate risk without concrete vulnerability evidence.",
		},
		Assumptions: []string{
			"The triage agent only used data already stored in Hunt Engine.",
			"Target policy is treated as advisory context here; execution blocking remains deterministic.",
			"Manual testing recommendations are safe-by-default and require human judgment.",
		},
	}
}

func rankFindings(findings []localFinding) []triageFinding {
	out := make([]triageFinding, 0)

	for _, f := range findings {
		status := strings.ToLower(strings.TrimSpace(f.Status))
		if status == "false_positive" || status == "resolved" || status == "closed" {
			continue
		}

		score := interestScore(f)
		if score < 25 {
			continue
		}

		out = append(out, triageFinding{
			FindingID:            f.ID,
			Title:                clean(f.Title),
			Severity:             clean(f.Severity),
			Category:             clean(f.Category),
			SourceTool:           clean(f.SourceTool),
			InterestScore:        score,
			BugBountyValue:        bugBountyValue(score, f),
			WhyInteresting:       whyInteresting(f),
			EvidenceSummary:      evidenceSummary(f),
			ManualValidationHint: validationHints(f),
			FalsePositiveRisk:    falsePositiveRisk(f),
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		return out[i].InterestScore > out[j].InterestScore
	})

	if len(out) > 10 {
		out = out[:10]
	}

	return out
}

func interestScore(f localFinding) int {
	score := severityWeight(f.Severity)

	cat := strings.ToLower(f.Category)
	source := strings.ToLower(f.SourceTool)
	title := strings.ToLower(f.Title)

	if strings.Contains(cat, "takeover") || strings.Contains(title, "takeover") {
		score += 20
	}
	if strings.Contains(source, "nuclei") {
		score += 12
	}
	if strings.Contains(source, "js") || strings.Contains(cat, "js-") {
		score += 8
	}
	if strings.Contains(cat, "secret") || strings.Contains(title, "secret") || strings.Contains(title, "token") || strings.Contains(title, "key") {
		score += 20
	}
	if strings.Contains(cat, "xss") || strings.Contains(title, "xss") {
		score += 18
	}
	if strings.Contains(cat, "redirect") || strings.Contains(title, "redirect") {
		score += 12
	}
	if strings.Contains(cat, "cors") || strings.Contains(title, "cors") {
		score += 10
	}
	if strings.Contains(cat, "server-error") || strings.Contains(title, "server error") {
		score += 6
	}

	if score > 100 {
		return 100
	}
	if score < 0 {
		return 0
	}
	return score
}

func severityWeight(sev string) int {
	switch strings.ToLower(strings.TrimSpace(sev)) {
	case "critical":
		return 90
	case "high":
		return 75
	case "medium":
		return 55
	case "low":
		return 30
	case "info", "informational":
		return 15
	default:
		return 25
	}
}

func bugBountyValue(score int, f localFinding) string {
	text := strings.ToLower(f.Title + " " + f.Category + " " + f.SourceTool)

	switch {
	case score >= 85:
		return "high"
	case score >= 60:
		return "medium"
	case strings.Contains(text, "js") || strings.Contains(text, "endpoint"):
		return "lead"
	default:
		return "low"
	}
}

func whyInteresting(f localFinding) []string {
	out := []string{}

	if strings.TrimSpace(f.Severity) != "" {
		out = append(out, "Severity signal: "+f.Severity)
	}
	if strings.TrimSpace(f.SourceTool) != "" {
		out = append(out, "Source tool: "+f.SourceTool)
	}
	if strings.TrimSpace(f.Category) != "" {
		out = append(out, "Category: "+f.Category)
	}

	text := strings.ToLower(f.Title + " " + f.Category)
	if strings.Contains(text, "takeover") {
		out = append(out, "Potential account/asset control impact if confirmed.")
	}
	if strings.Contains(text, "secret") || strings.Contains(text, "token") || strings.Contains(text, "key") {
		out = append(out, "Potential sensitive credential exposure if confirmed.")
	}
	if strings.Contains(text, "xss") {
		out = append(out, "Potential client-side impact; requires safe manual validation.")
	}
	if strings.Contains(text, "redirect") {
		out = append(out, "Potential phishing/auth flow abuse if exploitable.")
	}

	if len(out) == 0 {
		out = append(out, "Finding may guide manual testing, but needs validation.")
	}

	return out
}

func evidenceSummary(f localFinding) []string {
	out := []string{}
	raw := []byte(f.EvidenceJSON)

	if len(raw) > 0 && string(raw) != "{}" && string(raw) != "null" {
		var obj map[string]interface{}
		if err := json.Unmarshal(raw, &obj); err == nil {
			keys := []string{
				"asset",
				"url",
				"final_url",
				"template_id",
				"matcher",
				"provider",
				"cname",
				"js_url",
				"signal_type",
				"status_code",
				"confidence",
			}

			for _, key := range keys {
				if value, ok := obj[key]; ok {
					text := clean(value)
					if text != "" {
						out = append(out, key+": "+text)
					}
				}
			}
		}
	}

	if len(out) == 0 && strings.TrimSpace(f.Evidence) != "" {
		out = append(out, truncate(f.Evidence, 220))
	}

	if len(out) == 0 {
		out = append(out, "No structured evidence summary available; inspect raw finding evidence.")
	}

	return out
}

func validationHints(f localFinding) []string {
	text := strings.ToLower(f.Title + " " + f.Category + " " + f.SourceTool)
	out := []string{}

	switch {
	case strings.Contains(text, "takeover"):
		out = append(out, "Verify CNAME/provider ownership state using passive DNS and provider-specific takeover conditions.")
		out = append(out, "Do not claim takeover without provider-specific confirmation.")
	case strings.Contains(text, "xss"):
		out = append(out, "Use inert reflection/context checks first; avoid destructive or user-impacting payloads.")
		out = append(out, "Confirm source, sink, encoding context, and exploitability manually.")
	case strings.Contains(text, "redirect"):
		out = append(out, "Check whether redirects allow external destinations and whether allowlists are bypassable.")
	case strings.Contains(text, "cors"):
		out = append(out, "Validate Origin reflection, credentials behavior, and sensitive response exposure.")
	case strings.Contains(text, "secret") || strings.Contains(text, "token") || strings.Contains(text, "key"):
		out = append(out, "Confirm whether the value is real, active, scoped, and sensitive without abusing it.")
	default:
		out = append(out, "Open the affected asset/URL, confirm reproducibility, and collect safe evidence.")
	}

	return out
}

func falsePositiveRisk(f localFinding) string {
	text := strings.ToLower(f.Title + " " + f.Category + " " + f.SourceTool)

	switch {
	case strings.Contains(text, "takeover"):
		return "medium-high: takeover fingerprints can be noisy without CNAME/provider confirmation."
	case strings.Contains(text, "js") || strings.Contains(text, "endpoint"):
		return "medium: JavaScript-discovered endpoints are leads, not confirmed vulnerabilities."
	case strings.Contains(text, "nuclei"):
		return "medium: template findings require response/evidence validation."
	case strings.Contains(text, "secret") || strings.Contains(text, "token"):
		return "medium: exposed-looking tokens may be test, expired, public, or non-sensitive."
	default:
		return "medium: validate manually before reporting."
	}
}

func recommendedTests(top []triageFinding, policy localPolicy, hasPolicy bool) []manualTest {
	tests := make([]manualTest, 0)
	seen := map[string]bool{}

	add := func(test manualTest) {
		if seen[test.TestType] {
			return
		}
		seen[test.TestType] = true
		tests = append(tests, test)
	}

	for _, item := range top {
		text := strings.ToLower(item.Title + " " + item.Category)

		switch {
		case strings.Contains(text, "takeover"):
			add(manualTest{
				TestType:    "subdomain_takeover_validation",
				Priority:    "high",
				Why:         "Takeover candidates can have high impact if provider ownership is truly unclaimed.",
				SafetyLevel: "passive_or_manual_safe",
				PolicyNotes: policyNotesForTest(policy, hasPolicy),
				Validation: []string{
					"Confirm DNS CNAME target.",
					"Confirm provider-specific unclaimed-resource signal.",
					"Avoid creating resources unless explicitly allowed by program policy.",
				},
			})
		case strings.Contains(text, "xss"):
			add(manualTest{
				TestType:    "safe_xss_candidate_review",
				Priority:    "medium",
				Why:         "XSS-like signals can be valuable but require context-aware manual validation.",
				SafetyLevel: "manual_safe",
				PolicyNotes: policyNotesForTest(policy, hasPolicy),
				Validation: []string{
					"Check reflection or DOM sink context.",
					"Use inert payloads first.",
					"Do not test authenticated/user-impacting flows without permission.",
				},
			})
		case strings.Contains(text, "redirect"):
			add(manualTest{
				TestType:    "open_redirect_candidate_review",
				Priority:    "medium",
				Why:         "Redirect weaknesses can support phishing or auth-flow abuse if exploitable.",
				SafetyLevel: "manual_safe",
				PolicyNotes: policyNotesForTest(policy, hasPolicy),
				Validation: []string{
					"Identify redirect parameter.",
					"Check allowlist and parser bypass behavior.",
					"Collect request/response evidence.",
				},
			})
		case strings.Contains(text, "cors"):
			add(manualTest{
				TestType:    "cors_misconfiguration_review",
				Priority:    "medium",
				Why:         "CORS issues matter when sensitive responses and credentials are involved.",
				SafetyLevel: "manual_safe",
				PolicyNotes: policyNotesForTest(policy, hasPolicy),
				Validation: []string{
					"Check Origin reflection.",
					"Check Access-Control-Allow-Credentials.",
					"Confirm sensitive response exposure.",
				},
			})
		case strings.Contains(text, "secret") || strings.Contains(text, "token") || strings.Contains(text, "key"):
			add(manualTest{
				TestType:    "exposed_secret_review",
				Priority:    "high",
				Why:         "Credential-like exposure may be high impact if active and scoped to sensitive systems.",
				SafetyLevel: "passive_manual",
				PolicyNotes: policyNotesForTest(policy, hasPolicy),
				Validation: []string{
					"Do not abuse the secret.",
					"Confirm whether the value is public/test/expired using non-invasive checks.",
					"Capture location and context evidence.",
				},
			})
		}
	}

	if len(tests) == 0 {
		add(manualTest{
			TestType:    "manual_evidence_review",
			Priority:    "low",
			Why:         "No high-confidence bug-class candidates were identified; review evidence and coverage gaps manually.",
			SafetyLevel: "passive",
			PolicyNotes: policyNotesForTest(policy, hasPolicy),
			Validation: []string{
				"Review structured evidence.",
				"Prioritize assets with live services and sensitive technologies.",
				"Avoid active testing until policy allows it.",
			},
		})
	}

	return tests
}

func policyNotesForTest(policy localPolicy, hasPolicy bool) []string {
	if !hasPolicy {
		return []string{"No target policy found. Treat this as passive/manual-only until scope and allowed tests are configured."}
	}

	out := []string{"Target policy exists; verify this test is allowed before execution."}

	if strings.TrimSpace(policy.MaxTestIntensity) != "" {
		out = append(out, "Max intensity: "+policy.MaxTestIntensity)
	}

	if strings.TrimSpace(policy.SafeTestingNotes) != "" {
		out = append(out, "Safe testing notes: "+policy.SafeTestingNotes)
	}

	return out
}

func policyStatusFor(hasPolicy bool, policy localPolicy) string {
	if !hasPolicy {
		return models.AgentRunPolicyStatusWarning
	}

	if strings.TrimSpace(policy.MaxTestIntensity) == "" {
		return models.AgentRunPolicyStatusWarning
	}

	return models.AgentRunPolicyStatusAllowed
}

func marshalJSON(value interface{}) (datatypes.JSON, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return datatypes.JSON(b), nil
}

func digestJSON(raw datatypes.JSON) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func clean(value interface{}) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "<nil>" {
		return ""
	}
	return text
}

func truncate(value string, max int) string {
	value = strings.TrimSpace(value)
	if len(value) <= max {
		return value
	}
	return value[:max] + "..."
}
