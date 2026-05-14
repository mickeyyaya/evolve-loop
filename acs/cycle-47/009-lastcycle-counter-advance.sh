#!/usr/bin/env bash
# ACS predicate: 009 — ship.sh advances lastCycleNumber before post-push integrity check
# cycle: 47
# task: T1 (counter-advance ordering fix)
# severity: HIGH
set -uo pipefail

SHIP_SH="scripts/lifecycle/ship.sh"

# Verify the fix: lastCycleNumber advance must occur before the C1 integrity check
# by checking that the pre-integrity-check advance block appears in the worktree path
# before the integrity_fail call for tree-SHA mismatch.
if ! grep -q 'advanced state.json:lastCycleNumber.*pre-integrity-check' "$SHIP_SH" 2>/dev/null; then
    echo "FAIL: ship.sh does not contain pre-integrity-check lastCycleNumber advance" >&2
    exit 1
fi

# Verify ordering: the pre-integrity advance block must appear BEFORE the integrity_fail
# for INTEGRITY BREACH (tree-SHA mismatch). Extract line numbers and compare.
_advance_line=$(grep -n 'advanced state.json:lastCycleNumber.*pre-integrity-check' "$SHIP_SH" 2>/dev/null | head -1 | cut -d: -f1 || echo "0")
_integrity_line=$(grep -n 'INTEGRITY BREACH: audit-bound tree SHA' "$SHIP_SH" 2>/dev/null | head -1 | cut -d: -f1 || echo "0")

if [ "$_advance_line" = "0" ] || [ "$_integrity_line" = "0" ]; then
    echo "FAIL: could not locate advance block (line=$_advance_line) or integrity_fail (line=$_integrity_line) in ship.sh" >&2
    exit 1
fi

if [ "$_advance_line" -ge "$_integrity_line" ]; then
    echo "FAIL: advance block (line $_advance_line) is not before integrity_fail (line $_integrity_line) — ordering bug not fixed" >&2
    exit 1
fi

echo "PASS: lastCycleNumber advance (line $_advance_line) precedes integrity_fail (line $_integrity_line)"
exit 0
