#!/usr/bin/env bash
# AC-ID:         cycle-232-t1-AC1
# Description:   cycle-230 @ 201f7cb landed — touched packages all green post-cherry-pick
# Evidence:      go/cmd/evolve + go/internal/core + go/internal/phasespec test suites
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 1 (land-audited-resolution-fix) AC1

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

# Landing marker (auxiliary, NOT load-bearing): resolveACSSuiteRoot is the
# cycle-230 content; its absence means the cherry-pick never happened, so the
# behavioral runs below would vacuously pass on the pre-land tree.
grep -q 'resolveACSSuiteRoot' "$ROOT/go/cmd/evolve/cmd_acs.go" \
  || { echo "RED: resolveACSSuiteRoot not in cmd_acs.go — cycle-230 @ 201f7cb not landed" >&2; exit 1; }

# Behavioral (load-bearing): the three cherry-pick-touched package trees must
# be green on the landed tree — exit-code assertion per cycle-137.
assert_go_test_pass ./cmd/evolve/... || exit 1
assert_go_test_pass ./internal/core/... || exit 1
assert_go_test_pass ./internal/phasespec/... || exit 1

echo "GREEN: cycle-230 landed and all touched packages pass" >&2
exit 0
