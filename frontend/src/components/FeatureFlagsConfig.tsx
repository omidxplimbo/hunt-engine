import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Save, ShieldCheck, RotateCcw } from "lucide-react";
import clsx from "clsx";
import {
  getMyFeatureFlags,
  putMyFeatureFlags,
  type AccountFeatureFlag,
  type FeatureFlagState,
  type PutMyFeatureFlagItem,
} from "../api/me";

type FeatureFlagRow = {
  key: string;
  state: FeatureFlagState;
};

const stateOptions: { value: FeatureFlagState; label: string }[] = [
  { value: "inherit", label: "Inherit global" },
  { value: "enabled", label: "Enabled" },
  { value: "disabled", label: "Disabled" },
];

const boolLabel = (value: boolean) => (value ? "enabled" : "disabled");

const stateClass = (value: boolean) =>
  value
    ? "border-hack-primary text-hack-primary bg-hack-primary/10"
    : "border-hack-danger/70 text-hack-danger bg-hack-danger/10";

const fromAPI = (flag: AccountFeatureFlag): FeatureFlagRow => ({
  key: flag.key,
  state: flag.state || "inherit",
});

export const FeatureFlagsConfig = () => {
  const queryClient = useQueryClient();
  const [rows, setRows] = useState<FeatureFlagRow[]>([]);
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["me", "feature-flags"],
    queryFn: getMyFeatureFlags,
    staleTime: 30_000,
  });

  useEffect(() => {
    if (!query.data?.flags) return;
    setRows(query.data.flags.map(fromAPI));
  }, [query.data]);

  const flagByKey = useMemo(() => {
    const out = new Map<string, AccountFeatureFlag>();
    for (const flag of query.data?.flags || []) {
      out.set(flag.key, flag);
    }
    return out;
  }, [query.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload: PutMyFeatureFlagItem[] = rows.map((row) => ({
        key: row.key,
        state: row.state,
      }));

      return putMyFeatureFlags(payload);
    },
    onSuccess: (res) => {
      setErrorMsg(null);
      setMessage(res?.message || "Feature flags updated");
      queryClient.invalidateQueries({ queryKey: ["me", "feature-flags"] });
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(
        err?.response?.data?.error ||
          err?.response?.data?.message ||
          "Failed to update feature flags",
      );
    },
  });

  const updateRow = (key: string, state: FeatureFlagState) => {
    setRows((prev) =>
      prev.map((row) => (row.key === key ? { ...row, state } : row)),
    );
  };

  const resetAllToInherit = () => {
    setRows((prev) => prev.map((row) => ({ ...row, state: "inherit" })));
  };

  return (
    <div className="border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
            <ShieldCheck size={18} /> Feature Access
          </h2>
          <p className="mt-1 font-mono text-xs text-hack-dim">
            Account-scoped feature flags. Inherit uses global System Config;
            account overrides take precedence.
          </p>
          {query.data?.scope && (
            <p className="mt-1 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
              Scope: {query.data.scope} · Owner: {query.data.owner_key}
            </p>
          )}
        </div>

        <div className="flex gap-2">
          <button
            type="button"
            onClick={resetAllToInherit}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
          >
            <RotateCcw size={14} /> Inherit All
          </button>

          <button
            type="button"
            onClick={() => {
              setMessage(null);
              setErrorMsg(null);
              saveMutation.mutate();
            }}
            className="hack-btn border border-hack-primary/60 px-3 py-1 text-[10px] uppercase tracking-wider"
            disabled={saveMutation.isPending}
          >
            <Save size={14} /> {saveMutation.isPending ? "Saving..." : "Save"}
          </button>
        </div>
      </div>

      {message && (
        <div className="mb-3 border border-hack-primary/60 bg-hack-primary/10 p-3 font-mono text-sm text-hack-primary">
          {message}
        </div>
      )}
      {errorMsg && (
        <div className="mb-3 border border-hack-danger/60 bg-hack-danger/10 p-3 font-mono text-sm text-hack-danger">
          {errorMsg}
        </div>
      )}

      {query.isLoading ? (
        <div className="font-mono text-hack-dim">Loading feature flags...</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[920px] text-left font-mono text-sm">
            <thead>
              <tr className="border-b border-hack-border text-xs uppercase tracking-wider text-hack-dim">
                <th className="py-2 pr-3">Feature</th>
                <th className="py-2 pr-3">Account State</th>
                <th className="py-2 pr-3">Global</th>
                <th className="py-2 pr-3">Effective</th>
                <th className="py-2 pr-3">Source</th>
                <th className="py-2 pr-3">Description</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((row) => {
                const flag = flagByKey.get(row.key);
                const effective = Boolean(flag?.effective);
                const globalValue = Boolean(flag?.global_value);

                return (
                  <tr
                    key={row.key}
                    className="border-b border-hack-border/50 align-top"
                  >
                    <td className="py-3 pr-3">
                      <div className="text-white">{row.key}</div>
                      <div className="mt-1 text-[10px] uppercase tracking-wider text-hack-dim">
                        default: {boolLabel(Boolean(flag?.default))}
                      </div>
                    </td>

                    <td className="py-3 pr-3">
                      <select
                        value={row.state}
                        onChange={(event) =>
                          updateRow(
                            row.key,
                            event.target.value as FeatureFlagState,
                          )
                        }
                        className="border border-hack-border bg-black/40 px-3 py-2 text-white outline-none focus:border-hack-primary"
                      >
                        {stateOptions.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </td>

                    <td className="py-3 pr-3">
                      <span
                        className={clsx(
                          "border px-2 py-1 text-[10px] uppercase tracking-wider",
                          stateClass(globalValue),
                        )}
                      >
                        {boolLabel(globalValue)}
                      </span>
                    </td>

                    <td className="py-3 pr-3">
                      <span
                        className={clsx(
                          "border px-2 py-1 text-[10px] uppercase tracking-wider",
                          stateClass(effective),
                        )}
                      >
                        {boolLabel(effective)}
                      </span>
                    </td>

                    <td className="py-3 pr-3 text-hack-dim">
                      {flag?.source || "-"}
                    </td>

                    <td className="max-w-xl py-3 pr-3 text-xs leading-5 text-hack-dim">
                      {flag?.description || "-"}
                    </td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
};

export default FeatureFlagsConfig;
