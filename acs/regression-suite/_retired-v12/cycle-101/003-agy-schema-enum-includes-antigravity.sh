#!/usr/bin/env bash
# ACS 003 — _capabilities-schema.json enum includes "antigravity"; _capability-check.sh accepts it.
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCHEMA="$REPO_ROOT/scripts/cli_adapters/_capabilities-schema.json"
CAP_CHECK="$REPO_ROOT/scripts/cli_adapters/_capability-check.sh"
PASS=0
FAIL=0

check() {
    local label="$1" result="$2"
    if [ "$result" = "ok" ]; then
        echo "  PASS: $label"
        PASS=$((PASS + 1))
    else
        echo "  FAIL: $label — $result"
        FAIL=$((FAIL + 1))
    fi
}

echo "=== ACS 003: schema enum includes antigravity ==="

# 1. Schema file contains "antigravity" in the enum
if grep -q '"antigravity"' "$SCHEMA"; then
    check 'schema enum contains "antigravity"' "ok"
else
    check 'schema enum contains "antigravity"' "not found in $SCHEMA"
fi

# 2. _capability-check.sh accepts antigravity (exits 0)
if [ -x "$CAP_CHECK" ]; then
    bash "$CAP_CHECK" antigravity >/dev/null 2>&1
    _rc=$?
    if [ "$_rc" = "0" ]; then
        check "_capability-check.sh antigravity exits 0" "ok"
    else
        check "_capability-check.sh antigravity exits 0" "exit code $_rc"
    fi
else
    echo "  SKIP: _capability-check.sh not executable — skipping runtime check"
fi

# 3. Negative check: bogus CLI name is NOT in the enum
if grep -q '"bogus_cli_xyz123"' "$SCHEMA"; then
    check "schema does NOT contain bogus CLI name" "found bogus value — schema too permissive"
else
    check "schema does NOT contain bogus CLI name" "ok"
fi

# 4. Existing adapters still present (regression)
for _adapter in claude gemini codex; do
    if grep -q "\"$_adapter\"" "$SCHEMA"; then
        check "schema still contains existing adapter: $_adapter" "ok"
    else
        check "schema still contains existing adapter: $_adapter" "missing — regression"
    fi
done

# 5. agy.capabilities.json exists and has version:1
AGY_CAP="$REPO_ROOT/scripts/cli_adapters/agy.capabilities.json"
if [ -f "$AGY_CAP" ]; then
    check "agy.capabilities.json exists" "ok"
    if command -v jq >/dev/null 2>&1; then
        _ver=$(jq -r '.version' "$AGY_CAP" 2>/dev/null)
        if [ "$_ver" = "1" ]; then
            check 'agy.capabilities.json version == 1' "ok"
        else
            check 'agy.capabilities.json version == 1' "got: $_ver"
        fi
        _adapter=$(jq -r '.adapter' "$AGY_CAP" 2>/dev/null)
        if [ "$_adapter" = "antigravity" ]; then
            check 'agy.capabilities.json adapter == "antigravity"' "ok"
        else
            check 'agy.capabilities.json adapter == "antigravity"' "got: $_adapter"
        fi
    fi
else
    check "agy.capabilities.json exists" "not found at $AGY_CAP"
fi

echo ""
echo "Result: $PASS passed, $FAIL failed"
[ "$FAIL" = "0" ]
