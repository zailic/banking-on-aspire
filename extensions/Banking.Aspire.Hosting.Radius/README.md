# Banking.Aspire.Hosting.Radius

Temporary publishing adapter between `CommunityToolkit.Aspire.Hosting.Dapr` and
`Aspire.Hosting.Radius`.

The current Radius publisher does not translate Aspire's `DaprSidecarAnnotation` into the
`properties.extensions.daprSidecar` object consumed by the Kubernetes container recipe. This
library registers a pipeline step after Radius Bicep generation and adds that object for every
Aspire workload configured with `WithDaprSidecar(...)`.

Radius 13.5 preview also recognizes the older CLR name `DaprPubSubResource`, while the current
Community Toolkit models generic components as `DaprComponentResource`. In publish mode,
`AddRadiusDaprPubSub(...)` bridges that app-model mismatch so Radius emits and owns the
`Applications.Dapr/pubSubBrokers` infrastructure resource. Local run mode continues to use the
Community Toolkit's normal `AddDaprPubSub(...)` implementation.

The same adapter corrects the preview publisher's unavailable
`local-dev/daprpubsubbrokers:latest` recipe reference to Radius' published
`local-dev/pubsubbrokers:latest` artifact.

Radius' legacy Dapr recipe currently creates the component in its legacy application namespace
(for example `default-app`), while the Radius 0.60 container recipe places workloads in the
environment namespace (`default`). After infrastructure deployment the adapter idempotently mirrors
the Radius-owned component spec into the workload namespace, preserving the generated backing-store
endpoint. Dapr sidecars can then discover the component in their own namespace.

```csharp
builder.AddRadiusDaprPublishing(
    radius,
    dependsOn: "radius-schema-compatibility");
```

The Radius 0.60 schema accepts `appId`, `appPort`, and `config`, but not `appProtocol` or arbitrary
pod annotations. After each workload deployment, the release step patches the generated Kubernetes
Deployment with `dapr.io/app-protocol` and a common `dapr.io/internal-grpc-port` of `30002`, then
waits for the rollout. This preserves gRPC application discovery and avoids collisions between
Dapr's default internal port `50002` and Linux ephemeral client ports.

This adapter intentionally fails when a Dapr-enabled Aspire resource is absent from the generated
Radius workloads, or when Radius begins emitting a different `extensions` object. Those failures
prevent a publisher upgrade from silently generating an incomplete deployment.

## Independent release artifacts

`AddRadiusReleaseArtifacts(...)` splits the post-processed Radius Bicep into one infrastructure
artifact and one artifact per workload while keeping all resources in the same Radius application.
Shared resources are emitted as `existing` declarations in workload artifacts.

```csharp
builder.AddRadiusReleaseArtifacts(
    infrastructure.Radius,
    accounts.Resource,
    transactions.Resource);
```

Generate the files without deploying them:

```shell
aspire do publish-radius-release-artifacts --non-interactive
```

By default, the files are placed under `artifacts/radius`:

- `infrastructure.bicep` owns the shared Radius resources;
- `services/<service>.bicep` owns exactly one workload and refers to shared resources as
  `existing`.

After splitting, `artifacts/app.bicep` is replaced with the infrastructure-only artifact, so
the built-in `deploy-radius-radius` entry point can no longer deploy application workloads.

Both publishing and deployment honor Aspire's `-o`/`--output-path` option. For example,
`aspire do publish-radius-release-artifacts -o release-output --non-interactive` writes these files
under `release-output/radius` instead.

The deployment graph contains one infrastructure step and one step per workload. Each workload
step depends on its own image push plus the infrastructure and AppHost-specific prerequisites:

```text
deploy-radius-radius
└── configure-radius-keycloak
    ├── migrate-radius-accounts     -> deploy-radius-accounts     (push-accounts)
    ├── migrate-radius-contacts     -> deploy-radius-contacts     (push-contacts)
    ├── migrate-radius-users        -> deploy-radius-users        (push-users)
    ├── migrate-radius-transactions -> deploy-radius-transactions (push-transactions)
    └── deploy-radius-banking-web                              (push-banking-web)
```

A full `aspire deploy` executes all of them. A selective release can target one entry point:

```shell
aspire do deploy-radius-transactions --non-interactive
```

The selective step still reconciles the infrastructure artifact first, but that artifact contains
no application workloads. The Keycloak prerequisite creates a missing realm or updates an existing
realm from the checked-in export. Database migration gates are project-specific and are added by
this AppHost between that prerequisite and each backend workload deployment.

Deployment migrations run on the pipeline runner through a temporary `kubectl port-forward` to
PostgreSQL. The runner therefore needs `go`, `kubectl`, access to the Radius Kubernetes cluster,
and the Aspire PostgreSQL password parameter. A process-wide lock prevents concurrent migrations.
During a full deployment, schema prerequisites are ordered and executed once; a selective workload
deployment runs only that workload's migration.

The AppHost also adds `ensure-radius-postgres-persistence` immediately after the Radius
infrastructure deployment. This compensates for the Kubernetes PostgreSQL recipe not carrying
Aspire's `WithDataVolume` annotation into the generated workload: it creates the
`boa-postgres-data` PVC, mounts it at `/var/lib/postgresql/data`, and waits for PostgreSQL before
Keycloak configuration and database migrations continue.

The following `ensure-radius-keycloak-persistence` step applies the same workaround to Keycloak:
it creates the `boa-keycloak-data` PVC, mounts it at `/opt/keycloak/data`, and waits for the
deployment before the realm configuration step runs. This preserves manually created users and
other Keycloak state across pod restarts and restarts of the same kind cluster. Deleting and
recreating the kind cluster also deletes its local persistent volumes.

Keep `Parameters:postgres-password` stable between deployment runs. For local deployments it can
be stored with
`dotnet user-secrets set "Parameters:postgres-password" "<password>" --project Banking.AppHost.csproj`;
CI should inject the equivalent
`Parameters__postgres-password` secret. Rotating the parameter without rotating the PostgreSQL
role and all workload connection URIs will make already deployed services fail authentication.
The AppHost fails fast in publish mode when this value is absent instead of silently generating a
new credential for existing infrastructure.

Deploy-time parameter values are resolved from Aspire `ParameterResource`s and written to a
temporary, owner-only ARM parameter file. The file is removed after `rad deploy`, including failure
paths.
