# ADR 0005: Version deployable components independently

## Status

Accepted

## Context

The repository contains independently buildable Go services, a .NET frontend, and
shared Protobuf contracts. The previous deployment model assigned one timestamp-based
`BANKING_RELEASE_TAG` to every image. That value identified a deployment attempt, but
did not describe the version or change history of an individual component.

The versioning mechanism must remain independent of implementation language and must
not couple the Go modules merely because they share a repository and an Aspire AppHost.

## Decision

Use independent Semantic Versions for these release components:

- `accounts`
- `contacts`
- `transactions`
- `users`
- `banking-web`
- `contracts`

Release Please runs in manifest mode and uses the `simple` release strategy. Each
component owns a `version.txt`, a changelog, and Git tags in the form
`<component>-v<version>`.

Conventional Commits determine the increment:

- `fix` increments patch;
- `feat` increments minor;
- `!` or a `BREAKING CHANGE` footer increments major.

Container images are published with an immutable release tag and a source revision
tag. Deployments accept an image tag per workload. `BANKING_RELEASE_TAG` remains a
compatibility fallback, while `BANKING_<COMPONENT>_IMAGE_TAG` overrides it for a single
workload.

The supported variables are:

| Component | Variable |
| --- | --- |
| accounts | `BANKING_ACCOUNTS_IMAGE_TAG` |
| contacts | `BANKING_CONTACTS_IMAGE_TAG` |
| transactions | `BANKING_TRANSACTIONS_IMAGE_TAG` |
| users | `BANKING_USERS_IMAGE_TAG` |
| banking-web | `BANKING_WEB_IMAGE_TAG` |

Changes to `platform/` or `protos/` can affect more than one executable. The dependency
graph in `release-impact.json` maps shared inputs to deployable consumers. A deterministic
fingerprint stored inside each consumer directory changes when any of its shared inputs
changes, allowing Release Please to include that consumer without custom release logic.
CI rejects stale fingerprints. Rebuilding unchanged component versions and moving an
existing SemVer image tag is forbidden.

Protobuf compatibility is checked against the default branch with `buf breaking`.
Breaking schema changes require a new Protobuf package version (for example `v2`) in
addition to a major `contracts` release.

## Consequences

- A release changes only the components represented by its commits.
- Aspire and Radius can deploy mixed component versions.
- `BANKING_RELEASE_TAG` can still reproduce the legacy lock-step behavior during the
  transition.
- Shared-code changes require the dependency graph to remain accurate. The graph is
  deliberately conservative: a false-positive patch release is safer than an unchanged
  image tag containing different shared code.
- Production promotion should eventually pin image digests in an environment release
  manifest. SemVer tags are release metadata, not the final immutability boundary.
