export interface Target {
  id: number;
  name: string;
  root_domain: string;
  description: string;
  in_scope: boolean;
  created_at: string;
  asset_count: number; // این فیلد خیلی مهمه که تعداد شکارها رو نشون میده
  frequency?: number;
  last_scan_at?: string;
}

export interface TargetResponse {
  status: string;
  data: Target[];
  count: number;
}