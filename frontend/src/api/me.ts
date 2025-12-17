import { apiClient } from './client';

export interface MeData {
  id: number;
  username: string;
  role: string;
  createdAt: string;
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


