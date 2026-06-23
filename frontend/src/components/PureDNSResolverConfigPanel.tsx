import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Database, Save, Trash2 } from "lucide-react";
import {
  deleteMyPureDNSResolverConfig,
  getMyPureDNSResolverConfig,
  putMyPureDNSResolverConfig,
} from "../api/me";

export const PureDNSResolverConfigPanel = () => {
  const queryClient = useQueryClient();
  const [enabled, setEnabled] = useState(false);
  const [resolversText, setResolversText] = useState("");

  const query = useQuery({
    queryKey: ["me", "puredns-resolvers"],
    queryFn: getMyPureDNSResolverConfig,
  });

  useEffect(() => {
    if (!query.data) return;
    setEnabled(query.data.enabled);
    setResolversText(query.data.resolvers_text || "");
  }, [query.data]);

  const localResolverCount = useMemo(() => {
    const seen = new Set<string>();
    resolversText
      .split("\n")
      .map((line) => line.split("#")[0].trim())
      .filter(Boolean)
      .forEach((line) => seen.add(line));
    return seen.size;
  }, [resolversText]);

  const saveMutation = useMutation({
    mutationFn: () =>
      putMyPureDNSResolverConfig({
        enabled,
        resolvers_text: resolversText,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["me", "puredns-resolvers"] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteMyPureDNSResolverConfig,
    onSuccess: () => {
      setEnabled(false);
      setResolversText("");
      queryClient.invalidateQueries({ queryKey: ["me", "puredns-resolvers"] });
    },
  });

  const saveError: any = saveMutation.error;
  const apiErrors: string[] | undefined = saveError?.response?.data?.errors;
  const apiMessage: string | undefined = saveError?.response?.data?.message;

  return (
    <div className="hack-card p-5">
      <div className="mb-4 flex items-center gap-3">
        <div className="flex h-10 w-10 items-center justify-center border border-hack-primary/50 bg-hack-primary/10 text-hack-primary">
          <Database size={18} />
        </div>
        <div>
          <h3 className="font-mono text-lg font-semibold uppercase tracking-wider text-white">
            PureDNS Public Resolvers
          </h3>
          <p className="text-sm text-hack-dim">
            Account-scoped resolver list used for PureDNS brute-force runs on your targets.
          </p>
        </div>
      </div>

      {query.isLoading ? (
        <div className="text-sm text-hack-dim">Loading PureDNS resolver config...</div>
      ) : (
        <>
          <div className="mb-4 flex items-center gap-3">
            <input
              id="puredns-resolvers-enabled"
              type="checkbox"
              checked={enabled}
              onChange={(e) => setEnabled(e.target.checked)}
              className="h-4 w-4 accent-hack-primary"
            />
            <label
              htmlFor="puredns-resolvers-enabled"
              className="font-mono text-sm uppercase tracking-wider text-hack-dim"
            >
              Use my resolver list for PureDNS public resolving
            </label>
          </div>

          <label className="mb-2 block font-mono text-xs uppercase tracking-wider text-hack-dim">
            Resolvers / one IP per line
          </label>
          <textarea
            value={resolversText}
            onChange={(e) => setResolversText(e.target.value)}
            rows={10}
            placeholder={"1.1.1.1\n8.8.8.8\n9.9.9.9\n# one resolver IP per line"}
            className="hack-input mb-3 min-h-[220px] w-full font-mono"
          />

          <div className="mb-4 grid gap-2 text-xs text-hack-dim md:grid-cols-2">
            <div className="border border-hack-border bg-black/30 p-3">
              Saved resolvers: {query.data?.resolver_count || 0}
            </div>
            <div className="border border-hack-border bg-black/30 p-3">
              Current editor count: {localResolverCount}
            </div>
          </div>

          <div className="mb-4 border border-hack-warning/30 bg-hack-warning/10 p-3 text-xs text-hack-dim">
            Trusted validation resolvers remain system-controlled. This list only replaces the public resolver pool used by PureDNS brute-force.
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => saveMutation.mutate()}
              disabled={saveMutation.isPending}
              className="hack-btn flex items-center gap-2 px-4 py-2 disabled:opacity-50"
            >
              <Save size={15} />
              {saveMutation.isPending ? "Saving..." : "Save PureDNS Resolvers"}
            </button>

            <button
              type="button"
              onClick={() => {
                if (window.confirm("Delete saved PureDNS resolver list?")) {
                  deleteMutation.mutate();
                }
              }}
              disabled={deleteMutation.isPending || !query.data?.has_resolvers}
              className="border border-hack-danger/60 px-4 py-2 font-mono text-xs uppercase tracking-wider text-hack-danger disabled:opacity-50"
            >
              <Trash2 className="mr-2 inline h-4 w-4" />
              Delete Resolvers
            </button>
          </div>

          {saveMutation.isError && (
            <div className="mt-3 space-y-1 text-sm text-hack-danger">
              <div>{apiMessage || "Failed to save PureDNS resolver config."}</div>
              {apiErrors?.slice(0, 8).map((err) => (
                <div key={err} className="font-mono text-xs">
                  {err}
                </div>
              ))}
            </div>
          )}

          {deleteMutation.isError && (
            <div className="mt-3 text-sm text-hack-danger">
              Failed to delete PureDNS resolver config.
            </div>
          )}
        </>
      )}
    </div>
  );
};

export default PureDNSResolverConfigPanel;
