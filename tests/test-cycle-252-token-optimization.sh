#!/usr/bin/env bash
# Cycle-252 test suite — token-usage optimization (3 tasks):
#   taco-trajectory-compression-builder, psmas-phase-skip-wire-go-router,
#   auditor-context-diet.
#
# Single-source design: every assertion lives ONCE in acs/cycle-252/*.sh
# (plus go/internal/router/router_psmas_test.go, which predicate 001
# executes). This runner is a thin projection — it executes each predicate
# and tallies, duplicating nothing.
set -uo pipefail

top=$(git rev-parse --show-toplevel)
PASS=0; FAIL=0

for p in "$top"/acs/cycle-252/*.sh; do
    name=$(basename "$p")
    if bash "$p"; then
        echo "PASS: $name"; PASS=$((PASS+1))
    else
        echo "FAIL: $name"; FAIL=$((FAIL+1))
    fi
done

echo ""
echo "Results: $PASS PASS, $FAIL FAIL"
[ "$FAIL" -eq 0 ]
