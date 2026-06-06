package bugpatterns

import (
	"encoding/json"
	"strings"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"
	"github.com/omidxplimbo/hunt-engine/backend/internal/platform/database"
	"gorm.io/datatypes"
)

func jsonObject(value interface{}) datatypes.JSON {
	if value == nil {
		return datatypes.JSON([]byte("{}"))
	}
	b, err := json.Marshal(value)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte("{}"))
	}
	return datatypes.JSON(b)
}

func jsonArray(values []string) datatypes.JSON {
	clean := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v != "" {
			clean = append(clean, v)
		}
	}
	b, err := json.Marshal(clean)
	if err != nil || len(b) == 0 {
		return datatypes.JSON([]byte("[]"))
	}
	return datatypes.JSON(b)
}

func seedIfMissing(pattern models.BugPattern) error {
	var existing models.BugPattern
	err := database.DB.Where("key = ?", pattern.Key).First(&existing).Error
	if err == nil {
		updates := map[string]interface{}{
			"name":                 pattern.Name,
			"description":          pattern.Description,
			"bug_type":             pattern.BugType,
			"severity_hint":        pattern.SeverityHint,
			"confidence_default":   pattern.ConfidenceDefault,
			"test_level":           pattern.TestLevel,
			"safety_level":         pattern.SafetyLevel,
			"mode":                 pattern.Mode,
			"safe_by_default":      pattern.SafeByDefault,
			"requires_approval":    pattern.RequiresApproval,
			"source":               pattern.Source,
			"version":              pattern.Version,
			"matcher_json":         pattern.MatcherJSON,
			"evidence_schema_json": pattern.EvidenceSchemaJSON,
			"owasp_refs":           pattern.OWASPRefs,
			"tags":                 pattern.Tags,
		}
		return database.DB.Model(&existing).Updates(updates).Error
	}

	return database.DB.Create(&pattern).Error
}

// SeedCore seeds local core safe-testing patterns.
// It is idempotent and local-only in v3.8.0.
func SeedCore() error {
	patterns := []models.BugPattern{
		{
			Key:               "core.passive.xss.parameter_candidate.v1",
			Name:              "Passive XSS parameter candidate",
			Description:       "Flags URLs with reflection-prone query parameter names. No payload is sent.",
			BugType:           models.BugTypeXSS,
			SeverityHint:      models.FindingSeverityInfo,
			ConfidenceDefault: "low",
			TestLevel:         0,
			SafetyLevel:       0,
			Mode:              "passive",
			SafeByDefault:     true,
			RequiresApproval:  false,
			Enabled:           true,
			Source:            "core",
			Version:           "v1",
			MatcherJSON: jsonObject(map[string]interface{}{
				"type":       "url_query_param_name_contains",
				"parameters": []string{"q", "s", "search", "query"},
			}),
			EvidenceSchemaJSON: jsonObject(map[string]interface{}{
				"fields": []string{"url", "parameter", "active_testing", "manual_validation"},
			}),
			OWASPRefs: jsonArray([]string{"OWASP-WSTG-INPV-01", "OWASP-WSTG-INPV-02"}),
			Tags:      jsonArray([]string{"passive", "xss", "parameter_candidate"}),
		},
		{
			Key:               "core.passive.open_redirect.parameter_candidate.v1",
			Name:              "Passive open redirect parameter candidate",
			Description:       "Flags URLs with redirect-like query parameter names. No payload is sent.",
			BugType:           models.BugTypeOpenRedirect,
			SeverityHint:      models.FindingSeverityLow,
			ConfidenceDefault: "low",
			TestLevel:         0,
			SafetyLevel:       0,
			Mode:              "passive",
			SafeByDefault:     true,
			RequiresApproval:  false,
			Enabled:           true,
			Source:            "core",
			Version:           "v1",
			MatcherJSON: jsonObject(map[string]interface{}{
				"type":       "url_query_param_name_contains",
				"parameters": []string{"redirect", "return", "next", "url"},
			}),
			EvidenceSchemaJSON: jsonObject(map[string]interface{}{
				"fields": []string{"url", "parameter", "active_testing", "manual_validation"},
			}),
			OWASPRefs: jsonArray([]string{"OWASP-WSTG-CLNT-04"}),
			Tags:      jsonArray([]string{"passive", "open_redirect", "parameter_candidate"}),
		},
		{
			Key:               "core.passive.security_headers.review.v1",
			Name:              "Passive security headers manual review",
			Description:       "Creates manual-validation tasks for live HTTP assets. No active probe is sent by the bug test engine.",
			BugType:           models.BugTypeSecurityHeaders,
			SeverityHint:      models.FindingSeverityInfo,
			ConfidenceDefault: "low",
			TestLevel:         0,
			SafetyLevel:       0,
			Mode:              "passive",
			SafeByDefault:     true,
			RequiresApproval:  false,
			Enabled:           true,
			Source:            "core",
			Version:           "v1",
			MatcherJSON: jsonObject(map[string]interface{}{
				"type": "live_http_asset",
			}),
			EvidenceSchemaJSON: jsonObject(map[string]interface{}{
				"fields": []string{"asset", "final_url", "status_code", "web_server", "active_testing", "manual_validation"},
			}),
			OWASPRefs: jsonArray([]string{"OWASP-WSTG-CONF", "ASVS-14"}),
			Tags:      jsonArray([]string{"passive", "headers", "manual_validation"}),
		},
	}

	for _, pattern := range patterns {
		if err := seedIfMissing(pattern); err != nil {
			return err
		}
	}

	return nil
}
