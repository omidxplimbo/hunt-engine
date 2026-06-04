import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowLeft,
  ArrowUp,
  Brain,
  CheckCircle,
  Cloud,
  Database,
  Download,
  FileCode,
  FileText,
  Globe,
  Link2,
  Loader2,
  Network,
  Search,
  Shield,
  ShieldCheck,
  Terminal,
  XCircle,
} from "lucide-react";
import clsx from "clsx";
import {
  downloadAssets,
  downloadTargetPDFReport,
  downloadURLs,
  exportTargetIPs,
  getTargetAssets,
  getTargetDetails,
  getTargetURLs,
} from "../api/targets";
import {
  FEATURE_FLAGS,
  getMyFeatureFlags,
  isAccountFeatureEnabled,
} from "../api/me";
import FindingsPanel from "../components/FindingsPanel";
import AIAnalysisPanel from "../components/AIAnalysisPanel";
import AgentRunsPanel from "../components/AgentRunsPanel";
import AgentActionsPanel from "../components/AgentActionsPanel";
import TargetPolicyPanel from "../components/TargetPolicyPanel";

type ActiveTab = "assets" | "urls" | "findings" | "analysis" | "policy";

const KNOWN_ASSET_PROVIDERS = [
  { id: "subfinder", label: "Subfinder" },
  { id: "assetfinder", label: "Assetfinder" },
  { id: "crtsh", label: "crt.sh" },
  { id: "cero", label: "Cero" },
  { id: "alterx", label: "Alterx" },
  { id: "puredns", label: "PureDNS" },
  { id: "abusedb", label: "AbuseDB" },
  { id: "amass", label: "Amass" },
];

const KNOWN_SOURCES = [
  { id: "wayback", label: "Wayback" },
  { id: "gau", label: "GAU" },
  { id: "katana", label: "Katana" },
  { id: "waymore", label: "Waymore" },
  { id: "virustotal", label: "VirusTotal" },
];


const parseJSONList = (value: unknown): string[] => {
  if (Array.isArray(value)) return value.map(String);
  if (typeof value === "string" && value.trim() !== "") {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed.map(String) : [];
    } catch {
      return [];
    }
  }
  return [];
};

const sourceColor = (source: string) => {
  const colors: Record<string, string> = {
    subfinder: "border-blue-400 text-blue-400 bg-blue-900/20",
    assetfinder: "border-green-400 text-green-400 bg-green-900/20",
    cero: "border-purple-400 text-purple-400 bg-purple-900/20",
    crtsh: "border-orange-400 text-orange-400 bg-orange-900/20",
    alterx: "border-yellow-400 text-yellow-400 bg-yellow-900/20",
    abusedb: "border-red-500 text-red-500 bg-red-900/20",
    puredns: "border-cyan-400 text-cyan-300 bg-cyan-950/30",
    amass: "border-hack-primary text-hack-primary bg-hack-primary/10",
  };
  return (
    colors[source.toLowerCase()] ||
    "border-hack-border text-hack-dim bg-black/30"
  );
};

const renderPortsCell = (openPorts: unknown) => {
  let obj: Record<string, number[]> = {};
  try {
    if (typeof openPorts === "string") obj = JSON.parse(openPorts || "{}");
    else if (openPorts && typeof openPorts === "object")
      obj = openPorts as Record<string, number[]>;
  } catch {
    obj = {};
  }

  const union = new Set<number>();
  Object.values(obj || {}).forEach((ports) => {
    (ports || []).forEach((port) => union.add(Number(port)));
  });

  const portsSorted = Array.from(union)
    .filter(Number.isFinite)
    .sort((a, b) => a - b);
  if (portsSorted.length === 0) return <span className="text-hack-dim">-</span>;

  return (
    <div className="flex flex-wrap gap-1">
      {portsSorted.slice(0, 12).map((port) => (
        <span
          key={port}
          className="rounded border border-hack-border bg-black/30 px-1.5 py-0.5 text-[10px] text-hack-dim"
        >
          {port}
        </span>
      ))}
      {portsSorted.length > 12 && (
        <span className="rounded border border-hack-border bg-black/30 px-1.5 py-0.5 text-[10px] text-hack-dim">
          +{portsSorted.length - 12}
        </span>
      )}
    </div>
  );
};

const TargetAssets = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const targetId = Number(id);

  const [activeTab, setActiveTab] = useState<ActiveTab>("assets");
  const [page, setPage] = useState(1);
  const [searchTerm, setSearchTerm] = useState("");
  const [debouncedSearch, setDebouncedSearch] = useState("");
  const [sortBy, setSortBy] = useState("created_at");
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("desc");

  const [filterLive, setFilterLive] = useState<boolean | undefined>(undefined);
  const [filterHttpx, setFilterHttpx] = useState<boolean | undefined>(
    undefined,
  );
  const [filterDnsOnly, setFilterDnsOnly] = useState<boolean | undefined>(
    undefined,
  );
  const [filterHasPorts, setFilterHasPorts] = useState<boolean | undefined>(
    undefined,
  );
  const [filterNoCdn, setFilterNoCdn] = useState<boolean | undefined>(
    undefined,
  );
  const [filterHasCdn, setFilterHasCdn] = useState<boolean | undefined>(
    undefined,
  );
  const [filterHasWaf, setFilterHasWaf] = useState<boolean | undefined>(
    undefined,
  );
  const [filterHasCloud, setFilterHasCloud] = useState<boolean | undefined>(
    undefined,
  );
  const [filterAssetProvider, setFilterAssetProvider] = useState<string | null>(
    null,
  );
  const [filterStatusCode, setFilterStatusCode] = useState("");

  const [filterJsOnly, setFilterJsOnly] = useState(false);
  const [filterSources, setFilterSources] = useState<string[]>([]);
  const [reportDownloading, setReportDownloading] = useState(false);

  useEffect(() => {
    const timer = setTimeout(() => {
      setDebouncedSearch(searchTerm);
      setPage(1);
    }, 500);
    return () => clearTimeout(timer);
  }, [searchTerm]);

  useEffect(() => {
    setPage(1);
  }, [
    activeTab,
    filterLive,
    filterHttpx,
    filterDnsOnly,
    filterHasPorts,
    filterNoCdn,
    filterHasCdn,
    filterHasWaf,
    filterHasCloud,
    filterAssetProvider,
    filterStatusCode,
    filterJsOnly,
    filterSources,
  ]);

  const targetQuery = useQuery({
    queryKey: ["target", targetId],
    queryFn: () => getTargetDetails(targetId),
    enabled: Boolean(targetId),
  });

  const featureFlagsQuery = useQuery({
    queryKey: ["me", "feature-flags"],
    queryFn: getMyFeatureFlags,
    staleTime: 30_000,
  });

  const assetsQuery = useQuery({
    queryKey: [
      "assets",
      targetId,
      page,
      filterLive,
      filterHttpx,
      filterDnsOnly,
      filterHasPorts,
      filterNoCdn,
      filterHasCdn,
      filterHasWaf,
      filterHasCloud,
      filterAssetProvider,
      filterStatusCode,
      debouncedSearch,
      sortBy,
      sortOrder,
    ],
    queryFn: () =>
      getTargetAssets(
        targetId,
        page,
        50,
        {
          is_live: filterLive,
          search: debouncedSearch,
          has_httpx: filterHttpx,
          dns_only: filterDnsOnly,
          has_ports: filterHasPorts,
          no_cdn: filterNoCdn,
          has_cdn: filterHasCdn,
          has_waf: filterHasWaf,
          has_cloud: filterHasCloud,
          status_code: filterStatusCode || undefined,
          sources: filterAssetProvider ? [filterAssetProvider] : undefined,
        },
        sortBy,
        sortOrder,
      ),
    enabled: Boolean(targetId) && activeTab === "assets",
  });

  const urlsQuery = useQuery({
    queryKey: [
      "urls",
      targetId,
      page,
      debouncedSearch,
      filterJsOnly,
      sortBy,
      sortOrder,
      filterSources,
    ],
    queryFn: () =>
      getTargetURLs(
        targetId,
        page,
        50,
        debouncedSearch,
        filterJsOnly,
        sortBy,
        sortOrder,
        filterSources,
      ),
    enabled: Boolean(targetId) && activeTab === "urls",
  });

  const isFetching =
    activeTab === "assets"
      ? assetsQuery.isFetching
      : activeTab === "urls"
        ? urlsQuery.isFetching
        : false;
  const displayedAssets = assetsQuery.data?.data || [];
  const displayedUrls = urlsQuery.data?.data || [];
  const totalRecords =
    activeTab === "assets"
      ? (assetsQuery.data?.total ?? 0)
      : activeTab === "urls"
        ? ((urlsQuery.data as any)?.total ??
          (urlsQuery.data as any)?.total_count ??
          0)
        : 0;

  const accountFeatureFlags = featureFlagsQuery.data?.flags;
  const featureTargetPDFReport = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.targetPDFReport,
    true,
  );

  const featureTargetPolicy = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.targetPolicy,
    true,
  );
  const featureAIAnalysis = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.aiAnalysis,
    true,
  );
  const featureLLMAssistedAnalysis = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.llmAssistedAnalysis,
    true,
  );
  const featureAIRecommendations = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.aiRecommendations,
    true,
  );

  const featureAgentRuns = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.agentRuns,
    true,
  );

  const featureAgentActions = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.agentActions,
    true,
  );

  const featureAITriageAgent = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.aiTriageAgent,
    true,
  );

  const featureAISummaryAgent = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.aiSummaryAgent,
    true,
  );

  const featureAIReportAgent = isAccountFeatureEnabled(
    accountFeatureFlags,
    FEATURE_FLAGS.aiReportAgent,
    true,
  );

  const featureAnalysisTab = featureAIAnalysis || featureAgentRuns;

  useEffect(() => {
    if (activeTab === "analysis" && !featureAnalysisTab) {
      setActiveTab("assets");
    }
  }, [activeTab, featureAnalysisTab]);

  useEffect(() => {
    if (activeTab === "policy" && !featureTargetPolicy) {
      setActiveTab("assets");
    }
  }, [activeTab, featureTargetPolicy]);

  if (!targetId)
    return (
      <div className="p-6 text-hack-danger">FATAL ERROR: Invalid Target ID</div>
    );

  const toggleHttpx = () => {
    if (!filterHttpx) {
      setFilterDnsOnly(undefined);
      setFilterHttpx(true);
    } else {
      setFilterHttpx(undefined);
    }
  };

  const toggleDnsOnly = () => {
    if (!filterDnsOnly) {
      setFilterHttpx(undefined);
      setFilterDnsOnly(true);
    } else {
      setFilterDnsOnly(undefined);
    }
  };

  const toggleSource = (sourceId: string) => {
    setFilterSources((prev) =>
      prev.includes(sourceId)
        ? prev.filter((source) => source !== sourceId)
        : [...prev, sourceId],
    );
  };

  const handleSort = (field: string) => {
    if (sortBy === field)
      setSortOrder((prev) => (prev === "asc" ? "desc" : "asc"));
    else {
      setSortBy(field);
      setSortOrder("asc");
    }
  };

  const SortableHeader = ({
    field,
    label,
    className,
  }: {
    field: string;
    label: string;
    className?: string;
  }) => (
    <th
      onClick={() => handleSort(field)}
      className={clsx(
        "cursor-pointer select-none px-3 py-2 text-left text-xs uppercase tracking-wider text-hack-dim hover:text-white",
        className,
      )}
    >
      {label}{" "}
      {sortBy === field &&
        (sortOrder === "asc" ? (
          <ArrowUp className="inline h-3 w-3" />
        ) : (
          <ArrowDown className="inline h-3 w-3" />
        ))}
    </th>
  );

  const handleDownloadAssets = async () => {
    await downloadAssets(
      targetId,
      targetQuery.data?.root_domain || "target",
      {
        is_live: filterLive,
        search: debouncedSearch,
        has_httpx: filterHttpx,
        dns_only: filterDnsOnly,
        has_ports: filterHasPorts,
        no_cdn: filterNoCdn,
        has_cdn: filterHasCdn,
        has_waf: filterHasWaf,
        has_cloud: filterHasCloud,
        status_code: filterStatusCode || undefined,
        sources: filterAssetProvider ? [filterAssetProvider] : undefined,
      },
      sortBy,
      sortOrder,
    );
  };

  const handleDownloadURLs = async () => {
    await downloadURLs(
      targetId,
      targetQuery.data?.root_domain || "target",
      debouncedSearch,
      filterJsOnly,
      sortBy,
      sortOrder,
      filterSources,
    );
  };

  const handleDownloadReport = async () => {
    if (!targetQuery.data || reportDownloading || !featureTargetPDFReport)
      return;

    try {
      setReportDownloading(true);
      await downloadTargetPDFReport(
        targetId,
        targetQuery.data.root_domain || targetQuery.data.name || "target",
      );
    } finally {
      setReportDownloading(false);
    }
  };

  return (
    <div className="space-y-5 p-4 md:p-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-3">
          <button
            onClick={() => navigate("/targets")}
            className="rounded p-2 text-hack-dim transition-colors hover:bg-white/5 hover:text-white"
          >
            <ArrowLeft size={18} />
          </button>
          <div>
            <h1 className="font-mono text-xl font-bold text-white">
              # {targetQuery.isLoading ? "LOADING..." : targetQuery.data?.name}
            </h1>
            <div className="flex items-center gap-2 text-sm text-hack-dim">
              <Globe className="h-4 w-4" /> {targetQuery.data?.root_domain}
              {isFetching && <Loader2 className="h-3 w-3 animate-spin" />}
            </div>
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {targetQuery.data && (
            <button
              onClick={handleDownloadReport}
              disabled={reportDownloading || !featureTargetPDFReport}
              className="hack-btn-ghost flex items-center gap-2 border border-hack-primary/60 px-3 text-hack-primary disabled:opacity-50"
              title={
                featureTargetPDFReport
                  ? "Download a professional PDF report for this target"
                  : "PDF reports are disabled by feature flag"
              }
            >
              {reportDownloading ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <FileText className="h-4 w-4" />
              )}
              PDF Report
            </button>
          )}

          <button
            onClick={() => setActiveTab("assets")}
            className={clsx(
              "hack-btn flex-1 justify-center md:flex-none",
              activeTab === "assets"
                ? "bg-hack-primary text-black"
                : "bg-transparent text-hack-dim border-hack-dim/30",
            )}
          >
            <Database className="mr-1 h-4 w-4" /> Assets
          </button>
          <button
            onClick={() => setActiveTab("urls")}
            className={clsx(
              "hack-btn flex-1 justify-center md:flex-none",
              activeTab === "urls"
                ? "bg-hack-primary text-black"
                : "bg-transparent text-hack-dim border-hack-dim/30",
            )}
          >
            <Link2 className="mr-1 h-4 w-4" /> Intel / URLs
          </button>
                      <button
              onClick={() => featureTargetPolicy && setActiveTab("policy")}
              disabled={!featureTargetPolicy}
              title={
                featureTargetPolicy
                  ? "Target policy"
                  : "Target policy is disabled by feature flag"
              }
              className={clsx(
                "hack-btn flex-1 justify-center md:flex-none disabled:opacity-40",
                activeTab === "policy"
                  ? "bg-hack-primary text-black"
                  : "bg-transparent text-hack-dim border-hack-dim/30",
              )}
            >
              <ShieldCheck className="mr-1 h-4 w-4" /> Policy
            </button>

<button
            onClick={() => setActiveTab("findings")}
            className={clsx(
              "hack-btn flex-1 justify-center md:flex-none",
              activeTab === "findings"
                ? "bg-hack-primary text-black"
                : "bg-transparent text-hack-dim border-hack-dim/30",
            )}
          >
            <Shield className="mr-1 h-4 w-4" /> Findings
          </button>

          <button
            onClick={() => featureAnalysisTab && setActiveTab("analysis")}
            disabled={!featureAnalysisTab}
            title={
              featureAIAnalysis
                ? "Target analysis"
                : "Analysis is disabled by feature flag"
            }
            className={clsx(
              "hack-btn flex-1 justify-center md:flex-none disabled:opacity-40",
              activeTab === "analysis"
                ? "bg-hack-primary text-black"
                : "bg-transparent text-hack-dim border-hack-dim/30",
            )}
          >
            <Brain className="mr-1 h-4 w-4" /> Analysis
          </button>

          {activeTab === "assets" && targetQuery.data && (
            <>
              <button
                onClick={() =>
                  exportTargetIPs(targetId, targetQuery.data.root_domain)
                }
                className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3"
                title="Download all unique IPs as TXT file"
              >
                <Network className="h-4 w-4" /> Export IPs
              </button>
              <button
                onClick={handleDownloadAssets}
                className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3"
              >
                <Download className="h-4 w-4" /> Export Assets
              </button>
            </>
          )}

          {activeTab === "urls" && (
            <button
              onClick={handleDownloadURLs}
              className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3"
            >
              <Download className="h-4 w-4" /> Export URLs
            </button>
          )}
        </div>
      </div>

      {activeTab !== "findings" && activeTab !== "analysis" && (
        <div className="space-y-3 border border-hack-border bg-black/20 p-3">
          <div className="relative">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-hack-dim" />
            <input
              value={searchTerm}
              onChange={(event) => setSearchTerm(event.target.value)}
              placeholder={
                activeTab === "assets" ? "Search assets..." : "Search URLs..."
              }
              className="w-full border border-hack-border bg-black/30 py-2 pl-9 pr-3 font-mono text-sm text-white outline-none focus:border-hack-primary"
            />
          </div>

          {activeTab === "assets" && (
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <span className="uppercase tracking-wider text-hack-dim">
                Filters:
              </span>
              <button
                onClick={() => setFilterLive(undefined)}
                className={clsx(
                  "hack-btn-ghost border border-transparent px-2",
                  filterLive === undefined && "border-hack-dim/50 text-white",
                )}
              >
                All
              </button>
              <button
                onClick={() => setFilterLive(true)}
                className={clsx(
                  "hack-btn-ghost flex items-center gap-1 px-2",
                  filterLive === true &&
                    "!text-hack-primary border border-hack-primary/50",
                )}
              >
                <CheckCircle className="h-3 w-3" /> Live
              </button>
              <button
                onClick={() => setFilterLive(false)}
                className={clsx(
                  "hack-btn-ghost flex items-center gap-1 px-2",
                  filterLive === false &&
                    "!text-hack-danger border border-hack-danger/50",
                )}
              >
                <XCircle className="h-3 w-3" /> Dead
              </button>
              <button
                onClick={toggleHttpx}
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterHttpx && "border-hack-primary text-hack-primary",
                )}
              >
                Web
              </button>
              <button
                onClick={toggleDnsOnly}
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterDnsOnly && "border-hack-primary text-hack-primary",
                )}
              >
                DNS
              </button>
              <button
                onClick={() =>
                  setFilterHasPorts((prev) => (!prev ? true : undefined))
                }
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterHasPorts && "border-hack-primary text-hack-primary",
                )}
              >
                Ports
              </button>
              <button
                onClick={() =>
                  setFilterNoCdn((prev) => (!prev ? true : undefined))
                }
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterNoCdn && "border-hack-warning text-hack-warning",
                )}
              >
                No CDN
              </button>
              <button
                onClick={() =>
                  setFilterHasCdn((prev) => (!prev ? true : undefined))
                }
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterHasCdn && "border-hack-primary text-hack-primary",
                )}
              >
                CDN
              </button>
              <button
                onClick={() =>
                  setFilterHasWaf((prev) => (!prev ? true : undefined))
                }
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterHasWaf && "border-hack-primary text-hack-primary",
                )}
              >
                WAF
              </button>
              <button
                onClick={() =>
                  setFilterHasCloud((prev) => (!prev ? true : undefined))
                }
                className={clsx(
                  "hack-btn-ghost border px-2",
                  filterHasCloud && "border-hack-primary text-hack-primary",
                )}
              >
                Cloud
              </button>

              <select
                value={filterAssetProvider || ""}
                onChange={(event) =>
                  setFilterAssetProvider(event.target.value || null)
                }
                className="border border-hack-border bg-black/30 px-2 py-1 text-xs text-white outline-none"
              >
                <option value="">ALL PROVIDERS</option>
                {KNOWN_ASSET_PROVIDERS.map((provider) => (
                  <option key={provider.id} value={provider.id}>
                    {provider.label}
                  </option>
                ))}
              </select>

              <select
                value={filterStatusCode}
                onChange={(event) => setFilterStatusCode(event.target.value)}
                className="border border-hack-border bg-black/30 px-2 py-1 text-xs text-white outline-none"
              >
                <option value="">ALL STATUS</option>
                <option value="2xx">2xx</option>
                <option value="3xx">3xx</option>
                <option value="4xx">4xx</option>
                <option value="5xx">5xx</option>
                {[200, 301, 302, 401, 403, 404, 429, 500, 502, 503].map(
                  (code) => (
                    <option key={code} value={String(code)}>
                      {code}
                    </option>
                  ),
                )}
              </select>
            </div>
          )}

          {activeTab === "urls" && (
            <div className="flex flex-wrap items-center gap-2 text-xs">
              <button
                onClick={() => setFilterJsOnly(!filterJsOnly)}
                className={clsx(
                  "hack-btn-ghost flex items-center gap-1 border px-3",
                  filterJsOnly
                    ? "border-hack-warning text-hack-warning bg-hack-warning/5"
                    : "border-hack-border",
                )}
              >
                <FileCode className="h-3 w-3" /> JS Files
              </button>
              {KNOWN_SOURCES.map((source) => (
                <button
                  key={source.id}
                  onClick={() => toggleSource(source.id)}
                  className={clsx(
                    "hack-btn-ghost border px-2 py-1 text-[10px] uppercase tracking-wider",
                    filterSources.includes(source.id)
                      ? "border-hack-primary text-hack-primary bg-hack-primary/10"
                      : "border-hack-border text-hack-dim hover:text-white",
                  )}
                >
                  {source.label}
                </button>
              ))}
            </div>
          )}
        </div>
      )}

      {activeTab === "findings" ? (
        <FindingsPanel targetId={targetId} />
      ) : activeTab === "policy" ? (
        <TargetPolicyPanel targetId={targetId} enabled={featureTargetPolicy} />
      ) : activeTab === "analysis" ? (
        <>
          <AIAnalysisPanel
            targetId={targetId}
            aiAnalysisEnabled={featureAIAnalysis}
            llmAssistedEnabled={featureLLMAssistedAnalysis}
            recommendationsEnabled={featureAIRecommendations}
          />

          <AgentRunsPanel
            targetId={targetId}
            agentRunsEnabled={featureAgentRuns}
            triageEnabled={featureAITriageAgent}
            summaryEnabled={featureAISummaryAgent}
            reportEnabled={featureAIReportAgent}
          />

          <AgentActionsPanel
            targetId={targetId}
            enabled={featureAgentActions}
          />
        </>
      ) : (
        <div className="overflow-hidden border border-hack-border bg-black/20">
          <div className="border-b border-hack-border px-3 py-2 font-mono text-xs uppercase tracking-wider text-hack-dim">
            DATA_GRID_V1
          </div>

          <div className="overflow-x-auto">
            {activeTab === "assets" ? (
              <table className="min-w-full border-collapse text-sm">
                <thead className="border-b border-hack-border bg-black/30">
                  <tr>
                    <SortableHeader field="value" label="Providers / DNS" />
                    <th className="px-3 py-2 text-left text-xs uppercase tracking-wider text-hack-dim">
                      DNS IP
                    </th>
                    <th className="px-3 py-2 text-left text-xs uppercase tracking-wider text-hack-dim">
                      HTTP IP
                    </th>
                    <th className="px-3 py-2 text-left text-xs uppercase tracking-wider text-hack-dim">
                      Ports
                    </th>
                    <th className="px-3 py-2 text-left text-xs uppercase tracking-wider text-hack-dim">
                      CDN
                    </th>
                    <SortableHeader field="status_code" label="Status" />
                    <th className="px-3 py-2 text-left text-xs uppercase tracking-wider text-hack-dim">
                      Stack
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {displayedAssets.map((asset: any) => {
                    const sources = parseJSONList(asset.sources);
                    const dnsxIps = parseJSONList(asset.dnsx_ip);
                    const httpxIps = parseJSONList(asset.host_ip);
                    const techs = parseJSONList(asset.technologies);
                    const statusCode = asset.status_code || 0;

                    return (
                      <tr
                        key={asset.id || asset.value}
                        className="border-b border-hack-border/50 hover:bg-white/[0.02]"
                      >
                        <td className="max-w-md px-3 py-3 align-top">
                          <a
                            href={asset.final_url || `http://${asset.value}`}
                            target="_blank"
                            rel="noreferrer"
                            className="font-mono text-hack-primary hover:underline"
                          >
                            {asset.value}
                          </a>
                          <div className="mt-2 flex flex-wrap gap-1">
                            {sources.length > 0 ? (
                              sources.map((source, index) => (
                                <span
                                  key={`${source}-${index}`}
                                  className={clsx(
                                    "rounded border px-1.5 py-0.5 text-[10px] uppercase tracking-wider",
                                    sourceColor(source),
                                  )}
                                >
                                  {source}
                                </span>
                              ))
                            ) : (
                              <span className="text-hack-dim">-</span>
                            )}
                          </div>
                        </td>
                        <td className="px-3 py-3 align-top text-xs text-hack-dim">
                          {dnsxIps.length > 0 ? dnsxIps.join(", ") : "-"}
                        </td>
                        <td className="px-3 py-3 align-top text-xs text-hack-dim">
                          {httpxIps.length > 0 ? httpxIps.join(", ") : "-"}
                        </td>
                        <td className="px-3 py-3 align-top">
                          {renderPortsCell(asset.open_ports)}
                        </td>
                        <td className="px-3 py-3 align-top text-xs text-hack-dim">
                          {asset.cdn_name ||
                            asset.cdncheck_name ||
                            asset.wafcheck_name ||
                            asset.cloudcheck_name ||
                            "-"}
                          {(asset.cdncheck ||
                            asset.wafcheck ||
                            asset.cloudcheck) && (
                            <Cloud className="ml-1 inline h-3 w-3 text-hack-primary" />
                          )}
                        </td>
                        <td className="px-3 py-3 align-top">
                          {asset.is_live ? (
                            statusCode > 0 ? (
                              <span
                                className={clsx(
                                  "rounded border px-2 py-1 text-xs",
                                  statusCode >= 200 && statusCode < 300
                                    ? "border-hack-primary text-hack-primary"
                                    : statusCode >= 300 && statusCode < 400
                                      ? "border-hack-warning text-hack-warning"
                                      : "border-hack-danger text-hack-danger",
                                )}
                              >
                                {statusCode}
                              </span>
                            ) : (
                              <span className="text-hack-dim">-</span>
                            )
                          ) : (
                            <span className="text-hack-danger">DEAD</span>
                          )}
                        </td>
                        <td className="max-w-sm px-3 py-3 align-top text-xs text-hack-dim">
                          <div>{asset.title || "-"}</div>
                          <div className="mt-1 flex flex-wrap gap-1">
                            {asset.web_server && (
                              <span className="rounded border border-hack-border px-1.5 py-0.5">
                                {asset.web_server}
                              </span>
                            )}
                            {techs.map((tech, index) => (
                              <span
                                key={`${tech}-${index}`}
                                className="rounded border border-hack-border px-1.5 py-0.5"
                              >
                                {tech}
                              </span>
                            ))}
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            ) : (
              <table className="min-w-full border-collapse text-sm">
                <thead className="border-b border-hack-border bg-black/30">
                  <tr>
                    <SortableHeader field="value" label="Resource Locator" />
                    <SortableHeader field="source" label="Source" />
                    <SortableHeader field="created_at" label="Created" />
                  </tr>
                </thead>
                <tbody>
                  {displayedUrls.map((url: any) => (
                    <tr
                      key={url.id || url.value}
                      className="border-b border-hack-border/50 hover:bg-white/[0.02]"
                    >
                      <td className="max-w-3xl px-3 py-3 align-top">
                        {String(url.value).endsWith(".js") ? (
                          <FileCode className="mr-1 inline h-4 w-4 text-hack-warning" />
                        ) : (
                          <FileText className="mr-1 inline h-4 w-4 text-hack-dim" />
                        )}
                        <a
                          href={url.value}
                          target="_blank"
                          rel="noreferrer"
                          className="break-all font-mono text-hack-primary hover:underline"
                        >
                          {url.value}
                        </a>
                      </td>
                      <td className="px-3 py-3 align-top text-xs uppercase tracking-wider text-hack-dim">
                        {url.source || "UNKNOWN"}
                      </td>
                      <td className="px-3 py-3 align-top text-xs text-hack-dim">
                        {url.created_at
                          ? new Date(url.created_at).toLocaleString()
                          : "-"}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>

          <div className="flex items-center justify-between border-t border-hack-border px-3 py-2 text-xs text-hack-dim">
            <span>
              PAGE {page} // RECORDS:{" "}
              {Number(totalRecords || 0).toLocaleString()}
            </span>
            <div className="flex gap-2">
              <button
                onClick={() => setPage((prev) => Math.max(1, prev - 1))}
                disabled={page <= 1}
                className="hack-btn-ghost hover:bg-white/5 disabled:opacity-30"
              >
                PREV
              </button>
              <button
                onClick={() => setPage((prev) => prev + 1)}
                className="hack-btn-ghost hover:bg-white/5 disabled:opacity-30"
              >
                NEXT
              </button>
            </div>
          </div>
        </div>
      )}

      <div className="hidden text-hack-dim">
        <Terminal />
      </div>
    </div>
  );
};

export default TargetAssets;
