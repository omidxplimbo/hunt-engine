package dto

import "time"

// UserResponse اطلاعات امن کاربر (بدون پسورد) برای نمایش در لیست
type UserResponse struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Role      string    `json:"role"`
	CreatedAt time.Time `json:"created_at"`
}

// CreateUserRequest (قبلاً در auth.go بود، اینجا هم می‌تونه باشه یا همونجا)
// ما همون قبلی رو استفاده می‌کنیم ولی Update رو اینجا اضافه می‌کنیم

// UpdateUserRequest برای ویرایش رمز عبور یا نقش
type UpdateUserRequest struct {
	Username *string `json:"username"`
	Password *string `json:"password"` // اختیاری (اگر خالی بود تغییر نکند)
	Role     *string `json:"role"`
}
