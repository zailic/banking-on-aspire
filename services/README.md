# Services

Each service is an independently buildable Go module that owns its entrypoints,
domain behavior, adapters, tests, and dependencies.

Current service:

- `accounts` - the existing Dapr actor-backed bank account API.

Future services are added incrementally according to the migration plan rather than
as empty deployable skeletons.
