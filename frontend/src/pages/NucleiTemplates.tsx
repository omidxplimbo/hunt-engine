import { useMemo, useState } from 'react';
import { Navigate } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import toast from 'react-hot-toast';
import {
  AlertTriangle,
  CheckCircle2,
  FileCode2,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  ShieldCheck,
  Trash2,
} from 'lucide-react';
import { useAuth } from '../context/AuthContext';
import {
  deleteNucleiTemplate,
  getNucleiTemplate,
  listNucleiTemplates,
  saveNucleiTemplate,
  validateNucleiTemplate,
  type NucleiTemplate,
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

const validationBadgeClass = (validation: NucleiTemplateValidation | null) => {
  if (!validation) return 'border-hack-border text-hack-dim';
  return validation.valid
    ? 'border-hack-primary/50 text-hack-primary bg-hack-primary/10'
    : 'border-hack-danger/50 text-hack-danger bg-hack-danger/10';
};

const NucleiTemplates = () => {
  const { role } = useAuth();
  const queryClient = useQueryClient();
  const [selectedName, setSelectedName] = useState('');
  const [templateName, setTemplateName] = useState('hunt-custom-marker.yaml');
  const [content, setContent] = useState(defaultTemplate);
  const [validation, setValidation] = useState<NucleiTemplateValidation | null>(null);

  const templatesQuery = useQuery({
    queryKey: ['nuclei-templates'],
    queryFn: listNucleiTemplates,
  });

  const templates = useMemo(() => templatesQuery.data || [], [templatesQuery.data]);

  const loadMutation = useMutation({
    mutationFn: getNucleiTemplate,
    onSuccess: (template) => {
      setSelectedName(template.name);
      setTemplateName(template.name);
      setContent(template.content || '');
      setValidation(null);
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const validateMutation = useMutation({
    mutationFn: () => validateNucleiTemplate({ name: templateName, content }),
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
    mutationFn: () => saveNucleiTemplate({ name: templateName, content, validate: true }),
    onSuccess: (template) => {
      toast.success('Template saved');
      setSelectedName(template.name);
      setTemplateName(template.name);
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
      setSelectedName('');
      setTemplateName('hunt-custom-marker.yaml');
      setContent(defaultTemplate);
      setValidation(null);
      queryClient.invalidateQueries({ queryKey: ['nuclei-templates'] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  if (role !== 'admin') return <Navigate to="/" replace />;

  const isBusy =
    templatesQuery.isLoading ||
    loadMutation.isPending ||
    validateMutation.isPending ||
    saveMutation.isPending ||
    deleteMutation.isPending;

  const startNewTemplate = () => {
    setSelectedName('');
    setTemplateName('hunt-custom-marker.yaml');
    setContent(defaultTemplate);
    setValidation(null);
  };

  const handleDelete = () => {
    if (!selectedName) return;
    if (!confirm(`Delete Nuclei template "${selectedName}"?`)) return;
    deleteMutation.mutate(selectedName);
  };

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4 md:flex-row md:items-center md:justify-between">
        <div>
          <h1 className="text-3xl font-mono font-bold text-hack-primary flex items-center gap-3">
            <FileCode2 className="w-8 h-8" />
            NUCLEI TEMPLATE CONSOLE
          </h1>
          <p className="text-hack-dim mt-2 font-mono text-sm">
            Manage custom Nuclei YAML templates stored under /data/nuclei/custom.
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={() => templatesQuery.refetch()}
            className="hack-btn-ghost border border-hack-border flex items-center gap-2"
            disabled={isBusy}
          >
            <RefreshCw className={`w-4 h-4 ${templatesQuery.isFetching ? 'animate-spin' : ''}`} />
            Refresh
          </button>
          <button type="button" onClick={startNewTemplate} className="hack-btn flex items-center gap-2">
            <Plus className="w-4 h-4" />
            New Template
          </button>
        </div>
      </div>

      <div className="grid grid-cols-1 xl:grid-cols-[360px_1fr] gap-6">
        <section className="hack-box p-4">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-hack-primary font-mono font-bold">Custom_Templates</h2>
            <span className="text-xs font-mono text-hack-dim border border-hack-border px-2 py-1">
              {templates.length} files
            </span>
          </div>

          {templatesQuery.isLoading ? (
            <div className="flex items-center gap-2 text-hack-dim font-mono text-sm">
              <Loader2 className="w-4 h-4 animate-spin" />
              Loading templates...
            </div>
          ) : templates.length === 0 ? (
            <div className="border border-dashed border-hack-border p-4 text-sm text-hack-dim font-mono">
              No custom templates yet. Create one from the editor.
            </div>
          ) : (
            <div className="space-y-2 max-h-[640px] overflow-auto pr-1">
              {templates.map((template: NucleiTemplate) => (
                <button
                  key={template.name}
                  type="button"
                  onClick={() => loadMutation.mutate(template.name)}
                  className={`w-full text-left border p-3 font-mono transition-colors ${
                    selectedName === template.name
                      ? 'border-hack-primary/60 bg-hack-primary/10 text-hack-primary'
                      : 'border-hack-border text-hack-text hover:border-hack-primary/40 hover:bg-white/5'
                  }`}
                >
                  <div className="font-bold break-all">{template.name}</div>
                  <div className="mt-2 flex items-center justify-between text-xs text-hack-dim">
                    <span>{formatBytes(template.size_bytes)}</span>
                    <span>{formatDate(template.updated_at)}</span>
                  </div>
                </button>
              ))}
            </div>
          )}
        </section>

        <section className="hack-box p-4 space-y-4">
          <div className="grid grid-cols-1 lg:grid-cols-[1fr_auto] gap-3 lg:items-end">
            <label className="block">
              <span className="block text-hack-dim font-mono text-xs mb-2">Template filename</span>
              <input
                value={templateName}
                onChange={(event) => {
                  setTemplateName(event.target.value);
                  setValidation(null);
                }}
                className="hack-input w-full font-mono"
                placeholder="example.yaml"
                spellCheck={false}
              />
            </label>

            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => validateMutation.mutate()}
                className="hack-btn-ghost border border-hack-border flex items-center gap-2"
                disabled={validateMutation.isPending}
              >
                {validateMutation.isPending ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <ShieldCheck className="w-4 h-4" />
                )}
                Validate
              </button>
              <button
                type="button"
                onClick={() => saveMutation.mutate()}
                className="hack-btn flex items-center gap-2"
                disabled={saveMutation.isPending}
              >
                {saveMutation.isPending ? (
                  <Loader2 className="w-4 h-4 animate-spin" />
                ) : (
                  <Save className="w-4 h-4" />
                )}
                Save
              </button>
              <button
                type="button"
                onClick={handleDelete}
                className="hack-btn-danger flex items-center gap-2"
                disabled={!selectedName || deleteMutation.isPending}
              >
                <Trash2 className="w-4 h-4" />
                Delete
              </button>
            </div>
          </div>

          <div className={`border px-3 py-2 font-mono text-sm flex items-start gap-2 ${validationBadgeClass(validation)}`}>
            {validation?.valid ? <CheckCircle2 className="w-4 h-4 mt-0.5" /> : <AlertTriangle className="w-4 h-4 mt-0.5" />}
            <div>
              <div>{validation ? (validation.valid ? 'VALID TEMPLATE' : 'VALIDATION FAILED') : 'VALIDATION NOT RUN'}</div>
              {(validation?.error || validation?.output) && (
                <pre className="mt-2 whitespace-pre-wrap break-words text-xs text-hack-text">
                  {validation.error || validation.output}
                </pre>
              )}
            </div>
          </div>

          <label className="block">
            <span className="block text-hack-dim font-mono text-xs mb-2">YAML content</span>
            <textarea
              value={content}
              onChange={(event) => {
                setContent(event.target.value);
                setValidation(null);
              }}
              className="hack-input w-full min-h-[620px] font-mono text-sm leading-relaxed resize-y"
              spellCheck={false}
            />
          </label>
        </section>
      </div>
    </div>
  );
};

export default NucleiTemplates;
