import { useState } from 'react';
import { useMutation, useQueryClient, useQuery } from '@tanstack/react-query';
import { createTarget, type CreateTargetPayload, getWordlists, type Wordlist } from '../api/targets';
import { X, Loader2, Target, Zap, Globe, Network, FileText, Shield } from 'lucide-react';
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
    use_portscan: false, // پیش‌فرض
    use_cero: false, // پیش‌فرض
    use_crtsh: false, // پیش‌فرض
    use_puredns: false, // پیش‌فرض
    use_abusedb: false,
    puredns_wordlists: [], // پیش‌فرض
  });

  const { data: wordlists = [] } = useQuery({
    queryKey: ['wordlists'],
    queryFn: getWordlists,
  });

  const mutation = useMutation({
    mutationFn: createTarget,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['targets'] });
      onClose();
      setFormData({ name: '', root_domain: '', description: '', frequency: 720, modules: ['DISCOVERY', 'PROBING', 'CRAWLING'], use_alterx: true, use_waymore: false, use_portscan: false, use_cero: false, use_crtsh: false, use_puredns: false, use_abusedb: false, puredns_wordlists: [] });
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
    <div className="fixed inset-0 z-50 flex items-start justify-center p-4 bg-black/80 backdrop-blur-sm overflow-y-auto">
      <div className="hack-box w-full max-w-md relative animate-in fade-in zoom-in duration-200 flex flex-col max-h-[90vh] min-h-0 my-6">
        <div className="absolute top-0 left-0 w-full h-[1px] bg-gradient-to-r from-transparent via-hack-primary to-transparent opacity-50"></div>
        
        <div className="flex justify-between items-center p-4 border-b border-hack-border">
          <div className="flex items-center gap-2 text-hack-primary">
            <Target size={18} />
            <h2 className="font-mono font-bold tracking-widest text-lg">INIT_TARGET</h2>
          </div>
          <button onClick={onClose} className="text-hack-dim hover:text-hack-danger transition-colors"><X size={18} /></button>
        </div>

        <form onSubmit={(e) => { e.preventDefault(); mutation.mutate(formData); }} className="p-6 space-y-5 overflow-y-auto flex-1 min-h-0">
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

          <div className="grid grid-cols-2 gap-3">
            <div className="p-3 border border-hack-border bg-black/30 flex flex-col justify-center gap-2">
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="use_portscan"
                  checked={formData.use_portscan}
                  onChange={e => setFormData({ ...formData, use_portscan: e.target.checked })}
                  className="accent-hack-primary h-4 w-4"
                />
                <label htmlFor="use_portscan" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                  <Network size={12} className="text-hack-primary" /> PORTSCAN
                </label>
              </div>
              <p className="text-[8px] text-hack-dim leading-tight">Scan open ports (NMAP).</p>
            </div>

            <div className="p-3 border border-hack-border bg-black/30 flex flex-col justify-center gap-2">
              <div className="flex items-center gap-2">
                <input
                  type="checkbox"
                  id="use_cero"
                  checked={formData.use_cero || false}
                  onChange={e => setFormData({ ...formData, use_cero: e.target.checked })}
                  className="accent-hack-primary h-4 w-4"
                />
                <label htmlFor="use_cero" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                  <Network size={12} className="text-green-400" /> CERO
                </label>
              </div>
              <p className="text-[8px] text-hack-dim leading-tight">Scrape SSL certificates.</p>
            </div>
          </div>

          <div className="p-3 border border-hack-border bg-black/30 flex flex-col justify-center gap-2">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="use_crtsh"
                checked={formData.use_crtsh || false}
                onChange={e => setFormData({ ...formData, use_crtsh: e.target.checked })}
                className="accent-hack-primary h-4 w-4"
              />
              <label htmlFor="use_crtsh" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                <Globe size={12} className="text-purple-400" /> CRT.SH
              </label>
            </div>
            <p className="text-[8px] text-hack-dim leading-tight">Query crt.sh API for subdomains.</p>
          </div>

          <div className="p-3 border border-hack-border bg-black/30 flex flex-col justify-center gap-2">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="use_abusedb"
                checked={formData.use_abusedb || false}
                onChange={e => setFormData({ ...formData, use_abusedb: e.target.checked })}
                className="accent-hack-primary h-4 w-4"
              />
              <label htmlFor="use_abusedb" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                <Shield size={12} className="text-red-400" /> AbuseDB
              </label>
            </div>
            <p className="text-[8px] text-hack-dim leading-tight">Scrape suspicious hostnames from AbuseIPDB.</p>
          </div>

          <div className="p-3 border border-hack-border bg-black/30 flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                id="use_puredns"
                checked={formData.use_puredns || false}
                onChange={e => setFormData({ ...formData, use_puredns: e.target.checked, puredns_wordlists: e.target.checked ? formData.puredns_wordlists : [] })}
                className="accent-hack-primary h-4 w-4"
              />
              <label htmlFor="use_puredns" className="text-xs font-bold text-hack-text tracking-wide cursor-pointer flex items-center gap-1">
                <FileText size={12} className="text-orange-400" /> PUREDNS
              </label>
            </div>
            <p className="text-[8px] text-hack-dim leading-tight">Bruteforce subdomain discovery (only live subdomains).</p>
            
            {formData.use_puredns && (
              <div className="mt-2 space-y-2 max-h-40 overflow-y-auto">
                <label className="text-[9px] uppercase text-hack-dim tracking-widest">Select Wordlists:</label>
                <div className="space-y-1">
                  {wordlists.map((wl: Wordlist) => (
                    <div key={wl.path} className="flex items-center gap-2">
                      <input
                        type="checkbox"
                        id={`wl-${wl.path}`}
                        checked={formData.puredns_wordlists?.includes(wl.path) || false}
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
                      <label htmlFor={`wl-${wl.path}`} className="text-[9px] text-hack-text cursor-pointer flex items-center gap-1">
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
