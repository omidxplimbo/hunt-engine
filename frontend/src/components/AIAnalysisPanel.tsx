import { type ReactNode, useMemo } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Activity,
  AlertTriangle,
  Brain,
  CheckCircle2,
  ClipboardList,
  Loader2,
  RefreshCw,
  ShieldAlert,
  Sparkles,
} from "lucide-react";
import clsx from "clsx";
import {
  generateTargetAIAnalysis,
  getTargetAIAnalyses,
  type TargetAIAnalysis,
} from "../api/targets";

type Props = {
  targetId: number;
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

const asBool = (value: any) => value === true || String(value) === "true";

const cleanText = (value: any) => {
  const text = String(value ?? "").trim();
  return text === "<nil>" ? "" : text;
};

const riskClass = (level: string) => {
  switch (String(level || "").toLowerCase()) {
    case "critical":
      return "border-red-500 text-red-400 bg-red-950/30";
    case "high":
      return "border-orange-500 text-orange-300 bg-orange-950/30";
    case "medium":
      return "border-yellow-500 text-yellow-300 bg-yellow-950/30";
    case "low":
      return "border-hack-primary text-hack-primary bg-hack-primary/10";
    default:
      return "border-hack-border text-hack-dim bg-black/30";
  }
};

const formatDate = (value?: string) => {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
};

const MetricCard = ({
  label,
  value,
  hint,
}: {
  label: string;
  value: string | number;
  hint?: string;
}) => (
  <div className="border border-hack-border bg-black/20 p-3">
    <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
      {label}
    </div>
    <div className="mt-1 font-mono text-xl font-bold text-white">{value}</div>
    {hint && <div className="mt-1 text-xs text-hack-dim">{hint}</div>}
  </div>
);

const StatusPill = ({
  children,
  tone = "neutral",
}: {
  children: ReactNode;
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

const NarrativeList = ({
  title,
  items,
  icon,
}: {
  title: string;
  items: any[];
  icon: "check" | "dot";
}) => {
  if (items.length === 0) return null;

  return (
    <div className="border border-hack-border bg-black/20 p-4">
      <div className="mb-3 font-mono text-sm uppercase tracking-wider text-hack-primary">
        {title}
      </div>
      <ul className="space-y-2 text-sm text-hack-dim">
        {items.map((item, idx) => (
          <li key={idx} className="flex gap-2">
            {icon === "check" ? (
              <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-hack-primary" />
            ) : (
              <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-hack-primary" />
            )}
            <span>{String(item)}</span>
          </li>
        ))}
      </ul>
    </div>
  );
};

const AnalysisDetails = ({ analysis }: { analysis: TargetAIAnalysis }) => {
  const output = useMemo(
    () => parseJSONValue(analysis.output_json),
    [analysis.output_json],
  );

  const riskScore = output?.risk_score ?? output?.risk?.score ?? 0;
  const riskLevel = String(
    output?.risk_level || output?.risk?.level || "informational",
  );
  const confidenceScore =
    output?.confidence_score ?? output?.risk?.confidence_score ?? 0;
  const coverageScore =
    output?.coverage_score ?? output?.risk?.coverage_score ?? 0;
  const exposureScore =
    output?.exposure_score ?? output?.risk?.exposure_score ?? 0;

  const exposureBuckets = asArray(output?.exposure_buckets);
  const coverageGaps = asArray(output?.coverage_gaps);
  const nextActions = asArray(output?.next_actions);
  const topFindings = asArray(output?.top_findings);
  const remediationPlan = asArray(output?.remediation_plan);
  const reportNotes = asArray(output?.report_notes);
  const validationNotes = asArray(output?.validation_notes);

  const counts = output?.counts || {};
  const assetCounts = counts?.assets || {};
  const urlCounts = counts?.urls || {};
  const findingCounts = counts?.findings || {};

  const llmAssisted = asBool(output?.llm_assisted);
  const llmFallback = asBool(output?.llm_fallback);
  const llmError = cleanText(output?.llm_error);
  const llmProvider = cleanText(output?.llm_provider || analysis.provider);
  const llmModel = cleanText(output?.llm_model || analysis.model);
  const methodologyVersion = cleanText(
    output?.methodology_version || output?.methodology?.version,
  );
  const executiveSummary = cleanText(
    output?.executive_summary || analysis.summary || output?.summary,
  );
  const customerSummary = cleanText(output?.customer_summary);

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-4">
        <MetricCard
          label="Risk Score"
          value={`${riskScore}/100`}
          hint={riskLevel.toUpperCase()}
        />
        <MetricCard
          label="Confidence"
          value={`${confidenceScore}/100`}
          hint="evidence confidence"
        />
        <MetricCard
          label="Coverage"
          value={`${coverageScore}/100`}
          hint="scan completeness"
        />
        <MetricCard
          label="Exposure"
          value={`${exposureScore}/100`}
          hint="technical attack surface"
        />
      </div>

      <div className="grid gap-3 md:grid-cols-4">
        <MetricCard
          label="Assets"
          value={assetCounts?.total ?? 0}
          hint={`${assetCounts?.live ?? 0} live`}
        />
        <MetricCard
          label="URLs"
          value={urlCounts?.total ?? 0}
          hint={`${urlCounts?.js ?? 0} JS`}
        />
        <MetricCard
          label="Active Findings"
          value={findingCounts?.active_total ?? 0}
          hint="excluding fixed/false-positive"
        />
        <MetricCard
          label="LLM Mode"
          value={llmAssisted ? "ON" : "OFF"}
          hint={
            llmFallback
              ? "safe deterministic fallback"
              : llmProvider
                ? `${llmProvider}/${llmModel}`
                : "local deterministic"
          }
        />
      </div>

      <div className="flex flex-wrap gap-2">
        <StatusPill tone="primary">deterministic scoring</StatusPill>
        {llmAssisted ? (
          <StatusPill tone="primary">llm narrative</StatusPill>
        ) : llmFallback ? (
          <StatusPill tone="warning">llm fallback</StatusPill>
        ) : (
          <StatusPill>local narrative</StatusPill>
        )}
        {methodologyVersion && <StatusPill>{methodologyVersion}</StatusPill>}
      </div>

      {llmFallback && (
        <div className="border border-hack-warning/60 bg-hack-warning/10 p-3 text-sm text-hack-warning">
          LLM-assisted generation was requested, but Hunt Engine safely fell
          back to deterministic analysis. Reason:{" "}
          {llmError || "provider unavailable"}
        </div>
      )}

      <div className="border border-hack-border bg-black/20 p-4">
        <div className="mb-2 flex items-center gap-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
          <Brain className="h-4 w-4" /> Executive Summary
        </div>
        <p className="text-sm leading-6 text-hack-dim">
          {executiveSummary || "-"}
        </p>

        {customerSummary && customerSummary !== executiveSummary && (
          <>
            <div className="my-3 border-t border-hack-border" />
            <div className="mb-2 font-mono text-xs uppercase tracking-wider text-hack-dim">
              Customer Summary
            </div>
            <p className="text-sm leading-6 text-hack-dim">{customerSummary}</p>
          </>
        )}
      </div>

      <div className="grid gap-4 xl:grid-cols-2">
        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-3 flex items-center gap-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
            <ClipboardList className="h-4 w-4" /> Next Actions
          </div>
          {nextActions.length === 0 ? (
            <div className="text-sm text-hack-dim">No actions generated.</div>
          ) : (
            <ul className="space-y-2 text-sm text-hack-dim">
              {nextActions.map((item, idx) => (
                <li key={idx} className="flex gap-2">
                  <CheckCircle2 className="mt-0.5 h-4 w-4 shrink-0 text-hack-primary" />
                  <span>{String(item)}</span>
                </li>
              ))}
            </ul>
          )}
        </div>

        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-3 flex items-center gap-2 font-mono text-sm uppercase tracking-wider text-hack-warning">
            <AlertTriangle className="h-4 w-4" /> Coverage Gaps
          </div>
          {coverageGaps.length === 0 ? (
            <div className="text-sm text-hack-dim">
              No coverage gaps detected.
            </div>
          ) : (
            <ul className="space-y-2 text-sm text-hack-dim">
              {coverageGaps.map((item, idx) => (
                <li key={idx} className="flex gap-2">
                  <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-hack-warning" />
                  <span>{String(item)}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      </div>

      {(remediationPlan.length > 0 ||
        reportNotes.length > 0 ||
        validationNotes.length > 0) && (
        <div className="grid gap-4 xl:grid-cols-3">
          <NarrativeList
            title="LLM Remediation Plan"
            items={remediationPlan}
            icon="check"
          />
          <NarrativeList title="Report Notes" items={reportNotes} icon="dot" />
          <NarrativeList
            title="Validation Notes"
            items={validationNotes}
            icon="dot"
          />
        </div>
      )}

      <div className="border border-hack-border bg-black/20 p-4">
        <div className="mb-3 flex items-center gap-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
          <ShieldAlert className="h-4 w-4" /> Exposure Buckets
        </div>
        {exposureBuckets.length === 0 ? (
          <div className="text-sm text-hack-dim">
            No exposure buckets generated.
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2">
            {exposureBuckets.map((bucket: any, idx) => (
              <div
                key={idx}
                className="border border-hack-border bg-black/30 p-3"
              >
                <div className="flex items-center justify-between gap-3">
                  <div className="font-mono text-sm font-bold text-white">
                    {bucket?.name || "Bucket"}
                  </div>
                  <span
                    className={clsx(
                      "border px-2 py-0.5 text-[10px] uppercase tracking-wider",
                      riskClass(bucket?.severity),
                    )}
                  >
                    {bucket?.severity || "info"}
                  </span>
                </div>
                <div className="mt-1 font-mono text-xs text-hack-primary">
                  count: {bucket?.count ?? 0}
                </div>
                <div className="mt-2 text-xs leading-5 text-hack-dim">
                  {bucket?.description || "-"}
                </div>
              </div>
            ))}
          </div>
        )}
      </div>

      <div className="border border-hack-border bg-black/20 p-4">
        <div className="mb-3 flex items-center gap-2 font-mono text-sm uppercase tracking-wider text-hack-primary">
          <Activity className="h-4 w-4" /> Top Findings
        </div>
        {topFindings.length === 0 ? (
          <div className="text-sm text-hack-dim">No active top findings.</div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-left font-mono text-xs">
              <thead>
                <tr className="border-b border-hack-border text-hack-dim uppercase tracking-wider">
                  <th className="py-2 pr-3">Severity</th>
                  <th className="py-2 pr-3">Title</th>
                  <th className="py-2 pr-3">Source</th>
                  <th className="py-2 pr-3">Category</th>
                </tr>
              </thead>
              <tbody>
                {topFindings.map((finding: any, idx) => (
                  <tr
                    key={finding?.id || idx}
                    className="border-b border-hack-border/50"
                  >
                    <td className="py-2 pr-3 uppercase text-hack-warning">
                      {finding?.severity || "-"}
                    </td>
                    <td className="py-2 pr-3 text-white">
                      {finding?.title || "-"}
                    </td>
                    <td className="py-2 pr-3 text-hack-dim">
                      {finding?.source_tool || "-"}
                    </td>
                    <td className="py-2 pr-3 text-hack-dim">
                      {finding?.category || "-"}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      <details className="border border-hack-border bg-black/20 p-4">
        <summary className="cursor-pointer font-mono text-xs uppercase tracking-wider text-hack-dim hover:text-white">
          Raw Analysis Output JSON
        </summary>
        <pre className="mt-3 max-h-96 overflow-auto whitespace-pre-wrap break-words bg-black/40 p-3 text-xs text-hack-dim">
          {JSON.stringify(output, null, 2)}
        </pre>
      </details>
    </div>
  );
};

const AIAnalysisPanel = ({ targetId }: Props) => {
  const queryClient = useQueryClient();

  const query = useQuery({
    queryKey: ["target-ai-analyses", targetId],
    queryFn: () => getTargetAIAnalyses(targetId, 5),
    enabled: Boolean(targetId),
  });

  const generateMutation = useMutation({
    mutationFn: (useLLM: boolean) => generateTargetAIAnalysis(targetId, useLLM),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["target-ai-analyses", targetId],
      });
    },
  });

  const analyses = query.data?.data || [];
  const latest = analyses[0];
  const latestOutput = latest ? parseJSONValue(latest.output_json) : {};
  const latestLLMAssisted = asBool(latestOutput?.llm_assisted);
  const latestLLMFallback = asBool(latestOutput?.llm_fallback);

  return (
    <div className="space-y-4">
      <div className="border border-hack-border bg-black/30 p-4">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-center xl:justify-between">
          <div>
            <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
              <Sparkles className="h-5 w-5" /> AI Analysis
            </h2>
            <p className="mt-1 max-w-4xl text-sm text-hack-dim">
              Commercial-grade deterministic target analysis. LLM-assisted mode
              only adds report narrative; scoring, risk level, and finding
              priority remain controlled by deterministic guardrails. Invalid or
              missing provider config falls back safely.
            </p>
          </div>

          <div className="flex flex-col gap-2 sm:flex-row">
            <button
              type="button"
              onClick={() => generateMutation.mutate(false)}
              disabled={generateMutation.isPending}
              className="hack-btn-ghost flex items-center justify-center gap-2 border border-hack-border px-4 py-2 disabled:opacity-50"
            >
              {generateMutation.isPending &&
              generateMutation.variables === false ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <RefreshCw className="h-4 w-4" />
              )}
              Generate Local
            </button>

            <button
              type="button"
              onClick={() => generateMutation.mutate(true)}
              disabled={generateMutation.isPending}
              className="hack-btn flex items-center justify-center gap-2 px-4 py-2 disabled:opacity-50"
            >
              {generateMutation.isPending &&
              generateMutation.variables === true ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Brain className="h-4 w-4" />
              )}
              Generate LLM-Assisted
            </button>
          </div>
        </div>

        {generateMutation.isError && (
          <div className="mt-3 border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger">
            {(generateMutation.error as any)?.response?.data?.message ||
              "Failed to generate analysis"}
          </div>
        )}
      </div>

      {query.isLoading && (
        <div className="border border-hack-border bg-black/20 p-6 text-center font-mono text-hack-dim">
          Loading target analysis...
        </div>
      )}

      {!query.isLoading && !latest && (
        <div className="border border-hack-border bg-black/20 p-6 text-center">
          <div className="font-mono text-hack-dim">
            No target analysis exists for this target yet.
          </div>
          <div className="mt-2 text-sm text-hack-dim">
            Generate a local deterministic analysis first, then optionally run
            LLM-assisted narrative generation after configuring a provider in
            Account.
          </div>
        </div>
      )}

      {latest && (
        <div className="border border-hack-border bg-black/30 p-4">
          <div className="mb-4 flex flex-wrap items-center justify-between gap-3 border-b border-hack-border pb-3">
            <div>
              <div className="font-mono text-sm font-bold text-white">
                {latest.title || `Analysis #${latest.id}`}
              </div>
              <div className="mt-1 text-xs text-hack-dim">
                {latest.provider}/{latest.model} · {latest.prompt_version} ·{" "}
                {formatDate(latest.created_at)}
              </div>
              <div className="mt-2 flex flex-wrap gap-2">
                {latestLLMAssisted ? (
                  <StatusPill tone="primary">llm-assisted</StatusPill>
                ) : latestLLMFallback ? (
                  <StatusPill tone="warning">llm fallback</StatusPill>
                ) : (
                  <StatusPill>local deterministic</StatusPill>
                )}
                <StatusPill tone="primary">guardrails active</StatusPill>
                {latestOutput?.llm_provider && (
                  <StatusPill>
                    {String(latestOutput.llm_provider)}/
                    {String(latestOutput.llm_model || latest.model)}
                  </StatusPill>
                )}
              </div>
            </div>
            <span
              className={clsx(
                "border px-2 py-1 font-mono text-[10px] uppercase tracking-wider",
                riskClass(latestOutput?.risk_level),
              )}
            >
              {latestOutput?.risk_level || latest.status}
            </span>
          </div>

          <AnalysisDetails analysis={latest} />
        </div>
      )}

      {analyses.length > 1 && (
        <div className="border border-hack-border bg-black/20 p-4">
          <div className="mb-2 font-mono text-xs uppercase tracking-wider text-hack-dim">
            Recent Analysis History
          </div>
          <div className="space-y-2">
            {analyses.slice(1).map((item) => {
              const output = parseJSONValue(item.output_json);
              const mode = asBool(output?.llm_assisted)
                ? "LLM"
                : asBool(output?.llm_fallback)
                  ? "Fallback"
                  : "Local";

              return (
                <div
                  key={item.id}
                  className="flex flex-wrap items-center justify-between gap-3 border border-hack-border bg-black/20 p-2 text-xs"
                >
                  <span className="font-mono text-white">
                    #{item.id} · {mode} · {item.title}
                  </span>
                  <span className="text-hack-dim">
                    {formatDate(item.created_at)}
                  </span>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};

export default AIAnalysisPanel;
