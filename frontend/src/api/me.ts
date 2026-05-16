import { apiClient } from './client';

export interface MeData {
  id: number;
  username: string;
  role: string;
  createdAt: string;
  max_concurrent_scans?: number;
}

export const getMe = async () => {
  const res = await apiClient.get<{ status: string; data: MeData }>('/me');
  return res.data.data;
};

export interface ChangePasswordPayload {
  current_password: string;
  new_password: string;
}

export const changeMyPassword = async (payload: ChangePasswordPayload) => {
  const res = await apiClient.post<{ status: string; message: string }>('/me/change-password', payload);
  return res.data;
};

// -----------------------------
// Subfinder provider config (per-user)
// -----------------------------
export interface SubfinderProviderItem {
  provider: string;
  entries: any[];
}

export const getMySubfinderProviders = async () => {
  const res = await apiClient.get<{ status: string; data: { providers: SubfinderProviderItem[] } }>(
    '/me/subfinder/providers'
  );
  return res.data.data.providers;
};

export const putMySubfinderProviders = async (providers: SubfinderProviderItem[]) => {
  const res = await apiClient.put<{ status: string; message: string }>('/me/subfinder/providers', { providers });
  return res.data;
};

export const deleteMySubfinderProvider = async (provider: string) => {
  const p = encodeURIComponent(provider);
  const res = await apiClient.delete<{ status: string; message: string }>(`/me/subfinder/providers/${p}`);
  return res.data;
};
