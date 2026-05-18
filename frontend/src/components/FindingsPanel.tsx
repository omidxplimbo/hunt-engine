import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, Bug, CheckCircle2, Clock3, Loader2, Search, ShieldAlert, XCircle } from 'lucide-react';
import clsx from 'clsx';

import { getTargetFindings, updateFindingStatus } from '../api/findings';
import type { Finding, FindingSeverity, FindingStatus } from '../types/finding';

interface Props {
  targetId: number;
}

type StatusFilter = FindingStatus | 'all';
type SeverityFilter = FindingSeverity | 'all';

const severityOrder: FindingSeverity[] = ['critical', 'high', 'medium', 'low', 'info'];
const statusOptions: StatusFilter[] = ['all', 'open', 'accepted', 'false_positive', 'fixed'];
const severityOptions: SeverityFilter[] = ['all', ...severityOrder];

const severityClass: Record<FindingSeverity, string> = {
  critical: 'border-red-500 text-red-300 bg-red-950/40',
  high: 'border-orange-500 text-orange-300 bg-orange-950/40',
  medium: 'border-yellow-500 text-yellow-300 bg-yellow-950/30',
  low: 'border-blue-500 text-blue-300 bg-blue-950/30',
  info: 'border-hack-border text-hack-dim bg-black/30',
};

const statusClass: Record<FindingStatus, string> = {
  open: 'border-red-500/50 text-red-300 bg-red-950/20',
  accepted: 'border-yellow-500/50 text-yellow-300 bg-yellow-950/20',
  false_positive: 'border-purple-500/50 text-purple-300 bg-purple-950/20',
  fixed: 'border-hack-primary/50 text-hack-primary bg-hack-primary/10',
};

const sourceClass: Record<string, string> = {
  builtin: 'border-hack-primary/40 text-hack-primary bg-hack-primary/10',
  'builtin-url': 'border-cyan-400/40 text-cyan-300 bg-cyan-950/20',
  nuclei: 'border-red-400/50 text-red-300 bg-red-950/20',
  takeover: 'border-orange-400/50 text-orange-300 bg-orange-950/20',
};

const label = (value: string) => value.replace(/_/g, ' ').replace(/-/g, ' ').toUpperCase();

const formatDate = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return '-';
  return date.toLocaleString();
};

const SummaryCard = ({
  title,
  value,
  icon: Icon,
  className,
}: {
  title: string;
  value: number;
  icon: typeof ShieldAlert;
  className?: string;
}) => (
  <div className={clsx('border bg-black/20 p-4', className || 'border-hack-border')}>
    <div className="flex items-center justify-between gap-3">
      <div>
        <p className="text-[10px] uppercase tracking-[0.2em] text-hack-dim">{title}</p>
        <p className="mt-2 font-mono text-2xl font-bold text-white">{value}</p>
      </div>
      <Icon className="h-6 w-6 text-hack-primary" />
    </div>
  </div>
);

export const FindingsPanel = ({ targetId }: Props) => {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<StatusFilter>('all');
  const [severity, setSeverity] = useState<SeverityFilter>('all');
  const [search, setSearch] = useState('');

  const allFindingsQuery = useQuery({
    queryKey: ['target-findings-summary', targetId],
    queryFn: () => getTargetFindings(targetId, { limit: 500 }),
    enabled: Boolean(targetId),
  });

  const findingsQuery = useQuery({
    queryKey: ['target-findings', targetId, status, severity, search],
    queryFn: () => getTargetFindings(targetId, { status, severity, search, limit: 200 }),
    enabled: Boolean(targetId),
  });

  const updateStatusMutation = useMutation({
    mutationFn: ({ findingId, nextStatus }: { findingId: number; nextStatus: FindingStatus }) =>
      updateFindingStatus(findingId, nextStatus),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['target-findings', targetId] });
      queryClient.invalidateQueries({ queryKey: ['target-findings-summary', targetId] });
    },
  });

  const findings = findingsQuery.data?.data || [];
  const allFindings = allFindingsQuery.data?.data || [];

  const summary = useMemo(() => {
    const result = {
      total: allFindings.length,
      open: 0,
      highRisk: 0,
      fixed: 0,
    };

    for (const finding of allFindings) {
      if (finding.status === 'open') result.open += 1;
      if (finding.status === 'fixed') result.fixed += 1;
      if (finding.severity === 'critical' || finding.severity === 'high') result.highRisk += 1;
    }

    return result;
  }, [allFindings]);

  return (
    <div className="space-y-5">
      <div className="grid gap-3 md:grid-cols-4">
        <SummaryCard title="Total" value={summary.total} icon={Bug} />
        <SummaryCard title="Open" value={summary.open} icon={ShieldAlert} className="border-red-500/40" />
        <SummaryCard title="High+" value={summary.highRisk} icon={Activity} className="border-orange-500/40" />
        <SummaryCard title="Fixed" value={summary.fixed} icon={CheckCircle2} className="border-hack-primary/40" />
      </div>

      <div className="border border-hack-border bg-black/20 p-4">
        <div className="mb-3 flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-hack-primary">
          <Search className="h-4 w-4" />
          Findings Filters
        </div>
        <div className="grid gap-3 md:grid-cols-[1fr_180px_180px]">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search title, evidence, category..."
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          />
          <select
            value={severity}
            onChange={(event) => setSeverity(event.target.value as SeverityFilter)}
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          >
            {severityOptions.map((option) => (
              <option key={option} value={option}>{label(option)}</option>
            ))}
          </select>
          <select
            value={status}
            onChange={(event) => setStatus(event.target.value as StatusFilter)}
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          >
            {statusOptions.map((option) => (
              <option key={option} value={option}>{label(option)}</option>
            ))}
          </select>
        </div>
      </div>

      {findingsQuery.isLoading && (
        <div className="flex items-center gap-2 border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading findings...
        </div>
      )}

      {!findingsQuery.isLoading && findings.length === 0 && (
        <div className="border border-hack-border bg-black/20 p-8 text-center">
          <ShieldAlert className="mx-auto mb-3 h-8 w-8 text-hack-dim" />
          <p className="font-mono text-sm text-white">No findings matched the current filters.</p>
          <p className="mt-2 text-xs text-hack-dim">
            Built-in detectors run after probing and crawling. Future modules such as Nuclei and takeover checks will populate this tab too.
          </p>
        </div>
      )}

      <div className="space-y-3">
        {findings.map((finding: Finding) => (
          <article key={finding.id} className="border border-hack-border bg-black/20 p-4">
            <div className="flex flex-wrap items-center gap-2">
              <span className={clsx('border px-2 py-1 font-mono text-[10px] uppercase tracking-wider', severityClass[finding.severity])}>
                {finding.severity}
              </span>
              <span className={clsx('border px-2 py-1 font-mono text-[10px] uppercase tracking-wider', statusClass[finding.status])}>
                {label(finding.status)}
              </span>
              {finding.source_tool && (
                <span className={clsx('border px-2 py-1 font-mono text-[10px] uppercase tracking-wider', sourceClass[finding.source_tool] || 'border-hack-border text-hack-dim bg-black/30')}>
                  {finding.source_tool}
                </span>
              )}
              {finding.category && (
                <span className="border border-hack-border bg-black/30 px-2 py-1 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                  {finding.category}
                </span>
              )}
            </div>

            <h3 className="mt-3 font-mono text-base font-bold text-white">{finding.title}</h3>

            {finding.description && (
              <p className="mt-2 text-sm leading-6 text-hack-dim">{finding.description}</p>
            )}

            <div className="mt-3 grid gap-3 md:grid-cols-2">
              <div className="border border-hack-border bg-black/30 p-3">
                <div className="mb-2 flex items-center gap-2 text-[10px] font-bold uppercase tracking-[0.2em] text-hack-primary">
                  <Bug className="h-3.5 w-3.5" />
                  Evidence
                </div>
                <pre className="max-h-44 overflow-auto whitespace-pre-wrap break-words font-mono text-xs leading-5 text-hack-dim">
                  {finding.evidence || 'No evidence recorded.'}
                </pre>
              </div>

              <div className="border border-hack-border bg-black/30 p-3">
                <div className="mb-2 flex items-center gap-2 text-[10px] font-bold uppercase tracking-[0.2em] text-hack-primary">
                  <ShieldAlert className="h-3.5 w-3.5" />
                  Recommendation
                </div>
                <p className="text-xs leading-5 text-hack-dim">
                  {finding.recommendation || 'No recommendation recorded.'}
                </p>
              </div>
            </div>

            <div className="mt-3 flex flex-wrap items-center justify-between gap-3 border-t border-hack-border pt-3">
              <div className="flex flex-wrap items-center gap-3 text-[10px] uppercase tracking-wider text-hack-dim">
                <span className="flex items-center gap-1"><Clock3 className="h-3 w-3" /> First seen: {formatDate(finding.first_seen)}</span>
                <span>Last seen: {formatDate(finding.last_seen)}</span>
              </div>

              <div className="flex flex-wrap gap-2">
                {(['open', 'accepted', 'false_positive', 'fixed'] as FindingStatus[]).map((nextStatus) => (
                  <button
                    key={nextStatus}
                    type="button"
                    disabled={finding.status === nextStatus || updateStatusMutation.isPending}
                    onClick={() => updateStatusMutation.mutate({ findingId: finding.id, nextStatus })}
                    className={clsx(
                      'border px-2 py-1 text-[10px] uppercase tracking-wider transition-colors disabled:cursor-not-allowed disabled:opacity-40',
                      finding.status === nextStatus
                        ? statusClass[nextStatus]
                        : 'border-hack-border text-hack-dim hover:border-hack-primary hover:text-hack-primary'
                    )}
                  >
                    {nextStatus === 'fixed' && <CheckCircle2 className="mr-1 inline h-3 w-3" />}
                    {nextStatus === 'false_positive' && <XCircle className="mr-1 inline h-3 w-3" />}
                    {label(nextStatus)}
                  </button>
                ))}
              </div>
            </div>
          </article>
        ))}
      </div>
    </div>
  );
};

export default FindingsPanel;
