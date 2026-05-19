package dto

import "time"

// NucleiTemplateResponse is returned by the custom Nuclei template API.
type NucleiTemplateResponse struct {
	Name         string    `json:"name"`
	Placement    string    `json:"placement"`
	RelativePath string    `json:"relative_path"`
	Path         string    `json:"path"`
	SizeBytes    int64     `json:"size_bytes"`
	UpdatedAt    time.Time `json:"updated_at"`
	Content      string    `json:"content,omitempty"`
}

// UpsertNucleiTemplateRequest creates or replaces a custom Nuclei template.
type UpsertNucleiTemplateRequest struct {
	Name      string `json:"name"`
	Placement string `json:"placement,omitempty"`
	Content   string `json:"content"`
	Validate  *bool  `json:"validate,omitempty"`
}

// ValidateNucleiTemplateRequest validates either provided content or an existing template name.
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
