import { useState } from "react";
import { ChevronDown, ChevronRight, Copy, EyeOff } from "lucide-react";
import type { AgentEvent } from "../api/hunter";

interface ToolCallCardProps {
  event: AgentEvent;
  index: number;
}

// ToolCallCard renders a single tool_call event as a collapsible
// card. The default view shows the tool name, a one-line summary,
// and the action_id (truncated). Expanding reveals the masked
// params (read-only JSON tree) and a "Copy JSON" button.
export default function ToolCallCard({ event, index }: ToolCallCardProps) {
  const [open, setOpen] = useState(false);
  const [copied, setCopied] = useState(false);
  const actionIdShort = (event.action_id || "").slice(0, 8);
  const params = event.params || {};

  const copyJson = () => {
    const text = JSON.stringify(params, null, 2);
    if (navigator.clipboard) {
      navigator.clipboard.writeText(text).then(() => {
        setCopied(true);
        setTimeout(() => setCopied(false), 1500);
      });
    }
  };

  return (
    <div
      className="p-2 bg-blue-500/10 border border-blue-500/40 rounded font-mono text-xs"
      data-testid="tool-call-card"
    >
      <button
        onClick={() => setOpen(!open)}
        className="w-full flex items-center justify-between gap-2 text-left"
      >
        <div className="flex items-center gap-2 min-w-0">
          {open ? (
            <ChevronDown className="h-3 w-3 text-blue-400 shrink-0" />
          ) : (
            <ChevronRight className="h-3 w-3 text-blue-400 shrink-0" />
          )}
          <span className="text-blue-400 font-semibold">tool_call</span>
          <span className="text-white">{event.tool_name || "tool"}</span>
          {actionIdShort && (
            <span className="text-hack-dim text-[10px]">id:{actionIdShort}…</span>
          )}
        </div>
        <div className="flex items-center gap-1 text-hack-dim text-[10px]">
          <span>#{index + 1}</span>
        </div>
      </button>
      {open && (
        <div className="mt-2 space-y-2">
          <div className="bg-black/40 border border-hack-border rounded p-2">
            <div className="flex items-center justify-between mb-1">
              <span className="text-hack-dim text-[10px] uppercase">params (masked)</span>
              <button
                onClick={copyJson}
                className="text-hack-dim hover:text-hack-primary flex items-center gap-1 text-[10px]"
                data-testid="tool-call-copy"
              >
                {copied ? <EyeOff className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
                {copied ? "copied" : "copy"}
              </button>
            </div>
            <pre className="text-[11px] text-white whitespace-pre-wrap break-all max-h-48 overflow-y-auto">
              {JSON.stringify(params, null, 2)}
            </pre>
          </div>
        </div>
      )}
    </div>
  );
}
