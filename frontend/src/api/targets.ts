import { apiClient } from './client';
import type { Target, TargetResponse } from '../types/target';
import type { AssetResponse, AssetFilters } from '../types/asset';

// --- بخش مدیریت تارگت‌ها (Targets) ---

// دریافت لیست تارگت‌ها (با صفحه‌بندی)
export const getTargets = async (page = 1, limit = 50) => {
  const offset = (page - 1) * limit;
  const response = await apiClient.get<TargetResponse>('/targets', {
    params: { limit, offset },
  });
  return response.data;
};

// ساخت تارگت جدید
export interface CreateTargetPayload {
  name: string;
  root_domain: string;
  description?: string;
  frequency: number;
  modules: string[];
  use_alterx: boolean;
  // 👇 اضافه شد
  use_waymore: boolean;
}

export const createTarget = async (payload: CreateTargetPayload) => {
  const response = await apiClient.post<{ status: string; data: Target; message: string }>(
    '/targets',
    payload
  );
  return response.data;
};


export interface UpdateTargetPayload {
  name?: string;
  description?: string;
  frequency?: number;
  in_scope?: boolean;
  modules?: string[];
  use_alterx?: boolean;
  // 👇 اضافه شد
  use_waymore?: boolean;
}

export const updateTarget = async (id: number, payload: UpdateTargetPayload) => {
  const response = await apiClient.patch<{ status: string; data: Target }>(`/targets/${id}`, payload);
  return response.data;
};

// حذف تارگت
export const deleteTarget = async (id: number) => {
  await apiClient.delete(`/targets/${id}`);
};

// --- بخش مدیریت دارایی‌ها (Assets) ---

export const getTargetAssets = async (
  targetId: number, 
  page = 1, 
  limit = 50,
  filters?: AssetFilters,
  sortBy: string = 'value',
  order: 'asc' | 'desc' = 'asc'
) => {
  const offset = (page - 1) * limit;
  
  const params: any = { limit, offset, sort_by: sortBy, order };
  
  if (filters?.is_live !== undefined) params.is_live = filters.is_live;
  if (filters?.is_new !== undefined) params.is_new = filters.is_new;
  if (filters?.search) params.search = filters.search;
  if (filters?.has_httpx !== undefined) params.has_httpx = filters.has_httpx;
  
  // 👇 ارسال پارامتر جدید به بک‌اند
  if (filters?.dns_only !== undefined) params.dns_only = filters.dns_only;

  const response = await apiClient.get<AssetResponse>(`/targets/${targetId}/assets`, {
    params,
  });
  
  return response.data;
};

// دریافت جزئیات یک تارگت خاص (برای هدر صفحه دارایی‌ها)
export const getTargetDetails = async (targetId: number) => {
  const response = await apiClient.get<{ status: string; data: Target }>(`/targets/${targetId}`);
  return response.data.data;
};
export const stopTarget = async (id: number) => {
  await apiClient.post(`/targets/${id}/stop`);
};

// 👇 تابع شروع مجدد (Resume) - فاز ۱ را دوباره تریگر می‌کند
export const startDiscovery = async (id: number) => {
  await apiClient.post<{ status: string; message: string }>(`/targets/${id}/discovery`);
};

// --- بخش مدیریت URLهای کراول شده (فاز ۳) ---

export interface FoundURL {
  id: number;
  value: string;
  source: string;
  created_at: string;
}

export interface FoundURLResponse {
  status: string;
  data: FoundURL[];
  total_count: number;
}

export const getTargetURLs = async (
  targetId: number,
  page = 1,
  limit = 50,
  search = "",
  onlyJs = false,
  sortBy = 'created_at',
  order: 'asc' | 'desc' = 'desc',
  sources: string[] = [] // 👈 پارامتر جدید
) => {
  const offset = (page - 1) * limit;
  
  // ساخت پارامترها
  const params: any = { 
      limit, 
      offset, 
      search,
      only_js: onlyJs,
      sort_by: sortBy,
      order: order
  };

  // اگر لیستی از سورس‌ها انتخاب شده بود، اضافه کن
  if (sources.length > 0) {
      params.sources = sources.join(',');
  }

  const response = await apiClient.get<FoundURLResponse>(`/targets/${targetId}/urls`, { params });
  return response.data;
};