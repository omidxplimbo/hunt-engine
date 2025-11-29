import { apiClient } from './client';
import type { Target,TargetResponse } from '../types/target';
import type { AssetResponse, AssetFilters } from '../types/asset';

export const getTargets = async (page = 1, limit = 50) => {
  // محاسبه آفست بر اساس شماره صفحه
  const offset = (page - 1) * limit;
  
  const response = await apiClient.get<TargetResponse>('/targets', {
    params: { limit, offset },
  });
  
  return response.data;
};

export interface CreateTargetPayload {
  name: string;
  root_domain: string;
  description?: string;
  frequency: number;
  modules: string[];
}

export const createTarget = async (payload: CreateTargetPayload) => {
  const response = await apiClient.post<{ status: string; data: Target; message: string }>(
    '/targets',
    payload
  );
  return response.data;
};

export const getTargetAssets = async (
  targetId: number, 
  page = 1, 
  limit = 50,
  filters?: AssetFilters
) => {
  const offset = (page - 1) * limit;
  
  // ساخت پارامترهای کوئری
  const params: any = { limit, offset };
  if (filters?.is_live !== undefined) params.is_live = filters.is_live;
  if (filters?.is_new !== undefined) params.is_new = filters.is_new;
  if (filters?.search) params.search = filters.search;

  const response = await apiClient.get<AssetResponse>(`/targets/${targetId}/assets`, {
    params,
  });
  
  return response.data;
};

// 👇 تابع جدید برای گرفتن جزئیات خود تارگت (برای هدر صفحه)
export const getTargetDetails = async (targetId: number) => {
  // ما از همون تایپ TargetResponse استفاده می‌کنیم ولی دیتای تکی برمی‌گردونه
  // برای سادگی اینجا any می‌ذاریم یا اینترفیس جدید می‌سازیم. فعلا any:
  const response = await apiClient.get<{ status: string; data: any }>(`/targets/${targetId}`);
  return response.data.data;
};