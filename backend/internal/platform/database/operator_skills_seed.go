package database

import (
	"encoding/json"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type builtInOperatorSkillSeed struct {
	Name                 string
	Slug                 string
	Version              string
	Category             string
	BugClass             string
	Description          string
	DefaultRiskLevel     string
	DefaultSafetyLevel   int
	DefaultTestLevel     int
	DefaultAutonomyLevel int
	PermissionMode       string
	RequiredContext      []string
	TriggerSignals       []string
	SupportedActions     []string
	SuccessCriteria      []string
	FailureCriteria      []string
	MemoryPolicy         map[string]interface{}
	Metadata             map[string]interface{}
}

func skillJSON(value interface{}, fallback string) datatypes.JSON {
	if value == nil {
		return datatypes.JSON([]byte(fallback))
	}
	raw, err := json.Marshal(value)
	if err != nil || len(raw) == 0 {
		return datatypes.JSON([]byte(fallback))
	}
	return datatypes.JSON(raw)
}

func seedBuiltInOperatorSkills(db *gorm.DB) error {
	seeds := []builtInOperatorSkillSeed{
		{
			Name:                 "Parameter Inventory",
			Slug:                 "parameter_inventory",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryParameterIntel,
			BugClass:             "multi_class",
			Description:          "Mine URL, query, route, API, JS, memory, and runtime evidence to build a parameter inventory for authorized exploitation and validation workflows.",
			DefaultRiskLevel:     "low",
			DefaultSafetyLevel:   0,
			DefaultTestLevel:     0,
			DefaultAutonomyLevel: 1,
			PermissionMode:       "auto_scope_aware",
			RequiredContext:      []string{"target_urls", "assets", "memory", "controlled_runtime_evidence"},
			TriggerSignals:       []string{"high_impact_hunting_intent", "bug_class_validation_requested", "parameter_surface_unknown", "xss", "sqli", "open_redirect", "idor", "ssrf"},
			SupportedActions:     []string{"read_target_urls", "read_target_memory", "read_controlled_runtime_results", "create_skill_observations"},
			SuccessCriteria:      []string{"parameters extracted", "candidate bug classes assigned", "ranked parameter observations stored"},
			FailureCriteria:      []string{"no URLs or API evidence available", "all candidate sources exhausted"},
			MemoryPolicy: map[string]interface{}{
				"store_parameter_inventory": true,
				"store_candidate_classes":   true,
				"cross_target_allowed":      "abstract_patterns_only",
			},
			Metadata: map[string]interface{}{
				"seed_version":  "v3.14.0-skill-seed-v1",
				"priority":      100,
				"operator_note": "Foundation skill. Run before class-specific validation when parameter context is incomplete.",
			},
		},
		{
			Name:                 "HTTP Evidence Analysis",
			Slug:                 "http_evidence_analysis",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "multi_class",
			Description:          "Analyze controlled HTTP responses, headers, redirects, cache behavior, WAF/challenge indicators, content types, cookies, and reachability to guide next exploit/validation steps.",
			DefaultRiskLevel:     "low",
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     1,
			DefaultAutonomyLevel: 1,
			PermissionMode:       "auto_scope_aware",
			RequiredContext:      []string{"controlled_runtime_result", "request_response_metadata"},
			TriggerSignals:       []string{"review_endpoint_completed", "baseline_probe_completed", "blocked", "challenge", "redirect", "cache", "cors", "headers"},
			SupportedActions:     []string{"review_endpoint", "read_controlled_runtime_results", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"runtime evidence classified", "next skills recommended", "blocked/inconclusive evidence not promoted"},
			FailureCriteria:      []string{"missing response evidence", "runtime result unavailable"},
			MemoryPolicy: map[string]interface{}{
				"store_reachability":        true,
				"store_blocked_challenges":  true,
				"store_next_skill_hints":    true,
				"avoid_retesting_when_dead": true,
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     95,
			},
		},
		{
			Name:                 "JavaScript Audit",
			Slug:                 "js_audit",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryClientSide,
			BugClass:             "client_side",
			Description:          "Review JavaScript assets, routes, API calls, secrets, sinks, sources, postMessage handlers, prototype pollution candidates, and sanitizer usage for exploitable client-side paths.",
			DefaultRiskLevel:     "low",
			DefaultSafetyLevel:   0,
			DefaultTestLevel:     0,
			DefaultAutonomyLevel: 1,
			PermissionMode:       "auto_scope_aware",
			RequiredContext:      []string{"javascript_assets", "js_intelligence_findings", "urls", "memory"},
			TriggerSignals:       []string{"javascript", "dom_xss", "api_route", "secret", "postmessage", "prototype_pollution", "dompurify"},
			SupportedActions:     []string{"run_js_intelligence", "read_js_findings", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"sinks and sources identified", "candidate flows ranked", "next validation skill recommended"},
			FailureCriteria:      []string{"no JS assets", "JS analysis disabled or unavailable"},
			MemoryPolicy: map[string]interface{}{
				"store_js_sinks":        true,
				"store_sanitizers":      true,
				"store_candidate_flows": true,
				"cross_target_allowed":  "abstract_patterns_only",
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     90,
			},
		},
		{
			Name:                 "DOM XSS Audit",
			Slug:                 "dom_xss_audit",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryClientSide,
			BugClass:             "dom_xss",
			Description:          "Analyze DOM XSS candidates by connecting sources, sinks, routes, sanitizer behavior, and runtime context before executing authorized validation.",
			DefaultRiskLevel:     "low",
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     1,
			DefaultAutonomyLevel: 1,
			PermissionMode:       "scope_aware_authorized",
			RequiredContext:      []string{"javascript_assets", "sources_sinks", "url_routes", "parameter_inventory"},
			TriggerSignals:       []string{"dom_xss", "location.hash", "innerHTML", "document.write", "dompurify", "client_route_param"},
			SupportedActions:     []string{"run_js_intelligence", "review_endpoint", "generate_payload", "create_skill_observations"},
			SuccessCriteria:      []string{"DOM source/sink path identified", "sanitizer/context documented", "safe validation path proposed"},
			FailureCriteria:      []string{"no controllable source", "sink not reachable", "sanitizer blocks candidate path"},
			MemoryPolicy: map[string]interface{}{
				"store_dom_paths":           true,
				"store_failed_sanitizers":   true,
				"store_payload_family_hint": true,
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     86,
			},
		},
		{
			Name:                 "XSS Reflection Validation",
			Slug:                 "xss_reflection",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryClientSide,
			BugClass:             "xss",
			Description:          "Validate reflected XSS/HTML injection candidates with inert markers, context analysis, encoding checks, content-type review, CSP assessment, and authorized PoC escalation when allowed.",
			DefaultRiskLevel:     "medium",
			DefaultSafetyLevel:   2,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 2,
			PermissionMode:       "authorized_exploit_validation",
			RequiredContext:      []string{"parameter_inventory", "reachable_endpoint", "content_type", "reflection_candidate"},
			TriggerSignals:       []string{"xss", "html_injection", "reflected_parameter", "search_param", "callback_param"},
			SupportedActions:     []string{"review_endpoint", "generate_payload", "execute_payload_test", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"marker reflected", "context classified", "escaping behavior understood", "authorized PoC path available"},
			FailureCriteria:      []string{"no reflection", "blocked by WAF", "requires auth context", "content-type not executable"},
			MemoryPolicy: map[string]interface{}{
				"store_reflection_context": true,
				"store_failed_payloads":    true,
				"store_successful_poc":     true,
				"finding_candidate_when":   "authorized_poc_or_strong_context_evidence",
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     88,
			},
		},
		{
			Name:                 "Open Redirect Validation",
			Slug:                 "open_redirect",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "open_redirect",
			Description:          "Validate redirect parameters, external Location behavior, encoding bypasses, OAuth redirect_uri relevance, and exploit-chain potential when authorized.",
			DefaultRiskLevel:     "medium",
			DefaultSafetyLevel:   2,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 2,
			PermissionMode:       "authorized_exploit_validation",
			RequiredContext:      []string{"parameter_inventory", "redirect_candidate", "baseline_redirect_behavior"},
			TriggerSignals:       []string{"redirect", "url_param", "next", "return", "callback", "redirect_uri", "oauth"},
			SupportedActions:     []string{"review_endpoint", "execute_payload_test", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"external redirect observed", "bypass behavior documented", "chain relevance assessed"},
			FailureCriteria:      []string{"no redirect behavior", "allowlist blocks external target", "requires auth context"},
			MemoryPolicy: map[string]interface{}{
				"store_redirect_params": true,
				"store_bypass_results":  true,
				"chain_with_oauth":      true,
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     80,
			},
		},
		{
			Name:                 "CRLF Header Injection",
			Slug:                 "crlf_header_injection",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "crlf_header_injection",
			Description:          "Validate CRLF/header injection candidates by testing controlled inert header markers, Location/header reflection, encoding behavior, and response splitting signals.",
			DefaultRiskLevel:     "medium",
			DefaultSafetyLevel:   2,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 2,
			PermissionMode:       "authorized_exploit_validation",
			RequiredContext:      []string{"parameter_inventory", "header_reflection_candidate", "baseline_response_headers"},
			TriggerSignals:       []string{"crlf", "header_injection", "response_splitting", "location_header", "download_filename"},
			SupportedActions:     []string{"review_endpoint", "execute_payload_test", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"controlled header marker observed", "encoding behavior classified", "response splitting risk assessed"},
			FailureCriteria:      []string{"no header reflection", "normalization blocks marker", "blocked by WAF"},
			MemoryPolicy: map[string]interface{}{
				"store_header_reflection": true,
				"store_encoding_results":  true,
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     76,
			},
		},
		{
			Name:                 "Cacheability and Cache Deception",
			Slug:                 "cacheability",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "cache_poisoning_cache_deception",
			Description:          "Analyze cache headers, Vary behavior, CDN hints, Age/X-Cache, path confusion, and low-risk cache deception/poisoning candidates before authorized validation.",
			DefaultRiskLevel:     "medium",
			DefaultSafetyLevel:   2,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 2,
			PermissionMode:       "authorized_exploit_validation",
			RequiredContext:      []string{"baseline_response_headers", "url_paths", "cache_headers"},
			TriggerSignals:       []string{"cache", "cdn", "x-cache", "age", "vary", "cache-control", "path_confusion"},
			SupportedActions:     []string{"review_endpoint", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"cacheability classified", "candidate path behavior documented", "safe validation path selected"},
			FailureCriteria:      []string{"no cache hints", "endpoint uncacheable", "blocked or auth-required"},
			MemoryPolicy: map[string]interface{}{
				"store_cache_headers":   true,
				"store_cache_decisions": true,
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     74,
			},
		},
		{
			Name:                 "Path Traversal and File Read Baseline",
			Slug:                 "path_traversal_baseline",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryNetworkFileCloud,
			BugClass:             "path_traversal_file_read",
			Description:          "Identify and validate file/path-like parameters, normalization behavior, error signatures, download handlers, and authorized file-read proof paths.",
			DefaultRiskLevel:     "medium",
			DefaultSafetyLevel:   2,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 2,
			PermissionMode:       "authorized_exploit_validation",
			RequiredContext:      []string{"parameter_inventory", "file_like_params", "baseline_response"},
			TriggerSignals:       []string{"file", "path", "download", "template", "include", "dir", "path_traversal", "lfi"},
			SupportedActions:     []string{"review_endpoint", "generate_payload", "execute_payload_test", "create_skill_observations", "write_memory"},
			SuccessCriteria:      []string{"file-like parameter identified", "normalization behavior classified", "authorized proof path available"},
			FailureCriteria:      []string{"no file-like parameter", "normalization blocks traversal", "requires auth context"},
			MemoryPolicy: map[string]interface{}{
				"store_file_params":      true,
				"store_normalization":    true,
				"finding_candidate_when": "authorized_file_read_evidence",
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     82,
			},
		},
		{
			Name:                 "Authenticated Context Needed",
			Slug:                 "auth_context_needed",
			Version:              "v1",
			Category:             models.OperatorSkillCategoryAccessControl,
			BugClass:             "auth_access_control",
			Description:          "Detect when meaningful exploitation requires cookies, tokens, CSRF values, role context, second account, organization/tenant context, or explicit authorization details.",
			DefaultRiskLevel:     "low",
			DefaultSafetyLevel:   0,
			DefaultTestLevel:     0,
			DefaultAutonomyLevel: 1,
			PermissionMode:       "ask_for_context",
			RequiredContext:      []string{"http_status", "auth_headers", "cookies", "session_indicators", "target_policy"},
			TriggerSignals:       []string{"401", "403", "login", "session", "csrf", "idor", "bola", "bfla", "authz", "tenant"},
			SupportedActions:     []string{"read_runtime_evidence", "ask_user_for_context", "write_memory"},
			SuccessCriteria:      []string{"missing auth context identified", "precise context request produced", "testing paused until authorized context exists"},
			FailureCriteria:      []string{"auth need uncertain", "policy disallows authenticated testing"},
			MemoryPolicy: map[string]interface{}{
				"store_required_context": true,
				"store_paused_tests":     true,
				"avoid_guessing":         true,
			},
			Metadata: map[string]interface{}{
				"seed_version": "v3.14.0-skill-seed-v1",
				"priority":     84,
			},
		},
	}

	for _, seed := range seeds {
		row := models.OperatorSkill{
			OwnerKey:             "",
			Name:                 seed.Name,
			Slug:                 seed.Slug,
			Version:              seed.Version,
			Category:             seed.Category,
			BugClass:             seed.BugClass,
			Description:          seed.Description,
			DefaultRiskLevel:     seed.DefaultRiskLevel,
			DefaultSafetyLevel:   seed.DefaultSafetyLevel,
			DefaultTestLevel:     seed.DefaultTestLevel,
			DefaultAutonomyLevel: seed.DefaultAutonomyLevel,
			PermissionMode:       seed.PermissionMode,
			RequiredContext:      skillJSON(seed.RequiredContext, "[]"),
			TriggerSignals:       skillJSON(seed.TriggerSignals, "[]"),
			SupportedActions:     skillJSON(seed.SupportedActions, "[]"),
			SuccessCriteria:      skillJSON(seed.SuccessCriteria, "[]"),
			FailureCriteria:      skillJSON(seed.FailureCriteria, "[]"),
			MemoryPolicy:         skillJSON(seed.MemoryPolicy, "{}"),
			Metadata:             skillJSON(seed.Metadata, "{}"),
			IsBuiltIn:            true,
			IsEnabled:            true,
		}

		if err := db.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "slug"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"name",
				"version",
				"category",
				"bug_class",
				"description",
				"default_risk_level",
				"default_safety_level",
				"default_test_level",
				"default_autonomy_level",
				"permission_mode",
				"required_context",
				"trigger_signals",
				"supported_actions",
				"success_criteria",
				"failure_criteria",
				"memory_policy",
				"metadata",
				"is_built_in",
				"is_enabled",
				"updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return err
		}
	}

	return nil
}
