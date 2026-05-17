#!/usr/bin/env bash
# ACS predicate 002 — cycle 68
# Asserts that scripts/dispatch/subagent-run.sh's turn-overrun log
# message uses the clearer "turns=X vs ceiling=Y" form rather than
# the misleading "${X}x ceiling" form (which made 10 turns vs 4
# ceiling read as "10x ceiling", implying 40 turns).
#
# AC-ID: cycle-68-002
# Description: overrun-log-format
# Evidence: file-read of scripts/dispatch/subagent-run.sh asserts the
#           new "turns=X vs ceiling=Y" format string is present, the
#           legacy "${X}x ceiling" form is absent, and the structured
#           "turn-overrun" event-name token is preserved.
# Author: builder
# Created: 2026-05-17T00:00:00Z
# Acceptance-of: scout-report.md cycle-68 Task 2
#
# metadata:
#   id: 002-overrun-log-format
#   cycle: 68
#   task: fix-overrun-log-format
#   severity: LOW

set -uo pipefail

# Resolve repo root from this script's own location (see predicate 001
# for rationale): worktree state must be validated during audit, not
# the main-repo state EVOLVE_PROJECT_ROOT would point to.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
SCRIPT="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

if [ ! -f "$SCRIPT" ]; then
    echo "RED: subagent-run.sh not found at $SCRIPT" >&2
    exit 1
fi

content=$(cat "$SCRIPT")

# Required: new clarified format string must be present.
if ! [[ "$content" == *'(turns=${_actual_turns} vs ceiling=${_max_turns_profile})'* ]]; then
    echo "RED: new format '(turns=\${_actual_turns} vs ceiling=\${_max_turns_profile})' not found" >&2
    exit 1
fi

# Forbidden: old misleading "(${var}x ceiling)" form must not remain on
# the turn-overrun event line. Tolerate the substring elsewhere only if
# truly unrelated (currently nowhere else uses it).
if [[ "$content" == *'(${_actual_turns}x ceiling)'* ]]; then
    echo "RED: legacy misleading format '(\${_actual_turns}x ceiling)' still present" >&2
    exit 1
fi

# Sanity: the turn-overrun event name token must still be emitted
# (structured field consumers depend on it).
if ! [[ "$content" == *'"turn-overrun"'* ]]; then
    echo "RED: 'turn-overrun' event-name token missing — log refactor broke event semantics" >&2
    exit 1
fi

echo "GREEN: turn-overrun log format updated; event-name token preserved"
exit 0
