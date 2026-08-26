const API_BASE = "/api";

export interface AgentEvent {
  type: "turn" | "tool_call" | "tool_result" | "progress" | "finding" | "done";
  turn?: number;
  detail?: string;
  bug_class?: string;
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
  return res.json();
}

export async function getHuntEvidence(targetId: number): Promise<HuntEvidence[]> {
  const res = await fetch(`${API_BASE}/targets/${targetId}/hunter/evidence`, {
    headers: authHeaders(),
  });
  if (!res.ok) throw new Error("Failed to fetch evidence");
  const data = await res.json();
  return data.data || [];
}

export function huntWebSocket(targetId: number): WebSocket {
  const proto = location.protocol === "https:" ? "wss" : "ws";
  const token = localStorage.getItem("token");
  return new WebSocket(
    `${proto}://${location.host}/api/targets/${targetId}/hunter/ws?token=${token}`
  );
}
