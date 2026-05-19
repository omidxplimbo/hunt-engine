import { useMemo, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import { AlertCircle, CheckCircle2, FolderTree, Loader2, Plus, RefreshCw, Save, Search, ShieldCheck, Trash2 } from 'lucide-react';

import {
  deleteNucleiTemplate,
  getNucleiTemplate,
  listNucleiTemplates,
  saveNucleiTemplate,
  validateNucleiTemplate,
  type NucleiTemplate,
  type NucleiTemplatePlacement,
  type NucleiTemplateValidation,
} from '../api/nucleiTemplates';
import { useAuth } from '../context/AuthContext';

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

type PlacementOption = {
  value: NucleiTemplatePlacement;
  label: string;
  description: string;
  runsIn: string;
};

const placementOptions: PlacementOption[] = [
  {
    value: 'root',
    label: 'Root / Legacy',
    description: 'Save directly under /data/nuclei/custom.',
    runsIn: 'all profiles',
  },
  {
    value: 'shared',
    label: 'Shared',
    description: 'Reusable templates that should always run.',
    runsIn: 'all profiles',
  },
  {
    value: 'safe',
    label: 'Safe',
    description: 'Low-risk checks that are acceptable in every scan mode.',
    runsIn: 'all profiles',
  },
  {
    value: 'fast',
    label: 'Fast',
    description: 'Quick exposure and panel checks.',
    runsIn: 'fast, balanced, full, custom',
  },
  {
    value: 'exposure',
    label: 'Exposure',
    description: 'Exposure-focused checks grouped with fast scans.',
    runsIn: 'fast, balanced, full, custom',
  },
  {
    value: 'balanced',
    label: 'Balanced',
    description: 'Broader checks for regular scans.',
    runsIn: 'balanced, full, custom',
  },
  {
    value: 'misconfig',
    label: 'Misconfig',
    description: 'Configuration issues that should not run in the fastest mode.',
    runsIn: 'balanced, full, custom',
  },
  {
    value: 'cves',
    label: 'CVEs',
    description: 'CVE-oriented checks.',
    runsIn: 'cves-light, full, custom',
  },
  {
    value: 'cves-light',
    label: 'CVEs Light',
    description: 'Focused CVE checks for the cves-light profile.',
    runsIn: 'cves-light, full, custom',
  },
  {
    value: 'full',
    label: 'Full',
    description: 'Heavier templates reserved for full scans.',
    runsIn: 'full, custom',
  },
  {
    value: 'custom',
    label: 'Custom',
    description: 'Special-purpose templates for explicit custom scans.',
    runsIn: 'custom, full',
  },
];

const placementMap = new Map(
  placementOptions.map((option) => [option.value, option])
);

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

const inferPlacement = (template: Partial<NucleiTemplate>): NucleiTemplatePlacement => {
  if (template.placement) return template.placement;

  const source = `${template.path || ''}/${template.name || ''}`;
  const found = placementOptions.find(
    (option) =>
      option.value !== 'root' &&
      (source.includes(`/custom/${option.value}/`) ||
        source.startsWith(`${option.value}/`))
  );

  return found?.value || 'root';
};

const templateFileName = (template: Partial<NucleiTemplate>) => {
  const raw = template.name || '';
  return raw.split('/').filter(Boolean).pop() || raw || 'hunt-custom-marker.yaml';
};

const makeSavingPath = (placement: NucleiTemplatePlacement, name: string) => {
  const cleanName = name.trim() || 'template.yaml';
  if (placement === 'root') return `/data/nuclei/custom/${cleanName}`;
  return `/data/nuclei/custom/${placement}/${cleanName}`;
};

const validationState = (validation: NucleiTemplateValidation | null) => {
  if (!validation) {
    return {
      title: 'Validation not run',
      icon: ShieldCheck,
      className: 'border-hack-border/80 bg-white/[0.02] text-hack-dim',
    };
  }

  if (validation.valid) {
    return {
      title: 'Valid template',
      icon: CheckCircle2,
      className: 'border-hack-primary/40 bg-hack-primary/[0.07] text-hack-primary',
    };
  }

  return {
    title: 'Validation failed',
    icon: AlertCircle,
    className: 'border-hack-danger/50 bg-hack-danger/[0.07] text-hack-danger',
  };
};

const NucleiTemplates = () => {
  const { role } = useAuth();
  const queryClient = useQueryClient();

  const [selectedName, setSelectedName] = useState('');
  const [templateName, setTemplateName] = useState('hunt-custom-marker.yaml');
  const [placement, setPlacement] = useState<NucleiTemplatePlacement>('fast');
  const [content, setContent] = useState(defaultTemplate);
  const [validation, setValidation] = useState<NucleiTemplateValidation | null>(null);
  const [search, setSearch] = useState('');
  const [placementFilter, setPlacementFilter] = useState<'all' | NucleiTemplatePlacement>(
    'all'
  );

  const templatesQuery = useQuery({
    queryKey: ['nuclei-templates'],
    queryFn: listNucleiTemplates,
  });

  const templates = useMemo(() => templatesQuery.data || [], [templatesQuery.data]);

  const filteredTemplates = useMemo(() => {
    const query = search.trim().toLowerCase();

    return templates.filter((template) => {
      const templatePlacement = inferPlacement(template);
      if (placementFilter !== 'all' && templatePlacement !== placementFilter) {
        return false;
      }

      if (!query) return true;

      return [template.name, template.path, templatePlacement]
        .filter(Boolean)
        .join(' ')
        .toLowerCase()
        .includes(query);
    });
  }, [placementFilter, search, templates]);

  const selectedTemplate = useMemo(
    () => templates.find((template) => template.name === selectedName),
    [selectedName, templates]
  );

  const currentPlacement = placementMap.get(placement) || placementMap.get('root')!;
  const currentValidation = validationState(validation);
  const ValidationIcon = currentValidation.icon;

  const loadMutation = useMutation({
    mutationFn: getNucleiTemplate,
    onSuccess: (template) => {
      const nextPlacement = inferPlacement(template);
      setSelectedName(template.name);
      setTemplateName(templateFileName(template));
      setPlacement(nextPlacement);
      setContent(template.content || '');
      setValidation(null);
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const validateMutation = useMutation({
    mutationFn: () =>
      validateNucleiTemplate({ name: templateName, content, placement }),
    onSuccess: (result) => {
      setValidation(result);
      if (result.valid) {
        toast.success('Template validation passed');
      } else {
        toast.error(result.error || 'Template validation failed');
      }
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const saveMutation = useMutation({
    mutationFn: () =>
      saveNucleiTemplate({
        name: templateName,
        placement,
        content,
        validate: true,
      }),
    onSuccess: (template) => {
      const nextPlacement = inferPlacement(template);
      toast.success('Template saved');
      setSelectedName(template.name);
      setTemplateName(templateFileName(template));
      setPlacement(nextPlacement);
      setContent(template.content || content);
      setValidation({ valid: true, name: template.name });
      queryClient.invalidateQueries({ queryKey: ['nuclei-templates'] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteNucleiTemplate,
    onSuccess: () => {
      toast.success('Template deleted');
      startNewTemplate();
      queryClient.invalidateQueries({ queryKey: ['nuclei-templates'] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const startNewTemplate = () => {
    setSelectedName('');
    setTemplateName('hunt-custom-marker.yaml');
    setPlacement('fast');
    setContent(defaultTemplate);
    setValidation(null);
  };

  const handleDelete = () => {
    if (!selectedName) return;
    if (!confirm(`Delete Nuclei template "${selectedName}"?`)) return;
    deleteMutation.mutate(selectedName);
  };

  if (role !== 'admin') return <Navigate to="/" replace />;

  const isBusy =
    templatesQuery.isLoading ||
    loadMutation.isPending ||
    validateMutation.isPending ||
    saveMutation.isPending ||
    deleteMutation.isPending;

  const editorStatus = selectedTemplate
    ? `${formatBytes(selectedTemplate.size_bytes)} • ${formatDate(selectedTemplate.updated_at)}`
    : 'New unsaved template';

  return (
    <div className="mx-auto w-full max-w-[1500px] space-y-6 px-4 py-6 lg:px-8">
      <section className="rounded-2xl border border-hack-border/70 bg-hack-panel/70 p-6 shadow-[0_18px_70px_rgba(0,0,0,0.35)] backdrop-blur-xl">
        <div className="flex flex-col gap-5 xl:flex-row xl:items-center xl:justify-between">
          <div className="space-y-3">
            <div className="inline-flex items-center gap-2 rounded-full border border-hack-primary/30 bg-hack-primary/[0.06] px-3 py-1 text-xs font-semibold uppercase tracking-[0.28em] text-hack-primary">
              <ShieldCheck className="h-4 w-4" />
              Nuclei Security Engine
            </div>
            <div>
              <h1 className="text-3xl font-semibold tracking-tight text-hack-text md:text-4xl">
                Template Management
              </h1>
              <p className="mt-2 max-w-3xl text-sm leading-6 text-hack-dim">
                Create, validate, and place custom Nuclei templates into the exact
                scan profile that should execute them.
              </p>
            </div>
          </div>

          <div className="grid grid-cols-2 gap-3 sm:flex sm:items-center">
            <div className="rounded-xl border border-hack-border/70 bg-black/20 px-4 py-3">
              <p className="text-[10px] uppercase tracking-[0.22em] text-hack-dim">
                Templates
              </p>
              <p className="mt-1 text-2xl font-semibold text-hack-text">
                {templates.length}
              </p>
            </div>
            <div className="rounded-xl border border-hack-border/70 bg-black/20 px-4 py-3">
              <p className="text-[10px] uppercase tracking-[0.22em] text-hack-dim">
                Current placement
              </p>
              <p className="mt-1 text-sm font-semibold text-hack-primary">
                {currentPlacement.label}
              </p>
            </div>
            <button
              onClick={() => templatesQuery.refetch()}
              className="hack-btn-ghost min-h-[44px] rounded-xl border border-hack-border/80 bg-black/20 px-4"
              disabled={isBusy}
            >
              <RefreshCw className={templatesQuery.isFetching ? 'h-4 w-4 animate-spin' : 'h-4 w-4'} />
              Refresh
            </button>
            <button onClick={startNewTemplate} className="hack-btn min-h-[44px] rounded-xl">
              <Plus className="h-4 w-4" />
              New Template
            </button>
          </div>
        </div>
      </section>

      <div className="grid gap-6 xl:grid-cols-[390px_minmax(0,1fr)]">
        <aside className="rounded-2xl border border-hack-border/70 bg-hack-panel/70 p-4 backdrop-blur-xl">
          <div className="mb-4 flex items-center justify-between gap-3">
            <div>
              <div className="flex items-center gap-2 text-hack-text">
                <FolderTree className="h-4 w-4 text-hack-primary" />
                <h2 className="font-semibold">Custom templates</h2>
              </div>
              <p className="mt-1 text-xs text-hack-dim">
                {filteredTemplates.length} of {templates.length} shown
              </p>
            </div>
          </div>

          <div className="space-y-3">
            <label className="relative block">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-hack-dim" />
              <input
                value={search}
                onChange={(event) => setSearch(event.target.value)}
                className="hack-input w-full rounded-xl border-hack-border/80 pl-9 text-hack-text"
                placeholder="Search templates..."
                spellCheck={false}
              />
            </label>

            <select
              value={placementFilter}
              onChange={(event) =>
                setPlacementFilter(event.target.value as 'all' | NucleiTemplatePlacement)
              }
              className="hack-input w-full rounded-xl border-hack-border/80 text-hack-text"
            >
              <option value="all">All placements</option>
              {placementOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          <div className="mt-4 max-h-[720px] space-y-2 overflow-y-auto pr-1">
            {templatesQuery.isLoading ? (
              <div className="flex items-center gap-3 rounded-xl border border-hack-border/70 bg-black/20 p-4 text-sm text-hack-dim">
                <Loader2 className="h-4 w-4 animate-spin" />
                Loading templates...
              </div>
            ) : filteredTemplates.length === 0 ? (
              <div className="rounded-xl border border-dashed border-hack-border/80 bg-black/20 p-5 text-sm leading-6 text-hack-dim">
                No templates match the current filters.
              </div>
            ) : (
              filteredTemplates.map((template: NucleiTemplate) => {
                const itemPlacement = inferPlacement(template);
                const option = placementMap.get(itemPlacement);
                const active = selectedName === template.name;

                return (
                  <button
                    key={`${template.placement || 'root'}:${template.name}`}
                    onClick={() => loadMutation.mutate(template.name)}
                    className={`w-full rounded-xl border p-4 text-left transition-all ${
                      active
                        ? 'border-hack-primary/60 bg-hack-primary/[0.08] shadow-[0_0_0_1px_rgba(0,255,65,0.12)]'
                        : 'border-hack-border/70 bg-black/20 hover:border-hack-primary/40 hover:bg-white/[0.03]'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-hack-text">
                          {templateFileName(template)}
                        </p>
                        <p className="mt-1 truncate text-xs text-hack-dim">
                          {template.path || template.name}
                        </p>
                      </div>
                      <span className="rounded-full border border-hack-primary/30 bg-hack-primary/[0.06] px-2 py-1 text-[10px] font-semibold uppercase tracking-wider text-hack-primary">
                        {option?.label || itemPlacement}
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
        </aside>

        <section className="rounded-2xl border border-hack-border/70 bg-hack-panel/70 p-5 backdrop-blur-xl">
          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_320px]">
            <label className="space-y-2">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-hack-dim">
                Template filename
              </span>
              <input
                value={templateName}
                onChange={(event) => {
                  setTemplateName(event.target.value);
                  setValidation(null);
                }}
                className="hack-input h-12 w-full rounded-xl border-hack-border/80 font-mono text-hack-text"
                placeholder="example.yaml"
                spellCheck={false}
              />
            </label>

            <label className="space-y-2">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-hack-dim">
                Profile placement
              </span>
              <select
                value={placement}
                onChange={(event) => {
                  setPlacement(event.target.value as NucleiTemplatePlacement);
                  setValidation(null);
                }}
                className="hack-input h-12 w-full rounded-xl border-hack-border/80 text-hack-text"
              >
                {placementOptions.map((option) => (
                  <option key={option.value} value={option.value}>
                    {option.label}
                  </option>
                ))}
              </select>
            </label>
          </div>

          <div className="mt-4 rounded-2xl border border-hack-border/70 bg-black/25 p-4">
            <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_340px] lg:items-center">
              <div>
                <p className="text-xs font-semibold uppercase tracking-[0.2em] text-hack-dim">
                  Saving path
                </p>
                <p className="mt-2 break-all rounded-xl border border-hack-border/60 bg-black/25 px-3 py-2 font-mono text-sm text-hack-primary">
                  {makeSavingPath(placement, templateName)}
                </p>
              </div>
              <div className="rounded-xl border border-hack-border/60 bg-black/25 p-3 text-sm leading-6 text-hack-dim">
                <p className="font-semibold text-hack-text">{currentPlacement.label}</p>
                <p>{currentPlacement.description}</p>
                <p className="mt-1 text-xs text-hack-primary">
                  Runs in {currentPlacement.runsIn}
                </p>
              </div>
            </div>
          </div>

          <div className="mt-4 flex flex-col gap-3 border-b border-hack-border/70 pb-4 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-xs text-hack-dim">{editorStatus}</div>
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => validateMutation.mutate()}
                className="hack-btn-ghost min-h-[42px] rounded-xl border border-hack-border/80 bg-black/20 px-4"
                disabled={validateMutation.isPending}
              >
                {validateMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <ShieldCheck className="h-4 w-4" />
                )}
                Validate
              </button>
              <button
                onClick={() => saveMutation.mutate()}
                className="hack-btn min-h-[42px] rounded-xl"
                disabled={saveMutation.isPending}
              >
                {saveMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Save className="h-4 w-4" />
                )}
                Save
              </button>
              <button
                onClick={handleDelete}
                className="hack-btn-danger min-h-[42px] rounded-xl"
                disabled={!selectedName || deleteMutation.isPending}
              >
                <Trash2 className="h-4 w-4" />
                Delete
              </button>
            </div>
          </div>

          <div className={`mt-4 rounded-2xl border p-4 ${currentValidation.className}`}>
            <div className="flex items-start gap-3">
              <ValidationIcon className="mt-0.5 h-5 w-5 flex-shrink-0" />
              <div className="min-w-0">
                <p className="font-semibold uppercase tracking-[0.14em]">
                  {currentValidation.title}
                </p>
                {(validation?.error || validation?.output) && (
                  <pre className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap rounded-xl border border-current/20 bg-black/25 p-3 text-xs leading-5">
                    {validation.error || validation.output}
                  </pre>
                )}
              </div>
            </div>
          </div>

          <label className="mt-5 block space-y-2">
            <div className="flex items-center justify-between gap-3">
              <span className="text-xs font-semibold uppercase tracking-[0.2em] text-hack-dim">
                YAML content
              </span>
              <span className="text-xs text-hack-dim">
                {content.split('\n').length} lines
              </span>
            </div>
            <textarea
              value={content}
              onChange={(event) => {
                setContent(event.target.value);
                setValidation(null);
              }}
              className="hack-input min-h-[560px] w-full resize-y rounded-2xl border-hack-border/80 bg-black/35 p-4 font-mono text-sm leading-6 text-hack-primary"
              spellCheck={false}
            />
          </label>
        </section>
      </div>
    </div>
  );
};

export default NucleiTemplates;
