#!/usr/bin/env bash
# AC-ID:         cycle-230-003
# Description:   evolve acs suite resolves its root from cycle-state.json active_worktree (kernel-owned topology, mode 5 of user-phase-persona-resolution)
# Evidence:      go/cmd/evolve/cmd_acs.go (resolveACSSuiteRoot) + go/cmd/evolve/cmd_acs_test.go
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 3 (acs-suite-root-autosolve) — auditor C0 block becomes defense-in-depth only

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Behavioral: happy path (active_worktree honored) + fallback matrix (absent /
# empty / missing key / malformed JSON) — exit-code assertion per cycle-137.
assert_go_test_pass ./cmd/evolve/... 'TestACSSuiteRootAutosolve|TestACSSuiteRootFallback' || exit 1

# No collateral damage: the whole cmd/evolve package must stay green.
assert_go_test_pass ./cmd/evolve/... || exit 1

# Auxiliary (NOT load-bearing): the resolver must be wired to active_worktree
# in cmd_acs.go, not implemented elsewhere and left unwired.
grep -q 'active_worktree' "$WORKTREE/go/cmd/evolve/cmd_acs.go" \
  || { echo "RED: active_worktree not referenced in cmd_acs.go — resolver not wired into the suite command" >&2; exit 1; }

echo "GREEN: ACS suite root autosolve behavioral tests pass; resolver wired in cmd_acs.go" >&2
exit 0
