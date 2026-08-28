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
