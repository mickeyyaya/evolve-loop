#!/usr/bin/env bash
# ACS cycle-202 / AC1 — `go build ./...` stays green.
#
# Regression guard: the new marketplacepoll tests must not break the module
# build. GREEN at baseline (build already compiles) and must stay GREEN after
# Builder appends the four coverage tests. Behavioral: asserts on `go build`
# exit code via the shared assert lib (never scrapes stdout).
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

assert_go_build "./..." || exit 1
exit 0
