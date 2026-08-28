# Legacy Accounts service

This directory preserves the original HTTP and Dapr actor-based account flow as
a learning reference. New account functionality belongs in `services/accounts`,
which implements the protobuf-defined gRPC API over PostgreSQL.
