#!/usr/bin/env bash
# ACS cycle-188 — Task 2 AC1-AC4: checkPhaseLatency honours a per-phase ceiling
# override EVOLVE_<PHASE_UPPER>_LATENCY_CEILING_S (phase normalized ToUpper +
# "-"→"_"), falling back to the global ceiling when unset/invalid.
#
# BEHAVIORAL: runs the cyclehealth tests as a subprocess; pass/fail is the
# go-test EXIT CODE. Covers the positive override (WARN), the global fallback
# (no WARN — anti-no-op), the "build-planner" normalization probe, and the
# invalid-value fallback.
set -uo pipefail
. "$(git rev-parse --show-toplevel)/acs/lib/assert.sh"

assert_go_test_pass ./internal/cyclehealth/... \
  'TestCheck_PhaseLatency_PerPhaseOverride_Warn|TestCheck_PhaseLatency_NoOverride_UsesGlobal_NoWarn|TestCheck_PhaseLatency_BuildPlannerNormalization_Warn|TestCheck_PhaseLatency_InvalidOverride_FallsBackToGlobal' || exit 1
echo "PASS: per-phase latency ceiling override + normalization + fallback"
exit 0
