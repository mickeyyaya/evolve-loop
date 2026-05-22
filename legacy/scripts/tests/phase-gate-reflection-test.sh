#!/usr/bin/env bash
#
# phase-gate-reflection-test.sh — Validate phase-gate.sh check_reflection_artifact
# in both advisory and enforce stages.
#
# Schema reference: agents/agent-templates.md → Reflection Journal Schema
# Phase-gate function: scripts/lifecycle/phase-gate.sh → check_reflection_artifact
#
# Tests:
#   T1. Advisory + missing YAML → returns 0 (WARN only, no block)
#   T2. Advisory + present valid YAML → returns 0 (OK)
#   T3. Advisory + YAML missing required key → returns 0 (WARN only)
#   T4. Enforce + missing YAML → exits 1 (BLOCK)
#   T5. Enforce + valid YAML → returns 0 (OK)
#   T6. EVOLVE_REFLECTION_JOURNAL=0 + missing YAML → returns 0 (feature off, no check)
#
# Usage: bash scripts/tests/phase-gate-reflection-test.sh
# Exit:  0 if all PASS; non-zero otherwise.
#
# bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PHASE_GATE="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"

if [ ! -f "$PHASE_GATE" ]; then
    echo "FAIL: phase-gate.sh not found at $PHASE_GATE"
    exit 1
fi

if ! grep -q "^check_reflection_artifact()" "$PHASE_GATE"; then
    echo "FAIL: check_reflection_artifact function not present in phase-gate.sh"
    exit 1
fi

FIXTURE_DIR="${TMPDIR:-/tmp}/phase-gate-reflection-test.$$"
mkdir -p "$FIXTURE_DIR"
trap 'rm -rf "$FIXTURE_DIR"' EXIT

pass=0
fail=0

# Helper: source the check function in a sub-shell with mocked vars and capture
# exit code. Returns the function's exit code via the sub-shell's exit code.
run_check() {
    local phase="$1"
    local workspace="$2"
    local journal="${3:-1}"
    local enforce="${4:-0}"

    bash -c '
        set -uo pipefail
        WORKSPACE="'"$workspace"'"
        EVOLVE_REFLECTION_JOURNAL="'"$journal"'"
        EVOLVE_REFLECTION_JOURNAL_ENFORCE="'"$enforce"'"
        log() { echo "[log] $*" >&2; }
        fail() { echo "[FAIL] $*" >&2; exit 1; }
        # Source just the check_reflection_artifact function definition
        eval "$(awk "/^check_reflection_artifact\\(\\)/,/^}/" "'"$PHASE_GATE"'")"
        check_reflection_artifact "'"$phase"'"
    ' 2>/dev/null
}

assert_rc() {
    local label="$1"
    local expected="$2"
    local actual="$3"
    if [ "$expected" = "$actual" ]; then
        echo "PASS: $label (rc=$actual)"
        pass=$((pass+1))
    else
        echo "FAIL: $label — expected rc=$expected, got rc=$actual"
        fail=$((fail+1))
    fi
}

write_valid_yaml() {
    local path="$1"
    cat > "$path" <<EOF
schema_version: 1
cycle: 9000
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=10"
    severity: medium
suggested_improvements:
  - action: "Bump kb-search quota to 30"
    priority: medium
reflection_confidence: 0.8
phase_tracker_refs:
  latency_ms: 1000
  cost_usd: 0.5
  turns: 10
EOF
}

# ---- T1. Advisory + missing YAML → rc=0 (WARN only) ----
ws1="$FIXTURE_DIR/ws1"
mkdir -p "$ws1"
run_check scout "$ws1" 1 0
assert_rc "T1: advisory + missing → WARN-only rc=0" "0" "$?"

# ---- T2. Advisory + present valid YAML → rc=0 (OK) ----
ws2="$FIXTURE_DIR/ws2"
mkdir -p "$ws2"
write_valid_yaml "$ws2/scout-reflection.yaml"
run_check scout "$ws2" 1 0
assert_rc "T2: advisory + valid YAML → OK rc=0" "0" "$?"

# ---- T3. Advisory + YAML missing required key → rc=0 (WARN only) ----
ws3="$FIXTURE_DIR/ws3"
mkdir -p "$ws3"
cat > "$ws3/scout-reflection.yaml" <<EOF
schema_version: 1
cycle: 9000
phase: scout
agent: evolve-scout
# missing phase_smooth, suggested_improvements, reflection_confidence, phase_tracker_refs
EOF
run_check scout "$ws3" 1 0
assert_rc "T3: advisory + incomplete schema → WARN rc=0" "0" "$?"

# ---- T4. Enforce + missing YAML → rc=1 (BLOCK) ----
ws4="$FIXTURE_DIR/ws4"
mkdir -p "$ws4"
run_check scout "$ws4" 1 1
assert_rc "T4: enforce + missing → BLOCK rc=1" "1" "$?"

# ---- T5. Enforce + valid YAML → rc=0 (OK) ----
ws5="$FIXTURE_DIR/ws5"
mkdir -p "$ws5"
write_valid_yaml "$ws5/scout-reflection.yaml"
run_check scout "$ws5" 1 1
assert_rc "T5: enforce + valid YAML → OK rc=0" "0" "$?"

# ---- T6. Feature off (EVOLVE_REFLECTION_JOURNAL=0) + missing → rc=0 (no check) ----
ws6="$FIXTURE_DIR/ws6"
mkdir -p "$ws6"
run_check scout "$ws6" 0 0
assert_rc "T6: feature-off + missing → no-op rc=0" "0" "$?"

echo ""
echo "phase-gate-reflection-test: $pass PASS, $fail FAIL"
if [ "$fail" -gt 0 ]; then
    exit 1
fi
exit 0
