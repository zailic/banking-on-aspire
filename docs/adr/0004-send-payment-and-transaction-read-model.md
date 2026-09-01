# ADR 0004: Send payments before modeling cash withdrawals

Date: 2026-08-28
Status: Accepted

## Context

The canonical Accounts service needs a money-movement use case that exercises
the existing beneficiary boundary and can feed the future Transactions read
model. The preserved legacy service exposes deposit and withdraw operations, but
`Withdraw` describes cash removal rather than a payment to a known beneficiary.

## Decision

Accounts exposes `SendPayment` as the primary money-movement command.

- Accounts remains authoritative for balances and payment execution.
- The source account and selected beneficiary must belong to the authenticated
  application user.
- The payment amount is positive and uses the source account currency.
- A caller-generated `request_id` makes retries idempotent per source account.
- The debit, payment record, and `PaymentSent` outbox record are committed in one
  PostgreSQL transaction.
- When the beneficiary targets an internal account, that account is credited in
  the same transaction. External destinations currently represent the boundary
  where a future payment-rail adapter will be introduced.
- The Accounts outbox dispatcher publishes `PaymentSentEvent` through Dapr's
  Redis-backed `pubsub` component on `payments.sent`.
- Transactions consumes the event idempotently into its own PostgreSQL database
  and exposes an owner-scoped read API; it does not mutate balances.

The legacy withdraw operation remains available only in `accounts-legacy` for
learning and baseline compatibility. It is not the canonical payment workflow.

## Consequences

- Beneficiaries now have a concrete business use case.
- Duplicate command delivery cannot debit an account twice.
- Redis is used only as the pub/sub broker. PostgreSQL remains the durable source
  for the Accounts outbox and Transactions projection.
- External payment completion semantics require an explicit adapter and status
  model in a later increment; the current service only establishes the persisted
  payment boundary.
