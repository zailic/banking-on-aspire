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

Current delivery state:

1. `frontend/Banking.Web` provides the .NET 10 interactive-server Fluent UI
   frontend and thin BFF boundary.
2. The BFF authenticates with Keycloak, renews server-side access tokens, and
   forwards the caller token to Users, Contacts, Accounts, and Transactions over gRPC.
3. The overview renders the current profile, beneficiaries, accounts, and real
   balances. It also supports beneficiary and account creation.
4. Users, Contacts, Accounts, and Transactions are independently runnable Go services backed
   by PostgreSQL and protected by descriptor-declared permissions plus
   `sub`-based ownership checks.

Completed backend event increment:

1. Added the versioned `PaymentSentEvent` contract and Accounts outbox dispatcher.
2. Added Redis-backed Dapr pub/sub on `payments.sent`.
3. Added the idempotent Transactions projection in its own PostgreSQL database and an owner-scoped gRPC API.

Completed UI increment:

1. Added the Banking.Web `Send payment` dialog with account, beneficiary,
   amount, currency, reference, and automatic idempotency key handling.
2. Replaced the recent-activity placeholder with the owner-scoped Transactions API.

Next increment:

1. Add focused component/BFF tests for the payment UI.
2. Refine activity filters, pagination, and transaction details.

Cash-in enablement:

- Added the idempotent canonical `DepositFunds` operation so empty accounts can
  exercise the payment flow without direct database edits.
- Added `FundsDepositedEvent`, the `funds.deposited` Dapr topic, Transactions
  projection, and the Banking.Web `Add funds` dialog.

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

### M3 - Dapr deepening (In progress)
- Evaluate state store, actors, and pub/sub against a concrete service use case.
- Add a state store only if the selected use case requires it.
- Publish and consume `PaymentSentEvent` through Redis-backed Dapr pub/sub (completed).
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
- `services/accounts` - canonical GAIP-aligned account lifecycle service backed
  by PostgreSQL; implemented and integrated with Banking.Web.
- `services/transactions` - PostgreSQL read model consuming `PaymentSentEvent`; implemented.
- `services/contacts` - independently runnable beneficiaries context; implemented.
- `services/users` - independently runnable application profile context linked
  through the Keycloak `sub` claim; implemented. Keycloak remains the identity
  and credential source of truth.
- `services/accounts-legacy` - preserved actor-based implementation used only as
  learning/reference code; it is not the canonical account boundary.

### M5 - Radius environments
- Define dev/test environments with reusable recipes.
- Map Aspire outputs to Radius deployment model.

Done when:
- Same app model can be deployed to at least two environments.

### M6 - Observability and resilience
- End-to-end tracing for one critical flow.
- Dashboards for latency/errors/auth failures.
- Run failure drills (PostgreSQL down, Keycloak unavailable, sidecar restart).

Progress (2026-09-02):
- Completed end-to-end W3C trace propagation for DepositFunds and SendPayment
  across Banking.Web, Accounts, the durable outbox, Dapr pub/sub, and Transactions.
- Added automated outbox retry/trace-context coverage and completed local
  PostgreSQL, Keycloak, and Dapr recovery drills.
- Added the observability and failure-drill runbook. Alert definitions and a
  persistent production dashboard remain future deployment work.

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
