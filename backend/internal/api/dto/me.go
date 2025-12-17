package dto

// MeResponse اطلاعات کاربر فعلی
type MeResponse struct {
	ID        uint   `json:"id"`
	Username  string `json:"username"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
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



