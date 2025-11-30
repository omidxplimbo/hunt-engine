import { apiClient } from './client';
import type { UserResponse } from '../types/user';

export const getUsers = async () => {
  const response = await apiClient.get<UserResponse>('/users');
  return response.data;
};

export const createUser = async (data: any) => {
  await apiClient.post('/users', data);
};

export const updateUser = async (id: number, data: any) => {
  await apiClient.patch(`/users/${id}`, data);
};

export const deleteUser = async (id: number) => {
  await apiClient.delete(`/users/${id}`);
};