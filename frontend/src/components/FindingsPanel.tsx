import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bug, CheckCircle2, Filter, Loader2, ShieldAlert } from 'lucide-react';
import clsx from 'clsx';

import { getTargetFindings, updateFindingStatus } from '../api/findings';
import type { Finding, FindingSeverity, FindingStatus } from '../types/finding';

interface Props {
  targetId: number;
}

const severityOrder: FindingSeverity[] = ['critical', 'high', 'medium', 'low', 'info'];
const statusOptions: Array<FindingStatus | 'all'> = ['all', 'open', 'accepted', 'false_positive', 'fixed'];
const severityOptions: Array<FindingSeverity | 'all'> = ['all', ...severityOrder];

const severityClass: Record<FindingSeverity, string> = {
  critical: 'border-red-500 text-red-400 bg-red-950/40',
  high: 'border-orange-500 text-orange-400 bg-orange-950/40',
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

const label = (value: string) => value.replace(/_/g, ' ').toUpperCase();

export const FindingsPanel = ({ targetId }: Props) => {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<FindingStatus | 'all'>('all');
  const [severity, setSeverity] = useState<FindingSeverity | 'all'>('all');
  const [search, setSearch] = useState('');

  const findingsQuery = useQuery({
    queryKey: ['target-findings', targetId, status, severity, search],
    queryFn: () => getTargetFindings(targetId, { status, severity, search, limit: 100 }),
    enabled: Boolean(targetId),
  });

  const updateStatusMutation = useMutation({
    mutationFn: ({ findingId, nextStatus }: { findingId: number; nextStatus: FindingStatus }) =>
      updateFindingStatus(findingId, nextStatus),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['target-findings', targetId] });
    },
  });

  const findings = findingsQuery.data?.data || [];

  const stats = useMemo(() => {
    const result: Record<FindingSeverity, number> = {
      critical: 0,
      high: 0,
      medium: 0,
      low: 0,
      info: 0,
    };

    for (const finding of findings) {
      if (finding.severity in result) result[finding.severity] += 1;
    }

    return result;
  }, [findings]);

  return (
    <div className="space-y-4">
      <div className="grid gap-3 md:grid-cols-5">
        {severityOrder.map((sev) => (
          <div key={sev} className={clsx('border p-3', severityClass[sev])}>
            <div className="text-[10px] font-bold uppercase tracking-wider opacity-80">{sev}</div>
            <div className="mt-1 font-mono text-xl font-bold">{stats[sev]}</div>
          </div>
        ))}
      </div>

      <div className="flex flex-col gap-3 border border-hack-border bg-black/20 p-3 md:flex-row md:items-center md:justify-between">
        <div className="flex items-center gap-2 text-xs uppercase tracking-wider text-hack-dim">
          <Filter className="h-4 w-4" /> Findings Filters
        </div>

        <div className="flex flex-wrap gap-2">
          <input
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search findings..."
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          />

          <select
            value={severity}
            onChange={(event) => setSeverity(event.target.value as FindingSeverity | 'all')}
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          >
            {severityOptions.map((option) => (
              <option key={option} value={option}>{label(option)}</option>
            ))}
          </select>

          <select
            value={status}
            onChange={(event) => setStatus(event.target.value as FindingStatus | 'all')}
            className="border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
          >
            {statusOptions.map((option) => (
              <option key={option} value={option}>{label(option)}</option>
            ))}
          </select>
        </div>
      </div>

      {findingsQuery.isLoading && (
        <div className="flex items-center justify-center border border-hack-border bg-black/20 p-10 text-hack-dim">
          <Loader2 className="mr-2 h-4 w-4 animate-spin" /> Loading findings...
        </div>
      )}

      {!findingsQuery.isLoading && findings.length === 0 && (
        <div className="border border-dashed border-hack-border bg-black/20 p-10 text-center text-hack-dim">
          <ShieldAlert className="mx-auto mb-3 h-8 w-8 opacity-60" />
          No findings yet. Future detection modules will populate this tab.
        </div>
      )}

      <div className="space-y-3">
        {findings.map((finding: Finding) => (
          <div key={finding.id} className="border border-hack-border bg-black/20 p-4">
            <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
              <div className="min-w-0 space-y-2">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={clsx('border px-2 py-1 text-[10px] font-bold uppercase tracking-wider', severityClass[finding.severity])}>
                    {finding.severity}
                  </span>
                  <span className={clsx('border px-2 py-1 text-[10px] font-bold uppercase tracking-wider', statusClass[finding.status])}>
                    {label(finding.status)}
                  </span>
                  {finding.source_tool && (
                    <span className="border border-hack-border bg-black/30 px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim">
                      {finding.source_tool}
                    </span>
                  )}
                  {finding.category && (
                    <span className="border border-hack-border bg-black/30 px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim">
                      {finding.category}
                    </span>
                  )}
                </div>

                <h3 className="flex items-center gap-2 font-mono text-sm font-bold text-white">
                  <Bug className="h-4 w-4 text-hack-primary" />
                  {finding.title}
                </h3>

                {finding.description && (
                  <p className="text-sm leading-6 text-hack-dim">{finding.description}</p>
                )}

                {finding.evidence && (
                  <pre className="max-h-40 overflow-auto border border-hack-border bg-black/40 p-3 text-xs text-hack-dim">
                    {finding.evidence}
                  </pre>
                )}

                {finding.recommendation && (
                  <p className="text-xs text-hack-dim">
                    <span className="text-hack-primary">Recommendation:</span> {finding.recommendation}
                  </p>
                )}
              </div>

              <div className="flex shrink-0 flex-wrap gap-2">
                {(['open', 'accepted', 'false_positive', 'fixed'] as FindingStatus[]).map((nextStatus) => (
                  <button
                    key={nextStatus}
                    disabled={finding.status === nextStatus || updateStatusMutation.isPending}
                    onClick={() => updateStatusMutation.mutate({ findingId: finding.id, nextStatus })}
                    className="border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-40"
                  >
                    {nextStatus === 'fixed' && <CheckCircle2 className="mr-1 inline h-3 w-3" />}
                    {label(nextStatus)}
                  </button>
                ))}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default FindingsPanel;
