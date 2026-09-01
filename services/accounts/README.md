# Accounts service

The Accounts service is the canonical gRPC boundary for account queries and
lifecycle management. It uses the shared protobuf authorization interceptor,
derives ownership from the immutable Keycloak `sub` claim, and persists accounts
in PostgreSQL.

The former Dapr actor experiment is preserved separately under
`services/accounts-legacy` and is not used by this service.

## Commands

```bash
go run -buildvcs=false ./cmd/accountsmigrate
go run -buildvcs=false ./cmd/accounts
go test ./...
```

The AppHost injects `USERSDB_URI`, `KEYCLOAK_HTTP`, and `ACCOUNTS_PORT`.

## Payments

`SendPayment` is the canonical money-movement command. It debits an owned open
account and addresses a saved beneficiary rather than modeling the action as a
cash withdrawal. Calls require `payments.send` and a stable `request_id`.

The source debit, payment record, and `PaymentSentEvent` outbox record are
atomic. Internal beneficiaries are credited in the same database transaction.
The dispatcher publishes committed outbox rows through the local Dapr HTTP API
to `pubsub` / `payments.sent`; the gRPC handler never publishes directly.

## Demo cash-in

`DepositFunds` provides an explicit learning-only way to fund an owned account.
It requires `accounts.deposit`, validates the account currency, and uses a
caller request ID so retries cannot credit the balance twice. The balance,
deposit record, and `FundsDepositedEvent` outbox row commit atomically; the event
is published to `funds.deposited` and projected into transaction history.
