export interface Asset {
  id: number;
  value: string;
  type: string;
  is_new: boolean;
  is_live: boolean;
  created_at: string;
  
  // فیلدهای فاز ۲ (پروبینگ)
  final_url?: string;
  status_code?: number;
  title?: string;
  content_length?: number;
  host_ip?: string; 
  dnsx_ip?: string;
  web_server?: string;
  cdn_name?: string;
  cdncheck?: boolean;
  cdncheck_name?: string;
  wafcheck?: boolean;
  wafcheck_name?: string;
  cloudcheck?: boolean;
  cloudcheck_name?: string;
  technologies?: string[] | string; // 👈 اصلاح تایپ برای هندل کردن آرایه یا رشته
  response_time_ms?: number;

  // 👇 Port scan results (per IP) e.g. { "1.2.3.4": [80, 443] }
  open_ports?: Record<string, number[]> | string;
  
  // داده خام
  raw_httpx?: any;
  
  // 👇 Sources/Providers که این subdomain را پیدا کرده‌اند
  sources?: string[];
}

export interface AssetResponse {
  status: string;
  data: Asset[];
  page_count: number;
  total_count: number;
}

export interface AssetFilters {
  is_live?: boolean;
  is_new?: boolean;
  search?: string;
  has_httpx?: boolean;
  dns_only?: boolean;
  has_ports?: boolean; // 👈 فقط آیتم‌هایی که برایشان پورت ثبت شده
  no_cdn?: boolean; // 👈 فیلتر برای نمایش فقط ساب‌دامین‌های بدون CDN
  has_cdn?: boolean; // 👈 فقط آیتم‌هایی که CDN دارند
  has_waf?: boolean; // 👈 فقط آیتم‌هایی که WAF دارند
  has_cloud?: boolean; // 👈 فقط آیتم‌هایی که CLOUD دارند
  sources?: string[]; // 👈 فیلتر بر اساس providers (مثلاً ["subfinder","crtsh"])
}