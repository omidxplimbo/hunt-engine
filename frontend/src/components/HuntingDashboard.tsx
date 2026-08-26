import { useState } from "react";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import {
  Brain,
  Target,
  Bug,
  Shield,
  Zap,
  MessageSquare,
  ChevronRight,
  Loader2,
  AlertTriangle,
  Clock,
  Eye,
  Crosshair,
  CheckCircle,
  XCircle,
} from "lucide-react";
import { toast } from "react-hot-toast";
import HuntLivePanel from "./HuntLivePanel";

const API_BASE = "/api";

interface HuntingStats {
  total_assets: number;
  live_assets: number;
  total_findings: number;
  open_findings: number;
  critical_findings: number;
  high_findings: number;
  medium_findings: number;
  low_findings: number;
}

interface AgentAction {
  id: number;
  action_type: string;
  title: string;
  description: string;
  status: string;
  risk_level: string;
  safety_level: number;
  created_at: string;
}

interface AgentChatSession {
  id: number;
  title: string;
  created_at: string;
}

// Fetch dashboard stats
const fetchStats = async (targetId: number): Promise<HuntingStats> => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/findings/stats`, {
    headers: { Authorization: `Bearer ${localStorage.getItem("token")}` },
  });
  if (!res.ok) throw new Error("Failed to fetch stats");
  const data = await res.json();
  return data.data || {};
};

// Fetch agent actions
const fetchActions = async (targetId: number): Promise<AgentAction[]> => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/agent-actions`, {
    headers: { Authorization: `Bearer ${localStorage.getItem("token")}` },
  });
  if (!res.ok) throw new Error("Failed to fetch actions");
  const data = await res.json();
  return data.data || [];
};

// Fetch chat sessions
const fetchChatSessions = async (targetId: number): Promise<AgentChatSession[]> => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/agent-chat/sessions`, {
    headers: { Authorization: `Bearer ${localStorage.getItem("token")}` },
  });
  if (!res.ok) throw new Error("Failed to fetch sessions");
  const data = await res.json();
  return data.data || [];
};

// Approve action
const approveAction = async (targetId: number, actionId: number) => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/agent-actions/${actionId}/approve`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
    body: JSON.stringify({ reason: "Approved from Hunting Dashboard" }),
  });
  if (!res.ok) throw new Error("Failed to approve");
  return res.json();
};

// Dispatch action
const dispatchAction = async (targetId: number, actionId: number) => {
  const res = await fetch(`${API_BASE}/targets/${targetId}/agent-actions/${actionId}/dispatch`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${localStorage.getItem("token")}`,
    },
    body: JSON.stringify({ dry_run: false }),
  });
  if (!res.ok) throw new Error("Failed to dispatch");
  return res.json();
};

const statusColors: Record<string, string> = {
  proposed: "text-yellow-400",
  approved: "text-blue-400",
  executed: "text-green-400",
  rejected: "text-red-400",
  blocked_by_policy: "text-red-400",
};

const riskColors: Record<string, string> = {
  low: "text-green-400",
  medium: "text-yellow-400",
  high: "text-orange-400",
  critical: "text-red-400",
};

export default function HuntingDashboard({ targetId }: { targetId: number }) {
  const queryClient = useQueryClient();
  const [activeView, setActiveView] = useState<"dashboard" | "hunter" | "actions" | "chat">("dashboard");

  const { data: stats } = useQuery({
    queryKey: ["huntingStats", targetId],
    queryFn: () => fetchStats(targetId),
    enabled: !!targetId,
    refetchInterval: 10000,
  });

  const { data: actions, isLoading: actionsLoading } = useQuery({
    queryKey: ["agentActions", targetId],
    queryFn: () => fetchActions(targetId),
    enabled: !!targetId,
    refetchInterval: 5000,
  });

  const approveMutation = useMutation({
    mutationFn: (actionId: number) => approveAction(targetId, actionId),
    onSuccess: () => {
      toast.success("Action approved");
      queryClient.invalidateQueries({ queryKey: ["agentActions", targetId] });
    },
    onError: () => toast.error("Failed to approve"),
  });

  const dispatchMutation = useMutation({
    mutationFn: (actionId: number) => dispatchAction(targetId, actionId),
    onSuccess: () => {
      toast.success("Action dispatched");
      queryClient.invalidateQueries({ queryKey: ["agentActions", targetId] });
    },
    onError: () => toast.error("Failed to dispatch"),
  });

  const pendingActions = actions?.filter((a) => a.status === "proposed") || [];
  const approvedActions = actions?.filter((a) => a.status === "approved") || [];

  return (
    <div className="space-y-4">
      {/* Navigation Tabs */}
      <div className="flex gap-2 border-b border-hack-border pb-2">
        {[
          { id: "dashboard", label: "Dashboard", icon: Target },
          { id: "hunter", label: "Hunter", icon: Crosshair },
          { id: "actions", label: "Actions", icon: Zap },
          { id: "chat", label: "Chat", icon: MessageSquare },
        ].map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveView(tab.id as any)}
            className={`flex items-center gap-2 px-4 py-2 text-sm font-mono uppercase tracking-wider transition-colors ${
              activeView === tab.id
                ? "border-b-2 border-hack-primary text-hack-primary"
                : "text-hack-dim hover:text-white"
            }`}
          >
            <tab.icon className="h-4 w-4" />
            {tab.label}
          </button>
        ))}
      </div>

      {/* Dashboard View */}
      {activeView === "dashboard" && (
        <div className="space-y-4">
          {/* Stats Cards */}
          <div className="grid grid-cols-4 gap-4">
            <div className="bg-hack-panel border border-hack-border p-4 rounded">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-hack-dim text-xs uppercase">Assets</p>
                  <p className="text-2xl font-bold text-white">{stats?.total_assets || 0}</p>
                </div>
                <Target className="h-8 w-8 text-hack-primary opacity-50" />
              </div>
              <p className="text-hack-dim text-xs mt-2">{stats?.live_assets || 0} live</p>
            </div>

            <div className="bg-hack-panel border border-hack-border p-4 rounded">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-hack-dim text-xs uppercase">Findings</p>
                  <p className="text-2xl font-bold text-white">{stats?.total_findings || 0}</p>
                </div>
                <Bug className="h-8 w-8 text-yellow-400 opacity-50" />
              </div>
              <p className="text-hack-dim text-xs mt-2">{stats?.open_findings || 0} open</p>
            </div>

            <div className="bg-hack-panel border border-hack-border p-4 rounded">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-hack-dim text-xs uppercase">Critical/High</p>
                  <p className="text-2xl font-bold text-red-400">
                    {(stats?.critical_findings || 0) + (stats?.high_findings || 0)}
                  </p>
                </div>
                <AlertTriangle className="h-8 w-8 text-red-400 opacity-50" />
              </div>
              <p className="text-hack-dim text-xs mt-2">
                {stats?.critical_findings || 0} critical, {stats?.high_findings || 0} high
              </p>
            </div>

            <div className="bg-hack-panel border border-hack-border p-4 rounded">
              <div className="flex items-center justify-between">
                <div>
                  <p className="text-hack-dim text-xs uppercase">Pending Actions</p>
                  <p className="text-2xl font-bold text-yellow-400">{pendingActions.length}</p>
                </div>
                <Clock className="h-8 w-8 text-yellow-400 opacity-50" />
              </div>
              <p className="text-hack-dim text-xs mt-2">{approvedActions.length} ready to dispatch</p>
            </div>
          </div>

          {/* Pipeline Status */}
          <div className="bg-hack-panel border border-hack-border p-4 rounded">
            <h3 className="text-hack-primary font-mono text-sm uppercase mb-4">Agent Pipeline</h3>
            <div className="flex items-center justify-between">
              {[
                { name: "Recon", status: "complete", icon: Eye },
                { name: "Analysis", status: "running", icon: Brain },
                { name: "Exploit", status: "waiting", icon: Zap },
                { name: "Report", status: "waiting", icon: Shield },
              ].map((step, i) => (
                <div key={step.name} className="flex items-center">
                  <div className="flex flex-col items-center">
                    <div
                      className={`w-12 h-12 rounded-full flex items-center justify-center ${
                        step.status === "complete"
                          ? "bg-green-500/20 border-2 border-green-500"
                          : step.status === "running"
                          ? "bg-yellow-500/20 border-2 border-yellow-500 animate-pulse"
                          : "bg-hack-border border-2 border-hack-dim"
                      }`}
                    >
                      <step.icon
                        className={`h-6 w-6 ${
                          step.status === "complete"
                            ? "text-green-500"
                            : step.status === "running"
                            ? "text-yellow-500"
                            : "text-hack-dim"
                        }`}
                      />
                    </div>
                    <p className="text-xs mt-2 text-hack-dim">{step.name}</p>
                  </div>
                  {i < 3 && (
                    <ChevronRight className="h-6 w-6 text-hack-dim mx-4" />
                  )}
                </div>
              ))}
            </div>
          </div>

          {/* Top Pending Actions */}
          {pendingActions.length > 0 && (
            <div className="bg-hack-panel border border-hack-border p-4 rounded">
              <h3 className="text-hack-primary font-mono text-sm uppercase mb-4">
                Top Actions ({pendingActions.length} pending)
              </h3>
              <div className="space-y-2">
                {pendingActions.slice(0, 5).map((action) => (
                  <div
                    key={action.id}
                    className="flex items-center justify-between p-3 bg-black/30 border border-hack-border rounded"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <span className={`text-xs ${riskColors[action.risk_level]}`}>
                          [{action.risk_level.toUpperCase()}]
                        </span>
                        <span className="text-white text-sm">{action.title}</span>
                      </div>
                      <p className="text-hack-dim text-xs mt-1">{action.description?.substring(0, 100)}...</p>
                    </div>
                    <div className="flex gap-2">
                      <button
                        onClick={() => approveMutation.mutate(action.id)}
                        className="px-3 py-1 bg-green-500/20 border border-green-500 text-green-500 text-xs rounded hover:bg-green-500/30"
                      >
                        Approve
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Hunter View */}
      {activeView === "hunter" && (
        <HunterView targetId={targetId} />
      )}

      {/* Actions View */}
      {activeView === "actions" && (
        <div className="space-y-4">
          <div className="bg-hack-panel border border-hack-border p-4 rounded">
            <h3 className="text-hack-primary font-mono text-sm uppercase mb-4">All Actions</h3>
            {actionsLoading ? (
              <div className="flex items-center gap-2 text-hack-dim">
                <Loader2 className="h-4 w-4 animate-spin" /> Loading...
              </div>
            ) : (
              <div className="space-y-2">
                {actions?.map((action) => (
                  <div
                    key={action.id}
                    className="flex items-center justify-between p-3 bg-black/30 border border-hack-border rounded"
                  >
                    <div className="flex-1">
                      <div className="flex items-center gap-2">
                        <span className={`text-xs ${statusColors[action.status]}`}>
                          [{action.status.toUpperCase()}]
                        </span>
                        <span className={`text-xs ${riskColors[action.risk_level]}`}>
                          [{action.risk_level.toUpperCase()}]
                        </span>
                        <span className="text-white text-sm">{action.title}</span>
                      </div>
                      <p className="text-hack-dim text-xs mt-1">{action.action_type}</p>
                    </div>
                    <div className="flex gap-2">
                      {action.status === "proposed" && (
                        <button
                          onClick={() => approveMutation.mutate(action.id)}
                          className="px-3 py-1 bg-green-500/20 border border-green-500 text-green-500 text-xs rounded hover:bg-green-500/30"
                        >
                          Approve
                        </button>
                      )}
                      {action.status === "approved" && (
                        <button
                          onClick={() => dispatchMutation.mutate(action.id)}
                          className="px-3 py-1 bg-blue-500/20 border border-blue-500 text-blue-500 text-xs rounded hover:bg-blue-500/30"
                        >
                          Dispatch
                        </button>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}

      {/* Chat View */}
      {activeView === "chat" && (
        <ChatView targetId={targetId} />
      )}
    </div>
  );
}

// Full Chat Component
function ChatView({ targetId }: { targetId: number }) {
  const queryClient = useQueryClient();
  const [selectedSession, setSelectedSession] = useState<number | null>(null);
  const [newMessage, setNewMessage] = useState("");
  const [newSessionTitle, setNewSessionTitle] = useState("");
  const [showNewSession, setShowNewSession] = useState(false);

  const { data: sessions, isLoading: sessionsLoading } = useQuery({
    queryKey: ["chatSessions", targetId],
    queryFn: () => fetchChatSessions(targetId),
    enabled: !!targetId,
    refetchInterval: 10000,
  });

  const { data: messages, isLoading: messagesLoading } = useQuery({
    queryKey: ["chatMessages", targetId, selectedSession],
    queryFn: async () => {
      if (!selectedSession) return [];
      const res = await fetch(`${API_BASE}/targets/${targetId}/agent-chat/sessions/${selectedSession}/messages`, {
        headers: { Authorization: `Bearer ${localStorage.getItem("token")}` },
      });
      if (!res.ok) throw new Error("Failed to fetch messages");
      const data = await res.json();
      return data.data || [];
    },
    enabled: !!targetId && !!selectedSession,
    refetchInterval: 5000,
  });

  const createSessionMutation = useMutation({
    mutationFn: async (title: string) => {
      const res = await fetch(`${API_BASE}/targets/${targetId}/agent-chat/sessions`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
        body: JSON.stringify({ title: title || "New Chat Session" }),
      });
      if (!res.ok) throw new Error("Failed to create session");
      return res.json();
    },
    onSuccess: (data) => {
      toast.success("Session created");
      queryClient.invalidateQueries({ queryKey: ["chatSessions", targetId] });
      setSelectedSession(data.data.id);
      setShowNewSession(false);
      setNewSessionTitle("");
    },
    onError: () => toast.error("Failed to create session"),
  });

  const sendMessageMutation = useMutation({
    mutationFn: async (content: string) => {
      if (!selectedSession) throw new Error("No session selected");
      const res = await fetch(`${API_BASE}/targets/${targetId}/agent-chat/sessions/${selectedSession}/messages`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${localStorage.getItem("token")}`,
        },
        body: JSON.stringify({ content }),
      });
      if (!res.ok) throw new Error("Failed to send message");
      return res.json();
    },
    onSuccess: () => {
      setNewMessage("");
      queryClient.invalidateQueries({ queryKey: ["chatMessages", targetId, selectedSession] });
    },
    onError: () => toast.error("Failed to send message"),
  });

  const handleSend = () => {
    if (newMessage.trim()) {
      sendMessageMutation.mutate(newMessage.trim());
    }
  };

  return (
    <div className="flex gap-4 h-[600px]">
      {/* Sessions List */}
      <div className="w-64 bg-hack-panel border border-hack-border rounded p-4 flex flex-col">
        <div className="flex items-center justify-between mb-4">
          <h3 className="text-hack-primary font-mono text-xs uppercase">Sessions</h3>
          <button
            onClick={() => setShowNewSession(!showNewSession)}
            className="text-hack-primary hover:text-white text-xs"
          >
            + New
          </button>
        </div>

        {showNewSession && (
          <div className="mb-4 space-y-2">
            <input
              type="text"
              value={newSessionTitle}
              onChange={(e) => setNewSessionTitle(e.target.value)}
              placeholder="Session title..."
              className="w-full bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary"
            />
            <button
              onClick={() => createSessionMutation.mutate(newSessionTitle)}
              disabled={createSessionMutation.isPending}
              className="w-full px-3 py-1 bg-hack-primary/20 border border-hack-primary text-hack-primary text-xs rounded hover:bg-hack-primary/30 disabled:opacity-50"
            >
              {createSessionMutation.isPending ? "Creating..." : "Create Session"}
            </button>
          </div>
        )}

        <div className="flex-1 overflow-y-auto space-y-2">
          {sessionsLoading ? (
            <div className="flex items-center gap-2 text-hack-dim text-xs">
              <Loader2 className="h-3 w-3 animate-spin" /> Loading...
            </div>
          ) : sessions && sessions.length > 0 ? (
            sessions.map((session) => (
              <button
                key={session.id}
                onClick={() => setSelectedSession(session.id)}
                className={`w-full text-left p-2 rounded text-sm transition-colors ${
                  selectedSession === session.id
                    ? "bg-hack-primary/20 border border-hack-primary text-white"
                    : "bg-black/30 border border-hack-border text-hack-dim hover:text-white hover:border-hack-primary/50"
                }`}
              >
                <p className="truncate">{session.title}</p>
                <p className="text-[10px] text-hack-dim mt-1">
                  {new Date(session.created_at).toLocaleDateString()}
                </p>
              </button>
            ))
          ) : (
            <p className="text-hack-dim text-xs">No sessions yet</p>
          )}
        </div>
      </div>

      {/* Chat Area */}
      <div className="flex-1 bg-hack-panel border border-hack-border rounded flex flex-col">
        {selectedSession ? (
          <>
            {/* Messages */}
            <div className="flex-1 overflow-y-auto p-4 space-y-4">
              {messagesLoading ? (
                <div className="flex items-center justify-center h-full text-hack-dim">
                  <Loader2 className="h-6 w-6 animate-spin" />
                </div>
              ) : messages && messages.length > 0 ? (
                messages.map((msg: any) => (
                  <div
                    key={msg.id}
                    className={`flex ${msg.role === "user" ? "justify-end" : "justify-start"}`}
                  >
                    <div
                      className={`max-w-[80%] p-3 rounded ${
                        msg.role === "user"
                          ? "bg-hack-primary/20 border border-hack-primary"
                          : "bg-black/30 border border-hack-border"
                      }`}
                    >
                      <p className="text-white text-sm whitespace-pre-wrap">{msg.content}</p>
                      <p className="text-hack-dim text-[10px] mt-2">
                        {new Date(msg.created_at).toLocaleTimeString()}
                      </p>
                    </div>
                  </div>
                ))
              ) : (
                <div className="flex items-center justify-center h-full text-hack-dim text-sm">
                  Send a message to start the conversation
                </div>
              )}
            </div>

            {/* Input */}
            <div className="border-t border-hack-border p-4">
              <div className="flex gap-2">
                <input
                  type="text"
                  value={newMessage}
                  onChange={(e) => setNewMessage(e.target.value)}
                  onKeyDown={(e) => e.key === "Enter" && handleSend()}
                  placeholder="Ask the AI operator..."
                  className="flex-1 bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-hack-primary"
                />
                <button
                  onClick={handleSend}
                  disabled={!newMessage.trim() || sendMessageMutation.isPending}
                  className="px-4 py-2 bg-hack-primary text-black font-mono text-sm rounded hover:bg-hack-primary/80 disabled:opacity-50"
                >
                  {sendMessageMutation.isPending ? (
                    <Loader2 className="h-4 w-4 animate-spin" />
                  ) : (
                    "Send"
                  )}
                </button>
              </div>
            </div>
          </>
        ) : (
          <div className="flex items-center justify-center h-full text-hack-dim text-sm">
            Select a session or create a new one to start chatting
          </div>
        )}
      </div>
    </div>
  );
}

// Hunter View Component - Real AI Hacking
function HunterView({ targetId }: { targetId: number }) {
  const { data: huntResults, isLoading: resultsLoading } = useQuery({
    queryKey: ["huntResults", targetId],
    queryFn: async () => {
      const res = await fetch(`${API_BASE}/targets/${targetId}/hunter/results`, {
        headers: { Authorization: `Bearer ${localStorage.getItem("token")}` },
      });
      if (!res.ok) throw new Error("Failed to fetch results");
      const data = await res.json();
      return data.data || [];
    },
    enabled: !!targetId,
    refetchInterval: 10000,
  });

  return (
    <div className="space-y-4">
      {/* Live Hunt Panel: controls + agent stream + persisted evidence */}
      <HuntLivePanel targetId={targetId} />

      {/* Hunt Results */}
      <div className="bg-hack-panel border border-hack-border p-4 rounded">
        <h3 className="text-hack-primary font-mono text-sm uppercase mb-4">Hunt History</h3>
        {resultsLoading ? (
          <div className="flex items-center gap-2 text-hack-dim">
            <Loader2 className="h-4 w-4 animate-spin" /> Loading...
          </div>
        ) : huntResults && huntResults.length > 0 ? (
          <div className="space-y-3">
            {huntResults.map((result: any) => (
              <div
                key={result.id}
                className="p-3 bg-black/30 border border-hack-border rounded"
              >
                <div className="flex items-center justify-between mb-2">
                  <div className="flex items-center gap-2">
                    {result.content?.includes("0 vulnerabilities") ? (
                      <XCircle className="h-4 w-4 text-hack-dim" />
                    ) : (
                      <CheckCircle className="h-4 w-4 text-green-400" />
                    )}
                    <span className="text-white text-sm">{result.title}</span>
                  </div>
                  <span className="text-hack-dim text-xs">
                    {new Date(result.created_at).toLocaleString()}
                  </span>
                </div>
                <p className="text-hack-dim text-xs">{result.summary}</p>
              </div>
            ))}
          </div>
        ) : (
          <div className="text-center py-8">
            <Crosshair className="w-12 h-12 text-gray-600 mx-auto mb-3" />
            <p className="text-hack-dim text-sm">No hunts yet</p>
            <p className="text-gray-600 text-xs mt-1">
              Enter an objective and click "Start Hunt" to begin
            </p>
          </div>
        )}
      </div>
    </div>
  );
}
