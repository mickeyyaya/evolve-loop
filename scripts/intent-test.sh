#!/usr/bin/env bash
#
# intent-test.sh — Unit tests for the evolve-intent skill v0.1 (v8.19.0).
#
# Covers:
#   - cycle-state.sh: 'intent' is a valid phase; intent_required field at init
#   - cycle-state.sh accept-intent operator command
#   - phase-gate.sh: gate_calibrate_to_intent + gate_intent_to_research
#   - phase-gate-precondition.sh: scout denied when intent.md missing under
#     EVOLVE_REQUIRE_INTENT=1; allowed when intent ledger entry present
#
# Tests use temp state files (EVOLVE_CYCLE_STATE_FILE) and temp ledgers so
# the real .evolve/* tree is never touched.
#
# Usage: bash scripts/intent-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

unset EVOLVE_BYPASS_PHASE_GATE
unset EVOLVE_REQUIRE_INTENT

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CYCLE_STATE="$REPO_ROOT/scripts/cycle-state.sh"
PHASE_GATE="$REPO_ROOT/scripts/phase-gate.sh"
PRECONDITION="$REPO_ROOT/scripts/guards/phase-gate-precondition.sh"

TEST_DIR=$(mktemp -d -t intent-test.XXXXXX)
TEST_STATE="$TEST_DIR/cycle-state.json"
TEST_LEDGER="$TEST_DIR/ledger.jsonl"
TEST_WORKSPACE="$TEST_DIR/workspace/cycle-99"
mkdir -p "$TEST_WORKSPACE"
trap 'rm -rf "$TEST_DIR"' EXIT
export EVOLVE_CYCLE_STATE_FILE="$TEST_STATE"
export EVOLVE_LEDGER_OVERRIDE="$TEST_LEDGER"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Reset state between tests
reset_state() {
    rm -f "$TEST_STATE"
    : > "$TEST_LEDGER"
    rm -rf "$TEST_WORKSPACE"
    mkdir -p "$TEST_WORKSPACE"
}

# Compute SHA256 for a file (BSD-shasum compatible)
sha256_file() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}'
    else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# Write a sample valid intent.md
write_valid_intent() {
    local path="$1"
    local awn="${2:-IMKI}"
    cat > "$path" <<EOF
---
awn_class: $awn
goal: Add a force-directed graph to visualize node clusters.
non_goals:
  - "do not add zoom/pan"
constraints:
  - "must render under 3 seconds for 100 nodes"
interfaces:
  - "src/components/graph.tsx"
acceptance_checks:
  - check: "user can identify clusters within 3s"
    how_verified: "manual"
assumptions:
  - "data is already structured as nodes/edges"
challenged_premises:
  - premise: "user wants force-directed layout"
    challenge: "tree layout may be clearer for hierarchical data"
    proposed_alternative: "ask user about data shape before deciding"
risk_level: medium
---

# Restated Intent
The user wants to visualize relationships between nodes. They said force-directed,
but tree layout may serve them better depending on data shape.
EOF
}

# Write a ledger entry binding intent.md
write_intent_ledger() {
    local cycle="$1"
    local artifact="$2"
    local sha
    sha=$(sha256_file "$artifact")
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    cat >> "$TEST_LEDGER" <<EOF
{"ts":"${ts}","cycle":${cycle},"role":"intent","kind":"agent_subprocess","model":"opus","exit_code":0,"duration_s":"60","artifact_path":"${artifact}","artifact_sha256":"${sha}","challenge_token":"test-token","git_head":"none","tree_state_sha":"none"}
EOF
}

[ -f "$CYCLE_STATE" ] || { echo "FATAL: $CYCLE_STATE missing"; exit 1; }
[ -f "$PHASE_GATE" ] || { echo "FATAL: $PHASE_GATE missing"; exit 1; }
[ -f "$PRECONDITION" ] || { echo "FATAL: $PRECONDITION missing"; exit 1; }

# === Test 1: 'intent' is a valid phase value ==================================
header "Test 1: cycle_state_advance accepts 'intent' phase"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
set +e
bash "$CYCLE_STATE" advance intent intent >/dev/null 2>&1
rc=$?
set -e
phase=$(jq -r '.phase' "$TEST_STATE" 2>/dev/null)
[ "$rc" = "0" ] && [ "$phase" = "intent" ] && pass "advance to phase=intent worked (rc=0, phase=$phase)" || fail_ "rc=$rc phase=$phase"

# === Test 2: 'intent' completes between calibrate and research ================
header "Test 2: completed_phases tracks intent → research transition"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
bash "$CYCLE_STATE" advance intent intent >/dev/null 2>&1
bash "$CYCLE_STATE" advance research scout >/dev/null 2>&1
completed=$(jq -r '.completed_phases | join(",")' "$TEST_STATE" 2>/dev/null)
echo "$completed" | grep -q "intent" && pass "intent recorded in completed_phases ($completed)" || fail_ "intent missing from completed_phases ($completed)"

# === Test 3: cycle_state_init records intent_required from env ================
header "Test 3: cycle_state_init records intent_required:true when env=1"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
ir=$(jq -r '.intent_required' "$TEST_STATE" 2>/dev/null)
[ "$ir" = "true" ] && pass "intent_required=true recorded" || fail_ "intent_required=$ir (expected true)"

# === Test 4: cycle_state_init defaults intent_required to false ==============
header "Test 4: cycle_state_init defaults intent_required:false when env unset"
reset_state
unset EVOLVE_REQUIRE_INTENT
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
ir=$(jq -r '.intent_required' "$TEST_STATE" 2>/dev/null)
[ "$ir" = "false" ] && pass "intent_required=false default" || fail_ "intent_required=$ir (expected false)"

# === Test 5: re-running /intent replaces intent.md (autonomy: no checkpoint) ==
header "Test 5: re-running intent persona replaces prior intent.md (no human approval)"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
# First intent.md (CLEAR class)
write_valid_intent "$TEST_WORKSPACE/intent.md" CLEAR
sha1=$(sha256_file "$TEST_WORKSPACE/intent.md")
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
# Re-run: replace with a different intent.md (IMKI class)
write_valid_intent "$TEST_WORKSPACE/intent.md" IMKI
sha2=$(sha256_file "$TEST_WORKSPACE/intent.md")
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
# Gate should accept the latest ledger entry (sha2), not the first
set +e
bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && [ "$sha1" != "$sha2" ] && pass "latest intent.md accepted (sha changed: $sha1 → $sha2)" || fail_ "rc=$rc sha1=$sha1 sha2=$sha2"

# === Test 6: gate_intent_to_research denies when intent.md missing ===========
header "Test 6: gate_intent_to_research denies missing intent.md"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
set +e
bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" != "0" ] && pass "missing intent.md denied (rc=$rc)" || fail_ "rc=$rc (expected non-zero)"

# === Test 7: gate_intent_to_research denies when challenged_premises < 1 =====
header "Test 7: gate_intent_to_research denies zero-premise intent.md"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
cat > "$TEST_WORKSPACE/intent.md" <<'EOF'
---
awn_class: CLEAR
goal: Build the thing.
challenged_premises: []
---
EOF
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
set +e
out=$(bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" 2>&1)
rc=$?
set -e
echo "$out" | grep -q "challenged_premises" && matched=0 || matched=1
[ "$rc" != "0" ] && [ "$matched" = "0" ] && pass "zero-premise denied with clear message (rc=$rc)" || fail_ "rc=$rc matched=$matched out=[$out]"

# === Test 8: gate_intent_to_research denies awn_class=IBTC ===================
header "Test 8: gate_intent_to_research denies awn_class=IBTC"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
write_valid_intent "$TEST_WORKSPACE/intent.md" IBTC
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
set +e
out=$(bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" 2>&1)
rc=$?
set -e
echo "$out" | grep -qi "ibtc\|out of scope\|scope rejection" && matched=0 || matched=1
[ "$rc" != "0" ] && [ "$matched" = "0" ] && pass "IBTC short-circuit denied (rc=$rc)" || fail_ "rc=$rc matched=$matched out=[$out]"

# === Test 9: gate_intent_to_research passes valid intent.md + ledger =========
header "Test 9: gate_intent_to_research allows valid intent.md"
reset_state
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
write_valid_intent "$TEST_WORKSPACE/intent.md"
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
set +e
bash "$PHASE_GATE" intent-to-research 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "valid intent.md allowed (rc=0)" || fail_ "rc=$rc"

# === Test 10: precondition denies scout when intent_required=true + no intent ledger
header "Test 10: precondition denies scout when intent_required=true and no intent ledger entry"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
# Initial phase=calibrate, intent_required=true, no intent ledger entry yet
payload='{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'
set +e
echo "$payload" | bash "$PRECONDITION" >/dev/null 2>&1
rc=$?
set -e
# rc=2 = explicit deny; we want the precondition to deny scout under these conditions
[ "$rc" = "2" ] && pass "scout denied when intent.md missing (rc=2)" || fail_ "rc=$rc (expected 2 for deny)"

# === Test 11: precondition allows scout when intent ledger entry present =====
header "Test 11: precondition allows scout when intent.md ledger entry present"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
bash "$CYCLE_STATE" advance intent intent >/dev/null 2>&1
bash "$CYCLE_STATE" advance research scout >/dev/null 2>&1
write_valid_intent "$TEST_WORKSPACE/intent.md"
write_intent_ledger 99 "$TEST_WORKSPACE/intent.md"
payload='{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'
set +e
echo "$payload" | bash "$PRECONDITION" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "scout allowed when intent.md ledger present (rc=0)" || fail_ "rc=$rc (expected 0)"

# === Test 12: default behavior unchanged (intent_required=false) =============
header "Test 12: default flow (intent_required=false) — scout allowed at calibrate"
reset_state
unset EVOLVE_REQUIRE_INTENT
bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
payload='{"tool_input":{"command":"bash scripts/subagent-run.sh scout 99 .evolve/runs/cycle-99"}}'
set +e
echo "$payload" | bash "$PRECONDITION" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "default flow unchanged: scout allowed (rc=0)" || fail_ "rc=$rc (expected 0)"

# === Test 13: gate_calibrate_to_intent only fires when intent_required=true ==
header "Test 13: gate_calibrate_to_intent passes when intent_required=true"
reset_state
EVOLVE_REQUIRE_INTENT=1 bash "$CYCLE_STATE" init 99 ".evolve/runs/cycle-99" >/dev/null 2>&1
set +e
bash "$PHASE_GATE" calibrate-to-intent 99 "$TEST_WORKSPACE" >/dev/null 2>&1
rc=$?
set -e
[ "$rc" = "0" ] && pass "gate passes when intent_required=true (rc=0)" || fail_ "rc=$rc (expected 0)"

# === Test 14: /evolve-loop SKILL.md strict-mode invocation enables intent (v8.19.1) =
header "Test 14: /evolve-loop strict-mode bash command includes EVOLVE_REQUIRE_INTENT=1"
SKILL_FILE="$REPO_ROOT/skills/evolve-loop/SKILL.md"
[ -f "$SKILL_FILE" ] || fail_ "SKILL.md not at $SKILL_FILE"
# Look for the canonical invocation line in the strict-mode block. The line
# must set EVOLVE_REQUIRE_INTENT=1 before calling evolve-loop-dispatch.sh.
if grep -E '^EVOLVE_REQUIRE_INTENT=1[[:space:]]' "$SKILL_FILE" | grep -E 'evolve-loop-dispatch\.sh' >/dev/null 2>&1; then
    pass "SKILL.md sets EVOLVE_REQUIRE_INTENT=1 for /evolve-loop default path"
else
    fail_ "SKILL.md strict-mode invocation does not set EVOLVE_REQUIRE_INTENT=1 — /evolve-loop would skip intent capture"
fi

# === Test 15: commands/loop.md phase diagram lists /intent as first phase ===
header "Test 15: commands/loop.md execution diagram includes /intent"
LOOP_CMD="$REPO_ROOT/.claude-plugin/commands/loop.md"
[ -f "$LOOP_CMD" ] || fail_ "loop.md not at $LOOP_CMD"
if grep -qE '^/intent\b' "$LOOP_CMD"; then
    pass "loop.md execution diagram includes /intent"
else
    fail_ "loop.md execution diagram missing /intent — phase ordering doc out of sync"
fi

# === Test 17: SKILL.md auto-sets EVOLVE_SANDBOX_FALLBACK_ON_EPERM (v8.20.1+) ==
# macOS Darwin 25.4+ kernel-level restriction: sandbox-exec cannot be nested.
# When orchestrator (sandboxed) spawns builder (also sandboxed), the inner
# sandbox_apply() returns EPERM. EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 tells the
# adapter to retry without sandbox on EPERM. Must be auto-set by SKILL.md so
# `/evolve-loop` users on macOS don't hit this trap. No-op on Linux.
header "Test 17: /evolve-loop auto-sets EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"
SKILL_FILE="$REPO_ROOT/skills/evolve-loop/SKILL.md"
[ -f "$SKILL_FILE" ] || fail_ "SKILL.md not at $SKILL_FILE"
# The strict-mode bash invocation must include both env vars before the dispatcher.
if grep -E '^EVOLVE_REQUIRE_INTENT=1 EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1' "$SKILL_FILE" | grep -E 'evolve-loop-dispatch\.sh' >/dev/null 2>&1; then
    pass "SKILL.md auto-sets both EVOLVE_REQUIRE_INTENT=1 and EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1"
else
    fail_ "SKILL.md strict-mode invocation does not auto-set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 — macOS users will hit nested-sandbox EPERM"
fi

# === Test 18: SKILL.md dispatcher path is cwd-independent (v8.20.2+) ==========
# The slash-command agent's cwd is the user's project, NOT the plugin install.
# A relative `bash scripts/evolve-loop-dispatch.sh` will fail with rc=127. The
# SKILL.md must use an absolute or find-based path that resolves regardless of
# cwd. Without this, the v8.18.0 boundary class returns from the dead.
header "Test 18: SKILL.md dispatcher invocation is cwd-independent (v8.20.2+)"
# Negative: fail if SKILL.md uses bare relative `bash scripts/evolve-loop-dispatch.sh`.
# Allow it inside `\$EVOLVE_PLUGIN_ROOT/scripts/...` form (different concept).
# Search for the bad pattern: 'bash scripts/' with no $-prefix or $HOME or
# $(...) command substitution before it.
bad_relative=$(grep -E '^[A-Z_=0-9 ]*bash[[:space:]]+scripts/evolve-loop-dispatch\.sh' "$SKILL_FILE" | grep -vE '^[[:space:]]*#|^[[:space:]]*>' || true)
if [ -z "$bad_relative" ]; then
    pass "no bare-relative `bash scripts/evolve-loop-dispatch.sh` in SKILL.md (cwd-independent)"
else
    fail_ "SKILL.md uses bare-relative dispatcher path — fails when slash-command cwd is user project: $bad_relative"
fi
# Positive: dispatcher path must be either find-based or $HOME-prefixed
if grep -qE 'bash "\$\(find.*evolve-loop-dispatch' "$SKILL_FILE" || \
   grep -qE '\$HOME/\.claude/plugins.*evolve-loop-dispatch' "$SKILL_FILE"; then
    pass "SKILL.md uses cwd-independent dispatcher resolution (find or \$HOME)"
else
    fail_ "SKILL.md missing cwd-independent dispatcher resolution"
fi

# === Test 16: orchestrator profile uses bare-name patterns (v8.20.0+) =========
# The v8.20.0 PATH-based architecture requires kernel-script invocations to be
# bare names (cycle-state.sh advance) — NOT path-prefixed (bash scripts/...,
# bash **/scripts/..., bash /*/.claude/plugins/...). This test fails loudly
# if any of the 4 path-prefixed pattern forms re-appear in orchestrator.json.
#
# Why this matters: v8.18.x → v8.19.5 shipped 5 patches in 48 hours each adding
# more path-variant patterns to handle install-layout differences (140 patterns
# total). v8.20.0 collapsed to ~16 bare-name patterns by prepending PATH. If
# someone reverts to path-prefixed patterns, install-layout fragility returns.
header "Test 16: orchestrator.json uses bare-name patterns (no path-prefixed Bash kernel patterns)"
ORCH_PROFILE="$REPO_ROOT/.evolve/profiles/orchestrator.json"
[ -f "$ORCH_PROFILE" ] || fail_ "orchestrator profile not at $ORCH_PROFILE"
# Look for any of the 4 known fragile pattern forms applied to kernel-script
# invocations. Match patterns like:
#   Bash(bash scripts/...
#   Bash(bash **/scripts/...
#   Bash(bash /*/.claude/plugins/marketplaces/*/scripts/...
#   Bash(bash /*/.claude/plugins/cache/*/*/*/scripts/...
# Use jq to extract ONLY the actual list entries (skip _design_notes which
# may legitimately reference old patterns as historical documentation).
fragile_count=0
if command -v jq >/dev/null 2>&1; then
    fragile_count=$(jq -r '(.allowed_tools // []) + (.disallowed_tools // []) | .[]' "$ORCH_PROFILE" \
        | grep -cE 'Bash\(bash (scripts/|\*\*/scripts/|/\*/\.claude/plugins/(marketplaces|cache))' || true)
fi
if [ "$fragile_count" = "0" ]; then
    pass "no path-prefixed kernel-script patterns in allowlists (architecture preserved)"
else
    fail_ "$fragile_count path-prefixed kernel-script patterns found in allowlists — v8.20.0 collapse reverted"
fi
# Positive-evidence check: bare-name kernel-script patterns must be present.
bare_name_count=0
if command -v jq >/dev/null 2>&1; then
    bare_name_count=$(jq -r '.allowed_tools[]?' "$ORCH_PROFILE" \
        | grep -cE '^Bash\((cycle-state\.sh|ship\.sh|subagent-run\.sh)' || true)
fi
if [ "$bare_name_count" -ge 3 ]; then
    pass "bare-name kernel-script patterns present (count=$bare_name_count, positive architecture evidence)"
else
    fail_ "only $bare_name_count bare-name kernel-script patterns in orchestrator allowlist (expected >=3)"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
