import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Bug, CheckCircle2, Download, Filter, Loader2, ShieldAlert, TrendingUp } from 'lucide-react';
import clsx from 'clsx';

import {
  exportTargetFindings,
  getTargetFindingStats,
  getTargetFindings,
  updateFindingStatus,
  type FindingExportFormat,
} from '../api/findings';
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
const countValue = (value?: number) => value ?? 0;

const downloadBlob = (blob: Blob, filename: string) => {
  const url = window.URL.createObjectURL(blob);
  const link = document.createElement('a');
  link.href = url;
  link.download = filename;
  document.body.appendChild(link);
  link.click();
  link.remove();
  window.URL.revokeObjectURL(url);
};

export const FindingsPanel = ({ targetId }: Props) => {
  const queryClient = useQueryClient();
  const [status, setStatus] = useState<FindingStatus | 'all'>('all');
  const [severity, setSeverity] = useState<FindingSeverity | 'all'>('all');
  const [search, setSearch] = useState('');

  const currentFilters = { status, severity, search, limit: 100 };

  const statsQuery = useQuery({
    queryKey: ['target-findings-stats', targetId],
    queryFn: () => getTargetFindingStats(targetId),
    enabled: Boolean(targetId),
  });

  const findingsQuery = useQuery({
    queryKey: ['target-findings', targetId, status, severity, search],
    queryFn: () => getTargetFindings(targetId, currentFilters),
    enabled: Boolean(targetId),
  });

  const updateStatusMutation = useMutation({
    mutationFn: ({ findingId, nextStatus, triageNote }: { findingId: number; nextStatus: FindingStatus; triageNote?: string }) =>
      updateFindingStatus(findingId, nextStatus, triageNote || ''),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['target-findings', targetId] });
      queryClient.invalidateQueries({ queryKey: ['target-findings-stats', targetId] });
    },
  });

  const exportMutation = useMutation({
    mutationFn: async (format: FindingExportFormat) => {
      const blob = await exportTargetFindings(targetId, format, currentFilters);
      return { blob, format };
    },
    onSuccess: ({ blob, format }) => {
      const timestamp = new Date().toISOString().replace(/[:.]/g, '-');
      downloadBlob(blob, `target-${targetId}-findings-${timestamp}.${format}`);
    },
  });

  const findings = findingsQuery.data?.data || [];
  const stats = statsQuery.data?.data;
  const highPlus = countValue(stats?.by_severity?.critical) + countValue(stats?.by_severity?.high);

  const summaryCards = [
    { label: 'Total', value: countValue(stats?.total), icon: Bug, hint: 'All findings' },
    { label: 'Open', value: countValue(stats?.open), icon: ShieldAlert, hint: 'Needs review' },
    { label: 'High+', value: highPlus, icon: TrendingUp, hint: 'Critical + High' },
    { label: 'Fixed', value: countValue(stats?.fixed), icon: CheckCircle2, hint: 'No longer observed' },
  ];

  return (
    <div className="space-y-5">
      <div className="grid gap-3 md:grid-cols-4">
        {summaryCards.map((card) => {
          const Icon = card.icon;
          return (
            <div key={card.label} className="border border-hack-border bg-black/20 p-4">
              <div className="flex items-center justify-between text-xs font-bold uppercase tracking-[0.2em] text-hack-dim">
                <span>{card.label}</span>
                <Icon className="h-4 w-4 text-hack-primary" />
              </div>
              <div className="mt-2 font-mono text-2xl font-bold text-white">
                {statsQuery.isLoading ? '...' : card.value}
              </div>
              <div className="mt-1 text-[10px] uppercase tracking-wider text-hack-dim">{card.hint}</div>
            </div>
          );
        })}
      </div>

      <div className="rounded border border-hack-border bg-black/20 p-4">
        <div className="mb-3 flex flex-wrap items-center justify-between gap-3">
          <div className="flex items-center gap-2 text-xs font-bold uppercase tracking-[0.2em] text-hack-primary">
            <Filter className="h-4 w-4" />
            Findings Filters
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => exportMutation.mutate('csv')}
              disabled={exportMutation.isPending}
              className="inline-flex items-center gap-2 border border-hack-primary/40 px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-hack-primary hover:bg-hack-primary/10 disabled:opacity-50"
            >
              <Download className="h-3 w-3" />
              Export CSV
            </button>
            <button
              type="button"
              onClick={() => exportMutation.mutate('json')}
              disabled={exportMutation.isPending}
              className="inline-flex items-center gap-2 border border-hack-border px-3 py-2 text-[10px] font-bold uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-50"
            >
              <Download className="h-3 w-3" />
              Export JSON
            </button>
          </div>
        </div>

        <div className="grid gap-3 md:grid-cols-3">
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
              <option key={option} value={option}>
                {label(option)}
              </option>
            ))}
          </select>

          <select
            value={status}
            onChange={(event) => setStatus(event.target.value as FindingStatus | 'all')}
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
        <div className="flex items-center gap-2 border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
          <Loader2 className="h-4 w-4 animate-spin" />
          Loading findings...
        </div>
      )}

      {!findingsQuery.isLoading && findings.length === 0 && (
        <div className="border border-dashed border-hack-border bg-black/20 p-6 text-center text-sm text-hack-dim">
          No findings match the current filters. Future detection modules will keep populating this tab.
        </div>
      )}

      <div className="space-y-3">
        {findings.map((finding: Finding) => (
          <div key={finding.id} className="border border-hack-border bg-black/20 p-4">
            <div className="mb-3 flex flex-wrap items-center gap-2">
              <span className={clsx('rounded border px-2 py-1 text-[10px] font-bold uppercase tracking-wider', severityClass[finding.severity])}>
                {finding.severity}
              </span>
              <span className={clsx('rounded border px-2 py-1 text-[10px] font-bold uppercase tracking-wider', statusClass[finding.status])}>
                {label(finding.status)}
              </span>
              {finding.source_tool && (
                <span className="rounded border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim">
                  {finding.source_tool}
                </span>
              )}
              {finding.category && (
                <span className="rounded border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim">
                  {finding.category}
                </span>
              )}
            </div>

            <div className="flex items-start gap-3">
              <ShieldAlert className="mt-1 h-5 w-5 shrink-0 text-hack-primary" />
              <div className="min-w-0 flex-1">
                <h3 className="font-mono text-sm font-bold text-white">{finding.title}</h3>

                {finding.description && <p className="mt-2 text-sm leading-6 text-hack-dim">{finding.description}</p>}

                {finding.evidence && (
                  <div className="mt-3 rounded border border-hack-border bg-black/30 p-3">
                    <div className="mb-1 flex items-center gap-2 text-[10px] font-bold uppercase tracking-[0.2em] text-hack-primary">
                      <Bug className="h-3 w-3" />
                      Evidence
                    </div>
                    <pre className="whitespace-pre-wrap break-words font-mono text-xs leading-5 text-hack-dim">{finding.evidence}</pre>
                  </div>
                )}

                {finding.recommendation && (
                  <div className="mt-3 rounded border border-hack-primary/20 bg-hack-primary/5 p-3 text-xs leading-5 text-hack-dim">
                    <span className="font-bold uppercase tracking-wider text-hack-primary">Recommendation: </span>
                    {finding.recommendation}
                  </div>
                )}

                <div className="mt-3 text-[10px] uppercase tracking-wider text-hack-dim">
                  First seen: {finding.first_seen ? new Date(finding.first_seen).toLocaleString() : '-'} · Last seen:{' '}
                  {finding.last_seen ? new Date(finding.last_seen).toLocaleString() : '-'}
                </div>

                <div className="mt-4 flex flex-wrap gap-2">
                  {(['open', 'accepted', 'false_positive', 'fixed'] as FindingStatus[]).map((nextStatus) => (
                    <button
                      key={nextStatus}
                      type="button"
                      disabled={finding.status === nextStatus || updateStatusMutation.isPending}
                      onClick={() => {
                        const triageNote = window.prompt(`Optional triage note for ${label(nextStatus)}:`, finding.triage_note || '') || '';
                        updateStatusMutation.mutate({ findingId: finding.id, nextStatus, triageNote });
                      }}
                      className="border border-hack-border px-2 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:border-hack-primary hover:text-hack-primary disabled:opacity-40"
                    >
                      {nextStatus === 'fixed' && <CheckCircle2 className="mr-1 inline h-3 w-3" />}
                      {label(nextStatus)}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
};

export default FindingsPanel;
