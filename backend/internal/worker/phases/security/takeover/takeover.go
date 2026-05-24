package takeover

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

const (
	SourceToolTakeover    = "takeover"
	StepTakeoverDetection = "TAKEOVER_DETECTION"

	moduleProbing = "PROBING"
	category      = "subdomain-takeover"
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

type providerSignature struct {
	Provider      string
	Severity      string
	CNAMEContains []string
	TitleContains []string
	StatusCodes   []int
}

type candidate struct {
	AssetID    uint
	Host       string
	FinalURL   string
	StatusCode int
	Title      string
	CNAME      string
	Provider   string
	Severity   string
	Confidence string
	MatchedBy  []string
}

var providerSignatures = []providerSignature{
	{
		Provider:      "AWS S3",
		Severity:      models.FindingSeverityHigh,
		CNAMEContains: []string{".s3.amazonaws.com", ".s3-website", ".s3-website-"},
		TitleContains: []string{"nosuchbucket", "the specified bucket does not exist"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "GitHub Pages",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".github.io"},
		TitleContains: []string{"there isn't a github pages site here", "github pages"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Heroku",
		Severity:      models.FindingSeverityHigh,
		CNAMEContains: []string{".herokuapp.com"},
		TitleContains: []string{"no such app"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Azure App Service",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".azurewebsites.net"},
		TitleContains: []string{"404 web site not found"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Azure Traffic Manager",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".trafficmanager.net"},
		TitleContains: []string{"404 web site not found", "not found"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Azure Front Door",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".azurefd.net", ".azureedge.net", ".tm-azurefd.net"},
		TitleContains: []string{"404 not found", "page not found", "not found"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Netlify",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".netlify.app"},
		TitleContains: []string{"not found - request id", "netlify"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Vercel",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".vercel.app"},
		TitleContains: []string{"deployment_not_found", "the deployment could not be found"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Fastly",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".fastly.net"},
		TitleContains: []string{"fastly error: unknown domain"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "Pantheon",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".pantheonsite.io"},
		TitleContains: []string{"the gods are wise"},
		StatusCodes:   []int{404},
	},
	{
		Provider:      "ReadMe",
		Severity:      models.FindingSeverityMedium,
		CNAMEContains: []string{".readme.io"},
		TitleContains: []string{"project doesn't exist", "project doesnt exist"},
		StatusCodes:   []int{404},
	},
}

func Run(ctx Context) error {
	if ctx.TargetID == 0 {
		return nil
	}

	if ctx.CheckStop != nil && ctx.CheckStop(ctx.TargetID) {
		return fmt.Errorf("process killed by user request")
	}

	if ctx.ScanIsStepDone != nil && ctx.ScanIsStepDone(ctx.TargetID, moduleProbing, StepTakeoverDetection) {
		log.Printf("⏩ Resume: skipping takeover detection for target %d\n", ctx.TargetID)
		return nil
	}

	if ctx.UpdateTargetPhase != nil {
		ctx.UpdateTargetPhase(ctx.TargetID, "PHASE 2: TAKEOVER DETECTION")
	}

	if ctx.ScanMarkRunning != nil {
		ctx.ScanMarkRunning(ctx.TargetID, moduleProbing, StepTakeoverDetection)
	}

	assets, err := loadAssets(ctx.TargetID)
	if err != nil {
		return err
	}

	now := time.Now()
	seen := make(map[string]struct{})

	for _, asset := range assets {
		if ctx.CheckStop != nil && ctx.CheckStop(ctx.TargetID) {
			return fmt.Errorf("process killed by user request")
		}

		cand, ok := analyzeAsset(asset)
		if !ok {
			continue
		}

		fingerprint := fingerprintForCandidate(ctx.TargetID, cand)
		if err := upsertFinding(ctx.TargetID, fingerprint, cand, now); err != nil {
			log.Printf("⚠️ Failed to upsert takeover finding for target %d asset %d: %v\n", ctx.TargetID, asset.ID, err)
			continue
		}

		seen[fingerprint] = struct{}{}
	}

	if err := markMissingFindingsFixed(ctx.TargetID, seen, now); err != nil {
		return err
	}

	if ctx.ScanMarkStepDone != nil {
		ctx.ScanMarkStepDone(ctx.TargetID, moduleProbing, StepTakeoverDetection)
	}

	log.Printf("✅ Takeover detection complete for target %d: %d active findings\n", ctx.TargetID, len(seen))
	return nil
}

func loadAssets(targetID uint) ([]models.Asset, error) {
	var assets []models.Asset

	query := database.DB.
		Where("target_id = ? AND is_live = ?", targetID, true).
		Order("id")

	if max := maxAssets(); max > 0 {
		query = query.Limit(max)
	}

	if err := query.Find(&assets).Error; err != nil {
		return nil, fmt.Errorf("load assets for takeover detection: %w", err)
	}

	return assets, nil
}

func maxAssets() int {
	raw := strings.TrimSpace(os.Getenv("TAKEOVER_MAX_ASSETS"))
	if raw == "" {
		return 0
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		return 0
	}

	return value
}

func analyzeAsset(asset models.Asset) (candidate, bool) {
	host := normalizeHost(asset.Value)
	if host == "" {
		return candidate{}, false
	}

	cname := lookupCNAME(host)
	if cname == "" || cname == host {
		return candidate{}, false
	}

	text := strings.ToLower(strings.Join([]string{
		asset.Title,
		asset.FinalURL,
		asset.WebServer,
		asset.RawHttpx,
	}, " "))

	for _, sig := range providerSignatures {
		cnameMatch := containsAny(cname, sig.CNAMEContains)
		titleMatch := containsAny(text, sig.TitleContains)
		statusMatch := statusIn(asset.StatusCode, sig.StatusCodes)

		// Avoid generic 404/title false positives. A takeover candidate must first
		// point at a known third-party provider via CNAME, then show at least one
		// provider-compatible response signal.
		if !cnameMatch {
			continue
		}

		if !titleMatch && !statusMatch {
			continue
		}

		matchedBy := make([]string, 0, 3)
		if cnameMatch {
			matchedBy = append(matchedBy, "cname")
		}
		if titleMatch {
			matchedBy = append(matchedBy, "title")
		}
		if statusMatch {
			matchedBy = append(matchedBy, "status_code")
		}

		confidence := "medium"
		if cnameMatch && titleMatch && statusMatch {
			confidence = "high"
		}

		return candidate{
			AssetID:    asset.ID,
			Host:       host,
			FinalURL:   asset.FinalURL,
			StatusCode: asset.StatusCode,
			Title:      asset.Title,
			CNAME:      cname,
			Provider:   sig.Provider,
			Severity:   sig.Severity,
			Confidence: confidence,
			MatchedBy:  matchedBy,
		}, true
	}

	return candidate{}, false
}

func normalizeHost(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return ""
	}

	if parsed, err := url.Parse(value); err == nil && parsed.Hostname() != "" {
		value = parsed.Hostname()
	}

	value = strings.TrimSuffix(value, ".")
	if value == "" || strings.Contains(value, "/") {
		return ""
	}

	return value
}

func lookupCNAME(host string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()

	cname, err := net.DefaultResolver.LookupCNAME(ctx, host)
	if err != nil {
		return ""
	}

	cname = strings.TrimSpace(strings.ToLower(cname))
	cname = strings.TrimSuffix(cname, ".")

	return cname
}

func containsAny(value string, needles []string) bool {
	value = strings.ToLower(value)

	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle == "" {
			continue
		}

		if strings.Contains(value, needle) {
			return true
		}
	}

	return false
}

func statusIn(status int, statuses []int) bool {
	for _, candidate := range statuses {
		if status == candidate {
			return true
		}
	}

	return false
}

func upsertFinding(targetID uint, fingerprint string, cand candidate, now time.Time) error {
	assetID := cand.AssetID

	evidenceBytes := evidenceJSON(cand)
	evidence := string(evidenceBytes)

	title := fmt.Sprintf("Potential subdomain takeover: %s", cand.Provider)

	description := "The asset points to a third-party hosting provider and shows signals that may indicate an unclaimed or dangling service. Manual verification is required before treating this as exploitable."

	recommendation := "Verify ownership of the referenced third-party service. If it is unclaimed, either claim the service immediately or remove the DNS record. Confirm provider-specific takeover conditions before reporting externally."

	var existing models.Finding

	err := database.DB.
		Where("target_id = ? AND fingerprint = ?", targetID, fingerprint).
		First(&existing).Error

	updates := map[string]interface{}{
		"title":          title,
		"description":    description,
		"severity":       cand.Severity,
		"category":       category,
		"source_tool":    SourceToolTakeover,
		"evidence":       evidence,
		"evidence_json":  datatypes.JSON(evidenceBytes),
		"recommendation": recommendation,
		"last_seen":      now,
		"asset_id":       &assetID,
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
		AssetID:        &assetID,
		Title:          title,
		Description:    description,
		Severity:       cand.Severity,
		Category:       category,
		SourceTool:     SourceToolTakeover,
		Evidence:       evidence,
		EvidenceJSON:   datatypes.JSON(evidenceBytes),
		Recommendation: recommendation,
		Status:         models.FindingStatusOpen,
		Fingerprint:    fingerprint,
		FirstSeen:      now,
		LastSeen:       now,
	}

	return database.DB.Create(&finding).Error
}

func evidenceJSON(cand candidate) []byte {
	payload := map[string]interface{}{
		"asset":       cand.Host,
		"final_url":   cand.FinalURL,
		"status_code": cand.StatusCode,
		"title":       cand.Title,
		"cname":       cand.CNAME,
		"provider":    cand.Provider,
		"confidence":  cand.Confidence,
		"matched_by":  cand.MatchedBy,
		"note":        "Potential dangling third-party service. Manual verification is required before confirming takeover impact.",
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return []byte("{}")
	}

	return data
}

func fingerprintForCandidate(targetID uint, cand candidate) string {
	h := sha1.New()
	_, _ = h.Write([]byte(fmt.Sprintf("takeover:v1:%d:%s:%s", targetID, cand.Provider, cand.Host)))
	return hex.EncodeToString(h.Sum(nil))
}

func markMissingFindingsFixed(targetID uint, seen map[string]struct{}, now time.Time) error {
	var existing []models.Finding

	if err := database.DB.
		Where("target_id = ? AND source_tool = ? AND category = ? AND status <> ?", targetID, SourceToolTakeover, category, models.FindingStatusFixed).
		Find(&existing).Error; err != nil {
		return fmt.Errorf("load existing takeover findings: %w", err)
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
			return fmt.Errorf("mark stale takeover finding fixed: %w", err)
		}
	}

	return nil
}
