#!/usr/bin/env bash
# AC-ID: cycle-104-005-control-flags-advisory-default-on
# AC-source: scout-report.md AC EG-104-05 (lines 230-233); intent.md acceptance_checks[4]
# Behavioral predicate (CONTROL-FLAGS doc consistency):
#   docs/architecture/control-flags.md must document EVOLVE_BUILD_PLANNER's
#   new status as "advisory v10.20; default on" (replacing the cycle-103
#   text "wired v10.19; default off"). The EVOLVE_BUILD_PLANNER row must
#   appear in the file AND must include the literal status string
#   "advisory v10.20; default on" so operators can grep for the active mode.
#
#   This is a docs-source-of-truth invariant: a flag flip without the
#   corresponding doc update is documentation drift that violates the
#   "everything learned/applied must be documented" stewardship rule.
#
# Mutation spec (cycle-104-005-MUT):
#   Mutant: row still says "default off"                          -> must FAIL.
#   Mutant: row updated but version omits "v10.20"                -> must FAIL.
#   Mutant: EVOLVE_BUILD_PLANNER row deleted                      -> must FAIL.
#
# Bash 3.2 compatible. No GNU-only flags.
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

DOC="docs/architecture/control-flags.md"

if [ ! -f "$DOC" ]; then
  echo "RED: $DOC does not exist" >&2
  exit 1
fi

# Required: EVOLVE_BUILD_PLANNER row must exist.
if ! grep -q 'EVOLVE_BUILD_PLANNER' "$DOC"; then
  echo "RED: $DOC missing EVOLVE_BUILD_PLANNER row entirely" >&2
  exit 1
fi

# Required: status string must contain "advisory v10.20; default on".
if ! grep -qF 'advisory v10.20; default on' "$DOC"; then
  echo "RED: $DOC missing literal 'advisory v10.20; default on' (cycle-103 'default off' not updated)" >&2
  echo "Current EVOLVE_BUILD_PLANNER row:" >&2
  grep -n 'EVOLVE_BUILD_PLANNER' "$DOC" >&2 || true
  exit 1
fi

# Defensive: the EVOLVE_BUILD_PLANNER row must not still claim "default off"
# (cycle-103 stale text). Look for the two tokens together on a single line.
if grep -E 'EVOLVE_BUILD_PLANNER.*default off|default off.*EVOLVE_BUILD_PLANNER' "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC EVOLVE_BUILD_PLANNER row still says 'default off' (cycle-103 stale text)" >&2
  grep -n 'EVOLVE_BUILD_PLANNER' "$DOC" >&2 || true
  exit 1
fi

echo "GREEN: $DOC contains 'advisory v10.20; default on' for EVOLVE_BUILD_PLANNER"
exit 0
