---
feature: agent-live-view-qa-creative
status: designed
updated: 2026-08-28
branch: feat/agent-live-view-qa-creative
commits: <base-sha>..<head-sha>
---

# Agent Live View + Interactive Q&A + Creative Payload Generation (T14)

## Report

_(empty — design phase)_

## [S1] Problem

The Hunter Agent already streams its activity over the WebSocket (T5, v5.0.0) and the frontend shows a text feed (T7). The operator's complaint is concrete:

1. **"کاملاً لایو ببینم داره چیکار میکنه"** — the current feed is plain text. There is no visual structure, no progress bar per turn, no inline request/response viewer, no tool-call tree. A 20-turn hunt with 60+ tool calls becomes an undifferentiated wall of text the operator cannot follow in real time.
2. **"نه صرفا استفاده از ابزارهارو"** — the agent is currently bound to a hardcoded tool registry (`shell`/`http`/`browser`/`proxy`) and a small set of payload templates. When the LLM wants to try a 7-layer WAF bypass, mutate a payload on the fly based on the previous response, or chain a tool result into a custom request shape, it has nowhere to go.
3. **"خودش بتونه بعضی وقتا خلاقیت به خرج بده"** — the system prompt currently lists the four tools and a fixed set of skill markdown files. There is no explicit "creative mode" where the LLM is invited to invent payloads, mutate them based on observed response, and chain. Strix's hunter does this naturally.
4. **"نظر پرسیدن تعاملی از من"** — the agent cannot ask the operator questions mid-run. If it needs authorization to test an authenticated endpoint, or wants to confirm scope on a sensitive path, the only options today are: stop the hunt and start a new one with the new objective, or have the operator push a `set_objective` and let the LLM guess. There is no `ask_operator` tool / event.

The Hunter Agent must evolve from a scriptable tool-execution loop into a **collaborative, visually-observable, creative agent** that the operator watches and steers in real time.

## [S2] Design

### 2.1 Three additive features

This spec covers three orthogonal features that together address the operator's complaint. They share the same AgentLoop turn boundary and the same WebSocket protocol, so the backend plumbing is reused.

1. **Rich live stream** (frontend-only) — replace the text-only feed with a structured visual surface: per-turn cards, progress bar, tool-call tree, request/response inspector, terminal-like color-coded log.
2. **Interactive operator Q&A** (backend + frontend) — a new tool `ask_operator` the LLM can invoke. Blocks the loop until the operator answers (or 5-minute timeout → auto-skip). Frontend shows an inline question card with a free-text input.
3. **Creative payload generation** (backend prompt + new tool) — add a `craft_payload` tool that lets the LLM synthesize a custom payload for a specific endpoint based on the response it just saw. The system prompt invites this explicitly. Combine with `http`/`shell` for multi-layer evasion chains.

### 2.2 Rich live stream (frontend)

**File**: `frontend/src/components/HuntLivePanel.tsx` + new subcomponents.

Replace the current text-feed with a structured surface. Each WS event maps to a typed UI element:

| WS event | UI element |
|---|---|
| `turn` | Animated turn counter chip "Turn 5 / 20", pulsing while the LLM call is in flight |
| `tool_call` | Card with tool name, masked params (chips), and a "View request" toggle that opens the full request/response |
| `tool_result` | Card with the tool output (truncated by default; "Show more" expands), HTTP status badge if applicable |
| `finding` | Promoted card with confidence ring, severity badge, bug class, one-click "Promote to Finding" |
| `paused` / `resumed` / `cancelled` / `session_done` | Status banner with the appropriate icon and color |
| `operator_message` | Cyan "you: …" card (existing) |
| `objective_changed` | Purple card with the new objective |
| `approval_required` | Yellow card (existing) + the modal opens (existing) |
| `approval_resolved` | Grey card with the decision |

New subcomponents:

- `TurnCounter` — animated chip at the top, shows "Turn 5 / 20" and a thin progress bar
- `ToolCallCard` — collapsible card; the "params" view is a JSON tree with masked values highlighted in yellow; the "response" view is the raw text with a "Copy" button
- `RequestInspector` — modal opened from a `tool_call` card; shows method, URL, headers, body, response status, response headers, response body (truncated to 4KB with "Show full" toggle)
- `LiveProgress` — the global turn progress bar at the top of the panel

**Wire change**: NONE. The frontend just maps existing events to richer components.

### 2.3 Interactive operator Q&A (backend + frontend)

**Backend file**: `backend/internal/ai/hunter/tools/ask_operator.go` (new).

A new tool that registers like any other:

```go
type AskOperatorTool struct{}
func (AskOperatorTool) Name() string             { return "ask_operator" }
func (AskOperatorTool) Description() string      { return "Ask the operator a question. The loop blocks until the operator answers (or 5 minutes → auto-skip with no answer)." }
func (AskOperatorTool) Schema() map[string]any   { return map[string]any{...} }
func (AskOperatorTool) Execute(ctx, params) (string, error) {
    question, _ := params["question"].(string)
    if question == "" { return "", errors.New("question is required") }
    if a := askOperatorFromCtx(ctx); a != nil {
        return a.ask(ctx, question)
    }
    return "operator unavailable", nil
}
```

The tool hangs off an interface (not a free function) so the AgentLoop can pass an `OperatorChannel` via context. `OperatorChannel.ask(ctx, question) (string, error)` blocks until the operator answers (text) or 5 minutes elapses (returns `"[OPERATOR DID NOT ANSWER WITHIN 5 MINUTES — skipping]"`, exit-code-style so the LLM can choose to skip or reroute).

The session owns a `OperatorChannel` (a `chan string` with the answer and a `pending` flag). When `ask_operator` is called:

1. Emit `operator_question` event with `{question, action_id, asked_at}` so the UI can render it.
2. Block on the channel for up to 5 minutes.
3. When the operator answers (via WS `{"type":"operator_answer","action_id":"...","content":"..."}` or the HTTP endpoint), close the channel and return the text.
4. On timeout, return the skip marker.

The AgentLoop's `executeTool` already has a permission gate (T4). `ask_operator` is `RiskLow` so it auto-executes; the gating is on the channel, not the tool policy.

**Frontend**: a new subcomponent `OperatorQuestionCard` mounted in the events feed. When an `operator_question` event arrives, it shows the question as a quoted text + a textarea + a "Send" button. The textarea dispatches `{"type":"operator_answer","action_id":"...","content":"..."}` over the WS. Multiple questions can be pending in theory; the UI shows them as a stack with the oldest at the top.

### 2.4 Creative payload generation (backend prompt + new tool)

**Backend file**: `backend/internal/ai/hunter/tools/craft_payload.go` (new).

A new tool that generates a context-specific payload:

```go
type CraftPayloadTool struct{}
func (CraftPayloadTool) Name() string             { return "craft_payload" }
func (CraftPayloadTool) Description() string      { return "Generate a custom exploit payload for a specific endpoint based on the response you just saw. Use this for WAF bypass, multi-layer encoding, mutation chains, or context-specific variants the registry cannot enumerate." }
func (CraftPayloadTool) Schema() map[string]any   { return {type: object, properties: {vector: {type: string, enum: [xss_reflected, xss_stored, sqli_error, sqli_blind, sqli_time, ssti, cmdi, lfi, ssrf, idor, generic]}, target: {type: string, description: "The full URL or identifier you want to test"}, observations: {type: string, description: "What did the previous response look like? (e.g. 'WAF blocks <script>, but encodes single quotes')"}, strategy: {type: string, enum: [waf_bypass, multi_layer, mutation_chain, time_based, generic], default: generic}, max_attempts: {type: integer, default: 3, minimum: 1, maximum: 10}}}
func (CraftPayloadTool) Execute(ctx, params) (string, error) {
    // Returns 1..max_attempts payload variants as JSON list of {payload, technique, why}
}
```

The tool itself is a thin LLM call: it sends a system prompt that includes the LLM-generated payload templates + the operator's `observations` field, and asks the model to produce 1..N candidate payloads with a one-sentence `why` each. The candidates are then injected into a small "playbook" string the LLM can iterate on with `http` / `browser` calls in the next turn.

**System prompt update**: `agent_loop.go::buildSystemPrompt` gains a paragraph that explicitly invites the LLM to be creative:

> "You are not limited to the registry tools. When you encounter a response that suggests a payload mutation, a WAF signature, or an evasion opportunity, call `craft_payload` with the response observation; the tool returns 1..N payload variants. Iterate with `http` / `browser` to verify. You may also call `ask_operator` to confirm scope on sensitive endpoints."

**Wire change**: NONE. `craft_payload` and `ask_operator` look like any other tool call; the frontend's `ToolCallCard` already handles the generic case.

### 2.5 Concurrency contract

- `OperatorChannel` is a per-session struct owned by `HuntSession`, protected by the existing `HuntSession.mu`. `ask(ctx, q) (string, error)` increments a `pendingOperatorQ` counter; the channel carries `(actionID, answer)`.
- The WS read goroutine (T5) dispatches `operator_answer` to the same session; `OperatorChannel.resolve(actionID, answer)` matches by action_id and wakes the blocked `ask`.
- Multiple `ask_operator` calls in flight are allowed (one per action_id). The session tracks a `map[string]chan string` of pending questions, keyed by action_id. Resolved questions are removed from the map.
- `craft_payload` reuses the existing LLM call infrastructure (`llmclient.GenerateJSON`). It does NOT use the same context as the outer AgentLoop; it has its own short timeout (30s) and is `RiskLow`.
- The frontend's `OperatorQuestionCard` is mounted once in the events feed but renders every pending question in a stack, each independently dispatchable.

### 2.6 Data model additions

- New table `operator_questions`:
  - `id uuid PK`
  - `session_id uuid FK → hunt_sessions.id`
  - `action_id text unique`
  - `question text not null`
  - `asked_at timestamptz default now()`
  - `answered_at timestamptz nullable`
  - `answer text nullable`
  - `timed_out bool default false`
- No new table for `craft_payload` — the LLM-generated variants are not persisted (the next `tool_call` event already persists the request/response that uses them).

### 2.7 HTTP API additions

- `GET /api/targets/:id/hunter/operator-questions` — list pending + recent questions for a target (for operators who lost the WS connection).
- No new endpoint for the answer — the answer flows over the WS only (T5 already routes it to `OperatorChannel.resolve`).

## [S3] Out of Scope

- **Persisted "creative playbooks"** — the payloads `craft_payload` returns are not stored as named recipes for reuse. Future feature.
- **Operator-side rate limiting on questions** — the LLM could ask 100 questions in a row and stall the hunt. We rely on the LLM's system prompt to be reasonable; a "questions per turn" budget is a future feature.
- **Multi-modal operator input** — the operator can answer with text only. Voice / screenshot / file upload is future.
- **Cross-session shared operator memory** — questions and answers are per-session. Aggregating across sessions is future.
- **Payload sandboxing** — `craft_payload` returns a string the LLM uses. We do NOT execute the payload. The next `http` / `shell` / `browser` call is what executes it, and that call is already gated by the T4 tool policy.

## Tasks

- [ ] T1: **OperatorChannel + ask_operator tool** — `tools/ask_operator.go` with `OperatorChannel` on the session, `ask(ctx, q) (string, error)`, 5-minute timeout, ask/resolve by action_id, WS dispatch for `operator_answer`. Unit tests cover: ask + answer, ask + timeout, ask + WS dispatch, ask + multiple-pending-questions. (covers S2.3, S2.5, S2.6, S2.7)
- [ ] T2: **craft_payload tool** — `tools/craft_payload.go` with the LLM-backed payload synthesizer. Schema: vector, target, observations, strategy, max_attempts. Returns 1..N variants. Unit tests cover: schema validation, LLM call dispatched, output shape. (covers S2.4)
- [ ] T3: **System prompt creativity paragraph** — `buildSystemPrompt` adds the explicit invitation to use `craft_payload` and `ask_operator`. Reuse the existing "AUTHORIZATION CONTEXT" block as a template. (covers S2.4)
- [ ] T4: **Tool registry wires the new tools** — `AgentLoop.NewAgentLoop` registers `ask_operator` and `craft_payload` alongside the existing four. (covers S2.3, S2.4)
- [ ] T5: **Frontend: rich live stream subcomponents** — `TurnCounter`, `ToolCallCard`, `RequestInspector`, `LiveProgress`, `FindingCard`. Reuse existing `eventStyle` and `AgentEvent` types; do NOT change the wire schema. (covers S2.2)
- [ ] T6: **Frontend: OperatorQuestionCard** — mounted in the events feed, renders every pending `operator_question` event with a textarea + "Send" button. Dispatches `{"type":"operator_answer","action_id":"...","content":"..."}` over the WS. Handles the case where the answer is auto-skipped after 5 minutes (event `operator_skipped`). (covers S2.3, S2.5)
- [ ] T7: **Backend: `operator_questions` table + handler** — GORM migration + `GET /api/targets/:id/hunter/operator-questions` listing. (covers S2.6, S2.7)
- [ ] T8: **Wire: `operator_question` server event** — when `ask_operator` is invoked, emit `{type:"operator_question", question, action_id, asked_at}` BEFORE blocking. This is the only new event type. (covers S2.3, S2.5)
- [ ] T9: **E2E on vulnapp** — extend `scripts/smoke_steering_v5_0_0_rc1.sh` with two new sequences: (a) the agent's first turn is asked to call `ask_operator` with the question "may I test the /admin endpoint?"; the smoke script answers via WS. (b) the agent is asked to call `craft_payload` for an XSS vector that bypasses a hypothetical WAF; the response is asserted to be a JSON list. Document the new test in the script's [5/7] step. (covers S2.3, S2.4, S2.5)
- [ ] T10: **Documentation** — extend `docs/documentation/hunt-engine-v5-user-guide-fa.html` with a new section "Live view + Creative payload + Q&A (T14)" that describes the three features and how to use them. Also add a new "operator_questions" endpoint to the API quick reference. (covers all of S2)

## Task dependencies

- T5 depends on T8 (frontend renders `operator_question` events that the backend must emit)
- T6 depends on T1, T8 (frontend asks the operator; backend must have an OperatorChannel and emit the event)
- T7 depends on T1 (OperatorChannel must exist before we can persist questions to DB)
- T9 depends on T1, T2, T4, T8 (the smoke script drives both new tools through the live backend)
- T10 depends on everything (docs reflect the final surface)

## Out of scope reminders (carried from S3)

- No persisted "creative playbooks" table.
- No rate-limiting on operator questions (system prompt is the only guard).
- No cross-session operator memory.
- `craft_payload` is a thin LLM call; the next `http` / `browser` is what executes the generated payload.
