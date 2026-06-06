#!/usr/bin/env bash
# AC-ID:         cycle-234-T3 (task ship-closure-idempotency, ACs 1-4)
# Description:   post-push correction = report-only; worktree binary churn discarded (source preserved); expected_ship_sha pinned post-commit
# Evidence:      go test (exit code) on the cycle-234 RED tests in internal/phases/ship (real-git behavioral fixtures)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: inbox ship-closure-idempotency (3 defects, cycle-233 landing saga) folded into intent.md AC1's ship dead-end case
#
# Behavioral: every test runs the REAL native ship state machine against a
# real git repo + bare remote + worktree, asserting on git history, remote
# refs and state.json — a magic string cannot satisfy any of them.

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/phases/ship/... 'TestShip_PostPush_Idempotent_CorrectReportOnly|TestShip_PrePush_CorrectionStillFullShip|TestShip_BinaryChurnDiscarded|TestShip_SourceChangesPreserved|TestShip_PinPostCommitSha' || exit 1

echo "GREEN: ship-closure idempotency verified (post-push report-only + churn discard + post-commit pin)" >&2
exit 0
