export type FindingSeverity = 'info' | 'low' | 'medium' | 'high' | 'critical';
export type FindingStatus = 'open' | 'accepted' | 'false_positive' | 'fixed';

export interface Finding {
  id: number;
  target_id: number;
  asset_id?: number | null;
  url_id?: number | null;
  title: string;
  description?: string;
  severity: FindingSeverity;
  category?: string;
  source_tool?: string;
  evidence?: string;
  recommendation?: string;
  status: FindingStatus;
  fingerprint?: string;
  first_seen?: string;
  last_seen?: string;
  created_at?: string;
  updated_at?: string;
}

export interface FindingsResponse {
  status: string;
  data: Finding[];
  count?: number;
  total?: number;
  total_count?: number;
  page?: number;
}
