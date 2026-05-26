import { useMemo, useState } from "react";
import { Navigate } from "react-router-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import toast from "react-hot-toast";
import {
  AlertCircle,
  CheckCircle2,
  Loader2,
  Plus,
  RefreshCw,
  Save,
  Search,
  ShieldCheck,
  Trash2,
  Wand2,
} from "lucide-react";
import {
  deleteNucleiTemplate,
  generateNucleiTemplateDraft,
  getNucleiTemplate,
  getNucleiTemplateDraftStatus,
  getNucleiTemplateStrategy,
  listNucleiTemplates,
  saveNucleiTemplate,
  validateNucleiTemplate,
  type GenerateNucleiTemplateDraftPayload,
  type NucleiTemplate,
  type NucleiTemplateDraftResponse,
  type NucleiTemplatePlacement,
  type NucleiTemplateStrategy,
  type NucleiTemplateValidation,
} from "../api/nucleiTemplates";
import { useAuth } from "../context/AuthContext";

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
    value: "root",
    label: "Root",
    description: "Legacy location under /data/nuclei/custom.",
    runsIn: "all profiles",
  },
  {
    value: "shared",
    label: "Shared",
    description: "Reusable checks that should always run.",
    runsIn: "all profiles",
  },
  {
    value: "safe",
    label: "Safe",
    description: "Low-risk checks accepted in every scan mode.",
    runsIn: "all profiles",
  },
  {
    value: "fast",
    label: "Fast",
    description: "Quick exposure and panel checks.",
    runsIn: "fast, balanced, full, custom",
  },
  {
    value: "exposure",
    label: "Exposure",
    description: "Exposure checks grouped with fast scans.",
    runsIn: "fast, balanced, full, custom",
  },
  {
    value: "balanced",
    label: "Balanced",
    description: "Broader regular-scan checks.",
    runsIn: "balanced, full, custom",
  },
  {
    value: "misconfig",
    label: "Misconfig",
    description:
      "Configuration checks that should not run in the fastest mode.",
    runsIn: "balanced, full, custom",
  },
  {
    value: "cves",
    label: "CVEs",
    description: "CVE-oriented checks.",
    runsIn: "cves-light, full, custom",
  },
  {
    value: "cves-light",
    label: "CVEs Light",
    description: "Focused CVE checks for the cves-light profile.",
    runsIn: "cves-light, full, custom",
  },
  {
    value: "full",
    label: "Full",
    description: "Heavier templates reserved for full scans.",
    runsIn: "full, custom",
  },
  {
    value: "custom",
    label: "Custom",
    description: "Special-purpose templates for explicit custom scans.",
    runsIn: "custom, full",
  },
];

const placementMap = new Map(
  placementOptions.map((option) => [option.value, option]),
);

const formatBytes = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return "0 B";
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KiB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MiB`;
};

const formatDate = (value: string) => {
  if (!value) return "-";
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "-";
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
    "Operation failed"
  );
};

const inferPlacement = (
  template: Partial<NucleiTemplate>,
): NucleiTemplatePlacement => {
  if (template.placement) return template.placement;

  const source = `${template.path || ""}/${template.name || ""}`;
  const found = placementOptions.find(
    (option) =>
      option.value !== "root" &&
      (source.includes(`/custom/${option.value}/`) ||
        source.startsWith(`${option.value}/`)),
  );

  return found?.value || "root";
};

const templateFileName = (template: Partial<NucleiTemplate>) => {
  const raw = template.name || "";
  return (
    raw.split("/").filter(Boolean).pop() || raw || "hunt-custom-marker.yaml"
  );
};

const makeSavingPath = (placement: NucleiTemplatePlacement, name: string) => {
  const cleanName = name.trim() || "template.yaml";
  if (placement === "root") return `/data/nuclei/custom/${cleanName}`;
  return `/data/nuclei/custom/${placement}/${cleanName}`;
};

const validationState = (validation: NucleiTemplateValidation | null) => {
  if (!validation) {
    return {
      title: "Not validated",
      icon: ShieldCheck,
      className: "border-hack-border bg-black/20 text-hack-dim",
    };
  }

  if (validation.valid) {
    return {
      title: "Valid template",
      icon: CheckCircle2,
      className:
        "border-hack-primary/40 bg-hack-primary/[0.06] text-hack-primary",
    };
  }

  return {
    title: "Validation failed",
    icon: AlertCircle,
    className: "border-hack-danger/50 bg-hack-danger/[0.06] text-hack-danger",
  };
};

const buildDraftPayload = (
  templateName: string,
  strategy?: NucleiTemplateStrategy | null,
): GenerateNucleiTemplateDraftPayload => ({
  name: templateName || "hunt-ai-draft.yaml",
  title: "Hunt AI Draft Template",
  description:
    "Draft-only Nuclei template candidate. Review and validate before saving.",
  severity: "info",
  tags: strategy?.recommended_tags?.length
    ? strategy.recommended_tags.slice(0, 6)
    : ["exposure", "panel"],
  method: "GET",
  path: "/",
  matcher_type: "word",
  matcher_part: "body",
  matcher_value: "HUNT_TARGET_MARKER",
  validate: true,
});

const compactList = (values?: string[] | number[]) => {
  if (!values?.length) return "-";
  return values.join(", ");
};

const NucleiTemplates = () => {
  const { role } = useAuth();
  const queryClient = useQueryClient();

  const [selectedName, setSelectedName] = useState("");
  const [templateName, setTemplateName] = useState("hunt-custom-marker.yaml");
  const [placement, setPlacement] = useState<NucleiTemplatePlacement>("fast");
  const [content, setContent] = useState(defaultTemplate);
  const [validation, setValidation] = useState<NucleiTemplateValidation | null>(
    null,
  );
  const [search, setSearch] = useState("");
  const [placementFilter, setPlacementFilter] = useState<
    "all" | NucleiTemplatePlacement
  >("all");
  const [targetId, setTargetId] = useState("");
  const [strategy, setStrategy] = useState<NucleiTemplateStrategy | null>(null);
  const [draft, setDraft] = useState<NucleiTemplateDraftResponse | null>(null);

  const templatesQuery = useQuery({
    queryKey: ["nuclei-templates"],
    queryFn: listNucleiTemplates,
  });

  const draftStatusQuery = useQuery({
    queryKey: ["nuclei-template-draft-status"],
    queryFn: getNucleiTemplateDraftStatus,
    retry: false,
  });

  const templates = useMemo(
    () => templatesQuery.data || [],
    [templatesQuery.data],
  );

  const filteredTemplates = useMemo(() => {
    const query = search.trim().toLowerCase();

    return templates.filter((template) => {
      const templatePlacement = inferPlacement(template);
      if (placementFilter !== "all" && templatePlacement !== placementFilter) {
        return false;
      }
      if (!query) return true;
      return [template.name, template.path, templatePlacement]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(query);
    });
  }, [placementFilter, search, templates]);

  const selectedTemplate = useMemo(
    () => templates.find((template) => template.name === selectedName),
    [selectedName, templates],
  );

  const currentPlacement =
    placementMap.get(placement) || placementMap.get("root")!;
  const currentValidation = validationState(validation);
  const ValidationIcon = currentValidation.icon;
  const draftStatus = draftStatusQuery.data;

  const loadMutation = useMutation({
    mutationFn: getNucleiTemplate,
    onSuccess: (template) => {
      const nextPlacement = inferPlacement(template);
      setSelectedName(template.name);
      setTemplateName(templateFileName(template));
      setPlacement(nextPlacement);
      setContent(template.content || "");
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
        toast.success("Template validation passed");
      } else {
        toast.error(result.error || "Template validation failed");
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
      toast.success("Template saved");
      setSelectedName(template.name);
      setTemplateName(templateFileName(template));
      setPlacement(nextPlacement);
      setContent(template.content || content);
      setValidation({ valid: true, name: template.name });
      queryClient.invalidateQueries({ queryKey: ["nuclei-templates"] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const deleteMutation = useMutation({
    mutationFn: deleteNucleiTemplate,
    onSuccess: () => {
      toast.success("Template deleted");
      startNewTemplate();
      queryClient.invalidateQueries({ queryKey: ["nuclei-templates"] });
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const strategyMutation = useMutation({
    mutationFn: () => getNucleiTemplateStrategy(targetId.trim()),
    onSuccess: (result) => {
      setStrategy(result);
      setDraft(null);
      toast.success("Strategy loaded");
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const draftMutation = useMutation({
    mutationFn: () =>
      strategy?.suggested_draft_request
        ? generateNucleiTemplateDraft({
            ...strategy.suggested_draft_request,
            validate: true,
          })
        : generateNucleiTemplateDraft(
            buildDraftPayload(templateName, strategy),
          ),
    onSuccess: (result) => {
      setDraft(result);
      toast.success("Draft generated");
    },
    onError: (error) => toast.error(getErrorMessage(error)),
  });

  const startNewTemplate = () => {
    setSelectedName("");
    setTemplateName("hunt-custom-marker.yaml");
    setPlacement("fast");
    setContent(defaultTemplate);
    setValidation(null);
  };

  const handleDelete = () => {
    if (!selectedName) return;
    if (!confirm(`Delete Nuclei template "${selectedName}"?`)) return;
    deleteMutation.mutate(selectedName);
  };

  const loadDraftInEditor = () => {
    if (!draft) return;
    setSelectedName("");
    setTemplateName(templateFileName({ name: draft.name }));
    setPlacement(
      (strategy?.recommended_placements?.[0] as
        | NucleiTemplatePlacement
        | undefined) || "fast",
    );
    setContent(draft.content);
    setValidation(draft.validation || null);
  };

  if (role !== "admin") return <Navigate to="/" replace />;

  const isBusy =
    templatesQuery.isLoading ||
    loadMutation.isPending ||
    validateMutation.isPending ||
    saveMutation.isPending ||
    deleteMutation.isPending;

  const editorStatus = selectedTemplate
    ? `${formatBytes(selectedTemplate.size_bytes)} | ${formatDate(selectedTemplate.updated_at)}`
    : "New unsaved template";

  return (
    <div className="space-y-6 pb-8">
      <header className="flex flex-col gap-4 border-b border-hack-border/60 pb-5 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <p className="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-[0.28em] text-hack-primary">
            <ShieldCheck className="h-4 w-4" /> Nuclei Security Engine
          </p>
          <h1 className="hack-title text-3xl">Nuclei Templates</h1>
          <p className="mt-2 max-w-3xl text-sm leading-6 text-hack-dim">
            Manage custom YAML templates, choose their scan profile placement,
            and keep AI drafts behind manual approval.
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <button
            onClick={() => templatesQuery.refetch()}
            className="hack-btn-ghost border border-hack-border/70 bg-black/20"
            disabled={isBusy}
          >
            <RefreshCw className="h-4 w-4" /> Refresh
          </button>
          <button onClick={startNewTemplate} className="hack-btn">
            <Plus className="h-4 w-4" /> New Template
          </button>
        </div>
      </header>

      <div className="grid gap-6 xl:grid-cols-[360px_minmax(0,1fr)]">
        <section className="hack-box rounded-lg p-4">
          <div className="mb-4 flex items-start justify-between gap-3">
            <div>
              <h2 className="text-sm font-bold uppercase tracking-[0.18em] text-hack-text">
                Template Library
              </h2>
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
                className="hack-input w-full rounded-md pl-9 text-hack-text"
                placeholder="Search templates"
                spellCheck={false}
              />
            </label>
            <select
              value={placementFilter}
              onChange={(event) =>
                setPlacementFilter(
                  event.target.value as "all" | NucleiTemplatePlacement,
                )
              }
              className="hack-input w-full rounded-md text-hack-text"
            >
              <option value="all">All placements</option>
              {placementOptions.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </div>

          <div className="mt-4 max-h-[600px] space-y-2 overflow-y-auto pr-1">
            {templatesQuery.isLoading ? (
              <div className="flex items-center gap-2 rounded-md border border-hack-border/60 p-4 text-sm text-hack-dim">
                <Loader2 className="h-4 w-4 animate-spin" /> Loading
                templates...
              </div>
            ) : filteredTemplates.length === 0 ? (
              <div className="rounded-md border border-dashed border-hack-border/70 p-6 text-center text-sm text-hack-dim">
                No templates found.
              </div>
            ) : (
              filteredTemplates.map((template: NucleiTemplate) => {
                const itemPlacement = inferPlacement(template);
                const option = placementMap.get(itemPlacement);
                const active = selectedName === template.name;

                return (
                  <button
                    key={`${template.path || template.name}-${template.name}`}
                    onClick={() => loadMutation.mutate(template.name)}
                    className={`w-full rounded-md border p-3 text-left transition-all ${
                      active
                        ? "border-hack-primary/60 bg-hack-primary/[0.08]"
                        : "border-hack-border/60 bg-black/20 hover:border-hack-primary/40 hover:bg-white/[0.03]"
                    }`}
                  >
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <p className="truncate text-sm font-bold text-hack-primary">
                          {templateFileName(template)}
                        </p>
                        <p className="mt-1 truncate text-xs text-hack-dim">
                          {template.path || template.name}
                        </p>
                      </div>
                      <span className="hack-badge shrink-0 border-hack-border/70 text-hack-dim">
                        {option?.label || itemPlacement}
                      </span>
                    </div>
                    <p className="mt-2 text-xs text-hack-dim">
                      {formatBytes(template.size_bytes)} |{" "}
                      {formatDate(template.updated_at)}
                    </p>
                  </button>
                );
              })
            )}
          </div>
        </section>

        <section className="hack-box rounded-lg p-4 lg:p-5">
          <div className="mb-5 flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
            <div>
              <h2 className="text-sm font-bold uppercase tracking-[0.18em] text-hack-text">
                Template Editor
              </h2>
              <p className="mt-1 text-xs text-hack-dim">{editorStatus}</p>
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                onClick={() => validateMutation.mutate()}
                className="hack-btn-ghost border border-hack-border/70 bg-black/20"
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
                className="hack-btn"
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
                className="hack-btn-danger"
                disabled={!selectedName || deleteMutation.isPending}
              >
                <Trash2 className="h-4 w-4" /> Delete
              </button>
            </div>
          </div>

          <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_260px]">
            <div className="space-y-4">
              <div className="grid gap-4 md:grid-cols-2">
                <label className="block space-y-2">
                  <span className="text-xs font-bold uppercase tracking-[0.16em] text-hack-dim">
                    Template filename
                  </span>
                  <input
                    value={templateName}
                    onChange={(event) => {
                      setTemplateName(event.target.value);
                      setValidation(null);
                    }}
                    className="hack-input h-11 w-full rounded-md font-mono text-hack-text"
                    placeholder="example.yaml"
                    spellCheck={false}
                  />
                </label>

                <label className="block space-y-2">
                  <span className="text-xs font-bold uppercase tracking-[0.16em] text-hack-dim">
                    Profile placement
                  </span>
                  <select
                    value={placement}
                    onChange={(event) => {
                      setPlacement(
                        event.target.value as NucleiTemplatePlacement,
                      );
                      setValidation(null);
                    }}
                    className="hack-input h-11 w-full rounded-md text-hack-text"
                  >
                    {placementOptions.map((option) => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>

              <div
                className={`rounded-md border p-3 text-sm ${currentValidation.className}`}
              >
                <div className="flex items-center gap-2 font-bold uppercase tracking-[0.14em]">
                  <ValidationIcon className="h-4 w-4" />{" "}
                  {currentValidation.title}
                </div>
                {(validation?.error || validation?.output) && (
                  <pre className="mt-3 max-h-40 overflow-auto whitespace-pre-wrap text-xs leading-5">
                    {validation.error || validation.output}
                  </pre>
                )}
              </div>

              <label className="block space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <span className="text-xs font-bold uppercase tracking-[0.16em] text-hack-dim">
                    YAML content
                  </span>
                  <span className="text-xs text-hack-dim">
                    {content.split("\\n").length} lines
                  </span>
                </div>
                <textarea
                  value={content}
                  onChange={(event) => {
                    setContent(event.target.value);
                    setValidation(null);
                  }}
                  className="hack-input min-h-[520px] w-full resize-y rounded-md bg-black/35 p-4 font-mono text-sm leading-6 text-hack-primary"
                  spellCheck={false}
                />
              </label>
            </div>

            <aside className="space-y-4">
              <div className="rounded-md border border-hack-border/70 bg-black/20 p-4">
                <p className="text-xs font-bold uppercase tracking-[0.16em] text-hack-dim">
                  Saving path
                </p>
                <p className="mt-2 break-all font-mono text-xs text-hack-primary">
                  {makeSavingPath(placement, templateName)}
                </p>
                <div className="mt-4 border-t border-hack-border/50 pt-4">
                  <p className="text-sm font-bold text-hack-text">
                    {currentPlacement.label}
                  </p>
                  <p className="mt-1 text-xs leading-5 text-hack-dim">
                    {currentPlacement.description}
                  </p>
                  <p className="mt-3 text-xs text-hack-dim">
                    Runs in{" "}
                    <span className="text-hack-primary">
                      {currentPlacement.runsIn}
                    </span>
                  </p>
                </div>
              </div>

              <details
                className="rounded-md border border-hack-border/70 bg-black/20 p-4"
                open={false}
              >
                <summary className="cursor-pointer list-none text-xs font-bold uppercase tracking-[0.16em] text-hack-text">
                  AI draft and strategy
                </summary>

                <div className="mt-4 space-y-4">
                  <div className="rounded-md border border-hack-border/60 p-3 text-xs text-hack-dim">
                    <div className="flex items-center justify-between gap-3">
                      <span>Draft foundation</span>
                      <span
                        className={
                          draftStatus?.enabled
                            ? "text-hack-primary"
                            : "text-hack-dim"
                        }
                      >
                        {draftStatusQuery.isLoading
                          ? "checking"
                          : draftStatus?.enabled
                            ? "enabled"
                            : "disabled"}
                      </span>
                    </div>
                    <p className="mt-2 leading-5">
                      Auto-save and auto-execute stay disabled. Drafts require
                      review, validation, and manual save.
                    </p>
                    {draftStatus?.disabled_reason && (
                      <p className="mt-2 leading-5 text-hack-warning">
                        Disabled reason: {draftStatus.disabled_reason}
                      </p>
                    )}
                    {draftStatus?.scope && (
                      <p className="mt-2 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                        Scope: {draftStatus.scope} · Owner:{" "}
                        {draftStatus.owner_key || "-"}
                      </p>
                    )}
                    {draftStatus?.disabled_reason && (
                      <p className="mt-2 leading-5 text-hack-warning">
                        Disabled reason: {draftStatus.disabled_reason}
                      </p>
                    )}
                    {draftStatus?.scope && (
                      <p className="mt-2 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                        Scope: {draftStatus.scope} · Owner:{" "}
                        {draftStatus.owner_key || "-"}
                      </p>
                    )}
                  </div>

                  <label className="block space-y-2">
                    <span className="text-xs font-bold uppercase tracking-[0.16em] text-hack-dim">
                      Target ID
                    </span>
                    <input
                      value={targetId}
                      onChange={(event) => setTargetId(event.target.value)}
                      className="hack-input h-10 w-full rounded-md text-hack-text"
                      placeholder="e.g. 12"
                    />
                  </label>

                  <div className="flex flex-wrap gap-2">
                    <button
                      onClick={() => strategyMutation.mutate()}
                      className="hack-btn-ghost border border-hack-border/70 bg-black/20"
                      disabled={
                        !draftStatus?.enabled ||
                        !targetId.trim() ||
                        strategyMutation.isPending
                      }
                      title={
                        draftStatus?.enabled
                          ? "Generate target strategy"
                          : "AI template drafts are disabled by feature flag"
                      }
                    >
                      {strategyMutation.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Wand2 className="h-4 w-4" />
                      )}
                      Strategy
                    </button>
                    <button
                      onClick={() => draftMutation.mutate()}
                      className="hack-btn-ghost border border-hack-border/70 bg-black/20"
                      disabled={
                        !draftStatus?.enabled || draftMutation.isPending
                      }
                      title={
                        draftStatus?.enabled
                          ? "Generate draft"
                          : "AI template drafts are disabled by feature flag"
                      }
                    >
                      {draftMutation.isPending ? (
                        <Loader2 className="h-4 w-4 animate-spin" />
                      ) : (
                        <Wand2 className="h-4 w-4" />
                      )}
                      Draft
                    </button>
                  </div>

                  {strategy && (
                    <div className="rounded-md border border-hack-border/60 p-3 text-xs text-hack-dim">
                      <p className="font-bold uppercase tracking-[0.14em] text-hack-text">
                        Recommendation
                      </p>
                      <dl className="mt-3 space-y-2">
                        <div>
                          <dt className="text-hack-dim">Profile</dt>
                          <dd className="text-hack-primary">
                            {strategy.recommended_profile}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-hack-dim">Placements</dt>
                          <dd className="text-hack-primary">
                            {compactList(strategy.recommended_placements)}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-hack-dim">Tags</dt>
                          <dd>{compactList(strategy.recommended_tags)}</dd>
                        </div>
                      </dl>
                      {strategy.rationale?.length > 0 && (
                        <ul className="mt-3 list-disc space-y-1 pl-4 leading-5">
                          {strategy.rationale.slice(0, 3).map((item) => (
                            <li key={item}>{item}</li>
                          ))}
                        </ul>
                      )}
                    </div>
                  )}

                  {draft && (
                    <div className="rounded-md border border-hack-primary/40 bg-hack-primary/[0.04] p-3 text-xs text-hack-dim">
                      <p className="font-bold uppercase tracking-[0.14em] text-hack-primary">
                        Draft ready
                      </p>
                      <p className="mt-2 break-all font-mono">{draft.name}</p>
                      <button
                        onClick={loadDraftInEditor}
                        className="hack-btn mt-3 w-full"
                      >
                        Load in editor
                      </button>
                    </div>
                  )}
                </div>
              </details>
            </aside>
          </div>
        </section>
      </div>
    </div>
  );
};

export default NucleiTemplates;
