#!/usr/bin/env bash
# AC4: profiles/scout.json (and triage.json, memo.json) contain schema_filter_enabled field
# predicate: grep-check for schema_filter_enabled in relevant profiles
# metadata: cycle=47 task=T2a ac=AC4 risk=low

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
FAIL=0
for profile in scout triage memo; do
    FILE="$REPO_ROOT/.evolve/profiles/${profile}.json"
    [ -f "$FILE" ] || { echo "FAIL: $FILE missing"; FAIL=1; continue; }
    [[ $(cat "$FILE") =~ schema_filter_enabled ]] || { echo "FAIL: $FILE missing schema_filter_enabled field"; FAIL=1; }
done
[ "$FAIL" -eq 0 ] || exit 1
echo "PASS: scout.json, triage.json, memo.json all contain schema_filter_enabled field"
exit 0
