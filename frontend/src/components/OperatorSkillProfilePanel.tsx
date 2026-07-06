import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery } from "@tanstack/react-query";
import { BrainCircuit, RefreshCw, Save, ShieldCheck } from "lucide-react";
import clsx from "clsx";
import {
  getTargetOperatorSkillProfile,
  listOperatorSkills,
  updateTargetOperatorSkillProfile,
} from "../api/operatorSkills";
import { listOperatorLearningRecords } from "../api/operatorLearning";
import type {
  OperatorSkill,
  OperatorTargetSkillProfile,
} from "../api/operatorSkills";
import type { OperatorLearningRecord } from "../api/operatorLearning";

const runtimeBackendOptions = [
  { value: "internal_http_runtime", label: "Internal HTTP" },
  { value: "browser_runtime", label: "Browser" },
  { value: "payload_generator", label: "Payload Generator" },
  { value: "tool_runner", label: "Tool Runner" },
  { value: "shell_runner", label: "Shell Runner" },
  { value: "brute_force_runner", label: "Bounded Brute Force" },
];

const permissionModes = [
  { value: "scope_aware_authorized", label: "Scope-aware authorized" },
  { value: "manual_approval", label: "Manual approval" },
  { value: "assisted_autopilot", label: "Assisted autopilot" },
  { value: "authorized_autonomous", label: "Authorized autonomous" },
];

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

function categoryLabel(skill: OperatorSkill) {
  return skill.category?.replaceAll("_", " ") || "uncategorized";
}

function operatorSkillOriginLabel(skill: OperatorSkill) {
  if (skill.is_builtin || skill.origin === "builtin") return "built-in";
  return skill.origin || "user";
}

function operatorSkillScopeLabel(skill: OperatorSkill) {
  if (skill.is_builtin || skill.scope === "builtin") return "builtin";
  return skill.scope || "user";
}

function operatorSkillRuntimeLabel(skill: OperatorSkill) {
  return (skill.runtime_backend || "none").replaceAll("_", " ");
}

function operatorSkillTypeLabel(skill: OperatorSkill) {
  return (skill.skill_type || "planning").replaceAll("_", " ");
}

function operatorSkillBadgeClass(skill: OperatorSkill) {
  if (skill.is_builtin || skill.origin === "builtin") {
    return "border-hack-primary/40 bg-hack-primary/10 text-hack-primary";
  }
  return "border-purple-400/40 bg-purple-500/10 text-purple-300";
}

function learningAppliesToSkill(learning: OperatorLearningRecord, skill: OperatorSkill) {
  if (!learning.skill_slug && !learning.bug_class) return true;
  if (learning.skill_slug && learning.skill_slug === skill.slug) return true;
  if (learning.bug_class && skill.bug_class && learning.bug_class === skill.bug_class) return true;
  return false;
}

interface OperatorSkillProfilePanelProps {
  targetId: number;
}

export default function OperatorSkillProfilePanel({
  targetId,
}: OperatorSkillProfilePanelProps) {
  const skillsQuery = useQuery({
    queryKey: ["operator-skills", "all", "include-disabled"],
    queryFn: () => listOperatorSkills(true),
  });

  const profileQuery = useQuery({
    queryKey: ["target-operator-skill-profile", targetId],
    queryFn: () => getTargetOperatorSkillProfile(targetId),
    enabled: Boolean(targetId),
  });

  const learningQuery = useQuery({
    queryKey: ["operator-learning", "active", "target-profile"],
    queryFn: () =>
      listOperatorLearningRecords({
        status: "active",
        limit: 200,
      }),
  });

  const [isEnabled, setIsEnabled] = useState(true);
  const [permissionMode, setPermissionMode] = useState("scope_aware_authorized");
  const [disabledSkillSlugs, setDisabledSkillSlugs] = useState<string[]>([]);
  const [allowedRuntimeBackends, setAllowedRuntimeBackends] = useState<string[]>([
    "internal_http_runtime",
    "browser_runtime",
  ]);
  const [preferredLearningRecordIds, setPreferredLearningRecordIds] = useState<number[]>([]);
  const [budgetDefaults, setBudgetDefaults] = useState(
    '{\n  "max_skill_runs_per_chat": 5,\n  "max_requests_per_skill": 20\n}'
  );
  const [stopConditions, setStopConditions] = useState(
    '{\n  "stop_on_policy_block": true,\n  "stop_on_rate_limit": true\n}'
  );
  const [error, setError] = useState("");
  const [savedMessage, setSavedMessage] = useState("");

  useEffect(() => {
    const profile = profileQuery.data;
    if (!profile) return;

    setIsEnabled(profile.is_enabled);
    setPermissionMode(profile.permission_mode || "scope_aware_authorized");
    setDisabledSkillSlugs(profile.disabled_skill_slugs || []);
    setAllowedRuntimeBackends(profile.allowed_runtime_backends || []);
    setPreferredLearningRecordIds(profile.preferred_learning_record_ids || []);
    setBudgetDefaults(stringifyJson(profile.budget_defaults || {}, "{}"));
    setStopConditions(stringifyJson(profile.stop_conditions || {}, "{}"));
  }, [profileQuery.data]);

  const updateMutation = useMutation({
    mutationFn: (profile: Partial<OperatorTargetSkillProfile>) =>
      updateTargetOperatorSkillProfile(targetId, {
        is_enabled: profile.is_enabled,
        permission_mode: profile.permission_mode,
        enabled_skill_slugs: [],
        disabled_skill_slugs: profile.disabled_skill_slugs,
        preferred_learning_record_ids: profile.preferred_learning_record_ids,
        allowed_runtime_backends: profile.allowed_runtime_backends,
        budget_defaults: profile.budget_defaults,
        stop_conditions: profile.stop_conditions,
        metadata: {
          source: "target_skill_profile_ui",
        },
      }),
    onSuccess: () => {
      setSavedMessage("Profile saved.");
      setError("");
      void profileQuery.refetch();
    },
    onError: () => {
      setSavedMessage("");
      setError("Failed to save operator skill profile.");
    },
  });

  const skills = skillsQuery.data?.skills ?? [];
  const learningRecords = learningQuery.data?.learning ?? [];

  const groupedSkills = useMemo(() => {
    const groups = new Map<string, OperatorSkill[]>();
    for (const skill of skills) {
      const key = categoryLabel(skill);
      groups.set(key, [...(groups.get(key) || []), skill]);
    }
    return Array.from(groups.entries()).sort(([a], [b]) => a.localeCompare(b));
  }, [skills]);

  const selectedLearningRecords = useMemo(
    () =>
      learningRecords.filter((record) =>
        preferredLearningRecordIds.includes(Number(record.id))
      ),
    [learningRecords, preferredLearningRecordIds]
  );

  const toggleDisabledSkill = (slug: string) => {
    setDisabledSkillSlugs((prev) =>
      prev.includes(slug)
        ? prev.filter((item) => item !== slug)
        : [...prev, slug].sort()
    );
  };

  const toggleRuntimeBackend = (backend: string) => {
    setAllowedRuntimeBackends((prev) =>
      prev.includes(backend)
        ? prev.filter((item) => item !== backend)
        : [...prev, backend].sort()
    );
  };

  const toggleLearningRecord = (recordId: number) => {
    setPreferredLearningRecordIds((prev) =>
      prev.includes(recordId)
        ? prev.filter((item) => item !== recordId)
        : [...prev, recordId].sort((a, b) => a - b)
    );
  };

  const handleSave = () => {
    setError("");
    setSavedMessage("");

    try {
      updateMutation.mutate({
        is_enabled: isEnabled,
        permission_mode: permissionMode,
        disabled_skill_slugs: disabledSkillSlugs,
        preferred_learning_record_ids: preferredLearningRecordIds,
        allowed_runtime_backends: allowedRuntimeBackends,
        budget_defaults: parseJsonObject(budgetDefaults),
        stop_conditions: parseJsonObject(stopConditions),
      });
    } catch (saveError) {
      setError(saveError instanceof Error ? saveError.message : "Invalid JSON field.");
    }
  };

  return (
    <div className="space-y-4">
      <div className="border border-hack-border bg-black/30 p-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <div>
            <div className="flex items-center gap-2">
              <BrainCircuit className="h-5 w-5 text-hack-primary" />
              <h2 className="font-mono text-sm uppercase tracking-wider text-white">
                Target Operator Profile
              </h2>
            </div>
            <p className="mt-2 max-w-3xl text-sm text-hack-dim">
              Configure executable skills and user methodology records for this authorized target.
              Disabled executable skills are excluded from automatic chat planning. Selected
              methodology records guide how the operator should reason about and execute matching skills.
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => {
                void skillsQuery.refetch();
                void profileQuery.refetch();
                void learningQuery.refetch();
              }}
              className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3"
            >
              <RefreshCw
                className={clsx(
                  "h-4 w-4",
                  (skillsQuery.isFetching ||
                    profileQuery.isFetching ||
                    learningQuery.isFetching) &&
                    "animate-spin"
                )}
              />
              Refresh
            </button>
            <button
              type="button"
              onClick={handleSave}
              disabled={updateMutation.isPending}
              className="hack-btn flex items-center gap-2 bg-hack-primary px-3 text-black disabled:opacity-50"
            >
              <Save className="h-4 w-4" />
              {updateMutation.isPending ? "Saving..." : "Save Profile"}
            </button>
          </div>
        </div>

        {(error || savedMessage) && (
          <div
            className={clsx(
              "mt-4 rounded border px-3 py-2 text-sm",
              error
                ? "border-red-500/40 bg-red-500/10 text-red-300"
                : "border-hack-primary/40 bg-hack-primary/10 text-hack-primary"
            )}
          >
            {error || savedMessage}
          </div>
        )}
      </div>

      <div className="grid gap-4 lg:grid-cols-3">
        <div className="space-y-4 lg:col-span-1">
          <div className="border border-hack-border bg-black/30 p-4">
            <div className="mb-3 flex items-center gap-2 font-mono text-xs uppercase tracking-wider text-hack-dim">
              <ShieldCheck className="h-4 w-4 text-hack-primary" />
              Execution Controls
            </div>

            <label className="mb-4 flex items-center gap-2 text-sm text-white">
              <input
                type="checkbox"
                checked={isEnabled}
                onChange={(event) => setIsEnabled(event.target.checked)}
                className="h-4 w-4"
              />
              Enable profile for this target
            </label>

            <label className="mb-4 block space-y-1">
              <span className="text-xs uppercase tracking-wider text-hack-dim">
                Permission mode
              </span>
              <select
                value={permissionMode}
                onChange={(event) => setPermissionMode(event.target.value)}
                className="w-full border border-hack-border bg-black/60 px-3 py-2 text-sm text-white outline-none focus:border-hack-primary"
              >
                {permissionModes.map((mode) => (
                  <option key={mode.value} value={mode.value} className="bg-black text-white">
                    {mode.label}
                  </option>
                ))}
              </select>
            </label>

            <div className="mb-4 space-y-2">
              <div className="text-xs uppercase tracking-wider text-hack-dim">
                Allowed runtime backends
              </div>
              {runtimeBackendOptions.map((runtime) => (
                <label key={runtime.value} className="flex items-center gap-2 text-sm text-white">
                  <input
                    type="checkbox"
                    checked={allowedRuntimeBackends.includes(runtime.value)}
                    onChange={() => toggleRuntimeBackend(runtime.value)}
                    className="h-4 w-4"
                  />
                  {runtime.label}
                </label>
              ))}
            </div>

            <label className="mb-4 block space-y-1">
              <span className="text-xs uppercase tracking-wider text-hack-dim">
                Budget defaults JSON
              </span>
              <textarea
                value={budgetDefaults}
                onChange={(event) => setBudgetDefaults(event.target.value)}
                rows={5}
                className="w-full border border-hack-border bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>

            <label className="block space-y-1">
              <span className="text-xs uppercase tracking-wider text-hack-dim">
                Stop conditions JSON
              </span>
              <textarea
                value={stopConditions}
                onChange={(event) => setStopConditions(event.target.value)}
                rows={5}
                className="w-full border border-hack-border bg-black/60 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>
          </div>

          <div className="border border-hack-border bg-black/30 p-4">
            <div className="mb-2 font-mono text-xs uppercase tracking-wider text-hack-primary">
              Operator Methodology / Skill Instructions
            </div>
            <p className="mb-3 text-xs text-hack-dim">
              Select user-authored methodology records that should guide matching executable skills for this target.
            </p>

            {learningQuery.isLoading ? (
              <div className="text-sm text-hack-dim">Loading methodology records...</div>
            ) : learningRecords.length === 0 ? (
              <div className="text-sm text-hack-dim">
                No methodology records yet. Add them from Operator Learning.
              </div>
            ) : (
              <div className="max-h-[420px] space-y-2 overflow-y-auto pr-1">
                {learningRecords.map((record) => {
                  const selected = preferredLearningRecordIds.includes(Number(record.id));
                  const matchingSkills = skills.filter((skill) =>
                    learningAppliesToSkill(record, skill)
                  );
                  return (
                    <label
                      key={record.id}
                      className={clsx(
                        "block cursor-pointer border p-3 transition-colors",
                        selected
                          ? "border-hack-primary bg-hack-primary/10"
                          : "border-hack-border bg-black/20 hover:border-hack-primary/50"
                      )}
                    >
                      <div className="flex items-start gap-2">
                        <input
                          type="checkbox"
                          checked={selected}
                          onChange={() => toggleLearningRecord(Number(record.id))}
                          className="mt-1 h-4 w-4"
                        />
                        <div className="min-w-0 flex-1">
                          <div className="font-mono text-sm text-white">{record.title}</div>
                          {record.summary && (
                            <div className="mt-1 text-xs text-hack-dim">{record.summary}</div>
                          )}
                          <div className="mt-2 flex flex-wrap gap-1 text-[10px]">
                            <span className="rounded border border-hack-border px-1.5 py-0.5 text-hack-dim">
                              id:{record.id}
                            </span>
                            {record.scope && (
                              <span className="rounded border border-hack-primary/40 bg-hack-primary/10 px-1.5 py-0.5 text-hack-primary">
                                {record.scope}
                              </span>
                            )}
                            {record.bug_class && (
                              <span className="rounded border border-blue-400/40 bg-blue-500/10 px-1.5 py-0.5 text-blue-300">
                                bug:{record.bug_class}
                              </span>
                            )}
                            {record.skill_slug && (
                              <span className="rounded border border-purple-400/40 bg-purple-500/10 px-1.5 py-0.5 text-purple-300">
                                skill:{record.skill_slug}
                              </span>
                            )}
                          </div>
                          {matchingSkills.length > 0 && (
                            <div className="mt-2 text-[10px] uppercase tracking-wider text-hack-dim">
                              guides: {matchingSkills.map((skill) => skill.slug).join(", ")}
                            </div>
                          )}
                        </div>
                      </div>
                    </label>
                  );
                })}
              </div>
            )}

            {selectedLearningRecords.length > 0 && (
              <div className="mt-3 rounded border border-hack-primary/30 bg-hack-primary/10 px-3 py-2 text-xs text-hack-primary">
                Selected: {selectedLearningRecords.map((record) => record.title).join(", ")}
              </div>
            )}
          </div>
        </div>

        <div className="border border-hack-border bg-black/30 p-4 lg:col-span-2">
          <div className="mb-3 font-mono text-xs uppercase tracking-wider text-hack-primary">
            Executable Skills
          </div>

          {skillsQuery.isLoading || profileQuery.isLoading ? (
            <div className="text-sm text-hack-dim">Loading skill profile...</div>
          ) : groupedSkills.length === 0 ? (
            <div className="text-sm text-hack-dim">No executable skills available.</div>
          ) : (
            <div className="space-y-4">
              {groupedSkills.map(([category, categorySkills]) => (
                <div key={category} className="border border-hack-border/70 bg-black/20">
                  <div className="border-b border-hack-border/70 px-3 py-2 font-mono text-[11px] uppercase tracking-wider text-hack-primary">
                    {category}
                  </div>
                  <div className="divide-y divide-hack-border/60">
                    {categorySkills.map((skill) => {
                      const disabled = disabledSkillSlugs.includes(skill.slug);
                      const selectedMethodologies = learningRecords.filter(
                        (record) =>
                          preferredLearningRecordIds.includes(Number(record.id)) &&
                          learningAppliesToSkill(record, skill)
                      );

                      return (
                        <label
                          key={skill.slug}
                          className="flex cursor-pointer items-start gap-3 px-3 py-3 hover:bg-white/[0.03]"
                        >
                          <input
                            type="checkbox"
                            checked={!disabled}
                            onChange={() => toggleDisabledSkill(skill.slug)}
                            className="mt-1 h-4 w-4"
                          />
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="font-mono text-sm text-white">
                                {skill.name}
                              </span>
                              <span className="rounded border border-hack-border px-1.5 py-0.5 font-mono text-[10px] text-hack-dim">
                                {skill.slug}
                              </span>
                              <span
                                className={clsx(
                                  "rounded border px-1.5 py-0.5 font-mono text-[10px]",
                                  operatorSkillBadgeClass(skill)
                                )}
                              >
                                {operatorSkillOriginLabel(skill)}
                              </span>
                              <span className="rounded border border-hack-border bg-black/30 px-1.5 py-0.5 font-mono text-[10px] text-hack-dim">
                                scope:{operatorSkillScopeLabel(skill)}
                              </span>
                              <span className="rounded border border-hack-border bg-black/30 px-1.5 py-0.5 font-mono text-[10px] text-hack-dim">
                                type:{operatorSkillTypeLabel(skill)}
                              </span>
                              {skill.bug_class && (
                                <span className="rounded border border-hack-primary/40 bg-hack-primary/10 px-1.5 py-0.5 text-[10px] text-hack-primary">
                                  bug:{skill.bug_class}
                                </span>
                              )}
                              {!skill.is_enabled && (
                                <span className="rounded border border-yellow-500/40 bg-yellow-500/10 px-1.5 py-0.5 text-[10px] text-yellow-300">
                                  registry disabled
                                </span>
                              )}
                              {disabled && (
                                <span className="rounded border border-red-500/40 bg-red-500/10 px-1.5 py-0.5 text-[10px] text-red-300">
                                  disabled for target
                                </span>
                              )}
                            </div>
                            {skill.description && (
                              <p className="mt-1 text-xs text-hack-dim">
                                {skill.description}
                              </p>
                            )}
                            {selectedMethodologies.length > 0 && (
                              <div className="mt-2 rounded border border-hack-primary/30 bg-hack-primary/10 px-2 py-1 text-[10px] text-hack-primary">
                                methodology:{" "}
                                {selectedMethodologies
                                  .map((record) => record.title)
                                  .join(", ")}
                              </div>
                            )}
                            <div className="mt-2 flex flex-wrap gap-2 text-[10px] uppercase tracking-wider text-hack-dim">
                              <span>risk: {skill.default_risk_level}</span>
                              <span>safety: {skill.default_safety_level}</span>
                              <span>test: {skill.default_test_level}</span>
                              <span>autonomy: {skill.default_autonomy_level}</span>
                              <span>permission: {skill.permission_mode}</span>
                              <span>runtime: {operatorSkillRuntimeLabel(skill)}</span>
                            </div>

                            {!skill.is_builtin && (
                              <div className="mt-2 rounded border border-purple-400/30 bg-purple-500/10 px-2 py-1 text-[10px] text-purple-300">
                                User-defined executable skill definition. It can be selected for this target profile now; runtime execution is wired by later authorized execution patches.
                              </div>
                            )}
                          </div>
                        </label>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
