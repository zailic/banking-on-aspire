# Banking on Aspire Sprint 01 Backlog (Start Here)

Goal: establish a stable local learning platform with identity and state dependencies.

## Story 1 - AppHost as single entry point
Acceptance criteria:
- `apphost.cs` declares Go service, Dapr sidecar, Redis, statestore, Keycloak.
- Service references Redis, statestore, and Keycloak resources.
- Dependencies are awaited before service starts.

Tasks:
- [ ] Enable Redis resource and password parameter in AppHost.
- [ ] Enable Dapr statestore component in AppHost.
- [ ] Wire service references for Redis/statestore/Keycloak.
- [ ] Add startup dependency waits for Redis and Keycloak.

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

Tasks:
- [x] Add `docs/runbooks/keycloak-setup.md` with realm/client/scopes.
- [x] Map API endpoints to required scopes.
- [x] Add test matrix for valid/invalid token scenarios.

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
