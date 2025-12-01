package dto

// CreateTargetRequest ساختاری است که انتظار داریم فرانت‌اند برای ما بفرستد
type CreateTargetRequest struct {
	Name        string   `json:"name" validate:"required,min=3,max=100"`
	RootDomain  string   `json:"root_domain" validate:"required,hostname"`
	Description string   `json:"description"`
	Frequency   int      `json:"frequency"`
	Modules     []string `json:"modules"`
	UseAlterx   *bool    `json:"use_alterx"`
}

// UpdateTargetRequest برای ویرایش تنظیمات تارگت
type UpdateTargetRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Frequency   *int     `json:"frequency"`
	InScope     *bool    `json:"in_scope"`
	Modules     []string `json:"modules"`
	UseAlterx   *bool    `json:"use_alterx"`
}
