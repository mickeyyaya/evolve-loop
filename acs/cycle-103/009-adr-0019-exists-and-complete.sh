#!/usr/bin/env bash
# AC-ID: cycle-103-009-adr-0019-exists-and-complete
# AC-source: scout-report.md AC-10 (lines 329, 381-385), ADR template spec lines 107-116
# Behavioral predicate:
#   docs/architecture/adr/0019-build-planner-phase.md must exist and contain
#   the required ADR sections plus the 3-cycle rollout vocabulary and the
#   explicit revert path EVOLVE_BUILD_PLANNER=0.
#
# Mutation spec (cycle-103-009-MUT):
#   Mutant: ADR exists but missing "## Consequences"            -> must FAIL.
#   Mutant: ADR missing "## Decision"                            -> must FAIL.
#   Mutant: ADR missing 3-cycle rollout vocabulary               -> must FAIL.
#   Mutant: ADR missing EVOLVE_BUILD_PLANNER=0 revert path text  -> must FAIL.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

ADR="docs/architecture/adr/0019-build-planner-phase.md"

if [ ! -f "$ADR" ]; then
  echo "RED: $ADR does not exist" >&2
  exit 1
fi

# Required headings (case-sensitive, exact match at line start).
require_heading() {
  local heading="$1"
  if ! grep -qFx "$heading" "$ADR"; then
    echo "RED: $ADR missing required heading '$heading'" >&2
    return 1
  fi
  return 0
}

require_heading "## Context" || exit 1
require_heading "## Decision" || exit 1
require_heading "## Consequences" || exit 1

# Status: Proposed (per scout-report.md line 112).
if ! grep -Eq '^[*]?[*]?Status[*]?[*]?:[[:space:]]+Proposed' "$ADR"; then
  echo "RED: $ADR missing 'Status: Proposed' (allow markdown bold variants)" >&2
  exit 1
fi

# 3-cycle rollout vocabulary -- text must mention shadow, advisory, AND enforce.
if ! grep -qi 'shadow' "$ADR"; then
  echo "RED: $ADR missing '3-cycle rollout' vocabulary: 'shadow'" >&2
  exit 1
fi
if ! grep -qi 'advisory' "$ADR"; then
  echo "RED: $ADR missing '3-cycle rollout' vocabulary: 'advisory'" >&2
  exit 1
fi
if ! grep -qi 'enforce' "$ADR"; then
  echo "RED: $ADR missing '3-cycle rollout' vocabulary: 'enforce'" >&2
  exit 1
fi

# Revert path: EVOLVE_BUILD_PLANNER=0
if ! grep -q 'EVOLVE_BUILD_PLANNER=0' "$ADR"; then
  echo "RED: $ADR missing revert path 'EVOLVE_BUILD_PLANNER=0'" >&2
  exit 1
fi

echo "GREEN: $ADR has required headings, Status: Proposed, 3-cycle rollout vocab, and revert path"
exit 0
