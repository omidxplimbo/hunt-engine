export type FindingStatus = 'open' | 'triaged' | 'duplicate' | 'fixed' | 'wontfix';
export type FindingSeverity = 'info' | 'low' | 'medium' | 'high' | 'critical';

export interface Finding {
  id: number;
  created_at: string;
  updated_at: string;
  target_id: number;
  asset_id?: number | null;
  url_id?: number | null;

  title: string;
  type: string;
  severity: FindingSeverity;
  status: FindingStatus;
  hash: string;

  template_id?: string;
  matcher_name?: string;
  matched_at?: string;
  host?: string;
  curl_command?: string;

  tags?: string[] | any;
  evidence?: any;
  notes?: string;
  reported_to?: string;
}

export interface FindingListResponse {
  status: string;
  data: Finding[];
  page_count: number;
  total_count: number;
}


