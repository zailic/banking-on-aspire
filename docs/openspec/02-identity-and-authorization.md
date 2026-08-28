# Spec 02: Identity and Authorization

## Summary
Define how the bank account API authenticates requests and authorizes access to account operations.

## Problem
The service needs clear assumptions about tokens, roles, and account ownership so that authorization can be reasoned about and tested consistently.

## Requirements
- The API must verify bearer tokens through Keycloak-backed OIDC discovery.
- Required permissions must be expressed as client roles.
- Authorization decisions must be traceable for balance read, deposit, withdraw, and close operations.
- The service must map a known user to a known account for local smoke testing.

## Functional Rules
- Balance read requires the role accounts.balance.read.
- Deposit requires the role transactions.deposit.create.
- Withdraw requires the role transactions.withdraw.create.
- Close requires the role accounts.close.
- The current local ownership mapping uses ionut -> demo-account-usd.

## Design Overview
- Keycloak is configured with a dedicated realm and client.
- Role names are aligned with the service implementation.
- The API checks the token, required client role, and account ownership before executing the operation.

## Acceptance Criteria
- A valid token with the required role can access the corresponding endpoint.
- A valid token without the required role returns a forbidden response.
- A token without an Authorization header returns an unauthorized response.
- The local setup steps are documented so a developer can reproduce the flow.

## Implementation Tasks
- Document the realm, client, roles, and test user setup in the runbook.
- Add or preserve automated coverage for unauthorized, forbidden, and authorized requests.
- Validate the token retrieval flow and confirm the expected roles are present.
- Review whether the current ownership mapping should eventually move out of code and into configuration.

## Open Questions
- Should the ownership mapping move to configuration rather than code in a later milestone?
- Should authorization be enforced by scopes, roles, or a combination of both?

## Source Notes
This spec consolidates content from the Keycloak runbook and the server authorization tests.
