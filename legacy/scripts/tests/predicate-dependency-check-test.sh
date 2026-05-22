#!/usr/bin/env bash
#
# predicate-dependency-check-test.sh — Unit tests for the Opt A (v10.19.0)
# legacy/scripts/utility/predicate-dependency-check.sh helper.
#
# Tests cover:
#   1. Missing scout-report.md → exit 2 (infra error)
#   2. Empty/whitespace-only scout-report → exit 0 (no paths, safe to skip)
#   3. scout-report mentions only non-regression files → exit 0 (safe to skip)
#   4. scout-report mentions a regression-suite-grepped basename → exit 1
#   5. scout-report mentions a path-only token (no backticks) → exit 0
#      (parser is intentionally backtick-scoped to reduce false positives)
#   6. scout-report mentions a fully-qualified path that resolves to a
#      regression-grepped basename → exit 1
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
SCRIPT="$REPO_ROOT/legacy/scripts/utility/predicate-dependency-check.sh"
SCRATCH=$(mktemp -d -t "predicate-dep-XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Helper: build a synthetic workspace + scout-report with the given body.
build_ws() {
    local ws="$1"; shift
    local body="$1"; shift
    mkdir -p "$ws"
    printf '%s\n' "$body" > "$ws/scout-report.md"
    echo "$ws"
}

# All tests run against the REAL acs/regression-suite (we're checking the
# REACHABILITY heuristic, which depends on real predicate scripts). To
# guarantee a known reachable basename, we pick one we expect to exist in
# multiple regression-suite scripts: `state.json` is referenced everywhere.
KNOWN_REACHABLE_BASENAME="state.json"
KNOWN_NONREACHABLE_BASENAME="zzz-this-file-does-not-exist-anywhere-xyz123.md"

# === Test 1: missing scout-report → exit 2 =====================================
header "Test 1: missing scout-report.md → exit 2 (infra error)"
ws=$(mktemp -d "$SCRATCH/ws-1-XXXX")
rc=0
bash "$SCRIPT" 99000 "$ws" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "2" ]; then pass "missing scout-report → exit 2"; else fail_ "expected 2 got $rc"; fi

# === Test 2: empty scout-report → exit 0 (no paths to check) ===================
header "Test 2: empty scout-report → exit 0 (safe to skip)"
ws=$(build_ws "$(mktemp -d "$SCRATCH/ws-2-XXXX")" "# Scout Report — Cycle 99000

## Discovery Summary

No file changes proposed.
")
rc=0
bash "$SCRIPT" 99000 "$ws" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then pass "empty scout-report → exit 0"; else fail_ "expected 0 got $rc"; fi

# === Test 3: scout-report mentions only non-reachable basenames → exit 0 =======
header "Test 3: non-reachable basenames → exit 0 (safe to skip)"
ws=$(build_ws "$(mktemp -d "$SCRATCH/ws-3-XXXX")" "# Scout — Cycle 99000

## Proposed Tasks

Edit \`$KNOWN_NONREACHABLE_BASENAME\` to add a feature flag.
")
rc=0
bash "$SCRIPT" 99000 "$ws" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then pass "non-reachable basenames → exit 0"; else fail_ "expected 0 got $rc"; fi

# === Test 4: scout-report mentions a reachable basename → exit 1 ===============
header "Test 4: reachable basename ($KNOWN_REACHABLE_BASENAME) → exit 1 (Triage MUST run)"
ws=$(build_ws "$(mktemp -d "$SCRATCH/ws-4-XXXX")" "# Scout — Cycle 99000

## Proposed Tasks

Modify \`$KNOWN_REACHABLE_BASENAME\` to add a new field.
")
rc=0
bash "$SCRIPT" 99000 "$ws" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "1" ]; then pass "reachable basename → exit 1"; else fail_ "expected 1 got $rc"; fi

# === Test 5: path-only mention without backticks → exit 0 (parser ignores) =====
header "Test 5: bare path (no backticks) → exit 0 (parser scope = backticks only)"
ws=$(build_ws "$(mktemp -d "$SCRATCH/ws-5-XXXX")" "# Scout — Cycle 99000

## Proposed Tasks

Modify $KNOWN_REACHABLE_BASENAME to add a new field. (Note: no backticks here.)
")
rc=0
bash "$SCRIPT" 99000 "$ws" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "0" ]; then pass "bare path → exit 0 (scope is intentional)"; else fail_ "expected 0 got $rc"; fi

# === Test 6: fully-qualified path → exit 1 (basename resolves correctly) =======
header "Test 6: full path \`.evolve/state.json\` → exit 1 (basename grep matches)"
ws=$(build_ws "$(mktemp -d "$SCRATCH/ws-6-XXXX")" "# Scout — Cycle 99000

## Proposed Tasks

Update \`.evolve/state.json\` to add the new schema field.
")
rc=0
bash "$SCRIPT" 99000 "$ws" >/dev/null 2>&1 || rc=$?
if [ "$rc" = "1" ]; then pass "full path → exit 1"; else fail_ "expected 1 got $rc"; fi

# === Summary ===================================================================
echo
echo "=========================================="
echo "  Passed: $PASS"
echo "  Failed: $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ] && exit 0 || exit 1
