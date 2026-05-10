#!/usr/bin/env bash
#
# memo-enforcement-test.sh — v8.58.0 Layer E tests.
# Verifies the structural enforcement closes the v8.57 memo-skip gap:
#   E1: gate_audit_to_ship writes .cycle-verdict = PASS dotfile
#   E1: gate_audit_to_retrospective writes .cycle-verdict = WARN/FAIL
#   E2: gate_ship_to_learn FAILs on PASS verdict + missing memo ledger entry
#   E2: gate_ship_to_learn PASSes on PASS + memo entry + carryover-todos.json
#   E2: gate_ship_to_learn PASSes on WARN/FAIL (memo not required)
# Layer E3 (dispatcher) is tested in evolve-loop-dispatch-test.sh.
# Layer E4 (ship.sh dirty-main) is tested in ship-integration-test.sh.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PG="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
SCRATCH=$(mktemp -d -t memo-enforcement.XXXXXX)
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Construct a workspace + state.json + ledger that gate_audit_to_ship will accept.
# Most existing checks (substance, fresh, ledger SHA) need to pass — we replicate
# the minimal valid fixture pattern used in scout-carryover-decision-test.sh.
make_fixture() {
    local cycle="$1" verdict="$2"  # PASS|WARN|FAIL
    local root="$SCRATCH/repo-$cycle-$RANDOM"
    mkdir -p "$root/.evolve/runs/cycle-$cycle" "$root/.evolve/evals" "$root/src"
    echo '{"instinctSummary":[],"carryoverTodos":[],"failedApproaches":[],"lastCycleNumber":0,"version":1,"mastery":{"consecutiveSuccesses":0,"level":"novice"},"ledgerSummary":{"totalTasksShipped":0}}' > "$root/.evolve/state.json"
    : > "$root/.evolve/ledger.jsonl"

    # Build report (substance check needs >=20 words, file references)
    cat > "$root/.evolve/runs/cycle-$cycle/build-report.md" <<EOF
# Build Report — Cycle $cycle
## Diff Summary
- Modified: src/widget.js (added validation pass), src/types.ts (new schema)
- Lines changed: 35 added, 12 removed across 2 files
## Test Results
All 4 unit tests pass; eval graders exit 0
## Token Cost
\$0.42 actual vs \$0.50 budget
EOF

    # Audit report with the requested verdict (gate_audit_to_ship requires PASS,
    # gate_audit_to_retrospective requires FAIL/WARN). Substance check needs
    # >=50 words and file references.
    # Use single-line "## Verdict: <X>" because the gate's regex matches same-line.
    cat > "$root/.evolve/runs/cycle-$cycle/audit-report.md" <<EOF
# Audit Report — Cycle $cycle

## Verdict: $verdict

## Eval Results
- code-grader 1: PASS — node test/widget.test.js exit 0
- code-grader 2: PASS — node test/types.test.js exit 0
- acceptance check: PASS — schema validates against src/types.ts

## Defects Found
$( [ "$verdict" = "PASS" ] && echo "None — diff in src/widget.js + src/types.ts is minimal and behavior-correct" \
   || echo "MEDIUM-1: src/widget.js validator may not handle null deep-merge case correctly" )

## Coverage
- Files audited: src/widget.js, src/types.ts, scripts/run-tests.sh
- Lines reviewed: 47 lines of diff
EOF

    # Eval definition + scout-report (needed by some gates)
    cat > "$root/.evolve/evals/stub.md" <<EOF
# Eval: stub
## Code Graders
- \`[code]\` \`node -e 'require("$root/src/widget.js")' 2>&1 | grep -v error\`
EOF
    : > "$root/src/widget.js"

    # Auditor ledger entry with valid SHA so check_subagent_ledger_match passes
    local sha
    if command -v sha256sum >/dev/null 2>&1; then
        sha=$(sha256sum "$root/.evolve/runs/cycle-$cycle/audit-report.md" | awk '{print $1}')
    else
        sha=$(shasum -a 256 "$root/.evolve/runs/cycle-$cycle/audit-report.md" | awk '{print $1}')
    fi
    echo '{"role":"auditor","cycle":'"$cycle"',"ts":"2026-05-10T00:00:00Z","kind":"agent_subprocess","exit_code":0,"artifact_path":"'"$root/.evolve/runs/cycle-$cycle/audit-report.md"'","artifact_sha256":"'"$sha"'"}' > "$root/.evolve/ledger.jsonl"

    echo "$root"
}

append_memo_entry() {
    local root="$1" cycle="$2"
    echo '{"role":"memo","cycle":'"$cycle"',"ts":"2026-05-10T00:00:00Z","kind":"agent_subprocess","exit_code":0}' >> "$root/.evolve/ledger.jsonl"
}

# --- Test 1 (E1): gate_audit_to_ship writes .cycle-verdict=PASS -----------
header "Test 1 (E1): gate_audit_to_ship writes .cycle-verdict=PASS"
ROOT=$(make_fixture 1001 PASS)
WS="$ROOT/.evolve/runs/cycle-1001"
# state-checksum needed by check_git_diff_substance / verify_state_checksum
# Most anti-forgery checks are skipped when these helpers can't find their data.
# Use EVOLVE_BYPASS_PHASE_GATE? No, we want the real flow. Some checks may fail
# in the synthetic fixture — capture the gate's result regardless.
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" audit-to-ship 1001 "$WS" >/dev/null 2>&1
RC=$?
set -e
# Don't assert RC=0 here — the fixture may not satisfy all anti-forgery checks.
# What matters: when the gate REACHES the verdict-check section, it should
# write .cycle-verdict regardless of subsequent checks. Test 1's contract is
# "if PASS verdict observed, .cycle-verdict file exists with content PASS".
if [ -f "$WS/.cycle-verdict" ]; then
    VERDICT=$(cat "$WS/.cycle-verdict")
    [ "$VERDICT" = "PASS" ] && pass ".cycle-verdict written with content PASS" || \
        fail_ ".cycle-verdict has wrong content: '$VERDICT' (expected PASS)"
else
    fail_ ".cycle-verdict not written (rc=$RC) — gate exited before verdict-write"
fi

# --- Test 2 (E1): gate_audit_to_retrospective writes .cycle-verdict=WARN --
header "Test 2 (E1): gate_audit_to_retrospective writes .cycle-verdict=WARN"
ROOT=$(make_fixture 1002 WARN)
WS="$ROOT/.evolve/runs/cycle-1002"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" audit-to-retrospective 1002 "$WS" >/dev/null 2>&1
set -e
if [ -f "$WS/.cycle-verdict" ]; then
    VERDICT=$(cat "$WS/.cycle-verdict")
    [ "$VERDICT" = "WARN" ] && pass ".cycle-verdict written with content WARN" || \
        fail_ ".cycle-verdict has wrong content: '$VERDICT' (expected WARN)"
else
    fail_ ".cycle-verdict not written"
fi

# --- Test 3 (E1): FAIL verdict ---------------------------------------------
header "Test 3 (E1): gate_audit_to_retrospective writes .cycle-verdict=FAIL"
ROOT=$(make_fixture 1003 FAIL)
WS="$ROOT/.evolve/runs/cycle-1003"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" audit-to-retrospective 1003 "$WS" >/dev/null 2>&1
set -e
if [ -f "$WS/.cycle-verdict" ]; then
    VERDICT=$(cat "$WS/.cycle-verdict")
    [ "$VERDICT" = "FAIL" ] && pass ".cycle-verdict written with content FAIL" || \
        fail_ ".cycle-verdict has wrong content: '$VERDICT' (expected FAIL)"
else
    fail_ ".cycle-verdict not written"
fi

# --- Test 4 (E2): gate_ship_to_learn FAILs on PASS without memo ledger -----
header "Test 4 (E2): gate_ship_to_learn FAILs on PASS verdict + missing memo"
ROOT=$(make_fixture 1004 PASS)
WS="$ROOT/.evolve/runs/cycle-1004"
echo "PASS" > "$WS/.cycle-verdict"
# NOTE: working tree must be clean for gate_ship_to_learn's git-clean check.
# We use a temp git repo so this script doesn't dirty the actual repo.
TEST_GIT_DIR=$(mktemp -d -t memo-test-git.XXXXXX)
( cd "$TEST_GIT_DIR" && git init -q && git config user.email t@t && git config user.name t && git commit --allow-empty -q -m init )
# Run gate inside the test git dir so its `git status --porcelain` shows clean.
set +e
( cd "$TEST_GIT_DIR" && EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" ship-to-learn 1004 "$WS" >/dev/null 2>&1 )
RC=$?
set -e
[ "$RC" != "0" ] && pass "gate FAILed on PASS without memo ledger entry (rc=$RC)" || \
    fail_ "gate ALLOWED PASS without memo (rc=$RC)"
rm -rf "$TEST_GIT_DIR"

# --- Test 5 (E2): gate_ship_to_learn PASSes on PASS + memo + json ---------
header "Test 5 (E2): gate_ship_to_learn PASSes when memo + carryover-todos.json present"
ROOT=$(make_fixture 1005 PASS)
WS="$ROOT/.evolve/runs/cycle-1005"
echo "PASS" > "$WS/.cycle-verdict"
append_memo_entry "$ROOT" 1005
echo "[]" > "$WS/carryover-todos.json"
TEST_GIT_DIR=$(mktemp -d -t memo-test-git.XXXXXX)
( cd "$TEST_GIT_DIR" && git init -q && git config user.email t@t && git config user.name t && git commit --allow-empty -q -m init )
set +e
( cd "$TEST_GIT_DIR" && EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" ship-to-learn 1005 "$WS" >/dev/null 2>&1 )
RC=$?
set -e
[ "$RC" = "0" ] && pass "gate PASSed on PASS + memo + carryover-todos.json (rc=0)" || \
    fail_ "gate REJECTED valid PASS-with-memo (rc=$RC)"
rm -rf "$TEST_GIT_DIR"

# --- Test 6 (E2): gate_ship_to_learn PASSes on WARN — memo not required ---
header "Test 6 (E2): gate_ship_to_learn PASSes on WARN (memo not required)"
ROOT=$(make_fixture 1006 WARN)
WS="$ROOT/.evolve/runs/cycle-1006"
echo "WARN" > "$WS/.cycle-verdict"
TEST_GIT_DIR=$(mktemp -d -t memo-test-git.XXXXXX)
( cd "$TEST_GIT_DIR" && git init -q && git config user.email t@t && git config user.name t && git commit --allow-empty -q -m init )
set +e
( cd "$TEST_GIT_DIR" && EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" ship-to-learn 1006 "$WS" >/dev/null 2>&1 )
RC=$?
set -e
[ "$RC" = "0" ] && pass "gate PASSed on WARN without memo (rc=0)" || \
    fail_ "gate REJECTED WARN-no-memo (rc=$RC) — should not require memo on non-PASS"
rm -rf "$TEST_GIT_DIR"

# --- Test 7 (E2): missing .cycle-verdict on PASS = backward-compatible -----
header "Test 7 (E2): missing .cycle-verdict treated as legacy (no enforcement)"
# Backward-compat: if .cycle-verdict isn't present, the new check skips.
# Pre-v8.58 cycles must still ship without .cycle-verdict file.
ROOT=$(make_fixture 1007 PASS)
WS="$ROOT/.evolve/runs/cycle-1007"
# Intentionally do NOT create .cycle-verdict
TEST_GIT_DIR=$(mktemp -d -t memo-test-git.XXXXXX)
( cd "$TEST_GIT_DIR" && git init -q && git config user.email t@t && git config user.name t && git commit --allow-empty -q -m init )
set +e
( cd "$TEST_GIT_DIR" && EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" ship-to-learn 1007 "$WS" >/dev/null 2>&1 )
RC=$?
set -e
[ "$RC" = "0" ] && pass "gate PASSed when .cycle-verdict missing (legacy path)" || \
    fail_ "gate REJECTED legacy cycle (rc=$RC)"
rm -rf "$TEST_GIT_DIR"

# --- Summary ----------------------------------------------------------------
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
