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
  Radio,
} from "lucide-react";
import { toast } from "react-hot-toast";
import { getHuntEvidence, huntWebSocket, startHunt, type AgentEvent } from "../api/hunter";

const eventStyle: Record<AgentEvent["type"], { color: string; icon: typeof Terminal }> = {
  turn: { color: "text-hack-dim", icon: Terminal },
  tool_call: { color: "text-blue-400", icon: Wrench },
  tool_result: { color: "text-hack-dim", icon: Wrench },
  progress: { color: "text-yellow-400", icon: Zap },
  finding: { color: "text-green-400", icon: AlertTriangle },
  done: { color: "text-green-500", icon: CheckCircle },
};

const statusBadge = (status: string) =>
  status === "confirmed"
    ? "bg-green-500/20 border-green-500 text-green-400"
    : status === "candidate"
    ? "bg-yellow-500/20 border-yellow-500 text-yellow-400"
    : "bg-black/30 border-hack-border text-hack-dim";

export default function HuntLivePanel({ targetId }: { targetId: number }) {
  const [objective, setObjective] = useState("Find reflected XSS on this target");
  const [mode, setMode] = useState<"single" | "multi">("single");
  const [events, setEvents] = useState<AgentEvent[]>([]);
  const [live, setLive] = useState(false);
  const wsRef = useRef<WebSocket | null>(null);
  const feedRef = useRef<HTMLDivElement>(null);

  const { data: evidence } = useQuery({
    queryKey: ["huntEvidence", targetId],
    queryFn: () => getHuntEvidence(targetId),
    enabled: !!targetId,
    refetchInterval: live ? 4000 : 30000,
  });

  useEffect(() => {
    return () => wsRef.current?.close();
  }, []);

  useEffect(() => {
    feedRef.current?.scrollTo({ top: feedRef.current.scrollHeight });
  }, [events]);

  const beginHunt = () => {
    if (!objective.trim()) return;
    setEvents([]);
    setLive(true);

    const ws = huntWebSocket(targetId);
    wsRef.current = ws;
    ws.onmessage = (m) => {
      try {
        const ev = JSON.parse(m.data as string) as AgentEvent;
        setEvents((prev) => [...prev.slice(-199), ev]);
        if (ev.type === "finding") toast.success("Finding captured!");
        if (ev.type === "done") {
          setLive(false);
          toast.success("Hunt finished");
          ws.close();
        }
      } catch {
        /* ignore malformed frame */
      }
    };
    ws.onerror = () => setLive(false);

    startHunt(targetId, objective.trim(), mode)
      .catch(() => {
        setLive(false);
        toast.error("Failed to start hunt");
      });
  };

  const stop = () => {
    wsRef.current?.close();
    setLive(false);
  };

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="bg-hack-panel border border-hack-border p-4 rounded">
        <h3 className="text-hack-primary font-mono text-sm uppercase mb-3 flex items-center gap-2">
          <Crosshair className="h-4 w-4" /> AI Hunter
          {live && (
            <span className="ml-2 flex items-center gap-1 text-red-400 text-xs normal-case animate-pulse">
              <Radio className="h-3 w-3" /> live
            </span>
          )}
        </h3>
        <div className="flex gap-2">
          <input
            type="text"
            value={objective}
            onChange={(e) => setObjective(e.target.value)}
            placeholder="Objective for the agent..."
            className="flex-1 bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary"
          />
          <select
            value={mode}
            onChange={(e) => setMode(e.target.value as "single" | "multi")}
            className="bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary"
          >
            <option value="single">single</option>
            <option value="multi">multi</option>
          </select>
          {live ? (
            <button
              onClick={stop}
              className="px-4 py-2 bg-gray-700 hover:bg-gray-600 text-white text-sm rounded"
            >
              Detach
            </button>
          ) : (
            <button
              onClick={beginHunt}
              className="px-4 py-2 bg-red-600 hover:bg-red-700 text-white text-sm rounded flex items-center gap-2"
            >
              <Crosshair className="h-4 w-4" /> Start Hunt
            </button>
          )}
        </div>
      </div>

      {/* Live Feed */}
      {events.length > 0 && (
        <div className="bg-hack-panel border border-hack-border p-4 rounded">
          <h3 className="text-hack-primary font-mono text-sm uppercase mb-3">Agent Stream</h3>
          <div ref={feedRef} className="max-h-80 overflow-y-auto space-y-1 font-mono text-xs">
            {events.map((ev, i) => {
              const st = eventStyle[ev.type] || eventStyle.turn;
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

      {/* Evidence Viewer */}
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
          <span className={`px-2 py-0.5 border rounded text-[10px] uppercase ${statusBadge(ev.status)}`}>
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
