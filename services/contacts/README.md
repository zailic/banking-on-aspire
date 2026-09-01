# Contacts service

The Contacts service owns beneficiaries saved by a banking user. It exposes the
GAIP-style gRPC API defined in `../../protos/banking/contacts/v1/contacts.proto`.

## Run locally

Normally, start the service through Aspire so that PostgreSQL is provisioned and
`USERSDB_URI` plus the Accounts gRPC endpoint are injected:

```bash
aspire run --apphost ../../apphost.cs
```

To run only the Go process, provide a PostgreSQL URI explicitly:

```bash
USERSDB_URI='postgres://postgres:postgres@localhost:5432/usersdb' \
ACCOUNTS_GRPC='grpc://localhost:<accounts-port>' \
  go run ./cmd/contacts
```

The server listens on `CONTACTS_PORT` (default `8080`) inside its process,
publishes the standard gRPC health service, and persists contacts in PostgreSQL.
Aspire sets the internal development port to `8083` so it does not conflict with
the accounts process on the host, and injects the shared users database URL as
`USERSDB_URI`. It also wires `ACCOUNTS_GRPC` and starts Contacts only after
Accounts is healthy.

Creating an internal beneficiary, or changing an existing destination to an
internal account, calls `AccountsService.GetAccount` with the caller's bearer
token. Missing accounts and accounts the caller cannot read are rejected before
the contact is persisted. External beneficiaries do not call Accounts.

The persistence boundary has two implementations:

- `MemoryRepository`, used by transport tests;
- `PostgresRepository`, used by the runtime process.

The one-shot `contacts-migrations` Aspire resource applies the embedded migration
under `internal/contactrepo/migrations` before the service starts. Contacts are
stored as individual rows with a composite `(user_id, contact_id)` primary key, a
destination consistency check, and optimistic concurrency based on the public
ETag.

## Verify

```bash
go test ./...
```

Use the `contactsmoke` command with the gRPC URL reported by `aspire describe
contacts --format Json`:

```bash
export CONTACTS_SMOKE_TOKEN="<access-token-with-contacts-roles>"
export CONTACTS_PARENT="users/<current-user-id>"
# The create action's internal account must already exist and be readable by this token.
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -internal-account accounts/<existing-account-id> -action create
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action list
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action update
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action get
go run ./cmd/contactsmoke -endpoint grpc://localhost:<port> -parent "$CONTACTS_PARENT" -action delete
```

Read operations require `contacts.read`; mutations require `contacts.write`.
The imported Keycloak realm grants both through the `BankingUser` group.

From the repository root, `make smoke-auth` discovers all Aspire endpoints and
runs the authentication, permission, and ownership gate automatically.
