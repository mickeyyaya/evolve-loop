#!/usr/bin/env bash
# AC-ID: cycle-102-003-incident-doc-ship-refused
# AC-source: cycle-102/scout-report.md AC-6 (carryover abnormal-ship-refused-c100)
#
# Behavioral predicate: the cycle-100 ship-refused incident doc must
# exist at the canonical path, be tracked by git, contain >=30
# non-blank lines, and document all three concurrent root causes
# scout identified:
#
#   Reason 1: auditor ledger entry race
#   Reason 2: ship.sh SHA drift (now self-healed)
#   Reason 3: TTY constraint / EVOLVE_SHIP_AUTO_CONFIRM
#
# Path: docs/operations/incidents/cycle-100-ship-refused.md
#
# Predicate is BEHAVIORAL on side-effect: invokes git and grep
# subprocesses; assertions are on a doc artifact, not source-code
# semantics (acs/AGENTS.md mixed-grep waiver).
#
# Dual-check pattern (file + git-tracking) required by cycle-93 guide.
#
# Substance check: at least 2 of the 3 root-cause anchor strings must
# be present (ledger|race, ship.sh|SHA, TTY|EVOLVE_SHIP_AUTO_CONFIRM).
# Two-of-three avoids brittleness while still rejecting one-cause
# stubs.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN (doc exists, tracked, dense, covers >=2 of 3 root causes)
#   1 = RED   (missing, untracked, thin, or insufficient cause coverage)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd to repo root failed" >&2; exit 1; }

DOC="docs/operations/incidents/cycle-100-ship-refused.md"
MIN_LINES=30
MIN_CAUSES=2

# Disk presence
if [ ! -f "$DOC" ]; then
  echo "RED: $DOC missing on disk" >&2
  exit 1
fi

# Git tracking
if ! git ls-files --error-unmatch "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC exists on disk but is not tracked by git (worktree-only)" >&2
  exit 1
fi

# Content density
nonblank=$(grep -cvE '^[[:space:]]*$' "$DOC" 2>/dev/null || echo 0)
if [ "$nonblank" -lt "$MIN_LINES" ]; then
  echo "RED: $DOC has only $nonblank non-blank lines (need >=$MIN_LINES)" >&2
  exit 1
fi

# Root-cause coverage: count matches of each cause anchor.
causes_covered=0
cause_labels=""

# Cause 1: auditor ledger / race
if grep -qiE '(auditor.*ledger|ledger.*race|race.*condition|missing.*ledger|no auditor ledger)' "$DOC" 2>/dev/null; then
  causes_covered=$(( causes_covered + 1 ))
  cause_labels="${cause_labels}ledger-race "
fi

# Cause 2: ship.sh SHA drift
if grep -qiE '(ship\.sh.*sha|sha.*ship\.sh|sha.*drift|sha.*mismat|expected_ship_sha|e7dab80f)' "$DOC" 2>/dev/null; then
  causes_covered=$(( causes_covered + 1 ))
  cause_labels="${cause_labels}sha-drift "
fi

# Cause 3: TTY / EVOLVE_SHIP_AUTO_CONFIRM
if grep -qiE '(tty|stdin|EVOLVE_SHIP_AUTO_CONFIRM|--class manual)' "$DOC" 2>/dev/null; then
  causes_covered=$(( causes_covered + 1 ))
  cause_labels="${cause_labels}tty "
fi

if [ "$causes_covered" -lt "$MIN_CAUSES" ]; then
  echo "RED: $DOC covers only $causes_covered of 3 root causes (need >=$MIN_CAUSES). Covered: [${cause_labels:-none}]" >&2
  exit 1
fi

echo "GREEN: $DOC exists ($nonblank non-blank lines, covers $causes_covered/3 root causes: ${cause_labels}), git-tracked"
exit 0
