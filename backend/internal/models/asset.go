package models

import (
	"time"

	"gorm.io/gorm"
)

// ==========================================
// Target Model
// ==========================================
type Target struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// مالک تارگت (برای جداسازی دسترسی‌ها)
	CreatedByUserID uint `gorm:"index;not null;default:0;uniqueIndex:idx_targets_owner_root_domain,priority:1" json:"created_by_user_id"`

	Name        string `gorm:"size:100;not null" json:"name"`
	RootDomain  string `gorm:"size:255;not null;uniqueIndex:idx_targets_owner_root_domain,priority:2" json:"root_domain"`
	Description string `json:"description"`
	InScope     bool   `gorm:"default:true" json:"in_scope"`

	Frequency  int        `gorm:"default:0" json:"frequency"`
	LastScanAt *time.Time `json:"last_scan_at"`

	Status string `gorm:"size:50;default:'READY'" json:"status"`

	CurrentPhase  string `gorm:"size:100;default:'IDLE'" json:"current_phase"`
	StopRequested bool   `gorm:"default:false" json:"stop_requested"`
	ScanCount     int    `gorm:"default:0" json:"scan_count"`

	UseAlterx bool `gorm:"default:false" json:"use_alterx"`
	// 👇 فیلد جدید برای Waymore (پیش‌فرض غیرفعال)
	UseWaymore bool `gorm:"default:false" json:"use_waymore"`

	// 👇 Port scan (nmap) in PHASE 1 (optional)
	UsePortscan bool `gorm:"default:false" json:"use_portscan"`

	// 👇 ابزارهای جدید برای فاز اول (Discovery)
	UseCero          bool   `gorm:"default:false" json:"use_cero"`    // Scrape domain names from SSL certificates
	UseCrtsh         bool   `gorm:"default:false" json:"use_crtsh"`   // Use crt.sh API for subdomain discovery
	UsePuredns       bool   `gorm:"default:false" json:"use_puredns"` // Use puredns for bruteforce subdomain discovery
	UseAbusedb       bool   `gorm:"default:false" json:"use_abusedb"`
	UseAmass         bool   `gorm:"default:false" json:"use_amass"`
	UseNuclei        bool   `gorm:"default:false" json:"use_nuclei"`
	NucleiProfile    string `gorm:"size:50;default:\'safe\'" json:"nuclei_profile"`
	PurednsWordlists string `gorm:"type:jsonb;default:'[]'" json:"puredns_wordlists"` // Selected wordlists for puredns (e.g. ["wordlist1.txt", "wordlist2.txt"])

	ScanModules string `gorm:"type:jsonb;default:'[\"DISCOVERY\", \"PROBING\"]'" json:"scan_modules"`

	Assets []Asset `json:"-"`

	CreatedByUser User `gorm:"foreignKey:CreatedByUserID" json:"-"`
}

// ==========================================
// Asset Model
// ==========================================
type Asset struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	TargetID uint   `gorm:"not null;index;uniqueIndex:idx_assets_target_value,priority:1" json:"target_id"`
	Value    string `gorm:"size:255;not null;uniqueIndex:idx_assets_target_value,priority:2" json:"value"`
	Type     string `gorm:"size:50" json:"type"`

	IsNew  bool `gorm:"default:true;index" json:"is_new"`
	IsLive bool `gorm:"default:false;index" json:"is_live"`

	LastChangeAt *time.Time `gorm:"index" json:"last_change_at"`

	FinalURL       string `gorm:"size:512" json:"final_url"`
	StatusCode     int    `gorm:"index" json:"status_code"`
	Title          string `gorm:"size:512" json:"title"`
	ContentLength  int64  `json:"content_length"`
	HostIP         string `gorm:"type:jsonb;default:'[]'" json:"host_ip"`
	DnsxIP         string `gorm:"type:jsonb;default:'[]'" json:"dnsx_ip"`
	WebServer      string `gorm:"size:255" json:"web_server"`
	CDNName        string `gorm:"size:100;index" json:"cdn_name"`
	Cdncheck       bool   `gorm:"default:false;index" json:"cdncheck"`
	CdncheckName   string `gorm:"size:100" json:"cdncheck_name"`
	Wafcheck       bool   `gorm:"default:false;index" json:"wafcheck"`
	WafcheckName   string `gorm:"size:100" json:"wafcheck_name"`
	Cloudcheck     bool   `gorm:"default:false;index" json:"cloudcheck"`
	CloudcheckName string `gorm:"size:100" json:"cloudcheck_name"`
	Technologies   string `gorm:"type:jsonb;default:'[]'" json:"technologies"`
	BodyHash       string `gorm:"size:64" json:"body_hash"`
	HeaderHash     string `gorm:"size:64" json:"header_hash"`
	ResponseTimeMs int64  `json:"response_time_ms"`

	// 👇 Port scan results (per IP) e.g. {"1.2.3.4":[80,443]}
	OpenPorts string `gorm:"type:jsonb;default:'{}'" json:"open_ports"`

	RawHttpx string `gorm:"type:jsonb" json:"raw_httpx"`

	// 👇 Sources/Providers که این subdomain را پیدا کرده‌اند (مثلاً: ["subfinder", "assetfinder", "cero", "crtsh"])
	Sources string `gorm:"type:jsonb;default:'[]'" json:"sources"`

	History []AssetHistory `json:"history,omitempty"`
}

// ==========================================
// AssetHistory Model
// ==========================================
type AssetHistory struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	CreatedAt time.Time `json:"created_at"`

	AssetID uint   `gorm:"not null;index" json:"asset_id"`
	Asset   *Asset `json:"-"`

	FieldName string `gorm:"size:100;not null" json:"field_name"`
	OldValue  string `gorm:"type:text" json:"old_value"`
	NewValue  string `gorm:"type:text" json:"new_value"`
}
