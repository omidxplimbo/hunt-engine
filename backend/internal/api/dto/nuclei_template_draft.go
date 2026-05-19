package dto

// NucleiTemplateDraftStatusResponse describes the disabled-by-default draft foundation state.
type NucleiTemplateDraftStatusResponse struct {
	Enabled             bool   `json:"enabled"`
	Provider            string `json:"provider"`
	DraftOnly           bool   `json:"draft_only"`
	RequiresHumanReview bool   `json:"requires_human_review"`
	SaveAutomatically   bool   `json:"save_automatically"`
}

// GenerateNucleiTemplateDraftRequest asks Hunt Engine to draft a Nuclei template.
// This is intentionally constrained and draft-only; it does not call an LLM provider yet.
type GenerateNucleiTemplateDraftRequest struct {
	Name         string   `json:"name"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	Severity     string   `json:"severity"`
	Tags         []string `json:"tags"`
	Method       string   `json:"method"`
	Path         string   `json:"path"`
	MatcherType  string   `json:"matcher_type"`
	MatcherPart  string   `json:"matcher_part"`
	MatcherValue string   `json:"matcher_value"`
	Validate     bool     `json:"validate"`
}

// NucleiTemplateDraftResponse returns a generated draft. It is never saved automatically.
type NucleiTemplateDraftResponse struct {
	Name                string                            `json:"name"`
	Content             string                            `json:"content"`
	DraftOnly           bool                              `json:"draft_only"`
	RequiresHumanReview bool                              `json:"requires_human_review"`
	Saved               bool                              `json:"saved"`
	Validation          *NucleiTemplateValidationResponse `json:"validation,omitempty"`
}
