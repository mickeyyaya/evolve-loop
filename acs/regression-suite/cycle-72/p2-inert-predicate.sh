#!/usr/bin/env bash
# ACS predicate: cycle-72 P2 INERT marking verification
set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PASS=0
FAIL=0

check() {
    local desc="$1"
    local result="$2"
    if [ "$result" -eq 0 ]; then
        echo "PASS: $desc"
        PASS=$((PASS + 1))
    else
        echo "FAIL: $desc"
        FAIL=$((FAIL + 1))
    fi
}

# P1: P2 row contains "INERT cycle 72"
grep -q "INERT cycle 72" "$REPO_ROOT/docs/architecture/token-economics-2026.md"
check "P2 row contains 'INERT cycle 72'" $?

# P2: P2 row contains verbatim C71 telemetry delta string
grep -q "39 turns / \$0.7305 vs.*26 turns / \$0.5931" "$REPO_ROOT/docs/architecture/token-economics-2026.md"
check "P2 row contains C71 telemetry '39 turns / \$0.7305 vs.*26 turns / \$0.5931'" $?

# P3: ADR file exists
test -f "$REPO_ROOT/docs/architecture/adr/0009-p2-turn-budget-inert.md"
check "ADR 0009 file exists" $?

# P4: ADR file contains a rollback section
grep -qi "rollback" "$REPO_ROOT/docs/architecture/adr/0009-p2-turn-budget-inert.md"
check "ADR 0009 contains rollback section" $?

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
