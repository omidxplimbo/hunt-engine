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
import clsx from "clsx";
import {
  createOperatorSkill,
  deleteOperatorSkill,
  listOperatorSkills,
  updateOperatorSkill,
} from "../api/operatorSkills";
import type {
  CreateOperatorSkillPayload,
  UpdateOperatorSkillPayload,
  OperatorSkill,
} from "../api/operatorSkills";

const scopeOptions = [
  { label: "User", value: "user" },
  { label: "Project", value: "project" },
  { label: "Target", value: "target" },
];

const categoryOptions = [
  { label: "Recon", value: "recon" },
  { label: "Parameter Intelligence", value: "parameter_intelligence" },
  { label: "HTTP Evidence Analysis", value: "http_evidence_analysis" },
  { label: "Client Side", value: "client_side" },
  { label: "Injection", value: "injection" },
  { label: "Access Control", value: "access_control" },
  { label: "Network / File / Cloud", value: "network_file_cloud" },
  { label: "Business Logic", value: "business_logic" },
  { label: "Finding Promotion", value: "finding_promotion" },
  { label: "Exploit Validation", value: "exploit_validation" },
];

const skillTypeOptions = [
  { label: "Planning", value: "planning" },
  { label: "Analysis", value: "analysis" },
  { label: "Active Validation", value: "active_validation" },
  { label: "Exploit Runtime", value: "exploit_runtime" },
  { label: "Chain", value: "chain" },
  { label: "Advisory", value: "advisory" },
];

const runtimeBackendOptions = [
  { label: "None", value: "none" },
  { label: "Internal HTTP Runtime", value: "internal_http_runtime" },
  { label: "Browser Runtime", value: "browser_runtime" },
  { label: "Payload Generator", value: "payload_generator" },
  { label: "Tool Runner", value: "tool_runner" },
  { label: "Shell Runner", value: "shell_runner" },
  { label: "Custom Script Runner", value: "custom_script_runner" },
  { label: "Bounded Brute Force Runner", value: "brute_force_runner" },
];

const permissionModeOptions = [
  { label: "Scope-aware Authorized", value: "scope_aware_authorized" },
  { label: "Manual Approval", value: "manual_approval" },
  { label: "Assisted Autopilot", value: "assisted_autopilot" },
  { label: "Authorized Autonomous", value: "authorized_autonomous" },
];

const riskOptions = [
  { label: "Info", value: "info" },
  { label: "Low", value: "low" },
  { label: "Medium", value: "medium" },
  { label: "High", value: "high" },
  { label: "Critical", value: "critical" },
];

function parseJsonArray(value: string): unknown[] {
  const trimmed = value.trim();
  if (!trimmed) return [];
  const parsed = JSON.parse(trimmed);
  if (!Array.isArray(parsed)) throw new Error("Expected a JSON array");
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

function formatDate(value?: string | null) {
  if (!value) return "Never";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

interface SkillFormState {
  name: string;
  slug: string;
  description: string;
  scope: string;
  category: string;
  bugClass: string;
  skillType: string;
  runtimeBackend: string;
  permissionMode: string;
  riskLevel: string;
  safetyLevel: number;
  testLevel: number;
  autonomyLevel: number;
  isEnabled: boolean;
  triggerSignals: string;
  customDefinition: string;
  budgetDefaults: string;
  stopConditions: string;
}

function defaultFormState(): SkillFormState {
  return {
    name: "",
    slug: "",
    description: "",
    scope: "user",
    category: "exploit_validation",
    bugClass: "",
    skillType: "planning",
    runtimeBackend: "none",
    permissionMode: "manual_approval",
    riskLevel: "low",
    safetyLevel: 1,
    testLevel: 1,
    autonomyLevel: 1,
    isEnabled: true,
    triggerSignals: '["parameter","endpoint"]',
    customDefinition:
      '{\n  "workflow": [\n    "select candidates",\n    "check scope and authorization",\n    "plan controlled validation",\n    "capture evidence"\n  ],\n  "execution_note": "Definition only until runtime integration is explicitly wired."\n}',
    budgetDefaults: '{\n  "max_requests": 20,\n  "max_runtime_seconds": 120\n}',
    stopConditions: '{\n  "stop_on_out_of_scope": true,\n  "stop_on_policy_block": true,\n  "stop_on_rate_limit": true\n}',
  };
}

function formStateFromSkill(skill: OperatorSkill): SkillFormState {
  return {
    name: skill.name || "",
    slug: skill.slug || "",
    description: skill.description || "",
    scope: skill.scope || "user",
    category: skill.category || "exploit_validation",
    bugClass: skill.bug_class || "",
    skillType: skill.skill_type || "planning",
    runtimeBackend: skill.runtime_backend || "none",
    permissionMode: skill.permission_mode || "manual_approval",
    riskLevel: skill.default_risk_level || "low",
    safetyLevel: skill.default_safety_level || 1,
    testLevel: skill.default_test_level || 1,
    autonomyLevel: skill.default_autonomy_level || 1,
    isEnabled: skill.is_enabled,
    triggerSignals: stringifyJson(skill.trigger_signals || [], "[]"),
    customDefinition: stringifyJson(skill.custom_definition || {}, "{}"),
    budgetDefaults: stringifyJson(skill.budget_defaults || {}, "{}"),
    stopConditions: stringifyJson(skill.stop_conditions || {}, "{}"),
  };
}

function payloadFromFormState(form: SkillFormState): CreateOperatorSkillPayload {
  return {
    name: form.name.trim(),
    slug: form.slug.trim(),
    description: form.description.trim(),
    scope: form.scope,
    category: form.category,
    bug_class: form.bugClass.trim(),
    skill_type: form.skillType,
    runtime_backend: form.runtimeBackend,
    permission_mode: form.permissionMode,
    default_risk_level: form.riskLevel,
    default_safety_level: form.safetyLevel,
    default_test_level: form.testLevel,
    default_autonomy_level: form.autonomyLevel,
    is_enabled: form.isEnabled,
    trigger_signals: parseJsonArray(form.triggerSignals),
    custom_definition: parseJsonObject(form.customDefinition),
    budget_defaults: parseJsonObject(form.budgetDefaults),
    stop_conditions: parseJsonObject(form.stopConditions),
  };
}

function skillOrigin(skill: OperatorSkill) {
  if (skill.is_builtin || skill.origin === "builtin") return "built-in";
  return skill.origin || "user";
}

function badgeClass(skill: OperatorSkill) {
  if (skill.is_builtin || skill.origin === "builtin") {
    return "border-hack-primary/40 bg-hack-primary/10 text-hack-primary";
  }
  return "border-purple-400/40 bg-purple-500/10 text-purple-300";
}

export default function OperatorSkills() {
  const [includeDisabled, setIncludeDisabled] = useState(true);
  const [queryText, setQueryText] = useState("");
  const [originFilter, setOriginFilter] = useState<"" | "builtin" | "user">("");
  const [categoryFilter, setCategoryFilter] = useState("");
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [editingSkill, setEditingSkill] = useState<OperatorSkill | null>(null);
  const [form, setForm] = useState<SkillFormState>(defaultFormState());
  const [formError, setFormError] = useState("");

  const query = useQuery({
    queryKey: ["operator-skills", "management", includeDisabled],
    queryFn: () => listOperatorSkills(includeDisabled),
  });

  const createMutation = useMutation({
    mutationFn: createOperatorSkill,
    onSuccess: () => {
      closeModal();
      void query.refetch();
    },
    onError: () => {
      setFormError("Failed to create executable skill.");
    },
  });

  const updateMutation = useMutation({
    mutationFn: ({ id, payload }: { id: number; payload: UpdateOperatorSkillPayload }) =>
      updateOperatorSkill(id, payload),
    onSuccess: () => {
      closeModal();
      void query.refetch();
    },
    onError: () => {
      setFormError("Failed to update executable skill.");
    },
  });

  const deleteMutation = useMutation({
    mutationFn: deleteOperatorSkill,
    onSuccess: () => {
      void query.refetch();
    },
  });

  const skills = query.data?.skills ?? [];

  const filteredSkills = useMemo(() => {
    const needle = queryText.trim().toLowerCase();
    return skills.filter((skill) => {
      const origin = skill.is_builtin || skill.origin === "builtin" ? "builtin" : "user";
      if (originFilter && origin !== originFilter) return false;
      if (categoryFilter && skill.category !== categoryFilter) return false;
      if (!needle) return true;
      return [
        skill.name,
        skill.slug,
        skill.description,
        skill.category,
        skill.bug_class,
        skill.runtime_backend,
        skill.permission_mode,
      ]
        .filter(Boolean)
        .join(" ")
        .toLowerCase()
        .includes(needle);
    });
  }, [skills, queryText, originFilter, categoryFilter]);

  const customCount = skills.filter((skill) => !skill.is_builtin && skill.origin !== "builtin").length;
  const builtinCount = skills.length - customCount;

  const openCreateModal = () => {
    setEditingSkill(null);
    setForm(defaultFormState());
    setFormError("");
    setIsModalOpen(true);
  };

  const openEditModal = (skill: OperatorSkill) => {
    if (skill.is_builtin || skill.origin === "builtin") return;
    setEditingSkill(skill);
    setForm(formStateFromSkill(skill));
    setFormError("");
    setIsModalOpen(true);
  };

  function closeModal() {
    setIsModalOpen(false);
    setEditingSkill(null);
    setForm(defaultFormState());
    setFormError("");
  }

  const updateForm = <K extends keyof SkillFormState>(key: K, value: SkillFormState[K]) => {
    setForm((prev) => ({ ...prev, [key]: value }));
  };

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError("");

    if (!form.name.trim()) {
      setFormError("Name is required.");
      return;
    }

    try {
      const payload = payloadFromFormState(form);
      if (editingSkill) {
        updateMutation.mutate({ id: editingSkill.id, payload });
      } else {
        createMutation.mutate(payload);
      }
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Invalid JSON field.");
    }
  };

  const handleDelete = (skill: OperatorSkill) => {
    if (skill.is_builtin || skill.origin === "builtin") return;
    const ok = window.confirm(
      `Delete executable skill "${skill.name}"? This removes it from normal lists and target profile selectors.`
    );
    if (!ok) return;
    deleteMutation.mutate(skill.id);
  };

  const handleToggleEnabled = (skill: OperatorSkill) => {
    if (skill.is_builtin || skill.origin === "builtin") return;
    updateMutation.mutate({
      id: skill.id,
      payload: { is_enabled: !skill.is_enabled },
    });
  };

  return (
    <div className="space-y-6 p-6">
      <div className="border border-hack-border bg-black/30 p-5">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <BrainCircuit className="h-5 w-5 text-hack-primary" />
              <h1 className="font-mono text-xl uppercase tracking-wider text-white">
                Executable Skills
              </h1>
            </div>
            <p className="mt-2 max-w-4xl text-sm text-hack-dim">
              Manage built-in and user-defined operator skills. Built-in skills are read-only.
              User-defined skills define authorized validation, analysis, payload strategy, tool,
              shell, browser, and chained workflows; runtime execution is still governed by target
              scope, policy, permission mode, budgets, stop conditions, and audit controls.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => void query.refetch()}
              className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3"
            >
              <RefreshCw className={clsx("h-4 w-4", query.isFetching && "animate-spin")} />
              Refresh
            </button>
            <button
              type="button"
              onClick={openCreateModal}
              className="hack-btn flex items-center gap-2 bg-hack-primary px-3 text-black"
            >
              <Plus className="h-4 w-4" />
              New Skill
            </button>
          </div>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-3">
        <div className="border border-hack-border bg-black/30 p-4">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Total Skills
          </div>
          <div className="mt-1 font-mono text-2xl text-white">{skills.length}</div>
        </div>
        <div className="border border-hack-border bg-black/30 p-4">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Built-in
          </div>
          <div className="mt-1 font-mono text-2xl text-hack-primary">{builtinCount}</div>
        </div>
        <div className="border border-hack-border bg-black/30 p-4">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            User-defined
          </div>
          <div className="mt-1 font-mono text-2xl text-purple-300">{customCount}</div>
        </div>
      </div>

      <div className="border border-hack-border bg-black/30 p-4">
        <div className="grid gap-3 lg:grid-cols-4">
          <label className="space-y-1 lg:col-span-2">
            <span className="flex items-center gap-2 text-xs uppercase tracking-wider text-hack-dim">
              <Search className="h-3 w-3" /> Search
            </span>
            <input
              value={queryText}
              onChange={(event) => setQueryText(event.target.value)}
              placeholder="Search name, slug, bug class, runtime..."
              className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
            />
          </label>

          <label className="space-y-1">
            <span className="text-xs uppercase tracking-wider text-hack-dim">Origin</span>
            <select
              value={originFilter}
              onChange={(event) => setOriginFilter(event.target.value as "" | "builtin" | "user")}
              className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
            >
              <option value="">All origins</option>
              <option value="builtin">Built-in</option>
              <option value="user">User-defined</option>
            </select>
          </label>

          <label className="space-y-1">
            <span className="text-xs uppercase tracking-wider text-hack-dim">Category</span>
            <select
              value={categoryFilter}
              onChange={(event) => setCategoryFilter(event.target.value)}
              className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
            >
              <option value="">All categories</option>
              {categoryOptions.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </label>
        </div>

        <label className="mt-3 flex items-center gap-2 text-sm text-white">
          <input
            type="checkbox"
            checked={includeDisabled}
            onChange={(event) => setIncludeDisabled(event.target.checked)}
            className="h-4 w-4"
          />
          Include disabled registry skills
        </label>
      </div>

      <div className="border border-hack-border bg-black/30">
        {query.isLoading ? (
          <div className="p-5 text-sm text-hack-dim">Loading executable skills...</div>
        ) : filteredSkills.length === 0 ? (
          <div className="p-5 text-sm text-hack-dim">No executable skills matched.</div>
        ) : (
          <div className="divide-y divide-hack-border">
            {filteredSkills.map((skill) => {
              const custom = !skill.is_builtin && skill.origin !== "builtin";
              return (
                <div key={skill.id} className="p-4 hover:bg-white/[0.02]">
                  <div className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-sm text-white">{skill.name}</span>
                        <span className="rounded border border-hack-border px-1.5 py-0.5 font-mono text-[10px] text-hack-dim">
                          {skill.slug}
                        </span>
                        <span className={clsx("rounded border px-1.5 py-0.5 font-mono text-[10px]", badgeClass(skill))}>
                          {skillOrigin(skill)}
                        </span>
                        <span className="rounded border border-hack-border bg-black/30 px-1.5 py-0.5 font-mono text-[10px] text-hack-dim">
                          {skill.scope || "builtin"}
                        </span>
                        <span className="rounded border border-blue-400/40 bg-blue-500/10 px-1.5 py-0.5 font-mono text-[10px] text-blue-300">
                          {skill.category}
                        </span>
                        {skill.bug_class && (
                          <span className="rounded border border-hack-primary/40 bg-hack-primary/10 px-1.5 py-0.5 font-mono text-[10px] text-hack-primary">
                            bug:{skill.bug_class}
                          </span>
                        )}
                        {!skill.is_enabled && (
                          <span className="rounded border border-yellow-500/40 bg-yellow-500/10 px-1.5 py-0.5 font-mono text-[10px] text-yellow-300">
                            disabled
                          </span>
                        )}
                      </div>

                      {skill.description && (
                        <p className="mt-2 max-w-4xl text-sm text-hack-dim">{skill.description}</p>
                      )}

                      <div className="mt-2 flex flex-wrap gap-2 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                        <span>type: {(skill.skill_type || "planning").replaceAll("_", " ")}</span>
                        <span>runtime: {(skill.runtime_backend || "none").replaceAll("_", " ")}</span>
                        <span>risk: {skill.default_risk_level}</span>
                        <span>safety: {skill.default_safety_level}</span>
                        <span>test: {skill.default_test_level}</span>
                        <span>autonomy: {skill.default_autonomy_level}</span>
                        <span>permission: {skill.permission_mode}</span>
                        <span>updated: {formatDate(skill.updated_at)}</span>
                      </div>
                    </div>

                    <div className="flex flex-wrap gap-2">
                      {custom ? (
                        <>
                          <button
                            type="button"
                            onClick={() => handleToggleEnabled(skill)}
                            className="hack-btn-ghost border border-hack-border px-3 text-xs text-hack-dim hover:text-white"
                          >
                            {skill.is_enabled ? "Disable" : "Enable"}
                          </button>
                          <button
                            type="button"
                            onClick={() => openEditModal(skill)}
                            className="hack-btn-ghost flex items-center gap-2 border border-hack-primary/50 px-3 text-xs text-hack-primary"
                          >
                            <Edit3 className="h-3 w-3" />
                            Edit
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDelete(skill)}
                            className="hack-btn-ghost flex items-center gap-2 border border-red-500/50 px-3 text-xs text-red-300"
                          >
                            <Trash2 className="h-3 w-3" />
                            Delete
                          </button>
                        </>
                      ) : (
                        <span className="flex items-center gap-2 border border-hack-border bg-black/30 px-3 py-2 text-xs text-hack-dim">
                          <ShieldCheck className="h-3 w-3" />
                          Read-only built-in
                        </span>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {isModalOpen && (
        <div className="fixed inset-y-0 right-0 left-0 z-[10000] flex items-start justify-center overflow-y-auto bg-black/80 p-4 md:left-[340px]">
          <div className="my-6 w-full max-w-5xl border border-hack-border bg-[#050505] shadow-xl">
            <div className="flex items-center justify-between border-b border-hack-border px-5 py-4">
              <div>
                <h2 className="font-mono text-lg uppercase tracking-wider text-white">
                  {editingSkill ? "Edit User-defined Skill" : "Create User-defined Skill"}
                </h2>
                <p className="mt-1 text-sm text-hack-dim">
                  Define executable operator skill metadata and workflow instructions. Runtime execution is attached in later authorized runtime patches.
                </p>
              </div>
              <button
                type="button"
                onClick={closeModal}
                className="text-hack-dim hover:text-white"
              >
                <X className="h-5 w-5" />
              </button>
            </div>

            <form onSubmit={handleSubmit} className="space-y-5 p-5">
              {formError && (
                <div className="border border-red-500/50 bg-red-500/10 px-3 py-2 text-sm text-red-300">
                  {formError}
                </div>
              )}

              <div className="grid gap-4 md:grid-cols-2">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Name</span>
                  <input
                    value={form.name}
                    onChange={(event) => updateForm("name", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                    required
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">
                    Slug {editingSkill ? "(stored slug is preserved by backend)" : ""}
                  </span>
                  <input
                    value={form.slug}
                    onChange={(event) => updateForm("slug", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                    placeholder="custom-skill-slug"
                    disabled={Boolean(editingSkill)}
                  />
                </label>
              </div>

              <label className="block space-y-1">
                <span className="text-xs uppercase tracking-wider text-hack-dim">Description</span>
                <textarea
                  value={form.description}
                  onChange={(event) => updateForm("description", event.target.value)}
                  rows={3}
                  className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                />
              </label>

              <div className="grid gap-4 md:grid-cols-3">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Scope</span>
                  <select
                    value={form.scope}
                    onChange={(event) => updateForm("scope", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {scopeOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Category</span>
                  <select
                    value={form.category}
                    onChange={(event) => updateForm("category", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {categoryOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Bug class</span>
                  <input
                    value={form.bugClass}
                    onChange={(event) => updateForm("bugClass", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                    placeholder="xss, ssrf, idor, sqli..."
                  />
                </label>
              </div>

              <div className="grid gap-4 md:grid-cols-3">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Skill type</span>
                  <select
                    value={form.skillType}
                    onChange={(event) => updateForm("skillType", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {skillTypeOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Runtime backend</span>
                  <select
                    value={form.runtimeBackend}
                    onChange={(event) => updateForm("runtimeBackend", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {runtimeBackendOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Permission mode</span>
                  <select
                    value={form.permissionMode}
                    onChange={(event) => updateForm("permissionMode", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {permissionModeOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                </label>
              </div>

              <div className="grid gap-4 md:grid-cols-4">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Risk</span>
                  <select
                    value={form.riskLevel}
                    onChange={(event) => updateForm("riskLevel", event.target.value)}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                  >
                    {riskOptions.map((item) => (
                      <option key={item.value} value={item.value}>{item.label}</option>
                    ))}
                  </select>
                </label>

                {(["safetyLevel", "testLevel", "autonomyLevel"] as const).map((key) => (
                  <label key={key} className="space-y-1">
                    <span className="text-xs uppercase tracking-wider text-hack-dim">
                      {key.replace("Level", " level")}
                    </span>
                    <input
                      type="number"
                      min={0}
                      max={5}
                      value={form[key]}
                      onChange={(event) => updateForm(key, Number(event.target.value))}
                      className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
                    />
                  </label>
                ))}
              </div>

              <label className="flex items-center gap-2 text-sm text-white">
                <input
                  type="checkbox"
                  checked={form.isEnabled}
                  onChange={(event) => updateForm("isEnabled", event.target.checked)}
                  className="h-4 w-4"
                />
                Registry enabled
              </label>

              <div className="grid gap-4 lg:grid-cols-2">
                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Trigger signals JSON array</span>
                  <textarea
                    value={form.triggerSignals}
                    onChange={(event) => updateForm("triggerSignals", event.target.value)}
                    rows={7}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Custom definition JSON object</span>
                  <textarea
                    value={form.customDefinition}
                    onChange={(event) => updateForm("customDefinition", event.target.value)}
                    rows={7}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Budget defaults JSON object</span>
                  <textarea
                    value={form.budgetDefaults}
                    onChange={(event) => updateForm("budgetDefaults", event.target.value)}
                    rows={7}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>

                <label className="space-y-1">
                  <span className="text-xs uppercase tracking-wider text-hack-dim">Stop conditions JSON object</span>
                  <textarea
                    value={form.stopConditions}
                    onChange={(event) => updateForm("stopConditions", event.target.value)}
                    rows={7}
                    className="w-full border border-hack-border bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
                  />
                </label>
              </div>

              <div className="flex flex-wrap justify-end gap-2 border-t border-hack-border pt-4">
                <button
                  type="button"
                  onClick={closeModal}
                  className="hack-btn-ghost border border-hack-border px-4 text-hack-dim hover:text-white"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={createMutation.isPending || updateMutation.isPending}
                  className="hack-btn bg-hack-primary px-4 text-black disabled:opacity-50"
                >
                  {createMutation.isPending || updateMutation.isPending
                    ? "Saving..."
                    : editingSkill
                      ? "Save Skill"
                      : "Create Skill"}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
