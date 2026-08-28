# ADR 0002: Adopt a polyglot monorepo structure

## Status

Accepted

## Context

The initial repository placed one Go service directly at the repository root. The
learning plan now adds a Blazor frontend, shared platform capabilities, versioned
protobuf contracts, and independently deployable services inspired by Bank of
Anthos. Keeping all source at the root would blur ownership and make later service
extractions disruptive.

The migration must preserve the existing account flow and keep the Aspire AppHost
runnable after every increment.

## Decision

Use four top-level source boundaries:

- `App/` for user-facing applications, initially `Banking.Web`.
- `platform/` for reusable middleware, observability, shared contracts, and generated
  artifacts. Business behavior remains in the owning service.
- `protos/` for versioned protobuf source definitions designed around Google AIP
  resource-oriented conventions.
- `services/` for independently buildable Go modules.

The existing Go module becomes `services/accounts`. A root `go.work` coordinates
local development without coupling service release versions. The existing Aspire
resource name and Dapr app ID remain stable during this structural migration.

New service directories are introduced only when their bounded context becomes
executable and testable. Empty deployable service skeletons are avoided.

## Consequences

- AppHost working directories and developer commands must use service-relative paths.
- Go imports include the owning service module path until code is deliberately
  extracted into a platform module.
- Cross-service APIs must originate from `protos/`; generated files are outputs,
  not hand-maintained contracts.
- A future platform Go module must not import any service module.
- The first migration is structural and does not claim completion of the M4 domain
  decomposition milestone.
