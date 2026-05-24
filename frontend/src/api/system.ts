import { apiClient } from "./client";

export interface SystemConfig {
  key: string;
  value: string;
}

export const getSystemConfig = async () => {
  const response = await apiClient.get("/config");
  return response.data;
};

export const updateSystemConfig = async (key: string, value: string) => {
  const response = await apiClient.put<{ status: string; message: string }>(
    `/config/${key}`,
    { value },
  );
  return response.data;
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
