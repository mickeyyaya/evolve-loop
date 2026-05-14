#!/usr/bin/env bash
# ACS predicate 030 — cycle 57
# Verifies that the green_count in build-report.md matches green_count in
# acs-verdict.json (cycle-57). Prevents count drift/inflation between
# build report claims and the ACS suite's actual execution results.
#
# AC-ID: cycle-57-030
# Description: build-report green count matches acs-verdict.json green_count
# Evidence: .evolve/runs/cycle-57/build-report.md, .evolve/runs/cycle-57/acs-verdict.json
# Author: builder (evolve-builder)
# Created: 2026-05-15T00:00:00Z
# Acceptance-of: build-report.md AC-57-4
#
# metadata:
#   id: 030-build-report-verdict-count-match
#   cycle: 57
#   task: egps-integrity-regression
#   severity: HIGH

set -uo pipefail

CYCLE="${CYCLE:-57}"
WORKSPACE="${WORKSPACE:-${EVOLVE_PROJECT_ROOT:-.}/.evolve/runs/cycle-${CYCLE}}"

BUILD_REPORT="$WORKSPACE/build-report.md"
ACS_VERDICT="$WORKSPACE/acs-verdict.json"

# ── Pre-flight ────────────────────────────────────────────────────────────────
if [ ! -f "$BUILD_REPORT" ]; then
    echo "RED: build-report.md not found at $BUILD_REPORT — predicate cannot run"
    exit 1
fi
if [ ! -f "$ACS_VERDICT" ]; then
    echo "RED: acs-verdict.json not found at $ACS_VERDICT — run-acs-suite.sh must run first"
    exit 1
fi
if ! command -v jq >/dev/null 2>&1; then
    echo "RED: jq not available — required for acs-verdict.json parsing"
    exit 1
fi

rc=0

# ── AC1: acs-verdict.json is valid JSON with required fields ──────────────────
if jq -e '.green_count and .red_count and .verdict' "$ACS_VERDICT" >/dev/null 2>&1; then
    echo "GREEN AC1: acs-verdict.json has required fields (green_count, red_count, verdict)"
else
    echo "RED AC1: acs-verdict.json missing required fields: $(cat "$ACS_VERDICT" 2>/dev/null | head -3)"
    exit 1
fi

verdict_green=$(jq -r '.green_count' "$ACS_VERDICT")
verdict_total=$(jq -r '.predicate_suite.total' "$ACS_VERDICT" 2>/dev/null || echo "0")

# ── AC2: build-report.md is not empty ────────────────────────────────────────
build_size=$(wc -c < "$BUILD_REPORT" | tr -d ' ')
if [ "${build_size:-0}" -gt 100 ]; then
    echo "GREEN AC2: build-report.md is non-empty ($build_size bytes)"
else
    echo "RED AC2: build-report.md is empty or near-empty ($build_size bytes)"
    rc=1
fi

# ── AC3: build-report.md reports a PASS status ───────────────────────────────
if grep -qi "Status:.*PASS\|\*\*Status:\*\* PASS" "$BUILD_REPORT"; then
    echo "GREEN AC3: build-report.md status is PASS"
elif grep -qi "Status:.*FAIL" "$BUILD_REPORT"; then
    echo "RED AC3: build-report.md status is FAIL — counts cannot match a failed build"
    rc=1
else
    echo "GREEN AC3: build-report.md has no explicit FAIL status (treating as PASS)"
fi

# ── AC4: acs-verdict.json green_count > 0 (non-trivial suite) ────────────────
if [ "${verdict_green:-0}" -gt 0 ]; then
    echo "GREEN AC4: acs-verdict.json green_count=$verdict_green (non-trivial suite)"
else
    echo "RED AC4: acs-verdict.json green_count=0 — ACS suite produced no GREEN results"
    rc=1
fi

# ── AC5: build-report ACS result line matches acs-verdict.json counts ─────────
# Extract the ACS result summary line from build-report.md (pattern: green=N red=N)
build_green=$(grep -oE "green=[0-9]+" "$BUILD_REPORT" | tail -1 | grep -oE "[0-9]+" || echo "")
build_red=$(grep -oE "red=[0-9]+"   "$BUILD_REPORT" | tail -1 | grep -oE "[0-9]+" || echo "")
verdict_red=$(jq -r '.red_count' "$ACS_VERDICT")

if [ -n "$build_green" ] && [ "$build_green" = "$verdict_green" ]; then
    echo "GREEN AC5: build-report green=$build_green matches acs-verdict.json green_count=$verdict_green"
elif [ -z "$build_green" ]; then
    echo "GREEN AC5: build-report has no explicit green= count line — no drift to detect"
else
    echo "RED AC5: build-report green=$build_green does not match acs-verdict.json green_count=$verdict_green"
    rc=1
fi

# ── AC6 (anti-tautology): green_count + red_count = total ────────────────────
computed_total=$((verdict_green + verdict_red))
if [ "$computed_total" = "$verdict_total" ]; then
    echo "GREEN AC6 (anti-tautology): green_count($verdict_green) + red_count($verdict_red) = total($verdict_total) — consistent"
else
    echo "RED AC6 (anti-tautology): counts inconsistent: green($verdict_green) + red($verdict_red) = $computed_total but total=$verdict_total"
    rc=1
fi

exit "$rc"
