#!/usr/bin/env bash
# ACS — cycle 236 / AC3 (part 2) + scout C2/C6: whole-suite regression green,
# build clean, and gofmt -s CI parity on every .go file this landing touches.
#
# Classification: BEHAVIORAL — go build / go test subprocess via assert.sh
# (exit-code authoritative); gofmt -s -l invoked on real files.
#
# Context (cycle-233 lesson): commit-gate gofmt != CI `gofmt -s`; commit
# 81d2c2f exists solely because cycle-233 shipped gofmt-s-dirty files. The
# rescue tip carries one such file (orchestrator_phaseboundary_test.go,
# import-order nit) — Builder must land it gofmt-s clean.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel)
cd "$TOP"
. "$TOP/acs/lib/assert.sh"

# Build clean (C6).
assert_go_build ./... || exit 1

# Whole-suite regression (C2): pre-existing GREEN at baseline; must STAY green
# with the 1241 inserted lines and their tests in the tree.
assert_go_test_pass ./... || exit 1

# gofmt -s CI parity on the touched .go files (hard-coded — predicate must not
# depend on rescue SHAs for this check; list mirrors the e565834+6f2e1af diff
# plus the scaffold-origin auditleak test).
fail=0
for f in \
  go/cmd/evolve/cmd_loop.go \
  go/cmd/evolve/cmd_loop_coverage_test.go \
  go/cmd/evolve/cmd_loop_cyclelevel_test.go \
  go/internal/checkpoint/checkpoint.go \
  go/internal/checkpoint/phaseboundary_test.go \
  go/internal/core/cyclelevel_failure_test.go \
  go/internal/core/errors.go \
  go/internal/core/orchestrator.go \
  go/internal/core/orchestrator_phaseboundary_test.go \
  go/internal/core/orchestrator_auditleak_test.go \
  go/internal/phases/ship/closure_idempotency_test.go \
  go/internal/phases/ship/gitops.go \
  go/internal/phases/ship/native.go \
  go/internal/phases/ship/postship.go
do
  if [ ! -f "$f" ]; then
    echo "RED: $f missing on disk — touched file not landed" >&2
    fail=1
    continue
  fi
  dirty=$(gofmt -s -l "$f")
  if [ -n "$dirty" ]; then
    echo "RED: gofmt -s dirty: $dirty (CI parity violation — cycle-233 mode)" >&2
    fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

# errcheck CI parity: run when the tool is present; otherwise defer to CI.
# acs-predicate: tool-availability waiver — errcheck is not installed in every
# audit environment; absence is noted, not failed (manual checklist covers CI).
if command -v errcheck >/dev/null 2>&1; then
  out=$(cd go && errcheck ./... 2>&1)
  rc=$?
  if [ "$rc" -ne 0 ]; then
    echo "RED: errcheck reported violations:" >&2
    echo "$out" | head -10 >&2
    exit 1
  fi
  echo "GREEN: errcheck ./... clean" >&2
else
  echo "NOTE: errcheck not installed — errcheck parity deferred to CI (manual checklist)" >&2
fi

echo "GREEN: build clean, whole suite green, gofmt -s parity on all touched files" >&2
exit 0
