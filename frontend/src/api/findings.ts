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
  source_tool?: string;
  category?: string;
  search?: string;
  page?: number;
  limit?: number;
}

export type FindingExportFormat = 'csv' | 'json';

const cleanParams = (filters?: FindingFilters) => {
  const params: Record<string, string | number> = {};

  if (!filters) return params;

  if (filters.status && filters.status !== 'all') params.status = filters.status;
  if (filters.severity && filters.severity !== 'all') params.severity = filters.severity;
  if (filters.source_tool) params.source_tool = filters.source_tool;
  if (filters.category) params.category = filters.category;
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

export const getFindings = async (filters?: FindingFilters): Promise<FindingsResponse> => {
  const response = await apiClient.get<FindingsResponse>('/findings', {
    params: cleanParams(filters),
  });

  return response.data;
};

export const getTargetFindingStats = async (targetId: number): Promise<FindingStats> => {
  const response = await apiClient.get<FindingStatsResponse>(`/targets/${targetId}/findings/stats`);

  return response.data.data;
};

export const updateFindingStatus = async (
  findingId: number,
  status: FindingStatus,
  triageNote = '',
) => {
  const response = await apiClient.patch<{ status: string; data: Finding }>(
    `/findings/${findingId}/status`,
    {
      status,
      triage_note: triageNote,
    },
  );

  return response.data;
};

export const exportTargetFindings = async (
  targetId: number,
  format: FindingExportFormat,
  filters?: FindingFilters,
): Promise<Blob> => {
  const response = await apiClient.get(`/targets/${targetId}/findings/export`, {
    params: {
      ...cleanParams(filters),
      format,
    },
    responseType: 'blob',
  });

  return response.data;
};
