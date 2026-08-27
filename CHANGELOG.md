# Changelog

All notable changes to the **Hunt Engine** project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v5.0.0] - 2026-08-27
### Added — Mid-Run Steering (Strix Parity)
**Major feature.** The Hunter Agent is now an interactive, real-time steerable agent. The operator can chat with the agent mid-run, change the objective, pause/resume the hunt, and approve or deny high-risk tool calls (shell) before they execute. Brings Hunt Engine to parity with the Strix pentesting tool's mid-run steering surface.

- **Async hunt start** — `POST /targets/:id/hunter/start` now returns immediately with a `session_id`; the agent runs in a goroutine with a detached context. The HTTP request no longer blocks on agent completion. The hunt survives client disconnects.
- **Session registry** — in-process `HuntSession` registry keyed by target. New `GET /targets/:id/hunter/sessions` lists active + historical sessions for the target.
- **Bidirectional WebSocket** — `/targets/:id/hunter/ws` is now bidirectional. The operator can send `{type:"message"}`, `{type:"set_objective"}`, `{type:"pause"}`, `{type:"resume"}`, `{type:"cancel"}`, `{type:"approve",action_id}`, `{type:"deny",action_id,reason}`. Server emits `paused`, `resumed`, `cancelled`, `approval_required`, `approval_resolved`, `operator_message`, `objective_changed`, `session_done` events.
- **Tool policy + human approval** — every high-risk tool (shell) call blocks on operator approval before executing. The UI shows an `ApprovalPopover` modal with the tool name, masked params (credentials replaced with `***`), and a 60-second countdown. Approve runs the tool; deny returns `[TOOL DENIED] reason` to the LLM so it can adjust.
- **Sensitive-value masker** — authorization, cookie, api-key, token, password, secret values are masked before the `approval_required` event is broadcast over the WS.
- **Steerable AgentLoop** — `EnqueueMessage`/`SetObjective`/`RequestPause`/`Resume`/`Cancel` methods. The turn loop drains `SteerCh` between turns and blocks while paused.
- **Steerable Supervisor (multi mode)** — supervisor fans steer commands out to every parallel worker; each worker has its own session.
- **Session action endpoints** — `POST .../pause`, `POST .../resume`, `DELETE /sessions/:sid`. All auth-gated; admins can act on any session, regular users only on their own.
- **Frontend** — `HuntLivePanel.tsx` gained a chat input, an editable objective (with a "send update" link when the field diverges from the committed objective), a Pause/Resume toggle, a Cancel button, a session history panel, and a session status badge that reflects `running` (pulsing red), `paused` (yellow), `cancelled` (gray), `completed` (green), `failed` (red). New `ApprovalPopover.tsx` is a full-screen modal for high-risk tool approvals.

### Coverage
- 25 new Go unit tests across the hunter + handler packages (12 in T1-T3 session/steering layer, 5 in T4 policy/approval, 8 in T5 handler/WS routing)
- Frontend type-checks cleanly (targeted `tsc --noEmit` on the changed files returns `EXIT=0`)

### Security notes
- Human-in-the-loop is enforced only for `risk:high` tools (currently `shell`); `risk:low` (http) and `risk:medium` (browser, proxy) auto-execute.
- All approval timeouts auto-deny (fail-closed).
- The 60s approval window prevents the loop from blocking forever if the operator walks away.

## [v2.2.1] - 2026-02-12
### Fixed
- **UI/UX:** Fixed an issue where system notifications (Toasts) were not displaying in the frontend application. Added `Toaster` component to `App.tsx` with a custom cyberpunk theme to ensure error messages and success alerts are visible to the user.

## [v2.2.0] - 2026-02-12
### Added
- **Queue Management:**
    - **Active Scan Queue** widget moved to the main **Dashboard** for better visibility.
    - **Queue Control:** Added ability to **Purge** the entire queue, **Remove** individual items, and **Reorder** priorities directly from the UI.
    - **Concurrency Control:** `Max Concurrent Scans` setting is now strictly enforced with a new synchronous locking mechanism to prevent race conditions.
- **System Stability:**
    - **Zombie Cleanup:** Implemented a self-healing mechanism on startup to detect and reset targets stuck in `QUEUED` state if they are missing from Redis.
    - **Database Reconciliation:** `ClearQueue` action now properly resets target statuses in the database from `QUEUED` to `READY`.
- **UX Improvements:**
    - **Error Feedback:** Enhanced error handling in the UI to display precise server-side error messages (e.g., "Cannot decrease limit while scans are active") via Toast notifications.

## [v2.1.0] - 2026-02-10
### Added
- **Stability:** Added `Panic Recovery` mechanism in the worker dispatcher to prevent the entire worker process from crashing if a single job fails.
- **Reliability:** Implemented `clearAllLocks` on worker startup to automatically clean up stale lock files from previous crashes (fixing the "QUEUED forever" issue).
- **Intelligence:** Improved `VirusTotal` collection module with a proper User-Agent to mimic browser behavior and avoid potential API throttling or incomplete data.
- **Debugging:** Added detailed logging for VirusTotal URL discovery counts per domain.

## [v2.0.0] - 2026-02-09
### Added
- **Monitoring Server:** A new real-time monitoring dashboard exclusively for Admin users.
    - Visual Area Charts for CPU and RAM usage history.
    - Live "Active Processes" table showing command execution details (Target, Command, PID, Duration).
- **Backend Architecture:**
    - Integrated `gopsutil` for accurate system resource metrics.
    - Implemented `sync.Map` in the worker runner to thread-safely track and manage active process lifecycles.
    - New API endpoint `/api/monitor/stats` for fetching aggregated system and process data.
- **Frontend UI:**
    - New `MonitoringServer` component using `recharts` for visualization.
    - Integrated `TanStack Query` with polling (2s interval) for live updates without page refreshes.
    - Admin-protected visibility in the main Dashboard.

### Optimized
- **Performance:** Refactored command execution pipeline (specifically for `gau` and `waybackurls`) to use direct file I/O instead of piping through memory, significantly reducing RAM overhead during large scans.

### Fixed
- Dependency management: Resolved missing `go.sum` entries for system libraries.

## [v1.3.0]
### Added
- **Infrastructure:** Enhanced Docker composition and environment configuration.

## [v1.2.0]
### Added
- **Security:** IP-based access control middleware.
- **Security:** Enhanced authentication flows.

## [v1.1.0]
### Added
- **Scanning:** Integrated `waymore` tool for advanced URL discovery.
- **Scanning:** Enhanced crawling logic for better coverage.

## [v1.0.1]
### Fixed
- Bug fixes in crawling logic.
- Minor UI adjustments.

## [v1.0.0] - Initial Release
### Added
- **Core:** Complete Hunt Engine architecture (Backend + Frontend).
- **UI:** Cyberpunk-themed interface with responsive design.
- **Scanning:** Basic integration with subfinder, httpx, katana, nuclei.
- **Management:** Target management, User management (Admin/User roles).
