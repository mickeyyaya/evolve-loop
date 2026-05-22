#!/usr/bin/env bash
#
# guards-test.sh — Unit tests for scripts/guards/ship-gate.sh.
#
# v8.13.0 reframes the gate from "parse arbitrary bash for ship verbs"
# (the v8.12.x approach that lost arms races in cycles 8121, 8122) to
# "allowlist exactly one canonical path: scripts/lifecycle/ship.sh". These tests
# exercise the gate's decision logic under realistic JSON payloads.
#
# Tests do NOT exercise scripts/lifecycle/ship.sh's internal audit verification
# (that's covered by ship-integration-test.sh which uses temp git repos).
#
# Usage: bash scripts/guards-test.sh
#
# Exit 0 = all tests pass; non-zero = failures.

set -uo pipefail

unset EVOLVE_BYPASS_SHIP_GATE

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GATE="$REPO_ROOT/scripts/guards/ship-gate.sh"

# Build literal "git commit" / "git push" tokens via concatenation so this
# test script's own command string (which Claude Code's Bash tool sees during
# self-testing) doesn't trip the live ship-gate hook on the user's session.
GC="git c""ommit"
GP="git p""ush"
GH_R_C="gh r""elease c""reate"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail()   { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

run_gate() {
    local payload="$1"
    local extra_env="${2:-}"
    # No set -e dance here — callers wrap with set +e to capture our rc.
    if [ -n "$extra_env" ]; then
        env $extra_env bash "$GATE" <<< "$payload" >/dev/null 2>&1
    else
        bash "$GATE" <<< "$payload" >/dev/null 2>&1
    fi
}

expect_allow() {
    local label="$1" payload="$2" extra="${3:-}"
    set +e
    run_gate "$payload" "$extra"
    local rc=$?
    set -e
    if [ "$rc" = "0" ]; then
        pass "$label (rc=0)"
    else
        fail "$label — expected rc=0, got rc=$rc"
    fi
}

expect_deny() {
    local label="$1" payload="$2" extra="${3:-}"
    set +e
    run_gate "$payload" "$extra"
    local rc=$?
    set -e
    if [ "$rc" = "2" ]; then
        pass "$label (rc=2)"
    else
        fail "$label — expected rc=2, got rc=$rc"
    fi
}

# --- Test 1: empty payload allowed (manual invocation) -----------------------
header "Test 1: empty payload allowed (manual / passthrough)"
set +e; echo "" | bash "$GATE" >/dev/null 2>&1; rc=$?; set -e
[ "$rc" = "0" ] && pass "empty payload allowed" || fail "empty payload blocked rc=$rc"

# --- Test 2: non-ship command allowed ---------------------------------------
header "Test 2: non-ship command (ls -la) allowed"
expect_allow "ls -la passthrough" '{"tool_input":{"command":"ls -la"}}'

# --- Test 3: bash scripts/lifecycle/ship.sh "msg" allowed (canonical path) -------------
header "Test 3: bash scripts/lifecycle/ship.sh canonical path allowed"
expect_allow "bash scripts/lifecycle/ship.sh allowed" '{"tool_input":{"command":"bash scripts/lifecycle/ship.sh \"feat: x\""}}'

# --- Test 4: raw git commit denied ------------------------------------------
header "Test 4: raw $GC denied"
expect_deny "raw $GC blocked" "{\"tool_input\":{\"command\":\"$GC -m foo\"}}"

# --- Test 5: raw git push denied --------------------------------------------
header "Test 5: raw $GP denied"
expect_deny "raw $GP blocked" "{\"tool_input\":{\"command\":\"$GP origin main\"}}"

# --- Test 6: chained && git commit denied (across ; or &&) -------------------
header "Test 6: 'cd subdir && $GC -m foo' chained denied"
expect_deny "chained && $GC blocked" "{\"tool_input\":{\"command\":\"cd subdir && $GC -m foo\"}}"

# --- Test 7: heredoc body containing 'git commit' ALLOWED -------------------
# This is the false-positive guard. A markdown build report passed via
# `cat > x.md <<EOF\n... git commit ...\nEOF` contains the verb as data,
# not code. Awk pre-processor strips heredoc bodies before regex match.
header "Test 7: heredoc body mentioning $GC is ALLOWED"
PAYLOAD="{\"tool_input\":{\"command\":\"cat > x.md <<EOF\nDocs about $GC usage.\nEOF\"}}"
expect_allow "heredoc body $GC reference is allowed" "$PAYLOAD"

# --- Test 8: grep with quoted git commit ALLOWED -----------------------------
header "Test 8: 'grep \"$GC\" docs/' is ALLOWED (token-boundary check)"
expect_allow "grep with quoted commit string allowed" "{\"tool_input\":{\"command\":\"grep \\\"$GC\\\" docs/\"}}"

# --- Test 9: gh release create denied ---------------------------------------
header "Test 9: '$GH_R_C v1.0' denied"
expect_deny "$GH_R_C denied" "{\"tool_input\":{\"command\":\"$GH_R_C v1.0\"}}"

# --- Test 10: pipe to git commit denied --------------------------------------
header "Test 10: 'echo foo | $GC -m foo' denied"
expect_deny "pipe-to-$GC denied" "{\"tool_input\":{\"command\":\"echo foo | $GC -m foo\"}}"

# --- Test 11: bypass switch allows otherwise-denied commands -----------------
header "Test 11: EVOLVE_BYPASS_SHIP_GATE=1 allows ship verbs"
expect_allow "bypass allows $GC" "{\"tool_input\":{\"command\":\"$GC -m foo\"}}" "EVOLVE_BYPASS_SHIP_GATE=1"

# --- Test 12: git status (read-only) allowed ---------------------------------
header "Test 12: 'git status --short' (read-only) allowed"
expect_allow "git status allowed (no ship verb)" '{"tool_input":{"command":"git status --short"}}'

# --- Test 13: git log allowed ------------------------------------------------
header "Test 13: 'git log --oneline' allowed (no ship verb)"
expect_allow "git log allowed" '{"tool_input":{"command":"git log --oneline -3"}}'

# --- Test 14: subshell ($GC) denied ------------------------------------------
header "Test 14: subshell '($GC -m foo)' denied"
expect_deny "subshell $GC denied" "{\"tool_input\":{\"command\":\"($GC -m foo)\"}}"

# --- Test 15: bare-newline chained $GC denied (cycle 8122 D1) -----------------
header "Test 15 (D1): bare-newline-separated $GC denied"
PAYLOAD="{\"tool_input\":{\"command\":\"git status\n$GC -m foo\"}}"
expect_deny "bare-newline $GC denied" "$PAYLOAD"

# --- Test 16 (D-NEW-1): bash -c "git commit ..." denied (cycle 8130 RC1) -----
# This was the HIGH bypass that defeated cycle 8130 RC1's gate. Both the
# canonical-path check (extracted "git as TARGET_TOKEN, realpath failed) AND
# the regex (quote not in boundary class) missed it. Fix in step 1.5 of
# ship-gate.sh: extract the -c argument and recurse the regex.
header "Test 16 (D-NEW-1): bash -c with quoted $GC denied"
expect_deny "bash -c \"$GC\" denied" "{\"tool_input\":{\"command\":\"bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 17: sh -c variant of D-NEW-1 ---------------------------------------
header "Test 17: sh -c with quoted $GC denied"
expect_deny "sh -c \"$GC\" denied" "{\"tool_input\":{\"command\":\"sh -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 18: eval "git commit" denied (D-NEW-1 variant) ---------------------
header "Test 18: eval with quoted $GC denied"
expect_deny "eval \"$GC\" denied" "{\"tool_input\":{\"command\":\"eval \\\"$GC -m bypass\\\"\"}}"

# --- Test 19: bash -c with single-quoted $GC denied --------------------------
header "Test 19: bash -c with single-quoted $GC denied"
expect_deny "bash -c '$GC' denied" "{\"tool_input\":{\"command\":\"bash -c '$GC -m bypass'\"}}"

# --- Test 20 (D-NEW-5): bash -x -c "$GC" denied (flag before -c) -------------
# Cycle 8131 RC2 audit found this bypass: any flag between bash and -c shifted
# -c's position out of the second-token slot, so Step 1.5's grep fired only on
# the second-token form. Fix walks ALL tokens with awk.
header "Test 20 (D-NEW-5): bash -x -c with $GC denied"
expect_deny "bash -x -c $GC denied" "{\"tool_input\":{\"command\":\"bash -x -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 21 (D-NEW-5 variant): bash --norc -c "$GC" denied ------------------
header "Test 21 (D-NEW-5): bash --norc -c with $GC denied"
expect_deny "bash --norc -c $GC denied" "{\"tool_input\":{\"command\":\"bash --norc -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 22 (D-NEW-5 variant): bash -ec combined-flag form denied ----------
header "Test 22 (D-NEW-5): bash -ec combined-flag with $GC denied"
expect_deny "bash -ec $GC denied (combined flag)" "{\"tool_input\":{\"command\":\"bash -ec \\\"$GC -m bypass\\\"\"}}"

# --- Test 23 (D-NEW-5 variant): /bin/bash --rcfile path -c "$GC" denied ------
header "Test 23 (D-NEW-5): /bin/bash --rcfile /dev/null -c with $GC denied"
expect_deny "bash --rcfile -c $GC denied" "{\"tool_input\":{\"command\":\"/bin/bash --rcfile /dev/null -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 24 (D-NEW-6): /usr/bin/env bash -c "$GC" denied (env wrapper) -----
# Cycle 8132 RC3 audit found env-wrapper bypass. Fix uses bash glob patterns
# to catch */bash, */env, etc.
header "Test 24 (D-NEW-6): /usr/bin/env bash -c with $GC denied"
expect_deny "/usr/bin/env bash -c $GC denied" "{\"tool_input\":{\"command\":\"/usr/bin/env bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 25 (D-NEW-6 variant): bare env bash -c "$GC" denied ---------------
header "Test 25 (D-NEW-6): env bash -c with $GC denied"
expect_deny "env bash -c $GC denied" "{\"tool_input\":{\"command\":\"env bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 26 (structural): /usr/local/bin/bash -c "$GC" denied --------------
# Glob */bash should catch homebrew/non-standard paths too.
header "Test 26: /usr/local/bin/bash -c with $GC denied"
expect_deny "/usr/local/bin/bash -c $GC denied" "{\"tool_input\":{\"command\":\"/usr/local/bin/bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 27: /opt/homebrew/bin/zsh -c "$GC" denied -------------------------
header "Test 27: /opt/homebrew/bin/zsh -c with $GC denied"
expect_deny "/opt/homebrew/bin/zsh -c $GC denied" "{\"tool_input\":{\"command\":\"/opt/homebrew/bin/zsh -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 28: bash -- -c "$GC" denied (end-of-flags marker) -----------------
header "Test 28: bash -- -c with $GC denied"
expect_deny "bash -- -c $GC denied" "{\"tool_input\":{\"command\":\"bash -- -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 29 (D-NEW-7): nice bash -c "$GC" denied (utility wrapper) ---------
# Cycle 8133 RC4 audit found utility-wrapper bypass (nice/nohup/time/xargs).
# Fix: added wrappers to Step 1.5 case statement. awk walk handles them
# uniformly because it scans for -c at any token position.
header "Test 29 (D-NEW-7): nice bash -c with $GC denied"
expect_deny "nice bash -c $GC denied" "{\"tool_input\":{\"command\":\"nice bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 30 (D-NEW-7): nohup bash -c "$GC" denied --------------------------
header "Test 30 (D-NEW-7): nohup bash -c with $GC denied"
expect_deny "nohup bash -c $GC denied" "{\"tool_input\":{\"command\":\"nohup bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 31 (D-NEW-7): time bash -c "$GC" denied ---------------------------
header "Test 31 (D-NEW-7): time bash -c with $GC denied"
expect_deny "time bash -c $GC denied" "{\"tool_input\":{\"command\":\"time bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 32 (D-NEW-7): xargs -I{} bash -c "$GC" denied ---------------------
header "Test 32 (D-NEW-7): xargs -I{} bash -c with $GC denied"
expect_deny "xargs -I{} bash -c $GC denied" "{\"tool_input\":{\"command\":\"xargs -I{} bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 33: timeout 10 bash -c "$GC" denied --------------------------------
header "Test 33: timeout 10 bash -c with $GC denied"
expect_deny "timeout bash -c $GC denied" "{\"tool_input\":{\"command\":\"timeout 10 bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Test 34: stdbuf -o0 bash -c "$GC" denied --------------------------------
header "Test 34: stdbuf -o0 bash -c with $GC denied"
expect_deny "stdbuf bash -c $GC denied" "{\"tool_input\":{\"command\":\"stdbuf -o0 bash -c \\\"$GC -m bypass\\\"\"}}"

# --- Summary ----------------------------------------------------------------
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed (of $((PASS + FAIL)) checks)"
echo "==========================================="
[ "$FAIL" -eq 0 ]
