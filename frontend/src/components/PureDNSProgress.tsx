import { useQuery } from "@tanstack/react-query";
import { Activity } from "lucide-react";
import { getTargetScanState } from "../api/targets";

type PureDNSProgressMeta = {
  status?: string;
  domain?: string;
  wordlist_index?: number;
  wordlist_total?: number;
  wordlist_name?: string;
  total_lines?: number;
  processed_lines?: number;
  percent?: number;
  rate_per_second?: number;
  elapsed_seconds?: number;
  eta_seconds?: number;
  public_resolvers?: number;
  trusted_resolvers?: number;
  progress_source?: string;
};

const formatNumber = (value?: number) => {
  if (value === undefined || value === null || Number.isNaN(value)) return "0";
  return Math.round(value).toLocaleString();
};

const formatDuration = (seconds?: number) => {
  if (!seconds || seconds <= 0) return "—";

  const s = Math.round(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;

  if (h > 0) return `${h}h ${m}m`;
  if (m > 0) return `${m}m ${sec}s`;
  return `${sec}s`;
};

const parseMeta = (raw?: string): PureDNSProgressMeta | null => {
  if (!raw) return null;

  try {
    const parsed = JSON.parse(raw);
    return parsed?.puredns_progress || null;
  } catch {
    return null;
  }
};

export const PureDNSProgress = ({ targetId }: { targetId: number }) => {
  const { data } = useQuery({
    queryKey: ["target-scan-state", targetId],
    queryFn: () => getTargetScanState(targetId),
    refetchInterval: 5000,
  });

  const progress = parseMeta(data?.meta);
  if (!progress) return null;

  const scanStatus = String(data?.status || "").toUpperCase();
  const currentModule = String(data?.current_module || "").toUpperCase();
  const currentStep = String(data?.current_step || "").toUpperCase();

  const status = String(progress.status || "").toLowerCase();
  if (!["running", "stopping", "completed", "failed", "done"].includes(status)) {
    return null;
  }

  const isActivePureDNSStep =
    scanStatus === "RUNNING" &&
    currentModule === "DISCOVERY" &&
    currentStep === "PUREDNS";

  if (!isActivePureDNSStep) {
    return null;
  }

  const percent = Math.max(0, Math.min(100, Number(progress.percent || 0)));

  return (
    <div className="mt-3 w-[250px] max-w-[250px] border border-hack-primary/30 bg-black/40 p-3">
      <div className="mb-2 flex items-center justify-between gap-2">
        <div className="flex items-center gap-2 font-mono text-[11px] uppercase tracking-wider text-hack-primary">
          <Activity size={13} />
          PureDNS
        </div>
        <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
          {progress.status || "unknown"}
        </div>
      </div>

      <div className="mb-2 break-words font-mono text-[10px] leading-4 text-hack-dim">
        WL {progress.wordlist_index || 0}/{progress.wordlist_total || 0}
        {progress.wordlist_name ? ` · ${progress.wordlist_name}` : ""}
      </div>

      <div className="mb-2 h-2 w-full overflow-hidden border border-hack-border bg-black">
        <div className="h-full bg-hack-primary" style={{ width: `${percent}%` }} />
      </div>

      <div className="space-y-1 font-mono text-[10px] leading-4 text-hack-dim">
        <div className="flex items-center justify-between gap-2">
          <span>Done</span>
          <span className="text-white">{percent.toFixed(1)}%</span>
        </div>

        <div className="flex items-center justify-between gap-2">
          <span>Processed</span>
          <span className="text-right text-white">
            {formatNumber(progress.processed_lines)}
          </span>
        </div>

        <div className="flex items-center justify-between gap-2">
          <span>Total</span>
          <span className="text-right text-white">
            {formatNumber(progress.total_lines)}
          </span>
        </div>

        <div className="flex items-center justify-between gap-2">
          <span>Rate</span>
          <span className="text-right text-white">
            {formatNumber(progress.rate_per_second)}/s
          </span>
        </div>

        <div className="flex items-center justify-between gap-2">
          <span>ETA</span>
          <span className="text-right text-white">
            {formatDuration(progress.eta_seconds)}
          </span>
        </div>

        <div className="flex items-center justify-between gap-2">
          <span>Elapsed</span>
          <span className="text-right text-white">
            {formatDuration(progress.elapsed_seconds)}
          </span>
        </div>

        <div className="flex items-center justify-between gap-2">
          <span>Resolvers</span>
          <span className="text-right text-white">
            {progress.public_resolvers || 0}/{progress.trusted_resolvers || 0}
          </span>
        </div>
      </div>
    </div>
  );
};

export default PureDNSProgress;
