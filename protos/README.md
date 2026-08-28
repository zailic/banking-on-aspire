# Protobuf contracts

This directory is the source of truth for service APIs. Definitions follow the
Google API Improvement Proposals (GAIP) and are validated with Buf.

## Toolchain

From the repository root, install the pinned Buf and protoc versions and run
the protobuf workflow:

```bash
make tools
make proto
```

Use `make proto-check` for a non-mutating formatting, lint, and compilation
check. Direct invocations are available as `../tools/bin/buf` from this
directory. Run `buf dep update` explicitly when intentionally updating
`buf.lock`.

`buf.gen.yaml` pins every remote generator. Generated Go and C# files are written
under `../platform/gen`; `clean: true` removes stale generated files before each
generation pass.

The two RPC response lint exceptions in `buf.yaml` are intentional: standard GAIP
methods return the resource itself, while delete returns
`google.protobuf.Empty`.
