#!/usr/bin/env bash
# ACS — cycle-256 task `scrollback-lines-configurable`
# Behavioral: drives the real claude-tmux REPL engine and inspects the actual
# scrollback argument passed to CapturePane (the fake records every 3rd-arg).
# `go test` EXIT CODE is authoritative (assert.sh; cycle-137 lesson).
#   - EVOLVE_SCROLLBACK_LINES=2000 → final capture uses 2000 (not 10000)
#   - unset / 0 / -5 / abc → fall back to 10000 at both capture sites
# Contract source: go/internal/bridge/scrollback_lines_test.go.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/bridge/ 'TestScrollbackLines' || exit 1

echo "GREEN: configurable tmux scrollback depth behavioral suite passes"
exit 0
