import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { getTargets, exportTargets } from '../api/targets';
import { X, Download, Loader2, CheckSquare, Square } from 'lucide-react';
import clsx from 'clsx';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

export const ExportTargetModal = ({ isOpen, onClose }: Props) => {
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set());
  const [isExporting, setIsExporting] = useState(false);

  const { data: targetsData, isLoading } = useQuery({
    queryKey: ['targets'],
    queryFn: () => getTargets(1, 1000), // دریافت همه تارگت‌ها
    enabled: isOpen,
  });

  const toggleSelect = (id: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  const toggleSelectAll = () => {
    if (!targetsData?.data) return;
    
    if (selectedIds.size === targetsData.data.length) {
      setSelectedIds(new Set());
    } else {
      setSelectedIds(new Set(targetsData.data.map(t => t.id)));
    }
  };

  const handleExport = async () => {
    setIsExporting(true);
    try {
      const ids = selectedIds.size > 0 ? Array.from(selectedIds) : undefined;
      await exportTargets(ids);
      onClose();
      setSelectedIds(new Set());
    } catch (error) {
      alert('Failed to export targets');
    } finally {
      setIsExporting(false);
    }
  };

  if (!isOpen) return null;

  const allSelected = targetsData?.data && selectedIds.size === targetsData.data.length;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
      <div className="hack-box w-full max-w-2xl relative animate-in fade-in zoom-in duration-200 max-h-[90vh] flex flex-col">
        <div className="absolute top-0 left-0 w-full h-[1px] bg-gradient-to-r from-transparent via-hack-primary to-transparent opacity-50"></div>
        
        <div className="flex justify-between items-center p-4 border-b border-hack-border flex-shrink-0">
          <div className="flex items-center gap-2 text-hack-primary">
            <Download size={18} />
            <h2 className="font-mono font-bold tracking-widest text-lg">EXPORT TARGETS</h2>
          </div>
          <button onClick={onClose} className="text-hack-dim hover:text-hack-danger transition-colors">
            <X size={18} />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-6 space-y-4">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 size={24} className="animate-spin text-hack-primary" />
            </div>
          ) : (
            <>
              <div className="flex items-center justify-between mb-4">
                <label className="text-[10px] uppercase text-hack-dim tracking-widest">
                  Select Targets to Export
                </label>
                <button
                  onClick={toggleSelectAll}
                  className="text-xs text-hack-primary hover:text-hack-primary/80 font-mono flex items-center gap-2"
                >
                  {allSelected ? <CheckSquare size={16} /> : <Square size={16} />}
                  {allSelected ? 'Deselect All' : 'Select All'}
                </button>
              </div>

              <div className="space-y-2 max-h-[400px] overflow-y-auto">
                {targetsData?.data.map((target) => {
                  const isSelected = selectedIds.has(target.id);
                  return (
                    <div
                      key={target.id}
                      onClick={() => toggleSelect(target.id)}
                      className={clsx(
                        "p-3 border cursor-pointer transition-all text-sm font-mono",
                        isSelected
                          ? "bg-hack-primary/10 border-hack-primary text-hack-primary"
                          : "border-hack-border text-hack-dim hover:border-hack-dim hover:text-white"
                      )}
                    >
                      <div className="flex items-center gap-3">
                        <div className="flex-shrink-0">
                          {isSelected ? <CheckSquare size={18} /> : <Square size={18} />}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="font-bold text-white truncate">{target.name}</div>
                          <div className="text-xs text-hack-dim truncate">{target.root_domain}</div>
                          <div className="text-xs text-hack-dim mt-1">
                            {target.asset_count.toLocaleString()} assets
                          </div>
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>

              {targetsData?.data.length === 0 && (
                <div className="text-center py-8 text-hack-dim text-sm">
                  No targets found
                </div>
              )}
            </>
          )}
        </div>

        <div className="p-6 border-t border-hack-border flex-shrink-0 space-y-3">
          <div className="text-xs text-hack-dim font-mono">
            {selectedIds.size === 0 
              ? "Exporting all targets with all related data (assets, URLs, etc.)"
              : `Exporting ${selectedIds.size} target(s) with all related data (assets, URLs, etc.)`
            }
          </div>
          <div className="flex gap-3">
            <button
              type="button"
              onClick={onClose}
              className="flex-1 hack-btn-ghost border border-hack-border"
            >
              Cancel
            </button>
            <button
              type="button"
              onClick={handleExport}
              disabled={isExporting}
              className="flex-1 hack-btn"
            >
              {isExporting ? <Loader2 size={16} className="animate-spin" /> : 'EXPORT'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};

