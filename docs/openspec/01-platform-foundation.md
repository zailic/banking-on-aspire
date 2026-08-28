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
- [x] Wire the AppHost so the Go services and Keycloak start from one entry point.
- [x] Validate startup and shutdown with Aspire commands and record the evidence.
- [x] Keep only infrastructure with a current runtime consumer in the AppHost graph.
- [x] Remove Redis and statestore from the default graph after the persistence
  experiment; reintroducing either requires a concrete owner and use case.

## Resolved Questions
- Redis and statestore are not default dependencies. They return only when a
  bounded context explicitly owns their persistence semantics.
- Dapr pub/sub is the next candidate, tied to a concrete account event and an
  idempotent Transactions consumer.

## Source Notes
This spec consolidates content from the milestone plan, sprint backlog, and ADR goals documents.
