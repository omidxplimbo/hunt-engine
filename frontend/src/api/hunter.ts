const API_BASE = "/api";

// AgentEvent mirrors the backend hunter.AgentEvent struct. T7 widens
// the type set so the UI can render the full steering + approval
// surface, not just the original six event types.
export type AgentEventType =
  | "turn"
  | "tool_call"
  | "tool_result"
  | "progress"
  | "finding"
  | "done"
  | "error"
  | "paused"
  | "resumed"
  | "cancelled"
  | "session_done"
  | "operator_message"
  | "operator_question"
  | "operator_accepted"
  | "operator_skipped"
  | "objective_changed"
  | "approval_required"
  | "approval_resolved"
  | "steer_broadcast"
  | "pong";

export interface AgentEvent {
  type: AgentEventType;
  turn?: number;
  tool_name?: string;
  detail?: string;
  bug_class?: string;
  action_id?: string;
  params?: Record<string, unknown>;
  timestamp: string;
}

export interface HuntEvidence {
  id: number;
  created_at: string;
  agent_id: string;
  test_type: string;
  target: string;
  parameter: string;
  payload: string;
  status: string;
  confidence: number;
  severity: string;
  poc: string;
  notes: string;
}

// SessionSnapshot mirrors the backend hunter.SessionSnapshot struct.
export type SessionStatus =
  | "running"
  | "paused"
  | "cancelled"
  | "completed"
  | "failed";

export interface SessionSnapshot {
  id: string;
  target_id: number;
  user_id: number;
  mode: string;
  objective: string;
  status: SessionStatus;
  started_at: string;
  finished_at?: string;
  paused: boolean;
  summary?: string;
  vulns_found?: number;
  last_error?: string;
}

function authHeaders(): Record<string, string> {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${localStorage.getItem("token")}`,
  };
}

export async function startHunt(targetId: number, objective: string, mode: "single" | "multi") {
  const res = await fetch(`${API_BASE}/targets/${targetId}/hunter/start`, {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ objective, mode }),
  });
  if (!res.ok) throw new Error("Failed to start hunt");
  return res.json() as Promise<{ data: { session_id: string; status: SessionStatus; ws_path: string } }>;
}

export async function getHuntEvidence(targetId: number): Promise<HuntEvidence[]> {
  const res = await fetch(`${API_BASE}/targets/${targetId}/hunter/evidence`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("Failed to fetch evidence");
  const data = await res.json();
  return data.data || [];
}

export async function listHuntSessions(targetId: number): Promise<SessionSnapshot[]> {
  const res = await fetch(`${API_BASE}/targets/${targetId}/hunter/sessions`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("Failed to list sessions");
  const data = await res.json();
  return data.data || [];
}

async function sessionAction(
  method: "POST" | "DELETE",
  targetId: number,
  sessionId: string,
  path: string,
): Promise<void> {
  const res = await fetch(
    `${API_BASE}/targets/${targetId}/hunter/sessions/${sessionId}${path}`,
    { method, headers: authHeaders() },
  );
  if (!res.ok) throw new Error(`Failed session action ${method} ${path}`);
}

export const pauseHuntSession = (targetId: number, sessionId: string) =>
  sessionAction("POST", targetId, sessionId, "/pause");

export const resumeHuntSession = (targetId: number, sessionId: string) =>
  sessionAction("POST", targetId, sessionId, "/resume");

export const cancelHuntSession = (targetId: number, sessionId: string) =>
  sessionAction("DELETE", targetId, sessionId, "");

export function huntWebSocket(targetId: number): WebSocket {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const token = localStorage.getItem("token");
  return new WebSocket(
    `${proto}://${location.host}/api/targets/${targetId}/hunter/ws?token=${token}`,
  );
}
