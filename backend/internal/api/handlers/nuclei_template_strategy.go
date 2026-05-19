package handlers

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/omidxplimbo/hunt-engine/backend/internal/api/dto"
	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"github.com/omidxplimbo/hunt-engine/backend/internal/worker/phases/security/nuclei"
)

const maxNucleiStrategyAssets = 250

// GetNucleiTargetTemplateStrategy returns a safe, agent-ready plan for Nuclei profile/template selection.
// It does not save or execute any generated template. Draft generation remains gated by NUCLEI_ALLOW_AI_TEMPLATES.
func GetNucleiTargetTemplateStrategy(c *fiber.Ctx) error {
	targetID, err := strconv.ParseUint(c.Params("id"), 10, 64)
	if err != nil || targetID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "Invalid target id"})
	}

	var target models.Target
	if err := database.DB.First(&target, uint(targetID)).Error; err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"status": "error", "message": "Target not found"})
	}

	var assets []models.Asset
	if err := database.DB.Where("target_id = ? AND is_live = ?", target.ID, true).
		Order("updated_at DESC").
		Limit(maxNucleiStrategyAssets).
		Find(&assets).Error; err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "Failed to load target assets", "error": err.Error()})
	}

	cfg := nuclei.LoadConfig()
	strategy := buildNucleiTargetTemplateStrategy(target, assets, cfg.AllowAITemplates)

	includeDraft := parseBoolQuery(c.Query("include_draft"))
	validateDraft := parseBoolQuery(c.Query("validate"))
	if includeDraft && cfg.AllowAITemplates && strategy.SuggestedDraftRequest != nil {
		content, name, err := buildNucleiTemplateDraft(strategy.SuggestedDraftRequest)
		if err != nil {
			strategy.DraftError = err.Error()
		} else {
			generated := dto.NucleiTemplateDraftResponse{
				Name:                name,
				Content:             content,
				DraftOnly:           true,
				RequiresHumanReview: true,
				Saved:               false,
			}
			if validateDraft {
				validation := runNucleiTemplateValidation(name, content)
				generated.Validation = &validation
			}
			strategy.GeneratedDraft = &generated
		}
	}

	return c.JSON(fiber.Map{"status": "success", "data": strategy})
}

func buildNucleiTargetTemplateStrategy(target models.Target, assets []models.Asset, aiDraftsEnabled bool) dto.NucleiTargetTemplateStrategyResponse {
	context := collectNucleiTargetStrategyContext(target, assets)

	tags := orderedSet{"exposure": {}, "panel": {}}
	placements := orderedSet{"shared": {}, "safe": {}}
	templateSets := orderedSet{"shared": {}, "safe": {}}
	signals := make([]dto.NucleiTemplateStrategySignal, 0, 8)
	rationale := make([]string, 0, 8)

	profile := nuclei.NormalizeProfile(target.NucleiProfile)
	if profile == "" {
		profile = "fast"
	}

	if len(assets) == 0 {
		profile = "safe"
		tags.add("exposure")
		rationale = append(rationale, "No live assets were available yet, so the strategy stays conservative and avoids aggressive template groups.")
	} else {
		rationale = append(rationale, fmt.Sprintf("Analyzed %d live asset(s) for Nuclei template selection.", len(assets)))
	}

	if context.hasPanelSignal {
		tags.add("panel", "default-login", "exposure")
		placements.add("fast", "exposure")
		templateSets.add("fast", "exposure")
		signals = append(signals, dto.NucleiTemplateStrategySignal{Kind: "surface", Value: "panel/login", Confidence: "medium", Reason: "Titles or URLs suggest an exposed login/admin/panel surface."})
		rationale = append(rationale, "Panel/login indicators were found, so fast exposure/default-login templates should be prioritized.")
		if profile == "safe" {
			profile = "fast"
		}
	}

	if len(context.cmsTechnologies) > 0 {
		tags.add("misconfig", "exposure", "cms")
		placements.add("balanced", "misconfig")
		templateSets.add("balanced", "misconfig")
		signals = append(signals, dto.NucleiTemplateStrategySignal{Kind: "technology", Value: strings.Join(context.cmsTechnologies, ","), Confidence: "medium", Reason: "CMS/framework technologies were detected on live assets."})
		rationale = append(rationale, "Detected CMS/framework technologies, so balanced misconfiguration templates are useful.")
		if profile == "safe" || profile == "fast" {
			profile = "balanced"
		}
	}

	if context.hasInterestingPorts {
		tags.add("exposure", "network", "misconfig")
		placements.add("balanced", "misconfig")
		templateSets.add("balanced", "misconfig")
		signals = append(signals, dto.NucleiTemplateStrategySignal{Kind: "port", Value: intSliceToCSV(context.openPorts), Confidence: "medium", Reason: "Interesting web/admin ports were observed in probing or port-scan data."})
		rationale = append(rationale, "Interesting ports were observed, so balanced exposure/misconfiguration coverage is recommended.")
	}

	if context.hasSensitiveTech {
		tags.add("cve", "exposure", "misconfig")
		placements.add("cves", "cves-light")
		templateSets.add("cves-light")
		signals = append(signals, dto.NucleiTemplateStrategySignal{Kind: "technology", Value: strings.Join(context.sensitiveTechnologies, ","), Confidence: "low", Reason: "A potentially sensitive or versioned technology was detected."})
		rationale = append(rationale, "Potentially sensitive technologies were detected. Keep CVE checks light unless the operator approves a deeper profile.")
	}

	if profile == "full" || profile == "custom" {
		placements.add("full", "custom")
		templateSets.add("full", "custom")
		tags.add("cve", "misconfig")
		rationale = append(rationale, "Target is already configured for a broad Nuclei profile, so full/custom template groups are included in the plan.")
	}

	sampleURLs := sortedStrings(context.sampleURLs.values())
	technologies := sortedStrings(context.technologies.values())
	webServers := sortedStrings(context.webServers.values())
	openPorts := sortedInts(context.openPorts)

	var draftReq *dto.GenerateNucleiTemplateDraftRequest
	if len(assets) > 0 {
		draftReq = buildSuggestedNucleiDraftRequest(target, context, sortedStrings(tags.values()), sortedStrings(placements.values()))
	}

	return dto.NucleiTargetTemplateStrategyResponse{
		AgentReady:              true,
		DraftOnly:               true,
		AITemplateDraftsEnabled: aiDraftsEnabled,
		SaveAutomatically:       false,
		ExecuteAutomatically:    false,
		Target: dto.NucleiTemplateStrategyTargetSummary{
			ID:             target.ID,
			Name:           target.Name,
			RootDomain:     target.RootDomain,
			NucleiProfile:  target.NucleiProfile,
			UseNuclei:      target.UseNuclei,
			LiveAssetCount: len(assets),
			SampleURLs:     sampleURLs,
			Technologies:   technologies,
			WebServers:     webServers,
			OpenPorts:      openPorts,
		},
		AllowedActions: dto.NucleiTemplateStrategyAllowedActions{
			CanSelectProfile:       true,
			CanSelectBuiltInTags:   true,
			CanSelectCustomGroups:  true,
			CanGenerateDraft:       aiDraftsEnabled,
			CanSaveTemplate:        true,
			CanAutoSaveTemplate:    false,
			CanAutoExecuteTemplate: false,
			RequiresHumanApproval:  true,
		},
		RecommendedProfile:      profile,
		RecommendedTags:         sortedStrings(tags.values()),
		RecommendedPlacements:   sortedStrings(placements.values()),
		RecommendedTemplateSets: sortedStrings(templateSets.values()),
		Signals:                 signals,
		Rationale:               rationale,
		SuggestedDraftRequest:   draftReq,
	}
}

type nucleiStrategyContext struct {
	sampleURLs            orderedSet
	technologies          orderedSet
	webServers            orderedSet
	openPorts             []int
	cmsTechnologies       []string
	sensitiveTechnologies []string
	hasPanelSignal        bool
	hasInterestingPorts   bool
	hasSensitiveTech      bool
	bestMatcherValue      string
	bestMatcherPart       string
}

func collectNucleiTargetStrategyContext(target models.Target, assets []models.Asset) nucleiStrategyContext {
	ctx := nucleiStrategyContext{
		sampleURLs:       orderedSet{},
		technologies:     orderedSet{},
		webServers:       orderedSet{},
		bestMatcherValue: strings.TrimSpace(target.RootDomain),
		bestMatcherPart:  "body",
	}
	portSet := map[int]struct{}{}
	cmsSet := orderedSet{}
	sensitiveSet := orderedSet{}

	for _, asset := range assets {
		url := strings.TrimSpace(asset.FinalURL)
		if url == "" {
			url = strings.TrimSpace(asset.Value)
		}
		if url != "" && len(ctx.sampleURLs) < 20 {
			ctx.sampleURLs.add(url)
		}

		text := strings.ToLower(strings.Join([]string{asset.Value, asset.FinalURL, asset.Title, asset.WebServer, asset.Technologies}, " "))
		if containsAny(text, "admin", "login", "signin", "dashboard", "panel", "console", "grafana", "kibana", "jenkins", "swagger") {
			ctx.hasPanelSignal = true
			if strings.TrimSpace(asset.Title) != "" {
				ctx.bestMatcherValue = strings.TrimSpace(asset.Title)
				ctx.bestMatcherPart = "body"
			}
		}

		for _, tech := range parseJSONStringList(asset.Technologies) {
			tech = strings.TrimSpace(tech)
			if tech == "" {
				continue
			}
			ctx.technologies.add(tech)
			techLower := strings.ToLower(tech)
			if containsAny(techLower, "wordpress", "drupal", "joomla", "laravel", "django", "rails", "spring", "struts", "phpmyadmin") {
				cmsSet.add(tech)
			}
			if containsAny(techLower, "jenkins", "jira", "confluence", "grafana", "kibana", "elasticsearch", "solr", "tomcat", "swagger", "phpmyadmin") {
				sensitiveSet.add(tech)
			}
		}

		if strings.TrimSpace(asset.WebServer) != "" {
			ctx.webServers.add(asset.WebServer)
			if ctx.bestMatcherValue == "" {
				ctx.bestMatcherValue = asset.WebServer
				ctx.bestMatcherPart = "header"
			}
		}

		for _, port := range parseOpenPorts(asset.OpenPorts) {
			portSet[port] = struct{}{}
			if isInterestingWebPort(port) {
				ctx.hasInterestingPorts = true
			}
		}
	}

	ctx.cmsTechnologies = sortedStrings(cmsSet.values())
	ctx.sensitiveTechnologies = sortedStrings(sensitiveSet.values())
	ctx.hasSensitiveTech = len(ctx.sensitiveTechnologies) > 0
	ctx.openPorts = make([]int, 0, len(portSet))
	for port := range portSet {
		ctx.openPorts = append(ctx.openPorts, port)
	}
	sort.Ints(ctx.openPorts)
	return ctx
}

func buildSuggestedNucleiDraftRequest(target models.Target, context nucleiStrategyContext, tags []string, placements []string) *dto.GenerateNucleiTemplateDraftRequest {
	nameRoot := safeTemplateSlug(target.RootDomain)
	if nameRoot == "" {
		nameRoot = fmt.Sprintf("target-%d", target.ID)
	}

	matcherValue := strings.TrimSpace(context.bestMatcherValue)
	if matcherValue == "" {
		matcherValue = strings.TrimSpace(target.RootDomain)
	}
	if matcherValue == "" {
		matcherValue = "HUNT_TARGET_MARKER"
	}

	severity := "info"
	if containsAny(strings.Join(tags, " "), "default-login", "misconfig", "cve") {
		severity = "low"
	}

	return &dto.GenerateNucleiTemplateDraftRequest{
		Name:         "hunt-ai-" + nameRoot + "-candidate.yaml",
		Title:        "Hunt AI Candidate for " + target.RootDomain,
		Description:  "Draft-only target-specific template candidate. Review, validate, and approve before use.",
		Severity:     severity,
		Tags:         tags,
		Method:       "GET",
		Path:         "/",
		MatcherType:  "word",
		MatcherPart:  context.bestMatcherPart,
		MatcherValue: matcherValue,
		Validate:     false,
	}
}

func parseJSONStringList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err == nil {
		return values
	}
	return strings.Split(strings.Trim(raw, "[]"), ",")
}

func parseOpenPorts(raw string) []int {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "[]" || raw == "null" {
		return nil
	}
	ports := orderedIntSet{}
	var direct []int
	if err := json.Unmarshal([]byte(raw), &direct); err == nil {
		ports.add(direct...)
		return ports.values()
	}
	var byHost map[string][]int
	if err := json.Unmarshal([]byte(raw), &byHost); err == nil {
		for _, hostPorts := range byHost {
			ports.add(hostPorts...)
		}
		return ports.values()
	}
	var generic map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &generic); err == nil {
		for _, value := range generic {
			switch typed := value.(type) {
			case []interface{}:
				for _, item := range typed {
					if number, ok := item.(float64); ok {
						ports.add(int(number))
					}
				}
			case float64:
				ports.add(int(typed))
			}
		}
	}
	return ports.values()
}

func safeTemplateSlug(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteRune('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

func containsAny(value string, needles ...string) bool {
	value = strings.ToLower(value)
	for _, needle := range needles {
		if strings.Contains(value, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func isInterestingWebPort(port int) bool {
	switch port {
	case 80, 81, 82, 88, 443, 8000, 8008, 8080, 8081, 8088, 8443, 8888, 9000, 9090, 9200, 5601, 3000, 5000:
		return true
	default:
		return false
	}
}

func intSliceToCSV(values []int) string {
	parts := make([]string, 0, len(values))
	for _, value := range values {
		parts = append(parts, strconv.Itoa(value))
	}
	return strings.Join(parts, ",")
}

func sortedStrings(values []string) []string {
	values = append([]string(nil), values...)
	sort.Strings(values)
	return values
}

func sortedInts(values []int) []int {
	values = append([]int(nil), values...)
	sort.Ints(values)
	return values
}

func parseBoolQuery(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

type orderedSet map[string]struct{}

func (s orderedSet) add(values ...string) {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		s[value] = struct{}{}
	}
}

func (s orderedSet) values() []string {
	out := make([]string, 0, len(s))
	for value := range s {
		out = append(out, value)
	}
	return out
}

type orderedIntSet map[int]struct{}

func (s orderedIntSet) add(values ...int) {
	for _, value := range values {
		if value <= 0 || value > 65535 {
			continue
		}
		s[value] = struct{}{}
	}
}

func (s orderedIntSet) values() []int {
	out := make([]int, 0, len(s))
	for value := range s {
		out = append(out, value)
	}
	sort.Ints(out)
	return out
}
