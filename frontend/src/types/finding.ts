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
  triage_note?: string;
  triaged_at?: string | null;
  triaged_by_user_id?: number | null;
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

export interface FindingStats {
  total: number;
  open: number;
  accepted: number;
  false_positive: number;
  fixed: number;
  by_severity: Record<FindingSeverity, number> & Record<string, number>;
  by_status: Record<FindingStatus, number> & Record<string, number>;
  by_source: Record<string, number>;
  by_category: Record<string, number>;
}

export interface FindingStatsResponse {
  status: string;
  data: FindingStats;
}
