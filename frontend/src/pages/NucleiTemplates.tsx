import { useMemo, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import {
  AlertTriangle,
  Bot,
  CheckCircle2,
  Clipboard,
  FileCode2,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  Search,
  ShieldCheck,
  Sparkles,
  Trash2,
  Wand2,
} from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import {
  NUCLEI_TEMPLATE_PLACEMENTS,
  deleteNucleiTemplate,
  generateNucleiTemplateDraft,
  getNucleiTemplate,
  getNucleiTemplateDraftStatus,
  getNucleiTemplateStrategy,
  listNucleiTemplates,
  saveNucleiTemplate,
  validateNucleiTemplate,
  type NucleiTemplate,
  type NucleiTemplateDraft,
  type NucleiTemplateDraftRequest,
  type NucleiTemplatePlacement,
  type NucleiTemplateStrategy,
  type NucleiTemplateValidation,
} from '../api/nucleiTemplates';

const defaultTemplate = `id: hunt-custom-marker

info:
  name: Hunt Custom Marker
  author: hunt-engine
  severity: info
  description: Custom Nuclei template managed by Hunt Engine.
  tags: exposure,panel

http:
  - method: GET
    path:
      - "{{BaseURL}}/"

    matchers:
      - type: word
        part: body
        words:
          - "HUNT_NUCLEI_TEST_2026_05_19"
`;

const placementDetails: Record<
  NucleiTemplatePlacement,
  { label: string; path: string; profiles: string; tone: string }
> = {
  root: {
    label: 'Root / legacy',
    path: '/data/nuclei/custom/*.yaml',
    profiles: 'All profiles',
    tone: 'border-hack-border text-hack-dim',
  },
  shared: {
    label: 'Shared',
    path: '/data/nuclei/custom/shared',
    profiles: 'safe, fast, balanced, full',
    tone: 'border-hack-primary/40 text-hack-primary',
  },
  safe: {
    label: 'Safe',
    path: '/data/nuclei/custom/safe',
    profiles: 'safe, fast, balanced, full',
    tone: 'border-hack-primary/40 text-hack-primary',
  },
  fast: {
    label: 'Fast',
    path: '/data/nuclei/custom/fast',
    profiles: 'fast, balanced, full',
    tone: 'border-cyan-400/40 text-cyan-300',
  },
  exposure: {
    label: 'Exposure',
    path: '/data/nuclei/custom/exposure',
    profiles: 'fast, balanced, full',
    tone: 'border-cyan-400/40 text-cyan-300',
  },
  balanced: {
    label: 'Balanced',
    path: '/data/nuclei/custom/balanced',
    profiles: 'balanced, full',
    tone: 'border-amber-400/40 text-amber-300',
  },
  misconfig: {
    label: 'Misconfig',
    path: '/data/nuclei/custom/misconfig',
    profiles: 'balanced, full',
    tone: 'border-amber-400/40 text-amber-300',
  },
  cves: {
    label: 'CVEs',
    path: '/data/nuclei/custom/cves',
    profiles: 'cves-light, full',
    tone: 'border-orange-400/40 text-orange-300',
  },
  'cves-light': {
    label: 'CVEs light',
    path: '/data/nuclei/custom/cves-light',
    profiles: 'cves-light, full',
    tone: 'border-orange-400/40 text-orange-300',
  },
  full: {
    label: 'Full only',
    path: '/data/nuclei/custom/full',
    profiles: 'full only',
    tone: 'border-red-400/40 text-red-300',
  },
  custom: {
    label: 'Agent custom',
    path: '/data/nuclei/custom/custom',
    profiles: 'custom, full',
    tone: 'border-purple-400/40 text-purple-300',
  },
};

const defaultDraftRequest: NucleiTemplateDraftRequest = {
  name: 'hunt-agent-draft.yaml',
  title: 'Hunt Agent Draft',
  description: 'Draft-only template proposed by Hunt Engine for human review.',
  severity: 'info',
  tags: ['exposure', 'panel'],
  method: 'GET',
  path: '/',
  matcher_type: 'word',
  matcher_part: 'body',
  matcher_value: 'HUNT_NUCLEI_TEST_2026_05_19',
  validate: true,
};

const formatBytes = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
};

const formatDate = (value: string) => {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '-';
  return parsed.toLocaleString();
};

const getErrorMessage = (error: unknown) => {
  const maybeError = error as {
    response?: { data?: { message?: string; error?: string } };
    message?: string;
  };
  return (
    maybeError.response?.data?.message ||
    maybeError.response?.data?.error ||
    maybeError.message ||
    'Operation failed'
  );
};

const placementFromPath = (template: NucleiTemplate): NucleiTemplatePlacement => {
  if (template.placement && NUCLEI_TEMPLATE_PLACEMENTS.includes(template.placement)) {
    return template.placement;
  }

  const path = template.path || '';
  for (const placement of NUCLEI_TEMPLATE_PLACEMENTS) {
    if (placement === 'root') continue;
    if (path.includes(`/custom/${placement}/`) || path.includes(`\\custom\\${placement}\\`)) {
      return placement;
    }
  }

  return 'root';
};

const validationBadgeClass = (validation: NucleiTemplateValidation | null) => {
  if (!validation) return 'border-hack-border text-hack-dim bg-white/5';
  return validation.valid
    ? 'border-hack-primary/50 text-hack-primary bg-hack-primary/10'
    : 'border-hack-danger/50 text-hack-danger bg-hack-danger/10';
};

const asList = (value?: string[]) => (value && value.length > 0 ? value : []);

const NucleiTemplates = () => {
  const { role } = useAuth();
  const queryClient = useQueryClient();
  const [selectedName, setSelectedName] = useState('');
  const [selectedPlacement, setSelectedPlacement] = useState<NucleiTemplatePlacement>('fast');
  const [templateName, setTemplateName] = useState('hunt-custom-marker.yaml');
  const [content, setContent] = useState(defaultTemplate);
  const [validation, setValidation] = useState<NucleiTemplateValidation | null>(null);
  const [search, setSearch] = useState('');
  const [placementFilter, setPlacementFilter] = useState<'all' | NucleiTemplatePlacement>('all');
  const [draftPlacement, setDraftPlacement] = useState<NucleiTemplatePlacement>('fast');
  const [draftRequest, setDraftRequest] = useState<NucleiTemplateDraftRequest>(defaultDraftRequest);
  const [generatedDraft, setGeneratedDraft] = useState<NucleiTemplateDraft | null>(null);
  const [strategyTargetId, setStrategyTargetId] = useState('');
  const [includeDraft, setIncludeDraft] = useState(false);
  const [strategy, setStrategy] = useState<NucleiTemplateStrategy | null>(null);

  const templatesQuery = useQuery({
    queryKey: ['nuclei-templates'],
    queryFn: listNucleiTemplates,
  });

  const draftStatusQuery = useQuery({
    queryKey: ['nuclei-template-draft-status'],
    queryFn: getNucleiTemplateDraftStatus,
  });

  const templates = useMemo(() => templatesQuery.data || [], [templatesQuery.data]);

  const filteredTemplates = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase();
    return templates.filter((template) => {
      const placement = placementFromPath(template);
      const matchesPlacement = placementFilter === 'all' || placement === placementFilter;
      const matchesSearch =
        !normalizedSearch ||
        template.name.toLowerCase().includes(normalizedSearch) ||
        (template.path || '').toLowerCase().includes(normalizedSearch);
      return matchesPlacement && matchesSearch;
    });
  }, [templates, placementFilter, search]);

  const loadMutation = useMutation({
    mutationFn: ({ name, placement }: { name: string; placement: NucleiTemplatePlacement }) =>
      getNucleiTemplate(name, placement),
    onSuccess: (template) => {
      const placement = placementFromPath(template);
      setSelectedName(template.name);
      setSelectedPlacement(placement);
      setTemplateName(template.name);
      setContent(template.content || '');
      setValidation(null);
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const validateMutation = useMutation({
    mutationFn: () => validateNucleiTemplate({ name: templateName, content, placement: selectedPlacement }),
    onSuccess: (result) => {
      setValidation(result);
      if (result.valid) {
        toast.success('Template validation passed');
      } else {
        toast.error(result.error || 'Template validation failed');
      }
    },
  });

  const saveMutation = useMutation({
    mutationFn: () =>
      saveNucleiTemplate({
        name: templateName,
        content,
        placement: selectedPlacement,
        validate: true,
      }),
    onSuccess: (template) => {
      const placement = placementFromPath(template);
      toast.success('Template saved');
      setSelectedName(template.name);
      setSelectedPlacement(placement);
      setTemplateName(template.name);
      setContent(template.content || content);
      setValidation({ valid: true, name: template.name });
      queryClient.invalidateQueries({ queryKey: ['nuclei-templates'] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: ({ name, placement }: { name: string; placement: NucleiTemplatePlacement }) =>
      deleteNucleiTemplate(name, placement),
    onSuccess: () => {
      toast.success('Template deleted');
      startNewTemplate();
      queryClient.invalidateQueries({ queryKey: ['nuclei-templates'] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const draftMutation = useMutation({
    mutationFn: () => generateNucleiTemplateDraft(draftRequest),
    onSuccess: (draft) => {
      setGeneratedDraft(draft);
      toast.success('Draft generated for human review');
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const strategyMutation = useMutation({
    mutationFn: () =>
      getNucleiTemplateStrategy(strategyTargetId.trim(), {
        includeDraft,
        validate: includeDraft,
      }),
    onSuccess: (result) => {
      setStrategy(result);
      if (result.generated_draft) {
        setGeneratedDraft(result.generated_draft);
      }
      toast.success('Strategy generated');
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  if (role !== 'admin') return <Navigate to="/" replace />;

  const draftStatus = draftStatusQuery.data;
  const aiEnabled = !!draftStatus?.enabled;
  const isBusy =
    templatesQuery.isLoading ||
    loadMutation.isPending ||
    validateMutation.isPending ||
    saveMutation.isPending ||
    deleteMutation.isPending ||
    draftMutation.isPending ||
    strategyMutation.isPending;

  function startNewTemplate() {
    setSelectedName('');
    setSelectedPlacement('fast');
    setTemplateName('hunt-custom-marker.yaml');
    setContent(defaultTemplate);
    setValidation(null);
  }

  const handleDelete = () => {
    if (!selectedName) return;
    if (!confirm(`Delete Nuclei template "${selectedName}" from ${selectedPlacement}?`)) return;
    deleteMutation.mutate({ name: selectedName, placement: selectedPlacement });
  };

  const applyDraftToEditor = (draft: NucleiTemplateDraft, placement = draftPlacement) => {
    setSelectedName('');
    setSelectedPlacement(placement);
    setTemplateName(draft.name || draftRequest.name);
    setContent(draft.content || '');
    setValidation(draft.validation || null);
    toast.success('Draft loaded into editor. Validate and save manually.');
  };

  const copyDraft = async (draft: NucleiTemplateDraft) => {
    await navigator.clipboard.writeText(draft.content || '');
    toast.success('Draft copied');
  };

  const updateDraftRequest = <K extends keyof NucleiTemplateDraftRequest>(
    key: K,
    value: NucleiTemplateDraftRequest[K]
  ) => {
    setDraftRequest((current) => ({ ...current, [key]: value }));
  };

  const selectedPlacementInfo = placementDetails[selectedPlacement];

  return (
    <div className="space-y-6">
      <header className="rounded-2xl border border-hack-border bg-black/30 p-6 shadow-[0_0_30px_rgba(0,255,65,0.05)]">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
          <div>
            <div className="mb-2 flex items-center gap-2 text-xs uppercase tracking-[0.35em] text-hack-dim">
              <ShieldCheck size={16} className="text-hack-primary" />
              Nuclei Security Engine
            </div>
            <h1 className="text-2xl font-bold text-hack-text">Template Management</h1>
            <p className="mt-2 max-w-3xl text-sm text-hack-dim">
              Manage custom Nuclei YAML templates, assign profile placement, validate before execution,
              and prepare AI-assisted draft workflows behind a human approval gate.
            </p>
          </div>
          <div className="grid grid-cols-2 gap-3 text-xs sm:grid-cols-4">
            <Metric label="Templates" value={templates.length} />
            <Metric label="Visible" value={filteredTemplates.length} />
            <Metric label="AI drafts" value={aiEnabled ? 'Enabled' : 'Off'} tone={aiEnabled ? 'ok' : 'muted'} />
            <Metric label="Auto-save" value="Never" tone="safe" />
          </div>
        </div>
      </header>

      <div className="grid gap-6 xl:grid-cols-[380px_minmax(0,1fr)]">
        <section className="rounded-2xl border border-hack-border bg-black/25 p-4">
          <div className="mb-4 flex items-center justify-between gap-3">
            <div>
              <h2 className="font-mono text-sm uppercase tracking-wider text-hack-text">Template library</h2>
              <p className="text-xs text-hack-dim">Profile-aware custom templates</p>
            </div>
            <button
              type="button"
              onClick={() => templatesQuery.refetch()}
              className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
              disabled={isBusy}
            >
              <RefreshCw size={14} className={templatesQuery.isFetching ? 'animate-spin' : ''} />
              Refresh
            </button>
          </div>

          <div className="space-y-3">
            <label className="relative block">
              <Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2 text-hack-dim" />
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                className="hack-input w-full pl-9 text-sm"
                placeholder="Search templates"
              />
            </label>

            <select
              value={placementFilter}
              onChange={(event) => setPlacementFilter(event.target.value as 'all' | NucleiTemplatePlacement)}
              className="hack-input w-full text-sm"
            >
              <option value="all">All placements</option>
              {NUCLEI_TEMPLATE_PLACEMENTS.map((placement) => (
                <option key={placement} value={placement}>
                  {placementDetails[placement].label}
                </option>
              ))}
            </select>
          </div>

          <div className="mt-4 max-h-[640px] space-y-2 overflow-y-auto pr-1">
            {templatesQuery.isLoading ? (
              <EmptyState icon={<Loader2 className="animate-spin" size={18} />} title="Loading templates" />
            ) : filteredTemplates.length === 0 ? (
              <EmptyState icon={<FileCode2 size={18} />} title="No templates found" />
            ) : (
              filteredTemplates.map((template) => {
                const placement = placementFromPath(template);
                const details = placementDetails[placement];
                const active = selectedName === template.name && selectedPlacement === placement;
                return (
                  <button
                    key={`${placement}:${template.name}:${template.path}`}
                    type="button"
                    onClick={() => loadMutation.mutate({ name: template.name, placement })}
                    className={`w-full rounded-xl border p-3 text-left transition-all ${
                      active
                        ? 'border-hack-primary/60 bg-hack-primary/10 shadow-[0_0_20px_rgba(0,255,65,0.08)]'
                        : 'border-hack-border bg-black/20 hover:border-hack-primary/40 hover:bg-white/5'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <div className="truncate font-mono text-sm text-hack-text">{template.name}</div>
                        <div className="mt-1 truncate text-xs text-hack-dim">{template.path || details.path}</div>
                      </div>
                      <span className={`shrink-0 rounded-full border px-2 py-1 text-[10px] uppercase ${details.tone}`}>
                        {placement}
                      </span>
                    </div>
                    <div className="mt-3 flex items-center justify-between text-[11px] text-hack-dim">
                      <span>{formatBytes(template.size_bytes)}</span>
                      <span>{formatDate(template.updated_at)}</span>
                    </div>
                  </button>
                );
              })
            )}
          </div>
        </section>

        <main className="space-y-6">
          <section className="rounded-2xl border border-hack-border bg-black/25 p-5">
            <div className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
              <div>
                <h2 className="font-mono text-sm uppercase tracking-wider text-hack-text">Template editor</h2>
                <p className="mt-1 text-xs text-hack-dim">Validate first, then save to the selected profile placement.</p>
              </div>
              <div className="flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={startNewTemplate}
                  className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
                >
                  <Plus size={14} />
                  New
                </button>
                <button
                  type="button"
                  onClick={() => validateMutation.mutate()}
                  className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
                  disabled={validateMutation.isPending}
                >
                  {validateMutation.isPending ? <Loader2 size={14} className="animate-spin" /> : <ShieldCheck size={14} />}
                  Validate
                </button>
                <button
                  type="button"
                  onClick={() => saveMutation.mutate()}
                  className="hack-btn flex items-center gap-2 px-3 py-2 text-xs"
                  disabled={saveMutation.isPending}
                >
                  {saveMutation.isPending ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
                  Save
                </button>
                <button
                  type="button"
                  onClick={handleDelete}
                  className="hack-btn-ghost flex items-center gap-2 border border-hack-danger/40 px-3 py-2 text-xs text-hack-danger"
                  disabled={!selectedName || deleteMutation.isPending}
                >
                  <Trash2 size={14} />
                  Delete
                </button>
              </div>
            </div>

            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
              <div className="space-y-4">
                <label className="block space-y-2">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Template filename</span>
                  <input
                    value={templateName}
                    onChange={(event) => {
                      setTemplateName(event.target.value);
                      setValidation(null);
                    }}
                    className="hack-input w-full font-mono text-sm"
                    placeholder="example.yaml"
                    spellCheck={false}
                  />
                </label>

                <label className="block space-y-2">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">YAML content</span>
                  <textarea
                    value={content}
                    onChange={(event) => {
                      setContent(event.target.value);
                      setValidation(null);
                    }}
                    className="hack-input min-h-[620px] w-full resize-y font-mono text-sm leading-relaxed"
                    spellCheck={false}
                  />
                </label>
              </div>

              <aside className="space-y-4">
                <div className="rounded-xl border border-hack-border bg-black/25 p-4">
                  <label className="block space-y-2">
                    <span className="text-xs uppercase tracking-wider text-hack-dim">Placement</span>
                    <select
                      value={selectedPlacement}
                      onChange={(event) => {
                        setSelectedPlacement(event.target.value as NucleiTemplatePlacement);
                        setValidation(null);
                      }}
                      className="hack-input w-full text-sm"
                    >
                      {NUCLEI_TEMPLATE_PLACEMENTS.map((placement) => (
                        <option key={placement} value={placement}>
                          {placementDetails[placement].label}
                        </option>
                      ))}
                    </select>
                  </label>
                  <div className="mt-4 space-y-2 text-xs text-hack-dim">
                    <div className={`inline-flex rounded-full border px-2 py-1 ${selectedPlacementInfo.tone}`}>
                      {selectedPlacementInfo.profiles}
                    </div>
                    <div className="break-all font-mono">{selectedPlacementInfo.path}</div>
                  </div>
                </div>

                <div className={`rounded-xl border p-4 ${validationBadgeClass(validation)}`}>
                  <div className="flex items-center gap-2 text-sm font-semibold">
                    {validation?.valid ? <CheckCircle2 size={16} /> : <AlertTriangle size={16} />}
                    {validation ? (validation.valid ? 'Valid template' : 'Validation failed') : 'Not validated'}
                  </div>
                  {(validation?.error || validation?.output) && (
                    <pre className="mt-3 max-h-48 overflow-auto whitespace-pre-wrap text-xs">
                      {validation.error || validation.output}
                    </pre>
                  )}
                </div>

                <div className="rounded-xl border border-hack-border bg-black/25 p-4 text-xs text-hack-dim">
                  <div className="mb-2 flex items-center gap-2 text-hack-text">
                    <ShieldCheck size={15} className="text-hack-primary" />
                    Safety gates
                  </div>
                  <ul className="space-y-2">
                    <li>Manual validation before save</li>
                    <li>Profile placement controls execution scope</li>
                    <li>AI drafts never auto-save or auto-execute</li>
                  </ul>
                </div>
              </aside>
            </div>
          </section>

          <section className="grid gap-6 xl:grid-cols-2">
            <div className="rounded-2xl border border-hack-border bg-black/25 p-5">
              <div className="mb-4 flex items-start justify-between gap-4">
                <div>
                  <div className="mb-2 flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-hack-dim">
                    <Sparkles size={15} className="text-purple-300" />
                    AI draft foundation
                  </div>
                  <h3 className="text-lg font-semibold text-hack-text">Draft-only template builder</h3>
                  <p className="mt-1 text-sm text-hack-dim">
                    Generates a reviewable draft when enabled. It never saves or executes automatically.
                  </p>
                </div>
                <span
                  className={`rounded-full border px-3 py-1 text-xs uppercase ${
                    aiEnabled
                      ? 'border-hack-primary/50 bg-hack-primary/10 text-hack-primary'
                      : 'border-hack-border bg-white/5 text-hack-dim'
                  }`}
                >
                  {aiEnabled ? 'enabled' : 'disabled'}
                </span>
              </div>

              <div className="mb-4 grid gap-3 text-xs sm:grid-cols-3">
                <Policy label="Draft only" value={draftStatus?.draft_only !== false} />
                <Policy label="Human review" value={draftStatus?.requires_human_review !== false} />
                <Policy label="Auto save" value={!!draftStatus?.save_automatically} inverted />
              </div>

              {!aiEnabled && (
                <div className="mb-4 rounded-xl border border-amber-400/30 bg-amber-400/10 p-3 text-sm text-amber-200">
                  Set <span className="font-mono">NUCLEI_ALLOW_AI_TEMPLATES=true</span> to enable draft generation.
                  Strategy recommendations remain safe to use while draft generation is off.
                </div>
              )}

              <div className="grid gap-3 sm:grid-cols-2">
                <TextInput
                  label="Draft filename"
                  value={draftRequest.name}
                  onChange={(value) => updateDraftRequest('name', value)}
                />
                <TextInput
                  label="Title"
                  value={draftRequest.title}
                  onChange={(value) => updateDraftRequest('title', value)}
                />
                <TextInput
                  label="Tags"
                  value={draftRequest.tags.join(', ')}
                  onChange={(value) =>
                    updateDraftRequest(
                      'tags',
                      value
                        .split(',')
                        .map((item) => item.trim())
                        .filter(Boolean)
                    )
                  }
                />
                <label className="block space-y-2">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Severity</span>
                  <select
                    value={draftRequest.severity}
                    onChange={(event) => updateDraftRequest('severity', event.target.value as NucleiTemplateDraftRequest['severity'])}
                    className="hack-input w-full text-sm"
                  >
                    {['info', 'low', 'medium', 'high', 'critical'].map((severity) => (
                      <option key={severity} value={severity}>
                        {severity}
                      </option>
                    ))}
                  </select>
                </label>
                <TextInput label="Method" value={draftRequest.method} onChange={(value) => updateDraftRequest('method', value)} />
                <TextInput label="Path" value={draftRequest.path} onChange={(value) => updateDraftRequest('path', value)} />
                <TextInput
                  label="Matcher part"
                  value={draftRequest.matcher_part}
                  onChange={(value) => updateDraftRequest('matcher_part', value)}
                />
                <label className="block space-y-2">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Draft placement</span>
                  <select
                    value={draftPlacement}
                    onChange={(event) => setDraftPlacement(event.target.value as NucleiTemplatePlacement)}
                    className="hack-input w-full text-sm"
                  >
                    {NUCLEI_TEMPLATE_PLACEMENTS.map((placement) => (
                      <option key={placement} value={placement}>
                        {placementDetails[placement].label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <label className="mt-3 block space-y-2">
                <span className="text-xs uppercase tracking-wider text-hack-dim">Matcher value</span>
                <textarea
                  value={draftRequest.matcher_value}
                  onChange={(event) => updateDraftRequest('matcher_value', event.target.value)}
                  className="hack-input min-h-20 w-full resize-y font-mono text-sm"
                />
              </label>
              <label className="mt-3 block space-y-2">
                <span className="text-xs uppercase tracking-wider text-hack-dim">Description</span>
                <textarea
                  value={draftRequest.description}
                  onChange={(event) => updateDraftRequest('description', event.target.value)}
                  className="hack-input min-h-20 w-full resize-y text-sm"
                />
              </label>

              <div className="mt-4 flex flex-wrap gap-2">
                <button
                  type="button"
                  onClick={() => draftMutation.mutate()}
                  className="hack-btn flex items-center gap-2 px-3 py-2 text-xs"
                  disabled={!aiEnabled || draftMutation.isPending}
                >
                  {draftMutation.isPending ? <Loader2 size={14} className="animate-spin" /> : <Wand2 size={14} />}
                  Generate draft
                </button>
                {generatedDraft && (
                  <button
                    type="button"
                    onClick={() => applyDraftToEditor(generatedDraft)}
                    className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
                  >
                    <FileCode2 size={14} />
                    Load in editor
                  </button>
                )}
              </div>

              {generatedDraft && <DraftPreview draft={generatedDraft} onCopy={copyDraft} />}
            </div>

            <div className="rounded-2xl border border-hack-border bg-black/25 p-5">
              <div className="mb-4 flex items-start justify-between gap-4">
                <div>
                  <div className="mb-2 flex items-center gap-2 text-xs uppercase tracking-[0.25em] text-hack-dim">
                    <Bot size={15} className="text-cyan-300" />
                    Agent strategy foundation
                  </div>
                  <h3 className="text-lg font-semibold text-hack-text">Target-aware recommendation</h3>
                  <p className="mt-1 text-sm text-hack-dim">
                    Lets the future agent choose profiles, tags, placements, and optional draft requests from target context.
                  </p>
                </div>
              </div>

              <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
                <TextInput label="Target ID" value={strategyTargetId} onChange={setStrategyTargetId} placeholder="Example: 12" />
                <div className="flex items-end">
                  <button
                    type="button"
                    onClick={() => strategyMutation.mutate()}
                    className="hack-btn flex h-[42px] items-center gap-2 px-4 text-xs"
                    disabled={!strategyTargetId.trim() || strategyMutation.isPending}
                  >
                    {strategyMutation.isPending ? <Loader2 size={14} className="animate-spin" /> : <Sparkles size={14} />}
                    Analyze
                  </button>
                </div>
              </div>
              <label className="mt-3 flex items-center gap-2 text-sm text-hack-dim">
                <input
                  type="checkbox"
                  checked={includeDraft}
                  onChange={(event) => setIncludeDraft(event.target.checked)}
                  className="accent-hack-primary"
                />
                Include draft when AI template drafts are enabled
              </label>

              {strategy ? (
                <StrategyPanel
                  strategy={strategy}
                  onUseDraft={(draft) => applyDraftToEditor(draft, draftPlacement)}
                />
              ) : (
                <div className="mt-4 rounded-xl border border-dashed border-hack-border p-6 text-center text-sm text-hack-dim">
                  Enter a target ID to generate an agent-ready Nuclei strategy.
                </div>
              )}
            </div>
          </section>
        </main>
      </div>
    </div>
  );
};

const Metric = ({ label, value, tone = 'default' }: { label: string; value: string | number; tone?: 'default' | 'ok' | 'safe' | 'muted' }) => {
  const toneClass =
    tone === 'ok'
      ? 'text-hack-primary'
      : tone === 'safe'
        ? 'text-cyan-300'
        : tone === 'muted'
          ? 'text-hack-dim'
          : 'text-hack-text';

  return (
    <div className="rounded-xl border border-hack-border bg-black/30 px-3 py-2">
      <div className="text-[10px] uppercase tracking-wider text-hack-dim">{label}</div>
      <div className={`mt-1 font-mono text-sm ${toneClass}`}>{value}</div>
    </div>
  );
};

const EmptyState = ({ icon, title }: { icon: React.ReactNode; title: string }) => (
  <div className="rounded-xl border border-dashed border-hack-border p-6 text-center text-sm text-hack-dim">
    <div className="mb-2 flex justify-center">{icon}</div>
    {title}
  </div>
);

const Policy = ({ label, value, inverted = false }: { label: string; value: boolean; inverted?: boolean }) => {
  const ok = inverted ? !value : value;
  return (
    <div className={`rounded-xl border p-3 ${ok ? 'border-hack-primary/40 bg-hack-primary/10' : 'border-hack-danger/40 bg-hack-danger/10'}`}>
      <div className="flex items-center gap-2 text-xs text-hack-text">
        {ok ? <CheckCircle2 size={14} className="text-hack-primary" /> : <AlertTriangle size={14} className="text-hack-danger" />}
        {label}
      </div>
    </div>
  );
};

const TextInput = ({
  label,
  value,
  onChange,
  placeholder,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  placeholder?: string;
}) => (
  <label className="block space-y-2">
    <span className="text-xs uppercase tracking-wider text-hack-dim">{label}</span>
    <input
      value={value}
      onChange={(event) => onChange(event.target.value)}
      placeholder={placeholder}
      className="hack-input w-full text-sm"
      spellCheck={false}
    />
  </label>
);

const DraftPreview = ({ draft, onCopy }: { draft: NucleiTemplateDraft; onCopy: (draft: NucleiTemplateDraft) => void }) => (
  <div className="mt-4 rounded-xl border border-hack-border bg-black/30 p-4">
    <div className="mb-3 flex items-center justify-between gap-3">
      <div>
        <div className="font-mono text-sm text-hack-text">{draft.name}</div>
        <div className="text-xs text-hack-dim">
          saved={String(draft.saved)} · draft_only={String(draft.draft_only)} · human_review={String(draft.requires_human_review)}
        </div>
      </div>
      <button
        type="button"
        onClick={() => onCopy(draft)}
        className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
      >
        <Clipboard size={14} />
        Copy
      </button>
    </div>
    <pre className="max-h-64 overflow-auto whitespace-pre-wrap rounded-lg border border-hack-border bg-black/40 p-3 font-mono text-xs text-hack-dim">
      {draft.content}
    </pre>
  </div>
);

const StrategyPanel = ({
  strategy,
  onUseDraft,
}: {
  strategy: NucleiTemplateStrategy;
  onUseDraft: (draft: NucleiTemplateDraft) => void;
}) => (
  <div className="mt-4 space-y-4">
    <div className="grid gap-3 sm:grid-cols-2">
      <Metric label="Recommended profile" value={strategy.recommended_profile || '-'} tone="ok" />
      <Metric label="Agent ready" value={strategy.agent_ready ? 'yes' : 'no'} tone={strategy.agent_ready ? 'safe' : 'muted'} />
      <Metric label="Auto save" value={strategy.save_automatically ? 'yes' : 'no'} tone="safe" />
      <Metric label="Auto execute" value={strategy.execute_automatically ? 'yes' : 'no'} tone="safe" />
    </div>

    <TagGroup title="Recommended tags" values={asList(strategy.recommended_tags)} />
    <TagGroup title="Recommended placements" values={asList(strategy.recommended_placements)} />
    <TagGroup title="Template sets" values={asList(strategy.recommended_template_sets)} />
    <TagGroup title="Signals" values={asList(strategy.signals)} muted />

    {asList(strategy.rationale).length > 0 && (
      <div className="rounded-xl border border-hack-border bg-black/30 p-4">
        <div className="mb-2 text-xs uppercase tracking-wider text-hack-dim">Rationale</div>
        <ul className="list-disc space-y-1 pl-4 text-sm text-hack-dim">
          {asList(strategy.rationale).map((item) => (
            <li key={item}>{item}</li>
          ))}
        </ul>
      </div>
    )}

    {strategy.generated_draft && (
      <div className="rounded-xl border border-purple-400/30 bg-purple-400/10 p-4">
        <div className="mb-2 flex items-center justify-between gap-3">
          <div>
            <div className="font-mono text-sm text-hack-text">{strategy.generated_draft.name}</div>
            <div className="text-xs text-hack-dim">Generated draft attached to strategy. It is not saved.</div>
          </div>
          <button
            type="button"
            onClick={() => onUseDraft(strategy.generated_draft as NucleiTemplateDraft)}
            className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
          >
            <FileCode2 size={14} />
            Use draft
          </button>
        </div>
      </div>
    )}
  </div>
);

const TagGroup = ({ title, values, muted = false }: { title: string; values: string[]; muted?: boolean }) => (
  <div className="rounded-xl border border-hack-border bg-black/30 p-4">
    <div className="mb-2 text-xs uppercase tracking-wider text-hack-dim">{title}</div>
    {values.length === 0 ? (
      <div className="text-sm text-hack-dim">-</div>
    ) : (
      <div className="flex flex-wrap gap-2">
        {values.map((value) => (
          <span
            key={value}
            className={`rounded-full border px-2 py-1 text-xs ${
              muted ? 'border-hack-border text-hack-dim' : 'border-hack-primary/40 text-hack-primary'
            }`}
          >
            {value}
          </span>
        ))}
      </div>
    )}
  </div>
);

export default NucleiTemplates;
