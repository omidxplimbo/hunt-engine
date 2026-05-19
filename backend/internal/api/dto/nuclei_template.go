package dto

import "time"

// NucleiTemplateResponse is returned by the custom Nuclei template API.
type NucleiTemplateResponse struct {
	ID               uint       `json:"id"`
	UserID           uint       `json:"user_id,omitempty"`
	Name             string     `json:"name"`
	Placement        string     `json:"placement"`
	RelativePath     string     `json:"relative_path"`
	Path             string     `json:"path"`
	SizeBytes        int64      `json:"size_bytes"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	Content          string     `json:"content,omitempty"`
	Enabled          bool       `json:"enabled"`
	Source           string     `json:"source,omitempty"`
	Checksum         string     `json:"checksum,omitempty"`
	ValidationStatus string     `json:"validation_status,omitempty"`
	ValidationError  string     `json:"validation_error,omitempty"`
	LastValidatedAt  *time.Time `json:"last_validated_at,omitempty"`
	IsAIGenerated    bool       `json:"is_ai_generated"`
	RequiresApproval bool       `json:"requires_approval"`
}

// UpsertNucleiTemplateRequest creates or replaces a custom Nuclei template.
type UpsertNucleiTemplateRequest struct {
	Name      string `json:"name"`
	Placement string `json:"placement,omitempty"`
	Content   string `json:"content"`
	Validate  *bool  `json:"validate,omitempty"`
	Enabled   *bool  `json:"enabled,omitempty"`
}

// ValidateNucleiTemplateRequest validates either provided content or an already saved custom template.
type ValidateNucleiTemplateRequest struct {
	Name      string `json:"name"`
	Placement string `json:"placement,omitempty"`
	Content   string `json:"content"`
}

// NucleiTemplateValidationResponse describes nuclei -validate output.
type NucleiTemplateValidationResponse struct {
	Valid  bool   `json:"valid"`
	Name   string `json:"name,omitempty"`
	Output string `json:"output,omitempty"`
	Error  string `json:"error,omitempty"`
}
