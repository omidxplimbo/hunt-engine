import { apiClient } from "./client";

export interface MeData {
  id: number;
  username: string;
  role: string;
  createdAt: string;
  max_concurrent_scans?: number;
}

export const getMe = async () => {
  const res = await apiClient.get<{ status: string; data: MeData }>("/me");
  return res.data.data;
};

export interface ChangePasswordPayload {
  current_password: string;
  new_password: string;
}

export const changeMyPassword = async (payload: ChangePasswordPayload) => {
  const res = await apiClient.post<{ status: string; message: string }>(
    "/me/change-password",
    payload,
  );
  return res.data;
};

// -----------------------------
// Subfinder provider config (per-user)
// -----------------------------
export interface SubfinderProviderItem {
  provider: string;
  entries: any[];
}

export const getMySubfinderProviders = async () => {
  const res = await apiClient.get<{
    status: string;
    data: { providers: SubfinderProviderItem[] };
  }>("/me/subfinder/providers");
  return res.data.data.providers;
};

export const putMySubfinderProviders = async (
  providers: SubfinderProviderItem[],
) => {
  const res = await apiClient.put<{ status: string; message: string }>(
    "/me/subfinder/providers",
    { providers },
  );
  return res.data;
};

export const deleteMySubfinderProvider = async (provider: string) => {
  const p = encodeURIComponent(provider);
  const res = await apiClient.delete<{ status: string; message: string }>(
    `/me/subfinder/providers/${p}`,
  );
  return res.data;
};

// -----------------------------
// Telegram notification config
// -----------------------------
export const TELEGRAM_NOTIFICATION_EVENTS = [
  { key: "fresh_asset", label: "Fresh asset" },
  { key: "asset_change_is_live", label: "Asset live/dead change" },
  { key: "asset_change_status_code", label: "Status code change" },
  { key: "asset_change_title", label: "Title change" },
  { key: "asset_change_web_server", label: "Web server change" },
  { key: "asset_change_technologies", label: "Technologies change" },
  { key: "asset_change_host_ip", label: "Host IP change" },
  { key: "fresh_url", label: "Fresh crawl URL" },
] as const;

export type TelegramNotificationEvent =
  (typeof TELEGRAM_NOTIFICATION_EVENTS)[number]["key"];

export interface TelegramConfig {
  scope: "admin" | "user";
  owner_key: string;
  enabled: boolean;
  has_bot_token: boolean;
  chat_id: string;
  enabled_events: string[];
  fresh_asset_screenshot_enabled: boolean;
}

export interface TelegramConfigPayload {
  enabled: boolean;
  bot_token?: string;
  chat_id: string;
  enabled_events: string[];
  fresh_asset_screenshot_enabled: boolean;
}

export const getMyTelegramConfig = async () => {
  const res = await apiClient.get<{ status: string; data: TelegramConfig }>(
    "/me/telegram-config",
  );
  return res.data.data;
};

export const putMyTelegramConfig = async (payload: TelegramConfigPayload) => {
  const res = await apiClient.put<{ status: string; message: string }>(
    "/me/telegram-config",
    payload,
  );
  return res.data;
};

// -----------------------------
// LLM provider config
// -----------------------------
export interface LLMProviderConfig {
  id?: number;
  provider: string;
  display_name: string;
  api_key_saved: boolean;
  base_url: string;
  default_model: string;
  enabled: boolean;
  is_default: boolean;
  scope?: string;
  owner_key?: string;
  updated_at?: string;
}

export interface LLMProviderPayload {
  provider: string;
  display_name: string;
  api_key?: string;
  base_url?: string;
  default_model?: string;
  enabled: boolean;
  is_default: boolean;
  clear_key?: boolean;
}

export const getMyLLMProviders = async () => {
  const res = await apiClient.get<{
    status: string;
    data: { providers: LLMProviderConfig[]; scope: string; owner_key: string };
  }>("/me/llm-providers");

  return res.data.data;
};

export const putMyLLMProviders = async (providers: LLMProviderPayload[]) => {
  const res = await apiClient.put<{ status: string; message: string }>(
    "/me/llm-providers",
    {
      providers,
    },
  );

  return res.data;
};

export const deleteMyLLMProvider = async (provider: string) => {
  const p = encodeURIComponent(provider);
  const res = await apiClient.delete<{ status: string; message: string }>(
    `/me/llm-providers/${p}`,
  );
  return res.data;
};

// -----------------------------
// Account-scoped feature flags
// -----------------------------
export const FEATURE_FLAGS = {
  targetPolicy: "feature.target_policy",
  targetPDFReport: "feature.target_pdf_report",
  aiAnalysis: "feature.ai_analysis",
  llmAssistedAnalysis: "feature.llm_assisted_analysis",
  aiRecommendations: "feature.ai_recommendations",
  aiNucleiTemplateDrafts: "feature.ai_nuclei_template_drafts",
  agentRuns: "feature.agent_runs",
  aiTriageAgent: "feature.ai_triage_agent",
  aiSummaryAgent: "feature.ai_summary_agent",
  aiReportAgent: "feature.ai_report_agent",
} as const;

export type FeatureFlagKey = (typeof FEATURE_FLAGS)[keyof typeof FEATURE_FLAGS];
export type FeatureFlagState = "inherit" | "enabled" | "disabled";

export interface AccountFeatureFlag {
  key: FeatureFlagKey | string;
  description: string;
  default: boolean;
  global_value: boolean;
  state: FeatureFlagState;
  effective: boolean;
  source: "global" | "account" | string;
  scope: string;
  owner_key: string;
}

export interface MyFeatureFlagsData {
  flags: AccountFeatureFlag[];
  scope: string;
  owner_key: string;
}

export interface PutMyFeatureFlagItem {
  key: string;
  state: FeatureFlagState;
}

export const getMyFeatureFlags = async () => {
  const res = await apiClient.get<{
    status: string;
    data: MyFeatureFlagsData;
  }>("/me/feature-flags");

  return res.data.data;
};

export const putMyFeatureFlags = async (flags: PutMyFeatureFlagItem[]) => {
  const res = await apiClient.put<{
    status: string;
    message: string;
    data: MyFeatureFlagsData;
  }>("/me/feature-flags", { flags });

  return res.data;
};

export const isAccountFeatureEnabled = (
  flags: AccountFeatureFlag[] | undefined,
  key: FeatureFlagKey,
  defaultValue: boolean,
): boolean => {
  const flag = flags?.find((item) => item.key === key);
  if (!flag) return defaultValue;
  return Boolean(flag.effective);
};

// -----------------------------
// Account custom wordlists
// -----------------------------
export type UploadProgressCallback = (percent: number, loaded: number, total?: number) => void;

export interface MyWordlist {
  id: number;
  user_id: number;
  name: string;
  size_bytes: number;
  sha256: string;
  lines: number;
  source: "file" | "url" | string;
  created_at: string;
  updated_at: string;
  puredns_path: string;
}

export interface MyWordlistsData {
  wordlists: MyWordlist[];
  current_total_size_bytes: number;
  max_file_size_bytes: number;
  max_total_size_bytes: number;
  unlimited: boolean;
}

export const getMyWordlists = async () => {
  const res = await apiClient.get<{
    status: string;
    data: MyWordlistsData;
  }>("/me/wordlists");

  return res.data.data;
};

export const uploadMyWordlistFile = async (
  file: File,
  onProgress?: UploadProgressCallback,
) => {
  const form = new FormData();
  form.append("file", file);

  const res = await apiClient.post<{
    status: string;
    data: MyWordlist;
  }>("/me/wordlists/upload", form, {
    headers: {
      "Content-Type": "multipart/form-data",
    },
    onUploadProgress: (event: any) => {
      const loaded = Number(event.loaded || 0);
      const total = Number(event.total || file.size || 0);
      const percent =
        total > 0 ? Math.min(100, Math.round((loaded / total) * 100)) : 0;

      onProgress?.(percent, loaded, total || undefined);
    },
  });

  return res.data.data;
};

export const uploadMyWordlistURL = async (url: string) => {
  const res = await apiClient.post<{
    status: string;
    data: MyWordlist;
  }>("/me/wordlists/upload-url", { url });

  return res.data.data;
};

export const downloadMyWordlist = async (id: number, name = "wordlist.txt") => {
  const res = await apiClient.get(`/me/wordlists/${id}/download`, {
    responseType: "blob",
  });

  const blob = new Blob([res.data], { type: "text/plain" });
  const objectUrl = window.URL.createObjectURL(blob);
  const link = document.createElement("a");

  link.href = objectUrl;
  link.download = name || "wordlist.txt";

  document.body.appendChild(link);
  link.click();
  link.remove();

  window.URL.revokeObjectURL(objectUrl);
};

export const deleteMyWordlist = async (id: number) => {
  const res = await apiClient.delete<{
    status: string;
    message: string;
  }>(`/me/wordlists/${id}`);

  return res.data;
};

