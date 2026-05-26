import { apiClient } from "./client";

export interface SystemConfig {
  key: string;
  value: string;
  updated_at?: string;
}

export const FEATURE_FLAGS = {
  targetPDFReport: "feature.target_pdf_report",
  aiAnalysis: "feature.ai_analysis",
  llmAssistedAnalysis: "feature.llm_assisted_analysis",
  aiRecommendations: "feature.ai_recommendations",
  aiNucleiTemplateDrafts: "feature.ai_nuclei_template_drafts",
} as const;

export type FeatureFlagKey = (typeof FEATURE_FLAGS)[keyof typeof FEATURE_FLAGS];

export const getSystemConfig = async (): Promise<SystemConfig[]> => {
  const response = await apiClient.get<SystemConfig[]>("/config");
  return response.data;
};

export const updateSystemConfig = async (key: string, value: string) => {
  const response = await apiClient.put<{ status: string; message: string }>(
    `/config/${key}`,
    { value },
  );
  return response.data;
};

export const systemConfigValue = (
  configs: SystemConfig[] | undefined,
  key: string,
): string | undefined => configs?.find((config) => config.key === key)?.value;

export const isFeatureEnabled = (
  configs: SystemConfig[] | undefined,
  key: FeatureFlagKey,
  defaultValue: boolean,
): boolean => {
  const raw = systemConfigValue(configs, key);
  if (raw === undefined || raw === null || String(raw).trim() === "") {
    return defaultValue;
  }

  switch (String(raw).trim().toLowerCase()) {
    case "1":
    case "true":
    case "yes":
    case "on":
    case "enabled":
      return true;
    case "0":
    case "false":
    case "no":
    case "off":
    case "disabled":
      return false;
    default:
      return defaultValue;
  }
};

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

export interface QueueItem {
  index: number;
  position?: number;
  payload: string;
  module?: string;
  target_id?: number;
  root_domain?: string;
  target_name?: string;
  owner_username?: string;
}

export const getQueue = async (): Promise<QueueItem[]> => {
  const response = await apiClient.get<QueueItem[]>("/queue");
  return response.data;
};

export const removeFromQueue = async (index: number) => {
  const response = await apiClient.delete<{ status: string; message: string }>(
    `/queue/${index}`,
  );
  return response.data;
};

export const clearQueue = async () => {
  const response = await apiClient.delete<{ status: string; message: string }>(
    "/queue",
  );
  return response.data;
};

export const moveQueueItem = async (
  index: number,
  direction: "top" | "bottom",
) => {
  const response = await apiClient.post<{ status: string; message: string }>(
    `/queue/${index}/move-${direction}`,
  );
  return response.data;
};
