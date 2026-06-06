#!/usr/bin/env bash
# ACS cycle-218 / task fix-stale-acs-predicates AC2 — cycle100 observer-enforce
# predicate passes again after relocating its EVOLVE_OBSERVER_ENFORCE check
# from CLAUDE.md to docs/operations/runtime-reference.md (d8ac721 split).
#
# Behavioral: asserts on `go test` EXIT CODE via the shared assert lib.
# Greps are auxiliary anti-deletion guards (relocate, don't delete).
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

assert_go_test_pass ./acs/cycle100/... 'TestC100_001_ObserverEnforceDefaultOn' || exit 1

# auxiliary (not load-bearing): the assertion moved, it did not vanish
if ! grep -qF 'EVOLVE_OBSERVER_ENFORCE' "$TOP/go/acs/cycle100/predicates_test.go"; then
  echo "RED: cycle100 test no longer asserts EVOLVE_OBSERVER_ENFORCE — check was deleted, not relocated" >&2
  exit 1
fi
if ! grep -qF 'runtime-reference.md' "$TOP/go/acs/cycle100/predicates_test.go"; then
  echo "RED: cycle100 test does not target docs/operations/runtime-reference.md" >&2
  exit 1
fi
echo "GREEN: cycle100 observer-enforce predicate passes and targets runtime-reference.md" >&2
exit 0
