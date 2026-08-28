#!/usr/bin/env bash
set -euo pipefail

# Baseline smoke flow for account operations.
# Requires a bearer token with accounts.balance.read,
# transactions.deposit.create, and transactions.withdraw.create roles.

BASE_URL="${BASE_URL:-http://localhost:8082}"
ACCOUNT_ID="${ACCOUNT_ID:-demo-account-usd}"
TOKEN="${TOKEN:-}"

if [[ -z "$TOKEN" ]]; then
  echo "TOKEN is required. Export TOKEN with a valid bearer token." >&2
  exit 1
fi

auth_header=( -H "Authorization: Bearer $TOKEN" )

request() {
  local method="$1"
  local path="$2"
  echo "==> ${method} ${path}"
  curl -fsS -X "$method" "${BASE_URL}${path}" "${auth_header[@]}"
  echo
}

request GET "/accounts/${ACCOUNT_ID}/balance"
request POST "/accounts/${ACCOUNT_ID}/deposit"
request GET "/accounts/${ACCOUNT_ID}/balance"
request POST "/accounts/${ACCOUNT_ID}/withdraw"
request GET "/accounts/${ACCOUNT_ID}/balance"

echo "Smoke flow completed successfully."
