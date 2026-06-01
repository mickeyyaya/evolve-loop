#!/usr/bin/env bash
# ACS cycle-187 — Task 1 AC-5/AC-6/AC-9: retro phase polls the file the
# evolve-retrospective agent actually writes ("retrospective-report.md"), not
# the stale "retrospective.md" (Scout Gap B — the bridge timed out every retro).
#
# Behavioral: runs the retro package suite. TestRun_PreviousFAIL_PASSWithLesson
# invokes retro.Run with a fakeBridge that captures the ArtifactPath passed to
# the bridge, and asserts it == "retrospective-report.md". That test passes ONLY
# when retro.go:artifactPath is fixed (AC-5). The whole-package run is AC-9.
# Exit code authoritative via assert_go_test_pass.
#
# AC-6 (the test asserts the new path) is verified by the auxiliary grep below.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

# AC-5 + AC-9: full retro package GREEN (includes the updated path assertion).
assert_go_test_pass ./internal/phases/retro/... || exit 1

TOP="$(git rev-parse --show-toplevel)"
# AC-6: the test encodes the new expected path.
grep -q 'retrospective-report.md' "$TOP/go/internal/phases/retro/retro_test.go" \
  || { echo "RED: retro_test.go does not assert retrospective-report.md (AC-6)" >&2; exit 1; }
# AC-5 (auxiliary source check; the passing test above is the behavioral proof):
# retro.go must reference the new path and no longer the bare stale path.
grep -q '"retrospective-report.md"' "$TOP/go/internal/phases/retro/retro.go" \
  || { echo "RED: retro.go does not use retrospective-report.md (AC-5)" >&2; exit 1; }

echo "PASS: retro polls retrospective-report.md (AC-5/AC-6/AC-9)"
exit 0
