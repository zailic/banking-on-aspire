# Banking on Aspire Plan: Aspire + Dapr + Radius + Keycloak

## Scope
This repository, banking-on-aspire, becomes a progressive lab for distributed systems practices, not only a demo app.

Naming note:
- The repository name is banking-on-aspire.
- The accounts Go module path is dev.local/banking-on-aspire/services/accounts.
- The preserved actor implementation uses dev.local/banking-on-aspire/services/accounts-legacy.
- Default Keycloak realm URL in code is /realms/banking-on-aspire.

Primary technologies:
- Aspire (orchestration and local dev control plane)
- Dapr (service invocation by default; state, pub/sub, and actors added only for
  use cases that justify them)
- Keycloak (OIDC identity provider)
- Radius (environment modeling and platform recipes)
- OpenTelemetry (traces, metrics, logs)

## Migration Strategy
Use an incremental strangler approach:
1. Keep current behavior stable.
2. Add one capability at a time.
3. Keep the system runnable after each increment.

Porting principle from Bank of Anthos:
- Reuse architecture ideas (bounded contexts, read/write separation, operational practices).
- Do not copy cloud-vendor-specific implementation details.
- Keep Aspire + Dapr + Keycloak + Radius as the platform spine.

## Target Repository Structure

The repository evolves toward a polyglot monorepo with explicit application,
platform, contract, and service boundaries:

- `frontend/` contains user-facing applications. The initial frontend is a
  .NET 10 Blazor Web App using Fluent UI Blazor v5 and an interactive server
  render mode.
- `platform/` contains reusable platform code such as authentication middleware,
  observability helpers, shared service middleware, contracts, and generated code.
  Platform packages must remain dependency-light and must not contain business
  workflows owned by a service.
- `protos/` is the source of truth for versioned protobuf APIs. Definitions follow
  Google AIP resource-oriented conventions and generated artifacts are never edited
  manually.
- `services/` contains independently buildable Go modules. Each service owns its
  entrypoints, domain logic, transport adapters, tests, and module dependencies.

The target service boundaries remain those documented in
`docs/bank-of-anthos-mapping.md`: accounts, transaction history, contacts, and a
lightweight user profile boundary backed by Keycloak identity.

### Structural Migration Sequence

Completed steps:

1. Moved the original service into `services/accounts-legacy` and retained it as
   an explicit Aspire/Dapr learning resource.
2. Established the root Go workspace and documented ownership rules for `platform/`
   and `protos/`.
3. Introduced GAIP-aligned Users and Contacts contracts with reproducible Buf
   generation for Go and C#.
4. Extracted shared Keycloak verification and descriptor-driven RBAC into
   `platform/` after Users and Contacts became consumers.
5. Added independently runnable Users and Contacts services backed by PostgreSQL.
6. Added the GAIP-aligned Accounts gRPC service backed by PostgreSQL, with
   descriptor-driven RBAC and `sub`-based ownership.

Next steps:

1. Add the Fluent UI Blazor frontend under `frontend/Banking.Web` and a thin BFF
   boundary that forwards the caller's access token.
2. Connect Banking.Web to the Accounts gRPC service and render real balances.
3. Add the Transactions read model and the first justified Dapr pub/sub flow.

## Milestones

### M0 - Baseline (Completed)
- Capture a happy-path flow for account operations.
- Keep one smoke script/test as quality gate.
- Document current architecture and known gaps.

Done when:
- The baseline flow is reproducible and documented.

### M1 - AppHost foundation (Completed)
- AppHost runs Go service + Dapr sidecar + Keycloak.
- Service waits for critical dependencies.

Done when:
- `aspire start` shows healthy resources and app can serve requests.
- Only infrastructure with a current consumer is visible in the resource graph.

### M2 - Identity hardening (Completed 2026-08-22)
- Created the Keycloak realm, client roles, and `BankingUser` group.
- Unified OIDC/JWT validation and claims propagation in `platform/auth/keycloak`.
- Declared required permissions on protobuf RPCs and enforced them through a
  fail-closed gRPC interceptor.
- Enforced resource ownership in Users and Contacts using the immutable Keycloak
  `sub` claim.
- Added unit, contract, and `make smoke-auth` verification paths.

Done when:
- Missing or invalid tokens return `Unauthenticated`.
- Missing permissions and cross-user access return `PermissionDenied`.
- Auth configuration and operational setup are documented and repeatable.

### M3 - Dapr deepening
- Evaluate state store, actors, and pub/sub against a concrete service use case.
- Add a state store only if the selected use case requires it.
- Add at least one pub/sub event (`AccountOpened`, `FundsDeposited`).
- Add idempotency strategy for write operations.

Done when:
- The selected Dapr building block has an explicit owner and persistence model.
- Event-driven flow works and duplicate command handling is tested.

### M4 - Domain decomposition inspired by Bank of Anthos (In progress)
- Split read-heavy transaction history into a separate service.
- Add contacts/beneficiaries bounded context.
- Keep current account command flow stable.

Done when:
- At least one extracted context has independent deployment/runtime.

Planned service locations:
- `services/accounts` - existing command/actor flow; established before M4.
- `services/transactions` - read model extracted first during M4.
- `services/contacts` - independently runnable beneficiaries context; implemented.
- `services/users` - independently runnable application profile context linked
  through the Keycloak `sub` claim; implemented. Keycloak remains the identity
  and credential source of truth.

### M5 - Radius environments
- Define dev/test environments with reusable recipes.
- Map Aspire outputs to Radius deployment model.

Done when:
- Same app model can be deployed to at least two environments.

### M6 - Observability and resilience
- End-to-end tracing for one critical flow.
- Dashboards for latency/errors/auth failures.
- Run failure drills (PostgreSQL down, Keycloak unavailable, sidecar restart).

Done when:
- Alerts and troubleshooting steps are documented and validated.

## Suggested repo conventions
- `docs/adr/` for architecture decisions
- `docs/runbooks/` for operational playbooks
- `docs/learning-journal.md` for lessons learned by milestone
- `scripts/` for reproducible smoke tests

## Evidence per milestone
- At least one test or smoke script updated.
- One short runbook section with commands and expected outcomes.
- One "what I learned" note in `docs/learning-journal.md`.

## Guardrails
- Every milestone adds at least one automated check.
- No milestone closes without operational notes.
- Keep all secrets out of source files.
