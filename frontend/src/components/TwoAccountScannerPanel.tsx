import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { Play, Loader2, Brain, Shield, AlertTriangle, CheckCircle, XCircle, Clock } from "lucide-react";
import { toast } from "react-hot-toast";

const API_BASE = "/api";

interface TwoAccountSession {
  id: number;
  target_id: number;
  owner_id: number;
  auth_context_id_a: number;
  auth_context_id_b: number;
  context_a_name: string;
  context_b_name: string;
  status: string;
  current_strategy: string;
  requests_sent: number;
  differences_found: number;
  potential_bugs: number;
  last_significant_findings: string;
  started_at: string;
  ended_at?: string;
  created_at: string;
}

const statusColors: Record<string, string> = {
  initializing: "text-yellow-400",
  running: "text-blue-400",
  exploring: "text-cyan-400",
  attacking: "text-orange-400",
  verifying: "text-purple-400",
  completed: "text-green-400",
  failed: "text-red-400",
};

const statusIcons: Record<string, React.ReactNode> = {
  initializing: <Clock className="w-4 h-4" />,
  running: <Loader2 className="w-4 h-4 animate-spin" />,
  exploring: <Shield className="w-4 h-4" />,
  attacking: <AlertTriangle className="w-4 h-4" />,
  verifying: <CheckCircle className="w-4 h-4" />,
  completed: <CheckCircle className="w-4 h-4" />,
  failed: <XCircle className="w-4 h-4" />,
};

// Fetch Sessions
const fetchSessions = async (targetId: number): Promise<TwoAccountSession[]> => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/two-account-sessions`, {
    headers: { Authorization: `Bearer ${localStorage.getItem("token")}` },
  });
  if (!res.ok) throw new Error("Failed to fetch sessions");
  const data = await res.json();
  return data.data || [];
};

// Start Smart Test
const startSmartTest = async (targetId: number): Promise<any> => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/agent-actions/propose`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
    body: JSON.stringify({
      action_type: "START_TWO_ACCOUNT_TEST",
      title: "Smart IDOR/BOLA Test",
      description: "Two-account intelligent IDOR testing with auth context comparison",
      risk_level: "medium",
      safety_level: 2,
      test_level: 2,
      autonomy_level: 1,
      input_json: {},
    }),
  });
  if (!res.ok) {
    const err = await res.json();
    throw new Error(err.message || "Failed to start test");
  }
  return res.json();
};

export default function TwoAccountScannerPanel({ targetId }: { targetId: number }) {
  const queryClient = useQueryClient();
  const [isRunning, setIsRunning] = useState(false);

  const { data: sessions, isLoading } = useQuery({
    queryKey: ["twoAccountSessions", targetId],
    queryFn: () => fetchSessions(targetId),
    enabled: !!targetId,
    refetchInterval: 5000,
  });

  const startMutation = useMutation({
    mutationFn: () => startSmartTest(targetId),
    onMutate: () => setIsRunning(true),
    onSuccess: () => {
      setIsRunning(false);
      toast.success("Smart IDOR Test Started! Agent is analyzing...");
      queryClient.invalidateQueries({ queryKey: ["twoAccountSessions", targetId] });
      queryClient.invalidateQueries({ queryKey: ["agentActions", targetId] });
    },
    onError: (err: any) => {
      setIsRunning(false);
      toast.error(err.message || "Failed to start test");
    },
  });

  const formatDate = (dateStr: string) => {
    if (!dateStr) return "N/A";
    return new Date(dateStr).toLocaleString();
  };

  return (
    <div className="bg-[#0a0a0a] border border-[#333] rounded-lg p-4 mb-6">
      <div className="flex items-center justify-between mb-4">
        <h3 className="text-lg font-semibold text-white flex items-center gap-2">
          <Brain className="w-5 h-5 text-purple-400" />
          Smart Two-Account IDOR Scanner
        </h3>
        <button
          onClick={() => startMutation.mutate()}
          disabled={isRunning || startMutation.isPending}
          className="px-4 py-2 bg-purple-600 hover:bg-purple-700 disabled:bg-gray-700 text-white text-sm rounded flex items-center gap-2 transition-all"
        >
          {isRunning || startMutation.isPending ? <Loader2 className="w-4 h-4 animate-spin" /> : <Play className="w-4 h-4" />}
          {isRunning ? "Analyzing..." : "Start Smart Test"}
        </button>
      </div>

      {isLoading ? (
        <div className="text-gray-500 text-sm flex items-center gap-2">
          <Loader2 className="w-4 h-4 animate-spin" /> Loading sessions...
        </div>
      ) : sessions && sessions.length > 0 ? (
        <div className="space-y-3">
          {sessions.map((session) => (
            <div
              key={session.id}
              className="bg-[#111] border border-[#222] rounded-lg p-4 hover:border-purple-500/30 transition-all"
            >
              <div className="flex items-center justify-between mb-3">
                <div className="flex items-center gap-2">
                  <span className={statusColors[session.status] || "text-gray-400"}>
                    {statusIcons[session.status] || <Clock className="w-4 h-4" />}
                  </span>
                  <span className="text-white font-medium capitalize">{session.status}</span>
                  <span className="text-gray-500 text-xs">#{session.id}</span>
                </div>
                <span className="text-gray-500 text-xs">{formatDate(session.created_at)}</span>
              </div>

              <div className="grid grid-cols-2 gap-4 mb-3">
                <div>
                  <p className="text-gray-500 text-xs mb-1">Account A (Victim)</p>
                  <p className="text-white text-sm">{session.context_a_name || `Context #${session.auth_context_id_a}`}</p>
                </div>
                <div>
                  <p className="text-gray-500 text-xs mb-1">Account B (Attacker)</p>
                  <p className="text-white text-sm">{session.context_b_name || `Context #${session.auth_context_id_b}`}</p>
                </div>
              </div>

              <div className="grid grid-cols-4 gap-4 mb-3">
                <div className="text-center">
                  <p className="text-2xl font-bold text-white">{session.requests_sent}</p>
                  <p className="text-gray-500 text-xs">Requests</p>
                </div>
                <div className="text-center">
                  <p className="text-2xl font-bold text-yellow-400">{session.differences_found}</p>
                  <p className="text-gray-500 text-xs">Differences</p>
                </div>
                <div className="text-center">
                  <p className="text-2xl font-bold text-red-400">{session.potential_bugs}</p>
                  <p className="text-gray-500 text-xs">Potential Bugs</p>
                </div>
                <div className="text-center">
                  <p className="text-2xl font-bold text-purple-400 capitalize">{session.current_strategy}</p>
                  <p className="text-gray-500 text-xs">Strategy</p>
                </div>
              </div>

              {session.last_significant_findings && (
                <div className="bg-[#1a1a1a] border border-[#333] rounded p-3">
                  <p className="text-gray-500 text-xs mb-1">Last Significant Finding</p>
                  <p className="text-white text-sm">{session.last_significant_findings}</p>
                </div>
              )}

              {session.ended_at && (
                <div className="mt-3 text-gray-500 text-xs">
                  Completed: {formatDate(session.ended_at)}
                </div>
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className="text-center py-8">
          <Brain className="w-12 h-12 text-gray-600 mx-auto mb-3" />
          <p className="text-gray-500 text-sm">No test sessions yet</p>
          <p className="text-gray-600 text-xs mt-1">Click "Start Smart Test" to begin IDOR/BOLA analysis</p>
        </div>
      )}
    </div>
  );
}
