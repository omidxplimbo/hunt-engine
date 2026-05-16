package dto

// MeResponse اطلاعات کاربر فعلی
type MeResponse struct {
	ID                 uint   `json:"id"`
	Username           string `json:"username"`
	Role               string `json:"role"`
	CreatedAt          string `json:"created_at"`
	MaxConcurrentScans int    `json:"max_concurrent_scans"`
}

type UpdateMeRequest struct {
	Username *string `json:"username"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password"`
	NewPassword     string `json:"new_password"`
}

type DeleteMeRequest struct {
	CurrentPassword string `json:"current_password"`
}

// -----------------------------
// Subfinder Provider Config (User-scoped)
// -----------------------------

type SubfinderProviderItem struct {
	Provider string        `json:"provider"`
	Entries  []interface{} `json:"entries"`
}

type PutMySubfinderProvidersRequest struct {
	// Replace mode: the provided list becomes the full set for the current user.
	Providers []SubfinderProviderItem `json:"providers"`
}
