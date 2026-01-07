package dto

import "time"

// TargetResponse ساختار خروجی برای اطلاعات تارگت
type TargetResponse struct {
	ID           uint       `json:"id"`
	Name         string     `json:"name"`
	RootDomain   string     `json:"root_domain"`
	Description  string     `json:"description"`
	InScope      bool       `json:"in_scope"`
	CreatedAt    time.Time  `json:"created_at"`
	AssetCount   int64      `json:"asset_count"`
	Frequency    int        `json:"frequency"`
	LastScanAt   *time.Time `json:"last_scan_at"`
	Status       string     `json:"status"`
	CurrentPhase string     `json:"current_phase"`

	UseAlterx bool `json:"use_alterx"`
	// 👇 اضافه شد
	UseWaymore bool `json:"use_waymore"`
	// 👇 optional port scan during discovery
	UsePortscan bool `json:"use_portscan"`
	// 👇 ابزارهای جدید برای فاز اول (Discovery)
	UseCero  bool     `json:"use_cero"`
	UseCrtsh bool     `json:"use_crtsh"`
	UsePuredns bool   `json:"use_puredns"`
	UseAbusedb bool   `json:"use_abusedb"`
	PurednsWordlists interface{} `json:"puredns_wordlists"`

	ScanModules string `json:"scan_modules"`
}

// ... (بقیه DTOها بدون تغییر)
type AssetResponse struct {
	ID             uint        `json:"id"`
	Value          string      `json:"value"`
	Type           string      `json:"type"`
	IsNew          bool        `json:"is_new"`
	IsLive         bool        `json:"is_live"`
	CreatedAt      time.Time   `json:"created_at"`
	FinalURL       string      `json:"final_url"`
	StatusCode     int         `json:"status_code"`
	Title          string      `json:"title"`
	ContentLength  int64       `json:"content_length"`
	HostIP         string      `json:"host_ip"`
	DnsxIP         string      `json:"dnsx_ip"`
	WebServer      string      `json:"web_server"`
	CDNName        string      `json:"cdn_name"`
	Cdncheck       bool        `json:"cdncheck"`
	CdncheckName   string      `json:"cdncheck_name"`
	Wafcheck       bool        `json:"wafcheck"`
	WafcheckName   string      `json:"wafcheck_name"`
	Cloudcheck     bool        `json:"cloudcheck"`
	CloudcheckName string      `json:"cloudcheck_name"`
	Technologies   interface{} `json:"technologies"`
	ResponseTime   int64       `json:"response_time_ms"`
	OpenPorts      interface{} `json:"open_ports"`
	RawHttpx       interface{} `json:"raw_httpx,omitempty"`
	Sources        interface{} `json:"sources"` // لیست ابزارهایی که این subdomain را پیدا کرده‌اند
}

type FoundURLResponse struct {
	ID        uint      `json:"id"`
	Value     string    `json:"value"`
	Source    string    `json:"source"`
	CreatedAt time.Time `json:"created_at"`
}
