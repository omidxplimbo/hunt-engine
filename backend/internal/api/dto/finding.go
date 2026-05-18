package dto

import "time"

type FindingResponse struct {
	ID             uint      `json:"id"`
	TargetID       uint      `json:"target_id"`
	AssetID        *uint     `json:"asset_id"`
	URLID          *uint     `json:"url_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	Severity       string    `json:"severity"`
	Category       string    `json:"category"`
	SourceTool     string    `json:"source_tool"`
	Evidence       string    `json:"evidence"`
	Recommendation string    `json:"recommendation"`
	Status         string    `json:"status"`
	Fingerprint    string    `json:"fingerprint"`
	FirstSeen      time.Time `json:"first_seen"`
	LastSeen       time.Time `json:"last_seen"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type UpdateFindingStatusRequest struct {
	Status string `json:"status"`
}
