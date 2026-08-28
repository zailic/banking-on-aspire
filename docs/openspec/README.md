# OpenSpec: Banking on Aspire

## Purpose
This document consolidates the current planning, onboarding, and operational notes into an OpenSpec-style structure so the project can evolve with clear requirements, decisions, and acceptance criteria.

## Scope
- Platform foundation with Aspire, Dapr, Keycloak, and Go services
- Identity and authorization flows
- Local development and smoke validation
- Milestone-based delivery for future work

## Specs
- [01-platform-foundation.md](01-platform-foundation.md)
- [02-identity-and-authorization.md](02-identity-and-authorization.md)
- [03-local-development-and-smoke.md](03-local-development-and-smoke.md)

## Status
- M0, M1, and M2 are complete; domain decomposition is in progress with Users
  and Contacts running independently.
- The specs include boundaries, requirements, design, acceptance criteria,
  resolved questions, and implementation tasks.

## Next Implementation Backlog
1. Add the first authenticated frontend/BFF vertical slice.
2. Modernize Accounts around protobuf contracts, `sub` ownership, and explicit persistence.
3. Introduce an account event and idempotent Transactions read-model consumer.
4. Keep the OpenSpec documents updated as each milestone is implemented.
