import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { KeyRound, Save, Trash2 } from "lucide-react";
import {
  deleteMyVirusTotalConfig,
  getMyVirusTotalConfig,
  putMyVirusTotalConfig,
} from "../api/me";

export const VirusTotalConfigPanel = () => {
  const queryClient = useQueryClient();
  const [enabled, setEnabled] = useState(false);
  const [apiKey, setApiKey] = useState("");

  const query = useQuery({
    queryKey: ["me", "virustotal-config"],
    queryFn: getMyVirusTotalConfig,
  });

  useEffect(() => {
    if (!query.data) return;
    setEnabled(query.data.enabled);
    setApiKey("");
  }, [query.data]);

  const saveMutation = useMutation({
    mutationFn: () =>
      putMyVirusTotalConfig({
        enabled,
        api_key: apiKey.trim() || undefined,
      }),
    onSuccess: () => {
      setApiKey("");
      queryClient.invalidateQueries({ queryKey: ["me", "virustotal-config"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteMyVirusTotalConfig,
    onSuccess: () => {
      setEnabled(false);
      setApiKey("");
      queryClient.invalidateQueries({ queryKey: ["me", "virustotal-config"] });
    },
  });

  return (
    <div className="hack-card p-5">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center border border-hack-primary/50 bg-hack-primary/10 text-hack-primary">
          <KeyRound size={18} />
        </div>
        <div>
          <h3 className="font-mono text-lg font-semibold uppercase tracking-wider text-white">
            VirusTotal API
          </h3>
          <p className="text-sm text-hack-dim">
            Account-scoped API key used only when VirusTotal URLs is enabled on a target.
          </p>
        </div>
      </div>

      {query.isLoading ? (
        <div className="text-sm text-hack-dim">Loading VirusTotal config...</div>
      ) : (
        <>
          <div className="mb-4 flex items-center gap-3">
            <input
              id="virustotal-enabled"
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-4 w-4 accent-hack-primary"
            />
            <label
              htmlFor="virustotal-enabled"
              className="font-mono text-sm uppercase tracking-wider text-hack-dim"
            >
              Enable VirusTotal URL collection
            </label>
          </div>

          <label className="mb-2 block font-mono text-xs uppercase tracking-wider text-hack-dim">
            API Key
          </label>
          <input
            type="password"
            value={apiKey}
            onChange={(e) => setApiKey(e.target.value)}
            placeholder={
              query.data?.has_api_key
                ? `Saved: ${query.data.masked_api_key}`
                : "Paste VirusTotal API key"
            }
            className="hack-input mb-3 w-full"
          />

          <div className="mb-4 text-xs text-hack-dim">
            Leaving the field blank keeps the existing key. Use Delete to remove the saved key.
          </div>

          {query.data?.has_api_key && (
            <div className="mb-4 border border-hack-border bg-black/30 p-3 font-mono text-xs text-hack-dim">
              Saved key: {query.data.masked_api_key}
            </div>
          )}

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
              className="hack-btn flex items-center gap-2 px-4 py-2 disabled:opacity-50"
            >
              <Save size={15} />
              {saveMutation.isPending ? "Saving..." : "Save VirusTotal Settings"}
            </button>

            <button
              type="button"
              onClick={() => {
                if (window.confirm("Delete saved VirusTotal API key?")) {
                  deleteMutation.mutate();
                }
              }}
              disabled={deleteMutation.isPending || !query.data?.has_api_key}
              className="border border-hack-danger/60 px-4 py-2 font-mono text-xs uppercase tracking-wider text-hack-danger disabled:opacity-50"
            >
              <Trash2 className="mr-2 inline h-4 w-4" />
              Delete Key
            </button>
          </div>

          {saveMutation.isError && (
            <div className="mt-3 text-sm text-hack-danger">
              Failed to save VirusTotal config.
            </div>
          )}
          {deleteMutation.isError && (
            <div className="mt-3 text-sm text-hack-danger">
              Failed to delete VirusTotal config.
            </div>
          )}
        </>
      )}
    </div>
  );
};

export default VirusTotalConfigPanel;
