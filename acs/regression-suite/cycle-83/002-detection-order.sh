#!/usr/bin/env bash
# AC2: Detection order is CUSTOM_PROXY > API_KEY > SUBSCRIPTION_OAUTH > MISCONFIGURED.
# Verify by setting overlapping env and checking priority.
set -uo pipefail
ROOT="${WORKTREE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
SCRIPT="$ROOT/scripts/utility/doctor-subscription-auth.sh"

extract_mode() {
    grep -o '"mode":"[^"]*"' | grep -o '"[^"]*"$' | tr -d '"'
}

# Both EVOLVE_ANTHROPIC_BASE_URL + ANTHROPIC_API_KEY → CUSTOM_PROXY wins
m1=$(env -i EVOLVE_ANTHROPIC_BASE_URL="http://x" ANTHROPIC_API_KEY="sk-x" HOME="$HOME" /bin/bash "$SCRIPT" --json 2>/dev/null | extract_mode)
[ "$m1" = "CUSTOM_PROXY" ] || { echo "RED AC2.1: CUSTOM_PROXY did not win over API_KEY (got $m1)"; exit 1; }

# Only ANTHROPIC_API_KEY → API_KEY
tmpd=$(mktemp -d)
m2=$(env -i ANTHROPIC_API_KEY="sk-x" EVOLVE_DOCTOR_CRED_FILE_OVERRIDE="$tmpd/none" HOME="$tmpd" /bin/bash "$SCRIPT" --json 2>/dev/null | extract_mode)
[ "$m2" = "API_KEY" ] || { echo "RED AC2.2: expected API_KEY, got $m2"; exit 1; }

# Only OAuth cred file → SUBSCRIPTION_OAUTH
cred="$tmpd/c.json"
printf '{"claudeAiOauth":{"accessToken":"tok123"}}\n' > "$cred"
m3=$(env -i EVOLVE_DOCTOR_CRED_FILE_OVERRIDE="$cred" HOME="$tmpd" /bin/bash "$SCRIPT" --json 2>/dev/null | extract_mode)
[ "$m3" = "SUBSCRIPTION_OAUTH" ] || { echo "RED AC2.3: expected SUBSCRIPTION_OAUTH, got $m3"; exit 1; }

# Nothing → MISCONFIGURED
m4=$(env -i EVOLVE_DOCTOR_CRED_FILE_OVERRIDE="$tmpd/absent" HOME="$tmpd" /bin/bash "$SCRIPT" --json 2>/dev/null | extract_mode)
[ "$m4" = "MISCONFIGURED" ] || { echo "RED AC2.4: expected MISCONFIGURED, got $m4"; exit 1; }

rm -rf "$tmpd"
echo "GREEN AC2: detection order CUSTOM_PROXY > API_KEY > SUBSCRIPTION_OAUTH > MISCONFIGURED verified"
exit 0
