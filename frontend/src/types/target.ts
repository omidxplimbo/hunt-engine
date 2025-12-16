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
  scan_modules: string;
}

export interface TargetResponse {
  status: string;
  data: Target[];
  count: number;
}