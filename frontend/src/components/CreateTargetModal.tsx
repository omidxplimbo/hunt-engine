import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import clsx from 'clsx';
import {
  Database,
  FileText,
  Globe,
  Loader2,
  Network,
  Search,
  Shield,
  Target,
  X,
  Zap,
} from 'lucide-react';
import {
  createTarget,
  getWordlists,
  type CreateTargetPayload,
  type Wordlist,
} from '../api/targets';

interface Props {
  isOpen: boolean;
  onClose: () => void;
}

const MODULES = [
  { id: 'DISCOVERY', label: 'PHASE 1', description: 'Discovery' },
  { id: 'PROBING', label: 'PHASE 2', description: 'Probing' },
  { id: 'CRAWLING', label: 'PHASE 3', description: 'Crawling' },
];

type ToolKey =
  | 'use_alterx'
  | 'use_amass'
  | 'use_cero'
  | 'use_crtsh'
  | 'use_abusedb'
  | 'use_puredns'
  | 'use_waymore'
  | 'use_portscan';

type ToolConfig = {
  key: ToolKey;
  title: string;
  badge: string;
  description: string;
  Icon: typeof Zap;
};

const DISCOVERY_TOOLS: ToolConfig[] = [
  {
    key: 'use_alterx',
    title: 'ALTERX',
    badge: 'MUTATION',
    description: 'Generate permutation candidates before DNS validation.',
    Icon: Zap,
  },
  {
    key: 'use_amass',
    title: 'AMASS',
    badge: 'PASSIVE',
    description: 'Optional OWASP Amass passive enumeration with timeout control.',
    Icon: Search,
  },
  {
    key: 'use_cero',
    title: 'CERO',
    badge: 'CERTS',
    description: 'Extract subdomains from certificate transparency data.',
    Icon: Shield,
  },
  {
    key: 'use_crtsh',
    title: 'CRT.SH',
    badge: 'OSINT',
    description: 'Query crt.sh API for certificate-backed subdomains.',
    Icon: Globe,
  },
  {
    key: 'use_abusedb',
    title: 'ABUSEDB',
    badge: 'INTEL',
    description: 'Scrape suspicious hostnames from AbuseIPDB signals.',
    Icon: Shield,
  },
  {
    key: 'use_puredns',
    title: 'PUREDNS',
    badge: 'BRUTE',
    description: 'Bruteforce subdomain discovery and keep resolved results.',
    Icon: FileText,
  },
];

const EXTENDED_TOOLS: ToolConfig[] = [
  {
    key: 'use_waymore',
    title: 'WAYMORE',
    badge: 'CRAWL',
    description: 'Deep historical URL collection during Crawling.',
    Icon: Database,
  },
  {
    key: 'use_portscan',
    title: 'PORTSCAN',
    badge: 'NMAP',
    description: 'Run Nmap-based port discovery for live assets.',
    Icon: Network,
  },
];

const defaultFormData: CreateTargetPayload = {
  name: '',
  root_domain: '',
  description: '',
  frequency: 720,
  modules: ['DISCOVERY', 'PROBING', 'CRAWLING'],
  use_alterx: true,
  use_waymore: false,
  use_portscan: false,
  use_cero: false,
  use_crtsh: false,
  use_puredns: false,
  use_abusedb: false,
  use_amass: false,
  puredns_wordlists: [],
};

export const CreateTargetModal = ({ isOpen, onClose }: Props) => {
  const queryClient = useQueryClient();
  const [formData, setFormData] = useState<CreateTargetPayload>(defaultFormData);

  const { data: wordlists = [] } = useQuery({
    queryKey: ['wordlists'],
    queryFn: getWordlists,
  });

  const mutation = useMutation({
    mutationFn: createTarget,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['targets'] });
      setFormData(defaultFormData);
      onClose();
    },
  });

  if (!isOpen) return null;

  const toggleModule = (moduleId: string) => {
    setFormData((prev) => {
      const exists = prev.modules.includes(moduleId);
      return {
        ...prev,
        modules: exists
          ? prev.modules.filter((module) => module !== moduleId)
          : [...prev.modules, moduleId],
      };
    });
  };

  const toggleTool = (key: ToolKey, checked: boolean) => {
    setFormData((prev) => ({
      ...prev,
      [key]: checked,
      puredns_wordlists:
        key === 'use_puredns' && !checked ? [] : prev.puredns_wordlists,
    }));
  };

  const renderToolCard = (tool: ToolConfig) => {
    const enabled = Boolean(formData[tool.key]);
    const Icon = tool.Icon;

    return (
      <button
        key={tool.key}
        type="button"
        onClick={() => toggleTool(tool.key, !enabled)}
        className={clsx(
          'group relative flex min-h-[112px] items-start gap-3 border p-4 text-left transition-all',
          enabled
            ? 'border-hack-primary bg-hack-primary/10 shadow-[0_0_18px_rgba(0,255,65,0.12)]'
            : 'border-hack-border bg-black/40 hover:border-hack-primary/50 hover:bg-hack-primary/5',
        )}
      >
        <span
          className={clsx(
            'mt-1 flex h-5 w-5 shrink-0 items-center justify-center border',
            enabled ? 'border-hack-primary bg-hack-primary text-black' : 'border-hack-dim bg-black',
          )}
        >
          {enabled && <span className="h-2 w-2 bg-black" />}
        </span>

        <div className="min-w-0 flex-1">
          <div className="mb-2 flex flex-wrap items-center gap-2">
            <span className="flex h-8 w-8 items-center justify-center border border-hack-border text-hack-dim group-hover:text-hack-primary">
              <Icon size={16} />
            </span>
            <span className="font-mono text-sm font-bold uppercase tracking-wider text-white">
              {tool.title}
            </span>
            <span className="border border-hack-border px-2 py-0.5 font-mono text-[10px] uppercase tracking-widest text-hack-dim">
              {tool.badge}
            </span>
          </div>
          <p className="font-mono text-xs leading-5 text-hack-dim">{tool.description}</p>
        </div>

        <span
          className={clsx(
            'absolute right-3 top-3 h-2 w-2 rounded-full',
            enabled ? 'bg-hack-primary shadow-[0_0_8px_rgba(0,255,65,0.9)]' : 'bg-hack-border',
          )}
        />
      </button>
    );
  };

  return (
    <div className="fixed top-0 bottom-0 left-0 lg:left-[320px] right-0 z-[9999] flex items-center justify-center bg-black/75 backdrop-blur-sm p-4">
      <div className="flex max-h-[90vh] w-full max-w-[980px] flex-col overflow-hidden border border-hack-primary bg-hack-bg shadow-[0_0_35px_rgba(0,255,65,0.18)]">
        <div className="flex shrink-0 items-start justify-between border-b border-hack-border bg-black/70 px-5 py-4">
          <div>
            <h2 className="font-mono text-xl font-bold uppercase tracking-widest text-hack-primary">
              INIT_TARGET
            </h2>
            <p className="mt-1 font-mono text-xs text-hack-dim">
              Configure scan modules and optional Discovery tooling.
            </p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="text-hack-dim transition-colors hover:text-white"
          >
            <X size={20} />
          </button>
        </div>

        <form
          onSubmit={(event) => {
            event.preventDefault();
            mutation.mutate(formData);
          }}
          className="min-h-0 flex-1 space-y-5 overflow-y-auto px-5 py-4"
        >
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Operation Codename
              </span>
              <input
                required
                value={formData.name}
                onChange={(event) => setFormData({ ...formData, name: event.target.value })}
                className="w-full border border-hack-border bg-black px-3 py-3 font-mono text-sm text-white outline-none focus:border-hack-primary"
                placeholder="t-mobile"
              />
            </label>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Target Root
              </span>
              <input
                required
                value={formData.root_domain}
                onChange={(event) => setFormData({ ...formData, root_domain: event.target.value })}
                className="w-full border border-hack-border bg-black px-3 py-3 font-mono text-sm text-white outline-none focus:border-hack-primary"
                placeholder="example.com"
              />
            </label>
          </div>

          <div className="grid grid-cols-1 gap-3 md:grid-cols-[1fr_220px]">
            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Description
              </span>
              <input
                value={formData.description}
                onChange={(event) => setFormData({ ...formData, description: event.target.value })}
                className="w-full border border-hack-border bg-black px-3 py-3 font-mono text-sm text-white outline-none focus:border-hack-primary"
                placeholder="optional notes"
              />
            </label>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Frequency Min
              </span>
              <input
                type="number"
                min={0}
                value={formData.frequency}
                onChange={(event) =>
                  setFormData({ ...formData, frequency: parseInt(event.target.value, 10) || 0 })
                }
                className="w-full border border-hack-border bg-black px-3 py-3 font-mono text-sm text-white outline-none focus:border-hack-primary"
              />
            </label>
          </div>

          <section>
            <div className="mb-3 flex items-center gap-2">
              <Target size={15} className="text-hack-primary" />
              <h3 className="font-mono text-sm font-bold uppercase tracking-widest text-hack-primary">
                Discovery Tooling
              </h3>
            </div>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              {DISCOVERY_TOOLS.map(renderToolCard)}
            </div>

            {formData.use_puredns && (
              <div className="mt-3 border border-hack-border bg-black/50 p-4">
                <div className="mb-3 font-mono text-xs font-bold uppercase tracking-widest text-hack-primary">
                  PureDNS Wordlists
                </div>
                <div className="grid max-h-40 grid-cols-1 gap-2 overflow-y-auto pr-1 md:grid-cols-2">
                  {wordlists.map((wordlist: Wordlist) => {
                    const selected = formData.puredns_wordlists.includes(wordlist.path);
                    return (
                      <label
                        key={wordlist.path}
                        className={clsx(
                          'flex cursor-pointer items-center gap-2 border px-3 py-2 font-mono text-xs transition-colors',
                          selected
                            ? 'border-hack-primary bg-hack-primary/10 text-hack-primary'
                            : 'border-hack-border text-hack-dim hover:border-hack-primary/50',
                        )}
                      >
                        <input
                          type="checkbox"
                          checked={selected}
                          onChange={(event) => {
                            const current = formData.puredns_wordlists;
                            setFormData({
                              ...formData,
                              puredns_wordlists: event.target.checked
                                ? [...current, wordlist.path]
                                : current.filter((path) => path !== wordlist.path),
                            });
                          }}
                          className="accent-hack-primary"
                        />
                        <span className="truncate">[{wordlist.type}] {wordlist.name}</span>
                      </label>
                    );
                  })}
                  {wordlists.length === 0 && (
                    <div className="font-mono text-xs text-hack-dim">
                      No wordlists available. Add wordlists to /wordlists/custom.
                    </div>
                  )}
                </div>
              </div>
            )}
          </section>

          <section>
            <div className="mb-3 font-mono text-sm font-bold uppercase tracking-widest text-hack-primary">
              Extended Modules
            </div>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              {EXTENDED_TOOLS.map(renderToolCard)}
            </div>
          </section>

          <section>
            <div className="mb-3 font-mono text-sm font-bold uppercase tracking-widest text-hack-primary">
              Execution Modules
            </div>
            <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
              {MODULES.map((module) => {
                const enabled = formData.modules.includes(module.id);
                return (
                  <button
                    key={module.id}
                    type="button"
                    onClick={() => toggleModule(module.id)}
                    className={clsx(
                      'border p-3 text-left font-mono transition-all',
                      enabled
                        ? 'border-hack-primary bg-hack-primary/10 text-hack-primary'
                        : 'border-hack-border text-hack-dim hover:border-hack-primary/50',
                    )}
                  >
                    <div className="text-xs font-bold uppercase tracking-widest">{module.label}</div>
                    <div className="mt-1 text-[11px] uppercase tracking-widest">{module.description}</div>
                  </button>
                );
              })}
            </div>
          </section>

          <div className="sticky bottom-0 -mx-5 flex justify-end gap-3 border-t border-hack-border bg-hack-bg/95 px-5 py-4 backdrop-blur">
            <button
              type="button"
              onClick={onClose}
              className="border border-hack-border px-6 py-2 font-mono text-xs font-bold uppercase tracking-widest text-hack-dim hover:border-white hover:text-white"
            >
              Abort
            </button>
            <button
              type="submit"
              disabled={mutation.isPending}
              className="flex items-center gap-2 border border-hack-primary px-6 py-2 font-mono text-xs font-bold uppercase tracking-widest text-hack-primary hover:bg-hack-primary hover:text-black disabled:opacity-60"
            >
              {mutation.isPending && <Loader2 size={14} className="animate-spin" />}
              {mutation.isPending ? 'Initializing' : 'Initialize'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
