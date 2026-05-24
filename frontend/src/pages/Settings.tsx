import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Navigate } from "react-router-dom";
import { getUsers, deleteUser, updateUser } from "../api/users";
import { UserModal } from "../components/UserModal";
import {
  User,
  Plus,
  Edit2,
  Trash2,
  Power,
  ListOrdered,
  Bell,
  Camera,
} from "lucide-react";
import { useAuth } from "../context/AuthContext";
import { ConcurrencyConfig } from "../components/ConcurrencyConfig";
import {
  getSystemConfig,
  updateSystemConfig,
  TELEGRAM_NOTIFICATION_EVENTS,
} from "../api/system";

const readConfigValue = (
  configs: any[] | undefined,
  key: string,
  fallback: string,
) => configs?.find((item) => item.key === key)?.value ?? fallback;

const parseTelegramEvents = (value: string) => {
  try {
    const parsed = JSON.parse(value);
    if (Array.isArray(parsed)) return parsed.map(String);
  } catch {
    return value
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }

  return TELEGRAM_NOTIFICATION_EVENTS.map((event) => event.key);
};

const Settings = () => {
  const { role } = useAuth();
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingUser, setEditingUser] = useState<any>(null);
  const queryClient = useQueryClient();

  const { data, isLoading } = useQuery({
    queryKey: ["users"],
    queryFn: getUsers,
  });
  const configQuery = useQuery({
    queryKey: ["system-config"],
    queryFn: getSystemConfig,
  });

  const updateConfigMutation = useMutation({
    mutationFn: ({ key, value }: { key: string; value: string }) =>
      updateSystemConfig(key, value),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: ["system-config"] }),
  });

  const telegramEnabled =
    readConfigValue(
      configQuery.data,
      "telegram_notifications_enabled",
      "true",
    ) === "true";
  const freshAssetScreenshotEnabled =
    readConfigValue(
      configQuery.data,
      "telegram_fresh_asset_screenshot_enabled",
      "false",
    ) === "true";
  const selectedTelegramEvents = parseTelegramEvents(
    readConfigValue(
      configQuery.data,
      "telegram_notification_events",
      JSON.stringify(TELEGRAM_NOTIFICATION_EVENTS.map((event) => event.key)),
    ),
  );

  const updateBooleanConfig = (key: string, value: boolean) => {
    updateConfigMutation.mutate({ key, value: value ? "true" : "false" });
  };

  const toggleTelegramEvent = (eventKey: string) => {
    const next = selectedTelegramEvents.includes(eventKey)
      ? selectedTelegramEvents.filter((item) => item !== eventKey)
      : [...selectedTelegramEvents, eventKey];

    updateConfigMutation.mutate({
      key: "telegram_notification_events",
      value: JSON.stringify(next),
    });
  };

  const deleteMutation = useMutation({
    mutationFn: deleteUser,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  const toggleActiveMutation = useMutation({
    mutationFn: ({ id, is_active }: { id: number; is_active: boolean }) =>
      updateUser(id, { is_active }),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["users"] }),
  });

  if (role !== "admin") return <Navigate to="/" replace />;

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row md:items-center md:justify-between gap-4">
        <div>
          <h1 className="font-mono text-2xl uppercase tracking-wider text-hack-primary">
            SYSTEM CONFIGURATION
          </h1>
          <p className="text-hack-dim font-mono text-sm">
            Access Control & Parameters
          </p>
        </div>
        <button
          onClick={() => {
            setEditingUser(null);
            setIsModalOpen(true);
          }}
          className="hack-btn w-full md:w-auto flex items-center justify-center gap-2"
        >
          <Plus size={16} /> New User
        </button>
      </div>

      <ConcurrencyConfig />

      <div className="border border-hack-border bg-black/30 p-5">
        <div className="mb-4 flex flex-col gap-2 border-b border-hack-border pb-4 md:flex-row md:items-center md:justify-between">
          <div>
            <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
              <Bell size={18} /> Telegram_Notifications
            </h2>
            <p className="mt-1 font-mono text-xs text-hack-dim">
              Control which operational events are sent to Telegram.
            </p>
          </div>

          <button
            type="button"
            onClick={() =>
              updateBooleanConfig(
                "telegram_notifications_enabled",
                !telegramEnabled,
              )
            }
            className={`hack-btn ${telegramEnabled ? "bg-hack-primary text-black" : "border-hack-danger text-hack-danger"}`}
          >
            {telegramEnabled ? "ENABLED" : "DISABLED"}
          </button>
        </div>

        <div className={`space-y-4 ${telegramEnabled ? "" : "opacity-40"}`}>
          <div className="grid gap-3 md:grid-cols-2">
            {TELEGRAM_NOTIFICATION_EVENTS.map((event) => {
              const checked = selectedTelegramEvents.includes(event.key);

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
                    disabled={
                      !telegramEnabled || updateConfigMutation.isPending
                    }
                    onChange={() => toggleTelegramEvent(event.key)}
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
                  Capture the asset homepage, upload it to Telegram, then delete
                  the temporary file from /data/screenshots.
                </span>
              </span>
              <input
                type="checkbox"
                checked={freshAssetScreenshotEnabled}
                disabled={
                  !telegramEnabled ||
                  !selectedTelegramEvents.includes("fresh_asset") ||
                  updateConfigMutation.isPending
                }
                onChange={() =>
                  updateBooleanConfig(
                    "telegram_fresh_asset_screenshot_enabled",
                    !freshAssetScreenshotEnabled,
                  )
                }
                className="h-4 w-4 accent-hack-primary"
              />
            </label>
          </div>
        </div>
      </div>

      <div className="border border-hack-border bg-black/30 p-5">
        <h2 className="mb-4 font-mono text-lg uppercase tracking-wider text-hack-primary">
          User_Privileges_DB
        </h2>

        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-sm">
            <thead>
              <tr className="border-b border-hack-border text-hack-dim uppercase text-xs tracking-wider">
                <th className="py-3 pr-4">Identity</th>
                <th className="py-3 pr-4">Clearance Level</th>
                <th className="py-3 pr-4">Scan Slots</th>
                <th className="py-3 pr-4">Status</th>
                <th className="py-3 pr-4">Registration Date</th>
                <th className="py-3 text-right">Override</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                <tr>
                  <td colSpan={6} className="py-4 text-hack-dim">
                    DECRYPTING USER DATA...
                  </td>
                </tr>
              ) : (
                data?.data.map((user: any) => (
                  <tr key={user.id} className="border-b border-hack-border/50">
                    <td className="py-3 pr-4 text-white flex items-center gap-2">
                      <User size={16} className="text-hack-primary" />{" "}
                      {user.username}
                    </td>
                    <td className="py-3 pr-4 uppercase text-hack-primary">
                      {user.role}
                    </td>
                    <td className="py-3 pr-4">
                      <span className="inline-flex items-center gap-2 text-hack-dim">
                        <ListOrdered size={14} />{" "}
                        {user.role === "admin"
                          ? "UNLIMITED"
                          : user.max_concurrent_scans || 1}
                      </span>
                    </td>
                    <td className="py-3 pr-4">
                      {user.is_active ? "active" : "deactive"}
                    </td>
                    <td className="py-3 pr-4 text-hack-dim">
                      {new Date(user.created_at).toLocaleDateString()}
                    </td>
                    <td className="py-3 text-right">
                      <div className="inline-flex items-center gap-2">
                        <button
                          onClick={() =>
                            toggleActiveMutation.mutate({
                              id: user.id,
                              is_active: !user.is_active,
                            })
                          }
                          className={`p-2 transition-colors ${user.is_active ? "hover:text-hack-warning" : "hover:text-hack-primary"}`}
                          title={user.is_active ? "DEACTIVATE" : "ACTIVATE"}
                        >
                          <Power size={16} />
                        </button>
                        <button
                          onClick={() => {
                            setEditingUser(user);
                            setIsModalOpen(true);
                          }}
                          className="p-2 hover:text-hack-primary transition-colors"
                          title="MODIFY"
                        >
                          <Edit2 size={16} />
                        </button>
                        <button
                          onClick={() => {
                            if (confirm(">> EXECUTE DELETION PROTOCOL?"))
                              deleteMutation.mutate(user.id);
                          }}
                          className="p-2 hover:text-hack-danger transition-colors"
                          title="TERMINATE"
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      <UserModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        user={editingUser}
      />
    </div>
  );
};

export default Settings;
