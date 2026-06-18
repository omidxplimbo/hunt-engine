import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import clsx from "clsx";
import { Database, Loader2, RefreshCw, ShieldCheck, ToggleLeft, ToggleRight } from "lucide-react";
import {
  getBugPatternPacks,
  getBugPatterns,
  updateBugPatternEnabled,
  updateBugPatternPackEnabled,
  type BugPattern,
  type BugPatternPack,
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
  pack: BugPatternPack;
  busy: boolean;
  onToggle: (pack: BugPatternPack) => void;
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
        <Pill>patterns {pack.pattern_count}</Pill>
        <Pill>safety {pack.safety_score}</Pill>
        <Pill>quality {pack.quality_score}</Pill>
        <Pill>noise {pack.noise_score}</Pill>
        <Pill>false positive {pack.false_positive_rate}%</Pill>
        <Pill>{pack.update_mode}</Pill>
        {metadata?.rollback_ready && <Pill tone="primary">rollback ready</Pill>}
        {metadata?.external_update === false && <Pill>local only</Pill>}
      </div>
    </div>
  );
};

const PatternCard = ({
  pattern,
  busy,
  onToggle,
}: {
  pattern: BugPattern;
  busy: boolean;
  onToggle: (pattern: BugPattern) => void;
}) => {
  const matcher = parseJSONValue(pattern.matcher_json);
  const refs = asArray(pattern.owasp_refs);
  const tags = asArray(pattern.tags);

  return (
    <div className="border border-hack-border bg-black/20 p-4">
      <div className="mb-2 flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <Database className="h-4 w-4 text-hack-primary" />
            <span className="font-mono text-sm uppercase tracking-wider text-white">
              {pattern.name}
            </span>
            <Pill tone={pattern.enabled ? "primary" : "danger"}>
              {pattern.enabled ? "enabled" : "disabled"}
            </Pill>
            <Pill>{pattern.bug_type}</Pill>
            <Pill>{pattern.mode}</Pill>
          </div>

          <div className="break-all font-mono text-[11px] text-hack-primary">
            {pattern.key}
          </div>
        </div>

        <button
          type="button"
          disabled={busy}
          onClick={() => onToggle(pattern)}
          className={clsx(
            "flex items-center gap-2 border px-3 py-2 font-mono text-[10px] uppercase tracking-wider disabled:opacity-50",
            pattern.enabled
              ? "border-hack-danger/60 text-hack-danger"
              : "border-hack-primary/60 text-hack-primary",
          )}
        >
          {pattern.enabled ? (
            <ToggleRight className="h-4 w-4" />
          ) : (
            <ToggleLeft className="h-4 w-4" />
          )}
          {pattern.enabled ? "Disable" : "Enable"}
        </button>
      </div>

      <p className="mb-3 text-sm text-hack-dim">{pattern.description}</p>

      <div className="mb-3 flex flex-wrap gap-2">
        <Pill>severity {pattern.severity_hint}</Pill>
        <Pill>confidence {pattern.confidence_default}</Pill>
        <Pill>test L{pattern.test_level}</Pill>
        <Pill>safety L{pattern.safety_level}</Pill>
        <Pill>{pattern.source}</Pill>
        <Pill>{pattern.version}</Pill>
        {pattern.safe_by_default && <Pill tone="primary">safe default</Pill>}
        {pattern.requires_approval && <Pill tone="warning">approval</Pill>}
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
          Matcher JSON
        </summary>
        <pre className="mt-2 max-h-[220px] overflow-auto border border-hack-border bg-black/40 p-3 text-xs text-hack-dim">
          {JSON.stringify(matcher, null, 2)}
        </pre>
      </details>
    </div>
  );
};

const BugPatternRegistryPanel = ({ enabled = true }: { enabled?: boolean }) => {
  const queryClient = useQueryClient();
  const [bugType, setBugType] = useState("");
  const [enabledFilter, setEnabledFilter] = useState("");
  const [search, setSearch] = useState("");

  const packsQuery = useQuery({
    queryKey: ["bug-pattern-packs"],
    queryFn: () => getBugPatternPacks({ limit: 50 }),
    enabled,
    staleTime: 60_000,
  });

  const query = useQuery({
    queryKey: ["bug-patterns", bugType, enabledFilter],
    queryFn: () =>
      getBugPatterns({
        bug_type: bugType || undefined,
        enabled: enabledFilter || undefined,
        limit: 200,
      }),
    enabled,
    staleTime: 20_000,
  });

  const packToggleMutation = useMutation({
    mutationFn: (pack: BugPatternPack) =>
      updateBugPatternPackEnabled(pack.id, !pack.enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["bug-pattern-packs"] });
      queryClient.invalidateQueries({ queryKey: ["bug-patterns"] });
      queryClient.invalidateQueries({ queryKey: ["target"] });
    },
  });

  const toggleMutation = useMutation({
    mutationFn: (pattern: BugPattern) =>
      updateBugPatternEnabled(pattern.id, !pattern.enabled),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["bug-patterns"] });
      queryClient.invalidateQueries({ queryKey: ["target"] });
    },
  });

  const patternPacks = packsQuery.data?.data || [];
  const patterns = query.data?.data || [];

  const filteredPatterns = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return patterns;

    return patterns.filter((item) =>
      [
        item.key,
        item.name,
        item.description,
        item.bug_type,
        item.source,
        item.version,
      ]
        .join(" ")
        .toLowerCase()
        .includes(q),
    );
  }, [patterns, search]);

  if (!enabled) {
    return (
      <div className="border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
        Bug Pattern Registry is disabled for this account.
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
              Bug Pattern Registry
            </h3>
          </div>
          <p className="text-sm text-hack-dim">
            Registry-backed safe testing patterns. Disabling a pattern prevents it from generating future bug test results.
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

      {patternPacks.length > 0 && (
        <div className="space-y-2">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Pattern Packs
          </div>
          <div className="grid gap-3 xl:grid-cols-2">
            {patternPacks.map((pack) => (
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

      <div className="grid gap-3 md:grid-cols-3">
        <input
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder="Search patterns..."
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
        Showing {filteredPatterns.length} of {query.data?.total_count ?? patterns.length} patterns
      </div>

      <div className="grid gap-3 xl:grid-cols-2">
        {filteredPatterns.map((pattern) => (
          <PatternCard
            key={pattern.id}
            pattern={pattern}
            busy={toggleMutation.isPending}
            onToggle={(item) => toggleMutation.mutate(item)}
          />
        ))}
      </div>

      {!query.isLoading && filteredPatterns.length === 0 && (
        <div className="border border-hack-border bg-black/30 p-4 text-sm text-hack-dim">
          No bug patterns matched the current filters.
        </div>
      )}
    </div>
  );
};

export default BugPatternRegistryPanel;
