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

const MessageBubble = ({ message }: { message: TargetAgentChatMessage }) => {
  const isUser = message.role === "user";
  const output = parseJSONValue(message.output_json);
  const actionIds = Array.isArray(output?.action_ids) ? output.action_ids : [];

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

        {actionIds.length > 0 && (
          <div className="mt-3 border border-hack-warning/50 bg-hack-warning/10 p-2 font-mono text-[11px] text-hack-warning">
            <div>Created proposed action IDs: {actionIds.join(", ")}</div>
            <button
              type="button"
              onClick={() =>
                document
                  .getElementById("agent-actions-panel")
                  ?.scrollIntoView({ behavior: "smooth", block: "start" })
              }
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

const AgentChatPanel = ({ targetId, enabled = true }: Props) => {
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
      const count = result.proposed_actions?.length || 0;
      setNotice(
        count > 0
          ? `Created ${count} proposed action${count === 1 ? "" : "s"} from chat`
          : "Chat response created",
      );
      refreshAll();
    },
  });

  const actionCount = Number(latestAssistantOutput?.actions_created || 0);

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
            Conversational v3.7.0 foundation. The agent converts natural-language
            requests into policy-aware, approval-gated agent action proposals.
            Direct execution remains disabled in this step.
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
          messages.map((item) => <MessageBubble key={item.id} message={item} />)
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
