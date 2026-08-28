# Bank of Anthos Mapping to This Learning Lab

Purpose: map concepts from Bank of Anthos to banking-on-aspire, a stack centered on Aspire, Dapr, Keycloak, and Radius.

## Mapping Table

| Bank of Anthos concept | Banking-on-Aspire target | Technology focus | Notes |
|---|---|---|---|
| Frontend + BFF | API gateway/BFF layer (future) | Aspire resource graph, Dapr service invocation | Start with thin proxy and auth propagation |
| Users service | Identity and profile boundary | Keycloak + lightweight profile service | Keep identity source of truth in Keycloak |
| Ledger/transactions | Transaction history read service | Dapr pub/sub + state store | Build read model from emitted domain events |
| Accounts service | Existing bank account service | Dapr actors + state | Preserve actor model and enrich with idempotency |
| Contacts service | Beneficiaries context | Dapr state + APIs | Useful for authz and ownership rules |
| Platform/deploy setup | Environment model | Radius environments and recipes | Separate local orchestration from deploy topology |

## Recommended Port Order

1. Baseline and observability first.
2. Identity hardening with Keycloak scopes.
3. Event stream for account domain actions.
4. Transaction read model extraction.
5. Beneficiaries context extraction.
6. Radius environment templates for deployment.

## Role Model Aligned to Existing Flows

| Existing API flow | Required role | Bank of Anthos flow intent |
|---|---|---|
| Balance inquiry (`GET /accounts/{accounts}/balance`) | `accounts.balance.read` | Account overview/balance read |
| Deposit (`POST /accounts/{accounts}/deposit`) | `transactions.deposit.create` | Cash-in transaction submission |
| Withdraw (`POST /accounts/{accounts}/withdraw`) | `transactions.withdraw.create` | Cash-out/payment transaction submission |
| Close account (`POST /accounts/{accounts}/close`) | `accounts.close` | Account lifecycle administration |

Design notes:
- Transaction roles are operation-scoped (`transactions.*`) and separate from account lifecycle roles (`accounts.*`).
- Ownership constraints are still enforced from `preferred_username` to account ID mapping in the API.

## Non-goals for initial phase

- Exact parity with GCP deployment assets from Bank of Anthos.
- Full microservice decomposition in one sprint.
- Production-grade multi-region topology.

## Success Criteria

- Every extraction leaves at least one endpoint or flow fully working.
- Each new service has one smoke test and one runbook section.
- Architecture decisions are tracked in ADR files under docs/adr.
