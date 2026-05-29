#!/usr/bin/env bash
# ACS predicate: task-fingerprint.sh produces identical hash for whitespace-equivalent inputs
# metadata: cycle=49 slug=fingerprint-determinism

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SCRIPT="$WORKTREE/scripts/utility/task-fingerprint.sh"
[ -x "$SCRIPT" ] || { echo "FAIL: $SCRIPT not executable"; exit 1; }

FP1=$(bash "$SCRIPT" --action "Fix X" --criteria "Tests pass" --files "a.sh b.sh" 2>/dev/null)
FP2=$(bash "$SCRIPT" --action "Fix  X" --criteria "Tests  pass" --files "b.sh  a.sh" 2>/dev/null)
FP3=$(bash "$SCRIPT" --action "Fix Y" --criteria "Tests pass" --files "a.sh b.sh" 2>/dev/null)

[ -n "$FP1" ] || { echo "FAIL: empty fingerprint for input 1"; exit 1; }
[ "$FP1" = "$FP2" ] || { echo "FAIL: whitespace-equiv inputs produce different fp: $FP1 vs $FP2"; exit 1; }
[ "$FP1" != "$FP3" ] || { echo "FAIL: different action produces same fp"; exit 1; }
echo "GREEN: fingerprint is deterministic and content-sensitive"
