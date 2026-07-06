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


# AI Operator and Skills

## Attack Surface Chat / AI Operator

UI path: `/targets/:id -> Analysis -> Attack Surface Chat`

Purpose:

- Interactive AI-driven pentest operator for target-scoped reasoning.
- Uses target evidence, memory, policy, skills, previous controlled runtime results, URLs, assets, findings, JS intelligence, and user methodology records.
- Produces hypotheses, selected skills, proposed/controlled actions, observations, evidence analysis, memory learning, and next steps.

Features:

- Chat prompt input.
- Target-scoped evidence retrieval.
- Hypothesis generation.
- Bug-Class Reasoning Matrix usage.
- Selected skills.
- Skill execution.
- Controlled runtime output.
- Operator observations.
- Memory learning.
- Approval-required action handling.
- Blocked/inconclusive classification.
- Next recommended steps.

Important output fields:

- `selected_skills`
- `skill_execution`
- `output_json`
- `hypotheses`
- `observations`
- `observation_ids`
- `runtime_scope`
- `execution_level`
- `approval_required`
- `blocked_count`
- `inconclusive_count`
- `not_implemented`
- `next_step`

User actions:

- Ask what to test next.
- Ask for bug-class-specific validation.
- Ask why a result is inconclusive.
- Ask for evidence interpretation.
- Approve or reject higher-risk proposed actions.
- Provide missing context such as auth/session details when needed.
- Review skill execution and observations.

Outputs:

- Operator answer.
- Structured output JSON.
- Skill execution results.
- Controlled runtime observations.
- Memory records.
- Proposed actions.
- Evidence interpretation.

Evidence interpretation:

- Operator hypotheses are not findings by themselves.
- Skill execution evidence must be reviewed before promotion.
- Inconclusive means the system tested but evidence is not enough for a claim.
- Blocked/WAF/403/429/5xx does not confirm vulnerability.
- Previous failures and inconclusive tests should be learned and not blindly repeated.

Common errors:

- No target evidence.
- No candidate URL/parameter.
- Skill not implemented.
- Policy blocked.
- Approval required.
- Missing auth context.
- Runtime error.
- LLM provider not configured.
- Output too generic because target memory is sparse.

Security notes:

- Operator must remain target-scoped.
- Real validation/exploitation requires authorization and policy support.
- Payload execution, state-changing actions, browser proof, shell/tool runner, custom script runner, and brute-force workflows require explicit permission, budgets, stop conditions, and audit.
- Operator should not frame controls as passive-only restrictions; controls are professional authorized execution boundaries.

Documentation requirements:

- Screenshot of Operator Chat.
- Screenshot of selected skills.
- Screenshot of skill execution output.
- Screenshot of approval-required state.
- Explain output_json fields.
- Explain policy-gated execution.
- Explain evidence promotion path.

---

## Operator Skill Profile Panel

UI path: `/targets/:id -> Analysis -> Operator Profile`

Purpose:

- Configure target-specific Operator Skill Profile.
- Select which executable skills and methodology records guide the Operator for this target.

Features:

- Enable/disable target skill profile.
- Permission mode.
- Enabled skill slugs.
- Disabled skill slugs.
- Preferred learning record IDs.
- Allowed runtime backends.
- Budget defaults.
- Stop conditions.
- Metadata.

User actions:

- Enable profile for target.
- Allow or disable specific skills.
- Select preferred methodology records.
- Restrict runtime backends.
- Configure budget defaults.
- Configure stop conditions.
- Save profile.

Outputs:

- Target skill profile.
- Operator skill selection changes.
- Methodology context included in Operator reasoning.
- Runtime backend restrictions.

Evidence interpretation:

- Enabled skill does not mean automatic execution.
- Disabled skill prevents matching skill selection for that target.
- Methodology records guide reasoning but are not evidence.
- Runtime backend allowance controls what execution mechanisms are permitted.

Common errors:

- Skill appears unavailable because it is disabled in profile.
- Methodology record not applied because bug class/skill slug does not match.
- Runtime backend blocked by profile.
- User enters raw IDs manually and selects wrong record.
- Budget/stop condition JSON invalid.

Security notes:

- Target Skill Profile can affect real execution paths.
- Runtime backends must match target authorization.
- Budgets and stop conditions should be conservative for new targets.
- Higher-risk backends need explicit authorization.

Documentation requirements:

- Screenshot of Operator Profile panel.
- Explain enabled vs disabled skill slugs.
- Explain preferred learning records.
- Explain runtime backend control.
- Explain budgets and stop conditions.

---

## Operator Skills Page

UI route: `/operator-skills`

Purpose:

- Manage executable/plannable Operator Skills.
- Supports built-in skills and user-defined skill definitions.

Features:

- List skills.
- Include disabled toggle.
- Search name, slug, bug class, or runtime.
- Create skill.
- Edit skill.
- Delete skill.
- Scope selection.
- Category selection.
- Skill type selection.
- Runtime backend selection.
- Bug class field.
- Risk level.
- Safety level.
- Test level.
- Autonomy level.
- Permission mode.
- Trigger signals.
- Custom definition.
- Budget defaults.
- Stop conditions.
- Enabled/disabled state.

Categories:

- Recon.
- Parameter Intelligence.
- HTTP Evidence Analysis.
- Client Side.
- Injection.
- Access Control.
- Network / File / Cloud.
- Business Logic.
- Finding Promotion.
- Exploit Validation.

Skill types:

- Planning.
- Analysis.
- Active Validation.
- Exploit Runtime.
- Chain.
- Advisory.

Runtime backends:

- None.
- Internal HTTP Runtime.
- Browser Runtime.
- Payload Generator.
- Tool Runner.
- Shell Runner.
- Custom Script Runner.
- Bounded Brute Force Runner.

Permission modes:

- Scope-aware Authorized.
- Manual Approval.
- Assisted Autopilot.
- Authorized Autonomous.

Risk levels:

- Info.
- Low.
- Medium.
- High.
- Critical.

User actions:

- Search and inspect skills.
- Include disabled skills when needed.
- Create a new user-defined skill.
- Edit a user-defined skill.
- Delete a user-defined skill.
- Set runtime backend.
- Set permission/autonomy/risk levels.
- Define trigger signals.
- Define custom runtime/workflow metadata.
- Configure budget and stop conditions.

Outputs:

- OperatorSkill record.
- Skill metadata.
- Runtime backend choice.
- Permission model.
- Budget/stop-condition metadata.
- Skill availability to target profiles and Operator selector.

Common errors:

- Duplicate slug.
- Invalid slug.
- Runtime backend not yet implemented.
- Invalid JSON metadata.
- Skill disabled but user expects it to run.
- Permission mode too permissive for target policy.
- Missing trigger signals makes skill hard to select.

Security notes:

- User-defined skills may become execution-capable.
- Shell/tool/custom-script/brute-force runners require explicit authorization, scope, rate limits, budgets, stop conditions, and audit.
- Skills must not be used for out-of-scope, destructive, DoS-like, credential-theft, or uncontrolled activity.
- Permission model should describe authorized execution, not passive-only behavior.

Documentation requirements:

- Screenshot of skill list.
- Screenshot of create/edit form.
- Explain every field.
- Explain runtime backend options.
- Explain permission modes.
- Explain budget and stop condition requirements.
- Explain built-in vs user-defined skills.

---

## Built-in Operator Skills

Purpose:

- Provide system-defined skills for common pentest reasoning, evidence analysis, and controlled validation.

Known skill slugs:

- `parameter_inventory`
- `http_evidence_analysis`
- `auth_context_needed`
- `js_audit`
- `xss_reflection`
- `open_redirect`
- `path_traversal_baseline`
- `xss_reflection_context`
- `open_redirect_chain`
- `path_traversal_file_read_baseline`
- `crlf_header_injection`
- `cache_poisoning_deception`
- `cors_clickjacking_csrf`
- `dom_xss`

Documentation requirements for each skill:

- What it does.
- When Operator selects it.
- Required evidence/context.
- Runtime backend.
- Test/autonomy/safety level.
- What it executes.
- What it does not execute.
- Output fields.
- Evidence interpretation.
- Common inconclusive cases.
- Security boundaries.

---

## Operator Learning / Methodology Records

UI route: `/operator-learning`

Purpose:

- Store user-authored methodology and skill instructions.
- Guide the Operator when matching bug class, skill slug, target, project, or user-global context.

Features:

- Scope filter.
- Status filter.
- Bug class filter.
- Skill slug filter.
- Create methodology record.
- Edit methodology record.
- Delete methodology record.
- Title.
- Summary.
- Content.
- Scope.
- Source.
- Status.
- Project key.
- Target ID.
- Bug class.
- Skill slug.
- Applies to.
- Trigger signals.
- Methodology JSON.
- Constraints.
- Execution hints.
- Evidence JSON.
- Metadata.
- Confidence.
- Use count.
- Last used timestamp.

Scopes:

- User Global.
- Project.
- Target.
- Organization Global.

Statuses:

- Active.
- Disabled.
- Superseded.

Sources:

- User confirmed.
- Skill result.
- Operator inference.

User actions:

- Create methodology for a bug class.
- Create methodology for a skill slug.
- Filter existing records.
- Edit methodology content.
- Disable or supersede old methodology.
- Delete a record.
- Link preferred records from Target Skill Profile.

Outputs:

- OperatorLearningRecord.
- Methodology context injected into Operator reasoning.
- Preferred methodology list for target.
- Learning use count.

Evidence interpretation:

- Methodology is not vulnerability evidence.
- Methodology affects reasoning and sequencing.
- Methodology should be specific enough to map to bug class or skill slug.
- User-confirmed methodology is stronger than generic inferred notes.

Common errors:

- Record too generic.
- Wrong scope.
- Disabled record not applied.
- Bug class/skill slug mismatch.
- Target-specific record used on wrong target.
- JSON fields malformed.

Security notes:

- Methodology can guide active/exploit workflows, so constraints and authorization notes must be explicit.
- Do not store secrets or private credentials in methodology content.
- Methodology should not instruct uncontrolled or out-of-scope activity.

Documentation requirements:

- Screenshot of records list.
- Screenshot of create/edit modal.
- Explain scope/status/source.
- Explain difference between executable skills and methodology records.
- Explain how target profile selects preferred records.

---

# Bug-Class Validation Runtimes

## Overview

Access path: `AI Operator Chat -> selected skills -> skill_execution`

Purpose:

- Execute controlled evidence-gathering runtimes for supported bug classes.
- Provide structured observations without overclaiming.
- Store useful observations and learning for future Operator reasoning.

Supported controlled validation slugs:

- `xss_reflection_context`
- `dom_xss`
- `crlf_header_injection`
- `cache_poisoning_deception`
- `open_redirect_chain`
- `path_traversal_file_read_baseline`
- `cors_clickjacking_csrf`

General outputs:

- Candidate URL/parameter.
- Runtime scope.
- Execution level.
- Probe method.
- HTTP status.
- Headers.
- Reflection or behavior markers.
- Observation IDs.
- Evidence summary.
- Inconclusive/blocked/failure classification.
- Next recommended step.

General common errors:

- No candidate found.
- Missing parameter inventory.
- Target blocked by WAF.
- 403/429/5xx response.
- Policy requires approval.
- Missing auth context.
- Probe response does not preserve marker.
- Evidence insufficient for claim.

General security notes:

- These runtimes are controlled validation steps.
- They are not full exploit proof by default.
- Escalation to real exploit proof requires authorization, policy support, budget, stop conditions, and audit.

---

## XSS Reflection Context Runtime

Skill slug: `xss_reflection_context`

Runtime scope:

- `controlled_marker_reflection_probe_no_exploit_payload`

Purpose:

- Classify reflected input context using inert controlled markers.
- Identify whether a parameter reflects into HTML, attribute, script-like, JSON, or text contexts where detectable.

Executes:

- Controlled marker GET reflection probe.
- No exploit payload.
- No browser execution.

Does not execute:

- No script execution proof.
- No alert payload.
- No DOM/browser proof.
- No stored XSS mutation.

Outputs:

- Reflected marker status.
- Reflection context.
- Candidate parameter.
- Probe URL.
- HTTP status.
- Observation IDs.
- Inconclusive reason where applicable.

Evidence interpretation:

- Reflection is a candidate signal, not confirmed XSS.
- Context classification guides payload strategy later when authorized.
- No browser proof means no confirmed exploitability claim.

Security notes:

- Low/no-destructive marker only.
- Escalation to payload/browser proof requires authorization.

---

## DOM XSS Runtime

Skill slug: `dom_xss`

Runtime scope:

- `js_source_sink_evidence_no_browser_execution`

Purpose:

- Analyze JavaScript/source-sink evidence for DOM XSS candidates.

Executes:

- Static/source-sink review of collected JS/URLs.
- Candidate recording.

Does not execute:

- No browser execution.
- No payload execution.
- No DOM exploit proof.

Outputs:

- Source/sink candidates.
- JS URL.
- Signal type.
- Confidence.
- Observation IDs.
- Next browser-validation plan where appropriate.

Evidence interpretation:

- Source/sink evidence is a candidate, not proof.
- Confirmed DOM XSS requires controlled browser validation later.

Security notes:

- No browser payload execution in this runtime.
- Authenticated browser context is a later authorized workflow.

---

## CRLF/Header Injection Runtime

Skill slug: `crlf_header_injection`

Runtime scope:

- `controlled_header_marker_probe_no_raw_crlf_payload`

Purpose:

- Check for header-related marker behavior without sending raw CRLF payloads.

Executes:

- Encoded marker/header behavior probe.

Does not execute:

- No raw CRLF payload.
- No response splitting exploit.
- No cache poisoning chain.

Outputs:

- Marker behavior.
- Header evidence.
- Candidate parameter.
- HTTP status.
- Observation IDs.

Evidence interpretation:

- Header marker behavior may justify deeper validation.
- It is not proof of response splitting without controlled authorized proof.

Security notes:

- Raw CRLF and response splitting proof require stronger authorization.

---

## Cache Poisoning / Deception Runtime

Skill slug: `cache_poisoning_deception`

Runtime scope:

- `controlled_cache_behavior_probe_no_poisoning_payload`

Purpose:

- Compare cache-related behavior safely without poisoning payloads.

Executes:

- Cache-buster request.
- Two-request comparison.
- Header/cache evidence collection.

Does not execute:

- No cache poisoning payload.
- No victim-impacting poisoning.
- No persistent cache mutation proof.

Outputs:

- Cache headers.
- Vary behavior.
- CDN/cache indicators.
- First/second response comparison.
- Observation IDs.

Evidence interpretation:

- Cacheability or inconsistent behavior is a candidate signal.
- Confirmed poisoning/deception needs additional authorized proof.

Security notes:

- No payload designed to poison shared cache.

---

## Open Redirect Chain Runtime

Skill slug: `open_redirect_chain`

Runtime scope:

- `controlled_redirect_behavior_probe_no_external_follow`

Purpose:

- Evaluate redirect-like parameters and chain relevance without following external redirects.

Executes:

- Controlled marker URL probe.
- Redirect behavior inspection.
- No external redirect following.

Does not execute:

- No external navigation.
- No OAuth/account takeover chain proof.
- No phishing workflow.

Outputs:

- Location header behavior.
- Redirect status.
- Candidate parameter.
- Marker preservation.
- Observation IDs.

Evidence interpretation:

- Redirect behavior is candidate evidence.
- Impact depends on context such as OAuth, SSO, callback, allowlist bypass, or parser confusion.

Security notes:

- Chain validation requires explicit authorization.

---

## Path Traversal / File Read Baseline Runtime

Skill slug: `path_traversal_file_read_baseline`

Runtime scope:

- `controlled_path_baseline_probe_no_sensitive_file_read`

Purpose:

- Identify file/path-like parameter behavior using inert path markers.

Executes:

- Safe baseline path marker variants.
- Candidate behavior recording.

Does not execute:

- No sensitive file read.
- No `/etc/passwd` style proof.
- No filesystem extraction.
- No destructive access.

Outputs:

- Candidate endpoint.
- Parameter.
- Baseline behavior.
- HTTP status.
- Observation IDs.
- Inconclusive reason.

Evidence interpretation:

- Path-like behavior is candidate evidence.
- Sensitive file-read proof requires explicit authorization and controlled escalation.

Security notes:

- No sensitive file content is requested in this runtime.

---

## CORS / Clickjacking / CSRF Baseline Runtime

Skill slug: `cors_clickjacking_csrf`

Runtime scope:

- `controlled_cors_frame_cookie_header_probe_no_state_change`

Purpose:

- Collect header-level evidence relevant to CORS, framing, cookies, and CSRF posture.

Executes:

- GET/OPTIONS header evidence collection.
- Header and cookie attribute review.

Does not execute:

- No state-changing CSRF.
- No credentialed attack.
- No clickjacking interaction.
- No account/session mutation.

Outputs:

- CORS headers.
- Frame headers.
- Cookie attributes.
- OPTIONS behavior.
- Observation IDs.
- Risk hints.

Evidence interpretation:

- Header weakness is not always exploitability.
- Impact depends on sensitive endpoints, auth context, credentials, and browser behavior.

Security notes:

- State-changing validation requires explicit authorization and auth context.

---
