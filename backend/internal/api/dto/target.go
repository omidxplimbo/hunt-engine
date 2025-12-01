package dto

// CreateTargetRequest ساختاری است که انتظار داریم فرانت‌اند برای ما بفرستد
type CreateTargetRequest struct {
	// نام شرکت یا پروژه (اجباری)
	Name string `json:"name" validate:"required,min=3,max=100"`
	// دامنه اصلی بدون http (اجباری و باید فرمت دامنه داشته باشد)
	RootDomain string `json:"root_domain" validate:"required,hostname"`
	// توضیحات اختیاری
	Description string `json:"description"`

	// 👇👇👇 فیلدهای جدید
	Frequency int      `json:"frequency"` // 0 یعنی دستی، >0 یعنی اتوماتیک (دقیقه)
	UseAlterx *bool    `json:"use_alterx"`
	Modules   []string `json:"modules"` // ["DISCOVERY", "PROBING"]
}

// UpdateTargetRequest برای ویرایش تنظیمات تارگت
type UpdateTargetRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Frequency   *int     `json:"frequency"` // پوینتر: اگر نال بود یعنی تغییر نده
	InScope     *bool    `json:"in_scope"`  // پوینتر: برای تشخیص false واقعی
	Modules     []string `json:"modules"`
	UseAlterx   *bool    `json:"use_alterx"`
}
