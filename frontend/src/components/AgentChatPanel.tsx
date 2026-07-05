import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  Bot,
  Loader2,
  MessageSquare,
  Plus,
  RefreshCw,
  Send,
  ShieldCheck,
  Trash2,
  User,
} from "lucide-react";
import clsx from "clsx";
import {
  createTargetAgentChatMessage,
  createTargetAgentChatSession,
  deleteTargetAgentChatSession,
  getTargetAgentChatMessages,
  getTargetAgentChatSessions,
  type TargetAgentChatMessage,
  type TargetAgentChatSession,
} from "../api/targets";

type Props = {
  targetId: number;
  enabled?: boolean;
  onJumpToActions?: () => void;
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

const formatDate = (value?: string | null) => {
  if (!value) return "-";
  const d = new Date(value);
  return Number.isNaN(d.getTime()) ? value : d.toLocaleString();
};

const asArray = (value: any): any[] => (Array.isArray(value) ? value : []);

const truncateText = (value: any, max = 180) => {
  const text = String(value || "").trim();
  if (!text) return "";
  return text.length > max ? `${text.slice(0, max)}...` : text;
};

const actionLabel = (value: string) =>
  String(value || "")
    .replaceAll("_", " ")
    .replace(/\b\w/g, (ch) => ch.toUpperCase());

const methodologyTitle = (item: any) =>
  String(item?.title || item?.name || "Untitled methodology").trim();

const methodologySkillSlug = (item: any) =>
  String(item?.skill_slug || item?.skillSlug || "").trim();

const methodologyBugClass = (item: any) =>
  String(item?.bug_class || item?.bugClass || "").trim();

const methodologySummary = (item: any) =>
  String(item?.summary || item?.content || "").trim();

const buildAppliedMethodologyRows = (selectedSkills: any[]) => {
  const rows: Array<{
    key: string;
    title: string;
    skillSlug: string;
    bugClass: string;
    summary: string;
  }> = [];
  const seen = new Set<string>();

  selectedSkills.forEach((skill) => {
    const skillSlug = String(skill?.slug || "").trim();
    const applied = asArray(skill?.applied_methodologies);
    applied.forEach((methodology) => {
      const title = methodologyTitle(methodology);
      const bugClass = methodologyBugClass(methodology);
      const summary = methodologySummary(methodology);
      const key = `${skillSlug}:${methodology?.id || title}`;
      if (!title || seen.has(key)) return;
      seen.add(key);
      rows.push({
        key,
        title,
        skillSlug,
        bugClass,
        summary,
      });
    });
  });

  return rows;
};

const MessageBubble = ({
  message,
  onJumpToActions,
}: {
  message: TargetAgentChatMessage;
  onJumpToActions?: () => void;
}) => {
  const isUser = message.role === "user";
  const output = parseJSONValue(message.output_json);
  const input = parseJSONValue(message.input_json);
  const actionIds = Array.isArray(output?.action_ids) ? output.action_ids : [];
  const reusedActionIds = Array.isArray(output?.reused_action_ids)
    ? output.reused_action_ids
    : [];
  const actionsCreated = Number(output?.actions_created || 0);
  const actionsReused = Number(output?.actions_reused || 0);
  const operatorPlan = output?.operator_plan || {};
  const recommendedSteps = asArray(operatorPlan?.recommended_next_steps);
  const planActions = asArray(operatorPlan?.actions);
  const guardrails = asArray(output?.guardrails);
  const autopilot = output?.autopilot || {};
  const autopilotSummary = asArray(autopilot?.summary);
  const autopilotExecuted = asArray(autopilot?.executed_actions);
  const autopilotSkipped = asArray(autopilot?.skipped_actions);
  const autopilotRuns = asArray(autopilot?.controlled_runs);
  const autopilotResults = asArray(autopilot?.controlled_results);
  const autopilotErrors = asArray(autopilot?.errors);

  const selectedSkills = asArray(output?.selected_skills);
  const selectedMethodologies = asArray(output?.selected_methodologies);
  const methodologyContextUsed = Boolean(output?.operator_methodology_context_used);
  const appliedMethodologyRows = buildAppliedMethodologyRows(selectedSkills);

  const numericActionIds = actionIds
    .map((item: any) => Number(item))
    .filter((item: number) => Number.isFinite(item) && item > 0);

  const executedActionIds = autopilotExecuted
    .map((item: any) => Number(item))
    .filter((item: number) => Number.isFinite(item) && item > 0);

  const skippedActionIds = autopilotSkipped
    .map((item: any) => Number(item?.action_id || item))
    .filter((item: number) => Number.isFinite(item) && item > 0);

  const actionRuntimeState = (action: any, index: number) => {
    const directId = Number(action?.id || action?.action_id || action?.agent_action_id || numericActionIds[index]);
    const inferredSingleId =
      planActions.length === 1 && numericActionIds.length === 1 ? numericActionIds[0] : 0;
    const actionId = Number.isFinite(directId) && directId > 0 ? directId : inferredSingleId;

    if (actionId && executedActionIds.includes(actionId)) {
      return {
        state: "executed",
        actionId,
        label: "Executed by Autopilot",
      };
    }
    if (actionId && skippedActionIds.includes(actionId)) {
      return {
        state: "skipped",
        actionId,
        label: "Approval Required",
      };
    }
    if (planActions.length === 1 && executedActionIds.length > 0) {
      return {
        state: "executed",
        actionId: executedActionIds[0],
        label: "Executed by Autopilot",
      };
    }
    if (planActions.length === 1 && autopilotSkipped.length > 0) {
      return {
        state: "skipped",
        actionId: skippedActionIds[0] || 0,
        label: "Approval Required",
      };
    }

    return {
      state: "proposed",
      actionId: actionId || 0,
      label: "Proposed Action",
    };
  };

  const actionRuntimeToneClass = (state: string) => {
    if (state === "executed") return "border-hack-primary/60 bg-hack-primary/10 text-hack-primary";
    if (state === "skipped") return "border-hack-warning/60 bg-hack-warning/10 text-hack-warning";
    return "border-hack-border bg-black/30 text-hack-dim";
  };

  const llmAssisted = Boolean(output?.llm_assisted);
  const operatorMode = String(output?.operator_mode || input?.operator_mode || "");
  const operatorError = String(output?.operator_error || input?.operator_error || "");
  const provider = operatorPlan?.llm_provider;
  const model = operatorPlan?.llm_model;

  return (
    <div
      className={clsx(
        "flex gap-3",
        isUser ? "justify-end" : "justify-start",
      )}
    >
      {!isUser && (
        <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center border border-hack-primary/60 bg-hack-primary/10 text-hack-primary">
          <Bot className="h-4 w-4" />
        </div>
      )}

      <div
        className={clsx(
          "max-w-4xl border p-3",
          isUser
            ? "border-hack-border bg-black/40"
            : "border-hack-primary/40 bg-hack-primary/10",
        )}
      >
        <div className="mb-1 flex flex-wrap items-center gap-2 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
          <span>{isUser ? "You" : "Attack Surface Agent"}</span>
          <span>·</span>
          <span>{formatDate(message.created_at)}</span>
          <span>·</span>
          <span>{message.message_type}</span>
        </div>

        <div dir="auto" className="whitespace-pre-wrap text-sm text-white">
          {message.content}
        </div>

        {!isUser && operatorMode && (
          <div className="mt-3 flex flex-wrap gap-2 font-mono text-[10px] uppercase tracking-wider">
            <span
              className={clsx(
                "border px-2 py-1",
                llmAssisted
                  ? "border-hack-primary/60 bg-hack-primary/10 text-hack-primary"
                  : "border-hack-warning/60 bg-hack-warning/10 text-hack-warning",
              )}
            >
              {llmAssisted ? "LLM Assisted" : "Fallback"}
            </span>
            <span className="border border-hack-border bg-black/30 px-2 py-1 text-hack-dim">
              {operatorMode}
            </span>
            {provider && (
              <span className="border border-hack-border bg-black/30 px-2 py-1 text-hack-dim">
                {provider} / {model || "-"}
              </span>
            )}
          </div>
        )}

        {!isUser && (methodologyContextUsed || selectedMethodologies.length > 0 || appliedMethodologyRows.length > 0) && (
          <div className="mt-3 border border-purple-400/40 bg-purple-500/10 p-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div className="font-mono text-[10px] uppercase tracking-wider text-purple-300">
                Operator Methodology Applied
              </div>
              <div className="flex flex-wrap gap-2 font-mono text-[10px] uppercase tracking-wider">
                <span className="border border-purple-400/50 bg-black/30 px-2 py-1 text-purple-300">
                  context: {methodologyContextUsed ? "used" : "not used"}
                </span>
                <span className="border border-purple-400/50 bg-black/30 px-2 py-1 text-purple-300">
                  records: {selectedMethodologies.length}
                </span>
                <span className="border border-purple-400/50 bg-black/30 px-2 py-1 text-purple-300">
                  applied: {appliedMethodologyRows.length}
                </span>
              </div>
            </div>

            {appliedMethodologyRows.length > 0 ? (
              <div className="space-y-2">
                {appliedMethodologyRows.slice(0, 6).map((row) => (
                  <div key={`${message.id}-methodology-${row.key}`} className="border border-purple-400/30 bg-black/30 p-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-xs text-white">{row.title}</span>
                      {row.skillSlug && (
                        <span className="rounded border border-purple-400/40 bg-purple-500/10 px-1.5 py-0.5 font-mono text-[10px] text-purple-300">
                          → {row.skillSlug}
                        </span>
                      )}
                      {row.bugClass && (
                        <span className="rounded border border-blue-400/40 bg-blue-500/10 px-1.5 py-0.5 font-mono text-[10px] text-blue-300">
                          {row.bugClass}
                        </span>
                      )}
                    </div>
                    {row.summary && (
                      <div className="mt-1 text-xs text-hack-dim">
                        {truncateText(row.summary, 220)}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            ) : selectedMethodologies.length > 0 ? (
              <div className="space-y-2">
                {selectedMethodologies.slice(0, 6).map((methodology: any, index: number) => {
                  const title = methodologyTitle(methodology);
                  const skillSlug = methodologySkillSlug(methodology);
                  const bugClass = methodologyBugClass(methodology);
                  const summary = methodologySummary(methodology);
                  return (
                    <div key={`${message.id}-selected-methodology-${methodology?.id || index}`} className="border border-purple-400/30 bg-black/30 p-2">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-mono text-xs text-white">{title}</span>
                        {skillSlug && (
                          <span className="rounded border border-purple-400/40 bg-purple-500/10 px-1.5 py-0.5 font-mono text-[10px] text-purple-300">
                            skill: {skillSlug}
                          </span>
                        )}
                        {bugClass && (
                          <span className="rounded border border-blue-400/40 bg-blue-500/10 px-1.5 py-0.5 font-mono text-[10px] text-blue-300">
                            bug: {bugClass}
                          </span>
                        )}
                      </div>
                      {summary && (
                        <div className="mt-1 text-xs text-hack-dim">
                          {truncateText(summary, 220)}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="text-xs text-hack-dim">
                Methodology context was passed to the operator planner, but no per-skill methodology attachment was reported for this message.
              </div>
            )}
          </div>
        )}

        {!isUser && operatorError && (
          <div className="mt-3 border border-hack-warning/60 bg-hack-warning/10 p-2 font-mono text-[11px] text-hack-warning">
            Operator fallback reason: {truncateText(operatorError, 260)}
          </div>
        )}

        {!isUser && autopilot?.enabled && (
          <div className="mt-3 border border-hack-primary/50 bg-hack-primary/10 p-3">
            <div className="mb-2 flex flex-wrap items-center justify-between gap-2">
              <div className="font-mono text-[10px] uppercase tracking-wider text-hack-primary">
                Operator Autopilot
              </div>
              <div className="flex flex-wrap gap-2 font-mono text-[10px] uppercase tracking-wider">
                <span className="border border-hack-primary/60 bg-black/30 px-2 py-1 text-hack-primary">
                  mode: {autopilot.mode || "assisted_autopilot_v1"}
                </span>
                <span className="border border-hack-primary/60 bg-black/30 px-2 py-1 text-hack-primary">
                  executed: {autopilotExecuted.length}
                </span>
                <span className="border border-hack-primary/60 bg-black/30 px-2 py-1 text-hack-primary">
                  runs: {autopilotRuns.join(", ") || "-"}
                </span>
                <span className="border border-hack-primary/60 bg-black/30 px-2 py-1 text-hack-primary">
                  results: {autopilotResults.join(", ") || "-"}
                </span>
                <span className={clsx(
                  "border bg-black/30 px-2 py-1",
                  autopilot.memory_ingested
                    ? "border-hack-primary/60 text-hack-primary"
                    : "border-hack-warning/60 text-hack-warning",
                )}>
                  memory: {autopilot.memory_ingested ? "ingested" : "not ingested"}
                </span>
              </div>
            </div>

            {autopilotSummary.length > 0 && (
              <div className="space-y-2">
                {autopilotSummary.slice(0, 4).map((item: any, index: number) => (
                  <div key={`${message.id}-autopilot-${index}`} className="border border-hack-border bg-black/30 p-2">
                    <div className="grid gap-2 font-mono text-[11px] md:grid-cols-5">
                      <div>
                        <div className="text-[10px] uppercase text-hack-dim">Action</div>
                        <div className="text-white">#{item.action_id || "-"}</div>
                      </div>
                      <div>
                        <div className="text-[10px] uppercase text-hack-dim">Run</div>
                        <div className="text-white">#{item.run_id || "-"}</div>
                      </div>
                      <div>
                        <div className="text-[10px] uppercase text-hack-dim">Result</div>
                        <div className="text-white">#{item.result_id || "-"}</div>
                      </div>
                      <div>
                        <div className="text-[10px] uppercase text-hack-dim">Status</div>
                        <div className="text-white">{item.status || "-"}</div>
                      </div>
                      <div>
                        <div className="text-[10px] uppercase text-hack-dim">HTTP</div>
                        <div className="text-white">{item.status_code || "-"}</div>
                      </div>
                    </div>
                    {item.error && (
                      <div className="mt-2 border border-hack-danger/40 bg-hack-danger/10 p-2 text-xs text-hack-danger">
                        {truncateText(item.error, 220)}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            {autopilotErrors.length > 0 && (
              <div className="mt-2 border border-hack-warning/50 bg-hack-warning/10 p-2 text-xs text-hack-warning">
                {autopilotErrors.slice(0, 3).map((item: any, index: number) => (
                  <div key={`${message.id}-autopilot-error-${index}`}>
                    • {truncateText(item, 220)}
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

        {!isUser && recommendedSteps.length > 0 && (
          <div className="mt-3 border border-hack-border bg-black/30 p-3">
            <div className="mb-2 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
              Recommended Next Steps
            </div>
            <ul className="space-y-1 text-xs text-white">
              {recommendedSteps.slice(0, 6).map((step, index) => (
                <li key={`${message.id}-step-${index}`}>• {truncateText(step, 220)}</li>
              ))}
            </ul>
          </div>
        )}

        {!isUser && planActions.length > 0 && (
          <div className="mt-3 border border-hack-primary/40 bg-hack-primary/10 p-3">
            <div className="mb-2 font-mono text-[10px] uppercase tracking-wider text-hack-primary">
              Operator Actions
            </div>
            <div className="space-y-2">
              {planActions.slice(0, 5).map((action, index) => {
                const runtime = actionRuntimeState(action, index);
                return (
                  <div key={`${message.id}-action-${index}`} className="border border-hack-border bg-black/30 p-2">
                    <div className="flex flex-wrap items-start justify-between gap-2">
                      <div className="font-mono text-xs font-bold text-white">
                        {action.title || actionLabel(action.action_type)}
                      </div>
                      <span
                        className={clsx(
                          "border px-2 py-1 font-mono text-[10px] uppercase tracking-wider",
                          actionRuntimeToneClass(runtime.state),
                        )}
                      >
                        {runtime.label}
                      </span>
                    </div>
                    <div className="mt-1 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
                      {actionLabel(action.action_type)} · risk {action.risk_level || "low"} · safety {action.safety_level ?? 0} · test {action.test_level ?? 0}
                      {runtime.actionId ? ` · action #${runtime.actionId}` : ""}
                    </div>
                    {runtime.state === "executed" && (
                      <div className="mt-2 border border-hack-primary/40 bg-hack-primary/10 p-2 text-xs text-hack-primary">
                        This action was executed automatically by Operator Autopilot under the current target policy.
                      </div>
                    )}
                    {runtime.state === "skipped" && (
                      <div className="mt-2 border border-hack-warning/40 bg-hack-warning/10 p-2 text-xs text-hack-warning">
                        This action was not executed automatically. Current target policy requires explicit approval.
                      </div>
                    )}
                    {action.reason && (
                      <div className="mt-1 text-xs text-hack-dim">
                        {truncateText(action.reason, 220)}
                      </div>
                    )}
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {!isUser && guardrails.length > 0 && (
          <div className="mt-3 border border-hack-border bg-black/20 p-2">
            <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
              Guardrails
            </div>
            <div className="mt-1 text-[11px] text-hack-dim">
              {guardrails.slice(0, 3).map((item, index) => (
                <div key={`${message.id}-guardrail-${index}`}>• {truncateText(item, 160)}</div>
              ))}
            </div>
          </div>
        )}

        {(actionIds.length > 0 || actionsCreated > 0 || actionsReused > 0) && (
          <div className="mt-3 border border-hack-warning/50 bg-hack-warning/10 p-2 font-mono text-[11px] text-hack-warning">
            <div className="flex flex-wrap gap-2">
              <span className="border border-hack-warning/50 bg-black/30 px-2 py-1">
                created: {actionsCreated}
              </span>
              <span className="border border-hack-warning/50 bg-black/30 px-2 py-1">
                reused: {actionsReused}
              </span>
              {actionIds.length > 0 && (
                <span className="border border-hack-warning/50 bg-black/30 px-2 py-1">
                  action IDs: {actionIds.join(", ")}
                </span>
              )}
              {reusedActionIds.length > 0 && (
                <span className="border border-hack-warning/50 bg-black/30 px-2 py-1">
                  reused IDs: {reusedActionIds.join(", ")}
                </span>
              )}
            </div>
            <button
              type="button"
              onClick={() => {
                if (onJumpToActions) {
                  onJumpToActions();
                  window.setTimeout(() => {
                    document
                      .getElementById("agent-actions-panel")
                      ?.scrollIntoView({ behavior: "smooth", block: "start" });
                  }, 100);
                  return;
                }
                document
                  .getElementById("agent-actions-panel")
                  ?.scrollIntoView({ behavior: "smooth", block: "start" });
              }}
              className="mt-2 border border-hack-warning/70 px-2 py-1 text-[10px] uppercase tracking-wider hover:bg-hack-warning/10"
            >
              Jump to Agent Actions
            </button>
          </div>
        )}
      </div>

      {isUser && (
        <div className="mt-1 flex h-8 w-8 shrink-0 items-center justify-center border border-hack-border bg-black/40 text-hack-dim">
          <User className="h-4 w-4" />
        </div>
      )}
    </div>
  );
};

const SessionSelector = ({
  sessions,
  selectedSessionId,
  onSelect,
}: {
  sessions: TargetAgentChatSession[];
  selectedSessionId?: number | null;
  onSelect: (id: number) => void;
}) => {
  if (sessions.length === 0) return null;

  return (
    <select
      value={selectedSessionId || ""}
      onChange={(event) => onSelect(Number(event.target.value))}
      className="border border-hack-border bg-black px-3 py-2 font-mono text-xs text-white outline-none"
    >
      {sessions.map((session) => (
        <option key={session.id} value={session.id}>
          #{session.id} · {session.title || "Attack Surface Chat"}
        </option>
      ))}
    </select>
  );
};

const AgentChatPanel = ({ targetId, enabled = true, onJumpToActions }: Props) => {
  const queryClient = useQueryClient();
  const [selectedSessionId, setSelectedSessionId] = useState<number | null>(
    null,
  );
  const [message, setMessage] = useState("");
  const [notice, setNotice] = useState<string | null>(null);

  const sessionsQuery = useQuery({
    queryKey: ["targets", targetId, "agent-chat", "sessions"],
    queryFn: () => getTargetAgentChatSessions(targetId, 20),
    enabled: Boolean(targetId) && enabled,
    staleTime: 10_000,
  });

  const sessions = sessionsQuery.data?.data || [];

  useEffect(() => {
    if (!selectedSessionId && sessions.length > 0) {
      setSelectedSessionId(sessions[0].id);
    }
  }, [selectedSessionId, sessions]);

  const messagesQuery = useQuery({
    queryKey: [
      "targets",
      targetId,
      "agent-chat",
      "sessions",
      selectedSessionId,
      "messages",
    ],
    queryFn: () =>
      getTargetAgentChatMessages(targetId, Number(selectedSessionId)),
    enabled: Boolean(targetId) && Boolean(selectedSessionId) && enabled,
    refetchInterval: 20_000,
  });

  const messages = messagesQuery.data?.data || [];

  const latestAssistantOutput = useMemo(() => {
    const assistant = [...messages]
      .reverse()
      .find((item) => item.role === "assistant");
    return parseJSONValue(assistant?.output_json);
  }, [messages]);

  const refreshAll = () => {
    queryClient.invalidateQueries({
      queryKey: ["targets", targetId, "agent-chat"],
    });
    queryClient.invalidateQueries({
      queryKey: ["targets", targetId, "agent-actions"],
    });
  };

  const createSessionMutation = useMutation({
    mutationFn: () =>
      createTargetAgentChatSession(targetId, "Attack Surface Chat"),
    onSuccess: (session) => {
      setSelectedSessionId(session.id);
      setNotice("New chat session created");
      refreshAll();
    },
  });

  const deleteSessionMutation = useMutation({
    mutationFn: (sessionId: number) =>
      deleteTargetAgentChatSession(targetId, sessionId),
    onSuccess: () => {
      setSelectedSessionId(null);
      setNotice("Chat session deleted");
      refreshAll();
    },
  });

  const sendMessageMutation = useMutation({
    mutationFn: async () => {
      const content = message.trim();
      if (!content) throw new Error("Message is required");

      let sessionId = selectedSessionId;

      if (!sessionId) {
        const session = await createTargetAgentChatSession(
          targetId,
          "Attack Surface Chat",
        );
        sessionId = session.id;
        setSelectedSessionId(session.id);
      }

      return createTargetAgentChatMessage(targetId, Number(sessionId), content);
    },
    onSuccess: (result) => {
      setMessage("");
      const output = parseJSONValue(result.assistant_message?.output_json);
      const created = Number(output?.actions_created || 0);
      const reused = Number(output?.actions_reused || 0);
      const count = result.proposed_actions?.length || 0;
      setNotice(
        count > 0
          ? `Chat returned ${count} action${count === 1 ? "" : "s"}: ${created} created, ${reused} reused`
          : "Chat response created",
      );
      refreshAll();
    },
  });

  const actionCount =
    Number(latestAssistantOutput?.actions_created || 0) +
    Number(latestAssistantOutput?.actions_reused || 0);

  const busy =
    createSessionMutation.isPending ||
    deleteSessionMutation.isPending ||
    sendMessageMutation.isPending ||
    sessionsQuery.isFetching ||
    messagesQuery.isFetching;

  if (!enabled) return null;

  return (
    <div className="mt-6 border border-hack-border bg-black/30 p-5">
      <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="flex items-center gap-2 font-mono text-lg uppercase tracking-wider text-hack-primary">
            <MessageSquare className="h-5 w-5" /> Attack Surface Chat
          </h2>
          <p className="mt-1 max-w-4xl text-sm text-hack-dim">
            AI-driven v3.15 authorized pentest operator. The agent uses target
            memory, policy context, executable skills, and selected methodology
            records to plan evidence-driven validation and controlled action
            workflows under the target's authorization boundaries.
          </p>
        </div>

        <div className="flex flex-wrap gap-2">
          <SessionSelector
            sessions={sessions}
            selectedSessionId={selectedSessionId}
            onSelect={setSelectedSessionId}
          />

          <button
            type="button"
            onClick={() => refreshAll()}
            disabled={busy}
            className="hack-btn-ghost border border-hack-border px-3 py-1 text-[10px] uppercase tracking-wider text-hack-dim hover:text-white disabled:opacity-50"
          >
            <RefreshCw
              className={clsx("h-3 w-3", busy && "animate-spin")}
            />
            Refresh
          </button>

          <button
            type="button"
            onClick={() => createSessionMutation.mutate()}
            disabled={busy}
            className="hack-btn border border-hack-primary px-3 py-1 text-[10px] uppercase tracking-wider text-hack-primary disabled:opacity-50"
          >
            <Plus className="h-3 w-3" /> New Chat
          </button>

          <button
            type="button"
            onClick={() => {
              if (
                selectedSessionId &&
                window.confirm("Delete this chat session and its messages? This is a soft delete.")
              ) {
                deleteSessionMutation.mutate(selectedSessionId);
              }
            }}
            disabled={busy || !selectedSessionId}
            className="hack-btn-ghost border border-hack-danger/60 px-3 py-1 text-[10px] uppercase tracking-wider text-hack-danger disabled:opacity-50"
          >
            <Trash2 className="h-3 w-3" /> Delete Chat
          </button>
        </div>
      </div>

      {notice && (
        <div className="mb-3 border border-hack-primary/50 bg-hack-primary/10 p-3 font-mono text-sm text-hack-primary">
          {notice}
        </div>
      )}

      {(createSessionMutation.error || sendMessageMutation.error || deleteSessionMutation.error) && (
        <div className="mb-3 border border-hack-danger/60 bg-hack-danger/10 p-3 font-mono text-sm text-hack-danger">
          {(createSessionMutation.error as any)?.response?.data?.message ||
            (sendMessageMutation.error as any)?.response?.data?.message ||
            (deleteSessionMutation.error as any)?.response?.data?.message ||
            (createSessionMutation.error as any)?.message ||
            (sendMessageMutation.error as any)?.message ||
            (deleteSessionMutation.error as any)?.message ||
            "Chat operation failed"}
        </div>
      )}

      <div className="mb-4 grid gap-3 md:grid-cols-4">
        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Sessions
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {sessions.length}
          </div>
        </div>

        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Messages
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {messages.length}
          </div>
        </div>

        <div className="border border-hack-border bg-black/20 p-3">
          <div className="font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            Last Actions
          </div>
          <div className="mt-1 font-mono text-xl font-bold text-white">
            {actionCount}
          </div>
        </div>

        <div className="border border-hack-border bg-black/20 p-3">
          <div className="flex items-center gap-2 font-mono text-[10px] uppercase tracking-wider text-hack-dim">
            <ShieldCheck className="h-3 w-3 text-hack-primary" />
            Guardrail
          </div>
          <div className="mt-1 font-mono text-xs text-white">
            approval-gated only
          </div>
        </div>
      </div>

      <div className="mb-4 min-h-[220px] space-y-4 border border-hack-border bg-black/20 p-4">
        {messagesQuery.isLoading ? (
          <div className="flex items-center gap-2 text-sm text-hack-dim">
            <Loader2 className="h-4 w-4 animate-spin" /> Loading messages...
          </div>
        ) : messages.length === 0 ? (
          <div className="text-sm text-hack-dim">
            Start a chat with a request like:
            <div className="mt-2 border border-hack-border bg-black/40 p-3 font-mono text-xs text-hack-primary">
              بر اساس OWASP برای این تارگت تست‌های passive و safe xss و cors و redirect رو پیشنهاد بده
            </div>
          </div>
        ) : (
          messages.map((item) => <MessageBubble
                key={item.id}
                message={item}
                onJumpToActions={onJumpToActions}
              />)
        )}
      </div>

      <form
        className="flex flex-col gap-3 md:flex-row"
        onSubmit={(event) => {
          event.preventDefault();
          sendMessageMutation.mutate();
        }}
      >
        <textarea
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          rows={3}
          disabled={sendMessageMutation.isPending}
          placeholder="Ask the agent to plan OWASP checks, safe XSS/CORS/redirect testing, JS review, report preparation, severity review..."
          className="min-h-[84px] flex-1 resize-y border border-hack-border bg-black/50 p-3 text-sm text-white outline-none placeholder:text-hack-dim focus:border-hack-primary"
        />

        <button
          type="submit"
          disabled={sendMessageMutation.isPending || !message.trim()}
          className="hack-btn border border-hack-primary px-5 py-3 text-xs uppercase tracking-wider text-hack-primary disabled:opacity-50 md:w-40"
        >
          {sendMessageMutation.isPending ? (
            <Loader2 className="h-4 w-4 animate-spin" />
          ) : (
            <Send className="h-4 w-4" />
          )}
          Send
        </button>
      </form>
    </div>
  );
};

export default AgentChatPanel;
