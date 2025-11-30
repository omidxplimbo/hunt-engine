import { useState } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
// 👇 ایمپورت توابع جدید
import { getTargets, deleteTarget, stopTarget, startDiscovery } from '../api/targets';
// 👇 ایمپورت آیکون‌های جدید (Square, Play)
import { Plus, Search, Globe, Clock, Database, Trash2, Edit2, Activity, Square, Play } from 'lucide-react';
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
    refetchInterval: 2000, // رفرش سریع برای دیدن تغییر وضعیت
  });

  // --- Mutations ---

  const deleteMutation = useMutation({
    mutationFn: deleteTarget,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  // 👇 توقف اسکن
  const stopMutation = useMutation({
    mutationFn: stopTarget,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  // 👇 ادامه اسکن (شروع مجدد)
  const resumeMutation = useMutation({
    mutationFn: startDiscovery,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ['targets'] }),
  });

  // --- Handlers ---

  const handleDelete = (id: number) => {
    if (confirm('Are you sure? This will delete ALL assets and history for this target.')) {
      deleteMutation.mutate(id);
    }
  };

  const handleStop = (id: number) => {
    // نیازی به confirm نیست، چون توقف غیرمخرب است
    stopMutation.mutate(id);
  };

  const handleResume = (id: number) => {
    resumeMutation.mutate(id);
  };

  if (isLoading) return <div className="p-8 text-gray-400">Loading targets...</div>;
  if (isError) return <div className="p-8 text-red-500">Error loading targets!</div>;

  return (
    <div>
      <div className="flex justify-between items-center mb-8">
        <div>
          <h1 className="text-2xl font-bold text-white">Targets</h1>
          <p className="text-gray-400 mt-1">Manage your scope and hunting objectives</p>
        </div>
        <button 
          onClick={() => setIsCreateOpen(true)}
          className="bg-blue-600 hover:bg-blue-700 text-white px-4 py-2 rounded-lg flex items-center gap-2 transition-colors font-medium"
        >
          <Plus size={18} />
          Add Target
        </button>
      </div>

      {/* Search Bar */}
      <div className="bg-gray-900 p-4 rounded-lg border border-gray-800 mb-6 flex gap-4">
        <div className="relative flex-1">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 text-gray-500" size={18} />
          <input 
            type="text" 
            placeholder="Search targets..." 
            className="w-full bg-gray-950 border border-gray-800 text-gray-200 pl-10 pr-4 py-2 rounded-md focus:outline-none focus:border-blue-500"
          />
        </div>
      </div>

      {/* Table */}
      <div className="bg-gray-900 rounded-lg border border-gray-800 overflow-hidden">
        <table className="w-full text-left">
          <thead>
            <tr className="bg-gray-800/50 text-gray-400 text-sm uppercase">
              <th className="px-6 py-4 font-semibold">Target Name</th>
              <th className="px-6 py-4 font-semibold">Assets</th>
              <th className="px-6 py-4 font-semibold">Last Scan</th>
              <th className="px-6 py-4 font-semibold">Status / Phase</th>
              <th className="px-6 py-4 font-semibold text-right">Actions</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-gray-800">
            {data?.data.map((target) => (
              <tr key={target.id} className="hover:bg-gray-800/30 transition-colors">
                {/* Name */}
                <td className="px-6 py-4">
                  <div className="flex items-center gap-3">
                    <div className="w-10 h-10 rounded-lg bg-gray-800 flex items-center justify-center text-blue-500">
                      <Globe size={20} />
                    </div>
                    <div>
                      <div className="font-medium text-white">{target.name}</div>
                      <div className="text-sm text-gray-500">{target.root_domain}</div>
                    </div>
                  </div>
                </td>

                {/* Assets Count */}
                <td className="px-6 py-4">
                  <div className="flex items-center gap-2 text-gray-300">
                    <Database size={16} className="text-gray-500" />
                    <span className="font-mono text-lg">{target.asset_count.toLocaleString()}</span>
                  </div>
                </td>

                {/* Last Scan */}
                <td className="px-6 py-4">
                  <div className="flex items-center gap-2 text-gray-400 text-sm">
                    <Clock size={16} />
                    {target.last_scan_at 
                      ? new Date(target.last_scan_at).toLocaleString() 
                      : 'Never'}
                  </div>
                </td>

                {/* Status / Phase */}
                <td className="px-6 py-4">
                  <div className="flex flex-col gap-1">
                    <span className={`inline-flex items-center w-fit px-2.5 py-0.5 rounded-full text-xs font-medium ${
                      target.status === 'SCANNING' 
                        ? 'bg-yellow-500/10 text-yellow-400 border border-yellow-500/20 animate-pulse' 
                        : target.status === 'PAUSED' // 👈 استایل برای حالت PAUSED
                        ? 'bg-orange-500/10 text-orange-400 border border-orange-500/20'
                        : 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20'
                    }`}>
                      {target.status}
                    </span>
                    
                    {/* نمایش فاز جاری */}
                    {target.current_phase && target.current_phase !== "IDLE" && (
                        <span className={`text-xs font-mono flex items-center gap-1.5 ${
                             target.status === 'SCANNING' ? 'text-blue-400' : 'text-gray-500'
                        }`}>
                            {target.status === 'SCANNING' && <Activity size={12} className="animate-spin" />}
                            {target.current_phase}
                        </span>
                    )}
                  </div>
                </td>

                {/* Actions */}
                <td className="px-6 py-4 text-right">
                  <div className="flex items-center justify-end gap-3">
                    
                    {/* 👇 دکمه توقف (فقط وقتی اسکنینگ است) */}
                    {target.status === 'SCANNING' && (
                      <button 
                        onClick={() => handleStop(target.id)} 
                        className="text-orange-400 hover:text-orange-300 transition-colors"
                        title="Stop Scan"
                      >
                          <Square size={18} fill="currentColor" />
                      </button>
                    )}

                    {/* 👇 دکمه ادامه/شروع (وقتی PAUSED یا READY است) */}
                    {(target.status === 'PAUSED' || target.status === 'READY') && (
                      <button 
                        onClick={() => handleResume(target.id)} 
                        className="text-green-400 hover:text-green-300 transition-colors"
                        title={target.status === 'PAUSED' ? "Resume Scan" : "Start Scan"}
                      >
                          <Play size={18} fill="currentColor" />
                      </button>
                    )}

                    <div className="w-px h-4 bg-gray-700 mx-1"></div> {/* جداکننده */}

                    <Link to={`/targets/${target.id}`} className="text-blue-400 hover:text-blue-300 text-sm font-medium">View</Link>
                    
                    <button onClick={() => setEditingTarget(target)} className="text-gray-400 hover:text-white">
                        <Edit2 size={18} />
                    </button>
                    <button onClick={() => handleDelete(target.id)} className="text-red-400 hover:text-red-300">
                        <Trash2 size={18} />
                    </button>
                  </div>
                </td>
              </tr>
            ))}
            
            {data?.data.length === 0 && (
              <tr>
                <td colSpan={5} className="px-6 py-12 text-center text-gray-500">
                  No targets found. Create your first target to start hunting!
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>

      <CreateTargetModal isOpen={isCreateOpen} onClose={() => setIsCreateOpen(false)} />
      
      <EditTargetModal 
        isOpen={!!editingTarget} 
        target={editingTarget} 
        onClose={() => setEditingTarget(null)} 
      />
    </div>
  );
};

export default TargetsPage;