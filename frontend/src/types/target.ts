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
  status: string; // READY, SCANNING
  current_phase?: string; // 👇 فیلد جدید (PHASE 1: ..., PHASE 2: ...)
  use_alterx: boolean;
  scan_modules: string;
}

export interface TargetResponse {
  status: string;
  data: Target[];
  count: number;
}