# ADR 0001: Learning Lab Goals and Constraints

Status: Accepted
Date: 2026-07-30

## Context

The repository starts from a Go bank account service integrated with Dapr actors and orchestrated by Aspire AppHost.
The goal is to evolve it into a practical knowledge lab for distributed systems using Aspire, Dapr, Radius, and Keycloak.

## Decision

Adopt an incremental migration strategy with milestone-based delivery.

Core platform choices:
- Aspire for local orchestration and resource graph visibility.
- Dapr for service invocation, actors, state, and pub/sub.
- Keycloak for identity, authentication, and scope-based authorization.
- Radius for environment modeling and deployment recipes.
- OpenTelemetry for traces, metrics, and logs.

## Constraints

- Keep the system runnable after each milestone.
- Avoid big-bang decomposition.
- No secrets in source control.
- Every milestone must produce at least one verifiable artifact (test, runbook, or journal entry).

## Consequences

Positive:
- Lower risk and faster feedback loops.
- Better observability of architectural trade-offs.
- Reusable runbooks and ADRs for future projects.

Negative:
- More upfront documentation work.
- Temporary hybrid architecture while decomposition is in progress.

## Follow-up

- Create runbook for local startup and baseline verification.
- Add smoke script and identity setup guide.
- Execute Sprint 01 stories in order.
