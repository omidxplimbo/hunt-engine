import { apiClient } from './client';

export interface LoginPayload {
  username: string;
  password: string;
}

export interface AuthResponse {
  token: string;
  username: string;
  role: string;
}

export const loginUser = async (payload: LoginPayload) => {
  const response = await apiClient.post<AuthResponse>('/auth/login', payload);
  return response.data;
};