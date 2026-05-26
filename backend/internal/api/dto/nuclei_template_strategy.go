package dto

// NucleiTemplateStrategyTargetSummary is the compact target context exposed to future agents.
type NucleiTemplateStrategyTargetSummary struct {
	ID             uint     `json:"id"`
	Name           string   `json:"name"`
	RootDomain     string   `json:"root_domain"`
	NucleiProfile  string   `json:"nuclei_profile"`
	UseNuclei      bool     `json:"use_nuclei"`
	LiveAssetCount int      `json:"live_asset_count"`
	SampleURLs     []string `json:"sample_urls"`
	Technologies   []string `json:"technologies"`
	WebServers     []string `json:"web_servers"`
	OpenPorts      []int    `json:"open_ports"`
}

// NucleiTemplateStrategyAllowedActions describes the safety envelope for agent workflows.
type NucleiTemplateStrategyAllowedActions struct {
	CanSelectProfile       bool `json:"can_select_profile"`
	CanSelectBuiltInTags   bool `json:"can_select_builtin_tags"`
	CanSelectCustomGroups  bool `json:"can_select_custom_groups"`
	CanGenerateDraft       bool `json:"can_generate_draft"`
	CanSaveTemplate        bool `json:"can_save_template"`
	CanAutoSaveTemplate    bool `json:"can_auto_save_template"`
	CanAutoExecuteTemplate bool `json:"can_auto_execute_template"`
	RequiresHumanApproval  bool `json:"requires_human_approval"`
}

// NucleiTemplateStrategySignal is a normalized observation that affected the plan.
type NucleiTemplateStrategySignal struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	Confidence string `json:"confidence"`
	Reason     string `json:"reason"`
}

// NucleiTargetTemplateStrategyResponse is an agent-ready plan for future AI triage/recon agents.
type NucleiTargetTemplateStrategyResponse struct {
	AgentReady              bool                                 `json:"agent_ready"`
	DraftOnly               bool                                 `json:"draft_only"`
	AITemplateDraftsEnabled bool                                 `json:"ai_template_drafts_enabled"`
	SaveAutomatically       bool                                 `json:"save_automatically"`
	ExecuteAutomatically    bool                                 `json:"execute_automatically"`
	FeatureEnabled          bool                                 `json:"feature_enabled"`
	EnvironmentEnabled      bool                                 `json:"environment_enabled"`
	DisabledReason          string                               `json:"disabled_reason,omitempty"`
	Scope                   string                               `json:"scope,omitempty"`
	OwnerKey                string                               `json:"owner_key,omitempty"`
	Target                  NucleiTemplateStrategyTargetSummary  `json:"target"`
	AllowedActions          NucleiTemplateStrategyAllowedActions `json:"allowed_actions"`
	RecommendedProfile      string                               `json:"recommended_profile"`
	RecommendedTags         []string                             `json:"recommended_tags"`
	RecommendedPlacements   []string                             `json:"recommended_placements"`
	RecommendedTemplateSets []string                             `json:"recommended_template_sets"`
	Signals                 []NucleiTemplateStrategySignal       `json:"signals"`
	Rationale               []string                             `json:"rationale"`
	SuggestedDraftRequest   *GenerateNucleiTemplateDraftRequest  `json:"suggested_draft_request,omitempty"`
	GeneratedDraft          *NucleiTemplateDraftResponse         `json:"generated_draft,omitempty"`
	DraftError              string                               `json:"draft_error,omitempty"`
}
