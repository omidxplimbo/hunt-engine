import { apiClient } from './client';

export interface SystemConfig {
  key: string;
  value: string;
}

export const getSystemConfig = async () => {
  const response = await apiClient.get<SystemConfig[]>('/config');
  return response.data;
};

export const updateSystemConfig = async (key: string, value: string) => {
  const response = await apiClient.put<{ status: string; message: string }>(`/config/${key}`, { value });
  return response.data;
};

export interface QueueItem {
    index: number;
    payload: string;
}

export const getQueue = async () => {
    const response = await apiClient.get<QueueItem[]>('/queue');
    return response.data;
};

export const removeFromQueue = async (index: number) => {
    const response = await apiClient.delete<{ status: string; message: string }>(`/queue/${index}`);
    return response.data;
};

export const clearQueue = async () => {
    const response = await apiClient.delete<{ status: string; message: string }>('/queue');
    return response.data;
};

export const moveQueueItem = async (index: number, direction: 'top' | 'bottom') => {
    const response = await apiClient.post<{ status: string; message: string }>(`/queue/${index}/move-${direction}`);
    return response.data;
};