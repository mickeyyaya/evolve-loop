#!/usr/bin/env bash
# AC-ID:         cycle-231-004
# Description:   Stale dynamic_routing "off" default text purged from runtime-reference.md and dynamic-phase-routing.md; both docs now document advisory as the default; EVOLVE_DYNAMIC_ROUTING row references the registry pin
# Evidence:      docs/operations/runtime-reference.md (EVOLVE_DYNAMIC_ROUTING row) + docs/architecture/dynamic-phase-routing.md (Status line + table row)
# Author:        tester
# Created:       2026-06-06T08:30:00Z
# Acceptance-of: build-report.md Changes: runtime-reference.md — Update dynamic routing default value; dynamic-phase-routing.md — Update dynamic routing default description

set -uo pipefail

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

RR="$WORKTREE/docs/operations/runtime-reference.md"
DPR="$WORKTREE/docs/architecture/dynamic-phase-routing.md"

[ -f "$RR" ]  || { echo "RED: runtime-reference.md missing at $RR" >&2; exit 1; }
[ -f "$DPR" ] || { echo "RED: dynamic-phase-routing.md missing at $DPR" >&2; exit 1; }

# runtime-reference.md — stale text absent.
# The stale v13.0.0 parenthetical described the default as "off" with a note
# "(static state machine drives, v13.0.0/PR #4)". Its presence means the row
# was NOT updated.
if grep -q 'static state machine drives, v13.0.0' "$RR"; then
  echo "RED: runtime-reference.md still contains stale v13 'off' default text" >&2
  exit 1
fi

# runtime-reference.md — EVOLVE_DYNAMIC_ROUTING row must document the advisory
# registry pin. The positive pin is the substring 'pinned via .evolve/phase-registry.json'
# in the same row as the env var name.
awk '/EVOLVE_DYNAMIC_ROUTING/' "$RR" | grep -q 'pinned via .evolve/phase-registry.json' \
  || { echo "RED: runtime-reference.md EVOLVE_DYNAMIC_ROUTING row missing 'pinned via .evolve/phase-registry.json' — default not updated to advisory" >&2; exit 1; }

# dynamic-phase-routing.md — stale text absent.
if grep -q 'default-off' "$DPR"; then
  echo "RED: dynamic-phase-routing.md still says 'default-off'" >&2
  exit 1
fi

# dynamic-phase-routing.md — must document advisory as the default. Check the
# Status line and the table row; either is sufficient.
grep -qi 'default.*advisory\|advisory.*default' "$DPR" \
  || { echo "RED: dynamic-phase-routing.md does not document advisory as the default" >&2; exit 1; }

echo "GREEN: both docs have stale 'off' text purged and advisory default documented" >&2
exit 0
