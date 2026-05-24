import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  KeyRound,
  User2,
  Key,
  Plus,
  Save,
  Trash2,
  Eye,
  EyeOff,
} from "lucide-react";
import {
  changeMyPassword,
  getMe,
  getMySubfinderProviders,
  putMySubfinderProviders,
  type SubfinderProviderItem,
} from "../api/me";
import { QueueManager } from "../components/QueueManager";
import TelegramConfigPanel from "../components/TelegramConfigPanel";

const Account = () => {
  const {
    data: me,
    isLoading,
    isError,
  } = useQuery({
    queryKey: ["me"],
    queryFn: getMe,
    refetchInterval: 30000,
  });

  const subfinderProvidersQuery = useQuery({
    queryKey: ["me", "subfinder", "providers"],
    queryFn: getMySubfinderProviders,
    staleTime: 60_000,
  });

  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);
  const [sfMessage, setSfMessage] = useState<string | null>(null);
  const [sfError, setSfError] = useState<string | null>(null);
  const [showKeys, setShowKeys] = useState(false);
  const [sfRows, setSfRows] = useState<{ provider: string; apiKey: string }[]>(
    [],
  );

  const createdAt = useMemo(() => {
    if (!me?.createdAt) return "-";
    const d = new Date(me.createdAt);
    return Number.isNaN(d.getTime()) ? me.createdAt : d.toLocaleString();
  }, [me?.createdAt]);

  useEffect(() => {
    const data = subfinderProvidersQuery.data;
    if (!data) return;

    const rows = data
      .map((p) => {
        const apiKey =
          Array.isArray(p.entries) && typeof p.entries[0] === "string"
            ? (p.entries[0] as string)
            : "";
        return { provider: p.provider || "", apiKey };
      })
      .filter((r) => r.provider.trim() !== "");

    setSfRows(rows);
  }, [subfinderProvidersQuery.data]);

  const changePwMutation = useMutation({
    mutationFn: changeMyPassword,
    onSuccess: (res) => {
      setErrorMsg(null);
      setMessage(res?.message || "Password changed");
      setCurrentPassword("");
      setNewPassword("");
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(err?.response?.data?.error || "Failed to change password");
    },
  });

  const saveSubfinderMutation = useMutation({
    mutationFn: async () => {
      const providers: SubfinderProviderItem[] = sfRows
        .map((r) => ({
          provider: r.provider.trim().toLowerCase(),
          entries: r.apiKey.trim() ? [r.apiKey.trim()] : [],
        }))
        .filter((p) => p.provider !== "" && p.entries.length > 0);

      return putMySubfinderProviders(providers);
    },
    onSuccess: (res) => {
      setSfError(null);
      setSfMessage(res?.message || "Saved");
      subfinderProvidersQuery.refetch();
    },
    onError: (err: any) => {
      setSfMessage(null);
      setSfError(
        err?.response?.data?.error || "Failed to save subfinder providers",
      );
    },
  });

  if (isLoading)
    return (
      <div className="font-mono text-hack-dim">Loading account data...</div>
    );

  if (isError || !me) {
    return (
      <div className="font-mono text-hack-danger">
        SYSTEM ERROR: Failed to fetch account data.
      </div>
    );
  }

  return (
    <div className="space-y-6">
      <div>
        <h1 className="font-mono text-2xl uppercase tracking-wider text-hack-primary">
          ACCOUNT
        </h1>
        <p className="text-hack-dim font-mono text-sm">
          Identity, credentials, provider keys, and personal queue control.
        </p>
      </div>

      <div className="grid gap-6 xl:grid-cols-2">
        <div className="border border-hack-border bg-black/30 p-5">
          <h2 className="mb-4 font-mono text-lg uppercase tracking-wider text-hack-primary flex items-center gap-2">
            <User2 size={18} /> Profile
          </h2>
          <div className="space-y-3">
            <Row label="Username" value={me.username} />
            <Row label="Role" value={me.role} />
            <Row label="Created" value={createdAt} />
            <Row
              label="Concurrent Scan Slots"
              value={
                me.role === "admin"
                  ? "UNLIMITED (ADMIN)"
                  : String(me.max_concurrent_scans || 1)
              }
            />
          </div>
        </div>

        <div className="border border-hack-border bg-black/30 p-5">
          <h2 className="mb-4 font-mono text-lg uppercase tracking-wider text-hack-primary flex items-center gap-2">
            <KeyRound size={18} /> Change Password
          </h2>

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

          <form
            className="space-y-4"
            onSubmit={(e) => {
              e.preventDefault();
              setMessage(null);
              setErrorMsg(null);
              changePwMutation.mutate({
                current_password: currentPassword,
                new_password: newPassword,
              });
            }}
          >
            <input
              type="password"
              value={currentPassword}
              onChange={(e) => setCurrentPassword(e.target.value)}
              required
              placeholder="Current Password"
              className="w-full bg-black/40 border border-hack-border px-3 py-3 font-mono text-white focus:border-hack-primary focus:outline-none"
            />
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              required
              minLength={6}
              placeholder="New Password"
              className="w-full bg-black/40 border border-hack-border px-3 py-3 font-mono text-white focus:border-hack-primary focus:outline-none"
            />
            <button
              type="submit"
              className="hack-btn w-full py-3"
              disabled={changePwMutation.isPending}
            >
              {changePwMutation.isPending ? "UPDATING..." : "UPDATE PASSWORD"}
            </button>
          </form>
        </div>
      </div>

      <QueueManager
        title="My Scan Queue"
        description="Only your queued scan jobs are shown here. Reordering is applied to your own queue. Admins manage their own queue here, not other users' queues."
      />

      <TelegramConfigPanel role={me.role} />

      <div className="border border-hack-border bg-black/30 p-5">
        <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
          <div>
            <h2 className="font-mono text-lg uppercase tracking-wider text-hack-primary flex items-center gap-2">
              <Key size={18} /> Subfinder Provider API Keys
            </h2>
            <p className="mt-1 text-xs text-hack-dim font-mono">
              These keys are stored per-user and are only used when Subfinder
              runs for targets owned by you.
            </p>
          </div>
          <div className="flex gap-2">
            <button
              type="button"
              onClick={() => setShowKeys((v) => !v)}
              className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
              title={showKeys ? "Hide keys" : "Show keys"}
            >
              {showKeys ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
            <button
              type="button"
              onClick={() =>
                setSfRows((prev) => [...prev, { provider: "", apiKey: "" }])
              }
              className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
              title="Add provider key"
            >
              <Plus size={14} /> Add
            </button>
            <button
              type="button"
              onClick={() => {
                setSfMessage(null);
                setSfError(null);
                saveSubfinderMutation.mutate();
              }}
              className="hack-btn border border-hack-primary/60 px-3 py-1 text-[10px] uppercase tracking-wider"
              disabled={saveSubfinderMutation.isPending}
              title="Save keys"
            >
              <Save size={14} />{" "}
              {saveSubfinderMutation.isPending ? "Saving..." : "Save"}
            </button>
          </div>
        </div>

        {sfMessage && (
          <div className="mb-3 border border-hack-primary/60 bg-hack-primary/10 p-3 text-sm text-hack-primary font-mono">
            {sfMessage}
          </div>
        )}
        {sfError && (
          <div className="mb-3 border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger font-mono">
            {sfError}
          </div>
        )}

        {subfinderProvidersQuery.isLoading ? (
          <div className="font-mono text-hack-dim">
            Loading subfinder providers...
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-sm">
              <thead>
                <tr className="border-b border-hack-border text-hack-dim uppercase text-xs tracking-wider">
                  <th className="py-2 pr-3">Provider</th>
                  <th className="py-2 pr-3">API Key</th>
                  <th className="py-2 text-right">Action</th>
                </tr>
              </thead>
              <tbody>
                {sfRows.length === 0 ? (
                  <tr>
                    <td colSpan={3} className="py-4 text-hack-dim">
                      No provider keys configured. Click Add to add one.
                    </td>
                  </tr>
                ) : (
                  sfRows.map((row, idx) => (
                    <tr key={idx} className="border-b border-hack-border/50">
                      <td className="py-2 pr-3">
                        <input
                          value={row.provider}
                          onChange={(e) => {
                            const v = e.target.value;
                            setSfRows((prev) =>
                              prev.map((r, i) =>
                                i === idx ? { ...r, provider: v } : r,
                              ),
                            );
                          }}
                          placeholder="chaos"
                          className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                        />
                      </td>
                      <td className="py-2 pr-3">
                        <input
                          type={showKeys ? "text" : "password"}
                          value={row.apiKey}
                          onChange={(e) => {
                            const v = e.target.value;
                            setSfRows((prev) =>
                              prev.map((r, i) =>
                                i === idx ? { ...r, apiKey: v } : r,
                              ),
                            );
                          }}
                          placeholder="api-key"
                          className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                        />
                      </td>
                      <td className="py-2 text-right">
                        <button
                          type="button"
                          onClick={() =>
                            setSfRows((prev) =>
                              prev.filter((_, i) => i !== idx),
                            )
                          }
                          className="p-2 text-hack-danger/70 hover:text-hack-danger"
                        >
                          <Trash2 size={16} />
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
};

const Row = ({ label, value }: { label: string; value: string }) => (
  <div className="flex items-center justify-between gap-4 border-b border-hack-border/50 pb-2 font-mono text-sm">
    <span className="text-hack-dim uppercase tracking-wider">{label}</span>
    <span className="text-white text-right">{value}</span>
  </div>
);

export default Account;
