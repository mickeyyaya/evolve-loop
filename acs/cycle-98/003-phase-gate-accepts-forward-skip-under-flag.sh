#!/usr/bin/env bash
# AC-ID: cycle-98-003-phase-gate-accepts-forward-skip-under-flag
# AC-source: cycle-98/intent.md acceptance_checks[2] ; scout-report.md TASK-98-C
# Behavioral predicate:
#   scripts/lifecycle/phase-gate.sh MUST recognize the new skip-aware
#   transitions (e.g. "triage-to-build" skipping tdd-engineer, and
#   "audit-to-complete" skipping retrospective) AND that recognition
#   MUST be gated on EVOLVE_PSMAS_SKIP=1. Bad ordering MUST still fail.
#
# We do not exec phase-gate.sh against a live ledger here (would require
# coordinated fixture). Instead we assert the script's source contains
# the dispatch cases AND a flag-check around them — Builder must wire
# both. A separate runtime test (cycle-98 manifest's smoke run) covers
# end-to-end behavior.
#
# RED until Builder modifies scripts/lifecycle/phase-gate.sh (TASK-98-C).
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (skip-aware dispatch cases present + flag-gated)
#   1 = RED   (cases missing or flag check missing)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

GATE="scripts/lifecycle/phase-gate.sh"
if [ ! -f "$GATE" ]; then
  echo "RED: $GATE missing" >&2
  exit 1
fi

fail_count=0

# 1) Flag-name presence — phase-gate.sh must reference the opt-in env var.
if ! grep -q 'EVOLVE_PSMAS_SKIP' "$GATE"; then
  echo "RED: $GATE does not reference EVOLVE_PSMAS_SKIP" >&2
  fail_count=$(( fail_count + 1 ))
fi

# 2) At least one new skip-aware dispatch case must appear. We accept
#    either of the two patterns Scout enumerated as canonical.
if ! grep -Eq '(triage-to-build|audit-to-complete)' "$GATE"; then
  echo "RED: $GATE missing skip-aware dispatch case (triage-to-build or audit-to-complete)" >&2
  fail_count=$(( fail_count + 1 ))
fi

# 3) Forward-order enforcement: the implementation must reject the new
#    case when the flag is unset. We assert presence of a guard token
#    referencing the flag in close proximity to the new case names.
#    Heuristic: grep -A 8 the case name; the snippet must mention the
#    flag check.
guarded=0
for case_name in triage-to-build audit-to-complete; do
  if grep -q "$case_name" "$GATE"; then
    if grep -A 12 -E "[\"']?${case_name}[\"']?" "$GATE" 2>/dev/null \
       | grep -q 'EVOLVE_PSMAS_SKIP'; then
      guarded=1
      break
    fi
  fi
done

if [ "$guarded" -ne 1 ]; then
  echo "RED: $GATE new skip-aware case(s) are not gated on EVOLVE_PSMAS_SKIP nearby" >&2
  fail_count=$(( fail_count + 1 ))
fi

# 4) The script must still mention rejection / fail of out-of-order
#    transitions — sanity check that we have not deleted the kernel
#    invariant. (The existing "fail" helper or "Unknown gate" line is
#    enough.)
if ! grep -Eq 'Unknown gate|fail "' "$GATE"; then
  echo "RED: $GATE no longer contains a generic gate-rejection path" >&2
  fail_count=$(( fail_count + 1 ))
fi

if [ "$fail_count" -ne 0 ]; then
  echo "RED: phase-gate.sh skip-aware contract incomplete ($fail_count issue[s])" >&2
  exit 1
fi

echo "GREEN: $GATE wires skip-aware dispatch case(s), gated on EVOLVE_PSMAS_SKIP, preserving reject-bad-transition invariant"
exit 0
