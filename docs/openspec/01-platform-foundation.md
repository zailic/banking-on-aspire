# Spec 01: Platform Foundation

## Summary
Establish a local learning platform where the Go service, Dapr actors, and Keycloak can be orchestrated from a single AppHost entry point.

## Problem
The repository already contains a functional service and supporting domain code, but the architecture needs a consistent operating model for local development and future decomposition.

## Requirements
- The project must be runnable from one orchestration entry point.
- The AppHost must expose the Go service and its dependencies in a readable resource graph.
- The platform must support local development without requiring a big-bang rewrite.
- Each milestone must include at least one verifiable artifact such as a test, runbook, or smoke script.

## Non-Goals
- Full microservice decomposition in the first milestone.
- Production-grade deployment automation in this phase.
- Replacement of the current Go service implementation.

## Design Overview
- Aspire acts as the local application orchestrator.
- The Go service remains the main business entry point.
- Dapr provides service invocation and actor support.
- Keycloak provides identity and role-based authorization.
- The repository keeps documentation and operational notes alongside the code for learning and onboarding.

## Acceptance Criteria
- The AppHost can start the service and its dependencies from a single command.
- The resource graph shows the relevant services and dependencies.
- The baseline flow can be documented and exercised with a repeatable smoke script.

## Implementation Tasks
- Wire the AppHost so the Go service and Keycloak can be started together from one entry point.
- Validate startup and shutdown with Aspire commands and record the evidence.
- Keep the platform notes aligned with the current runtime reality, including any external dependencies.
- Revisit Redis and statestore wiring once the actor runtime and global instance strategy are clearer.

## Open Questions
- Should Redis and statestore be added to the AppHost immediately, or should they remain external for the current milestone?
- Which Dapr capabilities should be enabled first once the local runtime is stable?

## Source Notes
This spec consolidates content from the milestone plan, sprint backlog, and ADR goals documents.
