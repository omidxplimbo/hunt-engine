import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  Loader2,
  RotateCcw,
  Save,
  ShieldCheck,
  Trash2,
} from "lucide-react";
import clsx from "clsx";
import {
  deleteTargetPolicy,
  getTargetPolicy,
  putTargetPolicy,
  type TargetPolicyPayload,
} from "../api/targets";

type Props = {
  targetId: number;
  enabled?: boolean;
};

const defaultPolicy: TargetPolicyPayload = {
  platform_name: "",
  program_url: "",
  in_scope_patterns: [],
  out_of_scope_patterns: [],
  allowed_test_types: ["passive-recon", "safe-active-checks"],
  disallowed_test_types: ["destructive", "dos", "bruteforce", "credential-stuffing"],
  max_test_intensity: "safe",
  rate_limit_notes: "",
  auth_required: false,
  safe_testing_notes: "",
  operator_mode: "assisted_autopilot",
  auto_execute_level_0: true,
  auto_execute_level_1: true,
  require_approval_level_2: true,
  require_approval_level_3: true,
  reporting_preferences: "",
  business_context: "",
  asset_criticality_default: "medium",
};

const parseList = (value: any): string[] => {
  if (Array.isArray(value)) return value.map(String).filter(Boolean);
  if (typeof value === "string") {
    const trimmed = value.trim();
    if (!trimmed) return [];
    try {
      const parsed = JSON.parse(trimmed);
      if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean);
    } catch {
      return trimmed
        .split(/\r?\n|,/)
        .map((item) => item.trim())
        .filter(Boolean);
    }
  }
  return [];
};

const listToText = (items: string[]) => items.join("\n");

const textToList = (value: string) =>
  value
    .split(/\r?\n/)
    .map((item) => item.trim())
    .filter(Boolean);

const fromPolicy = (policy: any): TargetPolicyPayload => {
  if (!policy) return defaultPolicy;

  return {
    platform_name: policy.platform_name || "",
    program_url: policy.program_url || "",
    in_scope_patterns: parseList(policy.in_scope_patterns),
    out_of_scope_patterns: parseList(policy.out_of_scope_patterns),
    allowed_test_types: parseList(policy.allowed_test_types),
    disallowed_test_types: parseList(policy.disallowed_test_types),
    max_test_intensity: policy.max_test_intensity || "safe",
    rate_limit_notes: policy.rate_limit_notes || "",
    auth_required: Boolean(policy.auth_required),
    safe_testing_notes: policy.safe_testing_notes || "",
    operator_mode: policy.operator_mode || "assisted_autopilot",
    auto_execute_level_0: policy.auto_execute_level_0 ?? true,
    auto_execute_level_1: policy.auto_execute_level_1 ?? true,
    require_approval_level_2: policy.require_approval_level_2 ?? true,
    require_approval_level_3: policy.require_approval_level_3 ?? true,
    reporting_preferences: policy.reporting_preferences || "",
    business_context: policy.business_context || "",
    asset_criticality_default: policy.asset_criticality_default || "medium",
  };
};

const StatusPill = ({
  children,
  tone = "neutral",
}: {
  children: React.ReactNode;
  tone?: "neutral" | "primary" | "warning" | "danger";
}) => {
  const classes = {
    neutral: "border-hack-border text-hack-dim bg-black/30",
    primary: "border-hack-primary text-hack-primary bg-hack-primary/10",
    warning: "border-hack-warning text-hack-warning bg-hack-warning/10",
    danger: "border-hack-danger text-hack-danger bg-hack-danger/10",
  };

  return (
    <span
      className={clsx(
        "border px-2 py-1 font-mono text-[10px] uppercase tracking-wider",
        classes[tone],
      )}
    >
      {children}
    </span>
  );
};

const TextAreaList = ({
  label,
  value,
  onChange,
  placeholder,
  hint,
}: {
  label: string;
  value: string[];
  onChange: (next: string[]) => void;
  placeholder?: string;
  hint?: string;
}) => (
  <label className="block">
    <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
      {label}
    </span>
    <textarea
      value={listToText(value)}
      onChange={(event) => onChange(textToList(event.target.value))}
      rows={6}
      placeholder={placeholder}
      className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
    />
    {hint && <div className="mt-1 text-xs leading-5 text-hack-dim">{hint}</div>}
  </label>
);

const TargetPolicyPanel = ({ targetId, enabled = true }: Props) => {
  const queryClient = useQueryClient();
  const [form, setForm] = useState<TargetPolicyPayload>(defaultPolicy);
  const [message, setMessage] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["target-policy", targetId],
    queryFn: () => getTargetPolicy(targetId),
    enabled: Boolean(targetId) && enabled,
  });

  useEffect(() => {
    if (query.isSuccess) {
      setForm(fromPolicy(query.data));
    }
  }, [query.isSuccess, query.data]);

  const hasPolicy = Boolean(query.data?.id);

  const policyState = useMemo(() => {
    const hasScope = form.in_scope_patterns.length > 0;
    const hasOutOfScope = form.out_of_scope_patterns.length > 0;
    const intensity = form.max_test_intensity || "safe";

    if (!hasScope && !hasOutOfScope) {
      return {
        tone: "warning" as const,
        label: "policy incomplete",
        description:
          "No scope patterns are defined yet. Agents will treat policy as incomplete and avoid risky recommendations.",
      };
    }

    if (intensity === "manual-approved") {
      return {
        tone: "primary" as const,
        label: "manual approval first",
        description:
          "The target is configured for conservative, human-approved testing decisions.",
      };
    }

    return {
      tone: "primary" as const,
      label: "policy available",
      description:
        "Agents can use this policy as context. Deterministic guardrails remain authoritative.",
    };
  }, [form]);

  const saveMutation = useMutation({
    mutationFn: () => putTargetPolicy(targetId, form),
    onSuccess: () => {
      setMessage("Target policy saved");
      queryClient.invalidateQueries({ queryKey: ["target-policy", targetId] });
      queryClient.invalidateQueries({ queryKey: ["target-audit-logs", targetId] });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: () => deleteTargetPolicy(targetId),
    onSuccess: () => {
      setMessage("Target policy deleted");
      setForm(defaultPolicy);
      queryClient.invalidateQueries({ queryKey: ["target-policy", targetId] });
      queryClient.invalidateQueries({ queryKey: ["target-audit-logs", targetId] });
    },
  });

  if (!enabled) {
    return (
      <div className="border border-hack-border bg-black/30 p-6">
        <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-dim">
          <ShieldCheck className="h-5 w-5" /> Policy Disabled
        </h2>
        <p className="mt-2 text-sm text-hack-dim">
          Target policy configuration is disabled by feature flag.
        </p>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <div className="border border-hack-border bg-black/30 p-4">
        <div className="flex flex-col gap-3 xl:flex-row xl:items-start xl:justify-between">
          <div>
            <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
              <ShieldCheck className="h-5 w-5" /> Target Policy
            </h2>
            <p className="mt-1 max-w-4xl text-sm leading-6 text-hack-dim">
              Policy data defines scope, platform constraints, safe testing
              notes, and reporting context for AI agents and future
              approval-gated workflows. It does not authorize unsafe execution
              by itself.
            </p>
            <div className="mt-3 flex flex-wrap gap-2">
              <StatusPill tone={policyState.tone}>{policyState.label}</StatusPill>
              <StatusPill>
                max intensity: {form.max_test_intensity || "safe"}
              </StatusPill>
              <StatusPill>
                default criticality: {form.asset_criticality_default || "medium"}
              </StatusPill>
              <StatusPill tone={form.operator_mode === "strict_approval" ? "warning" : "primary"}>
                operator mode: {form.operator_mode || "assisted_autopilot"}
              </StatusPill>
              {form.auth_required && (
                <StatusPill tone="warning">auth required</StatusPill>
              )}
            </div>
            <p className="mt-2 text-xs leading-5 text-hack-dim">
              {policyState.description}
            </p>
          </div>

          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              onClick={() => {
                setMessage(null);
                setForm(defaultPolicy);
              }}
              className="hack-btn-ghost flex items-center gap-2 border border-hack-border px-3 py-2 text-xs"
            >
              <RotateCcw className="h-4 w-4" /> Reset Form
            </button>

            {hasPolicy && (
              <button
                type="button"
                onClick={() => {
                  if (confirm("Delete target policy?")) deleteMutation.mutate();
                }}
                disabled={deleteMutation.isPending}
                className="hack-btn-ghost flex items-center gap-2 border border-hack-danger/60 px-3 py-2 text-xs text-hack-danger disabled:opacity-50"
              >
                {deleteMutation.isPending ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Trash2 className="h-4 w-4" />
                )}
                Delete
              </button>
            )}

            <button
              type="button"
              onClick={() => {
                setMessage(null);
                saveMutation.mutate();
              }}
              disabled={saveMutation.isPending}
              className="hack-btn flex items-center gap-2 px-4 py-2 text-xs disabled:opacity-50"
            >
              {saveMutation.isPending ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <Save className="h-4 w-4" />
              )}
              Save Policy
            </button>
          </div>
        </div>

        {message && (
          <div className="mt-3 border border-hack-primary/60 bg-hack-primary/10 p-3 text-sm text-hack-primary">
            {message}
          </div>
        )}

        {(saveMutation.isError || deleteMutation.isError) && (
          <div className="mt-3 border border-hack-danger/60 bg-hack-danger/10 p-3 text-sm text-hack-danger">
            {((saveMutation.error || deleteMutation.error) as any)?.response?.data
              ?.message || "Policy operation failed"}
          </div>
        )}
      </div>

      {query.isLoading ? (
        <div className="border border-hack-border bg-black/20 p-6 text-center font-mono text-hack-dim">
          Loading target policy...
        </div>
      ) : (
        <div className="grid gap-4 xl:grid-cols-2">
          <div className="space-y-4 border border-hack-border bg-black/20 p-4">
            <div className="font-mono text-sm uppercase tracking-wider text-hack-primary">
              Program Context
            </div>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Platform Name
              </span>
              <input
                value={form.platform_name}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    platform_name: event.target.value,
                  }))
                }
                placeholder="HackerOne / Bugcrowd / Private Program"
                className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-sm text-white outline-none focus:border-hack-primary"
              />
            </label>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Program URL
              </span>
              <input
                value={form.program_url}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    program_url: event.target.value,
                  }))
                }
                placeholder="https://hackerone.com/example"
                className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-sm text-white outline-none focus:border-hack-primary"
              />
            </label>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Business Context
              </span>
              <textarea
                value={form.business_context}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    business_context: event.target.value,
                  }))
                }
                rows={5}
                placeholder="Customer-facing app, admin surfaces, sensitive flows, business priorities..."
                className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Reporting Preferences
              </span>
              <textarea
                value={form.reporting_preferences}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    reporting_preferences: event.target.value,
                  }))
                }
                rows={5}
                placeholder="Preferred report style, platform wording, severity preferences, required evidence..."
                className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>
          </div>

          <div className="space-y-4 border border-hack-border bg-black/20 p-4">
            <div className="font-mono text-sm uppercase tracking-wider text-hack-primary">
              Scope Rules
            </div>

            <TextAreaList
              label="In-scope Patterns"
              value={form.in_scope_patterns}
              onChange={(next) =>
                setForm((prev) => ({ ...prev, in_scope_patterns: next }))
              }
              placeholder={"*.example.com\napp.example.com\nhttps://example.com/api/*"}
              hint="One pattern per line. These are descriptive scope rules for agents and future policy checks."
            />

            <TextAreaList
              label="Out-of-scope Patterns"
              value={form.out_of_scope_patterns}
              onChange={(next) =>
                setForm((prev) => ({ ...prev, out_of_scope_patterns: next }))
              }
              placeholder={"*.thirdparty.com\nstatus.example.com\n/payment/destructive-flow"}
              hint="Use this to warn or block future agent recommendations and approval workflows."
            />

            <div className="border border-hack-warning/40 bg-hack-warning/10 p-3 text-xs leading-5 text-hack-warning">
              <AlertTriangle className="mr-2 inline h-4 w-4" />
              Out-of-scope rules are not vulnerability findings. They are policy
              constraints for future agent planning and human approval.
            </div>
          </div>

          <div className="space-y-4 border border-hack-border bg-black/20 p-4">
            <div className="font-mono text-sm uppercase tracking-wider text-hack-primary">
              Testing Controls
            </div>

            <div className="grid gap-3 md:grid-cols-2">
              <label className="block">
                <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                  Max Test Intensity
                </span>
                <select
                  value={form.max_test_intensity}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      max_test_intensity: event.target.value,
                    }))
                  }
                  className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-sm text-white outline-none focus:border-hack-primary"
                >
                  <option value="passive">Passive</option>
                  <option value="safe">Safe</option>
                  <option value="balanced">Balanced</option>
                  <option value="manual-approved">Manual-approved</option>
                </select>
              </label>

              <label className="block">
                <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                  Default Asset Criticality
                </span>
                <select
                  value={form.asset_criticality_default}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      asset_criticality_default: event.target.value,
                    }))
                  }
                  className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-sm text-white outline-none focus:border-hack-primary"
                >
                  <option value="low">Low</option>
                  <option value="medium">Medium</option>
                  <option value="high">High</option>
                  <option value="critical">Critical</option>
                </select>
              </label>
            </div>

            <div className="space-y-3 border border-hack-primary/30 bg-hack-primary/5 p-3">
              <div className="font-mono text-xs uppercase tracking-wider text-hack-primary">
                Operator Automation
              </div>

              <label className="block">
                <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                  Operator Mode
                </span>
                <select
                  value={form.operator_mode || "assisted_autopilot"}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      operator_mode: event.target.value,
                    }))
                  }
                  className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-sm text-white outline-none focus:border-hack-primary"
                >
                  <option value="manual_only">Manual only</option>
                  <option value="assisted_autopilot">Assisted Autopilot</option>
                  <option value="strict_approval">Strict Approval</option>
                </select>
                <div className="mt-1 text-xs leading-5 text-hack-dim">
                  Assisted Autopilot can execute low-risk controlled actions when target policy permits. Strict Approval requires explicit approval for every action.
                </div>
              </label>

              <label className="flex cursor-pointer items-center justify-between gap-3 border border-hack-border bg-black/20 p-3 font-mono text-sm">
                <span>
                  <span className="block text-white">Auto execute Level 0</span>
                  <span className="mt-1 block text-xs text-hack-dim">
                    Allow passive/very-low-risk operator actions to run automatically.
                  </span>
                </span>
                <input
                  type="checkbox"
                  checked={Boolean(form.auto_execute_level_0)}
                  disabled={form.operator_mode === "strict_approval" || form.operator_mode === "manual_only"}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      auto_execute_level_0: event.target.checked,
                    }))
                  }
                  className="h-4 w-4 accent-hack-primary disabled:opacity-40"
                />
              </label>

              <label className="flex cursor-pointer items-center justify-between gap-3 border border-hack-border bg-black/20 p-3 font-mono text-sm">
                <span>
                  <span className="block text-white">Auto execute Level 1</span>
                  <span className="mt-1 block text-xs text-hack-dim">
                    Allow low-risk controlled endpoint review/probe actions to run automatically.
                  </span>
                </span>
                <input
                  type="checkbox"
                  checked={Boolean(form.auto_execute_level_1)}
                  disabled={form.operator_mode === "strict_approval" || form.operator_mode === "manual_only"}
                  onChange={(event) =>
                    setForm((prev) => ({
                      ...prev,
                      auto_execute_level_1: event.target.checked,
                    }))
                  }
                  className="h-4 w-4 accent-hack-primary disabled:opacity-40"
                />
              </label>

              <div className="grid gap-3 md:grid-cols-2">
                <label className="flex cursor-pointer items-center justify-between gap-3 border border-hack-border bg-black/20 p-3 font-mono text-sm">
                  <span>
                    <span className="block text-white">Require approval Level 2</span>
                    <span className="mt-1 block text-xs text-hack-dim">
                      Controlled active validation should stay approval-gated.
                    </span>
                  </span>
                  <input
                    type="checkbox"
                    checked={form.require_approval_level_2 ?? true}
                    onChange={(event) =>
                      setForm((prev) => ({
                        ...prev,
                        require_approval_level_2: event.target.checked,
                      }))
                    }
                    className="h-4 w-4 accent-hack-primary"
                  />
                </label>

                <label className="flex cursor-pointer items-center justify-between gap-3 border border-hack-border bg-black/20 p-3 font-mono text-sm">
                  <span>
                    <span className="block text-white">Require approval Level 3</span>
                    <span className="mt-1 block text-xs text-hack-dim">
                      Exploit validation, payload execution, auth/rate-limit testing, and brute-force workflows must be explicitly approved.
                    </span>
                  </span>
                  <input
                    type="checkbox"
                    checked={form.require_approval_level_3 ?? true}
                    onChange={(event) =>
                      setForm((prev) => ({
                        ...prev,
                        require_approval_level_3: event.target.checked,
                      }))
                    }
                    className="h-4 w-4 accent-hack-primary"
                  />
                </label>
              </div>
            </div>

            <label className="flex cursor-pointer items-center justify-between gap-3 border border-hack-border bg-black/20 p-3 font-mono text-sm">
              <span>
                <span className="block text-white">Authentication Required</span>
                <span className="mt-1 block text-xs text-hack-dim">
                  Mark this when meaningful testing requires an authenticated
                  account or customer context.
                </span>
              </span>
              <input
                type="checkbox"
                checked={form.auth_required}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    auth_required: event.target.checked,
                  }))
                }
                className="h-4 w-4 accent-hack-primary"
              />
            </label>

            <TextAreaList
              label="Allowed Test Types"
              value={form.allowed_test_types}
              onChange={(next) =>
                setForm((prev) => ({ ...prev, allowed_test_types: next }))
              }
              placeholder={"passive-recon\nsafe-active-checks\nheaders\ncors"}
            />

            <TextAreaList
              label="Disallowed Test Types"
              value={form.disallowed_test_types}
              onChange={(next) =>
                setForm((prev) => ({ ...prev, disallowed_test_types: next }))
              }
              placeholder={"destructive\ndos\nbruteforce\ncredential-stuffing"}
            />
          </div>

          <div className="space-y-4 border border-hack-border bg-black/20 p-4">
            <div className="font-mono text-sm uppercase tracking-wider text-hack-primary">
              Safety Notes
            </div>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Rate Limit Notes
              </span>
              <textarea
                value={form.rate_limit_notes}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    rate_limit_notes: event.target.value,
                  }))
                }
                rows={6}
                placeholder="Known rate limits, testing windows, request pacing, platform constraints..."
                className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>

            <label className="block">
              <span className="mb-2 block font-mono text-[10px] uppercase tracking-widest text-hack-dim">
                Safe Testing Notes
              </span>
              <textarea
                value={form.safe_testing_notes}
                onChange={(event) =>
                  setForm((prev) => ({
                    ...prev,
                    safe_testing_notes: event.target.value,
                  }))
                }
                rows={8}
                placeholder="Do not test payment flows. Avoid account takeover attempts. Only use non-destructive probes..."
                className="w-full border border-hack-border bg-black/30 px-3 py-2 font-mono text-xs text-white outline-none focus:border-hack-primary"
              />
            </label>

            <div className="border border-hack-primary/40 bg-hack-primary/10 p-3 text-xs leading-5 text-hack-primary">
              <CheckCircle2 className="mr-2 inline h-4 w-4" />
              Deterministic product logic remains authoritative for severity,
              priority, validity, policy enforcement, and execution decisions.
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default TargetPolicyPanel;
