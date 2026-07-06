# Contributing to Hunt Engine

## Documentation requirement

Every Hunt Engine feature added, changed, removed, renamed, moved, or behaviorally modified must include Documentation Portal updates in the same work.

Before merge, update:

- `/documentation` portal content
- `docs/documentation/feature-inventory.md`
- screenshots when UI changes
- release notes
- smoke scripts when routes or critical behavior change

Security-sensitive documentation must explain:

- scope
- authorization
- policy gates
- budgets
- audit logging
- stop conditions
- runtime backend
- evidence interpretation
- what is executed and what is not executed

A feature is not release-ready unless documentation is updated, or an explicit roadmap/PDF-approved documentation exception exists.

## Screenshot hygiene

Documentation screenshots must use sanitized stage/demo data.

Do not include:

- real customer targets
- private bug bounty program data
- credentials
- API keys
- bot tokens
- cookies
- auth tokens
- private vulnerability evidence
