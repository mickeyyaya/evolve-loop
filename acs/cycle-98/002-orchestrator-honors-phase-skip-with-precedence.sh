#!/usr/bin/env bash
# AC-ID: cycle-98-002-orchestrator-honors-phase-skip-with-precedence
# AC-source: cycle-98/intent.md acceptance_checks[1] ; scout-report.md TASK-98-B
# Behavioral predicate:
#   agents/evolve-orchestrator.md MUST document
#     (a) reading triage.phase_skip[] when EVOLVE_PSMAS_SKIP=1,
#     (b) emitting kind:phase_skipped ledger entries for each skipped phase,
#     (c) the precedence merge rule resolving the multi-writer hazard
#         identified in cycle-98 intent.md challenged_premises:
#         adapter.skip_phases[] wins; triage.phase_skip[] is additive only.
#
# RED until Builder edits agents/evolve-orchestrator.md (TASK-98-B); GREEN after.
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (all three documentation requirements satisfied)
#   1 = RED   (any of {field read, ledger emit, precedence rule} missing)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

ORCH="agents/evolve-orchestrator.md"
if [ ! -f "$ORCH" ]; then
  echo "RED: $ORCH missing" >&2
  exit 1
fi

fail_count=0

# (a) phase_skip read + opt-in flag gate
if ! grep -q 'phase_skip' "$ORCH"; then
  echo "RED: $ORCH does not document reading triage.phase_skip[]" >&2
  fail_count=$(( fail_count + 1 ))
fi
if ! grep -q 'EVOLVE_PSMAS_SKIP' "$ORCH"; then
  echo "RED: $ORCH does not gate phase_skip behavior on EVOLVE_PSMAS_SKIP" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (b) kind:phase_skipped ledger entry contract
if ! grep -Eq 'phase_skipped' "$ORCH"; then
  echo "RED: $ORCH does not document kind:phase_skipped ledger entries" >&2
  fail_count=$(( fail_count + 1 ))
fi

# (c) precedence rule. We require BOTH skip_phases (the adapter field name)
#     and a precedence/additive verb token to appear, so the merge rule is
#     unambiguous in the spec.
if ! grep -q 'skip_phases' "$ORCH"; then
  echo "RED: $ORCH does not reference adapter skip_phases[]" >&2
  fail_count=$(( fail_count + 1 ))
fi
if ! grep -Eqi 'precedence|additive|union|adapter wins|adapter takes precedence' "$ORCH"; then
  echo "RED: $ORCH does not document a precedence/additive merge rule between adapter.skip_phases and triage.phase_skip" >&2
  fail_count=$(( fail_count + 1 ))
fi

if [ "$fail_count" -ne 0 ]; then
  echo "RED: orchestrator phase_skip contract incomplete ($fail_count issue[s])" >&2
  exit 1
fi

echo "GREEN: $ORCH documents phase_skip read, kind:phase_skipped emission, and adapter>triage precedence"
exit 0
