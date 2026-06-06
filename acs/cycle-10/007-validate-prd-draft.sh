#!/usr/bin/env bash
# AC-ID:         cycle-10-007
# Description:   `evolve phases validate prd-draft` exits 0 and reports OK (user-phase safety floor holds)
# Evidence:      go/cmd/evolve/cmd_phases.go (phasesValidate) run against worktree .evolve/phases/
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md AC#4 — wave-product-discovery-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

# Behavioral: invoke the validator built from THIS worktree's source against
# THIS worktree's .evolve/phases/ (the installed ~/go/bin binary may be stale).
out=$(cd go && EVOLVE_PROJECT_ROOT="$WORKTREE" go run ./cmd/evolve phases validate prd-draft 2>&1); rc=$?
if [ "$rc" -ne 0 ]; then
  echo "RED: phases validate prd-draft exited $rc" >&2
  echo "$out" | tail -5 >&2
  exit 1
fi
if ! echo "$out" | grep -q '^OK[[:space:]]*prd-draft'; then
  echo "RED: validator did not report OK for prd-draft: $out" >&2
  exit 1
fi

echo "GREEN: evolve phases validate prd-draft → OK" >&2
exit 0
