#!/usr/bin/env bash
# ACS cycle-188 — Task 1 AC1+AC2: the *-tmux REPL driver surfaces every
# stop-review verdict (extend AND pause) through a nil-safe
# Deps.OnStopReview(phase, action, reason) callback.
#
# BEHAVIORAL: runs the bridge tests as a subprocess; pass/fail is the go-test
# EXIT CODE (assert_go_test_pass), never a grep of source. The tests drive the
# REPL state machine with a scripted reviewer + an OnStopReview spy. Deleting
# the driver's callback invocation, or removing the Deps field, fails these.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/bridge/... 'TestRunTmuxREPL_OnStopReview' || exit 1
echo "PASS: OnStopReview callback fires for extend+pause and is nil-safe"
exit 0
