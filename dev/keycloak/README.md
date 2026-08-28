# Development Keycloak realm

The Aspire dashboard exposes two commands on the `keycloak` resource:

- **Export realm** writes `banking-on-aspire-realm.json` into this directory.
- **Import realm** imports that file into the running realm and overwrites matching
  clients, roles, groups, and other resources after confirmation.

Aspire also mounts this directory through `WithRealmImport`. Keycloak uses that import
when starting with fresh storage. It does not replace an existing realm in the persistent
Keycloak data volume.

Review exported JSON before committing it. Do not commit user passwords, client secrets,
private signing keys, or other credentials.
