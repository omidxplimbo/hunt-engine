# Hunt Engine

Continuous reconnaissance, asset monitoring, crawling, and security-finding platform for bug bounty hunters, security researchers, and red teams.

Current release branch: **v3.5.0**

---

## Overview

Hunt Engine is designed as a continuous hunting machine, not a one-shot scanner. It discovers assets, monitors changes, crawls URLs and JavaScript, runs security checks, stores structured evidence, and sends user-scoped Telegram notifications.

Core capabilities:

- Multi-source subdomain discovery
- Live probing and asset fingerprinting
- Asset diffing and change tracking
- URL crawling and archival discovery
- Built-in findings
- Nuclei scanning with custom template support
- Subdomain takeover candidate detection
- JavaScript intelligence
- Structured finding evidence with `evidence_json`
- Per-user queues and scan limits
- Per-account Telegram notifications
- Fresh asset screenshot upload and cleanup

- Target PDF reporting
- AI-ready analysis and recommendation data layer
- Commercial-grade deterministic target analysis
- Optional LLM-assisted report narrative
- Evidence-based target recommendations
- Audit logs for report, AI, recommendation, and Nuclei draft actions
- Global and account-scoped feature flags
- AI-generated Nuclei template draft workflow with human approval guardrails

---

## v3.5.0 Highlights

### Target PDF Reporting

Target-level PDF reporting is now available.

- Generates a professional downloadable PDF report per target.
- Includes target metadata, scan state, assets, URLs, findings, evidence summaries, severity/source breakdowns, and analysis output.
- Uses existing stored data and does not require an LLM token.
- PDF report downloads are audit logged.
- Controlled by feature flag `feature.target_pdf_report`.

### AI-Ready Data Layer

v3.5.0 adds the storage foundation for AI-assisted security workflows.

New data entities include:

- `ai_analyses`
- `ai_recommendations`
- `audit_logs`

These tables provide normalized storage for target analysis, recommendations, and operator/security-relevant actions.

### Commercial-Grade Target Analysis

Target analysis now uses deterministic commercial guardrails.

- Generates target-scoped analysis rows with `provider = local`.
- Uses `model = deterministic-target-v1`.
- Produces evidence-based `risk_score`, `risk_level`, `confidence_score`, `coverage_score`, `exposure_score`, and `finding_quality_score`.
- Separates true security risk from coverage gaps.
- Prevents reconnaissance-only or coverage-gap signals from inflating critical risk.
- Keeps deterministic logic authoritative for scoring, severity, prioritization, and finding validity.

### Optional LLM-Assisted Narrative

LLM-assisted target analysis can add narrative content when a valid account-scoped provider is configured.

- Deterministic risk scoring remains authoritative.
- LLM output is limited to narrative fields such as executive summary, customer summary, remediation plan, report notes, and validation notes.
- Missing, invalid, or unsupported provider settings safely fall back to deterministic output.
- Supports account-scoped LLM provider configuration.
- API keys are stored in the database and never returned to the frontend.
- Provider config is cached in Redis and invalidated on update/delete.

### Evidence-Based Recommendations

Target recommendations are generated from stored findings, evidence, latest target analysis, and coverage gaps.

- Generates deterministic, actionable recommendations.
- Preserves deterministic guardrails.
- Does not treat coverage gaps as vulnerabilities.
- Aggregates repetitive signals such as exposed login/admin interface indicators.
- Preserves user-decided recommendations during regeneration.
- Hard-deletes regenerated open system recommendations to avoid soft-deleted duplicates.
- Controlled by feature flag `feature.ai_recommendations`.

### Audit Logs

Audit logging now records important operator and system actions.

Covered v3.5.0 actions include:

- `target.report_pdf.download`
- `target.ai_analysis.generate`
- `target.ai_recommendations.generate`
- `nuclei.template_strategy.generate`
- `nuclei.template_draft.generate`

### Feature Flags

Feature flags are available globally and per account.

Global flags are managed through system configuration. Account-level feature access supports:

- `inherit`
- `enabled`
- `disabled`

Supported feature flags:

- `feature.target_pdf_report`
- `feature.ai_analysis`
- `feature.llm_assisted_analysis`
- `feature.ai_recommendations`
- `feature.ai_nuclei_template_drafts`

Admins share the `admin` owner scope. Non-admin users use their own `user:{id}` owner scope.

### AI-Generated Nuclei Template Draft Workflow

The Nuclei AI draft workflow is hardened for production safety.

- Disabled by default.
- Requires account feature flag `feature.ai_nuclei_template_drafts`.
- Requires environment kill switch `NUCLEI_ALLOW_AI_TEMPLATES=true`.
- Draft-only behavior.
- No auto-save.
- No auto-execute.
- Requires human review, validation, and manual save.
- Strategy and draft actions are audit logged.
- API and UI expose disabled reasons.
- Strategy output remains agent-ready while respecting deterministic safety guardrails.

---

## v3.4.0 Highlights

### Takeover Detection

Post-probing takeover detection is now included.

- Runs after full probing completion.
- Creates findings with `source_tool = takeover`.
- Uses `category = subdomain-takeover`.
- Stores structured evidence in `evidence_json`.
- Evidence includes provider, CNAME, confidence, matched signals, asset, and final URL.
- Matcher logic requires provider-compatible CNAME evidence before generic status/title signals are used.
- Azure Front Door and Azure Traffic Manager are handled separately.

### JavaScript Intelligence

Post-crawling JavaScript intelligence is now included.

- Runs after crawling completion.
- Analyzes JavaScript URLs discovered from crawled URLs.
- Falls back to extracting same-root-domain script URLs from live asset homepages.
- Filters out third-party JavaScript URLs.
- Creates findings with `source_tool = js-intel`.
- Supports categories such as `js-endpoints`, `js-source-map`, and `js-secret`.
- Stores structured metadata such as JS URL, signal type, endpoint count, status code, and content type.

### Structured Evidence

Findings now support structured evidence through the `evidence_json` JSONB field.

Covered finding sources:

- `builtin`
- `nuclei`
- `takeover`
- `js-intel`

This improves inspection, export, UI rendering, and future AI processing.

### Better Finding Evidence UI

The Findings panel now renders source-aware structured evidence.

UI improvements:

- Important evidence fields
- Additional evidence fields
- Legacy evidence text
- Raw JSON view
- Copy buttons for JSON/text
- Clipboard fallback for non-secure contexts

### Custom Nuclei Templates

Custom Nuclei template execution and ingestion were fixed and validated.

- Templates are stored in the database.
- Templates are synced into `/data/nuclei/custom/users/{user_id}/{placement}`.
- Custom templates run without built-in profile tag filtering.
- Built-in and custom outputs are merged before ingestion.
- Nuclei findings include structured `evidence_json`.

### Per-Account Telegram Notifications

Telegram settings moved from global System Config to Account settings.

Behavior:

- Non-admin users have their own Telegram settings.
- Admin users share one `admin` Telegram config.
- Bot token and chat ID are stored in the database.
- Telegram credentials are no longer read from `.env`.
- Bot token is never returned to the frontend.
- Leaving the token field blank preserves the existing saved token.
- Notification routing resolves the target owner and uses that owner's Telegram config.
- Telegram config resolution is cached in Redis with TTL.
- Updating Telegram settings invalidates the relevant Redis cache.

Configurable events:

- `fresh_asset`
- `fresh_url`
- `asset_change_is_live`
- `asset_change_status_code`
- `asset_change_title`
- `asset_change_web_server`
- `asset_change_technologies`
- `asset_change_host_ip`

### Fresh Asset Screenshots

When enabled, fresh live asset notifications can include a homepage screenshot.

Flow:

1. Capture the asset homepage.
2. Save temporarily under the user/target scoped screenshot path.
3. Upload to Telegram.
4. Delete the temporary screenshot from the server.

Temporary path pattern:

```text
/data/screenshots/users/{user_id}/targets/{target_id}/fresh-assets/
```

---

## Architecture

### Backend

- Go
- Fiber
- PostgreSQL
- GORM
- Redis
- JWT authentication
- RBAC and target ownership isolation
- Dockerized worker/API runtime

### Frontend

- React
- Vite
- TypeScript
- Tailwind CSS
- TanStack Query

### Runtime Services

- Backend API
- Frontend dashboard
- PostgreSQL
- Redis
- Nginx reverse proxy
- Optional DNS/certbot services

---

## Access Control

### Admin

- Can view all targets.
- Can manage users.
- Can see target owner metadata.
- Uses shared `admin` Telegram notification config.
- Has unlimited personal scan slots.

### Viewer

- Can only access owned targets.
- Can manage own account.
- Can configure own Telegram notifications.
- Can manage own scan queue.
- Uses per-user Telegram notification config.

Targets are scoped by `created_by_user_id`.

Queues are isolated per user:

```text
discovery_tasks:user:{user_id}
```

---

## Recon Pipeline

### Discovery

Integrated discovery and enrichment tools include:

- `subfinder`
- `assetfinder`
- `cero`
- `crtsh`
- `puredns`
- `amass`
- `alterx`
- `dnsx`
- `cdncheck`
- optional `nmap`

### Probing

Probing uses `httpx` to collect:

- status code
- final URL
- title
- web server
- technologies
- IPs
- CDN/WAF metadata
- raw HTTPX data

Post-probing security steps:

- built-in findings
- takeover detection
- Nuclei security scan

### Crawling

Crawling sources include:

- `gau`
- `waybackurls`
- `katana`
- `waymore`
- VirusTotal URL discovery

Post-crawling security step:

- JavaScript intelligence

---

## Findings

Findings can come from:

- `builtin`
- `nuclei`
- `takeover`
- `js-intel`

Important fields:

- `severity`
- `category`
- `source_tool`
- `evidence`
- `evidence_json`
- `recommendation`
- `status`
- `first_seen`
- `last_seen`
- `fingerprint`

---

## Nuclei Engine

Supported profiles include:

- `safe`
- `fast`
- `balanced`
- `misconfig`
- `full`

Custom templates are stored in DB and synced into:

```text
/data/nuclei/custom/users/{user_id}/{placement}
```

---

## Account Settings

The Account page includes:

- profile details
- password change
- personal scan queue control
- Subfinder provider keys
- Telegram notification settings

Telegram settings are configured from:

```text
Account -> Telegram Notifications
```

---

## Environment Configuration

Copy the example file:

```bash
cp .env.example .env
```

Required values:

```env
DB_PASSWORD=change_me_to_a_strong_database_password
JWT_SECRET=change_me_to_a_random_64_hex_secret
```

Telegram credentials are no longer configured in `.env`. Configure them from the Account page.

Optional integration:

```env
VIRUSTOTAL_API_KEY=
```

Nuclei tuning:

```env
NUCLEI_TIMEOUT_SECONDS=1800
NUCLEI_RATE_LIMIT=50
NUCLEI_CONCURRENCY=10
NUCLEI_BULK_SIZE=25
NUCLEI_DEFAULT_PROFILE=safe
HUNT_NUCLEI_TIMEOUT=10m
```

Worker tuning:

```env
PROBE_BATCH_SIZE=1000
DNSX_BATCH_SIZE=2000
DNSX_THREADS=30
AMASS_TIMEOUT_SECONDS=900
TRUSTED_RESOLVERS=1.1.1.1,8.8.8.8,1.0.0.1,8.8.4.4
```

---

## Quick Start

```bash
git clone https://github.com/omidxplimbo/hunt-engine.git
cd hunt-engine

cp .env.example .env
docker compose up -d --build
```

Default credentials:

```text
username: admin
password: admin123
```

Change the default password immediately after first login.

---

## Useful Commands

Backend tests:

```bash
cd backend
JWT_SECRET="hunt-engine-test-jwt-secret-minimum-32-chars" go test ./...
```

Frontend build:

```bash
cd frontend
npm install
npm run build
```

Rebuild containers:

```bash
docker compose up -d --build
```

Backend logs:

```bash
docker compose logs backend -f
```

Check scan state:

```bash
docker compose exec -T postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
SELECT target_id, status, current_module, current_step, completed_steps, last_error, updated_at
FROM target_scan_states
ORDER BY updated_at DESC
LIMIT 10;
"'
```

Check Telegram configs:

```bash
docker compose exec -T postgres sh -lc 'psql -U "$POSTGRES_USER" -d "$POSTGRES_DB" -c "
SELECT id, owner_key, user_id, enabled, chat_id, enabled_events,
       fresh_asset_screenshot_enabled,
       bot_token <> '\''\''' AS has_token
FROM user_telegram_configs
ORDER BY id;
"'
```

Check Telegram Redis cache:

```bash
docker compose exec -T redis sh -lc '
for k in $(redis-cli KEYS "telegram:*"); do
  echo "$k ttl=$(redis-cli TTL "$k")"
done
'
```

---

## Important Data Paths

Custom Nuclei templates:

```text
/data/nuclei/custom/users/{user_id}/{placement}
```

Fresh asset screenshots:

```text
/data/screenshots/users/{user_id}/targets/{target_id}/fresh-assets/
```

Worker temporary workspace:

```text
/tmp/hunt-engine
```

---

## Security Notes

- Change the default admin password.
- Keep `JWT_SECRET` strong and private.
- Use HTTPS in production.
- Restrict dashboard access.
- Telegram bot tokens are stored in DB and are not returned by the API.
- Validate custom Nuclei templates before enabling them.
- Treat takeover and JS intelligence findings as candidates until manually verified.

---

## Release Status

### v3.5.0

v3.5.0 is complete and ready for release.

Completed scope:

- Target PDF Reporting
- AI-ready Data Layer
- `ai_analyses`
- `ai_recommendations`
- `audit_logs`
- Account-scoped LLM provider configuration
- LLM-assisted target analysis narrative
- Commercial-grade deterministic target risk methodology
- Global and account-scoped feature flags
- Account Feature Access UI
- AI-generated Nuclei template draft workflow with human approval guardrails

Production safety notes:

- Deterministic product logic remains authoritative for risk score, risk level, severity, priority, and finding validity.
- LLM output is limited to narrative, interpretation, validation notes, remediation reasoning, and report text.
- Coverage gaps are reported separately and do not inflate vulnerability severity.
- Nuclei AI template drafts are disabled by default.
- Nuclei AI template drafts require both account feature access and `NUCLEI_ALLOW_AI_TEMPLATES=true`.
- Nuclei AI template drafts never auto-save or auto-execute.

### v3.4.0

v3.4.0 completed the structured-evidence security engine foundation:

- Takeover Detection
- JavaScript Intelligence
- `evidence_json`
- Better Finding Evidence UI
- Custom Nuclei execution and ingestion fixes
- Per-account Telegram notifications
- Fresh asset screenshot notification support

---

## Roadmap

### v3.6.0

Potential next milestones:

- Business context and asset criticality model
- Per-target risk policy configuration
- Finding validation workflow and analyst review states
- Recommendation lifecycle actions in UI
- Report branding and customer-ready report templates
- Scheduled report generation
- Organization/workspace model
- External LLM provider hardening and provider-specific validation
- AI-assisted finding clustering and deduplication
- AI-assisted remediation playbooks
- Extended audit log UI and export
- Billing/plan-aware feature entitlements

### Future AI Agent Layer

Planned AI agent capabilities should remain downstream of deterministic guardrails.

Possible agents:

- AI Triage Agent
- AI Summary Agent
- AI Report Agent
- AI Remediation Agent
- AI Nuclei Draft Assistant
- AI-assisted recon recommendation agent

Guardrails:

- AI may add interpretation, correlation, explanation, validation hypotheses, remediation reasoning, and customer-facing narrative.
- AI must not override deterministic source-of-truth fields such as risk score, risk level, severity, finding validity, or priority without explicit human approval and deterministic validation support.

---

## Current Release Summary

v3.5.0 includes:

- professional target PDF reports
- deterministic commercial target analysis
- optional LLM-assisted narrative
- account-scoped LLM provider settings
- deterministic target recommendations
- AI-ready data layer
- audit logging
- global and account-scoped feature flags
- Account Feature Access UI
- hardened Nuclei AI draft workflow

v3.4.0 includes:

- takeover detection
- JavaScript intelligence
- structured `evidence_json`
- improved finding evidence UI
- custom Nuclei execution and ingestion fixes
- per-account Telegram notifications
- fresh asset screenshot notifications
