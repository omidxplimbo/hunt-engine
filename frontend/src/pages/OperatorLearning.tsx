import { useMemo, useState } from "react";
import type { FormEvent } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { BrainCircuit, Plus, RefreshCw, Search, ShieldCheck, X } from "lucide-react";
import { createOperatorLearningRecord, listOperatorLearningRecords } from "../api/operatorLearning";
import type {
  OperatorLearningScope,
  OperatorLearningStatus,
} from "../api/operatorLearning";

const scopeOptions: Array<{ label: string; value: OperatorLearningScope | "" }> = [
  { label: "All scopes", value: "" },
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

export default function OperatorLearning() {
  const [scope, setScope] = useState<OperatorLearningScope | "">("");
  const [status, setStatus] = useState<OperatorLearningStatus | "active" | "">("active");
  const [bugClass, setBugClass] = useState("");
  const [skillSlug, setSkillSlug] = useState("");

  const [isCreateOpen, setIsCreateOpen] = useState(false);
  const [createError, setCreateError] = useState("");
  const [newTitle, setNewTitle] = useState("");
  const [newScope, setNewScope] = useState<OperatorLearningScope>("user_global");
  const [newBugClass, setNewBugClass] = useState("");
  const [newSkillSlug, setNewSkillSlug] = useState("");
  const [newSummary, setNewSummary] = useState("");
  const [newContent, setNewContent] = useState("");
  const [newConfidence, setNewConfidence] = useState(80);
  const [newAppliesTo, setNewAppliesTo] = useState('["query_params"]');
  const [newTriggerSignals, setNewTriggerSignals] = useState('["reflection"]');
  const [newMethodology, setNewMethodology] = useState('{"steps":["observe","classify","validate","capture_evidence"]}');
  const [newExecutionHints, setNewExecutionHints] = useState('{"permission_mode":"scope_aware_authorized"}');

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
      setIsCreateOpen(false);
      setCreateError("");
      setNewTitle("");
      setNewBugClass("");
      setNewSkillSlug("");
      setNewSummary("");
      setNewContent("");
      void query.refetch();
    },
    onError: () => {
      setCreateError("Failed to create operator learning record.");
    },
  });

  const handleCreate = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setCreateError("");

    if (!newTitle.trim()) {
      setCreateError("Title is required.");
      return;
    }

    try {
      createMutation.mutate({
        scope: newScope,
        source: "user_confirmed",
        status: "active",
        title: newTitle.trim(),
        summary: newSummary.trim(),
        content: newContent.trim(),
        bug_class: newBugClass.trim(),
        skill_slug: newSkillSlug.trim(),
        applies_to: parseJsonArray(newAppliesTo),
        trigger_signals: parseJsonArray(newTriggerSignals),
        methodology: parseJsonObject(newMethodology),
        execution_hints: parseJsonObject(newExecutionHints),
        confidence: newConfidence,
      });
    } catch (error) {
      setCreateError(error instanceof Error ? error.message : "Invalid JSON field.");
    }
  };

  const records = query.data?.learning ?? [];

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
                <h1 className="text-2xl font-bold text-white">Operator Learning</h1>
                <p className="mt-1 max-w-3xl text-sm text-gray-400">
                  Personal, project, and target-scoped methodology that can guide the AI pentest
                  operator across authorized validation, exploitation, skill selection, and evidence
                  learning workflows.
                </p>
              </div>
            </div>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => setIsCreateOpen(true)}
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-hack-primary px-4 py-2 text-sm font-semibold text-black transition hover:bg-hack-primary/90"
            >
              <Plus className="h-4 w-4" />
              Add Learning
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
              className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none transition placeholder:text-gray-600 focus:border-hack-primary focus:bg-black/80"
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
              className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none transition placeholder:text-gray-600 focus:border-hack-primary focus:bg-black/80"
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
            <h2 className="text-lg font-semibold text-white">Learning Records</h2>
            <p className="text-sm text-gray-500">
              {query.isLoading ? "Loading..." : `${query.data?.count ?? 0} records`}
            </p>
          </div>
          <div className="flex items-center gap-2 rounded-full bg-hack-primary/10 px-3 py-1 text-xs text-hack-primary">
            <ShieldCheck className="h-4 w-4" />
            Scope-aware
          </div>
        </div>

        {query.isError ? (
          <div className="p-6 text-sm text-red-400">
            Failed to load operator learning records.
          </div>
        ) : query.isLoading ? (
          <div className="p-6 text-sm text-gray-400">Loading operator learning...</div>
        ) : records.length === 0 ? (
          <div className="p-10 text-center">
            <BrainCircuit className="mx-auto h-10 w-10 text-gray-600" />
            <h3 className="mt-4 text-lg font-semibold text-white">No operator learning yet</h3>
            <p className="mx-auto mt-2 max-w-2xl text-sm text-gray-500">
              Future patches will allow users to teach Hunt reusable pentest methodology,
              payload strategy, execution hints, and exploit-chain preferences across target,
              project, and user-global scopes.
            </p>
          </div>
        ) : (
          <div className="divide-y divide-hack-primary/10">
            {records.map((record) => (
              <article key={record.id} className="p-6">
                <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                  <div>
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

                  <div className="min-w-[180px] rounded-xl border border-hack-primary/10 bg-hack-background/70 p-3 text-xs text-gray-400">
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

      {isCreateOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/90 p-4 backdrop-blur-sm">
          <div className="max-h-[90vh] w-full max-w-4xl overflow-y-auto rounded-2xl border border-hack-primary/40 bg-black p-6 shadow-2xl">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <h2 className="text-xl font-bold text-white">Add Operator Learning</h2>
                <p className="mt-1 text-sm text-gray-400">
                  Teach the operator reusable methodology, trigger signals, and execution hints for authorized pentest workflows.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsCreateOpen(false)}
                className="rounded-lg p-2 text-gray-400 hover:bg-white/5 hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleCreate} className="space-y-4">
              {createError && (
                <div className="rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-300">
                  {createError}
                </div>
              )}

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Title</span>
                  <input
                    value={newTitle}
                    onChange={(event) => setNewTitle(event.target.value)}
                    placeholder="XSS context-first methodology"
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Scope</span>
                  <select
                    value={newScope}
                    onChange={(event) => setNewScope(event.target.value as OperatorLearningScope)}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    <option className="bg-black text-white" value="user_global">User Global</option>
                    <option className="bg-black text-white" value="project">Project</option>
                    <option className="bg-black text-white" value="target">Target</option>
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Bug class</span>
                  <input
                    value={newBugClass}
                    onChange={(event) => setNewBugClass(event.target.value)}
                    placeholder="xss"
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Skill slug</span>
                  <input
                    value={newSkillSlug}
                    onChange={(event) => setNewSkillSlug(event.target.value)}
                    placeholder="xss_reflection"
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                  />
                </label>
              </div>

              <label className="block space-y-1">
                <span className="text-xs uppercase tracking-wide text-gray-500">Summary</span>
                <input
                  value={newSummary}
                  onChange={(event) => setNewSummary(event.target.value)}
                  placeholder="Classify reflection context before payload execution."
                  className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                />
              </label>

              <label className="block space-y-1">
                <span className="text-xs uppercase tracking-wide text-gray-500">Content</span>
                <textarea
                  value={newContent}
                  onChange={(event) => setNewContent(event.target.value)}
                  rows={4}
                  placeholder="Write the methodology the operator should remember..."
                  className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 text-sm text-white outline-none placeholder:text-gray-600 focus:border-hack-primary"
                />
              </label>

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Applies to JSON array</span>
                  <textarea
                    value={newAppliesTo}
                    onChange={(event) => setNewAppliesTo(event.target.value)}
                    rows={3}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Trigger signals JSON array</span>
                  <textarea
                    value={newTriggerSignals}
                    onChange={(event) => setNewTriggerSignals(event.target.value)}
                    rows={3}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Methodology JSON object</span>
                  <textarea
                    value={newMethodology}
                    onChange={(event) => setNewMethodology(event.target.value)}
                    rows={4}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wide text-gray-500">Execution hints JSON object</span>
                  <textarea
                    value={newExecutionHints}
                    onChange={(event) => setNewExecutionHints(event.target.value)}
                    rows={4}
                    className="w-full rounded-lg border border-hack-primary/25 bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>
              </div>

              <label className="block space-y-1">
                <span className="text-xs uppercase tracking-wide text-gray-500">Confidence: {newConfidence}%</span>
                <input
                  type="range"
                  min={1}
                  max={100}
                  value={newConfidence}
                  onChange={(event) => setNewConfidence(Number(event.target.value))}
                  className="w-full"
                />
              </label>

              <div className="flex flex-wrap justify-end gap-3 pt-2">
                <button
                  type="button"
                  onClick={() => setIsCreateOpen(false)}
                  className="rounded-lg border border-gray-700 px-4 py-2 text-sm text-gray-300 hover:bg-white/5"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending}
                  className="rounded-lg bg-hack-primary px-4 py-2 text-sm font-semibold text-black hover:bg-hack-primary/90 disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {createMutation.isPending ? "Saving..." : "Save Learning"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

    </div>
  );
}
