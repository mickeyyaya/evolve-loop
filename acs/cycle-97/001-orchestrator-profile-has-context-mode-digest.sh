#!/usr/bin/env bash
# AC-ID: cycle-97-001-orchestrator-profile-has-context-mode-digest
# AC-source: cycle-97/intent.md acceptance_checks[1] ; scout-report.md T1
# Behavioral predicate: .evolve/profiles/orchestrator.json MUST have a
#   top-level field "context_mode" with value "digest". This is L1's
#   declarative half — the profile flag that the loader-side bridge
#   (002 predicate) consumes.
#
# RED until Builder adds the field; GREEN once present.
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (profile field present with value "digest")
#   1 = RED   (field missing, wrong value, or profile unreadable)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

PROFILE=".evolve/profiles/orchestrator.json"
if [ ! -f "$PROFILE" ]; then
  echo "RED: $PROFILE missing" >&2
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "RED: jq required for this predicate (verifies JSON shape)" >&2
  exit 1
fi

if ! jq -e '.context_mode == "digest"' "$PROFILE" >/dev/null 2>&1; then
  echo "RED: $PROFILE missing top-level .context_mode == \"digest\"" >&2
  echo "current .context_mode value:" >&2
  jq -r '.context_mode // "(absent)"' "$PROFILE" >&2 || true
  exit 1
fi

echo "GREEN: $PROFILE has context_mode=digest"
exit 0
