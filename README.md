<!-- HUNT_V3_12_0_START -->

## v3.15.0 — User-Programmable Operator Skills + Personal Operator Learning

v3.15.0 adds the first user-programmable operator skill and methodology layer for Hunt Engine's AI-driven authorized pentest workflow. The Operator can now use target-selected methodology records as planning guidance, expose applied methodology in chat, manage user-defined executable skills, select matching custom skills from chat trigger signals, create planned OperatorSkillRun records for those custom skills, and show dispatcher-gated custom skill queues directly in Attack Surface Chat.

Custom runtime execution is not wired in v3.15.0. User-defined skills are plannable, auditable, target-profile-aware, and visible in the operator workflow, while runtime execution remains dispatcher-gated until later authorized runtime patches add explicit execution backends, permission modes, budgets, stop conditions, policy enforcement, and audit controls.

See docs/release-notes/v3.15.0.md for details. The official smoke test for this milestone is:

    ./scripts/smoke_operator_user_skills_v315.sh

## v3.14.0 — Pentest Skill Registry + Operator Skill Runtime Foundation

v3.14.0 adds the first modular Operator Skill foundation for Hunt Engine AI-driven authorized pentest workflow. The Operator can now select skills from evidence signals, create planned skill runs from chat, dispatch implemented skill runtimes, create observations, persist target memory, enrich skill run APIs, and show executed/queued skill status directly in chat.

Implemented v3.14 skill runtimes include parameter inventory, HTTP evidence analysis, authenticated-context requirements, JS/API inventory audit, XSS reflection planning, open redirect planning, and path traversal/file-read planning.

Planning runtimes in v3.14.0 do not execute payloads or claim impact. They prepare evidence-driven candidates and memory for future controlled validation and authorized exploitation workflows under scope, policy, rate-limit, audit, and approval controls.

See docs/release-notes/v3.14.0.md for details. The official smoke test for this milestone is:

    ./scripts/smoke_operator_skill_runtime_coverage_v314.sh

## v3.13.0 — Controlled Runtime Expansion + AI Pentest Reasoning Foundation

v3.13.0 adds the first evidence-driven AI Pentest Operator hunting foundation. The Operator can now select multiple high-value candidates, run controlled low-risk baseline probes through policy-aware Autopilot, analyze runtime evidence, persist analyzer learning into target memory, and avoid retesting blocked or low-value candidates.

This release is intentionally not full exploitation and not yet bug-class-specific validation. High-risk validation remains approval-gated, and blocked/challenged/inconclusive evidence is not treated as vulnerability proof.

See `docs/release-notes/v3.13.0.md` for details. The official smoke test for this milestone is:

```bash
TARGET_ID=5 ./scripts/smoke_operator_hunting_loop_v313.sh
```

## v3.12.0 - Controlled Runtime & Operator Autopilot Policy Controls

Hunt Engine v3.12.0 advances the RAG-backed LLM Pentest Operator from an advisory planner into a controlled, policy-gated execution workflow for supported low-risk actions.

### Highlights

- Added the Controlled Runtime foundation for approved operator actions.
- Added Controlled HTTP Probe execution for low-risk endpoint review workflows.
- Added structured evidence capture for request, response, matcher, metadata, and runtime classification.
- Added WAF / Cloudflare / 403 / 429 / 5xx handling as inconclusive evidence rather than confirmed vulnerability proof.
- Added Controlled Test Run, Result, and Event persistence.
- Connected controlled runtime results into target RAG memory.
- Updated the Operator to reason over controlled runtime evidence and avoid over-claiming blocked or inconclusive results.
- Added endpoint URL intent detection in Attack Surface Chat, mapping single-endpoint requests to review_endpoint actions.
- Added Operator Autopilot v1 for low-risk controlled endpoint review.
- Added Target Policy controls for Operator automation: manual_only, assisted_autopilot, strict_approval, auto_execute_level_0, auto_execute_level_1, require_approval_level_2, and require_approval_level_3.
- Added Target Policy UI controls for Operator Automation.
- Added Chat UI visibility for Autopilot execution results, controlled run IDs, result IDs, memory ingestion, and runtime status.
- Added action labels in Chat for Executed by Autopilot, Approval Required, and Proposed Action.
- Clarified dispatcher wording so unsupported action classes are shown as dispatcher previews, while supported controlled-runtime actions may execute after policy and approval checks.
- Added an official smoke script for Operator Autopilot policy behavior.

### Operator Autopilot Behavior

The Operator now supports policy-driven automation:

- assisted_autopilot: low-risk Level 0 / Level 1 controlled actions may execute automatically when target policy permits.
- strict_approval: actions are proposed but not executed automatically.
- manual_only: Autopilot is disabled for the target.
- Level 2 and Level 3 workflows remain approval-gated by default.

Autopilot is intentionally narrow in v3.12.0. It currently supports low-risk review_endpoint actions through the controlled runtime. Higher-risk workflows such as exploit validation, payload execution, authentication testing, rate-limit testing, brute-force workflows, and unsupported action classes remain approval-gated or preview-only.

### Smoke Test

Run the official Autopilot policy smoke test:

    ./scripts/smoke_operator_autopilot_policy.sh

The smoke test validates:

- assisted_autopilot executes a low-risk controlled endpoint review.
- strict_approval does not auto-execute the same workflow.
- blocked or inconclusive HTTP evidence is not treated as a confirmed vulnerability.
- the original target policy is restored after the test.

### Safety Model

v3.12.0 keeps deterministic guardrails authoritative:

- target policy controls execution eligibility.
- out-of-scope and blocked actions cannot execute.
- unsupported action classes remain dispatcher previews.
- controlled runtime evidence is stored and fed back into target memory.
- LLM output remains advisory and evidence-aware.
- high-risk actions require explicit approval and stronger validation evidence.

<!-- HUNT_V3_12_0_END -->
\n\n# Hunt Engine

Hunt Engine is a commercial-grade attack surface intelligence, reconnaissance, security finding, advisory analysis, and reporting platform. It is designed to help security teams and bug bounty hunters continuously discover assets, validate live exposure, collect evidence, prioritize findings, generate professional reports, and use policy-aware advisory agents without allowing uncontrolled AI execution or unsafe automated exploitation.

The platform combines deterministic scanner logic with structured evidence, account-scoped configuration, professional PDF reporting, and advisory agent workflows. Deterministic product logic remains authoritative for severity, risk scoring, finding validity, policy enforcement, and execution decisions. AI and advisory agents provide interpretation, triage, summaries, and report draft assistance only.

---

## Current release focus: v3.11.0

v3.11.0 introduces the LLM Pentest Operator MVP. It connects Attack Surface Chat to the account-scoped LLM provider, builds owner-scoped RAG/operator context from target memory, and produces context-grounded, approval-gated pentest action proposals.

The main strategic shift in this release is that the Analysis workspace is now moving from standalone analysis panels toward a professional Operator Workspace: RAG-backed target memory, LLM-driven Attack Surface Chat, OWASP-aware planning, approval-gated controlled testing, evidence feedback, finding labeling, and report drafting.

### Core v3.9.0 scope

- **Pattern Pack Foundation**
  - Added `bug_pattern_packs` metadata for versioned, trusted, source-aware pattern packs.
  - Pack metadata includes source, version, trust level, checksum, signature placeholder, enabled/locked state, update mode, pattern count, safety score, quality score, noise score, false-positive rate, and metadata JSON.
  - Core safe/passive patterns are linked to the seeded core Pattern Pack.

- **Payload Pack Foundation**
  - Added `bug_payload_packs` metadata for future payload intelligence and controlled operator workflows.
  - Payload packs are metadata-only in v3.9.0.
  - Payload execution remains disabled.
  - Core inert/safe payload metadata is linked to the seeded core Payload Pack.

- **Pattern/Payload Pack APIs**
  - Added read-only APIs for listing and inspecting Pattern Packs and Payload Packs.
  - Added enable/disable APIs for pack-level controls.
  - Pack control updates are owner-scoped and audit logged.

- **Pattern/Payload Pack UI**
  - Target Analysis Registry views now show Pattern Pack and Payload Pack summaries.
  - UI displays pack key, source, version, trust level, enabled/locked state, update mode, safety score, quality score, noise score, false-positive rate, rollback/update metadata, and pack counts.
  - UI supports enable/disable controls for pack-level management.

- **Pack-aware Safe Bug Testing**
  - The Safe Bug Testing runner now enforces enabled Pattern Packs.
  - Disabled Pattern Packs prevent linked patterns from being used by the runner.
  - Bug test evidence records pack-aware metadata such as pattern pack key and pack-aware filtering status.

- **Pack-aware Registry Inspection**
  - Agent Action registry inspection now includes Pattern Pack and Payload Pack intelligence.
  - Inspection output includes pack summaries, trust metadata, scoring, update-plan foundation metadata, and safety guardrails.
  - Inspection remains metadata-only and does not import feeds, execute payloads, or perform exploit validation.

- **RAG-backed LLM Pentest Operator Roadmap**
  - Added the new operator roadmap under `docs/roadmaps/`.
  - The roadmap deprecates standalone Analysis features that do not directly support RAG, LLM operator workflows, real OWASP-style testing, evidence feedback, finding promotion, and report drafting.
  - Future development prioritizes per-target/project RAG memory, LLM-powered Attack Surface Chat, approved real test execution, iterative pentest loops, and controlled operator runtime.


### v3.11.0 - LLM Pentest Operator MVP

v3.11.0 connects the RAG memory foundation to a real LLM-backed Pentest Operator. The operator uses target context and memory to produce grounded, approval-gated action plans while preserving deterministic guardrails, owner/account isolation, auditability, and safe-by-default execution controls.

Delivered in v3.11.0:

- Account-scoped LLM Operator Planning Foundation for Attack Surface Chat.
- RAG-backed operator context builder using target metadata, policy, assets, URLs, findings, bug test results, and memory.
- Context-grounded LLM responses that cite concrete target facts such as asset counts, live assets, URL inventory, findings, bug test results, policy limits, and representative evidence.
- Owner-scoped commercial isolation for Target Memory, Target Memory Chunks, Target Memory Events, and Agent Actions.
- Admin users share the `admin` owner scope while non-admin users use `user:<id>` owner scope.
- `user_id` / `created_by_user_id` remain actor/audit fields, while `owner_key` is the account/tenant boundary.
- LLM-generated action proposals are allowlist-filtered and always approval-gated.
- Unknown or dangerous action types from the LLM are dropped before action creation.
- Duplicate agent action handling was hardened so chat-created actions do not incorrectly reuse old actions from unrelated chat sessions.
- Attack Surface Chat UI now surfaces LLM Assisted/Fallback state, provider/model, operator mode, recommended next steps, proposed actions, guardrails, and fallback errors.
- Operator prompt wording now supports future controlled validation/exploitation through approved Agent Actions and controlled runtime execution, while blocking direct chat execution and uncontrolled payload execution.
- Small operator smoke/unit tests verify context fact extraction, grounded summaries, evidence_basis injection, action allowlisting, approval-gated behavior, and prompt guardrail wording.

v3.11.0 is still a planning/proposal MVP. It does not execute tests directly inside chat. Real validation/exploitation must go through target scope checks, target policy, approval gates, rate limits, audit logs, and controlled runtime support.

### v3.10.0 - RAG Memory Foundation

v3.10.0 introduces the target-scoped RAG memory foundation for the future LLM Pentest Operator. This release does not enable uncontrolled autonomous testing; it prepares the memory, retrieval, and interaction layer needed for professional, policy-aware operator planning.

Delivered in v3.10.0:

- Target Memory data layer with `target_memory_items`, `target_memory_chunks`, and `target_memory_events`.
- Target Memory APIs for listing, creating, reading, deleting, and chunking memory records.
- Target Memory Context API for compact, low-token operator context.
- Automatic Target Memory Ingestion from target overview, live asset summary, URL inventory, findings, and Safe Bug Testing results.
- Broad objective-based Operator Retrieval API for full vulnerability discovery, not limited to XSS or a small set of bug classes.
- Supported retrieval objectives include `all_bugs`, `vulnerability_discovery`, `xss`, `open_redirect`, `cors`, `security_headers`, `auth`, `access_control`, `idor`, `api`, `ssrf`, `sqli`, `nosqli`, `ssti`, `path_traversal`, `file_upload`, `csrf`, `clickjacking`, `takeover`, `secrets`, `cloud`, `cache_poisoning`, `rate_limit`, `session`, `business_logic`, `misconfiguration`, `cve`, `crawl_more`, and `report`.
- Agent Chat Memory Hooks for important user clarifications, approvals/rejections, policy constraints, vulnerability objectives, evidence interpretations, and recon/crawling decisions.
- Memory event audit trail for created, updated, chunked, ingested, and used memory events.
- Guardrail-first design: memory retrieval is target-scoped and owner-scoped; active or risky tests still require policy checks and approval.

v3.10.0 prepares the RAG and retrieval substrate for v3.11.0 LLM Pentest Operator MVP.

### v3.9.0 guardrail status

v3.9.0 is a foundation release. It intentionally does **not** enable uncontrolled execution.

Disabled by design in v3.9.0:

- Direct uncontrolled command execution.
- Payload execution without approval and policy checks.
- Automated exploit validation without explicit controls.
- Destructive testing.
- Out-of-scope testing.
- Data extraction workflows.
- Brute force or credential attacks.
- Automatic report submission.

Pattern and payload packs are metadata/control foundations. Real operator-driven testing will be introduced through the RAG-backed LLM Pentest Operator roadmap with target policy checks, scope validation, approval gates, safety classification, rate limits, audit logs, and evidence requirements.

### New strategic direction

Hunt Engine is being refocused toward a real, controlled, RAG-backed LLM Pentest Operator:

- `v3.10.0`: RAG Memory Foundation.
- `v3.11.0`: LLM Pentest Operator MVP. ✅
- `v3.12.0`: Approved Real OWASP Test Execution.
- `v4.0.0`: Iterative Pentest Loop v1.
- `v4.1.0`: Controlled Operator Runtime Expansion.
- `v4.2.0`: Operator-driven Findings and Report Drafting.
- `v4.3.0`: Automation and Memory-driven Learning.

Every future feature that affects the LLM Pentest Operator must include a small, low-token LLM smoke test in addition to normal backend/frontend tests.


## Guardrail principles

Hunt Engine is built around strict commercial-grade security product guardrails:

- Deterministic product logic remains authoritative for risk score, severity, priority, finding validity, policy enforcement, and execution decisions.
- AI/advisory agents provide interpretation, triage, summaries, report drafts, and manual validation guidance only.
- Coverage gaps are not treated as vulnerabilities.
- Reconnaissance-only signals do not become critical solely because they are numerous.
- No uncontrolled AI execution.
- No unsafe auto-exploitation.
- No out-of-scope testing.
- No destructive testing.
- Report Agent output must not be submitted automatically.
- Human validation is required before customer-facing or bug bounty claims.

---

## High-level capabilities

### Reconnaissance and asset discovery

- Passive and active subdomain discovery.
- Optional sources such as Subfinder, Assetfinder, crt.sh, Cero, AbuseDB, Amass, PureDNS, and AlterX.
- Large target support with streamed/controlled processing for massive candidate sets.
- Source attribution for discovered assets.
- Live DNS validation with DNSX.
- CDN/WAF/cloud enrichment.
- Optional port scanning for eligible live assets.

### Probing and HTTP intelligence

- HTTP probing and response metadata collection.
- Status code, title, final URL, content length, server, technologies, response timing, and raw metadata.
- Fresh asset detection and status change tracking.
- Screenshot-assisted notification flows where configured.

### Crawling and URL inventory

- URL collection from crawling and archive sources.
- JavaScript resource discovery.
- URL source attribution.
- JS-focused filtering and intelligence.

### Security findings and evidence

- Built-in deterministic findings.
- Nuclei security engine integration.
- Custom Nuclei template support.
- Subdomain takeover detection.
- JavaScript intelligence findings.
- Structured `evidence_json` for source-specific evidence display.
- Better Finding Evidence UI with source-aware structured evidence rendering.

### Reporting and analysis

- Professional target PDF reports.
- Deterministic commercial-grade target risk methodology.
- Local deterministic target analysis.
- Optional LLM-assisted narrative generation with deterministic guardrails.
- Deterministic target recommendations.
- v3.6.0 advisory agent outputs in PDF reports.
- v3.9.0 Pattern/Payload Pack Foundation, pack-aware Safe Bug Testing, registry inspection intelligence, and the RAG-backed LLM Pentest Operator roadmap.

### Account-scoped configuration

- Per-user Telegram notification settings.
- Per-user LLM provider settings.
- Account-scoped feature flags.
- Account custom PureDNS wordlists.
- User-specific quotas and concurrency controls.

---

## Architecture overview

Hunt Engine is deployed as a Docker Compose application with these primary services:

- **frontend**: React/Vite web UI.
- **backend**: Go/Fiber API and worker runtime.
- **postgres**: persistent application database.
- **redis**: queue, cache, and worker coordination.
- **nginx**: reverse proxy and production frontend/API routing.
- **certbot**: certificate automation where configured.
- **dns**: optional DNS service integration.

Typical data flow:

1. User creates or edits a target.
2. Target scan modules are queued in Redis.
3. Worker executes selected phases in order: Discovery, Probing, Crawling, and security engines.
4. Results are persisted to PostgreSQL with structured evidence.
5. UI displays assets, URLs, findings, analysis, policies, recommendations, and agent outputs.
6. Reports are generated from stored database state.

---

## Scan phases

### Discovery

Discovery collects candidate subdomains and validates live DNS results.

Supported/related components:

- Subfinder
- Assetfinder
- crt.sh
- Cero
- AbuseDB
- Amass
- PureDNS
- AlterX
- DNSX
- CDNCheck
- Optional Nmap/port scan

Important v3.6.0 behavior:

- PureDNS selected wordlists are executed sequentially.
- PureDNS persists only resolved/live results.
- AlterX candidate streams are validated before persistence.
- Unresolved AlterX candidates are not stored as dead assets.

### Probing

Probing enriches live assets with HTTP metadata and runs post-probing security checks.

Post-probing checks include:

- Built-in findings.
- Takeover detection.
- Nuclei security engine.

### Crawling

Crawling discovers and stores URLs and JavaScript resources.

Post-crawling checks include:

- JS Intelligence.
- JavaScript endpoint/source map/secret style signals where evidence supports them.

---

## Advisory agents

v3.6.0 introduces advisory agents as a policy-aware interpretation layer on top of stored Hunt Engine data.

### Agent types

- `triage`
- `summary`
- `report`
- `policy_review` reserved for future expansion

### Agent run storage

Agent executions are stored in `agent_runs`.

Important fields:

- `target_id`
- `created_by_user_id`
- `agent_type`
- `provider`
- `model`
- `status`
- `source`
- `policy_status`
- `input_digest`
- `input_json`
- `output_json`
- `error_message`
- `started_at`
- `completed_at`

### Triage Agent

Output includes:

- top interesting findings
- manual validation steps
- false-positive risk
- bug bounty value
- policy safety notes
- recommended manual tests

### Summary Agent

Output includes:

- attack surface summary
- most interesting assets
- coverage summary
- risk narrative
- what to test next
- policy safety notes
- assumptions
- limitations

### Report Agent

Output includes:

- report candidate
- impact hypothesis
- evidence needed
- validation checklist
- platform-safe wording
- suggested fix
- related findings/assets
- `human_validation_required=true`
- `do_not_submit_automatically=true`

---

## Target policy

Target policies provide the context required for safe advisory workflows and future human-approved automation.

Policy fields include:

- `platform_name`
- `program_url`
- `in_scope_patterns`
- `out_of_scope_patterns`
- `allowed_test_types`
- `disallowed_test_types`
- `max_test_intensity`
- `rate_limit_notes`
- `auth_required`
- `safe_testing_notes`
- `reporting_preferences`
- `business_context`
- `asset_criticality_default`

Policy does not automatically authorize unsafe actions. It gives the product and advisory agents context for safe, bounded recommendations.

---

## Environment variables

Create a `.env` from `.env.example` and set required values.

Important variables:

```env
# Required secret. Use a strong value in production.
JWT_SECRET=change-me-to-a-long-random-secret

# Backend max request body size in MB. Useful for large custom wordlist uploads.
HUNT_MAX_REQUEST_BODY_MB=5120

# Persistent host path for uploaded account PureDNS wordlists.
# Mounted into backend as /data/wordlists.
HUNT_WORDLISTS_DIR=./hunt-wordlists

# Optional worker/tool tuning.
TRUSTED_RESOLVERS=1.1.1.1,8.8.8.8,1.0.0.1,8.8.4.4
```

Database, Redis, domain, DNS, and production TLS values should also be configured in `.env` according to the deployment environment.

---

## Persistent volumes and mounts

The backend service should include persistent mounts for custom templates, wordlists, worker workspaces, and Docker socket access where required by the toolchain.

Recommended backend volume configuration:

```yaml
volumes:
  - ${HUNT_WORDLISTS_DIR:-./hunt-wordlists}:/data/wordlists
  - ./custom_wordlists:/wordlists/custom
  - ./custom_nuclei_templates:/data/nuclei/custom
  - hunt_workspaces:/tmp/hunt-engine
  - /var/run/docker.sock:/var/run/docker.sock
```

Production verification:

```bash
docker inspect hunt-backend --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'
```

Expected wordlist mount:

```text
/root/hunt-engine/hunt-wordlists -> /data/wordlists
```

---

## Quick start

### 1. Clone

```bash
git clone https://github.com/omidxplimbo/hunt-engine.git
cd hunt-engine
```

### 2. Configure environment

```bash
cp .env.example .env
nano .env
```

At minimum, set a strong `JWT_SECRET` and confirm persistent paths such as `HUNT_WORDLISTS_DIR`.

### 3. Start services

```bash
docker compose up -d --build
```

### 4. Check service status

```bash
docker compose ps
```

### 5. View logs

```bash
docker compose logs -f backend
```

---

## Production deployment checklist

Before production deployment:

- Configure `.env` securely.
- Confirm `JWT_SECRET` is strong and not committed.
- Confirm PostgreSQL and Redis persistence.
- Confirm `HUNT_WORDLISTS_DIR` is mounted to `/data/wordlists`.
- Confirm Nginx upload limits support expected wordlist size.
- Confirm account feature flags for production users.
- Confirm AI/advisory features are enabled only where intended.
- Confirm target policies are configured before active or sensitive workflows.

Recommended production update flow:

```bash
git checkout main
git pull origin main
docker compose build backend frontend
docker compose up -d --force-recreate backend frontend nginx
```

For frontend-only UI changes:

```bash
docker compose build frontend
docker compose up -d --force-recreate frontend nginx
```

---

## Development workflow

Typical v3.7.0 development branch workflow:

```bash
git checkout v3.7.0
git pull origin v3.7.0
```

Frontend build:

```bash
npm --prefix frontend run build
```

Backend tests:

```bash
cd backend
JWT_SECRET="hunt-engine-v36-test-jwt-secret-minimum-32-chars" go test ./...
cd ..
```

Local/test Docker Compose project example:

```bash
docker compose \
  --env-file .env.v35 \
  -p hunt35 \
  -f docker-compose.yml \
  -f docker-compose.hunt35.override.yml \
  build backend frontend


docker compose \
  --env-file .env.v35 \
  -p hunt35 \
  -f docker-compose.yml \
  -f docker-compose.hunt35.override.yml \
  up -d --force-recreate backend frontend nginx
```

---

## Validation checklist for v3.6.0

### Backend and frontend

```bash
npm --prefix frontend run build

cd backend
JWT_SECRET="hunt-engine-v36-test-jwt-secret-minimum-32-chars" go test ./...
cd ..
```

### Agent workflow validation

- Open Target -> Analysis.
- Confirm Advisory Agents panel is visible.
- Run Summary.
- Run Triage.
- Run Report.
- Confirm rows are stored in `agent_runs`.
- Confirm output is rendered in UI.
- Confirm guardrail wording is visible.

Example database check:

```bash
docker compose exec -T postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
SELECT id, target_id, agent_type, provider, model, status, policy_status, created_at
FROM agent_runs
ORDER BY id DESC
LIMIT 10;
"'
```

### PDF report validation

- Generate a target analysis.
- Run Summary/Triage/Report agents.
- Download target PDF.
- Confirm `Advisory Agent Outputs` appears after Target Analysis.
- Confirm Summary Agent, Triage Agent, and Report Agent Draft content appears when runs exist.

### Wordlist persistence validation

```bash
docker inspect hunt-backend --format '{{range .Mounts}}{{println .Source "->" .Destination}}{{end}}'
```

Confirm:

```text
/root/hunt-engine/hunt-wordlists -> /data/wordlists
```

Write test:

```bash
docker compose exec -T backend sh -lc '
mkdir -p /data/wordlists/users/999
echo test > /data/wordlists/users/999/persist-test.txt
ls -lah /data/wordlists/users/999/persist-test.txt
'

ls -lah ~/hunt-engine/hunt-wordlists/users/999/persist-test.txt
```

### PureDNS validation

- Upload one or more custom `.txt` wordlists from Account.
- Confirm upload progress appears for local files.
- Confirm uploaded files exist under host `hunt-wordlists`.
- Select custom and default wordlists on a target.
- Fresh Restart the target.
- Confirm PureDNS executes all selected wordlists sequentially.
- Confirm only resolved/live PureDNS results are persisted.

### AlterX validation

- Run a target with AlterX enabled.
- Confirm large candidate streams do not become huge dead-asset database inserts.
- Confirm live/resolved AlterX candidates retain `alterx` source attribution.

### Fresh Restart validation

- Stop or interrupt a target scan.
- Use Fresh Restart from the target controls.
- Confirm scan checkpoints are reset.
- Confirm queued jobs for the target are cleaned.
- Confirm assets/findings remain preserved.
- Confirm scan restarts from the first configured module.

---

## Feature flags

Important feature flags include:

```text
feature.target_policy
feature.agent_runs
feature.ai_triage_agent
feature.ai_summary_agent
feature.ai_report_agent
feature.target_pdf_report
feature.ai_analysis
feature.llm_assisted_analysis
feature.ai_recommendations
feature.ai_nuclei_template_drafts
```

Feature flags support account-scoped effective resolution. Admin/shared and non-admin user behavior should follow the account owner resolution implemented in the backend.

---

## Account custom PureDNS wordlists

Custom wordlists are uploaded from Account and become available in target create/edit forms.

Supported upload modes:

- Local `.txt` file upload.
- URL import from public safe URLs.

Notes:

- Local file upload shows browser upload progress.
- URL import is downloaded server-side and therefore does not expose browser upload progress.
- SSRF guardrails should block private or unsafe URL targets.
- Persistent storage must be mounted correctly before relying on uploaded wordlists in production.

---

## Nuclei templates

Hunt Engine supports custom Nuclei template workflows with guardrails:

- Templates are stored in the database.
- Templates are synced to user-specific custom template paths.
- Custom templates can be run without builtin tag filtering where appropriate.
- AI-generated Nuclei template drafts are draft-only and human-approval gated.
- No auto-save or auto-execute is allowed for AI-generated templates.

---

## Telegram notifications

Telegram notification settings are account scoped:

- Non-admin users have per-user Telegram config.
- Admin users can share an admin owner scope.
- Bot token and chat ID are stored in the database.
- Bot token is never returned to the frontend.
- Blank token update preserves the existing token.
- Redis caching is used for notification config resolution and invalidated on update.

---

## LLM provider settings

LLM providers are account scoped:

- Multiple providers/models can be configured.
- API keys are stored in the database but never returned to the frontend.
- Blank API key update preserves the existing key.
- Clear-key behavior is supported.
- Real LLM-assisted features use deterministic guardrails as source of truth.

LLM-assisted output may provide narrative and interpretation, but must not override deterministic risk, severity, finding validity, priority, or policy enforcement.

---

## PDF reports

Target PDF reports include:

- Target metadata.
- Executive summary.
- Metrics grid.
- Target analysis.
- Advisory Agent Outputs.
- Scan state.
- Asset and URL inventory.
- Findings summary.
- Source-specific finding tables.
- Recent assets and URLs.

v3.6.0 PDF reports include advisory outputs from the latest completed Summary, Triage, and Report agent runs.

---

## Release history

### v3.9.0

Pattern/Payload Pack Foundation and RAG-backed LLM Pentest Operator roadmap.

- Added Pattern Pack and Payload Pack data models.
- Added seeded core Pattern/Payload Packs with trust, source, version, scoring, and metadata.
- Added Pattern/Payload Pack APIs and UI summaries.
- Added enable/disable controls and audit logs for packs.
- Enforced enabled Pattern Packs in the Safe Bug Testing runner.
- Added pack-aware bug test evidence metadata.
- Added pack intelligence to registry inspection agent actions.
- Added the new RAG-backed LLM Pentest Operator roadmap.
- Refocused future Analysis work toward Operator Workspace, per-target RAG memory, LLM chat, real OWASP-style testing, iterative evidence feedback, finding labels, finding promotion, and report drafting.


### v3.7.0

Human-approved attack-surface workflow foundation:

- Agent Actions Data Layer and UI.
- Approval and rejection workflow for proposed actions.
- Audit logs for propose, approve, reject, duplicate proposal, dispatch, and cleanup operations.
- Attack Surface Chat backend and frontend UI.
- Target-scoped chat sessions and messages.
- Deterministic chat-to-action planner.
- Chat-to-proposed-agent-actions flow.
- Duplicate action proposal prevention.
- Agent Action Dispatcher Foundation.
- Dispatch Preview UI and dispatcher output details.
- `agent-action-dispatcher-v2` guardrail output.
- Policy Checker v2 and autonomy controls foundation.
- Action class classification, policy tokens, blocked reasons, warning reasons, required controls, and max-test-intensity enforcement.
- Target Analysis single-active-section workspace.
- Delete/cleanup controls for AI analyses, AI recommendations, advisory agent runs, agent actions, and chat sessions/messages.
- LLM provider secret-cache fix.
- Real command execution, payload execution, scan execution from actions, severity auto-apply, and report auto-submit remain disabled.

### v3.6.0

Policy-aware advisory agents and operational scale improvements:

- Target Policy Foundation.
- Agent Runs Data Layer.
- Deterministic Triage Agent.
- Deterministic Summary Agent.
- Deterministic Report Agent.
- Advisory Agents UI in Target Analysis.
- Advisory Agent Outputs in PDF reports.
- Account custom PureDNS wordlists.
- Persistent wordlist storage.
- Large upload support.
- Drag-and-drop wordlist uploader with progress.
- PureDNS sequential wordlist execution.
- PureDNS resolved-only persistence.
- AlterX live-only persistence.
- Target Fresh Restart.

### v3.5.0

AI-ready reporting and account-scoped configuration:

- Target PDF Reporting.
- AI-ready data layer.
- `ai_analyses`.
- LLM-assisted target analysis narrative.
- `ai_recommendations`.
- `audit_logs`.
- Global and account-scoped feature flags.
- Account Feature Access UI.
- Account-scoped LLM provider settings.
- Deterministic commercial-grade target risk methodology.
- Hardened AI-generated Nuclei template draft workflow.

### v3.4.0

Structured evidence and security intelligence foundation:

- Takeover Detection.
- JS Intelligence.
- More Evidence / `evidence_json`.
- Better Finding Evidence UI.
- Custom Nuclei template execution and ingestion fixes.
- Telegram notification settings moved from System Config to Account.

---

## Roadmap direction

Hunt Engine is evolving toward a commercial-grade AI-assisted Attack Surface Intelligence and Bug Bounty Automation Platform.

Planned future directions:

- Human-approved recon workflows.
- Policy-aware agent action approval.
- Safe Bug Testing Engine.
- XSS, open redirect, CORS, and security header candidate testing with strict safety levels.
- Pattern intelligence and update system.
- Learning engine from user feedback and test outcomes.
- Manual hunting workspace.
- Bug bounty report builder.
- Controlled automation and scheduling.

Future work must keep deterministic guardrails authoritative and keep risky execution human-approved, audited, policy-aware, and rate-limited.

---

## Safety statement

Use Hunt Engine only on assets and programs where you have authorization. Configure target policy and scope before running active testing. Advisory agents and generated report drafts are not proof of vulnerability and must be manually validated before use in customer-facing or bug bounty submissions.



### Next: v3.12.0 - Approved Real OWASP Test Execution

The next release should connect approved Operator actions to controlled real testing primitives:

- Approval-gated safe HTTP/probe runtime.
- Policy/scope/test-level checks before execution.
- Controlled OWASP-style validation for safe bug classes.
- Evidence capture for each test attempt.
- RAG memory updates after test results, approvals, rejections, failed hypotheses, and useful findings.
- Continued separation between chat planning, approval workflow, and controlled runtime execution.

## Releases

### v3.15.1 - Bug-Class Validation Runtimes

v3.15.1 adds controlled, evidence-producing Level 2 validation runtimes for the v3.15 bug-class validation skills: XSS reflection context, DOM XSS source/sink evidence, CRLF/header marker checks, cache behavior evidence, open redirect chain behavior, safe path traversal baseline checks, and CORS/clickjacking/CSRF baseline header review.

The release includes `scripts/smoke_operator_bug_class_validation_v3151.sh` and release notes at `docs/release-notes/v3.15.1.md`.

