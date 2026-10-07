#!/usr/bin/env bash
set -euo pipefail

mode="${1:---check}"
config="${RELEASE_IMPACT_CONFIG:-release-impact.json}"

if [[ "$mode" != "--check" && "$mode" != "--write" ]]; then
    printf 'Usage: %s [--check|--write]\n' "$0" >&2
    exit 2
fi

for command in git jq sha256sum; do
    if ! command -v "$command" >/dev/null 2>&1; then
        printf 'Required command is not available: %s\n' "$command" >&2
        exit 2
    fi
done

if [[ ! -f "$config" ]]; then
    printf 'Release impact configuration not found: %s\n' "$config" >&2
    exit 2
fi

repo_root="$(git rev-parse --show-toplevel)"
cd "$repo_root"

failed=0
while IFS= read -r component; do
    marker="$(jq -r --arg component "$component" '.components[$component].marker' "$config")"
    mapfile -t inputs < <(
        jq -r --arg component "$component" '.components[$component].inputs[]' "$config"
    )

    fingerprint_input="$(mktemp)"
    while IFS= read -r -d '' path; do
        if [[ -f "$path" ]]; then
            printf '%s\t%s\n' "$path" "$(git hash-object "$path")" >> "$fingerprint_input"
        else
            printf '%s\t%s\n' "$path" '<deleted>' >> "$fingerprint_input"
        fi
    done < <(git ls-files -co --exclude-standard -z -- "${inputs[@]}" | sort -z)

    fingerprint="$(sha256sum "$fingerprint_input" | cut -d ' ' -f 1)"
    rm -f "$fingerprint_input"
    expected="${fingerprint}  ${component} shared inputs"

    if [[ "$mode" == "--write" ]]; then
        mkdir -p "$(dirname "$marker")"
        printf '%s\n' "$expected" > "$marker"
        printf 'Updated %s\n' "$marker"
        continue
    fi

    actual=""
    if [[ -f "$marker" ]]; then
        actual="$(tr -d '\r\n' < "$marker")"
    fi
    if [[ "$actual" != "$expected" ]]; then
        printf 'Stale release impact marker for %s: %s\n' "$component" "$marker" >&2
        failed=1
    fi
done < <(jq -r '.components | keys[]' "$config")

if (( failed != 0 )); then
    printf 'Run make release-impact and commit the updated markers.\n' >&2
    exit 1
fi

if [[ "$mode" == "--check" ]]; then
    printf 'Release impact markers are up to date.\n'
fi
