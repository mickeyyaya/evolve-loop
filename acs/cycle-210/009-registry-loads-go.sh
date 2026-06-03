#!/usr/bin/env bash
# ACS cycle-210 — REGRESSION GUARD (behavioral): the real registry consumer
# (go/internal/config) must still parse docs/architecture/phase-registry.json
# after the spec-verify + doc-sync entries are added.
#
# This runs the actual Go loader against the actual repo registry
# (TestLoad_RealRegistry reads ../../../docs/architecture/phase-registry.json),
# so it is the load-bearing behavioral check that the new entries are
# structurally consumable — not a grep. It validates Scout hypothesis H3
# ("zero-Go specrunner pattern; registry still loads").
#
# UNLIKE predicates 001-008 this is GREEN at baseline (the registry is already
# valid) and must STAY green — it goes RED only if Builder inserts a malformed
# or structurally-invalid phase entry. Asserts on `go test` EXIT CODE via the
# shared lib (cycle-137 lesson: never scrape PASS/ok from stdout).
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
. "$TOP/acs/lib/assert.sh"

assert_go_test_pass ./internal/config/... '^TestLoad_RealRegistry$' || exit 1
echo "GREEN: real phase-registry.json still loads via go/internal/config" >&2
exit 0
