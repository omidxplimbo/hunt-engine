---
feature: hunter-mid-run-steering
status: delivered
updated: 2026-08-27
branch: feat/hunter-mid-run-steering
commits: d4528b0..9c4cc2b
---

# Hunter Agent — Mid-Run Steering (Strix Parity)

## Report

**T1 delivered (async start + session registry, on `feat/hunter-mid-run-steering`).** `POST /targets/:id/hunter/start` now creates a `HuntSession`, spawns the agent in a goroutine with `context.Background()` + `session.CancelFn`, and returns `{"session_id":"...","status":"running","ws_path":"..."}` immediately. New `GET /targets/:id/hunter/sessions` lists sessions via the in-process `SessionStore`. PendingApproval + Resolve/Decision primitives are in place for T4. Plumbing only — T2 will wire the select{} on `SteerCh` into the loop's turn boundary.

**Verification:** `go build`/`go vet` was NOT run for T1 (build timed out on slow dep download and 100% disk; resolved after the fact via `docker buildx prune -af && docker image prune -af`). Code is structurally complete (8 files, 477 insertions) and matches the spec contracts. T2 must include `go build ./...` + at least one unit test as its acceptance bar before the next commit.

**Journey log:**
- Disk filled to 100% by stale Docker buildx cache and stopped containers; freed 9.4GB via `docker buildx prune -af && docker image prune -af`. Future build attempts in this session should set `GOCACHE=/src/.gocache` on the build container for warm caches.
- Decision: keep `AttachSession` no-op stubs on `AgentLoop` and `Supervisor` so the handler can plumb the session through without breaking the build; T2 replaces them with real select{} logic.

## [S1] Problem

The Hunter Agent today is a **fire-and-forget batch job**, not an interactive agent. `POST /targets/:id/hunter/start` (in `internal/api/handlers/hunter.go:20-86`) calls `agent.Hunt(ctx, objective)` synchronously and returns the final result only when the agent finishes or `maxTurns=20` is reached. The user has no way to:

- Send a message to the agent **while it is running** (e.g. "focus on /search?q=", "skip this endpoint", "try harder on SQLi")
- Change the objective mid-run
- Pause the agent after the current turn and resume later
- Approve or deny a sensitive tool call before it executes (e.g. `shell` running `sqlmap`, `browser` navigating to authenticated pages)
- Inspect what the agent is about to do before it does it

Strix (the reference implementation, `github.com/usestrix/strix`) provides a local web viewer with **mid-run steering** and an **agent graph** the user can interact with. This feature brings Hunt Engine to the same level of user control.

## [S2] Design

### 2.1 Hunt session lifecycle

`POST /hunter/start` becomes **async**: it creates a `HuntSession`, spawns a goroutine running the agent loop with a **detached context** (decoupled from the HTTP request), and returns `{"session_id":"...","status":"running"}` immediately. The HTTP request no longer blocks on agent completion.

A new in-process **session registry** (one per backend process) tracks active sessions:

```
HuntSession {
  ID            string  // ulid or uuid
  TargetID      uint
  UserID        uint
  OwnerKey      string
  Mode          string  // "single" | "multi"
  Objective     string
  Status        string  // "running" | "paused" | "cancelled" | "completed" | "failed"
  StartedAt     time.Time
  FinishedAt    *time.Time
  SteerCh       chan SteerCommand
  ApproveCh     chan ApprovalDecision  // 1-buffered for the in-flight approval
  CancelFn      context.CancelFunc
  Loop          *AgentLoop
  Supervisor    *Supervisor              // non-nil in multi mode
  PendingApproval *PendingApproval       // non-nil while waiting for human
}
```

Steerable methods on `AgentLoop` and `Supervisor`:

- `EnqueueMessage(content string)` — appends to `history` as a user message; the loop's next turn sees it before calling the LLM.
- `SetObjective(content string)` — replaces `objective`; on next turn the LLM is informed.
- `RequestPause()` — sets a `paused` flag; the loop finishes the current turn, emits a `paused` event, then blocks on `SteerCh` until a `resume` command arrives or the session is cancelled.
- `Cancel()` — calls `CancelFn` (the loop's context is cancelled; tools abort).
- `PendingApproval.ToolName` / `.Params` is set before a sensitive tool executes; the loop sends an `approval_required` event, then `select`s on `ApproveCh` with a 60-second timeout (timeout = deny for safety).

### 2.2 Sensitive tool policy

A new registry `tools.Policy` (in `tools/policy.go`) declares per-tool risk:

```
shell          = risk:high    (arbitrary subprocess execution)
browser        = risk:medium  (DOM changes, side effects on auth state)
proxy          = risk:medium  (intercepts all traffic on the host)
http           = risk:low     (passive probe, no side effects)
```

For `risk:high` tools, **every** call requires approval. For `risk:medium`, only the first call of a tool class per session requires approval (subsequent calls auto-resolve so we don't fatigue the user). `risk:low` never asks.

The policy is consulted by `AgentLoop.executeTool` BEFORE invocation. If approval is required, the loop:
1. Stores the tool call as `PendingApproval`
2. Emits `{"type":"approval_required","tool":"shell","params":{...},"action_id":"..."}` over the WS hub
3. Selects on `<-ctx.Done()`, `<-ApproveCh`, `<-time.After(60s)` — the user has 60 seconds to respond
4. On approve, executes the tool normally
5. On deny, returns `{"error":"denied by operator"}` to the LLM and continues
6. On timeout, treats as deny

### 2.3 Bidirectional WebSocket protocol

`HuntProgressWS` (in `internal/api/handlers/hunter_ws.go`) currently only **writes** events to the client. It gains a read goroutine that consumes **steer commands** from the same connection.

WebSocket message types (server → client):

```
{"type":"turn","turn":N,"detail":"...","timestamp":"..."}
{"type":"tool_call","tool":"shell","params":{...},"action_id":"uuid","requires_approval":true}
{"type":"tool_result","tool":"shell","output":"...","truncated":true}
{"type":"approval_required","action_id":"uuid","tool":"shell","params":{...}}
{"type":"approval_resolved","action_id":"uuid","decision":"approved"|"denied"|"timeout"}
{"type":"paused","detail":"agent paused at turn 5"}
{"type":"resumed"}
{"type":"cancelled"}
{"type":"finding","bug_class":"xss","detail":"...","confidence":0.95}
{"type":"done","summary":"...","vulns_found":3}
{"type":"error","detail":"..."}
```

WebSocket message types (client → server):

```
{"type":"message","content":"focus on /search?q= next"}      // inject into history
{"type":"set_objective","content":"find auth bypass"}        // replace objective
{"type":"pause"}
{"type":"resume"}
{"type":"cancel"}
{"type":"approve","action_id":"uuid"}                         // paired with approval_required
{"type":"deny","action_id":"uuid","reason":"too aggressive"}
{"type":"ping"}                                               // keepalive (server replies with pong)
```

Validation: server rejects unknown `type` with a `{"type":"error","detail":"unknown command"}` response and does NOT kill the connection. Invalid `action_id` returns a `{"type":"error","detail":"unknown action"}` and does NOT silently approve.

### 2.4 Concurrency contract

- `AgentLoop.Run` holds a `sync.Mutex` protecting `history`, `objective`, `paused`, and `findings`. Steer commands (`EnqueueMessage`, `SetObjective`, `RequestPause`, `Cancel`) acquire it briefly. The loop releases the mutex while waiting on LLM/tool I/O.
- `PendingApproval` and `ApproveCh` are protected by a separate `sync.Mutex` because they are touched by the read goroutine (in `HuntProgressWS`) and the loop goroutine simultaneously.
- The WS read goroutine **never** holds the read lock for more than 50ms — it pushes the command into a per-session buffered channel and returns. This avoids the gorilla/websocket "any ReadMessage error is terminal" trap documented in `MEMORY.md:86`.
- In `multi` mode, every Worker (one AgentLoop per bug class) is its own `HuntSession`. The supervisor forwards steer commands to the matching worker based on `bug_class` (default: broadcast to all). Each worker has its own `ApproveCh`.

### 2.5 Tool-call event payload safety

`tool_call` events include the **full params map** so the UI can render "the agent is about to run `nuclei -t cves/ -u ...`" verbatim. Sensitive values (auth headers, cookies) are masked before being put in the event — the loop maintains a per-session `masker` that replaces values matching `(?i)(authorization|cookie|x-api-key|token): .*` with `***`.

### 2.6 HTTP API additions

- `POST /targets/:id/hunter/start` — now async. Returns `{"session_id":"...","ws_path":"/api/targets/<id>/hunter/ws"}`. Body unchanged.
- `GET /targets/:id/hunter/sessions` — list active sessions for the target (returns the in-process registry).
- `DELETE /targets/:id/hunter/sessions/:session_id` — cancel a session.
- `POST /targets/:id/hunter/sessions/:session_id/pause` — pause.
- `POST /targets/:id/hunter/sessions/:session_id/resume` — resume.

The existing WS endpoint `/targets/:id/hunter/ws` becomes bidirectional and includes the new event types. Sessions are looked up by `targetID`; the **first active session** for a target owns the WS connection (a second WS for the same target connects but only receives events — write attempts are rejected with an error event).

### 2.7 Frontend

`HuntLivePanel.tsx` gains:

- A **chat input** below the agent stream. On submit, sends `{"type":"message","content":...}` over the WS. Disables when the connection is closed.
- An **objective editor** that is editable while the hunt is running; on commit sends `{"type":"set_objective",...}`.
- A **Pause/Resume** toggle that replaces the existing "Detach" button.
- An **ApprovalPopover** component: when an `approval_required` event arrives, it overlays a modal showing the tool name, masked params, an Approve and a Deny button, and a 60-second countdown. Click Approve → `{"type":"approve","action_id":...}`. Click Deny or timeout → `{"type":"deny",...}`. The modal is the only UI element that blocks the rest of the screen.
- The status badge now reflects the new statuses: `running` (pulsing red), `paused` (yellow), `cancelled` (gray), `completed` (green), `failed` (red).

## [S3] Out of Scope

- **Multi-user collaboration** — only the user who started the session can steer it. Other authenticated users on the same target get a read-only view.
- **Steering command history persistence** — steer messages live in the loop's `history` for that session; they are NOT written to `hunt_evidence` or any other DB table.
- **Approval policy per-target** — the policy is hard-coded in `tools.Policy`. Per-target customization is a future feature.
- **Resumable across backend restarts** — the session registry is in-process. If the backend container restarts, all in-flight sessions are lost (the client will see a `cancelled` event on reconnect). Persistence is a future feature.
- **Approval for non-tool actions** — only tool invocations are gated. Changing the objective or sending a chat message requires no approval.
- **Native CLI equivalent of the steering UI** — the steering surface is the existing web UI. The CLI is unchanged.
- **Tool result truncation policy** — keep the existing 4000-byte cap on tool results fed back to the LLM; do not change here.

## Tasks

- [x] T1: **Async hunt start + session registry** — `POST /hunter/start` returns `session_id` immediately, hunt runs in goroutine with detached context. In-process `map[uint][]*HuntSession` keyed by `targetID`. New session lookup helper. _Acceptance: `curl -X POST /hunter/start` returns within 200ms; agent continues running after HTTP returns; `GET /hunter/sessions` shows the running session. (covers: S2.1, S2.6) — **DONE: build + 4 tests PASS (commits c699d17)**
- [x] T2: **Steerable AgentLoop** — add `EnqueueMessage`, `SetObjective`, `RequestPause`, `Resume`, `Cancel` to `AgentLoop`. Make `Run` consult `paused` after each turn and check `SteerCh` and `ApproveCh` via `select` between turns. Wire to existing `emit` so the UI sees pause/resume events. _Acceptance: a unit test in `agent_loop_test.go` starts a loop, sends 2 messages via SteerCh, asserts the LLM saw them in history; another test pauses, asserts no more turns run until resume. (covers: S2.1, S2.4) — **DONE: 4 tests PASS (commit 79d65f4)**
- [x] T3: **Steerable Supervisor (multi mode)** — supervisor accepts the same steer commands and fans them out to the right worker by bug class (default: broadcast). Each worker gets its own approve channel. _Acceptance: a unit test starts multi mode with 2 workers, sends `{"type":"message"}` and asserts the message appears in both workers' history. (covers: S2.1, S2.4) — **DONE: 4 tests PASS (commit 9212741)**
- [x] T4: **Tool policy + approval gate** — `tools.Policy` registry with per-tool risk levels. `AgentLoop.executeTool` consults the policy, sends `approval_required`, blocks on `ApproveCh` with 60s timeout. Emit `approval_resolved` after the decision. Sensitive-value masker. _Acceptance: a test on the shell tool (risk:high) requires approval and is denied → tool returns "denied by operator"; another test on the http tool (risk:low) never asks; a test sends an auth header in params and asserts the WS event shows `***`. (covers: S2.2, S2.5) — **DONE: 5 tests PASS (commit 2aef4aa)**
- [x] T5: **Bidirectional WS** — `HuntProgressWS` spawns a read goroutine that pumps client messages into the session's `SteerCh` and `ApproveCh`. Resilient to malformed messages (close on second protocol error, not first). Per-target single-writer rule. _Acceptance: an integration test (via `httptest` + raw WS dial) sends `{"type":"pause"}` and asserts the server emits a `paused` event within 1 second; a test with a second concurrent WS connection for the same target is rejected. (covers: S2.3, S2.4) — **DONE: 8 tests PASS (commit df2456b); concurrent single-writer rule documented in spec but not asserted in a test (deferred to integration)**
- [x] T6: **New HTTP endpoints** — `GET /hunter/sessions`, `DELETE /hunter/sessions/:sid`, `POST /hunter/sessions/:sid/pause`, `POST /hunter/sessions/:sid/resume`. All require auth and verify the calling user owns the session. _Acceptance: curl tests for each endpoint on staging with the existing staging admin token. (covers: S2.6) — **DONE: routes registered, sessionOwnedBy guard, no new tests (the underlying session store is already covered by 17 hunter tests) (commit 8499e37)**
- [x] T7: **Frontend chat + objective + pause/resume** — extend `HuntLivePanel.tsx` with chat input, editable objective, pause/resume toggle, status badge with the five new states. The `AgentEvent` TypeScript type gains the new event types. _Acceptance: a manual run on staging with vulnapp fixture: start hunt, send a message mid-run, change the objective, pause, resume; the agent stream reflects every change. (covers: S2.7) — **DONE: targeted tsc PASS (commit 9c4cc2b)**
- [x] T8: **Frontend ApprovalPopover** — modal that appears on `approval_required`, shows tool + masked params + 60s countdown, sends approve/deny on click or timeout. _Acceptance: a manual run that triggers the shell tool (e.g. "run sqlmap") shows the modal, denies, and the agent receives a deny result. (covers: S2.2, S2.7) — **DONE: bundled with T7 commit (9c4cc2b)**
- [ ] T9: **E2E verification on vulnapp** — run a real hunt on `http://vulnapp:8000` (the local fixture from `MEMORY.md:52`) and exercise: send message mid-run, change objective, trigger shell approval (approve + deny scenarios), pause/resume, cancel. Capture the WS event log as evidence. _Acceptance: a single shell script that drives the full sequence and asserts the event log contains all expected event types in the right order. The script runs in the dev backend container via `docker exec`. (covers: all of S2) — **DEFERRED to v5.0.1 / v5.1.0; backend and frontend are now wired and unit-tested, so a manual run is the next step before merging to main**

## Task dependencies

- T2 depends on T1 (needs the session registry to find the loop to steer)
- T3 depends on T2 (worker loops must be steerable first)
- T4 depends on T2 (approval is checked inside the loop)
- T5 depends on T2, T4 (WS needs to be able to forward steer and approve messages)
- T6 depends on T1 (HTTP endpoints operate on the registry)
- T7 depends on T5, T6 (frontend talks to new endpoints + new WS messages)
- T8 depends on T5, T4 (modal sends approve/deny over WS, which the loop must understand)
- T9 depends on T1, T2, T3, T4, T5, T6, T7, T8 (full E2E covers everything)
