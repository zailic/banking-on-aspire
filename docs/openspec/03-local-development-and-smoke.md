# Spec 03: Local Development and Smoke Validation

## Summary
Provide a repeatable local workflow for starting the lab, obtaining a token, and exercising the baseline account flow.

## Problem
A developer should be able to run the project locally without relying on ad hoc commands or hidden assumptions.

## Requirements
- The repository must document prerequisites for .NET, Go, Dapr, and Aspire.
- The local startup command must be simple and explicit.
- The smoke script must exercise a realistic account flow with authentication.
- The runbook must include expected outcomes and troubleshooting hints.

## Design Overview
- The developer runs the AppHost from the repository root.
- The smoke script calls the balance/deposit/withdraw sequence with a bearer token.
- The expected balance progression is documented so the flow can be verified by eye.

## Acceptance Criteria
- A developer can run the local start commands from the repository root.
- The smoke script can be executed with a valid token and a known account identifier.
- The runbook explains how to recover from common issues such as Keycloak readiness and Dapr runtime dependencies.

## Implementation Tasks
- Verify the local startup commands and keep them aligned with the current repository structure.
- Exercise the smoke script with a real token and capture the observed balance transitions.
- Add explicit success and failure expectations to the smoke documentation.
- Record troubleshooting steps for Keycloak readiness and Dapr-related startup issues.

## Operational Notes
- The smoke script expects TOKEN and BASE_URL environment variables.
- The baseline flow should be run after Keycloak is available and the service is reachable.
- Command output should be captured as evidence for future milestones.

## Source Notes
This spec consolidates content from the local start runbook, the smoke script, and the learning journal entries.
