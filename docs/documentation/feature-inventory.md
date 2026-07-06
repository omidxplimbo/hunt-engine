# Hunt Engine Feature Inventory

This inventory is the source checklist for the `/documentation` portal.

Every user-visible feature, setting, action, output, workflow, route, modal, tab, runtime, and behavior change must be documented here and in the Documentation Portal UI.

## Documentation ownership rule

A Hunt Engine feature is not release-ready unless:

- The feature exists in this inventory.
- The `/documentation` portal explains it.
- Screenshots are added or updated when UI changes.
- Release notes mention the change.
- Smoke coverage is added or updated when route/runtime behavior changes.

---

# Public Access

## Landing Page

UI route: `/`

Purpose:

- Public entry page before authentication.
- Introduces the product.
- Routes unauthenticated users to login/session initialization.
- Redirects authenticated users toward dashboard behavior.

User actions:

- Open the public product URL.
- Read product entry information.
- Start session or go to login.
- Return to product entry from documentation.

Controls and visible elements:

- Product branding.
- Session initialization CTA.
- Login navigation.
- Public marketing/positioning content.

Outputs:

- Public React route.
- No authenticated data should be shown.

Common errors:

- Public route returns stale frontend bundle.
- Authenticated user does not redirect correctly.
- Direct IP/origin access blocked by nginx/Cloudflare origin policy.

Security notes:

- Public landing must not expose internal configuration.
- Public landing must not expose private target data.
- Direct IP/origin behavior should remain locked down in production.

Documentation requirements:

- Persian and English guide.
- Screenshot of public landing.
- Explain unauthenticated vs authenticated behavior.
- Explain production/stage URL behavior.

---

## Login

UI route: `/login`

Purpose:

- Authenticate users.
- Store token, username, and role client-side.
- Redirect valid sessions to dashboard.

User actions:

- Enter username.
- Enter password.
- Submit login.
- Handle invalid credentials.

Controls and fields:

- Username field.
- Password field.
- Login/submit button.

Outputs:

- JWT token stored in browser local storage.
- Username stored in browser local storage.
- Role stored in browser local storage.
- Redirect to `/dashboard`.

Common errors:

- Invalid credentials.
- Expired token.
- Missing authorization token.
- Invalid token format.
- Protected route redirects user to login.

Security notes:

- Never expose passwords or tokens in docs/screenshots.
- 401 should only clear session for actual token/auth errors.
- Login examples should use demo credentials only.

Documentation requirements:

- Login screenshot.
- Explain role-based access.
- Explain admin-only routes.
- Explain logout/disconnect behavior.

---

## Documentation Portal

UI route: `/documentation`

Purpose:

- Public, bilingual, HUNTOS-themed product documentation.
- Teach every page, feature, setting, workflow, output, error, and authorization boundary.
- Serve as mandatory user-facing documentation for every release.

User actions:

- Switch language between Persian and English.
- Search documentation topics.
- Navigate documentation map/sidebar.
- Read feature-specific guidance.
- Find screenshot references and expected UI paths.

Controls and visible elements:

- Language toggle.
- Search field.
- Documentation map/sidebar.
- Feature topic content.
- UI path panel.
- Screenshot required panel.
- Documentation Definition of Done panel.
- Current coverage panel.

Outputs:

- Feature-by-feature documentation.
- Screenshot paths.
- Documentation DoD.
- Product coverage overview.

Common errors:

- Missing bundle marker after frontend deploy.
- Stale frontend assets.
- Route returns React shell but docs topic missing.
- Persian typography broken if local font is missing.

Security notes:

- Documentation must not reveal secrets.
- Screenshots must be sanitized.
- Security-sensitive features must explain scope, authorization, policy, budget, audit, and stop conditions.

Documentation requirements:

- Route smoke must pass.
- Persian/English content must remain aligned.
- Every changed feature must update this portal.

---

# Dashboard

## Command Center

UI route: `/dashboard`

Purpose:

- High-level overview of Hunt Engine state.
- Help users understand whether discovery, attack surface, and system status need attention.

Features:

- Total Targets.
- Total Assets.
- Live Nodes.
- Fresh Intel in the last 24 hours.
- Asset status breakdown.
- Top technologies.
- Top open ports.
- Active/system process overview where available.

User actions:

- Review global project state.
- Identify whether targets/assets are increasing.
- Check live nodes and fresh intel.
- Navigate to Targets for detailed work.

Outputs:

- Dashboard statistics.
- Project health summary.
- Attack surface trend signals.

Evidence interpretation:

- Total Assets is not the same as exploitable surface.
- Live Nodes are better candidates for web review.
- Fresh Intel indicates newly discovered data and should be reviewed first.
- Top technologies and ports can guide testing priorities.

Common errors:

- Zero stats when no targets exist.
- Stats API failure.
- Process list empty because no jobs are active.
- Stale numbers due to backend/API/cache issue.

Security notes:

- Dashboard is a summary view.
- Do not treat dashboard counts as vulnerability evidence.

Documentation requirements:

- Screenshot of dashboard cards.
- Explain each metric.
- Explain when to move from dashboard to target detail.

---

# Targets

## Targets List

UI route: `/targets`

Purpose:

- Manage all targets and scan lifecycle.
- Create, import, export, edit, delete, stop, restart, and resume target scans.

Features:

- Active Targets list.
- Create Target modal.
- Import Target modal.
- Export Targets.
- Edit Target.
- Delete Target.
- Stop scan.
- Resume/execute scan.
- Fresh restart.
- Open target detail page.

User actions:

- Create a new target.
- Import target JSON.
- Export target data.
- Start/resume discovery.
- Stop a running scan.
- Fresh restart a scan.
- Edit target settings.
- Delete a target.
- Open target workspace.

Outputs:

- Target rows.
- Scan state.
- Target metadata.
- Import/export files.
- Active scan actions.

Common errors:

- Invalid domain.
- Duplicate target.
- Scan already running.
- Import JSON invalid.
- Delete confirmation not accepted.
- Target row state stale until refresh/refetch.

Security notes:

- Only create authorized targets.
- Confirm the root domain and scope before discovery.
- Fresh restart clears scan checkpoint/temp state but must not be confused with deleting previous assets/findings unless implementation says so.

Documentation requirements:

- Screenshot of targets list.
- Screenshot of scan action buttons.
- Explain each action: halt, fresh restart, execute/resume, edit, delete.
- Explain import/export behavior.

---

## Create Target Modal

UI component: `CreateTargetModal`

Purpose:

- Create a target and configure discovery modules.

Fields and controls:

- Name.
- Root domain.
- Description.
- Frequency.
- Modules.
- Use AlterX.
- Use Waymore.
- Use GAU.
- Use Katana.
- Use VirusTotal.
- Use port scan.
- Use Cero.
- Use crt.sh.
- Use PureDNS.
- Use AbuseDB.
- Use Amass.
- Use Nuclei.
- Nuclei profile.
- PureDNS wordlists.

User actions:

- Enter target identity.
- Select discovery/crawl/source modules.
- Select PureDNS wordlists when PureDNS is enabled.
- Select Nuclei profile when Nuclei is enabled.
- Submit target creation.

Outputs:

- New target row.
- Stored module configuration.
- Discovery settings for scan runtime.

Evidence interpretation:

- Module selection controls coverage.
- Passive sources and brute-force sources produce different candidate types.
- PureDNS depends heavily on resolver quality and selected wordlists.
- Nuclei profile affects template coverage and noise level.

Common errors:

- Root domain missing or malformed.
- Module combination too heavy for intended test.
- PureDNS enabled without proper wordlists/resolvers.
- VirusTotal enabled without valid API configuration.
- Nuclei enabled but templates/config missing.

Security notes:

- Only enable modules authorized for the target.
- Port scanning and active crawling may require explicit permission.
- Nuclei profile selection must align with authorization and noise tolerance.

Documentation requirements:

- Screenshot of modal.
- Explain every module toggle.
- Explain when to enable/disable each module.
- Explain safe initial configuration for new users.
- Explain advanced configuration for authorized deeper testing.

---

## Edit Target Modal

UI component: `EditTargetModal`

Purpose:

- Modify target metadata and module settings.

Fields and controls:

- Name.
- Root domain.
- Description.
- Frequency.
- Module toggles.
- Nuclei profile.
- PureDNS wordlists.
- In-scope state.

User actions:

- Open target edit modal.
- Update metadata or module settings.
- Save changes.

Outputs:

- Updated target configuration.
- Future scans use updated settings.

Common errors:

- Invalid root domain.
- Changing modules while scan is running may not affect current job.
- User expects old evidence to be deleted, but edit only changes configuration.

Security notes:

- Scope changes must be documented and intentional.
- Do not mark targets in-scope unless authorization exists.

Documentation requirements:

- Screenshot of edit modal.
- Explain which changes affect future scans vs existing data.

---

## Import Target Modal

UI component: `ImportTargetModal`

Purpose:

- Import previously exported target data.

User actions:

- Open import modal.
- Select/paste import file.
- Confirm import.

Outputs:

- Imported target metadata.
- Imported assets/URLs where supported.
- Import status.

Common errors:

- Invalid JSON.
- Unsupported export version.
- Duplicate target conflict.
- Partial import due to malformed data.

Security notes:

- Imported data may contain sensitive target information.
- Do not import data from untrusted sources without review.

Documentation requirements:

- Screenshot of import modal.
- Explain accepted format and versioning.

---

## Export Target Modal

UI component: `ExportTargetModal`

Purpose:

- Export target data for backup, migration, or offline review.

User actions:

- Open export modal.
- Select export scope/options.
- Download export.

Outputs:

- Target export JSON.

Security notes:

- Export may contain sensitive attack-surface data.
- Store and share exports securely.
- Sanitize before sharing in documentation or support requests.

Documentation requirements:

- Screenshot of export modal.
- Explain what the export includes.
- Explain what it does not include.

---


# Target Detail Workspace

## Target Detail Header

UI route: `/targets/:id`

Purpose:

- Main workspace header for a selected target.
- Shows target identity, root domain, loading state, and available target-level actions.

Features:

- Back to targets.
- Target name.
- Root domain.
- Loading/fetching indicator.
- PDF Report action.
- Assets tab.
- Intel / URLs tab.
- Policy tab.
- Findings tab.
- Analysis tab.
- Export IPs.
- Export Assets.
- Export URLs.

User actions:

- Return to targets list.
- Switch target workspace tabs.
- Download report or exports.
- Start review from Assets or URLs.
- Open Policy before controlled validation.
- Open Findings for evidence review.
- Open Analysis for AI/operator workflows.

Outputs:

- Active target context.
- Selected tab state.
- Export/download files.
- Report download when enabled.

Common errors:

- Target loading fails.
- Target ID not found.
- Feature flag disables PDF report, Policy, or Analysis.
- Export action returns empty file due to filters.
- Stale frontend bundle after deploy.

Security notes:

- Target context determines scope.
- Exported data may contain sensitive attack-surface evidence.
- Report downloads must be handled as sensitive documents.

Documentation requirements:

- Screenshot of target header and tabs.
- Explain every tab and action.
- Explain disabled feature-flag behavior.

---

## Assets Tab

UI route: `/targets/:id -> Assets`

Purpose:

- Review discovered assets/subdomains and prioritize live attack surface.

Features:

- Search assets.
- All filter.
- Live filter.
- Dead filter.
- Web filter.
- DNS filter.
- Ports filter.
- No CDN filter.
- CDN filter.
- WAF filter.
- Cloud filter.
- Status code filter.
- Source/provider filters.
- Sortable Providers / DNS column.
- Sortable Status column.
- Pagination.
- Export IPs.
- Export Assets.

Displayed asset evidence:

- Asset value.
- Source/provider.
- DNS status.
- HTTP status.
- Live/dead state.
- IP addresses.
- Ports.
- CDN indicator.
- WAF indicator.
- Cloud indicator.
- Technologies.
- Created/updated timestamps where shown.

User actions:

- Search by host/domain/IP.
- Filter to live web assets.
- Filter DNS-only assets.
- Filter by source to debug discovery quality.
- Sort by provider/status/value.
- Export assets or IPs.
- Select candidates for AI Operator review.

Outputs:

- Filtered asset table.
- Asset count and pagination state.
- Exported assets/IPs.
- Candidate list for manual or operator-guided testing.

Evidence interpretation:

- Live assets are higher priority for active web review.
- Dead assets may still matter for takeover, DNS history, or recon coverage.
- WAF/CDN/cloud indicators affect payload strategy and validation interpretation.
- Source attribution helps identify whether a host came from passive discovery, brute-force, mutation, or crawl.
- 403/429/5xx and WAF-blocked responses are not confirmed vulnerabilities.

Common errors:

- Filters hide expected assets.
- Pagination appears empty after filter changes.
- DNS-only asset is mistaken for live web asset.
- WAF/CDN causes blocked or misleading HTTP responses.
- Asset has no A record and should not be counted live.
- Source attribution looks missing because older data lacked source metadata.

Security notes:

- Do not actively test out-of-scope assets.
- Do not overclaim dead/blocked/inconclusive states.
- Export files should be treated as sensitive.

Documentation requirements:

- Screenshot of asset table.
- Screenshot of filters.
- Explain each filter.
- Explain source/provider interpretation.
- Explain live/dead and WAF/CDN/cloud evidence.

---

## Intel / URLs Tab

UI route: `/targets/:id -> Intel / URLs`

Purpose:

- Review collected URLs, endpoints, JavaScript resources, and historical/crawler intelligence.

Features:

- Search URLs.
- Only JS filter.
- Source filters.
- Sort by resource locator.
- Sort by source.
- Sort by created time.
- Pagination.
- Export URLs.

URL sources:

- Wayback.
- GAU.
- Katana.
- Waymore.
- VirusTotal.
- Imported or internal sources where available.

Displayed URL evidence:

- Resource locator.
- Source.
- Created timestamp.
- JS/resource type where inferred.

User actions:

- Search for endpoints, parameters, extensions, or keywords.
- Filter JavaScript files.
- Filter by source.
- Export URL inventory.
- Identify parameterized endpoints for validation.
- Identify JS resources for JS audit.

Outputs:

- URL inventory.
- JavaScript candidate list.
- Parameterized endpoint candidates.
- Exported URL file.

Evidence interpretation:

- Parameterized URLs are useful for XSS, open redirect, path traversal, SQLi/NoSQLi strategy, SSRF, authz, and business logic review.
- JavaScript URLs are useful for source/sink, route, secret, API endpoint, and DOM XSS analysis.
- Historical URLs may be stale but still valuable for endpoint discovery.
- Source attribution explains where the URL came from and how reliable/current it may be.

Common errors:

- URL list is empty because URL modules were disabled.
- Only JS filter hides non-JS endpoints.
- Source filter hides expected URLs.
- Historical URL no longer resolves.
- Export output follows active filters and may appear incomplete.

Security notes:

- Do not actively test out-of-scope URLs.
- Historical URLs should be validated safely before claims.
- Secret-looking strings from JS require confirmation and responsible handling.

Documentation requirements:

- Screenshot of URLs tab.
- Screenshot of JS-only filter.
- Explain every source.
- Explain endpoint and parameter hunting workflow.

---

## Target Policy Tab

UI route: `/targets/:id -> Policy`

Purpose:

- Define authorized execution boundaries for the target.
- Control Operator autonomy, approvals, runtime levels, rate/budget boundaries, and scope behavior.

Features:

- Target scope settings.
- Operator mode.
- Manual only mode.
- Assisted autopilot mode.
- Strict approval mode.
- Auto-execute level 0.
- Auto-execute level 1.
- Require approval level 2.
- Require approval level 3.
- Rate limit controls where available.
- Budget and stop-condition controls where available.

User actions:

- Review target authorization before testing.
- Set operator mode.
- Configure auto-execution for low-risk actions.
- Require approval for higher-risk actions.
- Save policy.
- Revisit policy before active validation or exploit proof.

Outputs:

- Stored target policy.
- Operator behavior changes.
- Runtime approval decisions.
- Action status such as executed, proposed, blocked, or approval required.

Evidence interpretation:

- Policy does not make Hunt Engine passive-only.
- Policy defines controlled, authorized, audited execution.
- A policy block is a safety/scope decision, not evidence that a bug does or does not exist.
- Approval-required means the system needs explicit user authorization before proceeding.

Common errors:

- Action blocked by policy.
- Approval required.
- Operator did not auto-execute because mode is strict/manual.
- Feature flag hides Policy tab.
- User expects high-risk validation to run under low-risk autopilot settings.

Security notes:

- Real validation and exploitation require explicit authorization.
- Out-of-scope, destructive, DoS-like, credential-theft, uncontrolled brute-force, or unbounded actions must be blocked.
- Budget, rate limit, stop condition, and audit requirements must be clear.

Documentation requirements:

- Screenshot of policy tab.
- Explain each operator mode.
- Explain each approval level.
- Explain policy blocked vs inconclusive vs vulnerable.
- Explain authorized exploitation boundaries.

---

## Findings Tab

UI route: `/targets/:id -> Findings`

Purpose:

- Review, triage, update, and export findings with evidence.

Features:

- Findings list.
- Finding stats.
- Status filter.
- Severity filter.
- Source tool filter.
- Category filter.
- Search.
- Update finding status.
- Triage note.
- Export target findings as CSV.
- Export target findings as JSON.

Displayed finding evidence:

- Title/name.
- Severity.
- Confidence.
- Status.
- Category.
- Source tool.
- Evidence JSON.
- Triage note.
- Created/updated timestamps where shown.

User actions:

- Filter by severity/status/category.
- Search findings.
- Open or inspect finding evidence.
- Update triage status.
- Add triage note.
- Export findings for report/review.

Outputs:

- Filtered finding list.
- Finding stats.
- Updated finding status.
- Exported CSV/JSON.

Evidence interpretation:

- Severity should follow demonstrated impact and scope.
- Confidence should follow evidence quality and reproducibility.
- Inconclusive evidence is not a confirmed vulnerability.
- Blocked/WAF responses are not confirmation by themselves.
- Findings should not be promoted without sufficient evidence.

Common errors:

- False positive finding.
- Missing reproduction.
- Inconclusive evidence.
- Export appears empty because filters are active.
- User changes status without evidence note.

Security notes:

- Findings and exports may contain sensitive vulnerability data.
- Do not disclose findings without authorization.
- Evidence must be sanitized for public reports.

Documentation requirements:

- Screenshot of findings list.
- Screenshot of evidence JSON.
- Explain severity/confidence/status.
- Explain export behavior.
- Explain triage workflow.

---

## Analysis Tab

UI route: `/targets/:id -> Analysis`

Purpose:

- Host AI-assisted analysis, recommendations, agent panels, controlled bug testing, registries, operator profile, and Attack Surface Chat.

Subpanels:

- AI Analysis.
- Recommendations.
- Advisory Agents.
- Agent Actions.
- Bug Tests.
- Pattern Registry.
- Payload Registry.
- Operator Profile.
- Attack Surface Chat.

User actions:

- Review AI-generated target analysis.
- Read recommendations.
- Inspect advisory agents.
- Review proposed or executed agent actions.
- Inspect bug test results.
- Review pattern/payload registries.
- Configure Operator Profile.
- Ask AI Operator to reason over target evidence.

Outputs:

- AI summaries.
- Recommendations.
- Agent action proposals/results.
- Bug test runs/results.
- Pattern and payload metadata.
- Operator profile configuration.
- Chat output and controlled runtime evidence.

Evidence interpretation:

- AI analysis is not automatically a finding.
- Recommendations are guidance until validated.
- Agent action results must be classified as confirmed, inconclusive, blocked, or failed based on evidence.
- Pattern/payload registry entries are metadata and strategy inputs, not vulnerability proof by themselves.

Common errors:

- Analysis tab hidden by feature flag.
- AI provider not configured.
- Operator has insufficient target evidence.
- Skill not implemented.
- Policy blocks execution.
- Result is inconclusive.

Security notes:

- Operator actions must remain target-scoped and policy-gated.
- Payload and exploit-capable workflows require authorization and audit.
- Registry payload metadata should not imply uncontrolled execution.

Documentation requirements:

- Screenshot of Analysis tab.
- Screenshot each subpanel.
- Explain how AI analysis differs from confirmed findings.
- Explain operator evidence workflow.

---
