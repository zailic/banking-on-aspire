#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

for command in aspire curl jq go; do
  if ! command -v "$command" >/dev/null 2>&1; then
    echo "$command is required for the auth smoke gate" >&2
    exit 1
  fi
done

read_secret() {
  local key="$1"
  local value
  if ! value="$(aspire secret get "$key" --apphost apphost.cs --non-interactive 2>/dev/null)"; then
    echo "Aspire secret '$key' is required; configure it with:" >&2
    echo "  aspire secret set '$key' '<value>' --apphost apphost.cs" >&2
    exit 1
  fi
  if [[ -z "$value" ]]; then
    echo "Aspire secret '$key' must not be empty" >&2
    exit 1
  fi
  printf '%s' "$value"
}

KEYCLOAK_USER="$(read_secret 'SmokeAuth:Keycloak:Username')"
KEYCLOAK_PASSWORD="$(read_secret 'SmokeAuth:Keycloak:Password')"
KEYCLOAK_CLIENT_SECRET="$(read_secret 'SmokeAuth:Keycloak:ClientSecret')"

started_apphost=false
temp_dir="$(mktemp -d)"

cleanup() {
  local exit_code=$?
  if [[ "$started_apphost" == true ]]; then
    aspire stop --non-interactive >/dev/null || true
  fi
  rm -rf "$temp_dir"
  exit "$exit_code"
}
trap cleanup EXIT INT TERM

running_apphosts="$(aspire ps --format Json --non-interactive)"
if [[ "$(jq 'length' <<<"$running_apphosts")" -eq 0 ]]; then
  aspire start --non-interactive
  started_apphost=true
fi

aspire wait keycloak --timeout 180 --non-interactive
aspire wait users --timeout 180 --non-interactive
aspire wait contacts --timeout 180 --non-interactive

describe_resource() {
  aspire describe "$1" --format Json --non-interactive | sed -n '/^{/,$p'
}

keycloak_url="$(describe_resource keycloak | jq -er '.resources[0].urls[] | select(.name == "http") | .url')"
users_endpoint="$(describe_resource users | jq -er '.resources[0].urls[] | select(.name == "grpc") | .url')"
contacts_endpoint="$(describe_resource contacts | jq -er '.resources[0].urls[] | select(.name == "grpc") | .url')"

token_response="$(curl -kfsS --connect-timeout 10 -X POST \
  "${keycloak_url}/realms/banking-on-aspire/protocol/openid-connect/token" \
  -H 'Content-Type: application/x-www-form-urlencoded' \
  -d 'grant_type=password' \
  -d 'client_id=banking-on-aspire-app' \
  --data-urlencode "client_secret=${KEYCLOAK_CLIENT_SECRET}" \
  --data-urlencode "username=${KEYCLOAK_USER}" \
  --data-urlencode "password=${KEYCLOAK_PASSWORD}")"
token="$(jq -er '.access_token' <<<"$token_response")"

token_permissions="$(printf '%s' "$token" | jq -Rr '
  split(".")[1]
  | gsub("-"; "+")
  | gsub("_"; "/")
  | @base64d
  | fromjson
  | .resource_access["banking-on-aspire-app"].roles // []
')"
required_permissions=(users.profile.read users.profile.write contacts.read contacts.write accounts.balance.read accounts.deposit payments.send transactions.read)
missing_permissions=()
for permission in "${required_permissions[@]}"; do
  if ! jq -e --arg permission "$permission" 'index($permission) != null' \
    <<<"$token_permissions" >/dev/null; then
    missing_permissions+=("$permission")
  fi
done
if (( ${#missing_permissions[@]} > 0 )); then
  echo "the smoke user is missing required permissions:" >&2
  printf '  - %s\n' "${missing_permissions[@]}" >&2
  echo "assign the Keycloak BankingUser group, request a new token, and rerun make smoke-auth" >&2
  exit 1
fi

go build -o "$temp_dir/userssmoke" ./services/users/cmd/userssmoke
go build -o "$temp_dir/contactsmoke" ./services/contacts/cmd/contactsmoke

user_output="$(USERS_SMOKE_TOKEN="$token" "$temp_dir/userssmoke" \
  -endpoint "$users_endpoint" -action resolve)"
user_name="${user_output%% *}"
if [[ ! "$user_name" =~ ^users/[^/]+$ ]]; then
  echo "Users smoke returned an invalid resource name: $user_name" >&2
  exit 1
fi

expect_grpc_code() {
  local expected_code="$1"
  shift
  local output
  if output="$("$@" 2>&1)"; then
    echo "expected gRPC $expected_code, but the call succeeded" >&2
    exit 1
  fi
  if ! grep -Fq "code = ${expected_code}" <<<"$output"; then
    echo "expected gRPC $expected_code, got:" >&2
    echo "$output" >&2
    exit 1
  fi
  printf '✓ expected %s\n' "$expected_code"
}

echo "==> authenticated caller can access owned contacts"
CONTACTS_SMOKE_TOKEN="$token" "$temp_dir/contactsmoke" \
  -endpoint "$contacts_endpoint" -parent "$user_name" -action list

echo "==> missing token is rejected"
expect_grpc_code Unauthenticated env CONTACTS_SMOKE_TOKEN= \
  "$temp_dir/contactsmoke" -endpoint "$contacts_endpoint" -parent "$user_name" -action list

echo "==> malformed token is rejected"
expect_grpc_code Unauthenticated env CONTACTS_SMOKE_TOKEN=not-a-jwt \
  "$temp_dir/contactsmoke" -endpoint "$contacts_endpoint" -parent "$user_name" -action list

echo "==> another user's resource is rejected"
expect_grpc_code PermissionDenied env CONTACTS_SMOKE_TOKEN="$token" \
  "$temp_dir/contactsmoke" -endpoint "$contacts_endpoint" -parent users/not-the-caller -action list

echo "==> valid token without a required permission is rejected by the descriptor interceptor"
go test ./platform/auth/keycloak -run '^TestUnaryServerInterceptorRejectsMissingRole$' -count=1

echo "Auth smoke gate completed successfully."
