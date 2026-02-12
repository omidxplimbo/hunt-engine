import { useState, useEffect } from 'react';
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { getSystemConfig, updateSystemConfig, type SystemConfig } from '../api/system';import { Settings, Save } from 'lucide-react';
import toast from 'react-hot-toast';

export const ConcurrencyConfig = () => {
  const queryClient = useQueryClient();
  const [maxScans, setMaxScans] = useState<string>('');

  const { data: configs, isLoading, error } = useQuery({
    queryKey: ['systemConfig'],
    queryFn: getSystemConfig,
  });

  useEffect(() => {
    if (configs) {
      console.log("System Configs Loaded:", configs);
      const cfg = configs.find((c: SystemConfig) => c.key === 'max_concurrent_scans');
      if (cfg) {
        setMaxScans(cfg.value);
      } else {
        setMaxScans('2');
      }
    }
  }, [configs]);

  const updateMutation = useMutation({
    mutationFn: (val: string) => updateSystemConfig('max_concurrent_scans', val),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['systemConfig'] });
      toast.success('Configuration updated');
    },
    onError: (err: any) => {
      const errorMessage = err.response?.data?.error || 'Failed to update configuration';
      toast.error(errorMessage);
    }
  });

  if (isLoading) {
    return <div className="text-sm text-gray-500">Loading configuration...</div>;
  }

  if (error) {
    return <div className="text-red-500">Error loading config</div>;
  }

  const handleSave = () => {
    const val = parseInt(maxScans);
    if (isNaN(val) || val < 1) {
      toast.error('Please enter a valid number >= 1');
      return;
    }
    updateMutation.mutate(maxScans);
  };

  return (
    <div className="hack-box p-6 relative overflow-hidden">
      <div className="flex items-center gap-2 mb-4 text-hack-primary font-mono text-sm tracking-wider uppercase border-b border-hack-border/50 pb-2">
        <Settings size={16} />
        <span>Queue Configuration</span>
      </div>

      <div className="flex items-end gap-4">
        <div className="flex-1 max-w-xs">
          <label className="block text-hack-dim text-xs mb-2 font-mono">Max Concurrent Scans</label>
          <input
            type="number"
            min="1"
            value={maxScans}
            onChange={(e) => setMaxScans(e.target.value)}
            className="w-full bg-black/50 border border-hack-border text-white px-3 py-2 font-mono focus:border-hack-primary focus:outline-none transition-colors"
          />
        </div>
        <button 
          onClick={handleSave}
          disabled={updateMutation.isPending || isLoading}
          className="hack-btn h-[42px] px-6"
        >
          {updateMutation.isPending ? 'SAVING...' : <><Save size={16} /> SAVE</>}
        </button>
      </div>
      <p className="mt-4 text-xs text-hack-dim/70 font-mono">
        Controls how many targets can be in 'SCANNING' state simultaneously. 
        Excess requests will be queued.
      </p>
    </div>
  );
};