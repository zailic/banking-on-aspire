# Platform

Reusable platform capabilities belong here: authentication middleware,
observability helpers, generated protobuf code, and cross-service contracts.

Business workflows remain inside their owning service. A platform package must not
import a service module.

## Authentication

`auth/keycloak` contains the shared OpenID Connect verifier, normalized Keycloak
claims, client-role checks, and a gRPC unary authentication interceptor. It
validates bearer tokens through issuer discovery but deliberately has no access
to the Keycloak Admin API.

Accounts uses the verifier and client-role checks from its HTTP middleware;
Users also uses the shared gRPC interceptor and claims context.

## Generated contracts

`gen/go` and `gen/csharp` are generated from the definitions in `../protos`.
Do not edit generated files by hand.

- Go consumers import packages from the `dev.local/banking-on-aspire/platform`
  module.
- .NET consumers reference
  `dotnet/Banking.Contracts/Banking.Contracts.csproj`.

Regenerate and validate the contracts from the repository root:

```bash
cd protos
buf format -w
buf lint
buf generate
buf build
```
