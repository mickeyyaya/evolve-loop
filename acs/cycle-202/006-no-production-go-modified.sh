#!/usr/bin/env bash
# ACS cycle-202 / AC6 — no production .go file is modified (test-only cycle).
#
# Scope guard: this is a coverage-hardening cycle; only *_test.go may change.
# Behavioral: diffs the cycle worktree against the base branch for tracked .go
# files and asserts that every changed .go path ends in _test.go. Comparing
# against the base ref (not HEAD) captures the cycle's full change regardless
# of whether Builder committed, so it cannot be evaded by an intermediate
# commit. GREEN at baseline (nothing changed) and after a test-only Build; RED
# the moment any non-test .go is touched.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }

BASE=main
git -C "$TOP" rev-parse --verify --quiet "$BASE" >/dev/null 2>&1 || BASE=HEAD

PROD=$(git -C "$TOP" diff --name-only "$BASE" -- '*.go' 2>/dev/null | grep -v '_test\.go$' || true)
if [ -n "$PROD" ]; then
  echo "RED: production (non-test) .go files modified vs ${BASE}:" >&2
  echo "$PROD" >&2
  exit 1
fi
echo "GREEN: no production .go files modified (only *_test.go vs ${BASE})" >&2
exit 0
