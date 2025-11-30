import { apiClient } from './client';

export interface CountItem {
  name: string;
  count: number;
}

export interface DashboardStats {
  total_targets: number;
  total_assets: number;
  live_assets: number;
  fresh_assets: number;
  assets_by_status: CountItem[];
  top_technologies: CountItem[];
}

export const getDashboardStats = async () => {
  const response = await apiClient.get<{ status: string; data: DashboardStats }>('/dashboard/stats');
  return response.data.data;
};