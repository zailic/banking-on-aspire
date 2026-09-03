# Observability and Failure Drills

This runbook covers `DepositFunds` and `SendPayment`, the payment flow that
replaced the earlier withdraw idea. The expected path is:

```text
Banking.Web BFF -> Accounts gRPC -> durable outbox -> Accounts Dapr sidecar
                -> Redis pub/sub -> Transactions Dapr sidecar
                -> Transactions HTTP subscriber -> transactionsdb
```

## Safety

- Run these drills only against the local Aspire development environment.
- Deposit and payment probes change demo balances by the requested amount.
- Stop one resource at a time and restore it before starting another drill.
- Always use `aspire resource`; do not kill processes or containers manually.
- Finish with `aspire stop --non-interactive` when the session is no longer needed.

## Start and inspect

```bash
aspire start --non-interactive
aspire wait accounts --timeout 180 --non-interactive
aspire wait transactions --timeout 180 --non-interactive
aspire wait banking-web --timeout 180 --non-interactive
```

The Go resources opt in to dashboard export through
`WithOtlpExporter(OtlpProtocol.Grpc)`. Banking.Web uses ServiceDefaults with
ASP.NET Core, HTTP, and gRPC-client instrumentation. Dapr sidecars export their
own spans.

## UI trace: BFF to Transactions

1. Sign in to Banking.Web.
2. Execute one small **Add funds** and one small **Send payment** operation.
3. Query the BFF spans:

```bash
aspire otel traces banking-web --search 'name=DepositFunds' --format Json --limit 10 --non-interactive
aspire otel traces banking-web --search 'name=SendPayment' --format Json --limit 10 --non-interactive
```

Copy a returned `traceId`, then inspect the complete trace:

```bash
aspire otel traces --trace-id <trace-id> --format Json --non-interactive
```

Expected sources in one trace are `banking-web`, `accounts`,
`accounts-dapr-cli`, `transactions-dapr-cli`, and `transactions`. The BFF span
also carries resource names such as `banking.account`, `banking.deposit`,
`banking.source_account`, `banking.beneficiary`, and `banking.payment`.

## Backend trace probe

When browser automation is unavailable, set a valid smoke token and discover the
Accounts endpoint through `aspire describe`:

```bash
export ACCOUNTS_SMOKE_TOKEN='<access-token>'
export ACCOUNTS_ENDPOINT='<grpc URL from aspire describe accounts --format Json>'

go run ./services/accounts/cmd/accountsmoke \
  -endpoint "$ACCOUNTS_ENDPOINT" -parent accounts/<account-id> \
  -currency RON -amount 1.00 -action deposit

go run ./services/accounts/cmd/accountsmoke \
  -endpoint "$ACCOUNTS_ENDPOINT" -parent accounts/<account-id> \
  -beneficiary users/<user-id>/contacts/<contact-id> \
  -currency RON -amount 1.00 -action send
```

Each command prints a `trace_id`. Query it with `aspire otel traces --trace-id`.
The durable outbox stores W3C `traceparent` and `tracestate`, so publish and
consumer spans remain children of the original Accounts command even when
delivery happens later.

## Retry verification

The automated gate proves that a transient Dapr HTTP failure does not mark the
outbox row published, that the next dispatch retries it, and that the same trace
context is propagated:

```bash
go test ./services/accounts/internal/outbox -run TestDispatchRetriesUnpublishedEventAndPropagatesTraceContext -count=1
```

The BFF retries exactly once only when a gRPC call returns `Unauthenticated`. It
forces token refresh, adds `access_token.refresh_retry` and
`banking.auth.retry=true` to the current activity, and logs a warning. Other gRPC
errors are not replayed automatically. Accounts still enforces command
idempotency through `request_id`.

## Dapr unavailable

```bash
aspire resource accounts-dapr-cli stop --include-hidden --non-interactive
# Execute a small deposit using accountsmoke or the UI.
aspire logs accounts --non-interactive
aspire resource accounts-dapr-cli start --include-hidden --non-interactive
```

Expected: the database transaction succeeds, the event remains in the outbox,
and after sidecar recovery Transactions applies it once under the original trace.

## PostgreSQL unavailable

```bash
aspire resource postgres stop --non-interactive
# An Accounts or Transactions request now fails with a temporary error.
aspire resource postgres start --non-interactive
aspire wait postgres --timeout 120 --non-interactive
# Repeat the request; pgx pools reconnect without a service restart.
```

Do not retry an ambiguous money command with a new `request_id`; reuse the same
idempotency key because the database outcome may be unknown to the caller.

## Keycloak unavailable

```bash
aspire resource keycloak stop --non-interactive
# New login, token issuance, and forced refresh fail here.
# A still-valid JWT may continue through the verifier's cached JWKS.
aspire resource keycloak start --non-interactive
aspire wait keycloak --timeout 180 --non-interactive
```

Existing sessions work only until refresh is required. New sign-in and refresh
fail while Keycloak is down; token issuance recovers after it becomes healthy.

## Useful diagnostics

```bash
aspire describe --include-hidden --format Json --non-interactive
aspire otel traces --has-error --format Json --limit 50 --non-interactive
aspire otel logs --format Json --limit 50 --non-interactive
aspire logs accounts --non-interactive
aspire logs transactions --non-interactive
aspire export
```

Use `--include-hidden` for Dapr sidecars and proxies. Do not attach raw
`aspire describe` output to tickets because resource environments can contain
credentials; select only the state, relationship, URL, or OTEL fields required.
