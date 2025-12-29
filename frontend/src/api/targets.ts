import { apiClient } from './client';
import type { Target, TargetResponse } from '../types/target';
import type { AssetResponse, AssetFilters } from '../types/asset';

// --- بخش مدیریت تارگت‌ها (Targets) ---

// دریافت لیست تارگت‌ها (با صفحه‌بندی)
export const getTargets = async (
  page = 1,
  limit = 50,
  options?: { withPorts?: boolean }
) => {
  const offset = (page - 1) * limit;
  const response = await apiClient.get<TargetResponse>('/targets', {
    params: {
      limit,
      offset,
      ...(options?.withPorts ? { with_ports: true } : {}),
    },
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
  // 👇 optional port scan during discovery
  use_portscan: boolean;
  // 👇 ابزارهای جدید برای فاز اول (Discovery)
  use_cero?: boolean;  // Scrape domain names from SSL certificates
  use_crtsh?: boolean; // Use crt.sh API for subdomain discovery
  use_puredns?: boolean; // Use puredns for bruteforce subdomain discovery
  puredns_wordlists?: string[]; // Selected wordlists for puredns
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
  // 👇 optional port scan during discovery
  use_portscan?: boolean;
  // 👇 ابزارهای جدید برای فاز اول (Discovery)
  use_cero?: boolean;  // Scrape domain names from SSL certificates
  use_crtsh?: boolean; // Use crt.sh API for subdomain discovery
  use_puredns?: boolean; // Use puredns for bruteforce subdomain discovery
  puredns_wordlists?: string[]; // Selected wordlists for puredns
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
  if (filters?.dns_only !== undefined) params.dns_only = filters.dns_only;
  if (filters?.has_ports !== undefined) params.has_ports = filters.has_ports;
  if (filters?.no_cdn !== undefined) params.no_cdn = filters.no_cdn;
  if (filters?.has_cdn !== undefined) params.has_cdn = filters.has_cdn;
  if (filters?.has_waf !== undefined) params.has_waf = filters.has_waf;
  if (filters?.has_cloud !== undefined) params.has_cloud = filters.has_cloud;
  if (filters?.status_code) params.status_code = filters.status_code;
  if (filters?.sources && filters.sources.length > 0) params.sources = filters.sources.join(',');

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
  await apiClient.post<{ status: string; message: string }>(`/targets/${id}/resume`);
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

// --- بخش Export/Import تارگت‌ها ---

// ساختار داده Export/Import
export interface TargetExportData {
  version: string;
  export_date: string;
  targets: TargetExportItem[];
}

export interface TargetExportItem {
  name: string;
  root_domain: string;
  description: string;
  in_scope: boolean;
  frequency: number;
  modules: string[];
  use_alterx: boolean;
  use_waymore: boolean;
  use_portscan: boolean;
  use_cero: boolean;
  use_crtsh: boolean;
  use_puredns: boolean;
  puredns_wordlists: string[];
  assets: AssetExportItem[];
  urls: URLExportItem[];
}

export interface AssetExportItem {
  value: string;
  type: string;
  is_new: boolean;
  is_live: boolean;
  final_url: string;
  status_code: number;
  title: string;
  content_length: number;
  host_ip: string;
  dnsx_ip: string;
  web_server: string;
  cdn_name: string;
  technologies: string;
  body_hash: string;
  header_hash: string;
  response_time_ms: number;
  raw_httpx: string;
  open_ports: string;
  created_at: string;
}

export interface URLExportItem {
  value: string;
  source: string;
  created_at: string;
}

export interface ImportTargetPayload {
  data: TargetExportData;
  skip_existing?: boolean;
}

export interface ImportTargetResponse {
  status: string;
  message: string;
  data: {
    created: Target[];
    skipped: string[];
    errors: string[];
  };
}

// Export تارگت‌ها (اگر targetIds ارسال شود، فقط آن تارگت‌ها export می‌شوند، در غیر این صورت همه)
export const exportTargets = async (targetIds?: number[]): Promise<void> => {
  let response;
  
  if (targetIds && targetIds.length > 0) {
    // ارسال لیست IDها در body
    response = await apiClient.post('/targets/export', 
      { target_ids: targetIds },
      { responseType: 'blob' }
    );
  } else {
    // Export همه تارگت‌ها
    response = await apiClient.get('/targets/export', {
      responseType: 'blob',
    });
  }
  
  // ایجاد لینک دانلود
  const blob = new Blob([response.data], { type: 'application/json' });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `targets_export_${new Date().toISOString().split('T')[0]}.json`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

// Import تارگت‌ها
export const importTargets = async (payload: ImportTargetPayload): Promise<ImportTargetResponse> => {
  const response = await apiClient.post<ImportTargetResponse>('/targets/import', payload);
  return response.data;
};

// Export IPs یک تارگت به صورت فایل txt
export const exportTargetIPs = async (targetId: number, rootDomain: string): Promise<void> => {
  const response = await apiClient.get(`/targets/${targetId}/ips`, {
    responseType: 'blob',
  });
  
  // ایجاد لینک دانلود
  const blob = new Blob([response.data], { type: 'text/plain' });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  // نام فایل: root_domain_ips.txt (جایگزین . با _)
  link.download = `${rootDomain.replace(/\./g, '_')}_ips.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

// Wordlists API
export interface Wordlist {
  path: string;
  name: string;
  type: 'default' | 'custom';
}

export const getWordlists = async (): Promise<Wordlist[]> => {
  const response = await apiClient.get<{ status: string; data: Wordlist[] }>('/wordlists');
  return response.data.data;
};