# Hunt Engine Screenshot Checklist

Screenshots are required for complete Documentation Portal coverage.

## Rules

- Use sanitized stage/demo data.
- Do not show real customer targets.
- Do not show private bug bounty program data.
- Do not show credentials.
- Do not show API keys.
- Do not show bot tokens.
- Do not show cookies or auth tokens.
- Do not show private vulnerability evidence.
- Keep screenshots aligned with the current HUNTOS UI theme.
- Update screenshots whenever the UI changes.

## Required screenshots

### Public / Access

- `access-landing.png`
- `access-login.png`
- `documentation-portal.png`

### Dashboard

- `dashboard-command-center.png`
- `dashboard-stats-cards.png`
- `dashboard-active-processes.png`

### Targets

- `targets-list.png`
- `targets-create-target-modal.png`
- `targets-edit-target-modal.png`
- `targets-import-modal.png`
- `targets-export-modal.png`
- `targets-scan-actions.png`

### Target Detail

- `target-header-tabs.png`
- `target-assets-tab.png`
- `target-assets-filters.png`
- `target-assets-pagination.png`
- `target-urls-tab.png`
- `target-urls-filters.png`
- `target-policy-tab.png`
- `target-findings-tab.png`
- `target-analysis-tab.png`
- `target-pdf-report-action.png`

### Recon / Discovery

- `recon-start-controls.png`
- `recon-active-processes.png`
- `recon-puredns-progress.png`
- `recon-dnsx-results.png`
- `recon-alterx-results.png`
- `recon-wildcard-filtering.png`
- `recon-wordlist-selection.png`

### Findings / Evidence

- `findings-list.png`
- `findings-filters.png`
- `findings-evidence-json.png`
- `findings-triage-status.png`
- `findings-export.png`

### AI Operator

- `operator-chat.png`
- `operator-chat-selected-skills.png`
- `operator-chat-skill-execution.png`
- `operator-chat-controlled-runtime-output.png`
- `operator-chat-approval-required.png`
- `operator-chat-inconclusive-output.png`
- `operator-chat-memory-learning.png`

### Operator Skill Profile

- `operator-profile-panel.png`
- `operator-profile-enabled-skills.png`
- `operator-profile-methodology-selector.png`
- `operator-profile-budget-stop-conditions.png`

### Operator Skills

- `operator-skills-list.png`
- `operator-skills-filters.png`
- `operator-skills-create-modal.png`
- `operator-skills-edit-modal.png`
- `operator-skills-runtime-backend.png`
- `operator-skills-permission-mode.png`
- `operator-skills-budget-stop-conditions.png`

### Operator Learning

- `operator-learning-list.png`
- `operator-learning-filters.png`
- `operator-learning-create-modal.png`
- `operator-learning-edit-modal.png`
- `operator-learning-target-profile-selection.png`

### Bug-Class Validation

- `bug-class-validation-summary.png`
- `bug-class-validation-xss-reflection-context.png`
- `bug-class-validation-dom-xss.png`
- `bug-class-validation-crlf.png`
- `bug-class-validation-cache.png`
- `bug-class-validation-open-redirect.png`
- `bug-class-validation-path-baseline.png`
- `bug-class-validation-cors-cj-csrf.png`

### Nuclei

- `nuclei-templates-list.png`
- `nuclei-template-editor.png`
- `nuclei-template-validation.png`
- `nuclei-template-ai-draft.png`
- `nuclei-template-placement-filters.png`

### Account

- `account-profile.png`
- `account-change-password.png`
- `account-scan-queue.png`
- `account-provider-keys-sanitized.png`
- `account-llm-providers-sanitized.png`
- `account-telegram-config-sanitized.png`
- `account-feature-flags.png`

### System Config

- `settings-users.png`
- `settings-user-modal.png`
- `settings-queue-manager.png`
- `settings-concurrency.png`
- `settings-wordlists.png`
- `settings-wordlist-url-import.png`
- `settings-puredns-resolvers.png`
- `settings-llm-provider-sanitized.png`
- `settings-telegram-sanitized.png`
- `settings-virustotal-sanitized.png`
- `settings-monitoring.png`
- `settings-system-logs-sanitized.png`
- `settings-feature-flags.png`

### Reports / Export

- `report-pdf-download.png`
- `export-targets.png`
- `export-assets.png`
- `export-ips.png`
- `export-urls.png`
- `export-findings-csv-json.png`

## Screenshot naming

Use lowercase kebab-case.

Good examples:

- `target-assets-filters.png`
- `operator-chat-controlled-runtime-output.png`
- `settings-puredns-resolvers.png`

Bad examples:

- `Screenshot 1.png`
- `real-client-target.png`
- `prod-secret-key-visible.png`

## Storage paths

Persian screenshots:

- `frontend/public/docs/screenshots/fa/`

English screenshots:

- `frontend/public/docs/screenshots/en/`

## Review checklist before commit

- Screenshot matches current UI.
- Screenshot is readable.
- Sensitive data is sanitized.
- File name follows convention.
- Documentation references the screenshot path.
- The feature inventory mentions the feature.
