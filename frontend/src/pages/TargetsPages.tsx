import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getTargets, deleteTarget, stopTarget, startDiscovery } from '../api/targets';
import { Plus, Globe, Clock, Database, Trash2, Edit2, Activity, Square, Play, Terminal } from 'lucide-react';
import { CreateTargetModal } from '../components/CreateTargetModal';
import { EditTargetModal } from '../components/EditTargetModal';
import { Link } from 'react-router-dom';
import type { Target } from '../types/target';

const TargetsPage = () => {
  const queryClient = useQueryClient();
  const [isCreateOpen, setIsCreateOpen] = useState(false);
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

  const resumeMutation = useMutation({
    mutationFn: startDiscovery,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  const handleDelete = (id: number) => {
    if (confirm('>> WARNING: DELETE OPERATION IS IRREVERSIBLE. CONFIRM?')) deleteMutation.mutate(id);
  };

  const handleStop = (id: number) => stopMutation.mutate(id);
  const handleResume = (id: number) => resumeMutation.mutate(id);

  if (isLoading) return <div className="p-8 text-hack-dim font-mono animate-pulse"> Initializing target list...</div>;
  if (isError) return <div className="p-8 text-hack-danger font-mono"> ERROR: Connection lost.</div>;

  return (
    <div>
      {/* Header Responsive */}
      <div className="flex flex-col md:flex-row justify-between items-start md:items-center mb-8 border-b border-hack-border/50 pb-4 gap-4">
        <div>
          <h1 className="hack-title text-xl md:text-2xl">ACTIVE TARGETS</h1>
          <p className="text-hack-dim text-xs font-mono mt-1 tracking-wider">Scope Management & Operations</p>
        </div>
        <button onClick={() => setIsCreateOpen(true)} className="hack-btn w-full md:w-auto">
          <Plus size={16} /> New Target
        </button>
      </div>

      <div className="hack-box p-1 mb-6 flex items-center">
        <div className="px-3 text-hack-primary"><Terminal size={18} /></div>
        <input 
          type="text" 
          placeholder="QUERY TARGET_DB..." 
          className="bg-transparent border-none text-hack-text w-full focus:ring-0 placeholder-hack-dim/50 font-mono text-sm py-2 min-w-0"
        />
        <div className="px-4 text-hack-dim text-xs animate-pulse hidden sm:block">_</div>
      </div>

      {/* Table Scroll Container */}
      <div className="hack-box overflow-hidden flex flex-col">
        <div className="overflow-x-auto">
            <table className="w-full text-left min-w-[800px]">
            <thead>
                <tr className="bg-black/40 text-hack-dim text-xs uppercase tracking-widest border-b border-hack-border">
                <th className="px-6 py-4 font-normal">Target Identity</th>
                <th className="px-6 py-4 font-normal">Intelligence</th>
                <th className="px-6 py-4 font-normal">Last Sweep</th>
                <th className="px-6 py-4 font-normal">Operational Status</th>
                <th className="px-6 py-4 font-normal text-right">Command</th>
                </tr>
            </thead>
            <tbody className="divide-y divide-hack-border/30">
                {data?.data.map((target) => (
                <tr key={target.id} className="hover:bg-hack-primary/5 transition-colors group">
                    <td className="px-6 py-4">
                    <div className="flex items-center gap-4">
                        <div className="w-10 h-10 border border-hack-primary/30 bg-black flex items-center justify-center text-hack-primary group-hover:shadow-neon transition-all flex-shrink-0">
                        <Globe size={20} />
                        </div>
                        <div className="min-w-0">
                        <div className="font-bold text-white font-mono tracking-wide truncate max-w-[150px]">{target.name}</div>
                        <div className="text-xs text-hack-dim font-mono truncate max-w-[150px]">{target.root_domain}</div>
                        </div>
                    </div>
                    </td>

                    <td className="px-6 py-4">
                    <div className="flex items-center gap-2 font-mono text-hack-text">
                        <Database size={14} className="text-hack-dim" />
                        <span className="text-lg">{target.asset_count.toLocaleString()}</span>
                    </div>
                    </td>

                    <td className="px-6 py-4">
                    <div className="flex items-center gap-2 text-hack-dim text-xs font-mono whitespace-nowrap">
                        <Clock size={14} />
                        {target.last_scan_at ? new Date(target.last_scan_at).toLocaleString() : 'PENDING'}
                    </div>
                    </td>

                    <td className="px-6 py-4">
                    <div className="flex flex-col gap-1.5">
                        <span className={`inline-flex items-center w-fit px-2 py-0.5 border text-[10px] font-bold uppercase tracking-wider ${
                        target.status === 'SCANNING' 
                            ? 'bg-hack-warning/10 text-hack-warning border-hack-warning/50 animate-pulse' 
                            : target.status === 'PAUSED'
                            ? 'bg-hack-danger/10 text-hack-danger border-hack-danger/50'
                            : 'bg-hack-primary/10 text-hack-primary border-hack-primary/50'
                        }`}>
                        {target.status}
                        </span>
                        
                        {target.current_phase && target.current_phase !== "IDLE" && (
                            <span className="text-[10px] font-mono text-hack-dim flex items-center gap-1.5 truncate max-w-[200px]">
                                {target.status === 'SCANNING' && <Activity size={10} className="animate-spin text-hack-primary" />}
                                {target.current_phase}
                            </span>
                        )}
                    </div>
                    </td>

                    <td className="px-6 py-4 text-right">
                    <div className="flex items-center justify-end gap-2">
                        {target.status === 'SCANNING' && (
                        <button onClick={() => handleStop(target.id)} className="p-2 hover:text-hack-warning transition-colors" title="HALT">
                            <Square size={16} fill="currentColor" />
                        </button>
                        )}
                        {(target.status === 'PAUSED' || target.status === 'READY') && (
                        <button onClick={() => handleResume(target.id)} className="p-2 hover:text-hack-primary transition-colors" title="EXECUTE">
                            <Play size={16} fill="currentColor" />
                        </button>
                        )}
                        <div className="w-px h-4 bg-hack-border mx-2"></div>
                        <Link to={`/targets/${target.id}`} className="text-xs font-mono text-hack-primary hover:underline underline-offset-4 mr-2">DATA</Link>
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
      </div>

      <CreateTargetModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} />
      <EditTargetModal isOpen={!!editingTarget} target={editingTarget} onClose={() => setEditingTarget(null)} />
    </div>
  );
};

export default TargetsPage;