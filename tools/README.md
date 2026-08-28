# Project developer tools

This directory keeps the repository-specific CLI tools reproducible. Versions
live in `versions.mk`; downloaded binaries live in the ignored `tools/bin`
directory.

Install and inspect them from the repository root:

```bash
make tools
make tools-versions
```

The project currently installs:

- Buf CLI 1.72.0 for protobuf formatting, linting, dependency management, and
  generation.
- protoc 33.5 for local protobuf inspection and tooling that requires the
  compiler directly. Buf generation itself uses the remote plugins pinned in
  `protos/buf.gen.yaml`.

`curl` plus either `unzip` or Python 3 are bootstrap prerequisites. Go, .NET,
Aspire, Dapr, Docker, and `jq` remain system tools because they are runtimes or
environment-level CLIs rather than repository build utilities.

Run `make help` for the normal project workflows. `make tools-clean` removes
only downloaded tools; the next `make tools` recreates them.
