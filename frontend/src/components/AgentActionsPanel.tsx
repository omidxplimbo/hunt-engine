import { useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  CheckCircle2,
  ClipboardList,
  Loader2,
  RefreshCw,
  ShieldAlert,
  XCircle,
} from "lucide-react";
import clsx from "clsx";
import {
  approveTargetAgentAction,
  getTargetAgentActions,
  proposeTargetAgentAction,
  rejectTargetAgentAction,
  type TargetAgentAction,
} from "../api/targets";

type Props = {
  targetId: number;
  enabled?: boolean;
};

const parseJSONValue = (value: any) => {
  if (!value) return {};
  if (typeof value === "string") {
    try {
      return JSON.parse(value);
    } catch {
      return {};
    }
  }
  return value;
};

const toneForStatus = (status: string) => {
  switch (String(status || "").toLowerCase()) {
    case "approved":
    case "executed":
      return "primary";
    case "proposed":
      return "warning";
    case "blocked_by_policy":
    case "failed":
    case "rejected":
      return "danger";
    default:
      return "neutral";
  }
};

const toneForPolicy = (status: string) => {
  switch (String(status || "").toLowerCase()) {
    case "allowed":
      return "primary";
    case "warning":
    case "unknown":
      return "warning";
    case "blocked":
      return "danger";
    default:
      return "neutral";
  }
};

const Pill = ({
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
        "inline-flex border px-2 py-1 font-mono text-[10px] uppercase tracking-wider",
        classes[tone],
      )}
    >
      {children}
    </span>
  );
};

const formatDate = (value?: string | null) => {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
};

const ActionCard = ({
  action,
  busy,
  onApprove,
  onReject,
}: {
  action: TargetAgentAction;
  busy: boolean;
  onApprove: (action: TargetAgentAction) => void;
  onReject: (action: TargetAgentAction) => void;
}) => {
  const policyCheck = parseJSONValue(action.policy_check_json);
  const canApprove =
    action.status === "proposed" && action.policy_status !== "blocked";
  const canReject =
    action.status === "proposed" || action.status === "approved";

  return (
    <div className="border border-hack-border bg-black/20 p-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0">
          <div className="break-words font-mono text-sm text-white">
            #{action.id} · {action.title || action.action_type}
          </div>
          <div className="mt-1 break-all font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            {action.action_type}
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          <Pill tone={toneForStatus(action.status) as any}>
            {action.status}
          </Pill>
          <Pill tone={toneForPolicy(action.policy_status) as any}>
            policy: {action.policy_status}
          </Pill>
          <Pill>risk: {action.risk_level}</Pill>
          <Pill>level: {action.test_level}</Pill>
        </div>
      </div>

      {action.description && (
        <p className="mt-3 text-sm text-hack-dim">{action.description}</p>
      )}

      <div className="mt-3 grid gap-2 md:grid-cols-4">
        <div className="border border-hack-border/70 bg-black/30 p-2">
          <div className="font-mono text-[10px] uppercase text-hack-dim">
            Safety
          </div>
          <div className="font-mono text-sm text-white">
            {action.safety_level}
          </div>
        </div>
        <div className="border border-hack-border/70 bg-black/30 p-2">
          <div className="font-mono text-[10px] uppercase text-hack-dim">
            Autonomy
          </div>
          <div className="font-mono text-sm text-white">
            {action.autonomy_level}
          </div>
        </div>
        <div className="border border-hack-border/70 bg-black/30 p-2">
          <div className="font-mono text-[10px] uppercase text-hack-dim">
            Created
          </div>
          <div className="font-mono text-xs text-white">
            {formatDate(action.created_at)}
          </div>
        </div>
        <div className="border border-hack-border/70 bg-black/30 p-2">
          <div className="font-mono text-[10px] uppercase text-hack-dim">
            Approval
          </div>
          <div className="font-mono text-xs text-white">
            {action.requires_approval ? "required" : "not required"}
          </div>
        </div>
      </div>

      {policyCheck?.reason && (
        <div className="mt-3 border border-hack-warning/50 bg-hack-warning/10 p-3 text-sm text-hack-warning">
          <div className="mb-1 flex items-center gap-2 font-mono text-[10px] uppercase tracking-wider">
            <AlertTriangle className="h-3 w-3" /> Policy Check
          </div>
          {policyCheck.reason}
        </div>
      )}

      {action.error_message && (
        <div className="mt-3 border border-hack-danger/50 bg-hack-danger/10 p-3 text-sm text-hack-danger">
          {action.error_message}
        </div>
      )}

      <div className="mt-4 flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => onApprove(action)}
          disabled={!canApprove || busy}
          className="hack-btn border border-hack-primary px-3 py-1 text-[10px] uppercase tracking-wider text-hack-primary disabled:opacity-50"
        >
          <CheckCircle2 className="h-3 w-3" /> Approve
        </button>

        <button
          type="button"
          onClick={() => onReject(action)}
          disabled={!canReject || busy}
          className="hack-btn-ghost border border-hack-danger/60 px-3 py-1 text-[10px] uppercase tracking-wider text-hack-danger disabled:opacity-50"
        >
          <XCircle className="h-3 w-3" /> Reject
        </button>
      </div>
    </div>
  );
};

const AgentActionsPanel = ({ targetId, enabled = true }: Props) => {
  const queryClient = useQueryClient();
  const [message, setMessage] = useState<string | null>(null);

  const query = useQuery({
    queryKey: ["targets", targetId, "agent-actions"],
    queryFn: () => getTargetAgentActions(targetId, 30),
    enabled: Boolean(targetId) && enabled,
    refetchInterval: 20_000,
  });

  const actions = query.data?.data || [];

  const counts = useMemo(() => {
    return actions.reduce(
      (acc, item) => {
        acc.total += 1;
        acc[item.status] = (acc[item.status] || 0) + 1;
        return acc;
      },
      { total: 0 } as Record<string, number>,
    );
  }, [actions]);

  const refresh = () => {
    queryClient.invalidateQueries({ queryKey: ["targets", targetId, "agent-actions"] });
  };

  const proposeMutation = useMutation({
    mutationFn: () =>
      proposeTargetAgentAction(targetId, {
        action_type: "run_owasp_checklist",
        title: "OWASP passive checklist proposal",
        description:
          "Foundation action proposal for v3.7.0. This creates an approval-gated passive OWASP checklist action without executing tests.",
        risk_level: "low",
        safety_level: 0,
        test_level: 0,
        autonomy_level: 0,
        requested_by_agent: true,
        requires_approval: true,
        input_json: {
          standard: "owasp",
          profile: "passive",
          execution_enabled: false,
        },
      }),
    onSuccess: () => {
      setMessage("Agent action proposed");
      refresh();
    },
  });

  const approveMutation = useMutation({
    mutationFn: (action: TargetAgentAction) =>
      approveTargetAgentAction(targetId, action.id, "approved from target analysis UI"),
    onSuccess: () => {
      setMessage("Agent action approved");
      refresh();
    },
  });

  const rejectMutation = useMutation({
    mutationFn: (action: TargetAgentAction) =>
      rejectTargetAgentAction(targetId, action.id, "rejected from target analysis UI"),
    onSuccess: () => {
      setMessage("Agent action rejected");
      refresh();
    },
  });

  const busy =
    proposeMutation.isPending ||
    approveMutation.isPending ||
    rejectMutation.isPending;

  if (!enabled) {
    return null;
  }

  return (
    <div id="agent-actions-panel" className="mt-6 border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
            <ShieldAlert className="h-5 w-5" /> Agent Actions
          </h2>
          <p className="mt-1 max-w-4xl text-sm text-hack-dim">
            v3.7.0 foundation for proposed, approved, rejected, and
            policy-blocked actions. Execution is intentionally disabled in this
            first step; this panel validates the approval workflow and audit
            chain.
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={refresh}
            disabled={query.isFetching}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white disabled:opacity-50"
          >
            <RefreshCw className={clsx("h-3 w-3", query.isFetching && "animate-spin")} />
            Refresh
          </button>

          <button
            type="button"
            onClick={() => proposeMutation.mutate()}
            disabled={busy}
            className="hack-btn border border-hack-primary px-3 py-1 text-[10px] uppercase tracking-wider text-hack-primary disabled:opacity-50"
          >
            {proposeMutation.isPending ? (
              <Loader2 className="h-3 w-3 animate-spin" />
            ) : (
              <ClipboardList className="h-3 w-3" />
            )}
            Propose OWASP Passive Action
          </button>
        </div>
      </div>

      {message && (
        <div className="mb-3 border border-hack-primary/50 bg-hack-primary/10 p-3 font-mono text-sm text-hack-primary">
          {message}
        </div>
      )}

      {(proposeMutation.error || approveMutation.error || rejectMutation.error) && (
        <div className="mb-3 border border-hack-danger/60 bg-hack-danger/10 p-3 font-mono text-sm text-hack-danger">
          {(proposeMutation.error as any)?.response?.data?.message ||
            (approveMutation.error as any)?.response?.data?.message ||
            (rejectMutation.error as any)?.response?.data?.message ||
            (proposeMutation.error as any)?.message ||
            (approveMutation.error as any)?.message ||
            (rejectMutation.error as any)?.message ||
            "Agent action operation failed"}
        </div>
      )}

      <div className="mb-4 grid gap-3 md:grid-cols-4">
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Total
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {counts.total || 0}
          </div>
        </div>
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Proposed
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {counts.proposed || 0}
          </div>
        </div>
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Approved
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {counts.approved || 0}
          </div>
        </div>
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Blocked
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {counts.blocked_by_policy || 0}
          </div>
        </div>
      </div>

      {query.isLoading ? (
        <div className="flex items-center gap-2 border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
          <Loader2 className="h-4 w-4 animate-spin" /> Loading agent actions...
        </div>
      ) : actions.length === 0 ? (
        <div className="border border-hack-border bg-black/20 p-4 text-sm text-hack-dim">
          No agent actions yet. Use <span className="text-hack-primary">Propose OWASP Passive Action</span> to create the first approval-gated action.
        </div>
      ) : (
        <div className="space-y-3">
          {actions.map((action) => (
            <ActionCard
              key={action.id}
              action={action}
              busy={busy}
              onApprove={(item) => approveMutation.mutate(item)}
              onReject={(item) => rejectMutation.mutate(item)}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default AgentActionsPanel;
