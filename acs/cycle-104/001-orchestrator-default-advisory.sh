#!/usr/bin/env bash
# AC-ID: cycle-104-001-orchestrator-default-advisory
# AC-source: scout-report.md AC EG-104-01 (lines 210-213); intent.md acceptance_checks[0]
# Behavioral predicate (ADVISORY invariant, v10.20):
#   agents/evolve-orchestrator-reference.md must default EVOLVE_BUILD_PLANNER
#   to 1 (advisory mode). The literal "${EVOLVE_BUILD_PLANNER:-1}" must appear
#   in the orchestrator's build-planner conditional. The cycle-103 shadow
#   default of :-0 must NOT remain anywhere in this file.
#
# Mutation spec (cycle-104-001-MUT):
#   Mutant: orchestrator default :- 0 (cycle-103 state)            -> must FAIL.
#   Mutant: orchestrator gate removed entirely                     -> must FAIL.
#   Mutant: EVOLVE_BUILD_PLANNER referenced without :- default     -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags. No declare -A, no mapfile.
#
# Exit codes:
#   0 = GREEN (predicate satisfied)
#   1 = RED   (predicate violated)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

ORCH_REF="agents/evolve-orchestrator-reference.md"

if [ ! -f "$ORCH_REF" ]; then
  echo "RED: $ORCH_REF does not exist" >&2
  exit 1
fi

# Must mention EVOLVE_BUILD_PLANNER at all.
if ! grep -q 'EVOLVE_BUILD_PLANNER' "$ORCH_REF"; then
  echo "RED: $ORCH_REF does not mention EVOLVE_BUILD_PLANNER" >&2
  exit 1
fi

# Required: the advisory default ${EVOLVE_BUILD_PLANNER:-1} must appear.
# Accept optional whitespace inside the :- default to match cycle-103 tolerance.
if ! grep -Eq '\$\{EVOLVE_BUILD_PLANNER:-[[:space:]]*1\}' "$ORCH_REF"; then
  echo "RED: $ORCH_REF missing advisory default \${EVOLVE_BUILD_PLANNER:-1} (v10.20)" >&2
  echo "Found these EVOLVE_BUILD_PLANNER lines:" >&2
  grep -n 'EVOLVE_BUILD_PLANNER' "$ORCH_REF" >&2 || true
  exit 1
fi

# Forbidden: the shadow default ${EVOLVE_BUILD_PLANNER:-0} must NOT remain.
if grep -Eq '\$\{EVOLVE_BUILD_PLANNER:-[[:space:]]*0\}' "$ORCH_REF"; then
  echo "RED: $ORCH_REF still contains shadow default \${EVOLVE_BUILD_PLANNER:-0} (cycle-103 state)" >&2
  grep -n 'EVOLVE_BUILD_PLANNER:-[[:space:]]*0' "$ORCH_REF" >&2 || true
  exit 1
fi

echo "GREEN: orchestrator advisory default \${EVOLVE_BUILD_PLANNER:-1} present; shadow :-0 absent"
exit 0
