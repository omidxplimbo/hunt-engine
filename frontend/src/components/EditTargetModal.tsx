import { useState, useEffect } from 'react';
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query';
import { updateTarget, type UpdateTargetPayload, getWordlists, type Wordlist } from '../api/targets';
import type { Target } from '../types/target';
import { X, Loader2, Settings, Zap, Globe, Network, FileText } from 'lucide-react';
import clsx from 'clsx';

interface Props {
  isOpen: boolean;
  onClose: () => void;
  target: Target | null;
}

const MODULES = [
  { id: 'DISCOVERY', label: 'DISCOVERY' },
  { id: 'PROBING', label: 'PROBING' },
  { id: 'CRAWLING', label: 'CRAWLING' },
];

export const EditTargetModal = ({ isOpen, onClose, target }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<UpdateTargetPayload>({});

  useEffect(() => {
    if (target) {
      let existingModules: string[] = ['DISCOVERY', 'PROBING', 'CRAWLING'];
      const t = target as any;
      if (t.scan_modules) {
          try {
              const parsed = typeof t.scan_modules === 'string' ? JSON.parse(t.scan_modules) : t.scan_modules;
              if (Array.isArray(parsed)) existingModules = parsed;
          } catch (e) {}
      }
      setFormData({
        name: target.name,
        description: target.description,
        frequency: target.frequency,
        in_scope: target.in_scope,
        use_alterx: target.use_alterx,
        use_waymore: target.use_waymore, // 👈 مقداردهی اولیه
        use_portscan: (target as any).use_portscan ?? false,
        use_cero: (target as any).use_cero ?? false,
        use_crtsh: (target as any).use_crtsh ?? false,
        use_puredns: (target as any).use_puredns ?? false,
        puredns_wordlists: (target as any).puredns_wordlists || [],
        modules: existingModules,
      });
    }
  }, [target]);

  const { data: wordlists = [] } = useQuery({
    queryKey: ['wordlists'],
    queryFn: getWordlists,
  });

  const mutation = useMutation({
    mutationFn: (data: UpdateTargetPayload) => updateTarget(target!.id, data),
    onSuccess: () => { queryClient.invalidateQueries({ queryKey: ['targets'] }); onClose(); },
  });

  if (!isOpen || !target) return null;

  const toggleModule = (moduleId: string) => {
    setFormData(prev => {
      const currentModules = prev.modules || [];
      const exists = currentModules.includes(moduleId);
      return { ...prev, modules: exists ? currentModules.filter(m => m !== moduleId) : [...currentModules, moduleId] };
    });
  };

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center p-4 bg-black/80 backdrop-blur-sm overflow-y-auto">
      <div className="hack-box w-full max-w-md relative animate-in fade-in zoom-in duration-200 flex flex-col max-h-[90vh] min-h-0 my-6">
        <div className="flex justify-between items-center p-4 border-b border-hack-border">
          <div className="flex items-center gap-2 text-hack-primary">
            <Settings size={18} />
            <h2 className="font-mono font-bold tracking-widest text-lg">CONFIG_TARGET</h2>
          </div>
          <button onClick={onClose} className="text-hack-dim hover:text-hack-danger transition-colors"><X size={18} /></button>
        </div>

        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(formData); }} className="p-6 space-y-5 overflow-y-auto flex-1 min-h-0">
          <div className="space-y-1">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Name</label>
            <input type="text" className="hack-input w-full" value={formData.name || ''} onChange={e => setFormData({ ...formData, name: e.target.value })} />
          </div>

          <div className="space-y-1">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Freq (Min)</label>
            <input type="number" className="hack-input w-full" value={formData.frequency || 0} onChange={e => setFormData({ ...formData, frequency: parseInt(e.target.value) })} />
          </div>

          <div className="space-y-2">
            <label className="text-[10px] uppercase text-hack-dim tracking-widest">Active Modules</label>
            <div className="flex gap-2">
                {MODULES.map(mod => (
                    <div key={mod.id} onClick={() => toggleModule(mod.id)} className={clsx("cursor-pointer px-3 py-1.5 border transition-colors select-none text-[10px] font-bold tracking-wider", (formData.modules || []).includes(mod.id) ? "bg-hack-primary/10 border-hack-primary text-hack-primary" : "border-hack-border text-hack-dim")}>
                        {mod.label}
                    </div>
                ))}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="p-3 border border-hack-border bg-black/30 flex items-center gap-2">
                <input type="checkbox" checked={formData.use_alterx ?? true} onChange={e => setFormData({...formData, use_alterx: e.target.checked})} className="accent-hack-primary h-4 w-4" />
                <label className="block text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1"><Zap size={12} className="text-hack-warning"/> ALTERX</label>
            </div>

            <div className="p-3 border border-hack-border bg-black/30 flex items-center gap-2">
                <input type="checkbox" checked={formData.use_waymore ?? false} onChange={e => setFormData({...formData, use_waymore: e.target.checked})} className="accent-hack-primary h-4 w-4" />
                <label className="block text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1"><Globe size={12} className="text-blue-400"/> WAYMORE</label>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3">
            <div className="p-3 border border-hack-border bg-black/30 flex items-center gap-2">
              <input
                type="checkbox"
                checked={formData.use_portscan ?? false}
                onChange={e => setFormData({ ...formData, use_portscan: e.target.checked })}
                className="accent-hack-primary h-4 w-4"
              />
              <label className="block text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                <Network size={12} className="text-hack-primary" /> PORTSCAN
              </label>
            </div>

            <div className="p-3 border border-hack-border bg-black/30 flex items-center gap-2">
              <input
                type="checkbox"
                checked={formData.use_cero ?? false}
                onChange={e => setFormData({ ...formData, use_cero: e.target.checked })}
                className="accent-hack-primary h-4 w-4"
              />
              <label className="block text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                <Network size={12} className="text-green-400" /> CERO
              </label>
            </div>
          </div>

          <div className="p-3 border border-hack-border bg-black/30 flex items-center gap-2">
            <input
              type="checkbox"
              checked={formData.use_crtsh ?? false}
              onChange={e => setFormData({ ...formData, use_crtsh: e.target.checked })}
              className="accent-hack-primary h-4 w-4"
            />
            <label className="block text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
              <Globe size={12} className="text-purple-400" /> CRT.SH
            </label>
          </div>

          <div className="p-3 border border-hack-border bg-black/30 flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={formData.use_puredns ?? false}
                onChange={e => setFormData({ ...formData, use_puredns: e.target.checked, puredns_wordlists: e.target.checked ? (formData.puredns_wordlists || []) : [] })}
                className="accent-hack-primary h-4 w-4"
              />
              <label className="block text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                <FileText size={12} className="text-orange-400" /> PUREDNS
              </label>
            </div>
            
            {formData.use_puredns && (
              <div className="mt-2 space-y-2 max-h-40 overflow-y-auto">
                <label className="text-[9px] uppercase text-hack-dim tracking-widest">Select Wordlists:</label>
                <div className="space-y-1">
                  {wordlists.map((wl: Wordlist) => (
                    <div key={wl.path} className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id={`wl-edit-${wl.path}`}
                        checked={(formData.puredns_wordlists || []).includes(wl.path)}
                        onChange={(e) => {
                          const current = formData.puredns_wordlists || [];
                          if (e.target.checked) {
                            setFormData({ ...formData, puredns_wordlists: [...current, wl.path] });
                          } else {
                            setFormData({ ...formData, puredns_wordlists: current.filter(p => p !== wl.path) });
                          }
                        }}
                        className="accent-hack-primary h-3 w-3"
                      />
                      <label htmlFor={`wl-edit-${wl.path}`} className="text-[9px] text-hack-text cursor-pointer flex items-center gap-1">
                        <span className={wl.type === 'custom' ? 'text-orange-400' : 'text-blue-400'}>[{wl.type}]</span>
                        {wl.name}
                      </label>
                    </div>
                  ))}
                  {wordlists.length === 0 && (
                    <p className="text-[8px] text-hack-dim">No wordlists available. Add wordlists to /wordlists/custom directory.</p>
                  )}
                </div>
              </div>
            )}
          </div>

          <div className="flex items-center gap-3">
             <input type="checkbox" checked={formData.in_scope ?? true} onChange={e => setFormData({...formData, in_scope: e.target.checked})} className="accent-hack-primary h-4 w-4" />
             <label className="text-xs text-hack-text tracking-wide uppercase">Active Scope</label>
          </div>

          <div className="pt-2 flex gap-3">
            <button type="button" onClick={onClose} className="flex-1 hack-btn-ghost border border-hack-border">Discard</button>
            <button type="submit" disabled={mutation.isPending} className="flex-1 hack-btn">
              {mutation.isPending ? <Loader2 size={16} className="animate-spin" /> : 'UPDATE'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};