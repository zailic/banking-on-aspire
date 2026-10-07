# Component releases

## Commit format

Use Conventional Commits and name the affected component in the scope:

```text
fix(accounts): reject duplicate payment commands
feat(banking-web): add transaction filters
feat(contracts)!: introduce the v2 payment API
```

Release Please groups all pending component changes into a release pull request. When
that pull request is merged, it creates tags such as `accounts-v1.2.3` and the release
workflow publishes the corresponding container image.

## Shared changes

`release-impact.json` defines the shared inputs compiled into each deployable component.
After changing `platform/`, generated contracts, or `protos/`, update the deterministic
component fingerprints:

```bash
make release-impact
git add services/*/.release-impact frontend/Banking.Web/.release-impact
```

The pull-request workflow runs `make release-impact-check` and rejects stale markers.
Because the markers live under component paths, Release Please includes the affected
components in its next release. The graph is intentionally conservative; update it when
a shared package is split or a consumer stops depending on it.

The Conventional Commit type of the shared change is also applied to the affected
consumers. Use `fix(platform)` for an internal compatible change, `feat(platform)` when
the consumers gain externally observable capability, and a breaking-change marker only
when the deployable consumers themselves become incompatible.

Do not rebuild an old SemVer tag after shared code changes.

## Local deployment tags

Set one workload independently:

```bash
export BANKING_ACCOUNTS_IMAGE_TAG=1.4.2
export BANKING_WEB_IMAGE_TAG=1.5.0
```

`BANKING_RELEASE_TAG` remains the fallback for variables that are not set. If neither
form is supplied, the AppHost generates a timestamp tag for an ad-hoc local deployment.

The registry is parameterized through the standard Aspire configuration convention:

```bash
export Parameters__registry_endpoint=ghcr.io
export Parameters__registry_repository=<owner>/banking-on-aspire
```

Local publishing defaults to `localhost:5001/banking-on-aspire`.

## Contract compatibility

Before merging a Protobuf change, compare it to the default branch:

```bash
cd protos
../tools/bin/buf breaking --against '../.git#branch=main,subdir=protos'
```

The normal `make proto-check` command continues to cover formatting, linting, and
compilation.

## Registry output

The release workflow publishes each deployable component to:

```text
ghcr.io/<repository-owner>/banking-on-aspire/<component>:<version>
ghcr.io/<repository-owner>/banking-on-aspire/<component>:sha-<commit>
```

Images include OCI version/revision labels, provenance, and an SBOM. The `contracts`
component creates a GitHub release but does not publish a container image.
