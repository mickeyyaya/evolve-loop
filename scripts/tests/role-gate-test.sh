#!/usr/bin/env bash
#
# role-gate-test.sh — Unit tests for scripts/guards/role-gate.sh (v8.13.1).
#
# Tests exercise: cycle-state present + path inside/outside allowlist,
# cycle-state absent (transparent passthrough), bypass env, per-phase
# allowlists (calibrate/research/discover/build/audit/ship/learn), and
# always-safe paths (/tmp, /var/folders, $HOME/.claude/).
#
# Usage: bash scripts/role-gate-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

unset EVOLVE_BYPASS_ROLE_GATE

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$REPO_ROOT/scripts/guards/role-gate.sh"
HELPER="$REPO_ROOT/scripts/cycle-state.sh"

# Use an isolated cycle-state file so this test never collides with a real cycle
# in progress. The gate honors EVOLVE_CYCLE_STATE_FILE.
TEST_STATE_DIR=$(mktemp -d -t role-gate-test.XXXXXX)
TEST_STATE="$TEST_STATE_DIR/cycle-state.json"
trap 'rm -rf "$TEST_STATE_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_STATE"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail()   { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

run_gate() {
    local payload="$1"
    local extra_env="${2:-}"
    if [ -n "$extra_env" ]; then
        env $extra_env EVOLVE_CYCLE_STATE_FILE="$TEST_STATE" bash "$GATE" <<< "$payload" >/dev/null 2>&1
    else
        bash "$GATE" <<< "$payload" >/dev/null 2>&1
    fi
}

expect_allow() {
    local label="$1" payload="$2" extra="${3:-}"
    set +e; run_gate "$payload" "$extra"; local rc=$?; set -e
    if [ "$rc" = "0" ]; then pass "$label (rc=0)"
    else fail "$label — expected rc=0, got rc=$rc"; fi
}

expect_deny() {
    local label="$1" payload="$2" extra="${3:-}"
    set +e; run_gate "$payload" "$extra"; local rc=$?; set -e
    if [ "$rc" = "2" ]; then pass "$label (rc=2)"
    else fail "$label — expected rc=2, got rc=$rc"; fi
}

set_state() {
    # set_state <phase> [agent] [worktree]
    local phase="$1" agent="${2:-}" wt="${3:-}"
    rm -f "$TEST_STATE"
    bash "$HELPER" init 99000 "$REPO_ROOT/.evolve/runs/cycle-99000" >/dev/null
    if [ "$phase" != "calibrate" ]; then
        # Advance to target phase via direct write (skip the strict transition
        # logic — we only care about role-gate's view of cycle-state).
        local agent_arg='null'; [ -n "$agent" ] && agent_arg="\"$agent\""
        local wt_arg='null'; [ -n "$wt" ] && wt_arg="\"$wt\""
        jq -c \
            --arg phase "$phase" \
            --argjson agent "$agent_arg" \
            --argjson wt "$wt_arg" \
            '.phase = $phase | .active_agent = $agent | .active_worktree = $wt' \
            "$TEST_STATE" > "$TEST_STATE.tmp" && mv "$TEST_STATE.tmp" "$TEST_STATE"
    fi
}

# === Test 1: no cycle-state → ALLOW (transparent) =============================
header "Test 1: no cycle-state → ALLOW (transparent passthrough)"
rm -f "$TEST_STATE"
expect_allow "any path with no cycle" "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/scripts/evil.sh\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 2: workspace path always allowed ====================================
header "Test 2: workspace path always allowed regardless of phase"
set_state build builder ""
expect_allow "build phase + workspace path" \
    "{\"tool_input\":{\"file_path\":\".evolve/runs/cycle-99000/build-report.md\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 3: build phase + worktree path → ALLOW ==============================
header "Test 3: build phase + active worktree path → ALLOW"
set_state build builder "/tmp/wt-builder-99000"
expect_allow "build phase + worktree" \
    "{\"tool_input\":{\"file_path\":\"/tmp/wt-builder-99000/src/foo.py\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 4: build phase + non-worktree non-workspace → DENY ==================
header "Test 4: build phase + outside scope → DENY"
set_state build builder "/tmp/wt-builder-99000"
expect_deny "build phase + repo source file" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/scripts/evil.sh\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 5: discover phase + outside workspace → DENY ========================
header "Test 5: discover phase + outside workspace → DENY"
set_state discover scout ""
expect_deny "discover phase + skill edit" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/skills/evolve-loop/SKILL.md\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 6: audit phase + audit-report.md → ALLOW ============================
header "Test 6: audit phase + workspace audit-report.md → ALLOW"
set_state audit auditor ""
expect_allow "audit + audit-report.md" \
    "{\"tool_input\":{\"file_path\":\".evolve/runs/cycle-99000/audit-report.md\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 7: audit phase + skill edit → DENY ==================================
header "Test 7: audit phase + skill edit → DENY"
set_state audit auditor ""
expect_deny "audit + SKILL.md write" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/skills/evolve-loop/SKILL.md\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 8: ship phase + plugin.json → ALLOW =================================
header "Test 8: ship phase + .claude-plugin/plugin.json → ALLOW"
set_state ship orchestrator ""
expect_allow "ship + plugin.json" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/.claude-plugin/plugin.json\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 9: ship phase + CHANGELOG.md → ALLOW ================================
header "Test 9: ship phase + CHANGELOG.md → ALLOW"
set_state ship orchestrator ""
expect_allow "ship + CHANGELOG.md" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/CHANGELOG.md\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 10: ship phase + arbitrary source file → DENY =======================
header "Test 10: ship phase + arbitrary source file → DENY"
set_state ship orchestrator ""
expect_deny "ship + scripts/foo.sh" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/scripts/foo.sh\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 11: learn phase + lessons yaml → ALLOW ==============================
header "Test 11: learn phase + .evolve/instincts/lessons/inst-L1.yaml → ALLOW"
set_state learn orchestrator ""
expect_allow "learn + lesson yaml" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/.evolve/instincts/lessons/inst-L1.yaml\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 12: learn phase + state.json → ALLOW ================================
header "Test 12: learn phase + .evolve/state.json → ALLOW"
set_state learn orchestrator ""
expect_allow "learn + state.json" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/.evolve/state.json\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 13: /tmp path always allowed ========================================
header "Test 13: /tmp path always allowed (always-safe dir)"
set_state build builder ""
expect_allow "build phase + /tmp/foo.txt" \
    "{\"tool_input\":{\"file_path\":\"/tmp/foo.txt\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 14: /var/folders path always allowed ================================
header "Test 14: /var/folders path always allowed"
set_state build builder ""
expect_allow "audit phase + /var/folders/abc.json" \
    "{\"tool_input\":{\"file_path\":\"/var/folders/12/xyz/T/abc.json\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 15: $HOME/.claude path always allowed ===============================
header "Test 15: \$HOME/.claude path always allowed"
set_state audit auditor ""
expect_allow "audit + ~/.claude/projects/foo.json" \
    "{\"tool_input\":{\"file_path\":\"$HOME/.claude/projects/foo.json\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 16: bypass env → ALLOW (override) ===================================
header "Test 16: EVOLVE_BYPASS_ROLE_GATE=1 → ALLOW even when would deny"
set_state build builder "/tmp/wt-99000"
expect_allow "bypass + outside scope" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/scripts/evil.sh\"},\"cwd\":\"$REPO_ROOT\"}" \
    "EVOLVE_BYPASS_ROLE_GATE=1"

# === Test 17: missing file_path → ALLOW (passthrough) =========================
header "Test 17: payload missing file_path → ALLOW (passthrough)"
set_state build builder ""
expect_allow "missing file_path" '{"tool_input":{"command":"ls"}}'

# === Test 18: malformed cycle-state → ALLOW (fail-open warned) ================
header "Test 18: malformed cycle-state.json → ALLOW (fail-open with WARN)"
echo '{"phase":"","workspace_path":""}' > "$TEST_STATE"
expect_allow "malformed state" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/scripts/evil.sh\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 19: relative file_path resolved against cwd =========================
header "Test 19: relative file_path resolved against payload cwd"
set_state build builder "/tmp/wt-99000-rel"
mkdir -p /tmp/wt-99000-rel/src
expect_allow "relative path inside worktree" \
    "{\"tool_input\":{\"file_path\":\"src/foo.py\"},\"cwd\":\"/tmp/wt-99000-rel\"}"

# === Test 20: ship phase + skills/SKILL.md → ALLOW ============================
header "Test 20: ship phase + skills/evolve-loop/SKILL.md → ALLOW"
set_state ship orchestrator ""
expect_allow "ship + SKILL.md (version-bump file)" \
    "{\"tool_input\":{\"file_path\":\"$REPO_ROOT/skills/evolve-loop/SKILL.md\"},\"cwd\":\"$REPO_ROOT\"}"

# === Test 21: active_worktree with trailing slash → still matches (LOW-2 fix) =
header "Test 21: active_worktree with trailing slash → match still works"
rm -f "$TEST_STATE"
bash "$HELPER" init 99000 .evolve/runs/cycle-99000 >/dev/null
# Inject a worktree path with a trailing slash to verify the gate's defensive trim.
jq -c '.phase = "build" | .active_agent = "builder" | .active_worktree = "/tmp/wt-trail-99000/"' \
    "$TEST_STATE" > "$TEST_STATE.tmp" && mv "$TEST_STATE.tmp" "$TEST_STATE"
expect_allow "build + worktree path under trailing-slash worktree" \
    "{\"tool_input\":{\"file_path\":\"/tmp/wt-trail-99000/src/foo.py\"},\"cwd\":\"$REPO_ROOT\"}"

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ] && exit 0 || exit 1
