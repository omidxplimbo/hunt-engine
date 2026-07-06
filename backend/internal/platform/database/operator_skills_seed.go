package database

import (
	"encoding/json"

	"github.com/omidxplimbo/hunt-engine/backend/internal/models"

	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type builtInOperatorSkillSeed struct {
	Name                      string
	Slug                      string
	Version                   string
	Category                  string
	BugClass                  string
	Description               string
	DefaultRiskLevel          string
	DefaultSafetyLevel        int
	DefaultTestLevel          int
	DefaultAutonomyLevel      int
	PermissionMode            string
	SkillType                 string
	RuntimeBackend            string
	RequiredContext           []string
	TriggerSignals            []string
	SupportedActions          []string
	SuccessCriteria           []string
	FailureCriteria           []string
	MemoryPolicy              map[string]interface{}
	ExecutionProfile          map[string]interface{}
	AuthorizationRequirements map[string]interface{}
	BudgetDefaults            map[string]interface{}
	StopConditions            map[string]interface{}
	UserLearningPolicy        map[string]interface{}
	Metadata                  map[string]interface{}
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
			Slug:                 "xss_reflection_context",
			Name:                 "XSS Reflection Context Validation",
			Category:             models.OperatorSkillCategoryClientSide,
			BugClass:             "xss",
			Description:          "Classify reflected parameter candidates by response context and prepare controlled, sanitizer-aware XSS validation evidence without executing exploit payloads.",
			DefaultRiskLevel:     models.AgentActionRiskMedium,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeActiveValidation,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendInternalHTTP,
			RequiredContext:      []string{"parameter_inventory", "reachable_endpoint", "baseline_response", "content_type", "reflection_candidate"},
			TriggerSignals:       []string{"xss", "reflection", "html_context", "script_context", "attribute_context", "search", "query", "reflected_input"},
			SupportedActions:     []string{"classify_reflection_context", "record_xss_validation_candidate", "prepare_controlled_probe"},
			SuccessCriteria:      []string{"reflection candidate classified by context", "candidate evidence recorded", "next controlled probe path identified"},
			FailureCriteria:      []string{"no reachable reflection candidates", "no parameter inventory", "blocked or inconclusive baseline evidence"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":        "context_classification_no_payload_execution",
				"max_candidates":       25,
				"rate_limited":         true,
				"destructive":          false,
				"state_changing":       false,
				"requires_policy_gate": true,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":    true,
				"payload_execution": false,
				"external_callback": false,
			},
			BudgetDefaults: map[string]interface{}{
				"max_requests":    25,
				"timeout_seconds": 60,
			},
			StopConditions: map[string]interface{}{
				"stop_on_403_429_5xx": true,
				"stop_on_waf_signal":  true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_context_lessons": true,
				"cross_target_abstract": true,
			},
		},
		{
			Slug:                 "dom_xss",
			Name:                 "DOM XSS Audit",
			Category:             models.OperatorSkillCategoryClientSide,
			BugClass:             "dom_xss",
			Description:          "Review JavaScript/SPA route and source-sink candidates for DOM XSS validation planning and evidence capture.",
			DefaultRiskLevel:     models.AgentActionRiskLow,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     1,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeAnalysis,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendNone,
			RequiredContext:      []string{"javascript_assets", "routes", "parameter_inventory", "source_sink_candidates"},
			TriggerSignals:       []string{"dom_xss", "dom", "javascript", "hash", "location", "postmessage", "innerhtml", "document_write", "source_sink"},
			SupportedActions:     []string{"mine_dom_sources_sinks", "record_dom_xss_candidates", "prepare_browser_validation_plan"},
			SuccessCriteria:      []string{"DOM source/sink candidates recorded", "browser validation path identified"},
			FailureCriteria:      []string{"no javascript assets", "no SPA routes", "no source/sink indicators"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":  "analysis_only_no_browser_execution",
				"max_assets":     100,
				"destructive":    false,
				"state_changing": false,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":    true,
				"browser_execution": false,
			},
			BudgetDefaults: map[string]interface{}{
				"max_assets": 100,
			},
			StopConditions: map[string]interface{}{
				"stop_on_no_js_assets": true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_source_sink_lessons": true,
				"cross_target_abstract":     true,
			},
		},
		{
			Slug:                 "crlf_header_injection",
			Name:                 "CRLF/Header Injection Validation",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "crlf_header_injection",
			Description:          "Identify CRLF/header injection candidates from URL parameters and response/header behavior, preparing controlled validation without unsafe payload execution.",
			DefaultRiskLevel:     models.AgentActionRiskMedium,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeActiveValidation,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendInternalHTTP,
			RequiredContext:      []string{"parameter_inventory", "baseline_response_headers", "header_reflection_candidate"},
			TriggerSignals:       []string{"crlf", "header injection", "response splitting", "set-cookie", "location header", "x-forwarded", "header_reflection"},
			SupportedActions:     []string{"classify_header_reflection", "record_crlf_candidates", "prepare_controlled_header_probe"},
			SuccessCriteria:      []string{"header-influencing candidate recorded", "controlled probe path identified"},
			FailureCriteria:      []string{"no header-like parameters", "no baseline headers", "blocked evidence"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":        "candidate_classification_no_header_payload_execution",
				"max_candidates":       25,
				"destructive":          false,
				"state_changing":       false,
				"requires_policy_gate": true,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":    true,
				"payload_execution": false,
			},
			BudgetDefaults: map[string]interface{}{
				"max_requests":    25,
				"timeout_seconds": 60,
			},
			StopConditions: map[string]interface{}{
				"stop_on_403_429_5xx": true,
				"stop_on_waf_signal":  true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_header_behavior_lessons": true,
				"cross_target_abstract":         true,
			},
		},
		{
			Slug:                 "cache_poisoning_deception",
			Name:                 "Cache Poisoning/Deception Validation",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "cache_poisoning_deception",
			Description:          "Analyze cache headers, URL/path behavior, and cache-key candidate signals for controlled cache poisoning/deception validation planning.",
			DefaultRiskLevel:     models.AgentActionRiskMedium,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeActiveValidation,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendInternalHTTP,
			RequiredContext:      []string{"baseline_response_headers", "cache_headers", "url_inventory", "path_candidates"},
			TriggerSignals:       []string{"cache poisoning", "cache deception", "cache-control", "cdn", "akamai", "cloudflare", "vary", "x-cache", "web cache"},
			SupportedActions:     []string{"classify_cacheability", "record_cache_candidates", "prepare_controlled_cache_probe"},
			SuccessCriteria:      []string{"cacheable candidate recorded", "cache behavior evidence captured", "next controlled probe path identified"},
			FailureCriteria:      []string{"no cache headers", "no cacheable routes", "blocked evidence"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":        "cache_behavior_classification_no_poisoning_payload",
				"max_candidates":       25,
				"destructive":          false,
				"state_changing":       false,
				"requires_policy_gate": true,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":          true,
				"cache_poisoning_payload": false,
			},
			BudgetDefaults: map[string]interface{}{
				"max_requests":    25,
				"timeout_seconds": 60,
			},
			StopConditions: map[string]interface{}{
				"stop_on_403_429_5xx": true,
				"stop_on_waf_signal":  true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_cache_behavior_lessons": true,
				"cross_target_abstract":        true,
			},
		},
		{
			Slug:                 "open_redirect_chain",
			Name:                 "Open Redirect Chain Validation",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "open_redirect",
			Description:          "Extend open redirect planning toward chain-aware validation candidates such as OAuth/callback flows and parser-confusion redirect parameters.",
			DefaultRiskLevel:     models.AgentActionRiskMedium,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeActiveValidation,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendInternalHTTP,
			RequiredContext:      []string{"parameter_inventory", "redirect_candidate", "baseline_redirect_behavior", "auth_or_oauth_flow_optional"},
			TriggerSignals:       []string{"open_redirect", "redirect_chain", "oauth", "redirect_uri", "callback", "return_url", "next", "continue", "parser_confusion"},
			SupportedActions:     []string{"classify_redirect_chain_candidate", "record_redirect_chain_candidate", "prepare_controlled_redirect_probe"},
			SuccessCriteria:      []string{"redirect-chain candidate recorded", "chain impact path identified"},
			FailureCriteria:      []string{"no redirect-like parameters", "no callback routes", "blocked evidence"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":        "chain_candidate_classification_no_external_redirect_validation",
				"max_candidates":       25,
				"destructive":          false,
				"state_changing":       false,
				"requires_policy_gate": true,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":               true,
				"external_redirect_validation": false,
			},
			BudgetDefaults: map[string]interface{}{
				"max_requests":    25,
				"timeout_seconds": 60,
			},
			StopConditions: map[string]interface{}{
				"stop_on_403_429_5xx": true,
				"stop_on_waf_signal":  true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_redirect_chain_lessons": true,
				"cross_target_abstract":        true,
			},
		},
		{
			Slug:                 "path_traversal_file_read_baseline",
			Name:                 "Path Traversal/File Read Baseline Validation",
			Category:             models.OperatorSkillCategoryNetworkFileCloud,
			BugClass:             "path_traversal_file_read",
			Description:          "Classify file/path/download/template parameters for controlled file-read/path traversal baseline validation without executing file-read payloads in this patch.",
			DefaultRiskLevel:     models.AgentActionRiskMedium,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     2,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeActiveValidation,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendInternalHTTP,
			RequiredContext:      []string{"parameter_inventory", "file_like_params", "baseline_response", "download_or_template_routes"},
			TriggerSignals:       []string{"path_traversal", "file_read", "lfi", "download", "file", "path", "template", "include", "export"},
			SupportedActions:     []string{"classify_file_read_candidate", "record_path_traversal_candidate", "prepare_controlled_file_read_probe"},
			SuccessCriteria:      []string{"file/path candidate recorded", "controlled validation path identified"},
			FailureCriteria:      []string{"no file/path parameters", "no download/template routes", "blocked evidence"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":        "candidate_classification_no_file_read_payload_execution",
				"max_candidates":       25,
				"destructive":          false,
				"state_changing":       false,
				"requires_policy_gate": true,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":              true,
				"file_read_payload_execution": false,
			},
			BudgetDefaults: map[string]interface{}{
				"max_requests":    25,
				"timeout_seconds": 60,
			},
			StopConditions: map[string]interface{}{
				"stop_on_403_429_5xx": true,
				"stop_on_waf_signal":  true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_file_path_lessons": true,
				"cross_target_abstract":   true,
			},
		},
		{
			Slug:                 "cors_clickjacking_csrf",
			Name:                 "CORS/Clickjacking/CSRF Baseline Validation",
			Category:             models.OperatorSkillCategoryHTTPAnalysis,
			BugClass:             "cors_clickjacking_csrf",
			Description:          "Analyze baseline CORS, frame, cookie, and CSRF-relevant headers to prepare controlled client-side/session validation paths.",
			DefaultRiskLevel:     models.AgentActionRiskLow,
			DefaultSafetyLevel:   1,
			DefaultTestLevel:     1,
			DefaultAutonomyLevel: 1,
			PermissionMode:       models.OperatorSkillPermissionScopeAwareAuthorized,
			SkillType:            models.OperatorSkillTypeAnalysis,
			RuntimeBackend:       models.OperatorSkillRuntimeBackendInternalHTTP,
			RequiredContext:      []string{"baseline_response_headers", "cookies", "forms_or_state_changing_routes_optional"},
			TriggerSignals:       []string{"cors", "clickjacking", "csrf", "frame-ancestors", "x-frame-options", "access-control-allow-origin", "sameSite", "cookie"},
			SupportedActions:     []string{"classify_cors_headers", "classify_frame_headers", "classify_cookie_csrf_baseline", "record_session_header_candidates"},
			SuccessCriteria:      []string{"CORS/frame/cookie baseline evidence recorded", "state-changing validation prerequisites identified"},
			FailureCriteria:      []string{"no baseline HTTP evidence", "no response headers", "auth context required"},
			MemoryPolicy: map[string]interface{}{
				"write_target_memory": true,
				"memory_type":         "vulnerability_hypothesis",
				"leakage_safe":        true,
			},
			ExecutionProfile: map[string]interface{}{
				"runtime_scope":  "baseline_header_analysis_no_state_change",
				"max_candidates": 25,
				"destructive":    false,
				"state_changing": false,
			},
			AuthorizationRequirements: map[string]interface{}{
				"scope_required":                 true,
				"state_changing_requests":        false,
				"authenticated_context_optional": true,
			},
			BudgetDefaults: map[string]interface{}{
				"max_requests":    25,
				"timeout_seconds": 60,
			},
			StopConditions: map[string]interface{}{
				"stop_on_403_429_5xx": true,
				"stop_on_waf_signal":  true,
			},
			UserLearningPolicy: map[string]interface{}{
				"store_session_header_lessons": true,
				"cross_target_abstract":        true,
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
		if seed.SkillType == "" {
			seed.SkillType = models.OperatorSkillTypePlanning
		}
		if seed.RuntimeBackend == "" {
			seed.RuntimeBackend = models.OperatorSkillRuntimeBackendNone
		}
		if seed.ExecutionProfile == nil {
			seed.ExecutionProfile = map[string]interface{}{}
		}
		if seed.AuthorizationRequirements == nil {
			seed.AuthorizationRequirements = map[string]interface{}{}
		}
		if seed.BudgetDefaults == nil {
			seed.BudgetDefaults = map[string]interface{}{}
		}
		if seed.StopConditions == nil {
			seed.StopConditions = map[string]interface{}{}
		}
		if seed.UserLearningPolicy == nil {
			seed.UserLearningPolicy = map[string]interface{}{}
		}

		row := models.OperatorSkill{
			OwnerKey:                  "",
			Name:                      seed.Name,
			Slug:                      seed.Slug,
			Version:                   seed.Version,
			Category:                  seed.Category,
			BugClass:                  seed.BugClass,
			Description:               seed.Description,
			DefaultRiskLevel:          seed.DefaultRiskLevel,
			DefaultSafetyLevel:        seed.DefaultSafetyLevel,
			DefaultTestLevel:          seed.DefaultTestLevel,
			DefaultAutonomyLevel:      seed.DefaultAutonomyLevel,
			PermissionMode:            seed.PermissionMode,
			SkillType:                 seed.SkillType,
			RuntimeBackend:            seed.RuntimeBackend,
			RequiredContext:           skillJSON(seed.RequiredContext, "[]"),
			TriggerSignals:            skillJSON(seed.TriggerSignals, "[]"),
			SupportedActions:          skillJSON(seed.SupportedActions, "[]"),
			SuccessCriteria:           skillJSON(seed.SuccessCriteria, "[]"),
			FailureCriteria:           skillJSON(seed.FailureCriteria, "[]"),
			MemoryPolicy:              skillJSON(seed.MemoryPolicy, "{}"),
			ExecutionProfile:          skillJSON(seed.ExecutionProfile, "{}"),
			AuthorizationRequirements: skillJSON(seed.AuthorizationRequirements, "{}"),
			BudgetDefaults:            skillJSON(seed.BudgetDefaults, "{}"),
			StopConditions:            skillJSON(seed.StopConditions, "{}"),
			UserLearningPolicy:        skillJSON(seed.UserLearningPolicy, "{}"),
			Metadata:                  skillJSON(seed.Metadata, "{}"),
			IsBuiltIn:                 true,
			IsEnabled:                 true,
		}

		if err := db.Select("*").Clauses(clause.OnConflict{
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
				"skill_type",
				"runtime_backend",
				"execution_profile",
				"authorization_requirements",
				"budget_defaults",
				"stop_conditions",
				"user_learning_policy",
				"metadata",
				"is_built_in",
				"is_enabled",
				"updated_at",
			}),
		}).Create(&row).Error; err != nil {
			return err
		}

		// GORM default tags can cause zero-values to be omitted during create/upsert.
		// Force these numeric autonomy/risk fields so built-in level-0 skills remain level 0.
		if err := db.Model(&models.OperatorSkill{}).
			Where("slug = ?", seed.Slug).
			Updates(map[string]interface{}{
				"default_safety_level":   seed.DefaultSafetyLevel,
				"default_test_level":     seed.DefaultTestLevel,
				"default_autonomy_level": seed.DefaultAutonomyLevel,
			}).Error; err != nil {
			return err
		}
	}

	return nil
}
