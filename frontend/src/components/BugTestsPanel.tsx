import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Bug,
  Loader2,
  PlayCircle,
  RefreshCw,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import clsx from "clsx";
import {
  createTargetBugTestRun,
  deleteTargetBugTestRun,
  deleteTargetBugTestResult,
  getTargetBugTestResults,
  getTargetBugTestRuns,
  type TargetBugTestResult,
  type TargetBugTestRun,
} from "../api/targets";

type Props = {
  targetId: number;
  enabled?: boolean;
};

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

const formatDate = (value?: string | null) => {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
};

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

const toneForStatus = (value: string): "neutral" | "primary" | "warning" | "danger" => {
  switch (String(value || "").toLowerCase()) {
    case "completed":
    case "passed":
    case "executed":
      return "primary";
    case "warning":
    case "running":
    case "queued":
    case "candidate":
    case "needs_manual_validation":
    case "inconclusive":
      return "warning";
    case "failed":
    case "blocked":
    case "blocked_by_policy":
      return "danger";
    default:
      return "neutral";
  }
};

const RunCard = ({
  run,
  selected,
  onSelect,
}: {
  run: TargetBugTestRun;
  selected: boolean;
  onSelect: (run: TargetBugTestRun) => void;
}) => {
  const output = parseJSONValue(run.output_json);
  const bugTypes = asArray(run.bug_types);

  return (
    <button
      type="button"
      onClick={() => onSelect(run)}
      className={clsx(
        "w-full border p-3 text-left transition-colors",
        selected
          ? "border-hack-primary bg-hack-primary/10"
          : "border-hack-border bg-black/20 hover:border-hack-primary/60",
      )}
    >
      <div className="mb-2 flex flex-wrap items-center gap-2">
        <Bug className="h-4 w-4 text-hack-primary" />
        <span className="font-mono text-sm uppercase tracking-wider text-white">
          {run.profile}
        </span>
        <Pill tone={toneForStatus(run.status)}>{run.status}</Pill>
        <Pill tone={toneForStatus(run.policy_status)}>{run.policy_status}</Pill>
      </div>

      <div className="flex flex-wrap gap-2">
        <Pill>level {run.test_level}</Pill>
        <Pill>safety {run.safety_level}</Pill>
        <Pill>results {output?.results_created ?? 0}</Pill>
      </div>

      {bugTypes.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1">
          {bugTypes.slice(0, 6).map((item, idx) => (
            <Pill key={idx}>{String(item)}</Pill>
          ))}
        </div>
      )}

      <div className="mt-2 font-mono text-[10px] text-hack-dim">
        {formatDate(run.created_at)}
      </div>
    </button>
  );
};

const ResultCard = ({
  result,
  busy = false,
  onDelete,
}: {
  result: TargetBugTestResult;
  busy?: boolean;
  onDelete?: (result: TargetBugTestResult) => void;
}) => {
  const evidence = parseJSONValue(result.evidence_json);
  const tags = asArray(result.tags);
  const refs = asArray(result.owasp_refs);

  return (
    <div className="border border-hack-border bg-black/20 p-4">
      <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <Bug className="h-4 w-4 text-hack-primary" />
          <span className="font-mono text-sm uppercase tracking-wider text-white">
            {result.bug_type}
          </span>
          <Pill tone={toneForStatus(result.status)}>{result.status}</Pill>
          <Pill>{result.confidence}</Pill>
          <Pill tone={result.severity_hint === "low" ? "warning" : "neutral"}>
            {result.severity_hint}
          </Pill>
        </div>
        <div className="flex items-center gap-2">
          <div className="font-mono text-[10px] text-hack-dim">
            #{result.id}
          </div>
          {onDelete && (
            <button
              type="button"
              onClick={() => onDelete(result)}
              disabled={busy}
              className="border border-hack-danger/60 px-2 py-1 font-mono text-[10px] uppercase tracking-wider text-hack-danger disabled:opacity-50"
              title="Soft-delete this bug test result"
            >
              <Trash2 className="inline h-3 w-3" /> Delete
            </button>
          )}
        </div>
      </div>

      <div className="mb-2 font-mono text-xs text-hack-primary">
        {result.test_name}
      </div>

      {evidence?.note && (
        <div className="mb-2 text-sm text-hack-dim">{String(evidence.note)}</div>
      )}

      {evidence?.url && (
        <div className="mb-1 break-all font-mono text-xs text-white">
          URL: {String(evidence.url)}
        </div>
      )}

      {evidence?.asset && (
        <div className="mb-1 break-all font-mono text-xs text-white">
          Asset: {String(evidence.asset)}
        </div>
      )}

      <div className="mt-3 flex flex-wrap gap-2">
        {refs.map((item, idx) => (
          <Pill key={`ref-${idx}`}>{String(item)}</Pill>
        ))}
        {tags.map((item, idx) => (
          <Pill key={`tag-${idx}`}>{String(item)}</Pill>
        ))}
      </div>

      <details className="mt-3">
        <summary className="cursor-pointer font-mono text-[10px] uppercase tracking-wider text-hack-dim hover:text-white">
          Evidence JSON
        </summary>
        <pre className="mt-2 max-h-[260px] overflow-auto border border-hack-border bg-black/40 p-3 text-xs text-hack-dim">
          {JSON.stringify(evidence, null, 2)}
        </pre>
      </details>
    </div>
  );
};

const BugTestsPanel = ({ targetId, enabled = true }: Props) => {
  const queryClient = useQueryClient();
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);

  const runsQuery = useQuery({
    queryKey: ["target", targetId, "bug-test-runs"],
    queryFn: () => getTargetBugTestRuns(targetId, 30),
    enabled: Boolean(targetId) && enabled,
    staleTime: 15_000,
  });

  const runs = runsQuery.data?.data || [];
  const selectedRun =
    runs.find((item) => item.id === selectedRunId) || runs[0] || null;

  const resultsQuery = useQuery({
    queryKey: ["target", targetId, "bug-test-results", selectedRun?.id || "all"],
    queryFn: () => getTargetBugTestResults(targetId, 80, selectedRun?.id),
    enabled: Boolean(targetId) && enabled,
    staleTime: 15_000,
  });

  const results = resultsQuery.data?.data || [];

  const createRunMutation = useMutation({
    mutationFn: () =>
      createTargetBugTestRun(targetId, {
        profile: "passive",
        bug_types: ["xss", "open_redirect", "security_headers"],
        owasp_refs: ["OWASP-WSTG-INPV", "OWASP-WSTG-CONF"],
        safety_level: 0,
        test_level: 0,
        input_json: {
          source: "bug_tests_panel",
          active_testing: false,
        },
      }),
    onSuccess: (run) => {
      setSelectedRunId(run.id);
      queryClient.invalidateQueries({
        queryKey: ["target", targetId, "bug-test-runs"],
      });
      queryClient.invalidateQueries({
        queryKey: ["target", targetId, "bug-test-results"],
      });
    },
  });

  const deleteRunMutation = useMutation({
    mutationFn: (run: TargetBugTestRun) => deleteTargetBugTestRun(targetId, run.id),
    onSuccess: () => {
      setSelectedRunId(null);
      queryClient.invalidateQueries({
        queryKey: ["target", targetId, "bug-test-runs"],
      });
      queryClient.invalidateQueries({
        queryKey: ["target", targetId, "bug-test-results"],
      });
    },
  });

  const deleteResultMutation = useMutation({
    mutationFn: (result: TargetBugTestResult) =>
      deleteTargetBugTestResult(targetId, result.id),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["target", targetId, "bug-test-results"],
      });
    },
  });

  const resultSummary = useMemo(() => {
    return results.reduce<Record<string, number>>((acc, item) => {
      acc[item.bug_type] = (acc[item.bug_type] || 0) + 1;
      return acc;
    }, {});
  }, [results]);

  if (!enabled) {
    return (
      <div className="border border-hack-border bg-black/20 p-5 text-sm text-hack-dim">
        Safe Bug Testing is disabled for this account.
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="border border-hack-border bg-black/30 p-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <div className="flex items-center gap-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
              <ShieldCheck className="h-4 w-4" /> Safe Bug Testing Engine
            </div>
            <p className="mt-2 max-w-3xl text-sm text-hack-dim">
              v3.8.0 foundation runs passive/stub checks only. It does not send
              exploit payloads, perform destructive tests, validate exploitation,
              or change severity automatically. Results are candidates and need
              manual validation.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => runsQuery.refetch()}
              disabled={runsQuery.isFetching}
              className="hack-btn-ghost border border-hack-border px-3 py-2 text-xs uppercase tracking-wider text-hack-dim disabled:opacity-50"
            >
              <RefreshCw className={clsx("h-4 w-4", runsQuery.isFetching && "animate-spin")} />
              Refresh
            </button>

            <button
              type="button"
              onClick={() => createRunMutation.mutate()}
              disabled={createRunMutation.isPending}
              className="hack-btn border border-hack-primary px-3 py-2 text-xs uppercase tracking-wider text-hack-primary disabled:opacity-50"
            >
              {createRunMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <PlayCircle className="h-4 w-4" />
              )}
              Run Passive Bug Test
            </button>
          </div>
        </div>

        <div className="mt-4 grid gap-3 md:grid-cols-4">
          <div className="border border-hack-border bg-black/20 p-3">
            <div className="font-mono text-[10px] uppercase text-hack-dim">
              Runs
            </div>
            <div className="mt-1 font-mono text-xl text-white">{runs.length}</div>
          </div>
          <div className="border border-hack-border bg-black/20 p-3">
            <div className="font-mono text-[10px] uppercase text-hack-dim">
              Results
            </div>
            <div className="mt-1 font-mono text-xl text-white">{results.length}</div>
          </div>
          <div className="border border-hack-border bg-black/20 p-3">
            <div className="font-mono text-[10px] uppercase text-hack-dim">
              XSS Candidates
            </div>
            <div className="mt-1 font-mono text-xl text-white">
              {resultSummary.xss || 0}
            </div>
          </div>
          <div className="border border-hack-border bg-black/20 p-3">
            <div className="font-mono text-[10px] uppercase text-hack-dim">
              Redirect Candidates
            </div>
            <div className="mt-1 font-mono text-xl text-white">
              {resultSummary.open_redirect || 0}
            </div>
          </div>
        </div>

        <div className="mt-4 border border-hack-warning/60 bg-hack-warning/10 p-3 text-sm text-hack-warning">
          <div className="mb-1 flex items-center gap-2 font-mono text-xs uppercase tracking-wider">
            <AlertTriangle className="h-4 w-4" /> Guardrails
          </div>
          Passive/stub only. Active testing is false. Payload execution,
          exploitation, brute force, destructive testing, data extraction, and
          automatic severity/report changes are disabled.
        </div>

        {createRunMutation.isError && (
          <div className="mt-3 border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger">
            {(createRunMutation.error as any)?.response?.data?.message ||
              "Failed to create bug test run"}
          </div>
        )}
      </div>

      <div className="grid gap-4 xl:grid-cols-[360px_1fr]">
        <div className="space-y-2">
          {runsQuery.isLoading ? (
            <div className="flex items-center gap-2 text-sm text-hack-dim">
              <Loader2 className="h-4 w-4 animate-spin" /> Loading bug test runs...
            </div>
          ) : runs.length === 0 ? (
            <div className="border border-hack-border bg-black/20 p-5 text-sm text-hack-dim">
              No bug test runs yet.
            </div>
          ) : (
            runs.map((run) => (
              <RunCard
                key={run.id}
                run={run}
                selected={selectedRun?.id === run.id}
                onSelect={(item) => setSelectedRunId(item.id)}
              />
            ))
          )}
        </div>

        <div className="min-w-0 space-y-3">
          {resultsQuery.isFetching && (
            <div className="flex items-center gap-2 text-sm text-hack-dim">
              <Loader2 className="h-4 w-4 animate-spin" /> Loading results...
            </div>
          )}

          {selectedRun && (
            <div className="border border-hack-border bg-black/20 p-3">
              <div className="mb-2 flex flex-wrap items-center gap-2">
                <Pill tone={toneForStatus(selectedRun.status)}>
                  {selectedRun.status}
                </Pill>
                <Pill tone={toneForStatus(selectedRun.policy_status)}>
                  policy: {selectedRun.policy_status}
                </Pill>
                <Pill>profile: {selectedRun.profile}</Pill>
                <Pill>active_testing: false</Pill>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="font-mono text-xs text-hack-dim">
                  Run #{selectedRun.id} · {formatDate(selectedRun.created_at)}
                </div>
                <button
                  type="button"
                  onClick={() => {
                    if (window.confirm(`Delete bug test run #${selectedRun.id} and its results? This is a soft delete.`)) {
                      deleteRunMutation.mutate(selectedRun);
                    }
                  }}
                  disabled={deleteRunMutation.isPending}
                  className="border border-hack-danger/60 px-2 py-1 font-mono text-[10px] uppercase tracking-wider text-hack-danger disabled:opacity-50"
                  title="Soft-delete this bug test run"
                >
                  <Trash2 className="inline h-3 w-3" /> Delete Run
                </button>
              </div>
            </div>
          )}

          {results.length === 0 ? (
            <div className="border border-hack-border bg-black/20 p-5 text-sm text-hack-dim">
              No results for this run.
            </div>
          ) : (
            results.map((result) => (
              <ResultCard
                key={result.id}
                result={result}
                busy={deleteResultMutation.isPending}
                onDelete={(item) => {
                  if (window.confirm(`Delete bug test result #${item.id}? This is a soft delete.`)) {
                    deleteResultMutation.mutate(item);
                  }
                }}
              />
            ))
          )}
        </div>
      </div>
    </div>
  );
};

export default BugTestsPanel;
