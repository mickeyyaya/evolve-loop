#!/usr/bin/env bash
# AC-ID: cycle-96-002
# Description: phase-gate.sh cycle-complete recognizes canonical `## Verdict\n**PASS**` audit-report format and increments mastery
# Evidence: scripts/lifecycle/phase-gate.sh L1148-1166 (gate_cycle_complete mastery branch)
# Author: tdd-engineer (cycle-96 RED phase)
# Created: 2026-05-20
# Acceptance-of: triage-decision.md T2 AC1+AC2+AC3 — mastery consecutiveSuccesses increment after PASS
#
# Verifies T2 acceptance criteria from triage-decision.md:
#   AC1: phase-gate.sh:1149 grep pattern matches `## Verdict\n**PASS**` audit-report format.
#   AC2: ACS predicate exits 0 against canonical input.
#   AC3: state.json mastery.consecutiveSuccesses increments 0 → 1 after PASS.
#
# Behavioral, end-to-end: invokes `bash scripts/lifecycle/phase-gate.sh
# cycle-complete <cycle> <workspace>` with an isolated EVOLVE_PROJECT_ROOT
# fixture, then reads the resulting state.json to verify the mutation.
#
# This is NOT a grep predicate. We do not scan phase-gate.sh source for a
# pattern; we invoke the script and assert the side effect. That is the
# only way to catch the cycle-95 bug class: a grep that "looks right" in
# source but does not match the actual audit-report markdown emitted by
# the Auditor (the bug was exactly this — `Verdict:.*PASS` matches inline
# colon-form, not the multiline `## Verdict\n**PASS**` heading form).
#
# Hermetic: uses mktemp -d for the fixture root; no network; no shared
# state with the real .evolve/. EVOLVE_PROJECT_ROOT redirection per
# scripts/lifecycle/resolve-roots.sh:79 is the test seam.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN (mastery incremented after canonical PASS audit-report)
#   1 = RED   (no increment — grep pattern still buggy)

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT_OVERRIDE:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
[ -d "$REPO_ROOT" ] || { echo "RED: REPO_ROOT not a directory: $REPO_ROOT" >&2; exit 1; }

PHASE_GATE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
if [ ! -f "$PHASE_GATE" ]; then
  echo "RED: $PHASE_GATE missing" >&2
  exit 1
fi
# Tracking check from REPO_ROOT (relative path so git sees it).
( cd "$REPO_ROOT" && git ls-files --error-unmatch "scripts/lifecycle/phase-gate.sh" >/dev/null 2>&1 ) \
  || { echo "RED: scripts/lifecycle/phase-gate.sh untracked by git" >&2; exit 1; }

if ! command -v python3 >/dev/null 2>&1; then
  echo "RED: python3 not available — required by gate_cycle_complete mastery branch" >&2
  exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq not available — needed to assert state.json after invocation" >&2
  exit 1
fi

# ── Build isolated fixture project root ────────────────────────────────────
FIX=$(mktemp -d -t mastery-pass.XXXXXX)
# Guard against rm -rf of "" or "/".
case "$FIX" in
  /tmp/*|/var/folders/*|/private/var/folders/*) ;;
  *)
    echo "RED: refusing to use suspect mktemp path: $FIX" >&2
    exit 1
    ;;
esac
trap 'rm -rf "$FIX"' EXIT

mkdir -p "$FIX/.evolve" "$FIX/.evolve/history" "$FIX/.evolve/runs/cycle-99001"

# Minimal state.json (matches the schema phase-gate.sh's python3 reads).
cat > "$FIX/.evolve/state.json" <<'JSON'
{
  "version": 100,
  "mastery": {
    "consecutiveSuccesses": 0,
    "level": "novice"
  },
  "ledgerSummary": {
    "totalTasksShipped": 0
  }
}
JSON

# Empty ledger (gate uses it only with `command -v jq`; absence is tolerated).
: > "$FIX/.evolve/ledger.jsonl"

WS="$FIX/.evolve/runs/cycle-99001"

# Stub artifacts (gate_cycle_complete only `check_file_exists` here — no
# freshness or substance gate fires for cycle-complete). Non-empty content
# satisfies `[ -s ]`.
cat > "$WS/scout-report.md" <<'MD'
# Scout Report — fixture
Stub artifact for predicate 002.
MD
cat > "$WS/build-report.md" <<'MD'
# Build Report — fixture
Stub artifact for predicate 002.
MD

# Canonical Auditor format: `## Verdict\n**PASS**` (two-line, bold). This
# is the exact shape audit-report.md takes in cycles 91-95. The buggy
# pre-fix grep `Verdict:.*PASS` does NOT match this; the post-fix grep
# must.
cat > "$WS/audit-report.md" <<'MD'
# Audit Report — Cycle 99001

## Verdict
**PASS**

Confidence: 0.90.

## Evidence
Fixture for cycle-96 predicate 002 — verifies that gate_cycle_complete
recognizes the canonical Auditor verdict format and increments
state.json:mastery.consecutiveSuccesses.
MD

# ── Invoke phase-gate.sh cycle-complete ────────────────────────────────────
# Use EVOLVE_PROJECT_ROOT to redirect all state/ledger writes into $FIX.
# Use the REAL phase-gate.sh from the worktree under test (REPO_ROOT).
# Capture stdout+stderr for diagnostic; tolerate non-zero exit only if
# state mutation occurred (the gate may emit log lines that aren't fatal).
gate_log="$FIX/gate.log"
rc=0
EVOLVE_PROJECT_ROOT="$FIX" \
  bash "$PHASE_GATE" cycle-complete 99001 "$WS" >"$gate_log" 2>&1 || rc=$?

# A non-zero rc is acceptable IF the increment happened (gate may fail on
# unrelated archive issues but the mastery branch is what we test).
# Read the post-mutation state.
cs=$(jq -r '.mastery.consecutiveSuccesses // empty' "$FIX/.evolve/state.json" 2>/dev/null)

if [ -z "$cs" ]; then
  echo "RED: post-invocation state.json has no mastery.consecutiveSuccesses field" >&2
  echo "RED: gate log:" >&2
  sed 's/^/  /' "$gate_log" >&2
  exit 1
fi

if [ "$cs" != "1" ]; then
  echo "RED: state.json:mastery.consecutiveSuccesses=$cs (expected 1 after canonical PASS audit-report)" >&2
  echo "RED: phase-gate.sh exit code: $rc" >&2
  echo "RED: gate log:" >&2
  sed 's/^/  /' "$gate_log" >&2
  echo "RED: audit-report.md fixture (recap):" >&2
  sed 's/^/  /' "$WS/audit-report.md" >&2
  exit 1
fi

# Bonus assertion: totalTasksShipped also incremented (proves the full
# python3 block ran, not a partial mutation).
shipped=$(jq -r '.ledgerSummary.totalTasksShipped // empty' "$FIX/.evolve/state.json" 2>/dev/null)
if [ "$shipped" != "1" ]; then
  echo "RED: ledgerSummary.totalTasksShipped=$shipped (expected 1 — partial python3 mutation)" >&2
  exit 1
fi

echo "GREEN: phase-gate.sh cycle-complete recognized canonical '## Verdict / **PASS**' format and incremented mastery 0→1 (gate rc=$rc, shipped=$shipped)"
exit 0
