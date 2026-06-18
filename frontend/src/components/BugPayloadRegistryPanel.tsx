import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import { Database, Loader2, RefreshCw, ShieldCheck, ToggleLeft, ToggleRight } from "lucide-react";
import {
  getBugPayloadPacks,
  getBugPayloads,
  updateBugPayloadPackEnabled,
  type BugPayload,
  type BugPayloadPack,
} from "../api/targets";

const parseJSONValue = (value: any) => {
  if (!value) return {};
  if (typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch {
      return {};
    }
  }
  return value;
};

const asArray = (value: any): any[] => (Array.isArray(value) ? value : []);

const Pill = ({
  children,
  tone = "neutral",
}: {
  children: React.ReactNode;
  tone?: "neutral" | "primary" | "warning" | "danger";
}) => {
  const classes = {
    neutral: "border-hack-border text-hack-dim bg-black/30",
    primary: "border-hack-primary text-hack-primary bg-hack-primary/10",
    warning: "border-hack-warning text-hack-warning bg-hack-warning/10",
    danger: "border-hack-danger text-hack-danger bg-hack-danger/10",
  };

  return (
    <span
      className={clsx(
        "border px-2 py-1 font-mono text-[10px] uppercase tracking-wider",
        classes[tone],
      )}
    >
      {children}
    </span>
  );
};

const PackCard = ({
  pack,
  busy,
  onToggle,
}: {
  pack: BugPayloadPack;
  busy: boolean;
  onToggle: (pack: BugPayloadPack) => void;
}) => {
  const metadata = parseJSONValue(pack.metadata);

  return (
    <div className="border border-hack-border bg-black/30 p-3">
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Database className="h-4 w-4 text-hack-primary" />
        <span className="font-mono text-xs uppercase tracking-wider text-white">
          {pack.name}
        </span>
        <Pill tone={pack.enabled ? "primary" : "danger"}>
          {pack.enabled ? "enabled" : "disabled"}
        </Pill>
        <Pill>{pack.version}</Pill>
        <Pill tone={pack.trust_level === "trusted_core" ? "primary" : "warning"}>
          {pack.trust_level}
        </Pill>
        {pack.locked && <Pill>locked</Pill>}

        <button
          type="button"
          disabled={busy}
          onClick={() => onToggle(pack)}
          className={clsx(
            "ml-auto flex items-center gap-2 border px-2 py-1 font-mono text-[10px] uppercase tracking-wider disabled:opacity-50",
            pack.enabled
              ? "border-hack-danger/60 text-hack-danger"
              : "border-hack-primary/60 text-hack-primary",
          )}
        >
          {pack.enabled ? (
            <ToggleRight className="h-3 w-3" />
          ) : (
            <ToggleLeft className="h-3 w-3" />
          )}
          {pack.enabled ? "Disable Pack" : "Enable Pack"}
        </button>
      </div>

      <div className="mb-2 break-all font-mono text-[11px] text-hack-primary">
        {pack.key}
      </div>

      <p className="mb-3 text-xs text-hack-dim">{pack.description}</p>

      <div className="flex flex-wrap gap-2">
        <Pill>payloads {pack.payload_count}</Pill>
        <Pill>safety {pack.safety_score}</Pill>
        <Pill>quality {pack.quality_score}</Pill>
        <Pill>noise {pack.noise_score}</Pill>
        <Pill>false positive {pack.false_positive_rate}%</Pill>
        <Pill>{pack.update_mode}</Pill>
        {metadata?.rollback_ready && <Pill tone="primary">rollback ready</Pill>}
        {metadata?.metadata_only && <Pill tone="primary">metadata only</Pill>}
        {metadata?.payload_execution === false && <Pill>no execution</Pill>}
      </div>
    </div>
  );
};

const PayloadCard = ({ payload }: { payload: BugPayload }) => {
  const metadata = parseJSONValue(payload.metadata);
  const refs = asArray(payload.owasp_refs);
  const tags = asArray(payload.tags);

  return (
    <div className="border border-hack-border bg-black/20 p-4">
      <div className="mb-2 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <Database className="h-4 w-4 text-hack-primary" />
            <span className="font-mono text-sm uppercase tracking-wider text-white">
              {payload.name}
            </span>
            <Pill tone={payload.enabled ? "primary" : "danger"}>
              {payload.enabled ? "enabled" : "disabled"}
            </Pill>
            <Pill>{payload.bug_type}</Pill>
            <Pill tone={payload.safety_class === "safe" ? "primary" : "neutral"}>
              {payload.safety_class}
            </Pill>
          </div>

          <div className="break-all font-mono text-[11px] text-hack-primary">
            {payload.key}
          </div>
        </div>
      </div>

      <p className="mb-3 text-sm text-hack-dim">{payload.description}</p>

      <div className="mb-3 flex flex-wrap gap-2">
        <Pill>mode {payload.mode}</Pill>
        <Pill>context {payload.context}</Pill>
        <Pill>test L{payload.test_level}</Pill>
        <Pill>safety L{payload.safety_level}</Pill>
        <Pill>{payload.source}</Pill>
        <Pill>{payload.version}</Pill>
        {payload.requires_approval && <Pill tone="warning">approval</Pill>}
      </div>

      <div className="mb-3 border border-hack-border bg-black/40 p-3">
        <div className="mb-1 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
          Payload Template
        </div>
        <div className="break-all font-mono text-xs text-white">
          {payload.payload_template || "-"}
        </div>
      </div>

      <div className="flex flex-wrap gap-2">
        {refs.map((item, idx) => (
          <Pill key={`ref-${idx}`}>{String(item)}</Pill>
        ))}
        {tags.map((item, idx) => (
          <Pill key={`tag-${idx}`}>{String(item)}</Pill>
        ))}
      </div>

      <details className="mt-3">
        <summary className="cursor-pointer font-mono text-[10px] uppercase tracking-wider text-hack-dim hover:text-white">
          Metadata JSON
        </summary>
        <pre className="mt-2 max-h-[220px] overflow-auto border border-hack-border bg-black/40 p-3 text-xs text-hack-dim">
          {JSON.stringify(metadata, null, 2)}
        </pre>
      </details>
    </div>
  );
};

const BugPayloadRegistryPanel = ({ enabled = true }: { enabled?: boolean }) => {
  const queryClient = useQueryClient();
  const [bugType, setBugType] = useState("");
  const [safetyClass, setSafetyClass] = useState("");
  const [enabledFilter, setEnabledFilter] = useState("");
  const [search, setSearch] = useState("");

  const packsQuery = useQuery({
    queryKey: ["bug-payload-packs"],
    queryFn: () => getBugPayloadPacks({ limit: 50 }),
    enabled,
    staleTime: 60_000,
  });

  const packToggleMutation = useMutation({
    mutationFn: (pack: BugPayloadPack) =>
      updateBugPayloadPackEnabled(pack.id, !pack.enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["bug-payload-packs"] });
      queryClient.invalidateQueries({ queryKey: ["bug-payloads"] });
      queryClient.invalidateQueries({ queryKey: ["target"] });
    },
  });

  const query = useQuery({
    queryKey: ["bug-payloads", bugType, safetyClass, enabledFilter],
    queryFn: () =>
      getBugPayloads({
        bug_type: bugType || undefined,
        safety_class: safetyClass || undefined,
        enabled: enabledFilter || undefined,
        limit: 200,
      }),
    enabled,
    staleTime: 20_000,
  });

  const payloadPacks = packsQuery.data?.data || [];
  const payloads = query.data?.data || [];

  const filteredPayloads = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return payloads;

    return payloads.filter((item) =>
      [
        item.key,
        item.name,
        item.description,
        item.bug_type,
        item.safety_class,
        item.context,
        item.source,
        item.version,
      ]
        .join(" ")
        .toLowerCase()
        .includes(q),
    );
  }, [payloads, search]);

  if (!enabled) {
    return (
      <div className="border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
        Bug Payload Registry is disabled for this account.
      </div>
    );
  }

  return (
    <div className="space-y-4 border border-hack-border bg-black/20 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="mb-1 flex items-center gap-2">
            <ShieldCheck className="h-4 w-4 text-hack-primary" />
            <h3 className="font-mono text-sm uppercase tracking-wider text-white">
              Bug Payload Registry
            </h3>
          </div>
          <p className="text-sm text-hack-dim">
            Metadata-only payload registry for future safe/approved testing. v3.8.3 does not execute payloads.
          </p>
        </div>

        <button
          type="button"
          onClick={() => query.refetch()}
          disabled={query.isFetching}
          className="border border-hack-border px-3 py-2 font-mono text-xs uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-50"
        >
          {query.isFetching ? (
            <Loader2 className="inline h-3 w-3 animate-spin" />
          ) : (
            <RefreshCw className="inline h-3 w-3" />
          )}{" "}
          Refresh
        </button>
      </div>

      {payloadPacks.length > 0 && (
        <div className="space-y-2">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Payload Packs
          </div>
          <div className="grid gap-3 xl:grid-cols-2">
            {payloadPacks.map((pack) => (
              <PackCard
                key={pack.id}
                pack={pack}
                busy={packToggleMutation.isPending}
                onToggle={(item) => packToggleMutation.mutate(item)}
              />
            ))}
          </div>
        </div>
      )}

      <div className="grid gap-3 md:grid-cols-4">
        <input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search payloads..."
          className="border border-hack-border bg-black/40 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
        />

        <select
          value={bugType}
          onChange={(event) => setBugType(event.target.value)}
          className="border border-hack-border bg-black/40 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
        >
          <option value="">All bug types</option>
          <option value="xss">XSS</option>
          <option value="open_redirect">Open Redirect</option>
          <option value="security_headers">Security Headers</option>
          <option value="cors">CORS</option>
          <option value="api">API</option>
        </select>

        <select
          value={safetyClass}
          onChange={(event) => setSafetyClass(event.target.value)}
          className="border border-hack-border bg-black/40 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
        >
          <option value="">All safety classes</option>
          <option value="inert">Inert</option>
          <option value="safe">Safe</option>
          <option value="controlled">Controlled</option>
        </select>

        <select
          value={enabledFilter}
          onChange={(event) => setEnabledFilter(event.target.value)}
          className="border border-hack-border bg-black/40 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
        >
          <option value="">All states</option>
          <option value="true">Enabled</option>
          <option value="false">Disabled</option>
        </select>
      </div>

      <div className="font-mono text-xs text-hack-dim">
        Showing {filteredPayloads.length} of {query.data?.total_count ?? payloads.length} payloads
      </div>

      <div className="grid gap-3 xl:grid-cols-2">
        {filteredPayloads.map((payload) => (
          <PayloadCard key={payload.id} payload={payload} />
        ))}
      </div>

      {!query.isLoading && filteredPayloads.length === 0 && (
        <div className="border border-hack-border bg-black/30 p-4 text-sm text-hack-dim">
          No bug payloads matched the current filters.
        </div>
      )}
    </div>
  );
};

export default BugPayloadRegistryPanel;
