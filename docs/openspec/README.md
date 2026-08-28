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
- Current state: baseline documentation has been migrated into an OpenSpec-style structure.
- The specs now include scope, requirements, design, acceptance criteria, open questions, and implementation tasks.

## Implementation Backlog
1. Validate the local Aspire startup path end to end.
2. Capture the current Keycloak/token flow as a reproducible runbook step.
3. Exercise the smoke flow with a real token and record the observed outcomes.
4. Keep the OpenSpec documents updated as each milestone is implemented.
