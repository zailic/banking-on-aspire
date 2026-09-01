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
- M0, M1, and M2 are complete.
- The authenticated Banking.Web/BFF slice is implemented and integrates Users,
  Contacts, and the canonical Accounts gRPC service.
- Domain decomposition is in progress with Users, Contacts, and Accounts running
  independently. Transactions has not been implemented yet.
- The specs include boundaries, requirements, design, acceptance criteria,
  resolved questions, and implementation tasks.

## Next Implementation Backlog
1. Define the Transactions protobuf API and the first versioned account event.
2. Add a justified Dapr pub/sub component owned by the account-to-transactions flow.
3. Implement an idempotent Transactions read-model consumer and persistence model.
4. Expose transaction history through the BFF and replace the recent-activity placeholder.
5. Add automated event duplication, authorization, and end-to-end smoke coverage.
6. Keep the OpenSpec documents updated as each milestone is implemented.
