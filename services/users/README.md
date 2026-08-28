# Users service

The Users service owns application profiles linked to Keycloak identities.
Keycloak remains the source of truth for authentication, credentials, sessions,
MFA, and access roles; this service stores only banking-application profile data.

The first increment exposes authenticated gRPC operations:

- `GetOrCreateCurrentUser`, which resolves the JWT `sub` claim and synchronizes
  the local username, display name, and email;
- `GetUser`, restricted to the profile owned by the caller's `sub` claim.

Both Users and Contacts use the Aspire-managed `usersdb` database, while keeping
separate tables and migration executables.

For an authenticated runtime check, obtain an access token as described in the
Keycloak runbook, discover the Users endpoint with `aspire describe users`, then
run:

```bash
USERS_SMOKE_TOKEN="$TOKEN" go run ./cmd/userssmoke -endpoint grpc://localhost:<port> -action resolve
USERS_SMOKE_TOKEN="$TOKEN" go run ./cmd/userssmoke -endpoint grpc://localhost:<port> -action get -name users/<id>
```
