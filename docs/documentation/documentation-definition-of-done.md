# Documentation Definition of Done

Every Hunt Engine feature added, changed, removed, renamed, moved, or behaviorally modified must update documentation in the same work.

## Required for UI changes

- Update `/documentation` portal content.
- Update `docs/documentation/feature-inventory.md`.
- Update screenshots when UI changes.
- Update release notes.
- Update smoke scripts when routes or critical behavior change.

## Required for backend/runtime/API changes

- Document what changed.
- Document how users interact with it.
- Document output fields.
- Document evidence interpretation.
- Document common errors.
- Document authorization, scope, policy, budget, audit, and stop-condition boundaries.
- Update release notes.
- Add or update smoke coverage where applicable.

## Required for Operator/security features

- Explain scope.
- Explain policy gates.
- Explain approval requirements.
- Explain runtime backend.
- Explain budget and stop conditions.
- Explain audit and memory behavior.
- Explain what is executed and what is not executed.
- Explain what evidence is enough or not enough.

## Release rule

A feature is not release-ready unless documentation is updated, or an explicit roadmap/PDF-approved documentation exception exists.
