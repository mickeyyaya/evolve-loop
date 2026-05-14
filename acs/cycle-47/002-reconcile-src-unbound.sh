#!/usr/bin/env bash
# ACS predicate: T2 — reconcile-carryover-todos.sh uses $_src not $src on line 274
# cycle: 47
# task: T2
# severity: HIGH
set -uo pipefail

RECONCILE="scripts/lifecycle/reconcile-carryover-todos.sh"

# Verify fix: no bare $src usage in the SKIP promote log line
if grep -n 'SKIP promote \$src' "$RECONCILE" 2>/dev/null | grep -q .; then
    echo "FAIL: reconcile-carryover-todos.sh still uses bare \$src (unbound under set -u)" >&2
    grep -n 'SKIP promote \$src' "$RECONCILE" >&2
    exit 1
fi

# Verify the correct variable is used
if ! grep -q 'SKIP promote \$_src' "$RECONCILE" 2>/dev/null; then
    echo "FAIL: reconcile-carryover-todos.sh missing 'SKIP promote \$_src' log line" >&2
    exit 1
fi

echo "PASS: reconcile-carryover-todos.sh uses \$_src (no unbound variable risk)"
exit 0
