# Learning Journal

## 2026-08-18 - Structural migration: accounts service

- Moved the existing Go application into the independent
  `services/accounts` module without changing its HTTP API, Dapr app ID, or Aspire
  resource name.
- Added a root `go.work` so future Go services can remain independent modules while
  sharing one local development workspace.
- Established `App`, `platform`, `protos`, and `services` ownership boundaries in
  ADR 0002. Empty deployable microservices are intentionally deferred until their
  migration milestone.
- AppHost remains the single local entry point and now runs the accounts command from
  its service directory.

Purpose: capture what was implemented, what failed, and what was learned at each milestone.

## Entry Template

Date:
Milestone:
Change summary:
What worked:
What failed:
Why it failed:
Debug notes:
Decisions taken:
Next step:

## M0 - Baseline (started)

Date: 2026-07-30
Milestone: M0
Change summary:
- Created the banking-on-aspire roadmap and Sprint 01 backlog.
- Added a mapping document from Bank of Anthos concepts to this repository.

What worked:
- Documentation-first approach reduced accidental code churn.
- Current AppHost remains stable with Go service + Dapr sidecar + Keycloak.

What failed:
- Initial attempt to enable Redis/statestore in AppHost was reverted.

Why it failed:
- Team preference was to defer runtime changes and proceed with planning artifacts first.

Debug notes:
- Keep AppHost edits behind explicit milestone tasks and validate with aspire start.

Decisions taken:
- Continue with incremental milestones and story-based execution.

Next step:
- Implement Sprint 01 Story 1 (AppHost + Redis + statestore), then run smoke checks.

## M0 - Baseline (update)

Date: 2026-07-30
Milestone: M0
Change summary:
- Completed Sprint 01 Story 2 artifacts.
- Added executable smoke script under scripts.
- Expanded local runbook with concrete HTTP sequence and expected outcomes.

What worked:
- Aspire orchestration lifecycle completed successfully with start/ps/stop commands.
- Smoke script syntax and executable permissions validated.

What failed:
- End-to-end HTTP smoke was not executed yet due to missing runtime access token in this step.

Why it failed:
- Token acquisition flow is pending Keycloak setup runbook (Story 3).

Debug notes:
- Keep TOKEN injection explicit via environment variable for repeatability.

Decisions taken:
- Consider Story 2 done for documentation and automation scaffolding.
- Execute full authenticated smoke immediately after Story 3 token flow is documented.

Next step:
- Start Story 3: Keycloak realm/client/scopes + endpoint-to-scope matrix.

## M2 - Identity hardening (documentation pass)

Date: 2026-07-30
Milestone: M2
Change summary:
- Added Keycloak setup runbook for realm, client, client roles, and test user.
- Documented endpoint-to-scope mapping from API handlers.
- Added token test matrix for success and failure auth scenarios.

What worked:
- Existing code already had explicit role names and ownership checks, making mapping straightforward.

What failed:
- End-to-end token retrieval and API call were not executed in this step.

Why it failed:
- This step focused on onboarding artifacts and matrices; live token flow remains next validation action.

Debug notes:
- Required roles are client roles under resource_access for banking-on-aspire-app.
- Current role taxonomy is flow-oriented: accounts.balance.read, transactions.deposit.create, transactions.withdraw.create, accounts.close.
- Preferred username must align with account ownership map.

Decisions taken:
- Treat Story 3 as completed for documentation and test design.

Next step:
- Execute authenticated smoke run and capture outcomes in runbook evidence.
