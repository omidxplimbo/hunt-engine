import { useEffect, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Crosshair,
  Terminal,
  Wrench,
  Zap,
  AlertTriangle,
  CheckCircle,
  ChevronDown,
  ChevronRight,
  Send,
  Pause,
  Play,
  X,
  MessageSquare,
  Shield,
} from "lucide-react";
import { toast } from "react-hot-toast";
import {
  cancelHuntSession,
  getHuntEvidence,
  huntWebSocket,
  listHuntSessions,
  startHunt,
  type AgentEvent,
  type AgentEventType,
  type SessionStatus,
} from "../api/hunter";
import ApprovalPopover from "./ApprovalPopover";
import TurnCounter from "./TurnCounter";
import ToolCallCard from "./ToolCallCard";
import OperatorQuestionCard from "./OperatorQuestionCard";

const STATUS_STYLES: Record<
  AgentEventType,
  { color: string; icon: typeof Terminal }
> = {
  turn: { color: "text-hack-dim", icon: Terminal },
  tool_call: { color: "text-blue-400", icon: Wrench },
  tool_result: { color: "text-hack-dim", icon: Wrench },
  progress: { color: "text-yellow-400", icon: Zap },
  finding: { color: "text-green-400", icon: AlertTriangle },
  done: { color: "text-green-500", icon: CheckCircle },
  error: { color: "text-red-400", icon: AlertTriangle },
  paused: { color: "text-yellow-400", icon: Pause },
  resumed: { color: "text-green-400", icon: Play },
  cancelled: { color: "text-hack-dim", icon: X },
  session_done: { color: "text-hack-dim", icon: CheckCircle },
  operator_message: { color: "text-cyan-400", icon: MessageSquare },
  operator_question: { color: "text-cyan-400", icon: MessageSquare },
  operator_accepted: { color: "text-hack-dim", icon: CheckCircle },
  operator_skipped: { color: "text-hack-dim", icon: X },
  objective_changed: { color: "text-purple-400", icon: Zap },
  approval_required: { color: "text-yellow-400", icon: Shield },
  approval_resolved: { color: "text-hack-dim", icon: CheckCircle },
  steer_broadcast: { color: "text-hack-dim", icon: Terminal },
  pong: { color: "text-hack-dim", icon: Terminal },
};

// (No placeholder Shield — the real lucide-react import is used above.)

const EVIDENCE_STATUS_BADGE = (status: string) =>
  status === "confirmed"
    ? "bg-green-500/20 border-green-500 text-green-400"
    : status === "candidate"
    ? "bg-yellow-500/20 border-yellow-500 text-yellow-400"
    : "bg-black/30 border-hack-border text-hack-dim";

const SESSION_STATUS_BADGE: Record<SessionStatus, string> = {
  running: "bg-red-500/20 border-red-500 text-red-400",
  paused: "bg-yellow-500/20 border-yellow-500 text-yellow-400",
  cancelled: "bg-black/30 border-hack-border text-hack-dim",
  completed: "bg-green-500/20 border-green-500 text-green-400",
  failed: "bg-red-500/20 border-red-500 text-red-400",
};

export default function HuntLivePanel({ targetId }: { targetId: number }) {
  const [objective, setObjective] = useState("Find reflected XSS on this target");
  const [committedObjective, setCommittedObjective] = useState(objective);
  const [mode, setMode] = useState<"single" | "multi">("single");
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [sessionId, setSessionId] = useState<string | null>(null);
  const [sessionStatus, setSessionStatus] = useState<SessionStatus>("completed");
  const [chatDraft, setChatDraft] = useState("");
  const [approval, setApproval] = useState<{
    actionId: string;
    tool: string;
    params: Record<string, unknown>;
  } | null>(null);
  // T14: track the current turn number (from the latest 'turn' event)
  // and the set of pending operator questions. The turn counter drives
  // the live progress bar; the pending questions drive the inline
  // OperatorQuestionCard in the stream.
  const [currentTurn, setCurrentTurn] = useState(0);
  const [pendingQuestions, setPendingQuestions] = useState<Record<string, AgentEvent>>({});
  const wsRef = useRef<WebSocket | null>(null);
  const feedRef = useRef<HTMLDivElement>(null);
  const chatRef = useRef<HTMLInputElement>(null);

  const { data: evidence } = useQuery({
    queryKey: ["huntEvidence", targetId],
    queryFn: () => getHuntEvidence(targetId),
    enabled: !!targetId,
    refetchInterval: sessionStatus === "running" || sessionStatus === "paused" ? 4000 : 30000,
  });

  const { data: sessions, refetch: refetchSessions } = useQuery({
    queryKey: ["huntSessions", targetId],
    queryFn: () => listHuntSessions(targetId),
    enabled: !!targetId,
    refetchInterval: 5000,
  });

  useEffect(() => {
    return () => wsRef.current?.close();
  }, []);

  useEffect(() => {
    feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight });
  }, [events]);

  // Detect "approval_required" events and surface them in the popover;
  // track "turn" for the live progress bar; track "operator_question"
  // for the inline question card; clear pending questions when the
  // server acknowledges the answer.
  useEffect(() => {
    const last = events[events.length - 1];
    if (!last) return;
    if (
      last.type === "approval_required" &&
      last.action_id &&
      (!approval || approval.actionId !== last.action_id)
    ) {
      setApproval({
        actionId: last.action_id,
        tool: last.tool_name || "tool",
        params: last.params || {},
      });
    }
    if (last.type === "turn" && typeof last.turn === "number") {
      setCurrentTurn(last.turn);
    }
    if (last.type === "operator_question" && last.action_id) {
      setPendingQuestions((prev) => ({ ...prev, [last.action_id!]: last }));
    }
    if (last.type === "operator_accepted" && last.action_id) {
      setPendingQuestions((prev) => {
        const next = { ...prev };
        delete next[last.action_id!];
        return next;
      });
    }
    if (last.type === "operator_skipped" && last.action_id) {
      setPendingQuestions((prev) => {
        const next = { ...prev };
        delete next[last.action_id!];
        return next;
      });
    }
  }, [events, approval]);

  const sendWs = (msg: Record<string, unknown>) => {
    const ws = wsRef.current;
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      toast.error("Not connected to the hunt");
      return false;
    }
    ws.send(JSON.stringify(msg));
    return true;
  };

  const beginHunt = async () => {
    if (!objective.trim()) return;
    setEvents([]);
    setSessionStatus("running");
    setCommittedObjective(objective.trim());

    const ws = huntWebSocket(targetId);
    wsRef.current = ws;
    ws.onmessage = (m) => {
      try {
        const ev = JSON.parse(m.data as string) as AgentEvent;
        setEvents((prev) => [...prev.slice(-199), ev]);
        if (ev.type === "finding") toast.success("Finding captured!");
        if (ev.type === "paused") setSessionStatus("paused");
        if (ev.type === "resumed") setSessionStatus("running");
        if (ev.type === "cancelled") setSessionStatus("cancelled");
        if (ev.type === "session_done") {
          setSessionStatus((ev.detail as SessionStatus) || "completed");
          refetchSessions();
        }
        if (ev.type === "done") {
          setSessionStatus("completed");
          toast.success("Hunt finished");
          refetchSessions();
        }
      } catch {
        /* ignore malformed frame */
      }
    };
    ws.onerror = () => {
      setSessionStatus("cancelled");
    };
    ws.onclose = () => {
      // don't change status here — the server sends a final session_done
      // before closing; onclose is a fallback.
    };

    try {
      const res = await startHunt(targetId, objective.trim(), mode);
      setSessionId(res.data.session_id);
    } catch {
      setSessionStatus("cancelled");
      toast.error("Failed to start hunt");
    }
  };

  const sendMessage = () => {
    const content = chatDraft.trim();
    if (!content) return;
    if (sendWs({ type: "message", content })) {
      setChatDraft("");
    }
  };

  const setObjectiveLive = () => {
    const content = objective.trim();
    if (!content || content === committedObjective) return;
    if (sendWs({ type: "set_objective", content })) {
      setCommittedObjective(content);
      toast.success("Objective updated");
    }
  };

  const pause = () => {
    if (sendWs({ type: "pause" })) {
      setSessionStatus("paused");
    }
  };

  const resume = () => {
    if (sendWs({ type: "resume" })) {
      setSessionStatus("running");
    }
  };

  const cancel = async () => {
    if (!sessionId) return;
    if (!confirm("Cancel this hunt? Any in-progress tool call will be aborted.")) return;
    try {
      await cancelHuntSession(targetId, sessionId);
      setSessionStatus("cancelled");
      toast.success("Hunt cancelled");
    } catch {
      toast.error("Failed to cancel");
    }
  };

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="bg-hack-panel border border-hack-border p-4 rounded">
        <h3 className="text-hack-primary font-mono text-sm uppercase mb-3 flex items-center gap-2">
          <Crosshair className="h-4 w-4" /> AI Hunter
          <span
            className={`ml-2 px-2 py-0.5 border rounded text-[10px] uppercase font-mono ${
              SESSION_STATUS_BADGE[sessionStatus] ||
              "bg-black/30 border-hack-border text-hack-dim"
            } ${sessionStatus === "running" ? "animate-pulse" : ""}`}
            data-testid="hunt-status"
          >
            {sessionStatus}
          </span>
        </h3>
        <div className="flex gap-2">
          <input
            type="text"
            value={objective}
            onChange={(e) => setObjective(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter" && sessionStatus !== "running" && sessionStatus !== "paused") {
                beginHunt();
              } else if (e.key === "Enter") {
                setObjectiveLive();
              }
            }}
            placeholder="Objective for the agent..."
            className="flex-1 bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary"
          />
          <select
            value={mode}
            onChange={(e) => setMode(e.target.value as "single" | "multi")}
            disabled={sessionStatus === "running" || sessionStatus === "paused"}
            className="bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary disabled:opacity-50"
          >
            <option value="single">single</option>
            <option value="multi">multi</option>
          </select>
          {sessionStatus === "running" && (
            <button
              onClick={pause}
              className="px-4 py-2 bg-yellow-600 hover:bg-yellow-700 text-white text-sm rounded flex items-center gap-2"
              data-testid="hunt-pause"
            >
              <Pause className="h-4 w-4" /> Pause
            </button>
          )}
          {sessionStatus === "paused" && (
            <button
              onClick={resume}
              className="px-4 py-2 bg-green-600 hover:bg-green-700 text-white text-sm rounded flex items-center gap-2"
              data-testid="hunt-resume"
            >
              <Play className="h-4 w-4" /> Resume
            </button>
          )}
          {(sessionStatus === "running" || sessionStatus === "paused") && sessionId && (
            <button
              onClick={cancel}
              className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded flex items-center gap-2"
              data-testid="hunt-cancel"
            >
              <X className="h-4 w-4" /> Cancel
            </button>
          )}
          {sessionStatus !== "running" && sessionStatus !== "paused" && (
            <button
              onClick={beginHunt}
              className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white text-sm rounded flex items-center gap-2"
              data-testid="hunt-start"
            >
              <Crosshair className="h-4 w-4" /> Start Hunt
            </button>
          )}
        </div>
        {(sessionStatus === "running" || sessionStatus === "paused") && (
          <p className="text-hack-dim text-[10px] mt-2 font-mono">
            session_id: {sessionId} · objective: {committedObjective}
            {objective !== committedObjective && (
              <button
                onClick={setObjectiveLive}
                className="ml-2 underline text-hack-primary"
              >
                (send update)
              </button>
            )}
          </p>
        )}
      </div>

      {/* Chat input — visible while connected */}
      {(sessionStatus === "running" || sessionStatus === "paused") && (
        <div className="bg-hack-panel border border-hack-border p-3 rounded">
          <div className="flex gap-2">
            <input
              ref={chatRef}
              type="text"
              value={chatDraft}
              onChange={(e) => setChatDraft(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") sendMessage();
              }}
              placeholder="Send a hint to the agent (e.g. 'focus on /search?q=')"
              className="flex-1 bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary"
              data-testid="hunt-chat"
            />
            <button
              onClick={sendMessage}
              disabled={!chatDraft.trim()}
              className="px-4 py-2 bg-cyan-600 hover:bg-cyan-700 text-white text-sm rounded flex items-center gap-2 disabled:opacity-50"
              data-testid="hunt-chat-send"
            >
              <Send className="h-4 w-4" /> Send
            </button>
          </div>
        </div>
      )}

      {/* Live Feed */}
      {events.length > 0 && (
        <div className="bg-hack-panel border border-hack-border p-4 rounded">
          <div className="flex items-center justify-between mb-3">
            <h3 className="text-hack-primary font-mono text-sm uppercase">Agent Stream</h3>
            <TurnCounter current={currentTurn} max={20} status={sessionStatus === "running" ? "running" : sessionStatus} />
          </div>
          <div ref={feedRef} className="max-h-96 overflow-y-auto space-y-1 font-mono text-xs">
            {events.map((ev, i) => {
              const st = STATUS_STYLES[ev.type] || STATUS_STYLES.turn;
              if (ev.type === "finding") {
                return (
                  <div key={i} className="p-2 bg-green-500/10 border border-green-500/40 rounded flex gap-2">
                    <AlertTriangle className="h-3.5 w-3.5 text-green-400 shrink-0 mt-0.5" />
                    <div>
                      <p className="text-green-400">{ev.detail}</p>
                      {ev.bug_class && (
                        <p className="text-hack-dim text-[10px] mt-0.5">class: {ev.bug_class}</p>
                      )}
                    </div>
                  </div>
                );
              }
              if (ev.type === "progress") {
                return (
                  <div key={i} className="p-2 bg-black/30 border border-hack-border rounded">
                    <p className="text-yellow-400/90 whitespace-pre-wrap line-clamp-3">{ev.detail}</p>
                  </div>
                );
              }
              if (ev.type === "operator_message") {
                return (
                  <div key={i} className="p-2 bg-cyan-500/10 border border-cyan-500/40 rounded">
                    <p className="text-cyan-400/90 whitespace-pre-wrap">
                      <MessageSquare className="h-3 w-3 inline-block mr-1" />
                      you: {ev.detail}
                    </p>
                  </div>
                );
              }
              if (ev.type === "operator_question" && ev.action_id) {
                // Render the pending question as a card only if it's
                // still in pendingQuestions (operator_accepted/skipped
                // would have removed it).
                if (pendingQuestions[ev.action_id]) {
                  return (
                    <OperatorQuestionCard
                      key={i}
                      event={ev}
                      onAnswer={(actionId, content) => {
                        sendWs({ type: "operator_answer", action_id: actionId, content });
                      }}
                      onSkip={(actionId) => {
                        sendWs({ type: "operator_answer", action_id: actionId, content: "[skip]" });
                      }}
                    />
                  );
                }
                // Pending question was answered/skipped; fall through
                // and render a small acknowledgment line.
                return (
                  <div key={i} className="p-1 text-hack-dim text-[10px]">
                    question: {ev.detail} — answered
                  </div>
                );
              }
              if (ev.type === "tool_call") {
                return <ToolCallCard key={i} event={ev} index={i} />;
              }
              if (ev.type === "approval_required") {
                return (
                  <div key={i} className="p-2 bg-yellow-500/10 border border-yellow-500/40 rounded flex gap-2">
                    <Shield className="h-3.5 w-3.5 text-yellow-400 shrink-0 mt-0.5" />
                    <div>
                      <p className="text-yellow-400">approval requested for {ev.tool_name}</p>
                      {ev.action_id && (
                        <p className="text-hack-dim text-[10px] mt-0.5">action: {ev.action_id.slice(0, 8)}…</p>
                      )}
                    </div>
                  </div>
                );
              }
              const Icon = st.icon;
              return (
                <div key={i} className={`flex items-center gap-2 ${st.color}`}>
                  <Icon className="h-3 w-3 shrink-0" />
                  <span>{ev.detail}</span>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Persisted Evidence */}
      <div className="bg-hack-panel border border-hack-border p-4 rounded">
        <h3 className="text-hack-primary font-mono text-sm uppercase mb-3">
          Persisted Evidence ({evidence?.length || 0})
        </h3>
        {!evidence || evidence.length === 0 ? (
          <p className="text-hack-dim text-sm py-4 text-center">
            No persisted evidence yet — run a hunt.
          </p>
        ) : (
          <div className="space-y-2">
            {evidence.map((ev) => (
              <EvidenceRow key={ev.id} ev={ev} />
            ))}
          </div>
        )}
      </div>

      {/* Session history (in-process registry) */}
      {sessions && sessions.length > 0 && (
        <div className="bg-hack-panel border border-hack-border p-4 rounded">
          <h3 className="text-hack-primary font-mono text-sm uppercase mb-3">
            Session History ({sessions.length})
          </h3>
          <div className="space-y-1 text-xs font-mono">
            {sessions.map((s) => (
              <div
                key={s.id}
                className={`flex items-center justify-between p-2 border rounded ${
                  SESSION_STATUS_BADGE[s.status] || ""
                }`}
              >
                <span>{s.id.slice(0, 8)}…</span>
                <span>{s.mode}</span>
                <span>{s.status}</span>
                <span>{s.vulns_found || 0} vulns</span>
              </div>
            ))}
          </div>
        </div>
      )}

      <ApprovalPopover
        open={!!approval}
        tool={approval?.tool || ""}
        actionId={approval?.actionId || ""}
        params={approval?.params || {}}
        onApprove={() => {
          if (approval) {
            sendWs({ type: "approve", action_id: approval.actionId });
            setApproval(null);
          }
        }}
        onDeny={(reason) => {
          if (approval) {
            sendWs({ type: "deny", action_id: approval.actionId, reason });
            setApproval(null);
          }
        }}
        onClose={() => setApproval(null)}
      />
    </div>
  );
}

function EvidenceRow({ ev }: { ev: import("../api/hunter").HuntEvidence }) {
  const [open, setOpen] = useState(false);
  return (
    <div className="border border-hack-border rounded bg-black/30">
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between p-3 text-left"
      >
        <div className="flex items-center gap-2 min-w-0">
          {open ? (
            <ChevronDown className="h-4 w-4 text-hack-dim" />
          ) : (
            <ChevronRight className="h-4 w-4 text-hack-dim" />
          )}
          <span className={`px-2 py-0.5 border rounded text-[10px] uppercase ${EVIDENCE_STATUS_BADGE(ev.status)}`}>
            {ev.status}
          </span>
          <span className="font-mono text-xs text-white">{ev.test_type}</span>
          <span className="text-hack-dim text-xs truncate max-w-[280px]">{ev.payload}</span>
        </div>
        <div className="flex items-center gap-3 shrink-0 ml-2">
          <span className="text-hack-dim text-[10px]">{ev.agent_id}</span>
          <span className="text-yellow-400 text-[10px]">conf {(ev.confidence * 100).toFixed(0)}%</span>
          <span className="text-red-400 text-[10px] uppercase">{ev.severity}</span>
        </div>
      </button>
      {open && (
        <div className="border-t border-hack-border p-3 space-y-2 font-mono text-[11px]">
          <div>
            <p className="text-hack-dim uppercase text-[9px] mb-1">Target URL</p>
            <p className="text-white break-all">{ev.target}</p>
          </div>
          {ev.poc && (
            <div>
              <p className="text-hack-dim uppercase text-[9px] mb-1">PoC</p>
              <p className="text-green-400 break-all">{ev.poc}</p>
            </div>
          )}
          {ev.notes && (
            <div>
              <p className="text-hack-dim uppercase text-[9px] mb-1">Notes</p>
              <pre className="whitespace-pre-wrap text-hack-dim max-h-48 overflow-y-auto">{ev.notes}</pre>
            </div>
          )}
          <p className="text-hack-dim text-[9px]">
            saved {new Date(ev.created_at).toLocaleString()}
          </p>
        </div>
      )}
    </div>
  );
}
