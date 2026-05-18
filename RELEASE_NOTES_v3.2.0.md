# Hunt Engine v3.2.0

## Overview

Hunt Engine v3.2.0 introduces the first full **Findings System**. This release turns selected reconnaissance signals into structured, triageable, exportable security findings while keeping the core scan pipeline independent and stable.

This version is an important foundation for future security engines and AI-assisted workflows.

## Added

### Findings Core

- Added `Finding` model with severity, category, status, evidence, recommendation, fingerprint, first seen, and last seen fields.
- Added triage metadata:
  - `triage_note`
  - `triaged_at`
  - `triaged_by_user_id`
- Added target-level and global Findings APIs.
- Added target-level Finding stats endpoint grouped by severity, status, source, and category.
- Added CSV and JSON Findings export endpoints.

### Findings UI

- Added a dedicated Findings tab in target details.
- Added severity, status, and search filters.
- Added summary cards for Total, Open, High+, and Fixed findings.
- Added evidence and recommendation display.
- Added in-app triage modal for status changes and optional notes.
- Added CSV and JSON export buttons.

### Built-in Findings Generators

- Added asset-based findings after Probing:
  - Possible admin/login interface
  - Possible directory listing
  - Server error response
  - Potential exposed sensitive services from risky open ports

- Added URL-based findings after Crawling:
  - Admin/login/dashboard paths
  - Exposed configuration or secret-looking paths
  - Version-control paths
  - API documentation/schema paths
  - Debug/monitoring paths
  - Backup/archive artifacts

### URL Canonicalization

- Added URL canonicalization for finding deduplication.
- Removed volatile query parameters such as csrf, nonce, token, session, signature, timestamp, cache, `utm_*`, fbclid, and gclid from finding fingerprints.
- Preserved meaningful parameter names while dropping noisy values.
- Improved URL finding evidence with `canonical_url` and shortened `sample_raw_url`.

### Crawled URL Storage Dedupe

- Added canonical URL fields for new crawled URL storage:
  - `canonical_value`
  - `canonical_hash`
  - `occurrence_count`
  - `last_seen`
- New crawled URLs are deduplicated by canonical hash to reduce noisy archival variants.
- Existing historical URL records are not destructively merged.

### Tests

- Added generator tests for asset findings, URL findings, risky open ports, and URL canonicalization.

## Changed

- Findings are generated after Probing and Crawling when the relevant data is available.
- URL findings use canonical fingerprints instead of raw URL IDs to reduce duplicate findings.
- Finding triage status changes now track who changed the status, when, and why.

## Upgrade Notes

After pulling v3.2.0, rebuild the backend so AutoMigrate can add new columns and tables:

```bash
docker compose up -d --build
```

New or updated database structures include:

- `findings`
- `findings.triage_note`
- `findings.triaged_at`
- `findings.triaged_by_user_id`
- `found_urls.canonical_value`
- `found_urls.canonical_hash`
- `found_urls.occurrence_count`
- `found_urls.last_seen`

Existing old `found_urls` records are not automatically merged. New records are canonicalized going forward.

## Notes for Future AI Work

The Findings layer is designed to support future AI-assisted workflows without making AI part of the critical scan path. Future AI agents should operate asynchronously on structured Findings, evidence, stats, and scan history. Core scans, findings, triage, and exports must continue to work if AI is disabled or unavailable.
