#!/usr/bin/env bash
# ACS predicate: T1 — ship.sh strips backtick characters from audit_bound_tree_sha
# cycle: 47
# task: T1
# severity: HIGH
set -uo pipefail

SHIP_SH="scripts/lifecycle/ship.sh"

# Verify the fix: tr -d must include backtick in the delete set
if ! grep -q 'tr -d ".*\[:space:\].*\\`"' "$SHIP_SH" 2>/dev/null; then
    echo "FAIL: ship.sh does not strip backtick from audit_bound_tree_sha (tr -d missing backtick in delete set)" >&2
    exit 1
fi

# Functional: confirm the extraction produces a clean SHA when input has backtick wrapping
_raw='audit_bound_tree_sha: `abc123def456789`'
_extracted=$(echo "$_raw" | grep -m1 'audit_bound_tree_sha:' | awk '{print $NF}' | tr -d "[:space:]\`" || true)
if [ "$_extracted" != "abc123def456789" ]; then
    echo "FAIL: backtick stripping produced '$_extracted', expected 'abc123def456789'" >&2
    exit 1
fi

echo "PASS: ship.sh correctly strips backticks from audit_bound_tree_sha"
exit 0
