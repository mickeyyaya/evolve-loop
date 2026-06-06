#!/usr/bin/env bash
# AC-ID:         cycle-6-007
# Description:   `evolve phases validate scope-baseline` exits 0 and reports OK (user-phase safety floor holds)
# Evidence:      go/cmd/evolve/cmd_phases.go (phasesValidate) run against worktree .evolve/phases/
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#7 — wave-pm-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

# Behavioral: invoke the validator built from THIS worktree's source against
# THIS worktree's .evolve/phases/ (the installed ~/go/bin binary may be stale).
out=$(cd go && EVOLVE_PROJECT_ROOT="$WORKTREE" go run ./cmd/evolve phases validate scope-baseline 2>&1); rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: phases validate scope-baseline exited $rc" >&2
  echo "$out" | tail -5 >&2
  exit 1
fi
if ! echo "$out" | grep -q '^OK[[:space:]]*scope-baseline'; then
  echo "RED: validator did not report OK for scope-baseline: $out" >&2
  exit 1
fi

echo "GREEN: evolve phases validate scope-baseline → OK" >&2
exit 0
