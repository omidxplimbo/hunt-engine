# Hunt Engine v5.0.0 — Mid-Run Steering (Strix Parity)

**Release date:** 2026-08-27
**Status:** Deployed to `feat/hunter-mid-run-steering` branch
**Spec:** [`docs/compose/spec/hunter-mid-run-steering.md`](docs/compose/spec/hunter-mid-run-steering.md)

## What changed

The Hunter Agent is no longer a fire-and-forget batch job. The operator can now steer the hunt while it is running, and the backend blocks any high-risk tool call until the operator approves it.

### Backend (Go)

| Layer | What was added |
|---|---|
| `internal/ai/hunter/session.go` (new) | `HuntSession`, `PendingApproval`, `SessionStore`. In-process registry keyed by targetID. |
| `internal/ai/hunter/agent_loop.go` | `EnqueueMessage` / `SetObjective` / `RequestPause` / `Resume` / `Cancel`; `select{}` on `SteerCh` between turns; `drainSteer(block bool)`; mutex-protected history. |
| `internal/ai/hunter/supervisor.go` | `steerDispatcher` goroutine + `broadcastSteer` for the multi-agent mode. |
| `internal/ai/hunter/tools/policy.go` (new) | `RiskLevel` table (low/medium/high) + `MaskSensitiveParams` (masks authorization/cookie/token/api_key/password/secret values). |
| `internal/ai/hunter/agent_loop.go` (T4) | `executeTool` consults the policy; high-risk tools block on operator approval with a 60s timeout → deny. |
| `internal/api/handlers/hunter.go` | `StartHunt` is now async; returns `session_id` immediately. |
| `internal/api/handlers/hunter_ws.go` | Bidirectional WS: `handleClientMessage` parses operator commands. |
| `internal/api/handlers/hunt_session_actions.go` (new) | `POST .../pause`, `POST .../resume`, `DELETE /sessions/:sid`. |
| `internal/api/handlers/hunt_session_routes.go` (new) | `GET /targets/:id/hunter/sessions` lists the in-process registry. |
| `cmd/server/main.go` | Routes registered. |

### Frontend (React/TypeScript)

| File | What was added |
|---|---|
| `src/api/hunter.ts` | `AgentEvent` widened from 6 to 17 event types; `SessionSnapshot` / `SessionStatus` types; `listHuntSessions`, `pauseHuntSession`, `resumeHuntSession`, `cancelHuntSession` API helpers. |
| `src/components/HuntLivePanel.tsx` (rewrite) | Chat input, editable objective, Pause/Resume toggle, Cancel button, status badge, session history panel, session_id display. |
| `src/components/ApprovalPopover.tsx` (new) | Full-screen modal with tool name, masked params, 60s countdown, Approve / Deny buttons, optional reason input, auto-deny on timeout. |

## Wire protocol (WebSocket)

Server → client events now include:

```json
{"type":"paused","detail":"agent paused at turn 5"}
{"type":"resumed"}
{"type":"cancelled"}
{"type":"operator_message","detail":"from operator"}
{"type":"objective_changed","detail":"new objective"}
{"type":"approval_required","action_id":"uuid","tool_name":"shell","params":{...}}
{"type":"approval_resolved","detail":"approve:true"}
{"type":"session_done","detail":"completed"}
```

Client → server commands:

```json
{"type":"message","content":"focus on /search?q= next"}
{"type":"set_objective","content":"find auth bypass"}
{"type":"pause"}
{"type":"resume"}
{"type":"cancel"}
{"type":"approve","action_id":"uuid"}
{"type":"deny","action_id":"uuid","reason":"too aggressive"}
{"type":"ping"}
```

## HTTP API additions

```
POST   /api/targets/:id/hunter/start                          # now async, returns {session_id, status, ws_path}
GET    /api/targets/:id/hunter/sessions                       # list
POST   /api/targets/:id/hunter/sessions/:sid/pause
POST   /api/targets/:id/hunter/sessions/:sid/resume
DELETE /api/targets/:id/hunter/sessions/:sid
GET    /api/targets/:id/hunter/ws?token=<jwt>                 # now bidirectional
```

## Test coverage

25 new Go unit tests (12 in T1-T3 steering layer, 5 in T4 policy/approval, 8 in T5 handler/WS routing). All pass:

```
$ go test -count=1 ./internal/ai/hunter/ ./internal/api/handlers/
ok  internal/ai/hunter      0.044s
ok  internal/api/handlers   0.231s
```

## Out of scope (deferred to v5.x)

- **Multi-user collaboration** — only the user who started the session can steer it. Other authenticated users on the same target get a read-only view of the WS stream.
- **Steering command history persistence** — steer messages live in the loop's history for that session; they are NOT written to `hunt_evidence`.
- **Approval policy per-target** — the policy is hard-coded in `tools.Policy`.
- **Resumable across backend restarts** — the session registry is in-process. If the backend container restarts, all in-flight sessions are lost.
- **Approval for non-tool actions** — only tool invocations are gated. Changing the objective or sending a chat message requires no approval.
- **E2E on vulnapp** (T9) — script written: `scripts/smoke_steering_v5_0_0_rc1.sh` (syntax-validated, follows the existing `smoke_*.sh` pattern). The live run is the next deploy step — the hunt-backend container must first be rebuilt from a checkout that includes the v5.0.0-rc.1 steering commits; the current `hunt-backend` image is the v5.0.0 baseline and does not yet have the steering handlers.

## Security notes

- Human-in-the-loop is enforced only for `risk:high` tools (currently `shell`); `risk:low` (http) and `risk:medium` (browser, proxy) auto-execute.
- All approval timeouts auto-deny (fail-closed).
- The 60s approval window prevents the loop from blocking forever if the operator walks away.
- Sensitive credential values are masked before any `approval_required` event leaves the backend.
