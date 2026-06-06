#!/usr/bin/env bash
# AC-ID:         cycle-234-AC5 (intent.md: go test ./internal/... — N/N PASS, no regression)
# Description:   module builds + the two touched trees NOT covered by the standing regression predicate stay green
# Evidence:      go build ./... + go test ./internal/checkpoint/... ./internal/phases/ship/...
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC5 (no regression)
#
# Composition note: acs/cycle-232/001-land-packages-green.sh already asserts
# ./cmd/evolve/... ./internal/core/... ./internal/phasespec/... green on every
# suite run. This predicate adds the other trees cycle-234 touches
# (./internal/checkpoint/..., ./internal/phases/ship/...) plus a whole-module
# build pin, so union(232-001, 234-004) covers all four touched trees without
# doubling go-test runtime. The FULL `go test ./...` is Builder's verifiableBy
# step and an explicit Auditor checklist item.

set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

# Whole-module compile pin (cheap, catches cross-tree breakage).
assert_go_build ./... || exit 1

assert_go_test_pass ./internal/checkpoint/... || exit 1
assert_go_test_pass ./internal/phases/ship/... || exit 1

echo "GREEN: module builds; checkpoint + ship trees green (232-001 covers cmd/evolve + core + phasespec)" >&2
exit 0
