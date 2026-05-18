package dto

// CreateTargetRequest ساختاری است که انتظار داریم فرانت‌اند برای ما بفرستد
type CreateTargetRequest struct {
	Name        string   `json:"name" validate:"required,min=3,max=100"`
	RootDomain  string   `json:"root_domain" validate:"required,hostname"`
	Description string   `json:"description"`
	Frequency   int      `json:"frequency"`
	Modules     []string `json:"modules"`
	UseAlterx   *bool    `json:"use_alterx"`
	// 👇 اضافه شد
	UseWaymore *bool `json:"use_waymore"`
	// 👇 PHASE 1 optional port scan (nmap)
	UsePortscan *bool `json:"use_portscan"`
	// 👇 ابزارهای جدید برای فاز اول (Discovery)
	UseCero          *bool    `json:"use_cero"`    // Scrape domain names from SSL certificates
	UseCrtsh         *bool    `json:"use_crtsh"`   // Use crt.sh API for subdomain discovery
	UsePuredns       *bool    `json:"use_puredns"` // Use puredns for bruteforce subdomain discovery
	UseAbusedb       *bool    `json:"use_abusedb"`
	UseAmass         *bool    `json:"use_amass"`
	PurednsWordlists []string `json:"puredns_wordlists"` // Selected wordlists for puredns
}

// UpdateTargetRequest برای ویرایش تنظیمات تارگت
type UpdateTargetRequest struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Frequency   *int     `json:"frequency"`
	InScope     *bool    `json:"in_scope"`
	Modules     []string `json:"modules"`
	UseAlterx   *bool    `json:"use_alterx"`
	// 👇 اضافه شد
	UseWaymore *bool `json:"use_waymore"`
	// 👇 PHASE 1 optional port scan (nmap)
	UsePortscan *bool `json:"use_portscan"`
	// 👇 ابزارهای جدید برای فاز اول (Discovery)
	UseCero          *bool    `json:"use_cero"`    // Scrape domain names from SSL certificates
	UseCrtsh         *bool    `json:"use_crtsh"`   // Use crt.sh API for subdomain discovery
	UsePuredns       *bool    `json:"use_puredns"` // Use puredns for bruteforce subdomain discovery
	UseAbusedb       *bool    `json:"use_abusedb"`
	UseAmass         *bool    `json:"use_amass"`
	PurednsWordlists []string `json:"puredns_wordlists"` // Selected wordlists for puredns
}

// TargetExportData ساختار استاندارد برای Export/Import تارگت‌ها
// این ساختار شامل تمام اطلاعات لازم برای بازسازی تارگت است
type TargetExportData struct {
	Version    string             `json:"version"`     // نسخه فرمت Export (برای سازگاری آینده)
	ExportDate string             `json:"export_date"` // تاریخ Export
	Targets    []TargetExportItem `json:"targets"`     // لیست تارگت‌ها
}

// TargetExportItem اطلاعات یک تارگت برای Export/Import (شامل تمام داده‌های مرتبط)
type TargetExportItem struct {
	// اطلاعات تارگت
	Name             string   `json:"name" validate:"required,min=3,max=100"`
	RootDomain       string   `json:"root_domain" validate:"required,hostname"`
	Description      string   `json:"description"`
	InScope          bool     `json:"in_scope"`
	Frequency        int      `json:"frequency"`
	Modules          []string `json:"modules"`
	UseAlterx        bool     `json:"use_alterx"`
	UseWaymore       bool     `json:"use_waymore"`
	UsePortscan      bool     `json:"use_portscan"`
	UseCero          bool     `json:"use_cero"`
	UseCrtsh         bool     `json:"use_crtsh"`
	UsePuredns       bool     `json:"use_puredns"`
	UseAbusedb       bool     `json:"use_abusedb"`
	UseAmass         bool     `json:"use_amass"`
	PurednsWordlists []string `json:"puredns_wordlists"`

	// داده‌های مرتبط
	Assets []AssetExportItem `json:"assets"` // لیست ساب‌دامین‌ها و دارایی‌ها
	URLs   []URLExportItem   `json:"urls"`   // لیست URLهای پیدا شده
}

// AssetExportItem اطلاعات یک Asset برای Export/Import
type AssetExportItem struct {
	Value          string `json:"value"`
	Type           string `json:"type"`
	IsNew          bool   `json:"is_new"`
	IsLive         bool   `json:"is_live"`
	FinalURL       string `json:"final_url"`
	StatusCode     int    `json:"status_code"`
	Title          string `json:"title"`
	ContentLength  int64  `json:"content_length"`
	HostIP         string `json:"host_ip"`
	DnsxIP         string `json:"dnsx_ip"`
	WebServer      string `json:"web_server"`
	CDNName        string `json:"cdn_name"`
	Technologies   string `json:"technologies"`
	BodyHash       string `json:"body_hash"`
	HeaderHash     string `json:"header_hash"`
	ResponseTimeMs int64  `json:"response_time_ms"`
	RawHttpx       string `json:"raw_httpx"`
	OpenPorts      string `json:"open_ports"`
	CreatedAt      string `json:"created_at"`
}

// URLExportItem اطلاعات یک FoundURL برای Export/Import
type URLExportItem struct {
	Value     string `json:"value"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
}

// ImportTargetRequest درخواست Import تارگت‌ها
type ImportTargetRequest struct {
	Data TargetExportData `json:"data" validate:"required"`
	// اگر true باشد، تارگت‌های موجود با root_domain یکسان را skip می‌کند
	SkipExisting bool `json:"skip_existing"`
}
