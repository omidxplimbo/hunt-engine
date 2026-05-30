import { apiClient } from "./client";
import type { Target, Asset, TargetDetails, TargetResponse } from "../types";
import type { FoundURLResponse } from "../types/url";

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
  use_abusedb: boolean;
  use_amass: boolean;
  use_nuclei: boolean;
  nuclei_profile: string;
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
  use_abusedb?: boolean;
  use_amass?: boolean;
  use_nuclei?: boolean;
  nuclei_profile?: string;
  puredns_wordlists?: string[];
  in_scope?: boolean;
}

// --- API Functions ---

// دریافت لیست تارگت‌ها (خلاصه)
export const getTargets = async (page = 1, limit = 50) => {
  const params = { page, limit };
  const response = await apiClient.get<TargetResponse>("/targets", { params });
  return response.data;
};

// ایجاد تارگت جدید
export const createTarget = async (data: CreateTargetPayload) => {
  const response = await apiClient.post<Target>("/targets", data);
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
  const response = await apiClient.get<{ status: string; data: TargetDetails }>(
    `/targets/${id}`,
  );
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
  sortBy: string = "value",
  order: "asc" | "desc" = "asc",
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
  if (filters?.sources && filters.sources.length > 0)
    params.sources = filters.sources.join(",");

  const response = await apiClient.get<{ data: Asset[]; total: number }>(
    `/targets/${targetId}/assets`,
    { params },
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
  sortBy = "created_at",
  order: "asc" | "desc" = "desc",
  sources: string[] = [], // 👈 پارامتر جدید
) => {
  const offset = (page - 1) * limit;

  // ساخت پارامترها
  const params: any = {
    limit,
    offset,
    search,
    only_js: onlyJs,
    sort_by: sortBy,
    order: order,
  };

  // اگر لیستی از سورس‌ها انتخاب شده بود، اضافه کن
  if (sources.length > 0) {
    params.sources = sources.join(",");
  }

  const response = await apiClient.get<FoundURLResponse>(
    `/targets/${targetId}/urls`,
    { params },
  );
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
    response = await apiClient.post(
      "/targets/export",
      { target_ids: targetIds },
      { responseType: "blob" },
    );
  } else {
    // Export همه تارگت‌ها
    response = await apiClient.get("/targets/export", {
      responseType: "blob",
    });
  }

  // ایجاد لینک دانلود
  const blob = new Blob([response.data], { type: "application/json" });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `targets_export_${new Date().toISOString().split("T")[0]}.json`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

// Import تارگت‌ها
export const importTargets = async (
  payload: ImportTargetPayload,
): Promise<ImportTargetResponse> => {
  const response = await apiClient.post<ImportTargetResponse>(
    "/targets/import",
    payload,
  );
  return response.data;
};

// Export IPs یک تارگت به صورت فایل txt
export const exportTargetIPs = async (
  targetId: number,
  rootDomain: string,
): Promise<void> => {
  const response = await apiClient.get(`/targets/${targetId}/ips`, {
    responseType: "blob",
  });

  // ایجاد لینک دانلود
  const blob = new Blob([response.data], { type: "text/plain" });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  // نام فایل: root_domain_ips.txt (جایگزین . با _)
  link.download = `${rootDomain.replace(/\./g, "_")}_ips.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

// Wordlists API
export interface Wordlist {
  path: string;
  name: string;
  type: "default" | "custom";
}

export const getWordlists = async (): Promise<Wordlist[]> => {
  const response = await apiClient.get<{ status: string; data: Wordlist[] }>(
    "/wordlists",
  );
  return response.data.data;
};

// Download Assets
export const downloadAssets = async (
  targetId: number,
  targetName: string,
  filters?: AssetFilters,
  sortBy: string = "value",
  order: "asc" | "desc" = "asc",
): Promise<void> => {
  const params: any = { sort_by: sortBy, order };
  const parts = [targetName.replace(/[^a-zA-Z0-9]/g, "_"), "assets"];

  if (filters?.is_live !== undefined) {
    params.is_live = filters.is_live;
    parts.push(filters.is_live ? "live" : "dead");
  }
  if (filters?.is_new !== undefined) {
    params.is_new = filters.is_new;
    if (filters.is_new) parts.push("new");
  }
  if (filters?.search) {
    params.search = filters.search;
    parts.push(`search_${filters.search.replace(/[^a-zA-Z0-9]/g, "_")}`);
  }
  if (filters?.has_httpx !== undefined) {
    params.has_httpx = filters.has_httpx;
    if (filters.has_httpx) parts.push("web");
  }
  if (filters?.dns_only !== undefined) {
    params.dns_only = filters.dns_only;
    if (filters.dns_only) parts.push("dns");
  }
  if (filters?.has_ports !== undefined) {
    params.has_ports = filters.has_ports;
    if (filters.has_ports) parts.push("ports");
  }
  if (filters?.no_cdn !== undefined) {
    params.no_cdn = filters.no_cdn;
    if (filters.no_cdn) parts.push("nocdn");
  }
  if (filters?.has_cdn !== undefined) {
    params.has_cdn = filters.has_cdn;
    if (filters.has_cdn) parts.push("cdn");
  }
  if (filters?.has_waf !== undefined) {
    params.has_waf = filters.has_waf;
    if (filters.has_waf) parts.push("waf");
  }
  if (filters?.has_cloud !== undefined) {
    params.has_cloud = filters.has_cloud;
    if (filters.has_cloud) parts.push("cloud");
  }
  if (filters?.status_code) {
    params.status_code = filters.status_code;
    parts.push(`status_${filters.status_code}`);
  }
  if (filters?.sources && filters.sources.length > 0) {
    params.sources = filters.sources.join(",");
    parts.push(`sources_${filters.sources.join("_")}`);
  }

  const response = await apiClient.get(`/targets/${targetId}/assets/download`, {
    params,
    responseType: "blob",
  });

  const blob = new Blob([response.data], { type: "text/plain" });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `${parts.join("_")}.txt`;
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
  sortBy = "created_at",
  order: "asc" | "desc" = "desc",
  sources: string[] = [],
): Promise<void> => {
  const params: any = {
    search,
    only_js: onlyJs,
    sort_by: sortBy,
    order: order,
  };
  const parts = [targetName.replace(/[^a-zA-Z0-9]/g, "_"), "urls"];

  if (search) {
    parts.push(`search_${search.replace(/[^a-zA-Z0-9]/g, "_")}`);
  }
  if (onlyJs) {
    parts.push("js");
  }

  if (sources.length > 0) {
    params.sources = sources.join(",");
    parts.push(`sources_${sources.join("_")}`);
  }

  const response = await apiClient.get(`/targets/${targetId}/urls/download`, {
    params,
    responseType: "blob",
  });

  const blob = new Blob([response.data], { type: "text/plain" });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = `${parts.join("_")}.txt`;
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
  window.URL.revokeObjectURL(url);
};

export const downloadTargetPDFReport = async (
  targetId: number,
  targetName = "target",
) => {
  const response = await apiClient.get(`/targets/${targetId}/report.pdf`, {
    responseType: "blob",
  });

  const disposition = String(response.headers?.["content-disposition"] || "");
  const match = disposition.match(/filename="?([^";]+)"?/i);
  const fallback = `hunt-${String(targetName).replace(/[^a-z0-9._-]+/gi, "-")}-report.pdf`;
  const filename = match?.[1] || fallback;

  const blob = new Blob([response.data], { type: "application/pdf" });
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");

  link.href = url;
  link.download = filename;

  document.body.appendChild(link);
  link.click();
  link.remove();

  window.URL.revokeObjectURL(url);
};

// --- AI Analysis ---

export interface TargetAIAnalysis {
  id: number;
  target_id: number;
  scope: string;
  source: string;
  provider: string;
  model: string;
  prompt_version: string;
  status: string;
  title: string;
  summary: string;
  input_digest: string;
  input_json?: any;
  output_json?: any;
  error_message?: string;
  started_at?: string;
  completed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface TargetAIAnalysesResponse {
  status: string;
  data: TargetAIAnalysis[];
  count: number;
  total_count: number;
  page: number;
}

export const getTargetAIAnalyses = async (targetId: number, limit = 5) => {
  const response = await apiClient.get<TargetAIAnalysesResponse>(
    `/targets/${targetId}/ai/analyses`,
    {
      params: { limit },
    },
  );
  return response.data;
};

export const generateTargetAIAnalysis = async (
  targetId: number,
  useLLM = false,
) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAIAnalysis;
  }>(`/targets/${targetId}/ai/analyses/generate`, { use_llm: useLLM });

  return response.data.data;
};

// --- AI Recommendations ---

export interface TargetAIRecommendation {
  id: number;
  target_id: number;
  finding_id?: number | null;
  analysis_id?: number | null;
  created_by_user_id?: number | null;
  source: string;
  recommendation_type: string;
  priority: string;
  confidence: string;
  status: string;
  title: string;
  description: string;
  rationale: string;
  evidence_json?: any;
  action_json?: any;
  accepted_at?: string | null;
  accepted_by_user_id?: number | null;
  created_at: string;
  updated_at: string;
}

export interface TargetAIRecommendationsResponse {
  status: string;
  data: TargetAIRecommendation[];
  count: number;
  total_count: number;
  page: number;
}

export const getTargetAIRecommendations = async (
  targetId: number,
  limit = 20,
) => {
  const response = await apiClient.get<TargetAIRecommendationsResponse>(
    `/targets/${targetId}/ai/recommendations`,
    {
      params: { limit },
    },
  );
  return response.data;
};

export const generateTargetAIRecommendations = async (targetId: number) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAIRecommendation[];
    count: number;
  }>(`/targets/${targetId}/ai/recommendations/generate`);

  return response.data;
};


// --- Target Policy ---

export interface TargetPolicy {
  id: number;
  target_id: number;
  created_by_user_id?: number | null;
  platform_name: string;
  program_url: string;
  in_scope_patterns: string[] | string;
  out_of_scope_patterns: string[] | string;
  allowed_test_types: string[] | string;
  disallowed_test_types: string[] | string;
  max_test_intensity: string;
  rate_limit_notes: string;
  auth_required: boolean;
  safe_testing_notes: string;
  reporting_preferences: string;
  business_context: string;
  asset_criticality_default: string;
  created_at: string;
  updated_at: string;
}

export interface TargetPolicyPayload {
  platform_name: string;
  program_url: string;
  in_scope_patterns: string[];
  out_of_scope_patterns: string[];
  allowed_test_types: string[];
  disallowed_test_types: string[];
  max_test_intensity: string;
  rate_limit_notes: string;
  auth_required: boolean;
  safe_testing_notes: string;
  reporting_preferences: string;
  business_context: string;
  asset_criticality_default: string;
}

export const getTargetPolicy = async (targetId: number) => {
  const response = await apiClient.get<{
    status: string;
    data: TargetPolicy | null;
  }>(`/targets/${targetId}/policy`);

  return response.data.data;
};

export const putTargetPolicy = async (
  targetId: number,
  payload: TargetPolicyPayload,
) => {
  const response = await apiClient.put<{
    status: string;
    data: TargetPolicy;
  }>(`/targets/${targetId}/policy`, payload);

  return response.data.data;
};

export const deleteTargetPolicy = async (targetId: number) => {
  const response = await apiClient.delete<{
    status: string;
    message: string;
  }>(`/targets/${targetId}/policy`);

  return response.data;
};

// --- Agent Runs ---

export interface TargetAgentRun {
  id: number;
  target_id: number;
  created_by_user_id?: number | null;
  agent_type: string;
  provider: string;
  model: string;
  status: string;
  source: string;
  policy_status: string;
  input_digest: string;
  input_json?: any;
  output_json?: any;
  error_message?: string;
  started_at?: string | null;
  completed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface TargetAgentRunsResponse {
  status: string;
  data: TargetAgentRun[];
  count: number;
  total_count: number;
  page: number;
}

export const getTargetAgentRuns = async (targetId: number, limit = 20) => {
  const response = await apiClient.get<TargetAgentRunsResponse>(
    `/targets/${targetId}/agents/runs`,
    {
      params: { limit },
    },
  );

  return response.data;
};
