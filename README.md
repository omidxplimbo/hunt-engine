# Hunt Engine

Hunt Engine is a commercial-grade attack surface intelligence, reconnaissance, security finding, advisory analysis, and reporting platform. It is designed to help security teams and bug bounty hunters continuously discover assets, validate live exposure, collect evidence, prioritize findings, generate professional reports, and use policy-aware advisory agents without allowing uncontrolled AI execution or unsafe automated exploitation.

The platform combines deterministic scanner logic with structured evidence, account-scoped configuration, professional PDF reporting, and advisory agent workflows. Deterministic product logic remains authoritative for severity, risk scoring, finding validity, policy enforcement, and execution decisions. AI and advisory agents provide interpretation, triage, summaries, and report draft assistance only.

---

## Current release focus: v3.7.0

v3.7.0 extends Hunt Engine from policy-aware advisory agents into a human-approved attack-surface workflow layer. It introduces target-scoped Agent Actions, an Attack Surface Chat interface, deterministic chat-to-action planning, policy/autonomy guardrails, dispatch previews, duplicate proposal prevention, and cleanup controls for generated analysis data.

### Core v3.7.0 scope

- **Human-approved Agent Actions**
  - Target-scoped `agent_actions` records for proposed, approved, rejected, blocked, failed, and future executed workflow actions.
  - Supported action types include OWASP checklist planning, crawling, Nuclei profile runs, JS intelligence, safe bug-test planning, endpoint review, payload planning, severity review, report generation, and report submission planning.
  - Actions remain policy-aware and approval-gated.
  - Real command execution, payload execution, scan execution from actions, severity auto-apply, and report auto-submit remain disabled in the v3.7.0 foundation.

- **Agent Action Approval Workflow**
  - Users can approve or reject proposed actions from Target Analysis.
  - Human decisions are stored in `agent_action_approvals`.
  - Approval/rejection activity is audit logged.
  - Blocked-by-policy actions cannot be approved.

- **Attack Surface Chat**
  - Target Analysis includes an Attack Surface Chat workspace.
  - Users can write natural-language requests such as OWASP coverage planning, safe XSS/CORS/open redirect checks, JS review, report preparation, or severity review.
  - The chat agent uses existing target context and deterministically converts intent into proposed agent actions.
  - Chat creates action plans only; it does not execute commands, payloads, scans, or submissions.

- **Chat Sessions and Messages**
  - Persistent `agent_chat_sessions` and `agent_chat_messages` tables.
  - Sessions are target-scoped and user-owned.
  - Assistant responses can include action-plan metadata and proposed action IDs.
  - Chat sessions/messages can be soft-deleted from the UI.

- **Agent Action Dispatcher Foundation**
  - Approved actions can be sent to Dispatch Preview.
  - Dispatcher preview records output JSON, handler name, action class, guardrails, required controls, hard-block reasons, and execution-disabled status.
  - `agent-action-dispatcher-v2` does not execute commands, payloads, scans, severity changes, or report submissions.
  - Dispatcher actions are audit logged with `target.agent_action.dispatch`.

- **Policy Checker v2 and Autonomy Controls Foundation**
  - `policy_check_json` now uses `agent-actions-policy-v2`.
  - Action classes are classified into advisory, passive recon, template scan, safe bug test, deep scan, command execution, payload generation, payload execution, finding validation, severity apply, and report submission.
  - Policy checker emits policy tokens, blocked reasons, warning reasons, required controls, autonomy controls, max-test-intensity enforcement, allowed/disallowed test type handling, auth warnings, and rate-limit warnings.
  - High/critical risk, level-3 exploit validation, payload execution, report submission, and dangerous future action classes are blocked by default.

- **Duplicate Action Proposal Prevention**
  - Similar active actions are not repeatedly created for the same target.
  - Proposed, approved, and blocked-by-policy actions are considered active duplicates.
  - Duplicate proposal attempts are audit logged with `target.agent_action.propose_duplicate`.

- **Target Analysis Workspace Reorganization**
  - The Target Analysis tab now uses a single-active-section workspace.
  - Sections include AI Analysis, Recommendations, Advisory Agents, Agent Actions, and Attack Surface Chat.
  - Only one section is rendered at a time, reducing page length, clutter, and unnecessary frontend workload.

- **Analysis Workspace Cleanup Controls**
  - Users can soft-delete generated AI analyses, AI recommendations, advisory agent runs, agent actions, and chat sessions/messages.
  - Backend deletion is owner-scoped, target-scoped, and audit logged.
  - Executed agent actions are protected from UI deletion to preserve security/audit trail integrity.
  - Audit actions include `target.ai_analysis.delete`, `target.ai_recommendation.delete`, `target.agent_run.delete`, `target.agent_action.delete`, and `target.agent_chat.session.delete`.

- **LLM Provider Secret Cache Fix**
  - LLM provider configs containing API keys are no longer cached in Redis in a way that strips secrets through JSON serialization.
  - This prevents false fallback errors such as "LLM provider has no API key saved" after a key is correctly stored.
  - Deterministic fallback behavior remains safe when provider calls fail.

### v3.7.0 guardrail status

v3.7.0 is a foundation release for human-approved workflow orchestration. It intentionally does **not** enable real autonomous execution.

Disabled by design in v3.7.0:

- Direct command execution.
- Payload execution.
- Automated exploit validation.
- Scan execution from agent actions.
- Automatic severity changes.
- Automatic report submission.
- Out-of-scope or destructive testing.
- Uncontrolled AI-driven hacking workflows.

Future versions may enable selected execution paths only behind explicit account controls, target policy checks, feature flags, human approval, rate limits, audit logs, and evidence requirements.


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
- v3.7.0 human-approved Agent Actions, Attack Surface Chat, policy/autonomy checks, dispatcher previews, and cleanup controls.

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

