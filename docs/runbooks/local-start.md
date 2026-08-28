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
go test ./services/accounts/...
aspire start
```

The root `go.work` includes the independently buildable accounts module. To work
inside that module directly:

```bash
cd services/accounts
go test ./...
go run -buildvcs=false ./cmd/accounts
```

The direct `go run` command is useful for isolated service development. Use
`aspire start` for the normal integrated flow with Dapr and Keycloak.

Expected outcome:
- AppHost starts successfully.
- keycloak resource is healthy.
- bank-account-service and dapr sidecar appear healthy.

## Baseline Checks

1. Service health endpoint responds (if available).
2. Create account request succeeds.
3. Deposit/withdraw request succeeds.
4. Balance query returns expected value.

## Baseline Request Sequence

Run the smoke script after obtaining a valid bearer token from Keycloak.

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

## Evidence to capture

- Command outputs for startup validation.
- One successful baseline flow trace/log excerpt.
- Notes added to learning journal.
