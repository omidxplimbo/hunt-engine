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
