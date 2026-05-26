import { useEffect, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { BrainCircuit, Eye, EyeOff, Plus, Save, Trash2 } from "lucide-react";
import {
  getMyLLMProviders,
  putMyLLMProviders,
  type LLMProviderConfig as LLMProviderConfigDTO,
  type LLMProviderPayload,
} from "../api/me";

type LLMRow = {
  provider: string;
  displayName: string;
  apiKey: string;
  apiKeySaved: boolean;
  baseURL: string;
  defaultModel: string;
  enabled: boolean;
  isDefault: boolean;
  clearKey: boolean;
};

const providerOptions = [
  "openai",
  "anthropic",
  "gemini",
  "openrouter",
  "groq",
  "custom",
  "local",
];

const emptyRow = (): LLMRow => ({
  provider: "",
  displayName: "",
  apiKey: "",
  apiKeySaved: false,
  baseURL: "",
  defaultModel: "",
  enabled: true,
  isDefault: false,
  clearKey: false,
});

const fromAPI = (item: LLMProviderConfigDTO): LLMRow => ({
  provider: item.provider || "",
  displayName: item.display_name || item.provider || "",
  apiKey: "",
  apiKeySaved: Boolean(item.api_key_saved),
  baseURL: item.base_url || "",
  defaultModel: item.default_model || "",
  enabled: Boolean(item.enabled),
  isDefault: Boolean(item.is_default),
  clearKey: false,
});

export const LLMProviderConfig = () => {
  const [rows, setRows] = useState<LLMRow[]>([]);
  const [showKeys, setShowKeys] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [errorMsg, setErrorMsg] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["me", "llm-providers"],
    queryFn: getMyLLMProviders,
    staleTime: 60_000,
  });

  useEffect(() => {
    if (!query.data) return;
    setRows(query.data.providers.map(fromAPI));
  }, [query.data]);

  const saveMutation = useMutation({
    mutationFn: async () => {
      const payload: LLMProviderPayload[] = rows
        .map((row) => ({
          provider: row.provider.trim().toLowerCase(),
          display_name: row.displayName.trim(),
          api_key: row.apiKey.trim(),
          base_url: row.baseURL.trim(),
          default_model: row.defaultModel.trim(),
          enabled: row.enabled,
          is_default: row.isDefault,
          clear_key: row.clearKey,
        }))
        .filter((row) => row.provider !== "");

      return putMyLLMProviders(payload);
    },
    onSuccess: (res) => {
      setErrorMsg(null);
      setMessage(res?.message || "LLM providers saved");
      void query.refetch();
    },
    onError: (err: any) => {
      setMessage(null);
      setErrorMsg(err?.response?.data?.error || "Failed to save LLM providers");
    },
  });

  const updateRow = (index: number, patch: Partial<LLMRow>) => {
    setRows((prev) =>
      prev.map((row, i) => (i === index ? { ...row, ...patch } : row)),
    );
  };

  const setDefault = (index: number) => {
    setRows((prev) =>
      prev.map((row, i) => ({ ...row, isDefault: i === index })),
    );
  };

  return (
    <div className="border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="font-mono text-lg uppercase tracking-wider text-hack-primary flex items-center gap-2">
            <BrainCircuit size={18} /> LLM Provider Config
          </h2>
          <p className="mt-1 text-xs text-hack-dim font-mono">
            Provider settings are account-scoped. Admin users share one admin
            configuration; non-admin users have private settings.
          </p>
          {query.data?.scope && (
            <p className="mt-1 text-[10px] uppercase tracking-wider text-hack-dim font-mono">
              Scope: {query.data.scope}
            </p>
          )}
        </div>

        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => setShowKeys((value) => !value)}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
            title={showKeys ? "Hide keys" : "Show keys"}
          >
            {showKeys ? <EyeOff size={14} /> : <Eye size={14} />}
          </button>

          <button
            type="button"
            onClick={() => setRows((prev) => [...prev, emptyRow()])}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white"
          >
            <Plus size={14} /> Add
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
        <div className="font-mono text-hack-dim">Loading LLM providers...</div>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full min-w-[980px] text-left font-mono text-sm">
            <thead>
              <tr className="border-b border-hack-border text-hack-dim uppercase text-xs tracking-wider">
                <th className="py-2 pr-3">Provider</th>
                <th className="py-2 pr-3">Display Name</th>
                <th className="py-2 pr-3">API Key</th>
                <th className="py-2 pr-3">Base URL</th>
                <th className="py-2 pr-3">Default Model</th>
                <th className="py-2 pr-3">Enabled</th>
                <th className="py-2 pr-3">Default</th>
                <th className="py-2 text-right">Action</th>
              </tr>
            </thead>
            <tbody>
              {rows.length === 0 ? (
                <tr>
                  <td colSpan={8} className="py-4 text-hack-dim">
                    No LLM providers configured. Click Add to add one.
                  </td>
                </tr>
              ) : (
                rows.map((row, index) => (
                  <tr
                    key={`${row.provider}-${index}`}
                    className="border-b border-hack-border/50"
                  >
                    <td className="py-2 pr-3">
                      <input
                        list="llm-provider-options"
                        value={row.provider}
                        onChange={(event) =>
                          updateRow(index, { provider: event.target.value })
                        }
                        placeholder="openai"
                        className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                      />
                    </td>

                    <td className="py-2 pr-3">
                      <input
                        value={row.displayName}
                        onChange={(event) =>
                          updateRow(index, { displayName: event.target.value })
                        }
                        placeholder="OpenAI Production"
                        className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                      />
                    </td>

                    <td className="py-2 pr-3">
                      <div className="space-y-1">
                        <input
                          type={showKeys ? "text" : "password"}
                          value={row.apiKey}
                          onChange={(event) =>
                            updateRow(index, {
                              apiKey: event.target.value,
                              clearKey: false,
                            })
                          }
                          placeholder={
                            row.apiKeySaved
                              ? "saved - leave blank to keep"
                              : "api key"
                          }
                          className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                        />
                        {row.apiKeySaved && (
                          <label className="flex items-center gap-2 text-[10px] uppercase tracking-wider text-hack-dim">
                            <input
                              type="checkbox"
                              checked={row.clearKey}
                              onChange={(event) =>
                                updateRow(index, {
                                  clearKey: event.target.checked,
                                  apiKey: "",
                                })
                              }
                              className="h-3 w-3 accent-hack-primary"
                            />
                            Clear saved key on save
                          </label>
                        )}
                      </div>
                    </td>

                    <td className="py-2 pr-3">
                      <input
                        value={row.baseURL}
                        onChange={(event) =>
                          updateRow(index, { baseURL: event.target.value })
                        }
                        placeholder="optional custom endpoint"
                        className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                      />
                    </td>

                    <td className="py-2 pr-3">
                      <input
                        value={row.defaultModel}
                        onChange={(event) =>
                          updateRow(index, { defaultModel: event.target.value })
                        }
                        placeholder="model name"
                        className="w-full bg-black/40 border border-hack-border px-3 py-2 text-white focus:border-hack-primary focus:outline-none"
                      />
                    </td>

                    <td className="py-2 pr-3">
                      <input
                        type="checkbox"
                        checked={row.enabled}
                        onChange={(event) =>
                          updateRow(index, { enabled: event.target.checked })
                        }
                        className="h-4 w-4 accent-hack-primary"
                      />
                    </td>

                    <td className="py-2 pr-3">
                      <input
                        type="radio"
                        name="default-llm-provider"
                        checked={row.isDefault}
                        onChange={() => setDefault(index)}
                        className="h-4 w-4 accent-hack-primary"
                      />
                    </td>

                    <td className="py-2 text-right">
                      <button
                        type="button"
                        onClick={() =>
                          setRows((prev) => prev.filter((_, i) => i !== index))
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

          <datalist id="llm-provider-options">
            {providerOptions.map((provider) => (
              <option key={provider} value={provider} />
            ))}
          </datalist>
        </div>
      )}
    </div>
  );
};

export default LLMProviderConfig;
