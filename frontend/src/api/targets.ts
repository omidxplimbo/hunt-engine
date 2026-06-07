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
  use_gau: boolean;
  use_katana: boolean;
  use_virustotal: boolean;
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
  use_gau?: boolean;
  use_katana?: boolean;
  use_virustotal?: boolean;
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

// شروع دوباره از صفر: checkpoint/temp state پاک می‌شود ولی assets/findings قبلی حذف نمی‌شوند
export const restartTargetScan = async (id: number, force = false) => {
  await apiClient.post(`/targets/${id}/restart${force ? "?force=true" : ""}`);
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
  use_gau: boolean;
  use_katana: boolean;
  use_virustotal: boolean;
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

export const runTargetTriageAgent = async (targetId: number) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentRun;
  }>(`/targets/${targetId}/agents/triage/run`);

  return response.data.data;
};


export const runTargetSummaryAgent = async (targetId: number) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentRun;
  }>(`/targets/${targetId}/agents/summary/run`);

  return response.data.data;
};


export const runTargetReportAgent = async (targetId: number) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentRun;
  }>(`/targets/${targetId}/agents/report/run`);

  return response.data.data;
};

// -----------------------------
// Agent Actions - v3.7.0 foundation
// -----------------------------
export type AgentActionStatus =
  | "proposed"
  | "approved"
  | "rejected"
  | "executed"
  | "failed"
  | "blocked_by_policy";

export type AgentActionPolicyStatus =
  | "allowed"
  | "warning"
  | "blocked"
  | "unknown";

export interface TargetAgentAction {
  id: number;
  created_at: string;
  updated_at: string;
  target_id: number;
  agent_run_id?: number | null;
  created_by_user_id?: number | null;
  approved_by_user_id?: number | null;
  action_type: string;
  title: string;
  description: string;
  status: AgentActionStatus | string;
  policy_status: AgentActionPolicyStatus | string;
  risk_level: string;
  safety_level: number;
  test_level: number;
  autonomy_level: number;
  requested_by_agent: boolean;
  requires_approval: boolean;
  input_json: any;
  output_json: any;
  policy_check_json: any;
  error_message?: string;
  reject_reason?: string;
  approved_at?: string | null;
  rejected_at?: string | null;
  executed_at?: string | null;
  completed_at?: string | null;
}

export interface TargetAgentActionsResponse {
  status: string;
  data: TargetAgentAction[];
  count: number;
  total_count: number;
  page: number;
}

export interface ProposeAgentActionPayload {
  agent_run_id?: number | null;
  action_type: string;
  title: string;
  description?: string;
  risk_level?: string;
  safety_level?: number;
  test_level?: number;
  autonomy_level?: number;
  requested_by_agent?: boolean;
  requires_approval?: boolean;
  input_json?: Record<string, any>;
}

export const getTargetAgentActions = async (targetId: number, limit = 30) => {
  const response = await apiClient.get<TargetAgentActionsResponse>(
    `/targets/${targetId}/agent-actions`,
    { params: { limit } },
  );

  return response.data;
};

export const proposeTargetAgentAction = async (
  targetId: number,
  payload: ProposeAgentActionPayload,
) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentAction;
  }>(`/targets/${targetId}/agent-actions/propose`, payload);

  return response.data.data;
};

export const approveTargetAgentAction = async (
  targetId: number,
  actionId: number,
  reason = "",
) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentAction;
  }>(`/targets/${targetId}/agent-actions/${actionId}/approve`, { reason });

  return response.data.data;
};

export const rejectTargetAgentAction = async (
  targetId: number,
  actionId: number,
  reason = "",
) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentAction;
  }>(`/targets/${targetId}/agent-actions/${actionId}/reject`, { reason });

  return response.data.data;
};


// -----------------------------
// Agent Chat - v3.7.0 foundation
// -----------------------------
export interface TargetAgentChatSession {
  id: number;
  created_at: string;
  updated_at: string;
  target_id: number;
  created_by_user_id?: number | null;
  title: string;
  status: string;
  context_json: any;
  last_message_at?: string | null;
}

export interface TargetAgentChatMessage {
  id: number;
  created_at: string;
  updated_at: string;
  session_id: number;
  target_id: number;
  created_by_user_id?: number | null;
  role: "user" | "assistant" | "system" | "tool" | string;
  message_type: string;
  content: string;
  input_json: any;
  output_json: any;
  agent_run_id?: number | null;
  agent_action_id?: number | null;
}

export interface TargetAgentChatSessionsResponse {
  status: string;
  data: TargetAgentChatSession[];
  count: number;
  total_count: number;
  page: number;
}

export interface TargetAgentChatMessagesResponse {
  status: string;
  data: TargetAgentChatMessage[];
  count: number;
}

export interface CreateAgentChatMessageResponse {
  status: string;
  data: {
    user_message: TargetAgentChatMessage;
    assistant_message: TargetAgentChatMessage;
    proposed_actions: TargetAgentAction[];
  };
}

export const getTargetAgentChatSessions = async (
  targetId: number,
  limit = 20,
) => {
  const response = await apiClient.get<TargetAgentChatSessionsResponse>(
    `/targets/${targetId}/agent-chat/sessions`,
    { params: { limit } },
  );

  return response.data;
};

export const createTargetAgentChatSession = async (
  targetId: number,
  title = "Attack Surface Chat",
) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetAgentChatSession;
  }>(`/targets/${targetId}/agent-chat/sessions`, { title });

  return response.data.data;
};

export const getTargetAgentChatMessages = async (
  targetId: number,
  sessionId: number,
) => {
  const response = await apiClient.get<TargetAgentChatMessagesResponse>(
    `/targets/${targetId}/agent-chat/sessions/${sessionId}/messages`,
  );

  return response.data;
};

export const createTargetAgentChatMessage = async (
  targetId: number,
  sessionId: number,
  content: string,
) => {
  const response = await apiClient.post<CreateAgentChatMessageResponse>(
    `/targets/${targetId}/agent-chat/sessions/${sessionId}/messages`,
    { content },
  );

  return response.data.data;
};

// Agent Action Dispatcher - v3.7.0 foundation
export const dispatchTargetAgentAction = async (
  targetId: number,
  actionId: number,
  note = "",
) => {
  const response = await apiClient.post<{
    status: string;
    data: {
      action: TargetAgentAction;
      dispatch: any;
    };
  }>(`/targets/${targetId}/agent-actions/${actionId}/dispatch`, {
    dry_run: true,
    note,
  });

  return response.data.data;
};

// -----------------------------
// Analysis cleanup / delete endpoints
// -----------------------------
export const deleteTargetAIAnalysis = async (
  targetId: number,
  analysisId: number,
) => {
  await apiClient.delete(`/targets/${targetId}/ai/analyses/${analysisId}`);
};

export const deleteTargetAIRecommendation = async (
  targetId: number,
  recommendationId: number,
) => {
  await apiClient.delete(
    `/targets/${targetId}/ai/recommendations/${recommendationId}`,
  );
};

export const deleteTargetAgentRun = async (
  targetId: number,
  runId: number,
) => {
  await apiClient.delete(`/targets/${targetId}/agents/runs/${runId}`);
};

export const deleteTargetAgentAction = async (
  targetId: number,
  actionId: number,
) => {
  await apiClient.delete(`/targets/${targetId}/agent-actions/${actionId}`);
};

export const deleteTargetAgentChatSession = async (
  targetId: number,
  sessionId: number,
) => {
  await apiClient.delete(
    `/targets/${targetId}/agent-chat/sessions/${sessionId}`,
  );
};

// -----------------------------
// Safe Bug Testing - v3.8.0 foundation
// -----------------------------
export interface TargetBugTestRun {
  id: number;
  created_at: string;
  updated_at: string;
  target_id: number;
  created_by_user_id?: number | null;
  agent_action_id?: number | null;
  profile: string;
  status: string;
  policy_status: string;
  safety_level: number;
  test_level: number;
  bug_types?: any;
  owasp_refs?: any;
  input_json?: any;
  output_json?: any;
  policy_check_json?: any;
  error_message?: string;
  started_at?: string | null;
  completed_at?: string | null;
}

export interface TargetBugTestResult {
  id: number;
  created_at: string;
  updated_at: string;
  run_id: number;
  target_id: number;
  asset_id?: number | null;
  url_id?: number | null;
  finding_id?: number | null;
  pattern_id?: number | null;
  pattern_key?: string;
  bug_type: string;
  test_name: string;
  status: string;
  confidence: string;
  severity_hint: string;
  evidence_json?: any;
  owasp_refs?: any;
  tags?: any;
}

export interface TargetBugTestRunsResponse {
  status: string;
  data: TargetBugTestRun[];
  count: number;
  total_count: number;
  page: number;
}

export interface TargetBugTestResultsResponse {
  status: string;
  data: TargetBugTestResult[];
  count: number;
  total_count: number;
  page: number;
}

export const getTargetBugTestRuns = async (targetId: number, limit = 20) => {
  const response = await apiClient.get<TargetBugTestRunsResponse>(
    `/targets/${targetId}/bug-tests/runs`,
    {
      params: { limit },
    },
  );

  return response.data;
};

export const createTargetBugTestRun = async (
  targetId: number,
  payload: {
    profile: string;
    bug_types: string[];
    owasp_refs?: string[];
    safety_level?: number;
    test_level?: number;
    input_json?: any;
  },
) => {
  const response = await apiClient.post<{
    status: string;
    data: TargetBugTestRun;
  }>(`/targets/${targetId}/bug-tests/runs`, payload);

  return response.data.data;
};

export const getTargetBugTestResults = async (
  targetId: number,
  limit = 50,
  runId?: number | null,
) => {
  const response = await apiClient.get<TargetBugTestResultsResponse>(
    `/targets/${targetId}/bug-tests/results`,
    {
      params: {
        limit,
        ...(runId ? { run_id: runId } : {}),
      },
    },
  );

  return response.data;
};

export const deleteTargetBugTestRun = async (
  targetId: number,
  runId: number,
) => {
  await apiClient.delete(`/targets/${targetId}/bug-tests/runs/${runId}`);
};

export const deleteTargetBugTestResult = async (
  targetId: number,
  resultId: number,
) => {
  await apiClient.delete(`/targets/${targetId}/bug-tests/results/${resultId}`);
};

// -----------------------------
// Bug Pattern Registry - v3.8.3
// -----------------------------
export interface BugPattern {
  id: number;
  created_at: string;
  updated_at: string;
  key: string;
  name: string;
  description: string;
  bug_type: string;
  severity_hint: string;
  confidence_default: string;
  test_level: number;
  safety_level: number;
  mode: string;
  safe_by_default: boolean;
  requires_approval: boolean;
  enabled: boolean;
  source: string;
  version: string;
  matcher_json?: any;
  evidence_schema_json?: any;
  owasp_refs?: any;
  tags?: any;
}

export interface BugPatternsResponse {
  status: string;
  data: BugPattern[];
  count: number;
  total_count: number;
  page: number;
}

export const getBugPatterns = async (params?: {
  bug_type?: string;
  mode?: string;
  enabled?: string;
  limit?: number;
}) => {
  const response = await apiClient.get<BugPatternsResponse>("/bug-patterns", {
    params: {
      limit: params?.limit ?? 100,
      bug_type: params?.bug_type || undefined,
      mode: params?.mode || undefined,
      enabled: params?.enabled || undefined,
    },
  });

  return response.data;
};

export const updateBugPatternEnabled = async (
  patternId: number,
  enabled: boolean,
) => {
  const response = await apiClient.patch<{
    status: string;
    data: BugPattern;
  }>(`/bug-patterns/${patternId}/enabled`, { enabled });

  return response.data.data;
};
