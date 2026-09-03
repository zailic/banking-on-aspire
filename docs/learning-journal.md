# Learning Journal

## 2026-08-19 - Contacts API design

- Translated the Bank of Anthos contacts list/create behavior into a versioned,
  resource-oriented protobuf contract under `protos/banking/contacts/v1`.
- Replaced the source service's `is_external` flag with an explicit destination
  union: an internal account resource reference or external banking coordinates.
- Chose `users/{user}/contacts/{contact}` as the canonical resource hierarchy and
  kept ownership, self-reference prevention, and uniqueness as service invariants.
- Added a Buf module with the Google APIs dependency. Local `buf lint` remains a
  setup task because Buf/protoc are not installed in the current environment.

## 2026-08-18 - Structural migration: accounts service

- Moved the existing Go application into the independent
  `services/accounts` module without changing its HTTP API, Dapr app ID, or Aspire
  resource name.
- Added a root `go.work` so future Go services can remain independent modules while
  sharing one local development workspace.
- Established `frontend`, `platform`, `protos`, and `services` ownership boundaries in
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

## Contacts API - generated contracts

Date: 2026-08-20
Milestone: Contacts service foundation
Change summary:
- Added pinned Buf generation for Go and C# from the Contacts GAIP API.
- Added build boundaries for the generated Go package and .NET contracts library.
- Added the platform Go module to the repository workspace.

What worked:
- Buf format, lint, generation, and image build completed successfully.
- Remote plugins make generation reproducible without local protoc plugins.

What failed:
- Buf STANDARD initially rejected GAIP methods that return a resource directly or
  `google.protobuf.Empty`.

Why it failed:
- Buf's default RPC response naming rules are stricter than the GAIP standard
  method response shapes.

Decisions taken:
- Keep GAIP response semantics and document the two narrow Buf lint exceptions.
- Treat `protos` as the source of truth and `platform/gen` as generated output.

Next step:
- Scaffold the Go Contacts service against the generated server interface.

## Contacts service - first vertical slice

Date: 2026-08-20
Milestone: Contacts service foundation
Change summary:
- Added an independently buildable Go module for the Contacts service.
- Implemented the generated gRPC server interface with an in-memory repository.
- Added resource validation, pagination, server-managed timestamps, and optimistic
  concurrency through etags.
- Added gRPC transport tests using an in-memory listener.

What worked:
- The generated contract cleanly separates the API from service implementation.
- A partial update can be exercised through a field mask without resending
  unchanged fields.

Decisions taken:
- Keep storage in memory for this slice and replace it behind the service boundary
  with Dapr state in the next persistence step.
- Keep authentication and owner-claim enforcement for the AppHost integration
  slice, where Keycloak configuration is available.

Next step:
- Introduce a repository interface and Dapr state implementation, then wire the
  Contacts process into Aspire with its Dapr sidecar and health check.

## Contacts service - Dapr repository

Date: 2026-08-20
Milestone: Contacts persistence
Change summary:
- Extracted the Contacts persistence boundary into a repository interface.
- Kept a concurrency-safe memory implementation for isolated tests.
- Added a Dapr implementation backed by the `statestore` component.
- Wired the runtime process to the Dapr repository.

What worked:
- Storing one aggregate per user supports list operations without relying on the
  optional Dapr state query API.
- State-store ETags and first-write concurrency provide a retryable optimistic
  concurrency boundary for aggregate writes.
- Repository fakes make conflict and retry behavior deterministic in tests.

Decisions taken:
- Keep contact ETags distinct from Dapr document ETags: the former are part of the
  public API, while the latter protect the persisted aggregate.
- Return stable gRPC status codes from the service and keep Dapr-specific errors
  behind the repository boundary.

Next step:
- Add the Contacts executable and Dapr sidecar to Aspire, reference the existing
  `statestore`, and validate persistence with a local restart smoke test.

## Contacts service - Aspire and persistence validation

Date: 2026-08-20
Milestone: Contacts persistence
Change summary:
- Added Redis and the generated Dapr `statestore` component to the AppHost model.
- Connected both Go services to the state store and added a gRPC Dapr sidecar for
  Contacts.
- Added a reusable gRPC smoke client and validated create, restart, get, and
  cleanup against the running Aspire graph.

What worked:
- Dapr loaded `statestore` as `state.redis/v1` and the Contacts process discovered
  its Aspire-provided Dapr endpoints.
- The contact and its public ETag survived a Contacts resource restart.
- Every declared application and infrastructure resource reached Healthy.

What failed:
- Contacts and accounts initially both listened on host port 8080, so accounts
  failed with `bind: address already in use`.

Why it failed:
- Host-native executable resources share the host network even though each has a
  separate Dapr sidecar.

Decisions taken:
- Keep accounts on 8080 and configure Contacts through `CONTACTS_PORT=8083`.
- Keep external Contacts ports Aspire-managed and discover them with
  `aspire describe`.

Next step:
- Add Keycloak authentication and owner-claim enforcement to the Contacts gRPC
  service, then expose it to the frontend through a BFF or gRPC-aware client.

## Contacts service - PostgreSQL repository

Date: 2026-08-20
Milestone: Contacts persistence refinement
Change summary:
- Replaced the Contacts Dapr state repository with a PostgreSQL repository using
  pgxpool.
- Added a relational schema with a composite primary key, destination checks,
  timestamps, and ETag-based optimistic concurrency.
- Added PostgreSQL and the `contactsdb` database to the Aspire graph.
- Expanded the runtime smoke client to cover create, list, update, get, restart,
  and delete.

What worked:
- The repository interface allowed the persistence implementation to change
  without changing the generated gRPC contract.
- Aspire injected host, credentials, database name, and PostgreSQL URI from the
  database reference.
- The complete CRUD flow and persistence across a Contacts restart succeeded.

What failed:
- The first pgx connection attempt used `ConnectionStrings__contactsdb`, which is
  formatted as an Npgsql semicolon-delimited connection string.

Why it failed:
- pgx expects a PostgreSQL URI or keyword/value connection string. Aspire also
  exposes `CONTACTSDB_URI`, which is the correct connection property for pgx.

Decisions taken:
- Use PostgreSQL as the Contacts source of truth.
- Retain Redis/Dapr state for accounts actors and key-value use cases.
- Keep the Contacts Dapr sidecar for service invocation and telemetry, not storage.

Next step:
- Move schema migration into a dedicated startup/migration resource before the
  service needs multiple replicas, then add Keycloak authentication and ownership
  enforcement.

## Shared users database and dedicated Contacts migration

Date: 2026-08-20
Milestone: Users/Contacts persistence boundary
Change summary:
- Renamed the Aspire database resource from `contactsdb` to `usersdb` to model the
  shared Users/Contacts bounded context used by Bank of Anthos.
- Added a one-shot `contacts-migrations` process and made Contacts wait for its
  successful completion.
- Removed schema mutation from normal Contacts service startup.

Decisions taken:
- Users and Contacts may share the PostgreSQL database, while each service keeps
  ownership of its own tables and migrations.
- The current accounts actor service remains a legacy Aspire/Dapr learning asset;
  it does not define the persistence boundary of the future Accounts service.
- Renaming that legacy service is intentionally deferred to a separate change.

Next step:
- Introduce the Users service and its independently versioned migrations in
  `usersdb`, then decide whether Contacts should enforce a foreign key to Users.

## Remove the unused default state store

Date: 2026-08-20
Milestone: AppHost resource cleanup
Change summary:
- Removed Redis and the Dapr `statestore` component from the default Aspire graph.
- Removed the generated local statestore component definition.
- Reclassified the existing actor-based Accounts flow as learning code rather
  than the persistence design for the future Accounts service.

Decision taken:
- Add a Dapr state store again only when a concrete bounded context requires its
  key/value or actor-state semantics.

## Users service - Keycloak-linked local profiles

Date: 2026-08-20
Milestone: First Users increment
Change summary:
- Added a GAIP resource contract for `users/{user}` with `GetUser` and the custom
  `GetOrCreateCurrentUser` identity-resolution method.
- Added a PostgreSQL repository and a dedicated `users-migrations` startup
  resource targeting the shared `usersdb` database.
- Added gRPC bearer-token validation against the Keycloak issuer and ownership
  enforcement based on the immutable `sub` claim.
- Added the Users service and Dapr sidecar to the Aspire graph.

Decisions taken:
- Keycloak owns authentication, credentials, sessions, MFA, and roles.
- Users owns only the application profile and its lifecycle status.
- The first increment has no Keycloak Admin API permissions, delete operation,
  Contacts foreign key, or cross-user administrative reads.

Next step:
- Add an authenticated runtime smoke client, then decide how the frontend/BFF
  obtains and forwards the caller's access token to Users and Contacts.

Runtime validation:
- Authenticated `local-dev` through the Keycloak password grant and verified the
  expected `sub`, username, and `banking-on-aspire-app` audience.
- Confirmed `resolve -> get -> resolve` returns the same `users/{user}` profile.
- Changed identity synchronization to preserve `update_time` and ETag when the
  projected Keycloak claims have not changed.
- The smoke client reads its token from `USERS_SMOKE_TOKEN` so bearer tokens are
  not exposed in process arguments.

## Shared Keycloak token verification

Date: 2026-08-20
Milestone: Platform authentication extraction
Change summary:
- Extracted OpenID Connect discovery, JWT verification, Keycloak claims, and
  client-role checks into `platform/auth/keycloak`.
- Moved the Users gRPC unary interceptor and authenticated claims context into
  the same platform package.
- Updated both the legacy Accounts HTTP middleware and Users gRPC service to use
  the shared verifier, then removed their duplicate implementations.

Boundary decision:
- The platform package validates identity and exposes normalized claims; it does
  not contain service authorization policy or call the Keycloak Admin API.
- Each service remains responsible for deciding what the authenticated caller
  may do with its domain resources.

## Contacts ownership enforcement

Date: 2026-08-21
Milestone: Identity hardening
Change summary:
- Contacts now resolves the authenticated Keycloak `sub` to the canonical active
  `users/{user}` profile before accessing contact persistence.
- List, get, create, update, and delete reject resource names owned by another
  identity with `PermissionDenied`.
- Missing claims return `Unauthenticated`; a token without an active application
  profile cannot access Contacts.

Boundary decision:
- Ownership remains domain authorization in Contacts and is separate from the
  permission policy declared on protobuf RPCs.
- The resolver interface hides the current shared-PostgreSQL lookup so it can be
  replaced later by a Users service call without changing ownership rules.

## Automated authentication smoke gate

Date: 2026-08-21
Milestone: Identity hardening
Change summary:
- Added `make smoke-auth` to start or reuse Aspire, wait for Keycloak, Users, and
  Contacts, and discover their runtime endpoints without fixed ports.
- The gate obtains a real Keycloak token and verifies successful owned-resource
  access, missing and malformed tokens, cross-user denial, and the descriptor
  interceptor's missing-permission behavior.
- Cleanup stops Aspire only when the gate started it.

First run evidence:
- All required resources became healthy and cleanup completed successfully.
- The gate detected that the current `.env.smoke` user lacks the Contacts
  permissions, proving that the runtime preflight catches realm drift before a
  misleading happy-path failure.

Secret handling follow-up:
- Smoke credentials now live in the AppHost-scoped Aspire secret store under
  `SmokeAuth:Keycloak:*`; the gate no longer sources `.env.smoke`.
- Missing secret values fail before Aspire startup and include the exact
  `aspire secret set` command needed for local setup.

## M2 identity hardening closure

Date: 2026-08-22
Milestone: M2 completed
Evidence:
- Keycloak-backed authentication is shared by Users and Contacts.
- Protobuf RPCs declare all-of `required_permissions`; missing policies fail closed.
- Users and Contacts enforce ownership from the immutable Keycloak `sub` claim.
- Contract, interceptor, service, and cross-user tests pass through `make test`.
- `make smoke-auth` owns Aspire lifecycle, discovers endpoints, uses the Aspire
  secret store, and verifies the runtime authentication matrix.
- ADR 0003 records the boundary between RPC permissions and domain ownership.

Decision:
- Close M2. Local realm membership may still cause the smoke preflight to report
  missing roles, but this is detected configuration drift rather than missing
  authorization implementation.

Next step:
- Build one thin frontend/BFF vertical slice that authenticates with Keycloak and
  forwards the caller token to Users and Contacts.

## First frontend/BFF vertical slice

Date: 2026-08-22
Milestone: M3 started
Change summary:
- Added the .NET 10 interactive-server `Banking.Web` application and shared
  Aspire ServiceDefaults, with Fluent UI Blazor v5 RC pinned explicitly.
- Added confidential OIDC login and cookie sessions. Access tokens remain in the
  server authentication ticket and are forwarded by the BFF to Users and Contacts.
- The authenticated overview resolves `GetOrCreateCurrentUser`, calls
  `ListContacts` for that canonical user resource, and renders the profile and
  beneficiaries.
- Wired the app into Aspire with explicit Keycloak and gRPC endpoint expressions,
  startup dependencies, a stable HTTPS callback origin, and an Aspire secret
  parameter for the OIDC client secret.

Validation evidence:
- `dotnet build frontend/Banking.Web/Banking.Web.csproj --no-restore` passed with
  zero warnings and zero errors.
- `aspire wait banking-web` reported the resource healthy in the complete graph.
- The updated realm configuration was subsequently applied to the existing
  Keycloak data volume, enabling the browser OIDC callback.

## Beneficiary creation workflow

Date: 2026-08-22
Milestone: M3 frontend/BFF slice
Change summary:
- Added a validated Banking.Web form for internal and external beneficiaries.
- The BFF re-resolves the authenticated user, derives the contact parent, creates
  a collision-resistant contact identifier, and forwards the access token to
  `CreateContact`; neither ownership field is accepted from the browser.
- Successful creation refreshes the beneficiary list, while permission,
  validation, and conflict failures are translated into actionable UI messages.

Validation evidence:
- Banking.Web builds with zero warnings and zero errors.
- The full `make test` suite passes, including Contacts create validation and
  ownership behavior.
- `aspire wait banking-web` reports the updated application healthy.

Layout follow-up:
- Replaced the inline beneficiary form with a Fluent UI v5 drawer so the form
  remains usable on short and narrow viewports.
- Adopted `FluentProviders`, `FluentLayout`, v5 navigation, Fluent form fields,
  and CSS design tokens. The main content and drawer now have
  explicit, independent scrolling regions.

## Server-side access-token renewal

Date: 2026-08-22
Milestone: M3 frontend/BFF hardening
Problem:
- The Keycloak access token expires after five minutes, but the authentication
  cookie and SSO session remain valid. Long-lived Blazor circuits therefore
  forwarded an expired bearer token and received `Unauthenticated` from gRPC.

Change summary:
- Added a circuit-scoped access-token provider that reads the saved OIDC ticket,
  refreshes the token one minute before expiry, and serializes concurrent refresh
  attempts.
- Protected gRPC operations retry once with a forced refresh when a service
  returns `Unauthenticated`, covering expiry-boundary and clock-skew races.
- If the refresh session itself has expired, the UI offers a direct OIDC sign-in
  action instead of requiring a manual logout first.

Validation evidence:
- Banking.Web builds with zero warnings and zero errors.
- Banking.Web becomes healthy in Aspire.
- A real Keycloak password grant followed by a refresh-token grant succeeded and
  returned a renewed access token with the configured 300-second lifetime.

## Authenticated banking workspace checkpoint

Date: 2026-08-28
Milestone: Frontend/BFF slice complete; M3/M4 handoff

Current state:
- Banking.Web is an authenticated interactive-server Fluent UI application with
  a thin BFF that forwards and renews the caller's Keycloak access token.
- The overview integrates the canonical Users, Contacts, and Accounts gRPC
  services and renders profiles, beneficiaries, accounts, and real balances.
- Beneficiary and account creation are available through server-controlled
  workflows; ownership fields are not trusted from browser input.
- Users, Contacts, and Accounts are independently runnable Go services backed by
  PostgreSQL. The actor-based Accounts implementation is retained separately as
  `accounts-legacy` learning code.
- The recent-activity panel remains an explicit placeholder because Transactions
  and the account event stream do not exist yet.

Development-loop clarification:
- Aspire default watch reevaluates the AppHost and supports the C# project
  resource loop; it does not add source watching to Go `AddExecutable` resources.
- Banking.Web can use dynamic loopback ports internally while retaining
  `https://localhost:7443` as its stable browser and OIDC origin.

Next increment:
- Define a versioned account event and Transactions API, add the first justified
  Dapr pub/sub flow, implement an idempotent Transactions read model, and surface
  it through Banking.Web recent activity.

## SendPayment domain foundation

Date: 2026-08-28
Milestone: M3 Dapr deepening started

Decision:
- Replaced cash withdrawal as the canonical next use case with `SendPayment`, so
  the existing beneficiary context participates in a real business workflow.
- Accounts owns payment execution and balances. Transactions will consume events
  only to build queryable history.

Change summary:
- Added the `Payment` resource and idempotent `SendPayment` RPC protected by the
  `payments.send` client role.
- Added atomic PostgreSQL persistence for the source debit, payment record, and
  `PaymentSent` outbox record. Internal beneficiaries are credited atomically.
- Added validation for ownership, beneficiary existence, account status,
  currency, available funds, reference length, and idempotency-key reuse.
- Added the new role to the committed Keycloak realm and BankingUser group.
- Made Accounts migrations wait for Contacts migrations because Payments owns a
  foreign key to the saved beneficiary selected by the command.

Validation evidence:
- `make check` passes across protobuf validation and all Go workspace tests.
- Banking.Contracts and Banking.Web build with zero warnings and zero errors.

Next step:
- Add the outbox dispatcher, Dapr pub/sub component, versioned event contract,
  and idempotent Transactions consumer before exposing SendPayment in the UI.

## Payment event and Transactions read model

Date: 2026-08-28
Milestone: M3 event-driven backend slice

Change summary:
- Added the versioned `PaymentSentEvent` integration contract and an Accounts
  outbox dispatcher that publishes only committed rows.
- Added Redis as the concrete Dapr `pubsub` broker and the `payments.sent` topic.
- Added an HTTP Dapr subscriber that projects debit and internal-credit entries
  idempotently into the independently owned `transactionsdb` database.
- Added the authenticated, owner-scoped `ListTransactions` gRPC API protected by
  `transactions.read`.

Validation evidence:
- `make check` passes, including subscriber, pagination, ownership, and protobuf
  authorization-policy tests.
- Accounts, Transactions, and Banking.Web become healthy through `aspire wait`.
- The Transactions migration exits with code 0 and Dapr reports the active
  `payments.sent` subscription through `pubsub`.

Next step:
- Add Send Payment and recent transaction activity to Banking.Web.

## Send Payment UI and recent activity

Date: 2026-08-28
Milestone: M3/M4 vertical slice complete

Change summary:
- Added a Fluent UI payment dialog that selects an open source account and a
  saved beneficiary, validates the amount, derives the account currency, and
  generates an idempotency key inside the BFF.
- Connected Banking.Web to the Transactions gRPC API and replaced the activity
  placeholder with debit/credit rows, amounts, references, and local timestamps.
- A successful payment reloads balances and transaction history together.

Validation evidence:
- Banking.Web builds with zero warnings and zero errors.
- `make check` passes.
- Banking.Web, Accounts, and Transactions all become healthy through Aspire.

## Idempotent demo account funding

Date: 2026-08-29
Milestone: M3/M4 end-to-end flow enablement

Decision:
- Added a clearly labeled learning-only cash-in command instead of seeding or
  editing balances directly. The command remains inside Accounts because that
  service owns the balance invariant.

Change summary:
- Added `DepositFunds`, protected by `accounts.deposit`, with ownership,
  currency, account-state, positive-amount, and idempotency validation.
- Added atomic deposit persistence and a `FundsDepositedEvent` outbox record.
- Transactions now consumes `funds.deposited` and projects a credit entry.
- Banking.Web exposes an `Add funds` dialog and refreshes balances plus recent
  activity after completion.

Validation evidence:
- `make check` passes, including deposit idempotency/ownership tests and the new
  event subscriber test.
- Banking.Web builds with zero warnings and zero errors.
- Accounts migration exits with code 0; Accounts, Transactions, and Banking.Web
  become healthy; Dapr reports both `payments.sent` and `funds.deposited` topics.

## Internal beneficiary account validation

Date: 2026-08-30
Milestone: Contacts boundary hardening

Change summary:
- Contacts now calls `AccountsService.GetAccount` before persisting a new
  internal beneficiary or a changed internal destination.
- The caller's bearer token is forwarded, so Accounts ownership and RBAC remain
  authoritative; no privileged service bypass was introduced.
- AppHost models the `contacts → accounts` runtime dependency explicitly and
  waits for Accounts before starting Contacts.
- External beneficiaries and display-name-only updates remain independent of
  Accounts availability.

Behavioral consequence:
- With the current owner-scoped `GetAccount`, internal beneficiaries are limited
  to accounts readable by the authenticated user. Supporting another user's
  account safely would require a purpose-built destination-resolution API that
  reveals less information than `GetAccount`.

## Fluent UI v5 application shell

Date: 2026-09-02
Milestone: Frontend shell modernization

Change summary:
- Replaced the hand-written application grid in `MainLayout` with Fluent UI v5
  `FluentLayout` and `FluentLayoutItem` components while preserving the compact
  72-pixel navigation rail and independently scrolling content region.
- Replaced the navigation `NavLink` markup with vertical `FluentAppBar` and
  `FluentAppBarItem` components, including distinct regular and active icons.
- Kept primary navigation and the bottom profile action in separate app bars so
  the Fluent overflow algorithm does not hide the profile action.
- Allowed the header layout item to overflow above the content stacking context,
  so the custom user-menu popover is not clipped by FluentLayout's default
  `overflow: hidden` rule.

What failed and why:
- `LayoutArea.Navigation` renders as the CSS grid area `nav`, not `navigation`.
  Using the enum name as the CSS area name left the item unassigned and caused
  CSS Grid to place the navigation rail in the bottom-right corner.
- FluentLayout supplies opinionated header padding, brand background, overflow,
  and grid geometry. The application shell must override those defaults at the
  layout-item boundary when retaining its existing visual design.
- A fixed `height` on a child of the vertical navigation flex container can still
  shrink because `flex-shrink` defaults to `1`; fixed-height rail sections need an
  explicit non-shrinking flex basis.

Validation evidence:
- Banking.Web builds with zero warnings and zero errors after the layout and
  navigation migration.

Next step:
- Add a focused visual/component regression check for the application shell so
  header width, navigation placement, overflow menus, and fixed rail sections
  are verified together.

## End-to-end observability and resilience drills

Date: 2026-09-02
Milestone: M6 observability and resilience increment

Change summary:
- Enabled gRPC-client tracing in Banking.Web and added explicit BFF activities
  for `DepositFunds` and `SendPayment`, including a visible token-refresh retry event.
- Added a shared Go OpenTelemetry bootstrap, gRPC server instrumentation,
  instrumented Accounts-to-Dapr publishing, and Transactions HTTP subscriber tracing.
- Opted Accounts and Transactions into Aspire OTLP export with
  `WithOtlpExporter(OtlpProtocol.Grpc)`.
- Persisted W3C `traceparent` and `tracestate` in the Accounts outbox so an
  asynchronous publish remains correlated with the original command.
- Added a traceable `accountsmoke` client and an automated transient publish
  retry test.

Validation evidence:
- Deposit trace `3bd271ba471d07154e8e0b5574112b49` contained Accounts,
  Accounts Dapr, Transactions Dapr, and Transactions subscriber spans.
- SendPayment trace `94a9344feb0c6fca6cb516a11aab260b` contained the same
  correlated service chain.
- With the Accounts Dapr sidecar unavailable, deposit trace
  `75dff2c307f9f288856883d4fa561ddf` completed after recovery and preserved
  its trace ID across delayed delivery.
- With PostgreSQL stopped, Accounts returned `DeadlineExceeded`; after recovery,
  the existing Accounts process reconnected and listed all three demo accounts.
- With Keycloak stopped, token acquisition failed while calls using the already
  issued JWT continued through cached verification; token issuance recovered.
- `make check` and the Banking.Web build pass with zero test/build errors.

What failed and why:
- Aspire did not inject OTLP variables into Go `AddExecutable` resources until
  they explicitly opted in with `WithOtlpExporter`.
- The first Go resource merge mixed semantic-convention schemas `1.40.0` and
  `1.41.0`; OpenTelemetry refused the conflict. Aligning on `1.41.0` fixed startup.
- `OpenTelemetry.Instrumentation.GrpcNetClient` remains prerelease; the compatible
  `1.15.1-beta.1` version is pinned alongside the stable core/exporter packages.

Decision:
- Treat SendPayment as the canonical withdrawal/outgoing-money flow. Retain the
  same idempotency key when retrying an ambiguous money command.

Next step:
- Define deployment-specific dashboards and alerts after selecting a durable
  telemetry backend and target environment.
