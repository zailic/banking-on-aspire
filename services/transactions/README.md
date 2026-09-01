# Transactions service

`transactions` owns the immutable transaction-history read model. It consumes
`banking.events.v1.PaymentSentEvent` from the Dapr `pubsub` component on the
`payments.sent` and `funds.deposited` topics, projects debit/credit rows into
`transactionsdb`, and exposes them through the authenticated
`TransactionsService` gRPC API.

The consumer records each event ID before projection in the same PostgreSQL
transaction, making redelivery idempotent. The users database is read only to
map the authenticated Keycloak subject to its canonical `users/{user}` owner.
