package dto

type LoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type AuthResponse struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Role     string `json:"role"`
}

type CreateUserRequest struct {
	Username string `json:"username" validate:"required,min=3"`
	Password string `json:"password" validate:"required,min=6"`
	Role     string `json:"role"` // admin, viewer
}
