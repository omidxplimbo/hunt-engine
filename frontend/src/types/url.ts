export interface FoundURL {
  id: number;
  value: string;
  source: string;
  created_at: string;
}

export interface FoundURLResponse {
  status: string;
  data: FoundURL[];
  total_count: number;
  page: number;
}
