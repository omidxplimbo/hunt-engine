export interface Target {
  id: number;
  name: string;
  root_domain: string;
  description: string;
  in_scope: boolean;
  created_at: string;
  asset_count: number;
  frequency?: number;
  last_scan_at?: string;
  status: string;
  current_phase?: string;
  use_alterx: boolean;
  // 👇 اضافه شد
  use_waymore: boolean;
  // 👇 optional port scan during discovery
  use_portscan: boolean;
  // 👇 ابزارهای جدید برای فاز اول (Discovery)
  use_cero?: boolean;  // Scrape domain names from SSL certificates
  use_crtsh?: boolean; // Use crt.sh API for subdomain discovery
  use_puredns?: boolean; // Use puredns for bruteforce subdomain discovery
  use_abusedb?: boolean;
  puredns_wordlists?: string[]; // Selected wordlists for puredns
  scan_modules: string;
}

export interface TargetResponse {
  status: string;
  data: Target[];
  count: number;
}
