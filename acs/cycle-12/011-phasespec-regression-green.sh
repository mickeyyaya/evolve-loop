#!/usr/bin/env bash
# AC-ID:         cycle-12-011
# Description:   Behavioral regression — go test ./internal/phasespec/... must
#                stay green (build plan step 6; doc-only cycle must not break
#                the phase-spec loader). Uses acs/lib/assert.sh per the
#                cycle-137 mandate: assert on go test's EXIT CODE, never on
#                scraping PASS tokens.
# Evidence:      go test ./internal/phasespec/... exit code
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Build Plan step 6 (supplementary regression)
# NOTE: negative invariant — expected GREEN at RED baseline AND after build.
set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

. "$WORKTREE/acs/lib/assert.sh"

assert_go_test_pass ./internal/phasespec/... || exit 1
exit 0
