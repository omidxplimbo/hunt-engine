import { useEffect, useRef, useState } from "react";
import { Shield, ShieldOff, Clock } from "lucide-react";

// ApprovalPopover: modal overlay shown when the backend emits an
// approval_required event. The operator has 60s to approve or deny;
// timeout auto-deny is fired by the parent (because we need access to
// the WebSocket send function).
//
// Props:
//   open        — when true the modal is visible
//   tool        — tool name (e.g. "shell")
//   actionId    — uuid from the server, must be echoed back on approve/deny
//   params      — masked params from the server (Record<string, unknown>)
//   onApprove   — called when the operator clicks Approve
//   onDeny      — called when the operator clicks Deny (or timeout)
//   onClose     — called when the operator dismisses the modal manually
export default function ApprovalPopover({
  open,
  tool,
  actionId,
  params,
  onApprove,
  onDeny,
  onClose,
}: {
  open: boolean;
  tool: string;
  actionId: string;
  params: Record<string, unknown>;
  onApprove: () => void;
  onDeny: (reason: string) => void;
  onClose: () => void;
}) {
  const [secondsLeft, setSecondsLeft] = useState(60);
  const [reason, setReason] = useState("");
  const firedRef = useRef(false);

  // Reset the countdown + reason whenever a new approval opens.
  useEffect(() => {
    if (open) {
      setSecondsLeft(60);
      setReason("");
      firedRef.current = false;
    }
  }, [open, actionId]);

  // 1Hz countdown. On hit-zero, fire a deny with "timeout" reason.
  useEffect(() => {
    if (!open) return;
    if (secondsLeft <= 0) {
      if (!firedRef.current) {
        firedRef.current = true;
        onDeny("timeout");
      }
      return;
    }
    const t = setTimeout(() => setSecondsLeft((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [open, secondsLeft, onDeny]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 bg-black/70 flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-labelledby="approval-title"
    >
      <div className="bg-hack-panel border-2 border-yellow-500/60 rounded-lg shadow-2xl max-w-2xl w-full p-5">
        <div className="flex items-center gap-2 mb-3">
          <Shield className="h-5 w-5 text-yellow-400" />
          <h2 id="approval-title" className="text-yellow-400 font-mono text-base uppercase">
            Operator Approval Required
          </h2>
          <div className="ml-auto flex items-center gap-1 text-xs font-mono text-yellow-300">
            <Clock className="h-3 w-3" />
            <span>{secondsLeft}s</span>
          </div>
        </div>

        <p className="text-hack-dim text-sm mb-3">
          The hunter agent wants to execute a <span className="text-white font-mono">{tool}</span> tool.
          Sensitive values are masked. Review the parameters and approve or deny.
        </p>

        <div className="bg-black/40 border border-hack-border rounded p-3 mb-3 max-h-64 overflow-y-auto">
          <p className="text-hack-dim text-[10px] uppercase mb-1">Parameters (action_id: {actionId.slice(0, 8)}…)</p>
          <pre className="text-xs font-mono text-white whitespace-pre-wrap break-all">
            {JSON.stringify(params, null, 2)}
          </pre>
        </div>

        <input
          type="text"
          value={reason}
          onChange={(e) => setReason(e.target.value)}
          placeholder="Optional reason (sent with deny)"
          className="w-full bg-black/30 border border-hack-border text-white text-sm p-2 rounded mb-3 focus:outline-none focus:border-hack-primary"
        />

        <div className="flex justify-end gap-2">
          <button
            onClick={onClose}
            className="px-3 py-1.5 text-xs font-mono uppercase bg-black/30 text-hack-dim border border-hack-border rounded hover:bg-black/50"
          >
            Dismiss
          </button>
          <button
            onClick={() => onDeny(reason || "denied by operator")}
            className="px-3 py-1.5 text-xs font-mono uppercase bg-red-600 hover:bg-red-700 text-white rounded flex items-center gap-1"
          >
            <ShieldOff className="h-3 w-3" /> Deny
          </button>
          <button
            onClick={onApprove}
            className="px-3 py-1.5 text-xs font-mono uppercase bg-green-600 hover:bg-green-700 text-white rounded flex items-center gap-1"
          >
            <Shield className="h-3 w-3" /> Approve
          </button>
        </div>
      </div>
    </div>
  );
}
