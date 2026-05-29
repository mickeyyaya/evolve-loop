#!/usr/bin/env bash
# AC-ID: cycle-88-phase-gate-functions-migrated
#
# Verifies Cycle B kernel edit on scripts/lifecycle/phase-gate.sh:
#   1. gate_intent_to_research()   function body REMOVED (no `() {` definition).
#   2. gate_research_to_discover() function body REMOVED (no `() {` definition).
#   3. gate_intent_to_discover()   function body ADDED (definition present).
#   4. Comment about "verification happens at gate_intent_to_research" is gone
#      (stale-pointer cleanup; "what without why rots" principle).
#
# Behavioral, not tautological:
#   We require both *function-definition removal* AND *new-function presence*.
#   Mutating either edge (e.g., keeping the old function around as dead code)
#   trips the test. Comment cleanup is a separate assertion so simply renaming
#   the function body keyword does not silently pass.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
GATE_FILE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"

fail=0
errors=""

if [ ! -f "$GATE_FILE" ]; then
  echo "RED cycle-88-phase-gate-functions-migrated: phase-gate.sh missing at $GATE_FILE"
  exit 1
fi

# Match function definitions only — line starts with name + parens + open brace.
# This intentionally ignores case-statement string mentions (which we DO want
# to keep, with exit-2 error text).
old_intent_def=$(grep -cE '^[[:space:]]*gate_intent_to_research[[:space:]]*\([[:space:]]*\)[[:space:]]*\{' "$GATE_FILE" 2>/dev/null || echo 0)
old_research_def=$(grep -cE '^[[:space:]]*gate_research_to_discover[[:space:]]*\([[:space:]]*\)[[:space:]]*\{' "$GATE_FILE" 2>/dev/null || echo 0)
new_discover_def=$(grep -cE '^[[:space:]]*gate_intent_to_discover[[:space:]]*\([[:space:]]*\)[[:space:]]*\{' "$GATE_FILE" 2>/dev/null || echo 0)

if [ "${old_intent_def:-0}" -gt 0 ] 2>/dev/null; then
  errors="${errors}\n  phase-gate.sh STILL defines gate_intent_to_research() (must be removed)"
  fail=$((fail + 1))
fi
if [ "${old_research_def:-0}" -gt 0 ] 2>/dev/null; then
  errors="${errors}\n  phase-gate.sh STILL defines gate_research_to_discover() (must be removed)"
  fail=$((fail + 1))
fi
if [ "${new_discover_def:-0}" -lt 1 ] 2>/dev/null; then
  errors="${errors}\n  phase-gate.sh MISSING gate_intent_to_discover() function definition"
  fail=$((fail + 1))
fi

# Stale-pointer comment cleanup: the v8.19 calibrate-to-intent doc-block
# referenced "verification happens at gate_intent_to_research below" — that
# pointer is now wrong. Allow either the comment to be removed entirely OR
# rewritten to point at gate_intent_to_discover.
if grep -qE 'gate_intent_to_research below' "$GATE_FILE"; then
  errors="${errors}\n  phase-gate.sh still has stale 'gate_intent_to_research below' comment"
  fail=$((fail + 1))
fi

# Bonus: top-of-file usage block must NOT advertise the retired gate name as a
# valid one. (Allow it to mention retirement in error text — that's intent.)
if grep -qE '^#[[:space:]]+research-to-discover[[:space:]]+—[[:space:]]+Verify Phase 1 ran' "$GATE_FILE"; then
  errors="${errors}\n  phase-gate.sh usage header still lists 'research-to-discover — Verify Phase 1 ran'"
  fail=$((fail + 1))
fi

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-phase-gate-functions-migrated: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-phase-gate-functions-migrated: legacy gate functions removed, gate_intent_to_discover present, stale comments cleaned"
exit 0
