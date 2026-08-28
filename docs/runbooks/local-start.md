# Runbook: Local Start and Baseline Verification

## Purpose

Start the lab locally and validate a baseline workflow before changing architecture.

## Prerequisites

- .NET SDK installed
- Go toolchain installed
- Dapr CLI installed and initialized
- Aspire CLI available

## Start Commands

From repository root:

```bash
dotnet --version
go version
make test
aspire start
```

The root `go.work` includes both the canonical Accounts service and the preserved
legacy module. To work on the gRPC service directly:

```bash
cd services/accounts
go test ./...
go run -buildvcs=false ./cmd/accountsmigrate
go run -buildvcs=false ./cmd/accounts
```

The direct `go run` command is useful for isolated service development. Use
`aspire start` for the normal integrated flow with PostgreSQL, Dapr, and Keycloak.

Expected outcome:
- AppHost starts successfully.
- keycloak resource is healthy.
- postgres and usersdb resources are healthy.
- contacts-migrations completes successfully before contacts starts.
- users-migrations completes successfully before users starts.
- accounts-migrations completes successfully before accounts starts.
- accounts, accounts-legacy, contacts, users, and their Dapr sidecars are healthy.
- banking-web is healthy and exposes `https://localhost:7443`.

Use Aspire rather than guessing the dynamically assigned Contacts endpoint:

```bash
aspire describe contacts --format Json
```

Copy the `grpc` URL into the Contacts smoke client. To verify state survives a
service restart:

```bash
cd services/contacts
export CONTACTS_SMOKE_TOKEN="<access-token-with-contacts-roles>"
export CONTACTS_PARENT="users/<current-user-id>"
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action create
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action list
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action update
aspire resource contacts restart
aspire wait contacts
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action get
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action delete
```

The final `get` command must return the updated contact and public ETag after the
restart. Contacts persists in the shared `usersdb` PostgreSQL database;
The legacy accounts/actor flow lives under `services/accounts-legacy` as learning code, but no state store
is provisioned by default. Its stateful actor operations are not part of the
current integrated smoke gate.

For the automated authentication and ownership matrix, configure the local
Aspire secret store once:

```bash
aspire secret set 'SmokeAuth:Keycloak:Username' '<username>' --apphost apphost.cs
aspire secret set 'SmokeAuth:Keycloak:Password' '<password>' --apphost apphost.cs
aspire secret set 'SmokeAuth:Keycloak:ClientSecret' '<client-secret>' --apphost apphost.cs
aspire secret set 'Parameters:banking-web-client-secret' '<client-secret>' --apphost apphost.cs
```

Then run `make smoke-auth` from the repository root. The gate reads secrets with
`aspire secret get`, reuses an existing AppHost when present, or starts Aspire
and stops it during cleanup.
The configured smoke user must belong to the Keycloak `BankingUser` group; the
gate reports any missing Users or Contacts permissions before making RPC calls.

Banking.Web uses `https://localhost:7443` as its stable local OIDC origin. After
changing the committed realm JSON, apply it to an existing Keycloak data volume:

```bash
aspire resource keycloak import-realm
aspire wait banking-web
```

Open `https://localhost:7443` and sign in. The BFF creates or refreshes the local
profile, then displays the profile and owned beneficiaries. A fresh Keycloak
volume imports the callback configuration automatically.

## Baseline Checks

1. Contacts and its Dapr sidecar report healthy.
2. The Contacts CRUD smoke sequence succeeds.
3. Contacts data survives a service restart.
4. Users rejects calls without a bearer token and resolves an authenticated
   Keycloak identity to a stable local profile.

## Legacy Accounts Request Sequence

The following actor-based flow is retained for reference, but requires adding an
actor-compatible state store back to the AppHost before use. It is not part of
the default local baseline.

```bash
chmod +x scripts/smoke-baseline.sh
TOKEN="<paste-access-token>" BASE_URL="http://localhost:8082" scripts/smoke-baseline.sh
```

Expected response pattern:
- First balance call returns initial balance.
- Deposit increases balance by 100 USD.
- Withdraw decreases balance by 50 USD.
- Final balance reflects net +50 USD from initial state.

Manual request sequence (if needed):

```bash
curl -H "Authorization: Bearer $TOKEN" http://localhost:8082/accounts/demo-account-usd/balance
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8082/accounts/demo-account-usd/deposit
curl -H "Authorization: Bearer $TOKEN" http://localhost:8082/accounts/demo-account-usd/balance
curl -X POST -H "Authorization: Bearer $TOKEN" http://localhost:8082/accounts/demo-account-usd/withdraw
curl -H "Authorization: Bearer $TOKEN" http://localhost:8082/accounts/demo-account-usd/balance
```

## Validation Evidence (2026-07-30)

- `aspire --version` returned `13.4.6+87fe259e4fc244c599019a7b1304c85a1488f248`.
- `aspire start` succeeded and started AppHost with dashboard URL.
- `aspire ps` showed `apphost.cs` running.
- `aspire stop` shut down AppHost successfully.

## Troubleshooting

### Dapr actor callbacks not working

Symptoms:
- Actor method calls fail or timeout.

Checks:
- Ensure app exposes Dapr actor endpoints on app port.
- Ensure actor runtime is served via Go Dapr service wrapper.
- Verify actor factories are registered through the Dapr service runtime.

### Keycloak not ready

Symptoms:
- Token validation fails due to issuer/JWKS errors.

Checks:
- Wait until keycloak reports healthy in Aspire.
- Validate issuer URL and realm configuration.

### Go service fails with `bind: address already in use`

Each host-native executable needs a distinct internal listening port. Accounts
uses `8080`; Aspire sets `CONTACTS_PORT=8083` and configures the Contacts Dapr
sidecar with the same app port. External client ports remain Aspire-managed.

### Banking.Web hot reload fails on `localhost:0`

Kestrel does not support dynamic port binding through the special `localhost`
host. The Banking.Web launch profiles therefore use `127.0.0.1:0` for their
Aspire-managed internal HTTP and HTTPS ports. Keep the public browser and OIDC
origin at `https://localhost:7443`; forwarded headers preserve that external URL.

If a launch profile is regenerated, avoid values such as
`http://localhost:0`. Use `http://127.0.0.1:0` (and the HTTPS equivalent) so
`dotnet watch` can restart the Kestrel child process after a source change.

## Evidence to capture

- Command outputs for startup validation.
- One successful baseline flow trace/log excerpt.
- Notes added to learning journal.
