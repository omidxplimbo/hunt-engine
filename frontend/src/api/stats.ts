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
  top_open_ports: CountItem[];
}

export interface SystemStats {
  cpu_usage: number;
  memory_total: number;
  memory_used: number;
  memory_usage: number;
  num_goroutine: number;
}

export interface ProcessInfo {
  target_id: number;
  target_name: string;
  command: string;
  duration: string;
  pid: number;
}

export interface MonitorResponse {
  stats: SystemStats;
  processes: ProcessInfo[];
}

export const getDashboardStats = async () => {
  const response = await apiClient.get<{ status: string; data: DashboardStats }>('/dashboard/stats');
  return response.data.data;
};

export const getMonitorStats = async () => {
  const response = await apiClient.get<{ status: string; data: MonitorResponse }>('/monitor/stats');
  return response.data.data;
};