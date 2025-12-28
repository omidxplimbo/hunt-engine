package dto

import "time"

type FindingResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	TargetID uint  `json:"target_id"`
	AssetID  *uint `json:"asset_id"`
	URLID    *uint `json:"url_id"`

	Title    string `json:"title"`
	Type     string `json:"type"`
	Severity string `json:"severity"`
	Status   string `json:"status"`
	Hash     string `json:"hash"`

	TemplateID  string `json:"template_id"`
	MatcherName string `json:"matcher_name"`
	MatchedAt   string `json:"matched_at"`
	Host        string `json:"host"`
	CurlCommand string `json:"curl_command"`

	Tags       interface{} `json:"tags"`
	Evidence   interface{} `json:"evidence"`
	Notes      string      `json:"notes"`
	ReportedTo string      `json:"reported_to"`
}

type ListFindingsResponse struct {
	Status     string           `json:"status"`
	Data       []FindingResponse `json:"data"`
	PageCount  int              `json:"page_count"`
	TotalCount int64            `json:"total_count"`
}

type UpdateFindingRequest struct {
	Status     *string `json:"status"`
	Notes      *string `json:"notes"`
	ReportedTo *string `json:"reported_to"`
}


