#!/usr/bin/env bash
# cross-cli-consensus-test.sh — Tests for v8.53.0 cross-cli-vote merge mode.
#
# Verifies the consensus protocol: MAJORITY-PASS with FAIL-VETO.
#   - Any FAIL → consensus FAIL (rc=1)
#   - >= ceil(N/2) PASS, no FAIL → consensus PASS (rc=0)
#   - Otherwise → consensus WARN (rc=0, fluent default)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AGG="$REPO_ROOT/scripts/dispatch/aggregator.sh"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Helper: write a worker audit artifact with given verdict
write_audit() {
    local path="$1" verdict="$2" body="${3:-Worker audit body.}"
    cat > "$path" <<EOF
Verdict: $verdict

$body
EOF
}

# Helper: run cross-cli-vote, return verdict line + rc
# Args: <expected-rc> <expected-verdict> <verdict1> <verdict2> ...
run_consensus() {
    local expected_rc="$1" expected_verdict="$2"; shift 2
    local d
    d=$(mktemp -d -t "ccc.XXXXXX")
    local i=0
    local args=()
    for v in "$@"; do
        local f="$d/worker-$i.md"
        write_audit "$f" "$v" "verdict=$v worker $i"
        args+=("$f")
        i=$((i + 1))
    done
    set +e
    bash "$AGG" cross-cli-vote "$d/result.md" "${args[@]}" >/dev/null 2>&1
    local rc=$?
    : # was: set -e (no-op; original script uses set +o errexit)
    local actual_verdict=""
    if [ -f "$d/result.md" ]; then
        actual_verdict=$(awk '/^Verdict:/ {print $2; exit}' "$d/result.md")
    fi
    rm -rf "$d"
    if [ "$rc" = "$expected_rc" ] && [ "$actual_verdict" = "$expected_verdict" ]; then
        pass "expected $expected_verdict/rc=$expected_rc, got $actual_verdict/rc=$rc"
    else
        fail_ "expected $expected_verdict/rc=$expected_rc, got $actual_verdict/rc=$rc (verdicts: $*)"
    fi
}

# === Test 1: 3 PASS → consensus PASS ===
header "Test 1: 3-of-3 PASS → consensus PASS, rc=0"
run_consensus 0 PASS PASS PASS PASS

# === Test 2: 2 PASS + 1 FAIL → FAIL (veto rule) ===
header "Test 2: 2-of-3 PASS + 1 FAIL → consensus FAIL, rc=1 (veto)"
run_consensus 1 FAIL PASS PASS FAIL

# === Test 3: 1 PASS + 2 FAIL → FAIL ===
header "Test 3: 1-of-3 PASS + 2 FAIL → consensus FAIL, rc=1"
run_consensus 1 FAIL PASS FAIL FAIL

# === Test 4: All FAIL → FAIL ===
header "Test 4: 3-of-3 FAIL → consensus FAIL, rc=1"
run_consensus 1 FAIL FAIL FAIL FAIL

# === Test 5: 2 PASS + 1 WARN → PASS (quorum met, no FAIL) ===
header "Test 5: 2-of-3 PASS + 1 WARN → consensus PASS, rc=0"
run_consensus 0 PASS PASS PASS WARN

# === Test 6: 1 PASS + 2 WARN → WARN (below quorum, no FAIL) ===
header "Test 6: 1-of-3 PASS + 2 WARN → consensus WARN, rc=0"
run_consensus 0 WARN PASS WARN WARN

# === Test 7: 3 WARN → WARN ===
header "Test 7: 3-of-3 WARN → consensus WARN, rc=0"
run_consensus 0 WARN WARN WARN WARN

# === Test 8: 2-CLI consensus (quorum=1) ===
header "Test 8: 2-of-2 PASS → consensus PASS, rc=0"
run_consensus 0 PASS PASS PASS

# === Test 9: 2-CLI consensus, 1 FAIL → FAIL ===
header "Test 9: 1-of-2 PASS + 1 FAIL → consensus FAIL, rc=1"
run_consensus 1 FAIL PASS FAIL

# === Test 10: 5-CLI consensus, 3 PASS + 2 FAIL → FAIL (veto) ===
header "Test 10: 3-of-5 PASS + 2 FAIL → FAIL (veto, even with quorum=3 met)"
run_consensus 1 FAIL PASS PASS PASS FAIL FAIL

# === Test 11: 5-CLI, 4 PASS + 1 WARN → PASS (no FAIL) ===
header "Test 11: 4-of-5 PASS + 1 WARN → PASS, rc=0"
run_consensus 0 PASS PASS PASS PASS PASS WARN

# === Test 12: phase alias 'audit-consensus' works ===
header "Test 12: 'audit-consensus' alias works (phase parsing)"
d=$(mktemp -d)
write_audit "$d/w1.md" PASS
write_audit "$d/w2.md" PASS
set +e
bash "$AGG" audit-consensus "$d/result.md" "$d/w1.md" "$d/w2.md" >/dev/null 2>&1
rc=$?
: # was: set -e
[ "$rc" = "0" ] && pass "audit-consensus alias accepted" || fail_ "rc=$rc"
rm -rf "$d"

# === Test 13: result.md schema parity with audit verdict mode ===
header "Test 13: result has 'Verdict:' on first line (parity with verdict mode)"
d=$(mktemp -d)
write_audit "$d/w1.md" PASS
write_audit "$d/w2.md" PASS
bash "$AGG" cross-cli-vote "$d/result.md" "$d/w1.md" "$d/w2.md" >/dev/null 2>&1
first_line=$(head -1 "$d/result.md")
if [[ "$first_line" =~ ^Verdict: ]]; then
    pass "first line is Verdict: ($first_line)"
else
    fail_ "first line: $first_line"
fi
rm -rf "$d"

# === Test 14: result.md includes per-CLI vote breakdown ===
header "Test 14: result includes per-CLI verdict breakdown"
d=$(mktemp -d)
write_audit "$d/claude-audit.md" PASS
write_audit "$d/gemini-audit.md" PASS
write_audit "$d/codex-audit.md" FAIL
bash "$AGG" cross-cli-vote "$d/result.md" "$d/claude-audit.md" "$d/gemini-audit.md" "$d/codex-audit.md" >/dev/null 2>&1
if grep -q "Per-CLI verdicts" "$d/result.md" \
   && grep -q "claude-audit=PASS" "$d/result.md" \
   && grep -q "codex-audit=FAIL" "$d/result.md"; then
    pass "per-CLI verdicts surfaced"
else
    fail_ "missing per-CLI breakdown"
fi
rm -rf "$d"

# === Test 15: aggregator help / usage ===
header "Test 15: aggregator rejects unknown phase"
d=$(mktemp -d)
echo "x" > "$d/w.md"
set +e
bash "$AGG" bogus-phase "$d/r.md" "$d/w.md" >/dev/null 2>&1
rc=$?
: # was: set -e
if [ "$rc" = "2" ]; then
    pass "unknown phase → rc=2"
else
    fail_ "rc=$rc"
fi
rm -rf "$d"

echo
echo "==========================================="
echo "  Total: 15 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
