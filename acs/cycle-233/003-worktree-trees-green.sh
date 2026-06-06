#!/usr/bin/env bash
# AC-ID:         cycle-233-AC3
# Description:   touched package trees green from the worktree root (composes with regression acs/cycle-232/001)
# Evidence:      go build ./... + go test ./internal/phases/ship/...; cycle-232/001 covers ./cmd/evolve + ./internal/core + ./internal/phasespec
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC3 (go test from the worktree root — all PASS, no regression)
#
# Composition note: regression predicate acs/cycle-232/001-land-packages-green.sh
# already asserts ./cmd/evolve/... ./internal/core/... ./internal/phasespec/...
# green on every suite run. This predicate adds the fourth touched tree
# (./internal/phases/ship/...) plus a whole-module build assert, so
# union(232-001, 233-003) = all four cherry-pick-touched trees, without
# doubling the suite's go-test runtime. The FULL `go test ./...` run is the
# Builder's verifiableBy step and an explicit Auditor checklist item.

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

# Whole-module compile pin (cheap, catches cross-tree breakage).
assert_go_build ./... || exit 1

# Behavioral: the touched tree that 232-001 does not cover.
assert_go_test_pass ./internal/phases/ship/... || exit 1

echo "GREEN: module builds; ship tree green (232-001 covers the other three trees)" >&2
exit 0
