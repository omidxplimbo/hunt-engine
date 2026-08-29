import { useState } from "react";
import { MessageSquare, Send, X, AlertCircle } from "lucide-react";
import type { AgentEvent } from "../api/hunter";

interface OperatorQuestionCardProps {
  event: AgentEvent;
  onAnswer: (actionId: string, content: string) => void;
  onSkip: (actionId: string) => void;
}

// OperatorQuestionCard renders an `operator_question` event as an
// inline card with a textarea + chip-buttons for the suggested
// options (if any). The 60-second countdown is owned by the
// AgentLoop on the server; the client just listens for the
// `operator_skipped` event to remove the card.
export default function OperatorQuestionCard({
  event,
  onAnswer,
  onSkip,
}: OperatorQuestionCardProps) {
  const [draft, setDraft] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const actionId = event.action_id || "";
  const question = event.detail || "";
  const ctx = (event.params && (event.params as any).context) || "";
  const options: string[] = (event.params && (event.params as any).options) || [];

  const submit = (text: string) => {
    if (!actionId || submitting) return;
    setSubmitting(true);
    onAnswer(actionId, text);
    // The card stays visible until operator_accepted arrives; the
    // parent HuntLivePanel removes it from the feed.
  };

  return (
    <div
      className="p-3 bg-cyan-500/10 border border-cyan-500/40 rounded font-mono text-xs"
      data-testid="operator-question-card"
    >
      <div className="flex items-start gap-2 mb-2">
        <MessageSquare className="h-4 w-4 text-cyan-400 shrink-0 mt-0.5" />
        <div className="flex-1 min-w-0">
          <p className="text-cyan-400 font-semibold mb-1">agent is asking you</p>
          <p className="text-white text-sm whitespace-pre-wrap break-words">{question}</p>
          {ctx && (
            <p className="text-hack-dim text-[11px] mt-1 italic">{ctx}</p>
          )}
        </div>
      </div>

      {options.length > 0 && (
        <div className="flex flex-wrap gap-1 mb-2">
          {options.map((opt, i) => (
            <button
              key={i}
              onClick={() => submit(opt)}
              disabled={submitting}
              className="px-2 py-1 bg-black/40 border border-cyan-500/60 text-cyan-300 hover:bg-cyan-500/20 rounded text-[11px] disabled:opacity-50"
              data-testid="operator-question-chip"
            >
              {opt}
            </button>
          ))}
        </div>
      )}

      <div className="flex gap-1">
        <input
          type="text"
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") submit(draft);
          }}
          placeholder="Type your answer (or pick a chip above)..."
          disabled={submitting}
          className="flex-1 bg-black/30 border border-hack-border text-white text-sm p-2 rounded focus:outline-none focus:border-cyan-500 disabled:opacity-50"
          data-testid="operator-question-input"
        />
        <button
          onClick={() => submit(draft)}
          disabled={submitting || !draft.trim()}
          className="px-3 py-1.5 bg-cyan-600 hover:bg-cyan-700 text-white text-sm rounded flex items-center gap-1 disabled:opacity-50"
          data-testid="operator-question-send"
        >
          <Send className="h-3 w-3" /> Send
        </button>
        <button
          onClick={() => onSkip(actionId)}
          disabled={submitting}
          className="px-3 py-1.5 bg-black/40 border border-hack-border text-hack-dim hover:text-white text-sm rounded flex items-center gap-1"
          data-testid="operator-question-skip"
          title="Tell the agent to skip this question"
        >
          <X className="h-3 w-3" /> Skip
        </button>
      </div>

      {!actionId && (
        <div className="mt-2 text-yellow-400 text-[10px] flex items-center gap-1">
          <AlertCircle className="h-3 w-3" /> missing action_id; the server may have
          already timed out
        </div>
      )}
    </div>
  );
}
