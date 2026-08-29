import { Crosshair, Loader2 } from "lucide-react";

interface TurnCounterProps {
  current: number;     // 1-based turn number (0 means "no turn yet")
  max: number;         // typically 20
  status: "running" | "paused" | "cancelled" | "completed" | "failed" | "idle";
}

// TurnCounter renders the agent's current turn in the live panel.
// Shows a progress bar (filled = current/max) and an animated
// spinner when the LLM call is in flight.
export default function TurnCounter({ current, max, status }: TurnCounterProps) {
  const pct = max > 0 ? Math.min(100, (current / max) * 100) : 0;
  const isLive = status === "running";
  return (
    <div className="flex items-center gap-2 font-mono text-xs" data-testid="turn-counter">
      <div className="flex items-center gap-1 text-hack-primary">
        {isLive ? (
          <Loader2 className="h-3 w-3 animate-spin" />
        ) : (
          <Crosshair className="h-3 w-3" />
        )}
        <span>
          Turn {current || "–"} / {max}
        </span>
      </div>
      <div className="flex-1 h-1 bg-black/40 rounded overflow-hidden">
        <div
          className="h-full bg-hack-primary transition-all duration-300"
          style={{ width: `${pct}%` }}
        />
      </div>
    </div>
  );
}
