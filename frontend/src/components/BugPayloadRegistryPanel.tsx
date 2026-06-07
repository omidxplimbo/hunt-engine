import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import clsx from "clsx";
import { Database, Loader2, RefreshCw, ShieldCheck } from "lucide-react";
import {
  getBugPayloads,
  type BugPayload,
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
  const [bugType, setBugType] = useState("");
  const [safetyClass, setSafetyClass] = useState("");
  const [enabledFilter, setEnabledFilter] = useState("");
  const [search, setSearch] = useState("");

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
