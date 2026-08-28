# Bank of Anthos Mapping to This Learning Lab

Purpose: map concepts from Bank of Anthos to banking-on-aspire, a stack centered on Aspire, Dapr, Keycloak, and Radius.

## Mapping Table

| Bank of Anthos concept | Banking-on-Aspire target | Technology focus | Notes |
|---|---|---|---|
| Frontend + BFF | API gateway/BFF layer (next) | Aspire resource graph, token propagation | Start with one thin authenticated vertical slice |
| Users service | Identity and profile boundary (implemented) | Keycloak + PostgreSQL profile service | Keep identity source of truth in Keycloak |
| Ledger/transactions | Transaction history read service | Dapr pub/sub + state store | Build read model from emitted domain events |
| Accounts service | Legacy command/actor flow | Persistence selected from domain requirements | Keep the current actor implementation only as a learning reference until modernization |
| Contacts service | Beneficiaries context (implemented) | gRPC + PostgreSQL | Proto RBAC and `sub`-based ownership are enforced |
| Platform/deploy setup | Environment model | Radius environments and recipes | Separate local orchestration from deploy topology |

## Recommended Port Order

1. Baseline and observability first.
2. Identity hardening with Keycloak client-role permissions.
3. Users and beneficiaries contexts with canonical resource ownership.
4. Thin frontend/BFF vertical slice.
5. Event stream for account domain actions and transaction read model extraction.
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
- Users and Contacts enforce ownership from immutable `sub` to canonical user
  resources. Only the legacy Accounts flow still uses `preferred_username` to
  account ID mapping.

## Non-goals for initial phase

- Exact parity with GCP deployment assets from Bank of Anthos.
- Full microservice decomposition in one sprint.
- Production-grade multi-region topology.

## Success Criteria

- Every extraction leaves at least one endpoint or flow fully working.
- Each new service has one smoke test and one runbook section.
- Architecture decisions are tracked in ADR files under docs/adr.
