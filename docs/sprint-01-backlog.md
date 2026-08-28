# Banking on Aspire Sprint 01 Backlog (Start Here)

Status: Closed

Goal: establish a stable local learning platform with identity and only the state
dependencies that have an active consumer.

## Story 1 - AppHost as single entry point
Acceptance criteria:
- `apphost.cs` declares the Go services, their Dapr sidecars, PostgreSQL, and Keycloak.
- Services reference and wait for only the resources they consume.
- Dependencies are awaited before service starts.

Tasks:
- [x] Wire service references for PostgreSQL and Keycloak.
- [x] Add startup dependency waits and one-shot database migrations.
- [x] Remove Redis and the Dapr statestore after the actor-backed persistence
  experiment no longer had a default runtime consumer.
- [x] Record that state infrastructure returns only with a concrete owned use case.

## Story 2 - Baseline verification
Acceptance criteria:
- One command validates that the app compiles and resources resolve.
- One smoke scenario for account creation and balance read is documented.

Tasks:
- [x] Run a local build/start validation and capture output.
- [x] Add smoke test script placeholder under `scripts/`.
- [x] Document request sequence in a runbook.

## Story 3 - Identity onboarding
Acceptance criteria:
- Keycloak realm/client setup steps are documented.
- Service has clear auth assumptions documented.
- Permissions and ownership are enforced consistently across gRPC services.

Tasks:
- [x] Add `docs/runbooks/keycloak-setup.md` with realm/client roles.
- [x] Map RPCs and endpoints to required permissions.
- [x] Add test matrix for valid/invalid token scenarios.
- [x] Add proto-declared RBAC and Contacts ownership checks.
- [x] Add the automated `make smoke-auth` gate.

## Story 4 - Learning artifacts
Acceptance criteria:
- Progress can be reviewed without inspecting commits.
- Decisions and failures are captured as first-class outputs.

Tasks:
- [x] Create `docs/learning-journal.md` with one entry per milestone.
- [x] Add `docs/adr/0001-banking-on-aspire-goals.md` with architecture goals.
- [x] Add `docs/runbooks/local-start.md` with exact local startup commands.

## Definition of Done
- App starts from one orchestration entry point.
- Baseline flow documented and reproducible.
- Known risks and next actions captured.
- Identity hardening milestone M2 is closed with tests, runbooks, and ADR evidence.
