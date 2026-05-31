import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { getTargets, deleteTarget, stopTarget, startDiscovery, restartTargetScan } from '../api/targets';
import { Plus, Globe, Clock, Database, Trash2, Edit2, Square, Play, Download, Upload, User2 } from 'lucide-react';
import { CreateTargetModal } from '../components/CreateTargetModal';
import { EditTargetModal } from '../components/EditTargetModal';
import { ImportTargetModal } from '../components/ImportTargetModal';
import { ExportTargetModal } from '../components/ExportTargetModal';
import type { Target } from '../types/target';

const TargetsPage = () => {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [isImportOpen, setIsImportOpen] = useState(false);
  const [isExportOpen, setIsExportOpen] = useState(false);
  const [editingTarget, setEditingTarget] = useState<Target | null>(null);

  const { data, isLoading, isError } = useQuery({
    queryKey: ['targets'],
    queryFn: () => getTargets(1, 50),
    refetchInterval: 2000,
  });

  const deleteMutation = useMutation({
    mutationFn: deleteTarget,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  const stopMutation = useMutation({
    mutationFn: stopTarget,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  const restartMutation = useMutation({
    mutationFn: (id: number) => restartTargetScan(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['targets'] });
    },
  });

  const resumeMutation = useMutation({
    mutationFn: startDiscovery,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  const handleDelete = (id: number) => {
    if (confirm('>> WARNING: DELETE OPERATION IS IRREVERSIBLE.\nCONFIRM?')) deleteMutation.mutate(id);
  };

  if (isLoading) return <div className="font-mono text-hack-dim">Initializing target list...</div>;
  if (isError) return <div className="font-mono text-hack-danger">ERROR: Connection lost.</div>;

  return (
    <div className="space-y-6">
      <div className="flex flex-col md:flex-row md:items-start md:justify-between gap-4">
        <div>
          <h1 className="font-mono text-2xl uppercase tracking-wider text-hack-primary">ACTIVE TARGETS</h1>
          <p className="text-hack-dim font-mono text-sm">Scope Management & Operations</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button onClick={() => setIsExportOpen(true)} className="hack-btn-ghost border border-hack-border flex items-center gap-2">
            <Download size={16} /> Export
          </button>
          <button onClick={() => setIsImportOpen(true)} className="hack-btn-ghost border border-hack-border flex items-center gap-2">
            <Upload size={16} /> Import
          </button>
          <button onClick={() => setIsCreateOpen(true)} className="hack-btn flex items-center gap-2">
            <Plus size={16} /> New Target
          </button>
        </div>
      </div>

      <div className="border border-hack-primary/30 bg-black/30 p-4 font-mono text-hack-primary">&gt;_ QUERY TARGET_DB ...</div>

      <div className="overflow-x-auto border border-hack-primary/30 bg-black/30">
        <table className="w-full text-left font-mono text-sm">
          <thead>
            <tr className="border-b border-hack-border text-hack-dim uppercase text-xs tracking-wider">
              <th className="py-4 px-6">Target Identity</th>
              <th className="py-4 px-6">Owner</th>
              <th className="py-4 px-6">Intelligence</th>
              <th className="py-4 px-6">Last Sweep</th>
              <th className="py-4 px-6">Operational Status</th>
              <th className="py-4 px-6 text-right">Command</th>
            </tr>
          </thead>
          <tbody>
            {data?.data.map((target: Target) => (
              <tr key={target.id} className="border-b border-hack-border/50 hover:bg-hack-primary/5 transition-colors">
                <td className="py-4 px-6">
                  <div className="flex items-center gap-4">
                    <div className="border border-hack-primary/50 p-3 text-hack-primary">
                      <Globe size={20} />
                    </div>
                    <div>
                      <div className="text-white text-lg font-bold">{target.name}</div>
                      <div className="text-hack-dim">{target.root_domain}</div>
                    </div>
                  </div>
                </td>
                <td className="py-4 px-6">
                  <div className="inline-flex items-center gap-2 text-hack-dim">
                    <User2 size={15} />
                    <span>{target.owner_username || target.created_by_user_id || '-'}</span>
                  </div>
                </td>
                <td className="py-4 px-6">
                  <div className="inline-flex items-center gap-2 text-white">
                    <Database size={16} className="text-hack-dim" /> {Number(target.asset_count || 0).toLocaleString()}
                  </div>
                </td>
                <td className="py-4 px-6 text-hack-dim">
                  <div className="inline-flex items-center gap-2">
                    <Clock size={16} /> {target.last_scan_at ? new Date(target.last_scan_at).toLocaleString() : 'PENDING'}
                  </div>
                </td>
                <td className="py-4 px-6">
                  <span className="border border-hack-primary/50 px-2 py-1 text-xs uppercase text-hack-primary">{target.status}</span>
                  {target.current_phase && target.current_phase !== 'IDLE' && (
                    <div className="mt-2 text-[11px] text-hack-dim uppercase">{target.current_phase}</div>
                  )}
                </td>
                <td className="py-4 px-6 text-right">
                  <div className="inline-flex items-center gap-3">
                    {(target.status === 'SCANNING' || target.status === 'QUEUED') && (
                      <button onClick={() => stopMutation.mutate(target.id)} className="p-2 hover:text-hack-warning transition-colors" title="HALT">
                        <Square size={18} />
                      </button>
                    )}
                    {(target.status === 'PAUSED' || target.status === 'READY') && (
                      <>

                      <button
                        onClick={() => {
                          if (window.confirm('Fresh restart this target scan? Checkpoints/temp scan state will be reset, but existing assets/findings remain.')) {
                            restartMutation.mutate(target.id);
                          }
                        }}
                        className="p-2 hover:text-yellow-400 transition-colors"
                        title="FRESH RESTART"
                      >
                        ↻
                      </button>
                      <button onClick={() => resumeMutation.mutate(target.id)} className="p-2 hover:text-hack-primary transition-colors" title="EXECUTE">
                        <Play size={18} />
                      </button>

                      </>
                    )}
                    <Link to={`/targets/${target.id}`} className="text-hack-primary hover:text-white transition-colors">
                      DATA
                    </Link>
                    <button onClick={() => setEditingTarget(target)} className="text-hack-dim hover:text-white transition-colors">
                      <Edit2 size={16} />
                    </button>
                    <button onClick={() => handleDelete(target.id)} className="text-hack-danger/70 hover:text-hack-danger transition-colors ml-1">
                      <Trash2 size={16} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <CreateTargetModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} />
      <EditTargetModal target={editingTarget} isOpen={!!editingTarget} onClose={() => setEditingTarget(null)} />
      <ImportTargetModal isOpen={isImportOpen} onClose={() => setIsImportOpen(false)} />
      <ExportTargetModal isOpen={isExportOpen} onClose={() => setIsExportOpen(false)} />
    </div>
  );
};

export default TargetsPage;
