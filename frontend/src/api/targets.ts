import { apiClient } from './client';
import type { Target, Asset, TargetDetails, TargetResponse } from '../types';
import type { FoundURLResponse } from '../types/url';



// --- Types ---

export interface CreateTargetPayload {
  name: string;
  root_domain: string;
  description: string;
  frequency: number;
  modules: string[];
  use_alterx: boolean;
  use_waymore: boolean;
  use_portscan: boolean;
  use_cero: boolean;
  use_crtsh: boolean;
  use_puredns: boolean;
  use_abusedb: boolean; use_amass: boolean; use_nuclei: boolean; nuclei_profile: string;
  puredns_wordlists: string[];
}

export interface UpdateTargetPayload {
    name?: string;
    root_domain?: string;
    description?: string;
    frequency?: number;
    modules?: string[];
    use_alterx?: boolean;
    use_waymore?: boolean;
    use_portscan?: boolean;
    use_cero?: boolean;
    use_crtsh?: boolean;
    use_puredns?: boolean;
    use_abusedb?: boolean; use_amass?: boolean; use_nuclei?: boolean; nuclei_profile?: string;
    puredns_wordlists?: string[];
    in_scope?: boolean;
}

// --- API Functions ---

// دریافت لیست تارگت‌ها (خلاصه)
export const getTargets = async (page = 1, limit = 50) => {
  const params = { page, limit };
  const response = await apiClient.get<TargetResponse>('/targets', { params });
  return response.data;
};

// ایجاد تارگت جدید
export const createTarget = async (data: CreateTargetPayload) => {
  const response = await apiClient.post<Target>('/targets', data);
  return response.data;
};

// ویرایش تارگت
export const updateTarget = async (id: number, data: UpdateTargetPayload) => {
    const response = await apiClient.patch<Target>(`/targets/${id}`, data);
    return response.data;
};

// حذف تارگت
export const deleteTarget = async (id: number) => {
  await apiClient.delete(`/targets/${id}`);
};

// توقف اسکن
export const stopTarget = async (id: number) => {
    await apiClient.post(`/targets/${id}/stop`);
};

// شروع دستی اسکن (Discovery)
export const startDiscovery = async (id: number) => {
    await apiClient.post(`/targets/${id}/scan`);
};


// دریافت جزئیات کامل یک تارگت
export const getTargetDetails = async (id: number) => {
  const response = await apiClient.get<{ status: string; data: TargetDetails }>(`/targets/${id}`);
  return response.data.data;
};

// --- بخش Assets ---

export interface AssetFilters {
  is_live?: boolean;
  is_new?: boolean;
  search?: string;
  has_httpx?: boolean;
  dns_only?: boolean;
  has_ports?: boolean;
  no_cdn?: boolean;
  has_cdn?: boolean;
  has_waf?: boolean;
  has_cloud?: boolean;
  status_code?: string;
  sources?: string[]; // 👈 فیلتر جدید
}

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

  const response = await apiClient.get<{ data: Asset[]; total: number }>(
    `/targets/${targetId}/assets`,
    { params }
  );
  return response.data;
};

// --- بخش URLs ---

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

// Download Assets
export const downloadAssets = async (
  targetId: number,
  targetName: string,
  filters?: AssetFilters,
  sortBy: string = 'value',
  order: 'asc' | 'desc' = 'asc'
): Promise<void> => {
  const params: any = { sort_by: sortBy, order };
  const parts = [targetName.replace(/[^a-zA-Z0-9]/g, '_'), 'assets'];

  if (filters?.is_live !== undefined) {
      params.is_live = filters.is_live;
      parts.push(filters.is_live ? 'live' : 'dead');
  }
  if (filters?.is_new !== undefined) {
      params.is_new = filters.is_new;
      if (filters.is_new) parts.push('new');
  }
  if (filters?.search) {
      params.search = filters.search;
      parts.push(`search_${filters.search.replace(/[^a-zA-Z0-9]/g, '_')}`);
  }
  if (filters?.has_httpx !== undefined) {
      params.has_httpx = filters.has_httpx;
      if (filters.has_httpx) parts.push('web');
  }
  if (filters?.dns_only !== undefined) {
      params.dns_only = filters.dns_only;
      if (filters.dns_only) parts.push('dns');
  }
  if (filters?.has_ports !== undefined) {
      params.has_ports = filters.has_ports;
      if (filters.has_ports) parts.push('ports');
  }
  if (filters?.no_cdn !== undefined) {
      params.no_cdn = filters.no_cdn;
      if (filters.no_cdn) parts.push('nocdn');
  }
  if (filters?.has_cdn !== undefined) {
      params.has_cdn = filters.has_cdn;
      if (filters.has_cdn) parts.push('cdn');
  }
  if (filters?.has_waf !== undefined) {
      params.has_waf = filters.has_waf;
      if (filters.has_waf) parts.push('waf');
  }
  if (filters?.has_cloud !== undefined) {
      params.has_cloud = filters.has_cloud;
      if (filters.has_cloud) parts.push('cloud');
  }
  if (filters?.status_code) {
      params.status_code = filters.status_code;
      parts.push(`status_${filters.status_code}`);
  }
  if (filters?.sources && filters.sources.length > 0) {
      params.sources = filters.sources.join(',');
      parts.push(`sources_${filters.sources.join('_')}`);
  }

  const response = await apiClient.get(`/targets/${targetId}/assets/download`, {
    params,
    responseType: 'blob',
  });

  const blob = new Blob([response.data], { type: 'text/plain' });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${parts.join('_')}.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

// Download URLs
export const downloadURLs = async (
  targetId: number,
  targetName: string,
  search = "",
  onlyJs = false,
  sortBy = 'created_at',
  order: 'asc' | 'desc' = 'desc',
  sources: string[] = []
): Promise<void> => {
  const params: any = {
      search,
      only_js: onlyJs,
      sort_by: sortBy,
      order: order
  };
  const parts = [targetName.replace(/[^a-zA-Z0-9]/g, '_'), 'urls'];

  if (search) {
      parts.push(`search_${search.replace(/[^a-zA-Z0-9]/g, '_')}`);
  }
  if (onlyJs) {
      parts.push('js');
  }

  if (sources.length > 0) {
      params.sources = sources.join(',');
      parts.push(`sources_${sources.join('_')}`);
  }

  const response = await apiClient.get(`/targets/${targetId}/urls/download`, {
    params,
    responseType: 'blob',
  });

  const blob = new Blob([response.data], { type: 'text/plain' });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = `${parts.join('_')}.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

export const downloadTargetPDFReport = async (targetId: number, targetName = 'target') => {
  const response = await apiClient.get(`/targets/${targetId}/report.pdf`, {
    responseType: 'blob',
  });

  const disposition = String(response.headers?.['content-disposition'] || '');
  const match = disposition.match(/filename="?([^";]+)"?/i);
  const fallback = `hunt-${String(targetName).replace(/[^a-z0-9._-]+/gi, '-')}-report.pdf`;
  const filename = match?.[1] || fallback;

  const blob = new Blob([response.data], { type: 'application/pdf' });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');

  link.href = url;
  link.download = filename;

  document.body.appendChild(link);
  link.click();
  link.remove();

  window.URL.revokeObjectURL(url);
};

