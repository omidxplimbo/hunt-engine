import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { Bell, Camera, Eye, EyeOff, Save, Send } from "lucide-react";
import {
  getMyTelegramConfig,
  putMyTelegramConfig,
  TELEGRAM_NOTIFICATION_EVENTS,
  type TelegramConfigPayload,
} from "../api/me";

export const TelegramConfigPanel = ({ role }: { role: string }) => {
  const query = useQuery({
    queryKey: ["me", "telegram-config"],
    queryFn: getMyTelegramConfig,
    staleTime: 30_000,
  });

  const [enabled, setEnabled] = useState(false);
  const [botToken, setBotToken] = useState("");
  const [chatID, setChatID] = useState("");
  const [events, setEvents] = useState<string[]>([]);
  const [freshScreenshot, setFreshScreenshot] = useState(false);
  const [showToken, setShowToken] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  useEffect(() => {
    if (!query.data) return;

    setEnabled(query.data.enabled);
    setChatID(query.data.chat_id || "");
    setEvents(
      query.data.enabled_events ||
        TELEGRAM_NOTIFICATION_EVENTS.map((event) => event.key),
    );
    setFreshScreenshot(Boolean(query.data.fresh_asset_screenshot_enabled));
    setBotToken("");
  }, [query.data]);

  const saveMutation = useMutation({
    mutationFn: (payload: TelegramConfigPayload) =>
      putMyTelegramConfig(payload),
    onSuccess: (res) => {
      setErrorMsg(null);
      setMessage(res?.message || "Telegram config saved");
      setBotToken("");
      query.refetch();
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(
        err?.response?.data?.error || "Failed to save Telegram config",
      );
    },
  });

  const toggleEvent = (key: string) => {
    setEvents((prev) =>
      prev.includes(key) ? prev.filter((item) => item !== key) : [...prev, key],
    );
  };

  const save = () => {
    setMessage(null);
    setErrorMsg(null);

    const payload: TelegramConfigPayload = {
      enabled,
      chat_id: chatID.trim(),
      enabled_events: events,
      fresh_asset_screenshot_enabled: freshScreenshot,
    };

    if (botToken.trim()) {
      payload.bot_token = botToken.trim();
    }

    saveMutation.mutate(payload);
  };

  const isAdmin = role === "admin";

  return (
    <div className="border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-mono text-lg uppercase tracking-wider text-hack-primary flex items-center gap-2">
            <Bell size={18} /> Telegram Notifications
          </h2>
          <p className="mt-1 text-xs text-hack-dim font-mono">
            {isAdmin
              ? "Admins share one Telegram notification config."
              : "These Telegram settings only apply to targets owned by you."}
          </p>
        </div>

        <button
          type="button"
          onClick={() => setEnabled((value) => !value)}
          className={`hack-btn ${enabled ? "bg-hack-primary text-black" : "border-hack-danger text-hack-danger"}`}
        >
          {enabled ? "ENABLED" : "DISABLED"}
        </button>
      </div>

      {message && (
        <div className="mb-3 border border-hack-primary/60 bg-hack-primary/10 p-3 text-sm text-hack-primary font-mono">
          {message}
        </div>
      )}
      {errorMsg && (
        <div className="mb-3 border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger font-mono">
          {errorMsg}
        </div>
      )}

      {query.isLoading ? (
        <div className="font-mono text-hack-dim">
          Loading Telegram config...
        </div>
      ) : (
        <div className="space-y-4">
          <div className="grid gap-3 xl:grid-cols-2">
            <label className="block font-mono text-sm">
              <span className="mb-1 block uppercase tracking-wider text-hack-dim">
                Bot Token
              </span>
              <div className="flex gap-2">
                <input
                  type={showToken ? "text" : "password"}
                  value={botToken}
                  onChange={(e) => setBotToken(e.target.value)}
                  placeholder={
                    query.data?.has_bot_token
                      ? "Token saved - leave blank to keep it"
                      : "123456:ABC..."
                  }
                  className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                />
                <button
                  type="button"
                  onClick={() => setShowToken((value) => !value)}
                  className="hack-btn-ghost border border-hack-border px-3"
                  title={showToken ? "Hide token" : "Show token"}
                >
                  {showToken ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              </div>
            </label>

            <label className="block font-mono text-sm">
              <span className="mb-1 block uppercase tracking-wider text-hack-dim">
                Chat ID
              </span>
              <input
                value={chatID}
                onChange={(e) => setChatID(e.target.value)}
                placeholder="-1001234567890"
                className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
              />
            </label>
          </div>

          <div
            className={`grid gap-3 md:grid-cols-2 ${enabled ? "" : "opacity-40"}`}
          >
            {TELEGRAM_NOTIFICATION_EVENTS.map((event) => {
              const checked = events.includes(event.key);

              return (
                <label
                  key={event.key}
                  className="flex cursor-pointer items-center justify-between gap-3 border border-hack-border bg-black/20 p-3 font-mono text-sm"
                >
                  <span>
                    <span className="block text-white">{event.label}</span>
                    <span className="block text-[10px] uppercase tracking-wider text-hack-dim">
                      {event.key}
                    </span>
                  </span>
                  <input
                    type="checkbox"
                    checked={checked}
                    disabled={!enabled || saveMutation.isPending}
                    onChange={() => toggleEvent(event.key)}
                    className="h-4 w-4 accent-hack-primary"
                  />
                </label>
              );
            })}
          </div>

          <div className="border border-hack-border bg-black/20 p-3">
            <label className="flex cursor-pointer items-center justify-between gap-3 font-mono text-sm">
              <span>
                <span className="flex items-center gap-2 text-white">
                  <Camera size={16} className="text-hack-primary" /> Fresh asset
                  screenshot
                </span>
                <span className="mt-1 block text-xs text-hack-dim">
                  Capture fresh asset homepage, upload it to Telegram, then
                  delete the temporary file.
                </span>
              </span>
              <input
                type="checkbox"
                checked={freshScreenshot}
                disabled={
                  !enabled ||
                  !events.includes("fresh_asset") ||
                  saveMutation.isPending
                }
                onChange={() => setFreshScreenshot((value) => !value)}
                className="h-4 w-4 accent-hack-primary"
              />
            </label>
          </div>

          <button
            type="button"
            onClick={save}
            className="hack-btn w-full justify-center py-3"
            disabled={saveMutation.isPending}
          >
            {saveMutation.isPending ? <Send size={14} /> : <Save size={14} />}
            {saveMutation.isPending ? "Saving..." : "Save Telegram Settings"}
          </button>
        </div>
      )}
    </div>
  );
};

export default TelegramConfigPanel;
