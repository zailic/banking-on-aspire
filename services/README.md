# Services

Each service is an independently buildable Go module that owns its entrypoints,
domain behavior, adapters, tests, and dependencies.

Current services:

- `accounts` - account queries and lifecycle management over gRPC and PostgreSQL;
- `accounts-legacy` - the preserved Dapr actor-backed learning API;
- `contacts` - beneficiary CRUD backed by the shared PostgreSQL `usersdb`;
- `users` - application profiles linked to Keycloak identities through the
  immutable OpenID Connect `sub` claim.

Future services are added incrementally according to the migration plan rather than
as empty deployable skeletons.
