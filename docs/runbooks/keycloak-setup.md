# Runbook: Keycloak Setup for Identity Onboarding

## Purpose

Configure Keycloak so the bank account API can enforce role-based authorization.

This runbook aligns with the current service assumptions from code:
- Realm path suffix: /realms/banking-on-aspire
- Client ID expected by verifier: banking-on-aspire-app
- Roles checked in client roles: accounts.balance.read, accounts.create, transactions.deposit.create, transactions.withdraw.create, accounts.close, users.profile.read, users.profile.write, contacts.read, contacts.write
- Preferred username used for account ownership mapping: ionut

Naming note:
- The repository is named banking-on-aspire.
- The service currently expects the realm path /realms/banking-on-aspire.
- If you use a different realm, update the constant in
  `services/accounts/cmd/accounts/main.go` accordingly.

## Service Auth Assumptions

From current implementation:
- The API verifies the access token signature, issuer, and audience through OIDC discovery.
- The access token audience (`aud`) must contain `banking-on-aspire-app`.
- Required roles are checked in resource_access[banking-on-aspire-app].roles.
- A request is authorized only if:
  - token is valid
  - required client role is present
  - preferred_username maps to requested account in the internal users map

Current ownership mapping:
- ionut -> demo-account-usd

## Keycloak Configuration Steps

1. Open Keycloak admin console from Aspire dashboard resource link.
2. Create realm named banking-on-aspire.
3. Create client named banking-on-aspire-app:
   - Client type: OpenID Connect
   - Access type: Public for local testing, or Confidential if you need client secret
   - Enable Direct Access Grants for password-flow smoke testing
   - Valid redirect URIs can be local placeholders for API-only testing
   - Add an audience mapper so access tokens are intended for this API:
     1. Open **Client scopes** and select the dedicated scope for
        `banking-on-aspire-app` (usually named
        `banking-on-aspire-app-dedicated`).
     2. Open **Mappers**, choose **Configure a new mapper**, then **Audience**.
     3. Set **Name** to `banking-on-aspire-app-audience`.
     4. Set **Included Client Audience** to `banking-on-aspire-app`.
     5. Enable **Add to access token**, then save.
4. In client roles, create:
   - accounts.balance.read
   - accounts.create
   - transactions.deposit.create
   - transactions.withdraw.create
   - accounts.close
   - users.profile.read
   - users.profile.write
   - contacts.read
   - contacts.write
5. Create test user:
   - Username: ionut
   - Set a password and mark non-temporary
6. Assign the BankingUser group and any required account/transaction groups to user ionut.

## Token Retrieval for Smoke Tests

Store the smoke credentials in the AppHost's local Aspire secret store:

```bash
aspire secret set 'SmokeAuth:Keycloak:Username' 'local-dev' --apphost apphost.cs
aspire secret set 'SmokeAuth:Keycloak:Password' '<user-password>' --apphost apphost.cs
aspire secret set 'SmokeAuth:Keycloak:ClientSecret' '<client-secret>' --apphost apphost.cs
```

`make smoke-auth` retrieves these values with `aspire secret get`; it does not
load credentials from `.env` files. For a manual token request, read them into
the current shell without printing them:

```bash
KEYCLOAK_USER=$(aspire secret get 'SmokeAuth:Keycloak:Username' --apphost apphost.cs --non-interactive)
KEYCLOAK_PASSWORD=$(aspire secret get 'SmokeAuth:Keycloak:Password' --apphost apphost.cs --non-interactive)
KEYCLOAK_CLIENT_SECRET=$(aspire secret get 'SmokeAuth:Keycloak:ClientSecret' --apphost apphost.cs --non-interactive)
KEYCLOAK_BASE_URL="http://localhost:8080"
KEYCLOAK_REALM="banking-on-aspire"
KEYCLOAK_CLIENT_ID="banking-on-aspire-app"
TOKEN=$(curl -kfsS -X POST "${KEYCLOAK_BASE_URL}/realms/${KEYCLOAK_REALM}/protocol/openid-connect/token" \
  -H "Content-Type: application/x-www-form-urlencoded" \
  -d "grant_type=password" \
  -d "client_id=${KEYCLOAK_CLIENT_ID}" \
  -d "client_secret=${KEYCLOAK_CLIENT_SECRET}" \
  -d "username=${KEYCLOAK_USER}" \
  -d "password=${KEYCLOAK_PASSWORD}" | jq -r '.access_token')

echo "Token length: ${#TOKEN}"
```

Verify the token before calling the API. The output must include
`banking-on-aspire-app` in `aud`:

```bash
printf '%s' "$TOKEN" | jq -R '
  split(".")[1]
  | gsub("-"; "+")
  | gsub("_"; "/")
  | @base64d
  | fromjson
  | {aud, azp, resource_access}
'
```

Request a new token after changing a mapper; an already-issued token does not
gain the new audience.

Use token with smoke script:

```bash
TOKEN="$TOKEN" BASE_URL="http://localhost:8082" scripts/smoke-baseline.sh
```

## Endpoint to Scope Matrix

| Endpoint | Method | Required role | Notes |
|---|---|---|---|
| /v1/users/{user}/accounts | POST | accounts.create | Administrative account creation with a zero initial balance |
| /accounts/{accounts}/balance | GET | accounts.balance.read | Maps to Bank of Anthos-style balance inquiry flow |
| /accounts/{accounts}/deposit | POST | transactions.deposit.create | Maps to cash-in transaction flow |
| /accounts/{accounts}/withdraw | POST | transactions.withdraw.create | Maps to cash-out/payment initiation flow |
| /accounts/{accounts}/close | POST | accounts.close | Lifecycle/admin operation, separated from transaction roles |

## Token Test Matrix

| Scenario | Token validity | Client role | Username/account mapping | Expected HTTP |
|---|---|---|---|---|
| Missing Authorization header | N/A | N/A | N/A | 401 |
| Malformed Bearer token | Invalid | N/A | N/A | 401 |
| Valid token, missing required role | Valid | Missing | Valid | 403 |
| Valid token, has role, wrong account | Valid | Present | Invalid | 403 |
| Valid token, has role, correct account | Valid | Present | Valid | 200 |

## Known Caveats

- API currently returns typo strings in some forbidden branches, but status codes remain the source of truth.
- KEYCLOAK_BASE_URL must match the real Keycloak endpoint exposed by your local Aspire run.
- If jq is unavailable, parse JSON with another tool or inspect manually.
- `azp` (authorized party) is not a substitute for `aud`. A token can have
  `azp: banking-on-aspire-app` while still being rejected when its audience is
  only `account`.

## Troubleshooting Audience Rejections

The warning below means the token was issued successfully, but it was not
issued for the bank account API:

```text
oidc: expected audience "banking-on-aspire-app" got ["account"]
```

Keep audience validation enabled in the API. Add the audience mapper described
above, obtain a fresh access token, and confirm that its payload resembles:

```json
{
  "aud": ["banking-on-aspire-app", "account"]
}
```
