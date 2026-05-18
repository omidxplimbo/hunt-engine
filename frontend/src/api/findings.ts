import { apiClient } from './client';

import type {
  Finding,
  FindingSeverity,
  FindingStats,
  FindingStatsResponse,
  FindingStatus,
  FindingsResponse,
} from '../types/finding';

export interface FindingFilters {
  status?: FindingStatus | 'all';
  severity?: FindingSeverity | 'all';
  search?: string;
  page?: number;
  limit?: number;
}

const cleanParams = (filters?: FindingFilters) => {
  const params: Record<string, string | number> = {};

  if (!filters) return params;

  if (filters.status && filters.status !== 'all') params.status = filters.status;
  if (filters.severity && filters.severity !== 'all') params.severity = filters.severity;
  if (filters.search) params.search = filters.search;
  if (filters.page) params.page = filters.page;
  if (filters.limit) params.limit = filters.limit;

  return params;
};

export const getTargetFindings = async (
  targetId: number,
  filters?: FindingFilters,
): Promise<FindingsResponse> => {
  const response = await apiClient.get<FindingsResponse>(`/targets/${targetId}/findings`, {
    params: cleanParams(filters),
  });

  return response.data;
};

export const getTargetFindingStats = async (targetId: number): Promise<FindingStats> => {
  const response = await apiClient.get<FindingStatsResponse>(`/targets/${targetId}/findings/stats`);

  return response.data.data;
};

export const getFindings = async (filters?: FindingFilters): Promise<FindingsResponse> => {
  const response = await apiClient.get<FindingsResponse>('/findings', {
    params: cleanParams(filters),
  });

  return response.data;
};

export const updateFindingStatus = async (findingId: number, status: FindingStatus) => {
  const response = await apiClient.patch<{ status: string; data: Finding }>(
    `/findings/${findingId}/status`,
    { status },
  );

  return response.data;
};
