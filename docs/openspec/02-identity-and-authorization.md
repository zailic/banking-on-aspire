# Spec 02: Identity and Authorization

Status: Completed (M2, 2026-08-22)

## Summary
Define how services authenticate callers, enforce RPC permissions, and protect
resources owned by an application user.

## Problem
Authentication, permission checks, and resource ownership are distinct decisions.
They need one explicit contract so policies, generated APIs, and runtime
enforcement cannot silently drift apart.

## Requirements
- The API must verify bearer tokens through Keycloak-backed OIDC discovery.
- Required permissions must be declared on protobuf RPCs and sourced from
  Keycloak client roles in `resource_access[client_id].roles`.
- Every declared permission is required; an empty list means authenticated-only.
- Application RPCs without an authorization policy must fail closed.
- Domain services must enforce ownership independently of RPC permissions.

## Functional Rules
- Balance read requires the role accounts.balance.read.
- Deposit requires the role transactions.deposit.create.
- Withdraw requires the role transactions.withdraw.create.
- Close requires the role accounts.close.
- User profile reads require users.profile.read; profile resolution requires
  users.profile.read and users.profile.write.
- Contact reads require contacts.read; contact mutations require contacts.write.
- Users and Contacts derive ownership from the immutable Keycloak `sub` claim.
- The legacy Accounts flow retains its local username-to-account mapping until
  the Accounts contract is modernized.

## Design Overview
- Keycloak is configured with a dedicated realm and client.
- `banking.auth.v1.AuthorizationPolicy` extends protobuf method options with
  `required_permissions`.
- The shared gRPC interceptor resolves the method descriptor, authenticates the
  token, and requires all declared permissions.
- Users and Contacts perform resource ownership checks inside their service
  boundaries after authentication and RBAC.
- gRPC health endpoints are an explicit infrastructure exemption.

## Acceptance Criteria
- A valid token with the required role can access the corresponding endpoint.
- A valid token without every required permission returns `PermissionDenied`.
- A token without an Authorization header returns `Unauthenticated`.
- A caller targeting another user's resource returns `PermissionDenied`.
- An RPC without a policy is denied by default.
- The local setup steps are documented so a developer can reproduce the flow.

## Implementation Tasks
- [x] Document realm, client roles, groups, and smoke-user setup.
- [x] Generate Go and C# contracts from the shared authorization option.
- [x] Cover unauthenticated, missing-permission, authorized, and cross-user paths.
- [x] Add `make smoke-auth` with Aspire endpoint discovery and secret-store access.

## Resolved Questions
- Permissions are application capabilities represented by Keycloak client roles;
  OAuth scopes are not used as a second authorization vocabulary.
- Ownership is not configuration and is not encoded in RBAC. It is resolved from
  `sub` to the canonical application resource and enforced by the owning service.

## Follow-up
- Replace Contacts' transitional shared-database user resolver with a Users API
  call when service-to-service invocation is introduced.
- Modernize the legacy Accounts ownership mapping around `sub` and canonical
  account resources.

## Source Notes
This spec is implemented by ADR 0003, the protobuf authorization contract,
`platform/auth/keycloak`, service ownership checks, and the auth smoke gate.
