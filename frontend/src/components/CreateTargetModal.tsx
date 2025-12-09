import { useState } from 'react';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { createTarget, type CreateTargetPayload } from '../api/targets';
import { X, Loader2, Target, Zap, Globe } from 'lucide-react';
import clsx from 'clsx';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const MODULES = [
  { id: 'DISCOVERY', label: 'PHASE 1: DISCOVERY' },
  { id: 'PROBING', label: 'PHASE 2: PROBING' },
  { id: 'CRAWLING', label: 'PHASE 3: CRAWLING' },
];

export const CreateTargetModal = ({ isOpen, onClose }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<CreateTargetPayload>({
    name: '',
    root_domain: '',
    description: '',
    frequency: 720,
    modules: ['DISCOVERY', 'PROBING', 'CRAWLING'],
    use_alterx: true,
    use_waymore: false, // پیش‌فرض
  });

  const mutation = useMutation({
    mutationFn: createTarget,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['targets'] });
      onClose();
      setFormData({ name: '', root_domain: '', description: '', frequency: 720, modules: ['DISCOVERY', 'PROBING', 'CRAWLING'], use_alterx: true, use_waymore: false });
    },
  });

  if (!isOpen) return null;

  const toggleModule = (moduleId: string) => {
    setFormData(prev => {
      const exists = prev.modules.includes(moduleId);
      return { ...prev, modules: exists ? prev.modules.filter(m => m !== moduleId) : [...prev.modules, moduleId] };
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
      <div className="hack-box w-full max-w-md relative animate-in fade-in zoom-in duration-200">
        <div className="absolute top-0 left-0 w-full h-[1px] bg-gradient-to-r from-transparent via-hack-primary to-transparent opacity-50"></div>
        
        <div className="flex justify-between items-center p-4 border-b border-hack-border">
          <div className="flex items-center gap-2 text-hack-primary">
            <Target size={18} />
            <h2 className="font-mono font-bold tracking-widest text-lg">INIT_TARGET</h2>
          </div>
          <button onClick={onClose} className="text-hack-dim hover:text-hack-danger transition-colors"><X size={18} /></button>
        </div>

        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(formData); }} className="p-6 space-y-5">
          <div className="space-y-1">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Operation Codename</label>
            <input type="text" required className="hack-input w-full" placeholder="PROJECT_ALPHA" value={formData.name} onChange={e => setFormData({ ...formData, name: e.target.value })} />
          </div>

          <div className="space-y-1">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Target Root</label>
            <input type="text" required className="hack-input w-full" placeholder="target.com" value={formData.root_domain} onChange={e => setFormData({ ...formData, root_domain: e.target.value })} />
          </div>

          <div className="space-y-1">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Recon Frequency (Min)</label>
            <input type="number" min="0" className="hack-input w-full" value={formData.frequency} onChange={e => setFormData({ ...formData, frequency: parseInt(e.target.value) || 0 })} />
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="p-3 border border-hack-border bg-black/30 flex flex-col justify-center gap-2">
                <div className="flex items-center gap-2">
                    <input type="checkbox" id="use_alterx" checked={formData.use_alterx} onChange={e => setFormData({...formData, use_alterx: e.target.checked})} className="accent-hack-primary h-4 w-4" />
                    <label htmlFor="use_alterx" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1"><Zap size={12} className="text-hack-warning"/> ALTERX</label>
                </div>
                <p className="text-[8px] text-hack-dim leading-tight">Generate permutations.</p>
            </div>

            <div className="p-3 border border-hack-border bg-black/30 flex flex-col justify-center gap-2">
                <div className="flex items-center gap-2">
                    <input type="checkbox" id="use_waymore" checked={formData.use_waymore} onChange={e => setFormData({...formData, use_waymore: e.target.checked})} className="accent-hack-primary h-4 w-4" />
                    <label htmlFor="use_waymore" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1"><Globe size={12} className="text-blue-400"/> WAYMORE</label>
                </div>
                <p className="text-[8px] text-hack-dim leading-tight">Deep historical crawl.</p>
            </div>
          </div>

          <div className="space-y-2">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Execution Modules</label>
            <div className="space-y-1">
              {MODULES.map(mod => (
                <div key={mod.id} onClick={() => toggleModule(mod.id)} className={clsx("flex items-center gap-3 p-2 border cursor-pointer transition-all text-xs font-mono", formData.modules.includes(mod.id) ? "bg-hack-primary/10 border-hack-primary text-hack-primary" : "border-hack-border text-hack-dim hover:border-hack-dim")}>
                  <div className={clsx("w-3 h-3 border flex items-center justify-center", formData.modules.includes(mod.id) ? "border-hack-primary bg-hack-primary" : "border-hack-dim")} />
                  {mod.label}
                </div>
              ))}
            </div>
          </div>

          <div className="pt-2 flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 hack-btn-ghost border border-hack-border">Abort</button>
            <button type="submit" disabled={mutation.isPending} className="flex-1 hack-btn">
              {mutation.isPending ? <Loader2 size={16} className="animate-spin" /> : 'INITIALIZE'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};