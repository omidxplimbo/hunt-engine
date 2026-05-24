import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bug,
  CheckCircle2,
  Download,
  Filter,
  Loader2,
  ShieldAlert,
  TrendingUp,
  X,
} from "lucide-react";
import clsx from "clsx";
import {
  exportTargetFindings,
  getTargetFindingStats,
  getTargetFindings,
  updateFindingStatus,
  type FindingExportFormat,
} from "../api/findings";
import type {
  EvidenceJSON,
  Finding,
  FindingSeverity,
  FindingStats,
  FindingStatus,
} from "../types/finding";

interface Props {
  targetId: number;
}

type TriageModalState = {
  finding: Finding;
  nextStatus: FindingStatus;
  note: string;
};

const severityOrder: FindingSeverity[] = [
  "critical",
  "high",
  "medium",
  "low",
  "info",
];
const statusOptions: Array<FindingStatus | "all"> = [
  "all",
  "open",
  "accepted",
  "false_positive",
  "fixed",
];
const severityOptions: Array<FindingSeverity | "all"> = [
  "all",
  ...severityOrder,
];

const severityClass: Record<FindingSeverity, string> = {
  critical: "border-red-500 text-red-400 bg-red-950/40",
  high: "border-orange-500 text-orange-400 bg-orange-950/40",
  medium: "border-yellow-500 text-yellow-300 bg-yellow-950/30",
  low: "border-blue-500 text-blue-300 bg-blue-950/30",
  info: "border-hack-border text-hack-dim bg-black/30",
};

const statusClass: Record<FindingStatus, string> = {
  open: "border-red-500/50 text-red-300 bg-red-950/20",
  accepted: "border-yellow-500/50 text-yellow-300 bg-yellow-950/20",
  false_positive: "border-purple-500/50 text-purple-300 bg-purple-950/20",
  fixed: "border-hack-primary/50 text-hack-primary bg-hack-primary/10",
};

const evidenceLabels: Record<string, string> = {
  asset: "Asset",
  asset_id: "Asset ID",
  body_hash: "Body Hash",
  bytes_read: "Bytes Read",
  canonical_url: "Canonical URL",
  category: "Category",
  cname: "CNAME",
  confidence: "Confidence",
  content_length: "Content Length",
  content_type: "Content Type",
  detector: "Detector",
  endpoints_count: "Endpoints",
  evidence_text: "Evidence Text",
  extension: "Extension",
  final_url: "Final URL",
  host: "Host",
  ip: "IP",
  js_url: "JavaScript URL",
  matched_at: "Matched At",
  matcher_name: "Matcher",
  placement: "Placement",
  profile: "Profile",
  provider: "Provider",
  raw_url: "Raw URL",
  scheme: "Scheme",
  severity: "Severity",
  signal_type: "Signal",
  source: "Source",
  source_map: "Source Map",
  status_code: "Status",
  template_id: "Template ID",
  template_path: "Template Path",
  title_observed: "Observed Title",
  tool: "Tool",
  truncated: "Truncated",
  type: "Type",
  url: "URL",
  url_id: "URL ID",
  web_server: "Web Server",
};

const defaultEvidenceKeys = [
  "scope",
  "detector",
  "confidence",
  "asset",
  "status_code",
  "final_url",
  "canonical_url",
  "source",
];

const evidenceKeysForFinding = (finding: Finding): string[] => {
  const sourceTool = finding.source_tool || "";
  const category = finding.category || "";

  if (sourceTool === "nuclei") {
    return [
      "template_id",
      "matched_at",
      "host",
      "ip",
      "profile",
      "severity",
      "template_path",
      "matcher_name",
      "type",
    ];
  }

  if (sourceTool === "takeover") {
    return [
      "provider",
      "cname",
      "confidence",
      "asset",
      "final_url",
      "status_code",
      "title",
    ];
  }

  if (sourceTool === "js-intel") {
    if (category === "js-source-map") {
      return [
        "js_url",
        "source_map",
        "signal_type",
        "status_code",
        "content_type",
        "bytes_read",
      ];
    }

    if (category === "js-secret") {
      return [
        "js_url",
        "secret_type",
        "signal_type",
        "matches_count",
        "status_code",
        "content_type",
      ];
    }

    return [
      "js_url",
      "signal_type",
      "endpoints_count",
      "status_code",
      "content_type",
      "bytes_read",
    ];
  }

  if (sourceTool === "builtin") {
    if (category === "server-error") {
      return [
        "scope",
        "detector",
        "confidence",
        "asset",
        "status_code",
        "final_url",
        "web_server",
        "title_observed",
      ];
    }

    if (category === "exposed-interface") {
      return [
        "scope",
        "detector",
        "confidence",
        "asset",
        "status_code",
        "final_url",
        "title_observed",
        "web_server",
      ];
    }

    return defaultEvidenceKeys;
  }

  return defaultEvidenceKeys;
};

const label = (value: string) => value.replace(/_/g, " ").toUpperCase();
const countValue = (value?: number) => value ?? 0;

const isRecord = (value: unknown): value is Record<string, unknown> =>
  Boolean(value) && typeof value === "object" && !Array.isArray(value);

const parseEvidenceJSON = (
  value: EvidenceJSON | undefined,
): EvidenceJSON | undefined => {
  if (typeof value !== "string") return value;
  const trimmed = value.trim();
  if (!trimmed) return undefined;

  try {
    return JSON.parse(trimmed) as EvidenceJSON;
  } catch {
    return value;
  }
};

const hasEvidenceValue = (value: unknown): boolean => {
  if (value === null || value === undefined || value === "") return false;
  if (Array.isArray(value)) return value.length > 0;
  return true;
};

const formatEvidenceValue = (value: unknown): string => {
  if (!hasEvidenceValue(value)) return "-";
  if (Array.isArray(value))
    return value.map((entry) => formatEvidenceValue(entry)).join(", ");
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
};

const stringList = (value: unknown): string[] => {
  if (!value) return [];
  if (Array.isArray(value)) return value.map(String).filter(Boolean);
  if (typeof value === "string") {
    return value
      .split(",")
      .map((item) => item.trim())
      .filter(Boolean);
  }
  return [];
};

const jsonText = (value: unknown) => JSON.stringify(value, null, 2);

const copyTextToClipboard = async (value: string): Promise<boolean> => {
  if (!value) return false;

  if (navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch {
      // Fall through to textarea fallback below.
    }
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("readonly", "");
  textarea.style.position = "fixed";
  textarea.style.top = "0";
  textarea.style.left = "0";
  textarea.style.width = "1px";
  textarea.style.height = "1px";
  textarea.style.opacity = "0";
  textarea.style.pointerEvents = "none";

  document.body.appendChild(textarea);

  try {
    textarea.focus();
    textarea.select();
    textarea.setSelectionRange(0, textarea.value.length);
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    document.body.removeChild(textarea);
  }
};

const EvidenceValue = ({ value }: { value: unknown }) => {
  if (!hasEvidenceValue(value)) {
    return <span className="text-hack-dim">-</span>;
  }

  if (Array.isArray(value)) {
    const primitiveValues = value.filter((entry) => !isRecord(entry));

    if (primitiveValues.length === value.length) {
      return (
        <div className="flex flex-wrap gap-1">
          {primitiveValues.map((entry, index) => (
            <span
              key={`${String(entry)}-${index}`}
              className="border border-hack-border bg-black/30 px-2 py-0.5 text-[10px] text-hack-dim"
            >
              {formatEvidenceValue(entry)}
            </span>
          ))}
        </div>
      );
    }

    return (
      <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words text-[11px] text-hack-dim">
        {jsonText(value)}
      </pre>
    );
  }

  if (isRecord(value)) {
    return (
      <pre className="max-h-40 overflow-auto whitespace-pre-wrap break-words text-[11px] text-hack-dim">
        {jsonText(value)}
      </pre>
    );
  }

  const rendered = String(value);
  const isURL = /^https?:\/\//i.test(rendered);

  if (isURL) {
    return (
      <a
        href={rendered}
        target="_blank"
        rel="noreferrer"
        className="break-all text-hack-primary hover:underline"
      >
        {rendered}
      </a>
    );
  }

  return <span className="break-all text-hack-dim">{rendered}</span>;
};

const EvidenceField = ({
  name,
  value,
  wide = false,
}: {
  name: string;
  value: unknown;
  wide?: boolean;
}) => (
  <div
    className={clsx(
      "border border-hack-border bg-black/20 p-2",
      wide && "md:col-span-2",
    )}
  >
    <div className="mb-1 text-[10px] font-bold uppercase tracking-wider text-hack-dim">
      {evidenceLabels[name] || label(name)}
    </div>
    <div className="font-mono text-xs">
      <EvidenceValue value={value} />
    </div>
  </div>
);

const EvidenceCopyButton = ({
  label,
  value,
  copied,
  onCopied,
}: {
  label: string;
  value: string;
  copied: boolean;
  onCopied: () => void;
}) => (
  <button
    type="button"
    disabled={!value}
    onClick={async () => {
      const ok = await copyTextToClipboard(value);
      if (ok) {
        onCopied();
      } else {
        window.prompt("Copy manually:", value);
      }
    }}
    className="border border-hack-border px-2 py-1 text-[10px] font-bold uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-40"
  >
    {copied ? "Copied" : label}
  </button>
);

const downloadBlob = (blob: Blob, filename: string) => {
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);
};

const EvidenceDetails = ({ finding }: { finding: Finding }) => {
  const [copied, setCopied] = useState<"json" | "text" | null>(null);
  const parsed = parseEvidenceJSON(finding.evidence_json);

  if (!parsed && !finding.evidence) return null;

  const rememberCopied = (type: "json" | "text") => {
    setCopied(type);
    window.setTimeout(() => setCopied(null), 1600);
  };

  if (!isRecord(parsed)) {
    const fallbackValue = finding.evidence || formatEvidenceValue(parsed);

    return (
      <div className="border border-hack-border bg-black/20 p-3">
        <div className="mb-2 flex items-center justify-between gap-2">
          <div className="text-[10px] font-bold uppercase tracking-wider text-hack-primary">
            Evidence
          </div>
          <EvidenceCopyButton
            label="Copy"
            value={fallbackValue}
            copied={copied === "text"}
            onCopied={() => rememberCopied("text")}
          />
        </div>
        <pre className="whitespace-pre-wrap break-words text-xs text-hack-dim">
          {fallbackValue}
        </pre>
      </div>
    );
  }

  const primaryKeys = evidenceKeysForFinding(finding).filter((key) =>
    hasEvidenceValue(parsed[key]),
  );
  const chipKeys = ["tags", "matched_by", "query_keys", "redacted_matches"];
  const rawJSON = jsonText(parsed);
  const legacyText = finding.evidence || "";

  const extraKeys = Object.keys(parsed).filter(
    (key) =>
      !primaryKeys.includes(key) &&
      !chipKeys.includes(key) &&
      key !== "endpoints" &&
      key !== "extracted" &&
      key !== "extracts" &&
      key !== "results" &&
      key !== "evidence_text",
  );

  const extracted = parsed.extracted ?? parsed.extracts ?? parsed.results;
  const endpoints = parsed.endpoints;
  const sourceLabel = finding.source_tool
    ? finding.source_tool.toUpperCase()
    : "STRUCTURED";

  return (
    <div className="space-y-3 border border-hack-border bg-black/20 p-3">
      <div className="flex flex-col gap-2 border-b border-hack-border pb-3 md:flex-row md:items-center md:justify-between">
        <div>
          <div className="text-[10px] font-bold uppercase tracking-wider text-hack-primary">
            {sourceLabel} Evidence
          </div>
          <div className="mt-1 text-[11px] text-hack-dim">
            Structured evidence from{" "}
            <span className="font-mono text-white">
              {finding.source_tool || "unknown"}
            </span>
            {finding.category ? <span> / {finding.category}</span> : null}
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <EvidenceCopyButton
            label="Copy JSON"
            value={rawJSON}
            copied={copied === "json"}
            onCopied={() => rememberCopied("json")}
          />
          <EvidenceCopyButton
            label="Copy Text"
            value={legacyText}
            copied={copied === "text"}
            onCopied={() => rememberCopied("text")}
          />
        </div>
      </div>

      {primaryKeys.length > 0 && (
        <div className="grid gap-2 md:grid-cols-2">
          {primaryKeys.map((key) => (
            <EvidenceField
              key={key}
              name={key}
              value={parsed[key]}
              wide={[
                "template_path",
                "matched_at",
                "final_url",
                "canonical_url",
                "js_url",
                "source_map",
                "cname",
              ].includes(key)}
            />
          ))}
        </div>
      )}

      {chipKeys.map((key) => {
        const values = stringList(parsed[key]);
        if (values.length === 0) return null;

        return (
          <div key={key}>
            <div className="mb-1 text-[10px] font-bold uppercase tracking-wider text-hack-dim">
              {evidenceLabels[key] || label(key)}
            </div>
            <div className="flex flex-wrap gap-1">
              {values.map((value) => (
                <span
                  key={value}
                  className="border border-hack-primary/30 bg-hack-primary/5 px-2 py-0.5 font-mono text-[10px] text-hack-primary"
                >
                  {value}
                </span>
              ))}
            </div>
          </div>
        );
      })}

      {Array.isArray(endpoints) && endpoints.length > 0 && (
        <details className="border border-hack-border bg-black/20 p-2" open>
          <summary className="cursor-pointer text-[10px] font-bold uppercase tracking-wider text-hack-primary">
            Endpoints ({endpoints.length})
          </summary>
          <div className="mt-2 space-y-1">
            {endpoints.map((endpoint, index) => (
              <div
                key={`${formatEvidenceValue(endpoint)}-${index}`}
                className="break-all border border-hack-border bg-black/30 px-2 py-1 font-mono text-[11px] text-hack-dim"
              >
                {formatEvidenceValue(endpoint)}
              </div>
            ))}
          </div>
        </details>
      )}

      {extracted !== undefined && (
        <details className="border border-hack-border bg-black/20 p-2">
          <summary className="cursor-pointer text-[10px] font-bold uppercase tracking-wider text-hack-primary">
            Extracted Data
          </summary>
          <pre className="mt-2 max-h-48 overflow-auto whitespace-pre-wrap break-words text-[11px] text-hack-dim">
            {jsonText(extracted)}
          </pre>
        </details>
      )}

      {extraKeys.length > 0 && (
        <details className="border border-hack-border bg-black/20 p-2">
          <summary className="cursor-pointer text-[10px] font-bold uppercase tracking-wider text-hack-primary">
            Additional Evidence Fields ({extraKeys.length})
          </summary>
          <div className="mt-2 grid gap-2 md:grid-cols-2">
            {extraKeys.map((key) => (
              <EvidenceField
                key={key}
                name={key}
                value={parsed[key]}
                wide={typeof parsed[key] === "object"}
              />
            ))}
          </div>
        </details>
      )}

      {legacyText && (
        <details className="border border-hack-border bg-black/20 p-2">
          <summary className="cursor-pointer text-[10px] font-bold uppercase tracking-wider text-hack-primary">
            Legacy Evidence Text
          </summary>
          <pre className="mt-2 whitespace-pre-wrap break-words text-[11px] text-hack-dim">
            {legacyText}
          </pre>
        </details>
      )}

      <details className="border border-hack-border bg-black/20 p-2">
        <summary className="cursor-pointer text-[10px] font-bold uppercase tracking-wider text-hack-primary">
          Raw JSON
        </summary>
        <pre className="mt-2 max-h-72 overflow-auto whitespace-pre-wrap break-words text-[11px] text-hack-dim">
          {rawJSON}
        </pre>
      </details>
    </div>
  );
};

export const FindingsPanel = ({ targetId }: Props) => {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<FindingStatus | "all">("all");
  const [severity, setSeverity] = useState<FindingSeverity | "all">("all");
  const [search, setSearch] = useState("");
  const [triageModal, setTriageModal] = useState<TriageModalState | null>(null);

  const currentFilters = { status, severity, search, limit: 100 };

  const statsQuery = useQuery({
    queryKey: ["target-findings-stats", targetId],
    queryFn: () => getTargetFindingStats(targetId),
    enabled: Boolean(targetId),
  });

  const findingsQuery = useQuery({
    queryKey: ["target-findings", targetId, status, severity, search],
    queryFn: () => getTargetFindings(targetId, currentFilters),
    enabled: Boolean(targetId),
  });

  const updateStatusMutation = useMutation({
    mutationFn: ({
      findingId,
      nextStatus,
      triageNote,
    }: {
      findingId: number;
      nextStatus: FindingStatus;
      triageNote?: string;
    }) => updateFindingStatus(findingId, nextStatus, triageNote || ""),
    onSuccess: () => {
      setTriageModal(null);
      queryClient.invalidateQueries({
        queryKey: ["target-findings", targetId],
      });
      queryClient.invalidateQueries({
        queryKey: ["target-findings-stats", targetId],
      });
    },
  });

  const exportMutation = useMutation({
    mutationFn: async (format: FindingExportFormat) => {
      const blob = await exportTargetFindings(targetId, format, currentFilters);
      return { blob, format };
    },
    onSuccess: ({ blob, format }) => {
      const timestamp = new Date().toISOString().replace(/[:.]/g, "-");
      downloadBlob(blob, `target-${targetId}-findings-${timestamp}.${format}`);
    },
  });

  const findings = findingsQuery.data?.data || [];
  const stats = statsQuery.data as FindingStats | undefined;
  const highPlus =
    countValue(stats?.by_severity?.critical) +
    countValue(stats?.by_severity?.high);
  const summaryCards = [
    {
      label: "Total",
      value: countValue(stats?.total),
      icon: Bug,
      hint: "All findings",
    },
    {
      label: "Open",
      value: countValue(stats?.open),
      icon: ShieldAlert,
      hint: "Needs review",
    },
    {
      label: "High+",
      value: highPlus,
      icon: TrendingUp,
      hint: "Critical + High",
    },
    {
      label: "Fixed",
      value: countValue(stats?.fixed),
      icon: CheckCircle2,
      hint: "No longer observed",
    },
  ];

  const openTriageModal = (finding: Finding, nextStatus: FindingStatus) => {
    setTriageModal({
      finding,
      nextStatus,
      note: finding.triage_note || "",
    });
  };

  const submitTriage = () => {
    if (!triageModal) return;
    updateStatusMutation.mutate({
      findingId: triageModal.finding.id,
      nextStatus: triageModal.nextStatus,
      triageNote: triageModal.note,
    });
  };

  return (
    <div className="space-y-6">
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
        {summaryCards.map((card) => {
          const Icon = card.icon;
          return (
            <div
              key={card.label}
              className="border border-hack-border bg-hack-panel/70 p-4"
            >
              <div className="flex items-center justify-between gap-3">
                <div>
                  <div className="text-[10px] font-bold uppercase tracking-[0.2em] text-hack-dim">
                    {card.label}
                  </div>
                  <div className="mt-2 text-2xl font-bold text-white">
                    {statsQuery.isLoading ? "..." : card.value}
                  </div>
                  <div className="mt-1 text-[10px] uppercase tracking-wider text-hack-dim">
                    {card.hint}
                  </div>
                </div>
                <Icon className="h-7 w-7 text-hack-primary/70" />
              </div>
            </div>
          );
        })}
      </div>

      <div className="border border-hack-border bg-hack-panel/70 p-4">
        <div className="mb-4 flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
          <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-white">
            <Filter className="h-4 w-4 text-hack-primary" />
            Findings Filters
          </div>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => exportMutation.mutate("csv")}
              disabled={exportMutation.isPending}
              className="inline-flex items-center gap-2 border border-hack-primary/40 px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-hack-primary hover:bg-hack-primary/10 disabled:opacity-50"
            >
              <Download className="h-3 w-3" /> Export CSV
            </button>
            <button
              type="button"
              onClick={() => exportMutation.mutate("json")}
              disabled={exportMutation.isPending}
              className="inline-flex items-center gap-2 border border-hack-border px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-50"
            >
              <Download className="h-3 w-3" /> Export JSON
            </button>
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-[1fr_auto_auto]">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search findings..."
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          />
          <select
            value={severity}
            onChange={(event) =>
              setSeverity(event.target.value as FindingSeverity | "all")
            }
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          >
            {severityOptions.map((option) => (
              <option key={option} value={option}>
                {label(option)}
              </option>
            ))}
          </select>
          <select
            value={status}
            onChange={(event) =>
              setStatus(event.target.value as FindingStatus | "all")
            }
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          >
            {statusOptions.map((option) => (
              <option key={option} value={option}>
                {label(option)}
              </option>
            ))}
          </select>
        </div>
      </div>

      {findingsQuery.isLoading && (
        <div className="flex items-center gap-2 border border-hack-border bg-hack-panel/60 p-4 text-sm text-hack-dim">
          <Loader2 className="h-4 w-4 animate-spin text-hack-primary" />
          Loading findings...
        </div>
      )}

      {!findingsQuery.isLoading && findings.length === 0 && (
        <div className="border border-hack-border bg-hack-panel/60 p-6 text-sm text-hack-dim">
          No findings match the current filters. Detection modules will keep
          populating this tab.
        </div>
      )}

      <div className="space-y-4">
        {findings.map((finding: Finding) => (
          <article
            key={finding.id}
            className="border border-hack-border bg-hack-panel/70 p-4"
          >
            <div className="mb-3 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
              <div className="space-y-2">
                <div className="flex flex-wrap gap-2">
                  <span
                    className={clsx(
                      "border px-2 py-1 text-[10px] font-bold uppercase tracking-wider",
                      severityClass[finding.severity],
                    )}
                  >
                    {finding.severity}
                  </span>
                  <span
                    className={clsx(
                      "border px-2 py-1 text-[10px] font-bold uppercase tracking-wider",
                      statusClass[finding.status],
                    )}
                  >
                    {label(finding.status)}
                  </span>
                  {finding.source_tool && (
                    <span className="border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim">
                      {finding.source_tool}
                    </span>
                  )}
                  {finding.category && (
                    <span className="border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim">
                      {finding.category}
                    </span>
                  )}
                </div>
                <h3 className="text-lg font-bold text-white">
                  {finding.title}
                </h3>
              </div>
              <div className="text-right font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                #{finding.id}
              </div>
            </div>

            {finding.description && (
              <p className="mb-4 text-sm text-hack-dim">
                {finding.description}
              </p>
            )}

            <div className="mb-4">
              <EvidenceDetails finding={finding} />
            </div>

            {finding.recommendation && (
              <div className="mb-4 border border-hack-primary/30 bg-hack-primary/5 p-3 text-sm text-hack-primary">
                <span className="font-bold uppercase tracking-wider">
                  Recommendation:
                </span>{" "}
                {finding.recommendation}
              </div>
            )}

            {finding.triage_note && (
              <div className="mb-4 border border-hack-border bg-black/20 p-3 text-sm text-hack-dim">
                <span className="font-bold text-white">Triage Note:</span>{" "}
                {finding.triage_note}
                {finding.triaged_at && (
                  <span>
                    {" "}
                    ({new Date(finding.triaged_at).toLocaleString()})
                  </span>
                )}
              </div>
            )}

            <div className="flex flex-col gap-3 border-t border-hack-border pt-4 lg:flex-row lg:items-center lg:justify-between">
              <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                First seen:{" "}
                {finding.first_seen
                  ? new Date(finding.first_seen).toLocaleString()
                  : "-"}
                <span className="mx-2 text-hack-border">|</span>
                Last seen:{" "}
                {finding.last_seen
                  ? new Date(finding.last_seen).toLocaleString()
                  : "-"}
              </div>
              <div className="flex flex-wrap gap-2">
                {(
                  [
                    "open",
                    "accepted",
                    "false_positive",
                    "fixed",
                  ] as FindingStatus[]
                ).map((nextStatus) => (
                  <button
                    key={nextStatus}
                    type="button"
                    disabled={
                      updateStatusMutation.isPending ||
                      finding.status === nextStatus
                    }
                    onClick={() => openTriageModal(finding, nextStatus)}
                    className="border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-40"
                  >
                    {nextStatus === "fixed" && (
                      <CheckCircle2 className="mr-1 inline h-3 w-3" />
                    )}
                    {label(nextStatus)}
                  </button>
                ))}
              </div>
            </div>
          </article>
        ))}
      </div>

      {triageModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/80 p-4">
          <div className="w-full max-w-xl border border-hack-border bg-hack-bg p-5 shadow-2xl shadow-hack-primary/10">
            <div className="mb-4 flex items-start justify-between gap-4">
              <div>
                <h3 className="text-lg font-bold text-white">Triage Finding</h3>
                <p className="mt-1 text-sm text-hack-dim">
                  Change status to {label(triageModal.nextStatus)}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setTriageModal(null)}
                className="text-hack-dim hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <div className="mb-4 border border-hack-border bg-black/30 p-3">
              <div className="mb-1 text-[10px] uppercase tracking-wider text-hack-dim">
                Finding
              </div>
              <div className="text-sm font-bold text-white">
                {triageModal.finding.title}
              </div>
            </div>

            <label className="mb-4 block text-xs uppercase tracking-wider text-hack-dim">
              Optional triage note
              <textarea
                value={triageModal.note}
                onChange={(event) =>
                  setTriageModal({ ...triageModal, note: event.target.value })
                }
                rows={5}
                placeholder="Why is this accepted, false positive, fixed, or reopened?"
                className="mt-2 w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>

            <div className="flex justify-end gap-3 border-t border-hack-border pt-4">
              <button
                type="button"
                onClick={() => setTriageModal(null)}
                className="px-4 py-2 text-xs font-bold uppercase tracking-wider text-hack-dim hover:text-white"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={submitTriage}
                disabled={updateStatusMutation.isPending}
                className="border border-hack-primary bg-hack-primary/10 px-4 py-2 text-xs font-bold uppercase tracking-wider text-hack-primary hover:bg-hack-primary/20 disabled:opacity-50"
              >
                {updateStatusMutation.isPending ? "Saving..." : "Save Status"}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default FindingsPanel;
