package dto

// CreateTargetRequest ساختاری است که انتظار داریم فرانت‌اند برای ما بفرستد
type CreateTargetRequest struct {
	// نام شرکت یا پروژه (اجباری)
	Name string `json:"name" validate:"required,min=3,max=100"`
	// دامنه اصلی بدون http (اجباری و باید فرمت دامنه داشته باشد)
	RootDomain string `json:"root_domain" validate:"required,hostname"`
	// توضیحات اختیاری
	Description string `json:"description"`
}
