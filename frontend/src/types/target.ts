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
  use_waymore: boolean;
  use_portscan: boolean;
  use_cero?: boolean;
  use_crtsh?: boolean;
  use_puredns?: boolean;
  use_abusedb?: boolean;
  use_amass?: boolean; use_nuclei?: boolean; nuclei_profile?: string;
  puredns_wordlists?: string[];
  scan_modules: string;
  created_by_user_id?: number;
  owner_username?: string;
}

export type TargetDetails = Target;

export interface TargetResponse {
  status: string;
  data: Target[];
  count: number;
}
