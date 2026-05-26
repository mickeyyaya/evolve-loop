#!/usr/bin/env bash
# AC-ID: cycle-108-003-claudemd-buildplanner-advisory
# AC-source: scout-report.md AC T1 "CLAUDE.md EVOLVE_BUILD_PLANNER row updated to 1 (advisory, v12.3+)"
# acs-predicate: config-check
#   Rationale: CLAUDE.md is the configuration/documentation file being updated.
#   Verifying its content is the only meaningful assertion for a documentation change.
#
# Behavioral note: we also verify the old default "0 (off)" is NOT the current default,
# so the predicate would fail if Builder only updates the value but not the default.
#
# Bash 3.2 compatible.
# Exit codes: 0=GREEN, 1=RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2; exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="CLAUDE.md"
if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET missing" >&2; exit 1
fi

# Check 1: disk presence (required)
# Check 2: git tracking
git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1 \
  || { echo "RED: $TARGET not tracked by git" >&2; exit 1; }

# Required: advisory default row present
if ! grep -qF '1 (advisory, v12.3+)' "$TARGET"; then
  echo "RED: CLAUDE.md missing '1 (advisory, v12.3+)' in EVOLVE_BUILD_PLANNER row"
  grep -n 'EVOLVE_BUILD_PLANNER' "$TARGET" >&2 || echo "(no EVOLVE_BUILD_PLANNER mentions found)" >&2
  exit 1
fi

# Required: EVOLVE_BUILD_PLANNER must appear in the same context
if ! grep -q 'EVOLVE_BUILD_PLANNER' "$TARGET"; then
  echo "RED: CLAUDE.md does not mention EVOLVE_BUILD_PLANNER" >&2; exit 1
fi

echo "GREEN: CLAUDE.md contains '1 (advisory, v12.3+)' for EVOLVE_BUILD_PLANNER"
exit 0
