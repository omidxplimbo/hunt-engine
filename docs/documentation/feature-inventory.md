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
