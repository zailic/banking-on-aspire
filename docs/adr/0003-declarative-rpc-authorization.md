# ADR 0003: Declare RPC permissions in protobuf contracts

## Status

Accepted

## Date

2026-08-22

## Context

Users and Contacts expose gRPC APIs authenticated with Keycloak access tokens.
Keeping required roles only in Go handlers would make authorization invisible to
other generated clients and allow contracts and enforcement to drift. Role checks
also do not establish whether a caller owns a specific `users/{user}` resource.

The project already represents application capabilities as Keycloak client roles
under `resource_access[banking-on-aspire-app].roles`.

## Decision

Define `banking.auth.v1.AuthorizationPolicy` as a custom
`google.protobuf.MethodOptions` extension. Application RPCs declare a repeated
`required_permissions` field using stable capability names such as
`contacts.read` and `users.profile.write`.

Enforcement follows these rules:

- Every listed permission is required (all-of semantics).
- An empty permission list means any authenticated caller.
- `allow_unauthenticated` explicitly marks a public RPC.
- A missing policy fails closed.
- gRPC health methods are an explicit infrastructure exemption.
- Missing or invalid identity returns `Unauthenticated`.
- Missing permissions return `PermissionDenied`.

The shared interceptor in `platform/auth/keycloak` resolves the runtime method
descriptor, validates the token, and compares required permissions with Keycloak
client roles. Services separately enforce resource ownership using the immutable
Keycloak `sub` claim and canonical resource names.

## Consequences

Positive:

- Authorization requirements are versioned with the API contract.
- Go and C# consumers receive the same policy metadata.
- New RPCs are denied until their access model is deliberately specified.
- Permission enforcement is reusable while ownership remains within its domain.
- Contract tests can enumerate RPC policies without starting the application.

Negative:

- Renaming a permission requires coordinated proto, Keycloak, documentation, and
  deployment changes.
- Descriptor-driven enforcement adds runtime reflection and must remain covered
  by tests.
- Client roles are still Keycloak configuration and are not created by protobuf
  generation.

## Alternatives considered

- Handler-local role constants: rejected because policy is duplicated and hidden
  from the contract.
- A protobuf enum of roles: rejected because identity-provider capabilities need
  to evolve without recompiling a central enum for every bounded context.
- OAuth scopes as a parallel permission model: rejected to avoid two names for
  the same authorization decision.
- Encoding ownership in RBAC: rejected because ownership depends on the requested
  resource and authenticated subject, not only on a caller role.

## Verification

- `make proto-check` validates the authorization contract.
- Contract and interceptor tests cover policy resolution and all-of semantics.
- Contacts tests cover ownership for list, get, create, update, and delete.
- `make smoke-auth` exercises the Keycloak, RBAC, and ownership runtime paths.
