import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import {
  BrainCircuit,
  Edit3,
  Plus,
  RefreshCw,
  Search,
  ShieldCheck,
  Trash2,
  X,
} from "lucide-react";
import {
  createOperatorLearningRecord,
  deleteOperatorLearningRecord,
  listOperatorLearningRecords,
  updateOperatorLearningRecord,
} from "../api/operatorLearning";
import type {
  CreateOperatorLearningRecordPayload,
  OperatorLearningRecord,
  OperatorLearningScope,
  OperatorLearningStatus,
} from "../api/operatorLearning";

const scopeOptions: Array<{ label: string; value: OperatorLearningScope | "" }> = [
  { label: "All scopes", value: "" },
  { label: "User Global", value: "user_global" },
  { label: "Project", value: "project" },
  { label: "Target", value: "target" },
];

const editableScopeOptions: Array<{ label: string; value: OperatorLearningScope }> = [
  { label: "User Global", value: "user_global" },
  { label: "Project", value: "project" },
  { label: "Target", value: "target" },
];

const statusOptions: Array<{ label: string; value: OperatorLearningStatus | "" }> = [
  { label: "Active", value: "active" },
  { label: "All statuses", value: "" },
  { label: "Disabled", value: "disabled" },
  { label: "Superseded", value: "superseded" },
];

function formatDate(value?: string | null) {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function parseJsonArray(value: string): unknown[] {
  const trimmed = value.trim();
  if (!trimmed) return [];
  const parsed = JSON.parse(trimmed);
  if (!Array.isArray(parsed)) {
    throw new Error("Expected a JSON array");
  }
  return parsed;
}

function parseJsonObject(value: string): Record<string, unknown> {
  const trimmed = value.trim();
  if (!trimmed) return {};
  const parsed = JSON.parse(trimmed);
  if (!parsed || typeof parsed !== "object" || Array.isArray(parsed)) {
    throw new Error("Expected a JSON object");
  }
  return parsed as Record<string, unknown>;
}

function stringifyJson(value: unknown, fallback: string) {
  try {
    return JSON.stringify(value ?? JSON.parse(fallback), null, 2);
  } catch {
    return fallback;
  }
}

interface LearningFormState {
  title: string;
  scope: OperatorLearningScope;
  status: OperatorLearningStatus;
  bugClass: string;
  skillSlug: string;
  summary: string;
  content: string;
  confidence: number;
  appliesTo: string;
  triggerSignals: string;
  methodology: string;
  executionHints: string;
}

function defaultFormState(): LearningFormState {
  return {
    title: "",
    scope: "user_global",
    status: "active",
    bugClass: "",
    skillSlug: "",
    summary: "",
    content: "",
    confidence: 80,
    appliesTo: '["query_params"]',
    triggerSignals: '["reflection"]',
    methodology: '{"steps":["observe","classify","validate","capture_evidence"]}',
    executionHints: '{"permission_mode":"scope_aware_authorized"}',
  };
}

function formStateFromRecord(record: OperatorLearningRecord): LearningFormState {
  return {
    title: record.title || "",
    scope: record.scope || "user_global",
    status: record.status || "active",
    bugClass: record.bug_class || "",
    skillSlug: record.skill_slug || "",
    summary: record.summary || "",
    content: record.content || "",
    confidence: record.confidence || 80,
    appliesTo: stringifyJson(record.applies_to || [], "[]"),
    triggerSignals: stringifyJson(record.trigger_signals || [], "[]"),
    methodology: stringifyJson(record.methodology || {}, "{}"),
    executionHints: stringifyJson(record.execution_hints || {}, "{}"),
  };
}

function payloadFromFormState(form: LearningFormState): CreateOperatorLearningRecordPayload {
  return {
    scope: form.scope,
    source: "user_confirmed",
    status: form.status,
    title: form.title.trim(),
    summary: form.summary.trim(),
    content: form.content.trim(),
    bug_class: form.bugClass.trim(),
    skill_slug: form.skillSlug.trim(),
    applies_to: parseJsonArray(form.appliesTo),
    trigger_signals: parseJsonArray(form.triggerSignals),
    methodology: parseJsonObject(form.methodology),
    execution_hints: parseJsonObject(form.executionHints),
    confidence: form.confidence,
  };
}

export default function OperatorLearning() {
  const [scope, setScope] = useState<OperatorLearningScope | "">("");
  const [status, setStatus] = useState<OperatorLearningStatus | "active" | "">("active");
  const [bugClass, setBugClass] = useState("");
  const [skillSlug, setSkillSlug] = useState("");

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingRecord, setEditingRecord] = useState<OperatorLearningRecord | null>(null);
  const [form, setForm] = useState<LearningFormState>(defaultFormState());
  const [formError, setFormError] = useState("");

  const params = useMemo(
    () => ({
      scope,
      status,
      bug_class: bugClass.trim(),
      skill_slug: skillSlug.trim(),
      limit: 100,
    }),
    [scope, status, bugClass, skillSlug]
  );

  const query = useQuery({
    queryKey: ["operator-learning", params],
    queryFn: () => listOperatorLearningRecords(params),
  });

  const createMutation = useMutation({
    mutationFn: createOperatorLearningRecord,
    onSuccess: () => {
      closeModal();
      void query.refetch();
    },
    onError: () => {
      setFormError("Failed to create methodology record.");
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: CreateOperatorLearningRecordPayload }) =>
      updateOperatorLearningRecord(id, payload),
    onSuccess: () => {
      closeModal();
      void query.refetch();
    },
    onError: () => {
      setFormError("Failed to update methodology record.");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteOperatorLearningRecord,
    onSuccess: () => {
      void query.refetch();
    },
  });

  const records = query.data?.learning ?? [];

  const openCreateModal = () => {
    setEditingRecord(null);
    setForm(defaultFormState());
    setFormError("");
    setIsModalOpen(true);
  };

  const openEditModal = (record: OperatorLearningRecord) => {
    setEditingRecord(record);
    setForm(formStateFromRecord(record));
    setFormError("");
    setIsModalOpen(true);
  };

  function closeModal() {
    setIsModalOpen(false);
    setEditingRecord(null);
    setForm(defaultFormState());
    setFormError("");
  }

  const updateForm = <K extends keyof LearningFormState>(key: K, value: LearningFormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError("");

    if (!form.title.trim()) {
      setFormError("Title is required.");
      return;
    }

    try {
      const payload = payloadFromFormState(form);
      if (editingRecord) {
        updateMutation.mutate({ id: editingRecord.id, payload });
      } else {
        createMutation.mutate(payload);
      }
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Invalid JSON field.");
    }
  };

  const handleDelete = (record: OperatorLearningRecord) => {
    const ok = window.confirm(
      `Delete methodology record "${record.title}"? This removes it from normal lists and target profile selectors.`
    );
    if (!ok) return;
    deleteMutation.mutate(record.id);
  };

  const submitting = createMutation.isPending || updateMutation.isPending;

  return (
    <div className="space-y-6">
      <div className="rounded-2xl border border-hack-primary/20 bg-hack-surface/80 p-6 shadow-lg">
        <div className="flex flex-col gap-4 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <div className="flex items-center gap-3">
              <div className="rounded-xl bg-hack-primary/10 p-3 text-hack-primary">
                <BrainCircuit className="h-6 w-6" />
              </div>
              <div>
                <h1 className="text-2xl font-bold text-white">
                  Operator Methodology / Skill Instructions
                </h1>
                <p className="mt-1 max-w-3xl text-sm text-gray-400">
                  User-authored methodology records that guide executable pentest skills,
                  payload strategy, validation flow, and evidence-driven operator reasoning.
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={openCreateModal}
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-hack-primary px-4 py-2 text-sm font-semibold text-black transition hover:bg-hack-primary/90"
            >
              <Plus className="h-4 w-4" />
              Add Methodology
            </button>
            <button
              type="button"
              onClick={() => query.refetch()}
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-hack-primary/30 px-4 py-2 text-sm font-medium text-hack-primary transition hover:bg-hack-primary/10"
            >
              <RefreshCw className={`h-4 w-4 ${query.isFetching ? "animate-spin" : ""}`} />
              Refresh
            </button>
          </div>
        </div>

        <div className="mt-6 grid gap-3 md:grid-cols-4">
          <label className="space-y-1">
            <span className="text-xs uppercase tracking-wide text-gray-500">Scope</span>
            <select
              value={scope}
              onChange={(event) => setScope(event.target.value as OperatorLearningScope | "")}
              className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none transition focus:border-hack-primary focus:bg-black/80"
            >
              {scopeOptions.map((option) => (
                <option key={option.label} value={option.value} className="bg-black text-white">
                  {option.label}
                </option>
              ))}
            </select>
          </label>

          <label className="space-y-1">
            <span className="text-xs uppercase tracking-wide text-gray-500">Status</span>
            <select
              value={status}
              onChange={(event) => setStatus(event.target.value as OperatorLearningStatus | "")}
              className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none transition focus:border-hack-primary focus:bg-black/80"
            >
              {statusOptions.map((option) => (
                <option key={option.label} value={option.value} className="bg-black text-white">
                  {option.label}
                </option>
              ))}
            </select>
          </label>

          <label className="space-y-1">
            <span className="text-xs uppercase tracking-wide text-gray-500">Bug class</span>
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-gray-500" />
              <input
                value={bugClass}
                onChange={(event) => setBugClass(event.target.value)}
                placeholder="xss, idor, ssrf..."
                className="w-full rounded-lg border border-hack-primary/25 bg-black/60 py-2 pl-9 pr-3 text-sm text-white outline-none transition placeholder:text-gray-600 focus:border-hack-primary focus:bg-black/80"
              />
            </div>
          </label>

          <label className="space-y-1">
            <span className="text-xs uppercase tracking-wide text-gray-500">Skill slug</span>
            <input
              value={skillSlug}
              onChange={(event) => setSkillSlug(event.target.value)}
              placeholder="xss_reflection"
              className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none transition placeholder:text-gray-600 focus:border-hack-primary focus:bg-black/80"
            />
          </label>
        </div>
      </div>

      <div className="rounded-2xl border border-hack-primary/20 bg-hack-surface/80 shadow-lg">
        <div className="flex items-center justify-between border-b border-hack-primary/10 px-6 py-4">
          <div>
            <h2 className="text-lg font-semibold text-white">Methodology Records</h2>
            <p className="text-sm text-gray-500">
              {query.isLoading ? "Loading..." : `${query.data?.count ?? 0} records`}
            </p>
          </div>
          <div className="flex items-center gap-2 rounded-full bg-hack-primary/10 px-3 py-1 text-xs text-hack-primary">
            <ShieldCheck className="h-4 w-4" />
            Per-user
          </div>
        </div>

        {query.isError ? (
          <div className="p-6 text-sm text-red-400">Failed to load methodology records.</div>
        ) : query.isLoading ? (
          <div className="p-6 text-sm text-gray-400">Loading methodology records...</div>
        ) : records.length === 0 ? (
          <div className="p-10 text-center">
            <BrainCircuit className="mx-auto h-10 w-10 text-gray-600" />
            <h3 className="mt-4 text-lg font-semibold text-white">No methodology records yet</h3>
            <p className="mx-auto mt-2 max-w-2xl text-sm text-gray-500">
              Add reusable pentest methodology, payload strategy, execution hints, and
              exploit-chain preferences that can guide executable skills per target.
            </p>
          </div>
        ) : (
          <div className="divide-y divide-hack-primary/10">
            {records.map((record) => (
              <article key={record.id} className="p-6">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="text-base font-semibold text-white">{record.title}</h3>
                      <span className="rounded-full bg-hack-primary/10 px-2 py-0.5 text-xs text-hack-primary">
                        {record.scope}
                      </span>
                      <span className="rounded-full bg-gray-800 px-2 py-0.5 text-xs text-gray-300">
                        {record.source}
                      </span>
                      <span className="rounded-full bg-gray-800 px-2 py-0.5 text-xs text-gray-300">
                        {record.status}
                      </span>
                    </div>
                    {record.summary && (
                      <p className="mt-2 text-sm text-gray-400">{record.summary}</p>
                    )}
                    {record.content && (
                      <p className="mt-2 whitespace-pre-wrap text-sm text-gray-500">
                        {record.content}
                      </p>
                    )}
                  </div>

                  <div className="flex min-w-[220px] flex-col gap-3">
                    <div className="rounded-xl border border-hack-primary/10 bg-black/50 p-3 text-xs text-gray-400">
                      <div className="flex justify-between">
                        <span>Confidence</span>
                        <span className="text-white">{record.confidence}%</span>
                      </div>
                      <div className="mt-1 flex justify-between">
                        <span>Used</span>
                        <span className="text-white">{record.use_count}</span>
                      </div>
                      <div className="mt-1 flex justify-between">
                        <span>Last used</span>
                        <span className="text-right text-white">{formatDate(record.last_used_at)}</span>
                      </div>
                    </div>

                    <div className="flex gap-2">
                      <button
                        type="button"
                        onClick={() => openEditModal(record)}
                        className="inline-flex flex-1 items-center justify-center gap-2 rounded-lg border border-hack-primary/30 px-3 py-2 text-xs font-medium text-hack-primary transition hover:bg-hack-primary/10"
                      >
                        <Edit3 className="h-4 w-4" />
                        Edit
                      </button>
                      <button
                        type="button"
                        onClick={() => handleDelete(record)}
                        disabled={deleteMutation.isPending}
                        className="inline-flex flex-1 items-center justify-center gap-2 rounded-lg border border-red-500/30 px-3 py-2 text-xs font-medium text-red-300 transition hover:bg-red-500/10 disabled:opacity-50"
                      >
                        <Trash2 className="h-4 w-4" />
                        Delete
                      </button>
                    </div>
                  </div>
                </div>

                <div className="mt-4 flex flex-wrap gap-2 text-xs">
                  {record.bug_class && (
                    <span className="rounded bg-blue-500/10 px-2 py-1 text-blue-300">
                      bug: {record.bug_class}
                    </span>
                  )}
                  {record.skill_slug && (
                    <span className="rounded bg-purple-500/10 px-2 py-1 text-purple-300">
                      skill: {record.skill_slug}
                    </span>
                  )}
                  {record.target_id && (
                    <span className="rounded bg-orange-500/10 px-2 py-1 text-orange-300">
                      target: {record.target_id}
                    </span>
                  )}
                  {record.project_key && (
                    <span className="rounded bg-emerald-500/10 px-2 py-1 text-emerald-300">
                      project: {record.project_key}
                    </span>
                  )}
                </div>
              </article>
            ))}
          </div>
        )}
      </div>

      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4 backdrop-blur-sm">
          <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-2xl border border-hack-primary/40 bg-black p-6 shadow-2xl">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <h2 className="text-xl font-bold text-white">
                  {editingRecord ? "Edit Methodology" : "Add Methodology"}
                </h2>
                <p className="mt-1 text-sm text-gray-400">
                  Write user methodology and execution hints that guide executable skills.
                </p>
              </div>
              <button
                type="button"
                onClick={closeModal}
                className="rounded-lg p-2 text-gray-400 hover:bg-white/5 hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-4">
              {formError && (
                <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                  {formError}
                </div>
              )}

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Title</span>
                  <input
                    value={form.title}
                    onChange={(event) => updateForm("title", event.target.value)}
                    placeholder="XSS context-first methodology"
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Scope</span>
                  <select
                    value={form.scope}
                    onChange={(event) => updateForm("scope", event.target.value as OperatorLearningScope)}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {editableScopeOptions.map((option) => (
                      <option key={option.value} value={option.value} className="bg-black text-white">
                        {option.label}
                      </option>
                    ))}
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Status</span>
                  <select
                    value={form.status}
                    onChange={(event) => updateForm("status", event.target.value as OperatorLearningStatus)}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    <option className="bg-black text-white" value="active">Active</option>
                    <option className="bg-black text-white" value="disabled">Disabled</option>
                    <option className="bg-black text-white" value="superseded">Superseded</option>
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Confidence: {form.confidence}%</span>
                  <input
                    type="range"
                    min={1}
                    max={100}
                    value={form.confidence}
                    onChange={(event) => updateForm("confidence", Number(event.target.value))}
                    className="w-full"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Bug class</span>
                  <input
                    value={form.bugClass}
                    onChange={(event) => updateForm("bugClass", event.target.value)}
                    placeholder="xss"
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Skill slug</span>
                  <input
                    value={form.skillSlug}
                    onChange={(event) => updateForm("skillSlug", event.target.value)}
                    placeholder="xss_reflection"
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                  />
                </label>
              </div>

              <label className="block space-y-1">
                <span className="text-xs uppercase tracking-wide text-gray-500">Summary</span>
                <input
                  value={form.summary}
                  onChange={(event) => updateForm("summary", event.target.value)}
                  placeholder="Classify reflection context before payload execution."
                  className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                />
              </label>

              <label className="block space-y-1">
                <span className="text-xs uppercase tracking-wide text-gray-500">Content</span>
                <textarea
                  value={form.content}
                  onChange={(event) => updateForm("content", event.target.value)}
                  rows={4}
                  placeholder="Write the methodology the operator should remember..."
                  className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                />
              </label>

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Applies to JSON array</span>
                  <textarea
                    value={form.appliesTo}
                    onChange={(event) => updateForm("appliesTo", event.target.value)}
                    rows={3}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Trigger signals JSON array</span>
                  <textarea
                    value={form.triggerSignals}
                    onChange={(event) => updateForm("triggerSignals", event.target.value)}
                    rows={3}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Methodology JSON object</span>
                  <textarea
                    value={form.methodology}
                    onChange={(event) => updateForm("methodology", event.target.value)}
                    rows={4}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Execution hints JSON object</span>
                  <textarea
                    value={form.executionHints}
                    onChange={(event) => updateForm("executionHints", event.target.value)}
                    rows={4}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>
              </div>

              <div className="flex flex-wrap justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={closeModal}
                  className="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 hover:bg-white/5"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={submitting}
                  className="rounded-lg bg-hack-primary px-4 py-2 text-sm font-semibold text-black hover:bg-hack-primary/90 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {submitting ? "Saving..." : editingRecord ? "Save Changes" : "Save Methodology"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
