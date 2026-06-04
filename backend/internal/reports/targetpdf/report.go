package targetpdf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
)

type ReportData struct {
	GeneratedAt time.Time
	Target      models.Target
	ScanState   *models.TargetScanState

	Counts ReportCounts

	FindingsBySeverity map[string]int64
	FindingsByStatus   map[string]int64
	FindingsBySource   map[string]int64
	FindingsByCategory map[string]int64

	TopFindings      []models.Finding
	TakeoverFindings []models.Finding
	JSFindings       []models.Finding
	NucleiFindings   []models.Finding
	RecentAssets     []models.Asset
	RecentURLs       []models.FoundURL
	AssetChanges     []models.AssetHistory
	LatestAnalysis   *models.AIAnalysis
	LatestSummaryRun *models.AgentRun
	LatestTriageRun  *models.AgentRun
	LatestReportRun  *models.AgentRun
}

type ReportCounts struct {
	TotalAssets int64
	LiveAssets  int64
	DeadAssets  int64
	WebAssets   int64
	CDNAssets   int64
	WAFAssets   int64
	CloudAssets int64
	PortAssets  int64

	TotalURLs int64
	JSURLs    int64

	TotalFindings int64
	OpenFindings  int64
}

type statRow struct {
	Key   string `gorm:"column:key"`
	Count int64  `gorm:"column:count"`
}

type barRow struct {
	Label string
	Value int64
}

var filenameUnsafe = regexp.MustCompile(`[^a-zA-Z0-9._-]+`)

func Generate(targetID uint) ([]byte, string, error) {
	data, err := Load(targetID)
	if err != nil {
		return nil, "", err
	}

	pdfBytes, err := Render(data)
	if err != nil {
		return nil, "", err
	}

	return pdfBytes, Filename(data), nil
}

func Load(targetID uint) (*ReportData, error) {
	if targetID == 0 {
		return nil, fmt.Errorf("target id is required")
	}

	data := &ReportData{
		GeneratedAt:        time.Now().UTC(),
		FindingsBySeverity: map[string]int64{},
		FindingsByStatus:   map[string]int64{},
		FindingsBySource:   map[string]int64{},
		FindingsByCategory: map[string]int64{},
	}

	if err := database.DB.First(&data.Target, targetID).Error; err != nil {
		return nil, fmt.Errorf("load target: %w", err)
	}

	var scanState models.TargetScanState
	if err := database.DB.Where("target_id = ?", targetID).First(&scanState).Error; err == nil {
		data.ScanState = &scanState
	}

	var err error
	if data.Counts.TotalAssets, err = countModel(&models.Asset{}, "target_id = ?", targetID); err != nil {
		return nil, err
	}
	if data.Counts.LiveAssets, err = countModel(&models.Asset{}, "target_id = ? AND is_live = true", targetID); err != nil {
		return nil, err
	}
	if data.Counts.DeadAssets, err = countModel(&models.Asset{}, "target_id = ? AND is_live = false", targetID); err != nil {
		return nil, err
	}
	if data.Counts.WebAssets, err = countModel(&models.Asset{}, "target_id = ? AND (status_code > 0 OR final_url <> '')", targetID); err != nil {
		return nil, err
	}
	if data.Counts.CDNAssets, err = countModel(&models.Asset{}, "target_id = ? AND (cdncheck = true OR cdn_name <> '' OR cdncheck_name <> '')", targetID); err != nil {
		return nil, err
	}
	if data.Counts.WAFAssets, err = countModel(&models.Asset{}, "target_id = ? AND (wafcheck = true OR wafcheck_name <> '')", targetID); err != nil {
		return nil, err
	}
	if data.Counts.CloudAssets, err = countModel(&models.Asset{}, "target_id = ? AND (cloudcheck = true OR cloudcheck_name <> '')", targetID); err != nil {
		return nil, err
	}
	if data.Counts.PortAssets, err = countModel(&models.Asset{}, "target_id = ? AND open_ports IS NOT NULL AND open_ports::text <> '{}' AND open_ports::text <> 'null' AND open_ports::text <> ''", targetID); err != nil {
		return nil, err
	}
	if data.Counts.TotalURLs, err = countModel(&models.FoundURL{}, "target_id = ?", targetID); err != nil {
		return nil, err
	}
	if data.Counts.JSURLs, err = countModel(&models.FoundURL{}, "target_id = ? AND (LOWER(value) LIKE '%.js%' OR LOWER(value) LIKE '%.mjs%' OR LOWER(value) LIKE '%.map%')", targetID); err != nil {
		return nil, err
	}
	if data.Counts.TotalFindings, err = countModel(&models.Finding{}, "target_id = ?", targetID); err != nil {
		return nil, err
	}
	if data.Counts.OpenFindings, err = countModel(&models.Finding{}, "target_id = ? AND status = ?", targetID, models.FindingStatusOpen); err != nil {
		return nil, err
	}

	if data.FindingsBySeverity, err = countFindingsBy(targetID, "severity"); err != nil {
		return nil, err
	}
	if data.FindingsByStatus, err = countFindingsBy(targetID, "status"); err != nil {
		return nil, err
	}
	if data.FindingsBySource, err = countFindingsBy(targetID, "source_tool"); err != nil {
		return nil, err
	}
	if data.FindingsByCategory, err = countFindingsBy(targetID, "category"); err != nil {
		return nil, err
	}

	severityOrder := "CASE severity WHEN 'critical' THEN 5 WHEN 'high' THEN 4 WHEN 'medium' THEN 3 WHEN 'low' THEN 2 ELSE 1 END DESC"
	if err := database.DB.Where("target_id = ? AND status <> ?", targetID, models.FindingStatusFixed).
		Order(severityOrder).Order("last_seen DESC").Limit(15).Find(&data.TopFindings).Error; err != nil {
		return nil, fmt.Errorf("load top findings: %w", err)
	}
	if err := database.DB.Where("target_id = ? AND source_tool = ?", targetID, "takeover").
		Order(severityOrder).Order("last_seen DESC").Limit(20).Find(&data.TakeoverFindings).Error; err != nil {
		return nil, fmt.Errorf("load takeover findings: %w", err)
	}
	if err := database.DB.Where("target_id = ? AND source_tool = ?", targetID, "js-intel").
		Order(severityOrder).Order("last_seen DESC").Limit(20).Find(&data.JSFindings).Error; err != nil {
		return nil, fmt.Errorf("load js intelligence findings: %w", err)
	}
	if err := database.DB.Where("target_id = ? AND source_tool = ?", targetID, "nuclei").
		Order(severityOrder).Order("last_seen DESC").Limit(20).Find(&data.NucleiFindings).Error; err != nil {
		return nil, fmt.Errorf("load nuclei findings: %w", err)
	}
	if err := database.DB.Where("target_id = ?", targetID).
		Order("updated_at DESC").Limit(20).Find(&data.RecentAssets).Error; err != nil {
		return nil, fmt.Errorf("load recent assets: %w", err)
	}
	if err := database.DB.Where("target_id = ?", targetID).
		Order("id DESC").Limit(20).Find(&data.RecentURLs).Error; err != nil {
		return nil, fmt.Errorf("load recent urls: %w", err)
	}
	if err := database.DB.Preload("Asset").
		Joins("JOIN assets ON assets.id = asset_histories.asset_id").
		Where("assets.target_id = ?", targetID).
		Order("asset_histories.created_at DESC").Limit(20).Find(&data.AssetChanges).Error; err != nil {
		return nil, fmt.Errorf("load asset changes: %w", err)
	}

	var latestAnalysis models.AIAnalysis
	if err := database.DB.
		Where("target_id = ?", targetID).
		Where("status <> ?", "failed").
		Order("created_at DESC").
		First(&latestAnalysis).Error; err == nil {
		data.LatestAnalysis = &latestAnalysis
	}

	if row := latestAgentRun(targetID, models.AgentRunTypeSummary); row != nil {
		data.LatestSummaryRun = row
	}
	if row := latestAgentRun(targetID, models.AgentRunTypeTriage); row != nil {
		data.LatestTriageRun = row
	}
	if row := latestAgentRun(targetID, models.AgentRunTypeReport); row != nil {
		data.LatestReportRun = row
	}

	return data, nil
}

func countModel(model interface{}, where string, args ...interface{}) (int64, error) {
	var count int64
	query := database.DB.Model(model)
	if strings.TrimSpace(where) != "" {
		query = query.Where(where, args...)
	}
	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

func countFindingsBy(targetID uint, field string) (map[string]int64, error) {
	switch field {
	case "severity", "status", "source_tool", "category":
	default:
		return nil, fmt.Errorf("unsupported finding stat field: %s", field)
	}

	rows := make([]statRow, 0)
	if err := database.DB.Model(&models.Finding{}).
		Select(field+" AS key, COUNT(*) AS count").
		Where("target_id = ?", targetID).
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

func latestAgentRun(targetID uint, agentType string) *models.AgentRun {
	var row models.AgentRun
	if err := database.DB.
		Where("target_id = ? AND agent_type = ? AND status = ?", targetID, agentType, models.AgentRunStatusCompleted).
		Order("created_at DESC").
		First(&row).Error; err != nil {
		return nil
	}
	return &row
}

func Filename(data *ReportData) string {
	domain := strings.TrimSpace(data.Target.RootDomain)
	if domain == "" {
		domain = fmt.Sprintf("target-%d", data.Target.ID)
	}
	domain = filenameUnsafe.ReplaceAllString(strings.ToLower(domain), "-")
	domain = strings.Trim(domain, "-._")
	if domain == "" {
		domain = fmt.Sprintf("target-%d", data.Target.ID)
	}
	return fmt.Sprintf("hunt-target-%d-%s-report-%s.pdf", data.Target.ID, domain, data.GeneratedAt.Format("20060102"))
}

func Render(data *ReportData) ([]byte, error) {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetTitle("Hunt Engine Target Report", false)
	pdf.SetAuthor("Hunt Engine", false)
	pdf.SetSubject("Target reconnaissance and security report", false)
	pdf.SetMargins(14, 14, 14)
	pdf.SetAutoPageBreak(true, 17)
	pdf.SetCompression(true)
	pdf.SetFooterFunc(func() {
		pdf.SetY(-12)
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(110, 120, 135)
		pdf.CellFormat(0, 8, fmt.Sprintf("Hunt Engine Target Report - Page %d", pdf.PageNo()), "", 0, "C", false, 0, "")
	})

	coverPage(pdf, data)
	pdf.AddPage()
	sectionTitle(pdf, "Executive Summary")
	paragraph(pdf, "This report summarizes the current Hunt Engine reconnaissance and security findings for the selected target. It is generated from stored scan data, assets, URLs, findings, structured evidence, and the latest deterministic target analysis when available. Risk is evidence-based; coverage gaps are reported separately and do not inflate severity.")
	metricsGrid(pdf, data)
	analysisSection(pdf, data)
	agentRunsSection(pdf, data)
	scanStateSection(pdf, data)
	assetAndURLSection(pdf, data)
	findingSummarySection(pdf, data)
	findingTables(pdf, data)
	assetTables(pdf, data)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func coverPage(pdf *gofpdf.Fpdf, data *ReportData) {
	pdf.AddPage()
	pdf.SetFillColor(16, 24, 40)
	pdf.Rect(0, 0, 210, 297, "F")

	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 26)
	pdf.SetXY(18, 38)
	pdf.CellFormat(0, 12, "Hunt Engine", "", 1, "L", false, 0, "")
	pdf.SetFont("Helvetica", "B", 20)
	pdf.SetTextColor(73, 222, 128)
	pdf.CellFormat(0, 10, "Target Security Report", "", 1, "L", false, 0, "")

	pdf.SetDrawColor(73, 222, 128)
	pdf.SetLineWidth(0.6)
	pdf.Line(18, 68, 190, 68)

	pdf.SetY(88)
	coverKV(pdf, "Target", data.Target.Name)
	coverKV(pdf, "Root Domain", data.Target.RootDomain)
	coverKV(pdf, "Target ID", fmt.Sprintf("%d", data.Target.ID))
	coverKV(pdf, "Status", data.Target.Status)
	coverKV(pdf, "Current Phase", data.Target.CurrentPhase)
	coverKV(pdf, "Generated At", data.GeneratedAt.Format(time.RFC3339))
	if data.Target.LastScanAt != nil {
		coverKV(pdf, "Last Scan At", data.Target.LastScanAt.UTC().Format(time.RFC3339))
	}

	pdf.SetY(208)
	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(190, 200, 214)
	pdf.MultiCell(170, 6, cleanText("Hunt Engine v3.6.0 target report. The report includes target metadata, scan state, asset and URL inventory, findings, structured evidence summaries, source-specific security sections, and deterministic commercial-grade target analysis when available."), "", "L", false)
}

func coverKV(pdf *gofpdf.Fpdf, label string, value string) {
	pdf.SetX(22)
	pdf.SetFont("Helvetica", "B", 10)
	pdf.SetTextColor(148, 163, 184)
	pdf.CellFormat(40, 8, cleanText(label), "", 0, "L", false, 0, "")
	pdf.SetFont("Helvetica", "", 11)
	pdf.SetTextColor(255, 255, 255)
	pdf.CellFormat(120, 8, cleanText(emptyDash(value)), "", 1, "L", false, 0, "")
}

func metricsGrid(pdf *gofpdf.Fpdf, data *ReportData) {
	ensureSpace(pdf, 54)
	y := pdf.GetY() + 2
	w := 56.5
	h := 28.0
	gap := 4.0
	x := 14.0

	cards := []struct {
		Label string
		Value string
		Hint  string
	}{
		{"Assets", fmt.Sprintf("%d", data.Counts.TotalAssets), fmt.Sprintf("%d live / %d dead", data.Counts.LiveAssets, data.Counts.DeadAssets)},
		{"Web Assets", fmt.Sprintf("%d", data.Counts.WebAssets), fmt.Sprintf("%d CDN / %d WAF", data.Counts.CDNAssets, data.Counts.WAFAssets)},
		{"URLs", fmt.Sprintf("%d", data.Counts.TotalURLs), fmt.Sprintf("%d JS-like resources", data.Counts.JSURLs)},
		{"Findings", fmt.Sprintf("%d", data.Counts.TotalFindings), fmt.Sprintf("%d open", data.Counts.OpenFindings)},
		{"Takeover", fmt.Sprintf("%d", len(data.TakeoverFindings)), "subdomain takeover candidates"},
		{"JS Intel", fmt.Sprintf("%d", len(data.JSFindings)), "JS endpoint/secret/map findings"},
	}

	for i, c := range cards {
		cx := x + float64(i%3)*(w+gap)
		cy := y + float64(i/3)*(h+gap)
		metricCard(pdf, cx, cy, w, h, c.Label, c.Value, c.Hint)
	}
	pdf.SetY(y + 2*h + gap + 8)
}

func metricCard(pdf *gofpdf.Fpdf, x, y, w, h float64, label, value, hint string) {
	pdf.SetFillColor(248, 250, 252)
	pdf.SetDrawColor(210, 220, 230)
	pdf.Rect(x, y, w, h, "FD")
	pdf.SetXY(x+4, y+4)
	pdf.SetFont("Helvetica", "B", 16)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(w-8, 7, cleanText(value), "", 1, "L", false, 0, "")
	pdf.SetX(x + 4)
	pdf.SetFont("Helvetica", "B", 8)
	pdf.SetTextColor(30, 64, 175)
	pdf.CellFormat(w-8, 5, cleanText(strings.ToUpper(label)), "", 1, "L", false, 0, "")
	pdf.SetX(x + 4)
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(100, 116, 139)
	pdf.CellFormat(w-8, 5, cleanText(hint), "", 0, "L", false, 0, "")
}

func analysisSection(pdf *gofpdf.Fpdf, data *ReportData) {
	ensureSpace(pdf, 50)
	sectionTitle(pdf, "Target Analysis")

	if data.LatestAnalysis == nil {
		paragraph(pdf, "No target analysis has been generated yet. The report continues with deterministic inventory, findings, and evidence sections.")
		return
	}

	output := analysisOutputMap(data.LatestAnalysis.OutputJSON)

	summary := analysisString(output, "executive_summary")
	if summary == "" {
		summary = analysisString(output, "summary")
	}
	if summary == "" {
		summary = data.LatestAnalysis.Summary
	}

	meta := fmt.Sprintf(
		"Latest analysis #%d - %s/%s - %s - generated at %s",
		data.LatestAnalysis.ID,
		emptyDash(data.LatestAnalysis.Provider),
		emptyDash(data.LatestAnalysis.Model),
		emptyDash(data.LatestAnalysis.PromptVersion),
		data.LatestAnalysis.CreatedAt.UTC().Format(time.RFC3339),
	)
	paragraph(pdf, meta)
	if summary != "" {
		paragraph(pdf, summary)
	}

	analysisMetricsGrid(pdf, output)

	paragraph(pdf, "Methodology note: this section is deterministic and evidence-based. Coverage gaps are not vulnerabilities. Reconnaissance-only signals do not become critical solely because they are numerous. Manual validation is required before customer-facing severity claims.")

	analysisRiskDrivers(pdf, output)
	analysisBulletList(pdf, "Recommended Next Actions", stringListFromAnalysis(output["next_actions"]), 8)
	analysisBulletList(pdf, "Coverage Gaps", stringListFromAnalysis(output["coverage_gaps"]), 8)
	analysisBulletList(pdf, "Limitations", stringListFromAnalysis(output["limitations"]), 6)
}

func analysisMetricsGrid(pdf *gofpdf.Fpdf, output map[string]interface{}) {
	ensureSpace(pdf, 36)

	y := pdf.GetY() + 1
	w := 56.5
	h := 23.0
	gap := 4.0
	x := 14.0

	cards := []struct {
		Label string
		Value string
		Hint  string
	}{
		{"Risk", analysisValue(output, "risk_score") + "/100", strings.ToUpper(emptyDash(analysisString(output, "risk_level")))},
		{"Confidence", analysisValue(output, "confidence_score") + "/100", "evidence confidence"},
		{"Exposure", analysisValue(output, "exposure_score") + "/100", "attack surface"},
		{"Coverage", analysisValue(output, "coverage_score") + "/100", "scan completeness"},
		{"Finding Quality", analysisValue(output, "finding_quality_score") + "/100", "signal quality"},
		{"Methodology", "v2", ellipsize(analysisString(output, "methodology_version"), 28)},
	}

	for i, c := range cards {
		cx := x + float64(i%3)*(w+gap)
		cy := y + float64(i/3)*(h+gap)
		metricCard(pdf, cx, cy, w, h, c.Label, c.Value, c.Hint)
	}
	pdf.SetY(y + 2*h + gap + 7)
}

func analysisRiskDrivers(pdf *gofpdf.Fpdf, output map[string]interface{}) {
	drivers := interfaceList(output["risk_drivers"])
	if len(drivers) == 0 {
		return
	}

	ensureSpace(pdf, 28)
	analysisSubTitle(pdf, "Risk Drivers")
	tableHeader(pdf, []string{"Driver", "Count", "Impact", "Rationale"}, []float64{48, 18, 24, 88})

	limit := len(drivers)
	if limit > 8 {
		limit = 8
	}

	for _, raw := range drivers[:limit] {
		driver := interfaceMap(raw)
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{
			ellipsize(fmt.Sprint(driver["name"]), 32),
			ellipsize(fmt.Sprint(driver["count"]), 10),
			ellipsize(fmt.Sprint(driver["severity"]), 14),
			ellipsize(fmt.Sprint(driver["rationale"]), 64),
		}, []float64{48, 18, 24, 88})
	}
	pdf.Ln(2)
}

func analysisBulletList(pdf *gofpdf.Fpdf, title string, items []string, limit int) {
	if len(items) == 0 {
		return
	}

	if limit <= 0 || limit > len(items) {
		limit = len(items)
	}

	ensureSpace(pdf, 12+float64(limit)*7)
	analysisSubTitle(pdf, title)

	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(71, 85, 105)

	for _, item := range items[:limit] {
		ensureSpace(pdf, 7)
		pdf.MultiCell(0, 5, cleanText("- "+ellipsize(item, 170)), "", "L", false)
	}
	pdf.Ln(1)
}

func analysisSubTitle(pdf *gofpdf.Fpdf, title string) {
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(30, 64, 175)
	pdf.CellFormat(0, 6, cleanText(title), "", 1, "L", false, 0, "")
}

func analysisOutputMap(raw []byte) map[string]interface{} {
	out := map[string]interface{}{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return out
	}
	_ = json.Unmarshal(trimmed, &out)
	return out
}

func analysisString(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	v, ok := m[key]
	if !ok || v == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(v))
}

func analysisValue(m map[string]interface{}, key string) string {
	v := analysisString(m, key)
	if v == "" {
		return "0"
	}
	return v
}

func interfaceList(value interface{}) []interface{} {
	if value == nil {
		return nil
	}
	if out, ok := value.([]interface{}); ok {
		return out
	}
	return nil
}

func interfaceMap(value interface{}) map[string]interface{} {
	if value == nil {
		return map[string]interface{}{}
	}
	if out, ok := value.(map[string]interface{}); ok {
		return out
	}
	return map[string]interface{}{}
}

func stringListFromAnalysis(value interface{}) []string {
	items := interfaceList(value)
	out := make([]string, 0, len(items))
	for _, item := range items {
		text := strings.TrimSpace(fmt.Sprint(item))
		if text != "" && text != "<nil>" {
			out = append(out, text)
		}
	}
	return out
}

func agentRunsSection(pdf *gofpdf.Fpdf, data *ReportData) {
	if data.LatestSummaryRun == nil && data.LatestTriageRun == nil && data.LatestReportRun == nil {
		return
	}

	ensureSpace(pdf, 50)
	sectionTitle(pdf, "Advisory Agent Outputs")
	paragraph(pdf, "This section summarizes the latest advisory agent outputs. Agent outputs are draft guidance only: they do not override deterministic findings, severity, target risk score, policy enforcement, or human validation requirements. Report drafts must not be submitted automatically.")

	agentRunSummaryBlock(pdf, "Summary Agent", data.LatestSummaryRun)
	agentRunTriageBlock(pdf, "Triage Agent", data.LatestTriageRun)
	agentRunReportBlock(pdf, "Report Agent Draft", data.LatestReportRun)
}

func agentRunMeta(run *models.AgentRun) string {
	if run == nil {
		return ""
	}
	return fmt.Sprintf(
		"Agent run #%d - %s/%s - policy: %s - generated at %s",
		run.ID,
		emptyDash(run.Provider),
		emptyDash(run.Model),
		emptyDash(run.PolicyStatus),
		run.CreatedAt.UTC().Format(time.RFC3339),
	)
}

func agentOutputMap(raw []byte) map[string]interface{} {
	out := map[string]interface{}{}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || !json.Valid(trimmed) {
		return out
	}
	_ = json.Unmarshal(trimmed, &out)
	return out
}

func agentRunSummaryBlock(pdf *gofpdf.Fpdf, title string, run *models.AgentRun) {
	if run == nil {
		return
	}

	output := agentOutputMap(run.OutputJSON)
	ensureSpace(pdf, 28)
	analysisSubTitle(pdf, title)
	paragraph(pdf, agentRunMeta(run))

	summary := analysisString(output, "attack_surface_summary")
	if summary != "" {
		paragraph(pdf, summary)
	}

	coverage := interfaceMap(output["coverage_summary"])
	rows := [][2]string{
		{"Total Assets", fmt.Sprint(coverage["total_assets"])},
		{"Live Assets", fmt.Sprint(coverage["total_live_assets"])},
		{"Findings", fmt.Sprint(coverage["total_findings"])},
		{"URLs", fmt.Sprint(coverage["total_urls"])},
	}
	keyValueTable(pdf, rows)

	if narrative := analysisString(output, "risk_narrative"); narrative != "" {
		paragraph(pdf, "Risk narrative: "+narrative)
	}

	analysisBulletList(pdf, "Summary Agent - What To Test Next", stringListFromAnalysis(output["what_to_test_next"]), 6)
	analysisBulletList(pdf, "Summary Agent - Policy Safety Notes", stringListFromAnalysis(output["policy_safety_notes"]), 5)
}

func agentRunTriageBlock(pdf *gofpdf.Fpdf, title string, run *models.AgentRun) {
	if run == nil {
		return
	}

	output := agentOutputMap(run.OutputJSON)
	ensureSpace(pdf, 28)
	analysisSubTitle(pdf, title)
	paragraph(pdf, agentRunMeta(run))

	if summary := analysisString(output, "summary"); summary != "" {
		paragraph(pdf, summary)
	}

	findings := interfaceList(output["top_interesting_findings"])
	if len(findings) > 0 {
		analysisSubTitle(pdf, "Triage Agent - Top Interesting Findings")
		tableHeader(pdf, []string{"Finding", "Score", "Signal", "Reason"}, []float64{34, 18, 34, 92})

		limit := len(findings)
		if limit > 6 {
			limit = 6
		}

		for _, raw := range findings[:limit] {
			finding := interfaceMap(raw)
			reasons := stringListFromAnalysis(finding["why_interesting"])
			reason := ""
			if len(reasons) > 0 {
				reason = reasons[0]
			}

			ensureSpace(pdf, 8)
			tableRow(pdf, []string{
				ellipsize("#"+fmt.Sprint(finding["finding_id"])+" "+fmt.Sprint(finding["title"]), 30),
				ellipsize(fmt.Sprint(finding["interest_score"]), 10),
				ellipsize(fmt.Sprint(finding["source_tool"])+" / "+fmt.Sprint(finding["category"]), 28),
				ellipsize(reason, 70),
			}, []float64{34, 18, 34, 92})
		}
		pdf.Ln(2)
	}

	analysisBulletList(pdf, "Triage Agent - Manual Validation Steps", stringListFromAnalysis(output["manual_validation_steps"]), 6)
	analysisBulletList(pdf, "Triage Agent - Recommended Manual Tests", agentManualTestList(output["recommended_manual_tests"]), 6)
}

func agentRunReportBlock(pdf *gofpdf.Fpdf, title string, run *models.AgentRun) {
	if run == nil {
		return
	}

	output := agentOutputMap(run.OutputJSON)
	candidate := interfaceMap(output["report_candidate"])

	ensureSpace(pdf, 34)
	analysisSubTitle(pdf, title)
	paragraph(pdf, agentRunMeta(run))

	reportTitle := strings.TrimSpace(fmt.Sprint(candidate["title"]))
	if reportTitle == "" || reportTitle == "<nil>" {
		reportTitle = "Draft report candidate"
	}

	rows := [][2]string{
		{"Draft Title", reportTitle},
		{"Validation Status", fmt.Sprint(candidate["validation_status"])},
		{"Confidence", fmt.Sprint(candidate["confidence"])},
		{"Human Validation Required", fmt.Sprint(candidate["human_validation_required"])},
		{"Do Not Auto-submit", fmt.Sprint(candidate["do_not_submit_automatically"])},
	}
	keyValueTable(pdf, rows)

	if summary := strings.TrimSpace(fmt.Sprint(candidate["summary"])); summary != "" && summary != "<nil>" {
		paragraph(pdf, summary)
	}

	impact := strings.TrimSpace(fmt.Sprint(candidate["impact_hypothesis"]))
	if impact == "" || impact == "<nil>" {
		impact = analysisString(output, "impact_hypothesis")
	}
	if impact != "" {
		paragraph(pdf, "Impact hypothesis: "+impact)
	}

	analysisBulletList(pdf, "Report Agent - Evidence Needed", stringListFromAnalysis(firstNonNil(output["evidence_needed"], candidate["evidence_needed"])), 7)
	analysisBulletList(pdf, "Report Agent - Validation Checklist", stringListFromAnalysis(firstNonNil(output["validation_checklist"], candidate["validation_checklist"])), 7)
	analysisBulletList(pdf, "Report Agent - Platform Safe Wording", stringListFromAnalysis(firstNonNil(output["platform_safe_wording"], candidate["platform_safe_wording"])), 5)
	analysisBulletList(pdf, "Report Agent - Suggested Fix", stringListFromAnalysis(candidate["suggested_fix"]), 5)
}

func agentManualTestList(value interface{}) []string {
	items := interfaceList(value)
	out := make([]string, 0, len(items))

	for _, item := range items {
		m := interfaceMap(item)
		testType := strings.TrimSpace(fmt.Sprint(m["test_type"]))
		why := strings.TrimSpace(fmt.Sprint(m["why"]))
		priority := strings.TrimSpace(fmt.Sprint(m["priority"]))

		parts := []string{}
		if priority != "" && priority != "<nil>" {
			parts = append(parts, "priority="+priority)
		}
		if testType != "" && testType != "<nil>" {
			parts = append(parts, testType)
		}
		if why != "" && why != "<nil>" {
			parts = append(parts, why)
		}

		text := strings.Join(parts, " - ")
		if text != "" {
			out = append(out, text)
		}
	}

	return out
}

func firstNonNil(values ...interface{}) interface{} {
	for _, value := range values {
		if value != nil {
			return value
		}
	}
	return nil
}

func scanStateSection(pdf *gofpdf.Fpdf, data *ReportData) {
	ensureSpace(pdf, 36)
	sectionTitle(pdf, "Scan State")
	rows := [][2]string{
		{"Target Status", data.Target.Status},
		{"Current Phase", data.Target.CurrentPhase},
		{"Scan Count", fmt.Sprintf("%d", data.Target.ScanCount)},
		{"Modules", strings.Join(parseStringList(data.Target.ScanModules), ", ")},
		{"Nuclei Enabled", boolText(data.Target.UseNuclei)},
		{"Nuclei Profile", data.Target.NucleiProfile},
	}
	if data.Target.LastScanAt != nil {
		rows = append(rows, [2]string{"Last Scan", data.Target.LastScanAt.UTC().Format(time.RFC3339)})
	}
	if data.ScanState != nil {
		rows = append(rows,
			[2]string{"Persistent State", data.ScanState.Status},
			[2]string{"Current Step", joinNonEmpty(data.ScanState.CurrentModule, data.ScanState.CurrentStep, ":")},
			[2]string{"Completed Steps", fmt.Sprintf("%d", len(parseStringList(data.ScanState.CompletedSteps)))},
			[2]string{"Last Error", data.ScanState.LastError},
		)
	}
	keyValueTable(pdf, rows)
}

func assetAndURLSection(pdf *gofpdf.Fpdf, data *ReportData) {
	ensureSpace(pdf, 42)
	sectionTitle(pdf, "Asset and URL Inventory")
	drawBars(pdf, "Asset Distribution", []barRow{
		{"Live assets", data.Counts.LiveAssets},
		{"Dead assets", data.Counts.DeadAssets},
		{"Web probed", data.Counts.WebAssets},
		{"CDN tagged", data.Counts.CDNAssets},
		{"WAF tagged", data.Counts.WAFAssets},
		{"Cloud tagged", data.Counts.CloudAssets},
		{"Open ports", data.Counts.PortAssets},
	})
	drawBars(pdf, "URL Distribution", []barRow{
		{"Total URLs", data.Counts.TotalURLs},
		{"JS-like resources", data.Counts.JSURLs},
	})
}

func findingSummarySection(pdf *gofpdf.Fpdf, data *ReportData) {
	ensureSpace(pdf, 55)
	sectionTitle(pdf, "Findings Summary")
	drawBars(pdf, "Severity Breakdown", rowsFromMap(data.FindingsBySeverity, []string{"critical", "high", "medium", "low", "info"}))
	drawBars(pdf, "Finding Sources", rowsFromMap(data.FindingsBySource, []string{"builtin", "nuclei", "takeover", "js-intel"}))
	drawBars(pdf, "Finding Status", rowsFromMap(data.FindingsByStatus, []string{"open", "accepted", "false_positive", "fixed"}))
}

func findingTables(pdf *gofpdf.Fpdf, data *ReportData) {
	findingsTable(pdf, "Top Active Findings", data.TopFindings)
	takeoverTable(pdf, data.TakeoverFindings)
	jsIntelTable(pdf, data.JSFindings)
	nucleiTable(pdf, data.NucleiFindings)
}

func assetTables(pdf *gofpdf.Fpdf, data *ReportData) {
	recentAssetsTable(pdf, data.RecentAssets)
	recentURLsTable(pdf, data.RecentURLs)
	assetChangesTable(pdf, data.AssetChanges)
}

func findingsTable(pdf *gofpdf.Fpdf, title string, findings []models.Finding) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, title)
	if len(findings) == 0 {
		paragraph(pdf, "No findings were available for this section.")
		return
	}
	tableHeader(pdf, []string{"Severity", "Source", "Category", "Title"}, []float64{22, 26, 38, 92})
	for _, f := range findings {
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{f.Severity, f.SourceTool, f.Category, ellipsize(f.Title, 72)}, []float64{22, 26, 38, 92})
	}
	pdf.Ln(2)
}

func takeoverTable(pdf *gofpdf.Fpdf, findings []models.Finding) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, "Takeover Detection")
	if len(findings) == 0 {
		paragraph(pdf, "No takeover findings were available.")
		return
	}
	tableHeader(pdf, []string{"Asset", "Provider", "Confidence", "CNAME"}, []float64{50, 36, 24, 68})
	for _, f := range findings {
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{
			ellipsize(evidenceValue(f, "asset"), 34),
			ellipsize(evidenceValue(f, "provider"), 25),
			ellipsize(evidenceValue(f, "confidence"), 18),
			ellipsize(evidenceValue(f, "cname"), 48),
		}, []float64{50, 36, 24, 68})
	}
	pdf.Ln(2)
}

func jsIntelTable(pdf *gofpdf.Fpdf, findings []models.Finding) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, "JavaScript Intelligence")
	if len(findings) == 0 {
		paragraph(pdf, "No JavaScript intelligence findings were available.")
		return
	}
	tableHeader(pdf, []string{"Severity", "Type", "Count", "JS URL"}, []float64{22, 42, 18, 96})
	for _, f := range findings {
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{
			f.Severity,
			ellipsize(evidenceValue(f, "signal_type"), 28),
			ellipsize(evidenceValue(f, "endpoints_count"), 8),
			ellipsize(evidenceValue(f, "js_url"), 70),
		}, []float64{22, 42, 18, 96})
	}
	pdf.Ln(2)
}

func nucleiTable(pdf *gofpdf.Fpdf, findings []models.Finding) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, "Nuclei Findings")
	if len(findings) == 0 {
		paragraph(pdf, "No Nuclei findings were available.")
		return
	}
	tableHeader(pdf, []string{"Severity", "Template", "Host", "Matched At"}, []float64{22, 54, 42, 60})
	for _, f := range findings {
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{
			f.Severity,
			ellipsize(evidenceValue(f, "template_id"), 38),
			ellipsize(evidenceValue(f, "host"), 28),
			ellipsize(evidenceValue(f, "matched_at"), 44),
		}, []float64{22, 54, 42, 60})
	}
	pdf.Ln(2)
}

func recentAssetsTable(pdf *gofpdf.Fpdf, assets []models.Asset) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, "Recent Assets")
	if len(assets) == 0 {
		paragraph(pdf, "No assets were available.")
		return
	}
	tableHeader(pdf, []string{"Asset", "Live", "Status", "Title"}, []float64{58, 18, 20, 82})
	for _, a := range assets {
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{
			ellipsize(a.Value, 42),
			boolText(a.IsLive),
			fmt.Sprintf("%d", a.StatusCode),
			ellipsize(a.Title, 58),
		}, []float64{58, 18, 20, 82})
	}
	pdf.Ln(2)
}

func recentURLsTable(pdf *gofpdf.Fpdf, urls []models.FoundURL) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, "Recent URLs")
	if len(urls) == 0 {
		paragraph(pdf, "No URLs were available.")
		return
	}
	tableHeader(pdf, []string{"Source", "URL"}, []float64{32, 146})
	for _, u := range urls {
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{ellipsize(u.Source, 22), ellipsize(u.Value, 110)}, []float64{32, 146})
	}
	pdf.Ln(2)
}

func assetChangesTable(pdf *gofpdf.Fpdf, changes []models.AssetHistory) {
	ensureSpace(pdf, 24)
	sectionTitle(pdf, "Recent Asset Changes")
	if len(changes) == 0 {
		paragraph(pdf, "No recent asset changes were available.")
		return
	}
	tableHeader(pdf, []string{"Asset", "Field", "Old", "New"}, []float64{45, 28, 52, 53})
	for _, ch := range changes {
		assetValue := ""
		if ch.Asset != nil {
			assetValue = ch.Asset.Value
		}
		ensureSpace(pdf, 8)
		tableRow(pdf, []string{
			ellipsize(assetValue, 32),
			ellipsize(ch.FieldName, 18),
			ellipsize(ch.OldValue, 38),
			ellipsize(ch.NewValue, 38),
		}, []float64{45, 28, 52, 53})
	}
	pdf.Ln(2)
}

func sectionTitle(pdf *gofpdf.Fpdf, title string) {
	ensureSpace(pdf, 12)
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetTextColor(15, 23, 42)
	pdf.CellFormat(0, 8, cleanText(title), "", 1, "L", false, 0, "")
	pdf.SetDrawColor(73, 222, 128)
	pdf.SetLineWidth(0.4)
	y := pdf.GetY()
	pdf.Line(14, y, 196, y)
	pdf.Ln(3)
}

func paragraph(pdf *gofpdf.Fpdf, text string) {
	pdf.SetFont("Helvetica", "", 9)
	pdf.SetTextColor(71, 85, 105)
	pdf.MultiCell(0, 5, cleanText(text), "", "L", false)
	pdf.Ln(2)
}

func keyValueTable(pdf *gofpdf.Fpdf, rows [][2]string) {
	for _, row := range rows {
		ensureSpace(pdf, 7)
		pdf.SetFont("Helvetica", "B", 8)
		pdf.SetTextColor(51, 65, 85)
		pdf.SetFillColor(241, 245, 249)
		pdf.CellFormat(45, 6, cleanText(row[0]), "1", 0, "L", true, 0, "")
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(15, 23, 42)
		pdf.CellFormat(133, 6, cleanText(ellipsize(emptyDash(row[1]), 110)), "1", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
}

func drawBars(pdf *gofpdf.Fpdf, title string, rows []barRow) {
	filtered := make([]barRow, 0, len(rows))
	for _, row := range rows {
		if row.Value > 0 {
			filtered = append(filtered, row)
		}
	}
	if len(filtered) == 0 {
		return
	}
	ensureSpace(pdf, 12+float64(len(filtered))*8)
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(30, 64, 175)
	pdf.CellFormat(0, 6, cleanText(title), "", 1, "L", false, 0, "")

	max := int64(1)
	for _, row := range filtered {
		if row.Value > max {
			max = row.Value
		}
	}

	for _, row := range filtered {
		ensureSpace(pdf, 7)
		y := pdf.GetY() + 1
		pdf.SetFont("Helvetica", "", 8)
		pdf.SetTextColor(51, 65, 85)
		pdf.CellFormat(46, 5, cleanText(ellipsize(row.Label, 28)), "", 0, "L", false, 0, "")
		barX := pdf.GetX()
		barMaxW := 92.0
		barW := barMaxW * float64(row.Value) / float64(max)
		if barW < 1.5 {
			barW = 1.5
		}
		pdf.SetFillColor(96, 165, 250)
		pdf.Rect(barX, y, barW, 4.2, "F")
		pdf.SetDrawColor(203, 213, 225)
		pdf.Rect(barX, y, barMaxW, 4.2, "D")
		pdf.SetX(barX + barMaxW + 4)
		pdf.CellFormat(20, 5, fmt.Sprintf("%d", row.Value), "", 1, "L", false, 0, "")
	}
	pdf.Ln(3)
}

func tableHeader(pdf *gofpdf.Fpdf, headers []string, widths []float64) {
	ensureSpace(pdf, 8)
	pdf.SetFont("Helvetica", "B", 7)
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFillColor(30, 41, 59)
	for i, header := range headers {
		pdf.CellFormat(widths[i], 6, cleanText(header), "1", 0, "L", true, 0, "")
	}
	pdf.Ln(-1)
}

func tableRow(pdf *gofpdf.Fpdf, values []string, widths []float64) {
	pdf.SetFont("Helvetica", "", 7)
	pdf.SetTextColor(15, 23, 42)
	for i, value := range values {
		pdf.CellFormat(widths[i], 6, cleanText(emptyDash(value)), "1", 0, "L", false, 0, "")
	}
	pdf.Ln(-1)
}

func rowsFromMap(values map[string]int64, preferred []string) []barRow {
	seen := make(map[string]bool)
	rows := make([]barRow, 0, len(values))
	for _, key := range preferred {
		if values[key] > 0 {
			rows = append(rows, barRow{Label: key, Value: values[key]})
			seen[key] = true
		}
	}
	extra := make([]string, 0)
	for key, value := range values {
		if value > 0 && !seen[key] {
			extra = append(extra, key)
		}
	}
	sort.Strings(extra)
	for _, key := range extra {
		rows = append(rows, barRow{Label: key, Value: values[key]})
	}
	return rows
}

func ensureSpace(pdf *gofpdf.Fpdf, height float64) {
	if pdf.GetY()+height > 280 {
		pdf.AddPage()
	}
}

func parseStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var items []string
	if err := json.Unmarshal([]byte(raw), &items); err == nil {
		return items
	}
	return nil
}

func evidenceValue(f models.Finding, key string) string {
	var obj map[string]interface{}
	if len(f.EvidenceJSON) == 0 || json.Unmarshal([]byte(f.EvidenceJSON), &obj) != nil {
		return ""
	}
	value, ok := obj[key]
	if !ok || value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case []interface{}, map[string]interface{}:
		b, _ := json.Marshal(v)
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func boolText(v bool) string {
	if v {
		return "yes"
	}
	return "no"
}

func emptyDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func joinNonEmpty(a, b, sep string) string {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + sep + b
}

func ellipsize(value string, max int) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\n", " "))
	if max <= 3 || len(value) <= max {
		return value
	}
	return value[:max-3] + "..."
}

func cleanText(value string) string {
	value = strings.ReplaceAll(value, "\r", " ")
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\t", " ")
	return strings.Map(func(r rune) rune {
		if r < 32 {
			return -1
		}
		if r > 126 {
			return '?'
		}
		return r
	}, value)
}
