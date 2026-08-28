# Banking on Aspire Plan: Aspire + Dapr + Radius + Keycloak

## Scope
This repository, banking-on-aspire, becomes a progressive lab for distributed systems practices, not only a demo app.

Naming note:
- The repository name is banking-on-aspire.
- The accounts Go module path is dev.local/banking-on-aspire/services/accounts.
- Default Keycloak realm URL in code is /realms/banking-on-aspire.

Primary technologies:
- Aspire (orchestration and local dev control plane)
- Dapr (service invocation, state, pub/sub, actors)
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

- `App/` contains the user-facing Blazor application. The initial frontend is a
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

1. Move the existing service unchanged into `services/accounts` and keep the
   AppHost resource, Dapr app ID, API behavior, and smoke flow stable.
2. Establish the root Go workspace and document ownership rules for `platform/`
   and `protos/`.
3. Introduce GAIP-aligned account contracts and reproducible code generation.
4. Extract genuinely reusable middleware into `platform/` only after a second
   consumer exists or an explicit cross-service contract requires it.
5. Add the Fluent UI Blazor frontend and a thin BFF boundary.
6. Add transaction history, contacts, and user profile services incrementally in
   the milestone order below; do not scaffold empty deployable services in advance.

## Milestones

### M0 - Baseline
- Capture a happy-path flow for account operations.
- Keep one smoke script/test as quality gate.
- Document current architecture and known gaps.

Done when:
- The baseline flow is reproducible and documented.

### M1 - AppHost foundation
- AppHost runs Go service + Dapr sidecar + Keycloak.
- Add Redis and Dapr statestore in this milestone (currently pending).
- Service waits for critical dependencies.

Done when:
- `aspire start` shows healthy resources and app can serve requests.
- Redis/statestore are visible in the resource graph.

### M2 - Identity hardening
- Create Keycloak realm, clients, and roles/scopes.
- Unify JWT validation middleware in service layer.
- Enforce authorization by capabilities (read/write scopes).

Done when:
- Unauthorized requests are denied and scoped tokens are enforced.

### M3 - Dapr deepening
- Use statestore for actor and domain state.
- Add at least one pub/sub event (`AccountOpened`, `FundsDeposited`).
- Add idempotency strategy for write operations.

Done when:
- Event-driven flow works and duplicate command handling is tested.

### M4 - Domain decomposition inspired by Bank of Anthos
- Split read-heavy transaction history into a separate service.
- Add contacts/beneficiaries bounded context.
- Keep current account command flow stable.

Done when:
- At least one extracted context has independent deployment/runtime.

Planned service locations:
- `services/accounts` - existing command/actor flow; established before M4.
- `services/transactions` - read model extracted first during M4.
- `services/contacts` - beneficiaries context extracted after transactions.
- `services/users` - optional profile data only; Keycloak remains the identity
  source of truth.

### M5 - Radius environments
- Define dev/test environments with reusable recipes.
- Map Aspire outputs to Radius deployment model.

Done when:
- Same app model can be deployed to at least two environments.

### M6 - Observability and resilience
- End-to-end tracing for one critical flow.
- Dashboards for latency/errors/auth failures.
- Run failure drills (Redis down, Keycloak unavailable, sidecar restart).

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
