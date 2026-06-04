import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  Bot,
  Brain,
  CheckCircle2,
  ClipboardList,
  Loader2,
  RefreshCw,
  ShieldCheck,
  Sparkles,
} from "lucide-react";
import clsx from "clsx";
import {
  getTargetAgentRuns,
  runTargetSummaryAgent,
  runTargetTriageAgent,
  type TargetAgentRun,
} from "../api/targets";

type Props = {
  targetId: number;
  agentRunsEnabled?: boolean;
  triageEnabled?: boolean;
  summaryEnabled?: boolean;
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

const cleanText = (value: any) => {
  const text = String(value ?? "").trim();
  return text === "<nil>" ? "" : text;
};

const formatDate = (value?: string | null) => {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
};

const StatusPill = ({
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

const toneForPolicy = (value: string): "neutral" | "primary" | "warning" | "danger" => {
  switch (String(value || "").toLowerCase()) {
    case "allowed":
      return "primary";
    case "warning":
    case "unknown":
      return "warning";
    case "blocked":
      return "danger";
    default:
      return "neutral";
  }
};

const toneForStatus = (value: string): "neutral" | "primary" | "warning" | "danger" => {
  switch (String(value || "").toLowerCase()) {
    case "completed":
      return "primary";
    case "running":
    case "pending":
      return "warning";
    case "failed":
      return "danger";
    default:
      return "neutral";
  }
};

const ListBlock = ({ title, items }: { title: string; items: any[] }) => {
  if (!items.length) return null;

  return (
    <div className="border border-hack-border bg-black/20 p-4">
      <div className="mb-3 font-mono text-sm uppercase tracking-wider text-hack-primary">
        {title}
      </div>
      <ul className="space-y-2 text-sm text-hack-dim">
        {items.map((item, idx) => (
          <li key={idx} className="flex gap-2">
            <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-hack-primary" />
            <span>{String(item)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
};

const SummaryOutput = ({ output }: { output: any }) => {
  const interestingAssets = asArray(output?.most_interesting_assets);
  const whatNext = asArray(output?.what_to_test_next);
  const safety = asArray(output?.policy_safety_notes);
  const coverage = output?.coverage_summary || {};

  return (
    <div className="space-y-4">
      {cleanText(output?.attack_surface_summary) && (
        <div className="border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
          <div className="mb-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Attack Surface Summary
          </div>
          {cleanText(output.attack_surface_summary)}
        </div>
      )}

      <div className="grid gap-3 md:grid-cols-4">
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Total Assets
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {coverage?.total_assets ?? 0}
          </div>
        </div>
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Live Assets
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {coverage?.total_live_assets ?? 0}
          </div>
        </div>
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Findings
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {coverage?.total_findings ?? 0}
          </div>
        </div>
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            URLs
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {coverage?.total_urls ?? 0}
          </div>
        </div>
      </div>

      {cleanText(output?.risk_narrative) && (
        <div className="border border-hack-warning/50 bg-hack-warning/10 p-4 text-sm text-hack-warning">
          <div className="mb-2 flex items-center gap-2 font-mono text-sm uppercase tracking-wider">
            <AlertTriangle className="h-4 w-4" /> Risk Narrative
          </div>
          {cleanText(output.risk_narrative)}
        </div>
      )}

      {interestingAssets.length > 0 && (
        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-3 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Most Interesting Assets
          </div>
          <div className="space-y-3">
            {interestingAssets.slice(0, 8).map((asset: any, idx: number) => (
              <div key={`${asset.value || idx}`} className="border border-hack-border/70 bg-black/30 p-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="break-all font-mono text-sm text-white">
                    {asset.value || "-"}
                  </div>
                  <StatusPill tone="primary">
                    score {asset.interest_score ?? 0}
                  </StatusPill>
                </div>
                <div className="mt-2 flex flex-wrap gap-2">
                  {asArray(asset.reason).map((reason: any, reasonIdx: number) => (
                    <StatusPill key={reasonIdx}>{reason}</StatusPill>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <ListBlock title="What To Test Next" items={whatNext} />
      <ListBlock title="Policy Safety Notes" items={safety} />
    </div>
  );
};

const TriageOutput = ({ output }: { output: any }) => {
  const topFindings = asArray(output?.top_interesting_findings);
  const manualTests = asArray(output?.recommended_manual_tests);
  const validation = asArray(output?.manual_validation_steps);
  const safety = asArray(output?.policy_safety_notes);

  return (
    <div className="space-y-4">
      {cleanText(output?.summary) && (
        <div className="border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
          <div className="mb-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Triage Summary
          </div>
          {cleanText(output.summary)}
        </div>
      )}

      {topFindings.length > 0 && (
        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-3 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Top Interesting Findings
          </div>
          <div className="space-y-3">
            {topFindings.slice(0, 8).map((finding: any, idx: number) => (
              <div key={`${finding.finding_id || idx}`} className="border border-hack-border/70 bg-black/30 p-3">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div className="font-mono text-sm text-white">
                    #{finding.finding_id} · {finding.title || "-"}
                  </div>
                  <StatusPill tone="primary">
                    score {finding.interest_score ?? 0}
                  </StatusPill>
                </div>
                <div className="mt-2 flex flex-wrap gap-2">
                  <StatusPill>{finding.severity || "unknown"}</StatusPill>
                  <StatusPill>{finding.category || "uncategorized"}</StatusPill>
                  <StatusPill>{finding.source_tool || "unknown tool"}</StatusPill>
                  {finding.bug_bounty_value && (
                    <StatusPill tone="warning">{finding.bug_bounty_value}</StatusPill>
                  )}
                </div>
                {asArray(finding.why_interesting).length > 0 && (
                  <ul className="mt-3 space-y-1 text-xs text-hack-dim">
                    {asArray(finding.why_interesting).map((reason: any, reasonIdx: number) => (
                      <li key={reasonIdx}>• {String(reason)}</li>
                    ))}
                  </ul>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {manualTests.length > 0 && (
        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-3 font-mono text-sm uppercase tracking-wider text-hack-primary">
            Recommended Manual Tests
          </div>
          <div className="space-y-3">
            {manualTests.slice(0, 8).map((test: any, idx: number) => (
              <div key={idx} className="border border-hack-border/70 bg-black/30 p-3">
                <div className="flex flex-wrap items-center gap-2">
                  <StatusPill tone="primary">{test.priority || "priority"}</StatusPill>
                  <StatusPill>{test.safety_level || "safety"}</StatusPill>
                  <span className="font-mono text-sm text-white">
                    {test.test_type || "manual test"}
                  </span>
                </div>
                {test.why && <div className="mt-2 text-sm text-hack-dim">{test.why}</div>}
              </div>
            ))}
          </div>
        </div>
      )}

      <ListBlock title="Manual Validation Steps" items={validation} />
      <ListBlock title="Policy Safety Notes" items={safety} />
    </div>
  );
};

const AgentRunDetails = ({ run }: { run: TargetAgentRun }) => {
  const output = useMemo(() => parseJSONValue(run.output_json), [run.output_json]);

  return (
    <div className="space-y-4">
      {run.agent_type === "summary" ? (
        <SummaryOutput output={output} />
      ) : run.agent_type === "triage" ? (
        <TriageOutput output={output} />
      ) : (
        <pre className="max-h-[420px] overflow-auto border border-hack-border bg-black/40 p-3 text-xs text-hack-dim">
          {JSON.stringify(output, null, 2)}
        </pre>
      )}

      <details className="border border-hack-border bg-black/20 p-3">
        <summary className="cursor-pointer font-mono text-xs uppercase tracking-wider text-hack-dim">
          Raw Agent Output JSON
        </summary>
        <pre className="mt-3 max-h-[420px] overflow-auto bg-black/40 p-3 text-xs text-hack-dim">
          {JSON.stringify(output, null, 2)}
        </pre>
      </details>
    </div>
  );
};

const AgentRunsPanel = ({
  targetId,
  agentRunsEnabled = true,
  triageEnabled = true,
  summaryEnabled = true,
}: Props) => {
  const queryClient = useQueryClient();
  const [selectedRunId, setSelectedRunId] = useState<number | null>(null);

  const query = useQuery({
    queryKey: ["target", targetId, "agent-runs"],
    queryFn: () => getTargetAgentRuns(targetId, 30),
    enabled: Boolean(targetId) && agentRunsEnabled,
    staleTime: 15_000,
  });

  const runs = query.data?.data || [];
  const selectedRun =
    runs.find((item) => item.id === selectedRunId) ||
    runs.find((item) => item.agent_type === "summary") ||
    runs[0] ||
    null;

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["target", targetId, "agent-runs"] });
  };

  const triageMutation = useMutation({
    mutationFn: () => runTargetTriageAgent(targetId),
    onSuccess: (row) => {
      setSelectedRunId(row.id);
      refresh();
    },
  });

  const summaryMutation = useMutation({
    mutationFn: () => runTargetSummaryAgent(targetId),
    onSuccess: (row) => {
      setSelectedRunId(row.id);
      refresh();
    },
  });

  if (!agentRunsEnabled) {
    return (
      <div className="border border-hack-border bg-black/30 p-5">
        <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-dim">
          <Bot className="h-5 w-5" /> Agent Runs Disabled
        </h2>
        <p className="mt-2 text-sm text-hack-dim">
          Agent workflows are disabled for this account.
        </p>
      </div>
    );
  }

  const running = triageMutation.isPending || summaryMutation.isPending;

  return (
    <div className="border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
            <Bot className="h-5 w-5" /> Advisory Agents
          </h2>
          <p className="mt-1 max-w-4xl text-sm text-hack-dim">
            Policy-aware advisory agents summarize attack surface and triage leads.
            Deterministic findings, severity, risk score, and policy enforcement remain authoritative.
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => summaryMutation.mutate()}
            disabled={!summaryEnabled || running}
            className="hack-btn border border-hack-primary px-3 py-1 text-[10px] uppercase tracking-wider disabled:opacity-50"
            title={!summaryEnabled ? "Summary agent is disabled" : "Run Summary Agent"}
          >
            {summaryMutation.isPending ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <Sparkles className="h-3 w-3" />
            )}
            Run Summary
          </button>

          <button
            type="button"
            onClick={() => triageMutation.mutate()}
            disabled={!triageEnabled || running}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white disabled:opacity-50"
            title={!triageEnabled ? "Triage agent is disabled" : "Run Triage Agent"}
          >
            {triageMutation.isPending ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <Brain className="h-3 w-3" />
            )}
            Run Triage
          </button>

          <button
            type="button"
            onClick={() => refresh()}
            disabled={query.isFetching}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white disabled:opacity-50"
          >
            <RefreshCw className={clsx("h-3 w-3", query.isFetching && "animate-spin")} />
            Refresh
          </button>
        </div>
      </div>

      {(triageMutation.error || summaryMutation.error) && (
        <div className="mb-4 border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger">
          {(triageMutation.error as any)?.response?.data?.message ||
            (summaryMutation.error as any)?.response?.data?.message ||
            (triageMutation.error as any)?.message ||
            (summaryMutation.error as any)?.message ||
            "Failed to run agent"}
        </div>
      )}

      <div className="mb-4 flex flex-wrap gap-2">
        <StatusPill tone="primary">advisory only</StatusPill>
        <StatusPill>no auto exploitation</StatusPill>
        <StatusPill>human validation required</StatusPill>
        <StatusPill>policy aware</StatusPill>
      </div>

      {query.isLoading ? (
        <div className="flex items-center gap-2 text-sm text-hack-dim">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading agent runs...
        </div>
      ) : runs.length === 0 ? (
        <div className="border border-hack-border bg-black/20 p-5 text-sm text-hack-dim">
          No agent runs yet. Run Summary or Triage to generate advisory output.
        </div>
      ) : (
        <div className="grid gap-4 xl:grid-cols-[360px_1fr]">
          <div className="space-y-2">
            {runs.map((run) => (
              <button
                key={run.id}
                type="button"
                onClick={() => setSelectedRunId(run.id)}
                className={clsx(
                  "w-full border p-3 text-left transition-colors",
                  selectedRun?.id === run.id
                    ? "border-hack-primary bg-hack-primary/10"
                    : "border-hack-border bg-black/20 hover:border-hack-primary/60",
                )}
              >
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  {run.agent_type === "summary" ? (
                    <ClipboardList className="h-4 w-4 text-hack-primary" />
                  ) : run.agent_type === "triage" ? (
                    <Brain className="h-4 w-4 text-hack-primary" />
                  ) : (
                    <Bot className="h-4 w-4 text-hack-primary" />
                  )}
                  <span className="font-mono text-sm uppercase tracking-wider text-white">
                    {run.agent_type}
                  </span>
                  <StatusPill tone={toneForStatus(run.status)}>
                    {run.status}
                  </StatusPill>
                </div>

                <div className="flex flex-wrap gap-2">
                  <StatusPill>{run.model || "model"}</StatusPill>
                  <StatusPill tone={toneForPolicy(run.policy_status)}>
                    <ShieldCheck className="mr-1 inline h-3 w-3" />
                    {run.policy_status || "unknown"}
                  </StatusPill>
                </div>

                <div className="mt-2 font-mono text-[10px] text-hack-dim">
                  {formatDate(run.created_at)}
                </div>
              </button>
            ))}
          </div>

          <div className="min-w-0">
            {selectedRun ? <AgentRunDetails run={selectedRun} /> : null}
          </div>
        </div>
      )}
    </div>
  );
};

export default AgentRunsPanel;
