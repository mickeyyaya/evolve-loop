#!/usr/bin/env bash
# AC-ID: cycle-96-003
# Description: phase-gate.sh cycle-complete resets mastery on non-PASS audit (anti-overmatch guard for the grep fix)
# Evidence: scripts/lifecycle/phase-gate.sh L1167-1179 (gate_cycle_complete mastery RESET branch)
# Author: tdd-engineer (cycle-96 RED phase)
# Created: 2026-05-20
# Acceptance-of: triage-decision.md T2 — anti-regression for the grep pattern fix
#
# Verifies the anti-regression contract for T2:
#   The fix MUST distinguish PASS from FAIL/WARN/REGRESS. A naive fix like
#   `grep -qi PASS audit-report.md` would match a FAIL report that contains
#   the word "PASS" anywhere (e.g., "no AC reaches PASS state"). That would
#   silently increment consecutiveSuccesses on a FAIL — masking real
#   regressions and amplifying the cycle-95 mastery-gate bug.
#
# This predicate ships a FAIL audit-report that contains the word "PASS"
# in a non-verdict context, and asserts that gate_cycle_complete RESETS
# consecutiveSuccesses to 0 (the else branch).
#
# Together with predicate 002, this brackets the fix:
#   002 = no false negatives (canonical PASS is recognized)
#   003 = no false positives (PASS-as-noise does not trigger increment)
#
# Behavioral, end-to-end. Hermetic. Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN (mastery reset on non-PASS audit-report)
#   1 = RED   (false positive — incremented when verdict was FAIL)

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT_OVERRIDE:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
[ -d "$REPO_ROOT" ] || { echo "RED: REPO_ROOT not a directory: $REPO_ROOT" >&2; exit 1; }

PHASE_GATE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
if [ ! -f "$PHASE_GATE" ]; then
  echo "RED: $PHASE_GATE missing" >&2
  exit 1
fi
( cd "$REPO_ROOT" && git ls-files --error-unmatch "scripts/lifecycle/phase-gate.sh" >/dev/null 2>&1 ) \
  || { echo "RED: scripts/lifecycle/phase-gate.sh untracked by git" >&2; exit 1; }

if ! command -v python3 >/dev/null 2>&1; then
  echo "RED: python3 not available" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq not available" >&2
  exit 1
fi

# ── Fixture: seed consecutiveSuccesses=3 (mid-streak) ──────────────────────
FIX=$(mktemp -d -t mastery-fail.XXXXXX)
case "$FIX" in
  /tmp/*|/var/folders/*|/private/var/folders/*) ;;
  *) echo "RED: refusing to use suspect mktemp path: $FIX" >&2; exit 1 ;;
esac
trap 'rm -rf "$FIX"' EXIT

mkdir -p "$FIX/.evolve" "$FIX/.evolve/history" "$FIX/.evolve/runs/cycle-99002"

# Seed a non-zero streak so we can observe the reset transition.
cat > "$FIX/.evolve/state.json" <<'JSON'
{
  "version": 200,
  "mastery": {
    "consecutiveSuccesses": 3,
    "level": "competent"
  },
  "ledgerSummary": {
    "totalTasksShipped": 12
  }
}
JSON
: > "$FIX/.evolve/ledger.jsonl"

WS="$FIX/.evolve/runs/cycle-99002"
cat > "$WS/scout-report.md" <<'MD'
# Scout Report — fixture
Stub for predicate 003.
MD
cat > "$WS/build-report.md" <<'MD'
# Build Report — fixture
Stub for predicate 003.
MD

# FAIL audit-report — deliberately contains the word "PASS" in non-verdict
# contexts (acceptance criteria header, prose). A naive grep -i PASS would
# match and incorrectly increment.
cat > "$WS/audit-report.md" <<'MD'
# Audit Report — Cycle 99002

## Verdict
**FAIL**

Confidence: 0.95.

## Acceptance Criteria

| AC | Status |
|----|--------|
| AC1 | did not reach PASS state |
| AC2 | regression — no PASS |
| AC3 | blocked |

## Findings

The build did not achieve PASS on any criterion. Mastery streak must reset.
MD

# ── Invoke ─────────────────────────────────────────────────────────────────
gate_log="$FIX/gate.log"
rc=0
EVOLVE_PROJECT_ROOT="$FIX" \
  bash "$PHASE_GATE" cycle-complete 99002 "$WS" >"$gate_log" 2>&1 || rc=$?

cs=$(jq -r '.mastery.consecutiveSuccesses // empty' "$FIX/.evolve/state.json" 2>/dev/null)
if [ -z "$cs" ]; then
  echo "RED: post-invocation state.json has no mastery.consecutiveSuccesses field" >&2
  echo "RED: gate log:" >&2
  sed 's/^/  /' "$gate_log" >&2
  exit 1
fi

# Reset branch MUST set consecutiveSuccesses = 0.
if [ "$cs" != "0" ]; then
  echo "RED: state.json:mastery.consecutiveSuccesses=$cs after FAIL audit (expected 0 — reset)" >&2
  echo "RED: the fix over-matches — PASS-as-noise triggered increment" >&2
  echo "RED: phase-gate.sh exit code: $rc" >&2
  echo "RED: gate log:" >&2
  sed 's/^/  /' "$gate_log" >&2
  echo "RED: audit-report.md fixture (recap):" >&2
  sed 's/^/  /' "$WS/audit-report.md" >&2
  exit 1
fi

# Anti-regression: totalTasksShipped MUST stay at the seeded value (12),
# not increment, because the PASS branch did not run.
shipped=$(jq -r '.ledgerSummary.totalTasksShipped // empty' "$FIX/.evolve/state.json" 2>/dev/null)
if [ "$shipped" != "12" ]; then
  echo "RED: ledgerSummary.totalTasksShipped=$shipped (expected 12 — PASS branch ran when verdict was FAIL)" >&2
  exit 1
fi

echo "GREEN: phase-gate.sh cycle-complete correctly RESET mastery 3→0 on FAIL audit despite PASS-as-noise in body (gate rc=$rc, shipped=$shipped)"
exit 0
