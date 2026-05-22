#!/usr/bin/env bash
#
# preflight-test.sh — Unit tests for scripts/release/preflight.sh.
#
# Each test sets up an isolated temp repo (or runs against the real repo with
# --dry-run / --skip-tests) so the tests don't mutate the caller's state.
#
# Usage: bash scripts/release/preflight-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PREFLIGHT="$REPO_ROOT/scripts/release/preflight.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Set up a temp repo with .claude-plugin/plugin.json + an audit ledger entry.
make_repo() {
    local version="${1:-1.0.0}"
    local d
    d=$(mktemp -d -t preflight-test.XXXXXX)
    (
        cd "$d"
        git init -q -b main
        git config user.email t@t.t
        git config user.name t
        mkdir -p .claude-plugin .evolve/runs/cycle-99
        cat > .claude-plugin/plugin.json <<EOF
{"name":"x","version":"${version}"}
EOF
        # Real audit-report.md with PASS verdict.
        local now
        now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        cat > .evolve/runs/cycle-99/audit-report.md <<EOF
# Audit Report — Cycle 99

Verdict: PASS

Confidence: 1.0
EOF
        # Ledger entry — production schema (v8.14.0+: {ts, role, kind, artifact_sha256}).
        # v8.21.1: pre-v8.21.1 used {timestamp, agent, artifact_sha} which never
        # matched preflight.sh's grep '"role":"auditor"', silently failing the
        # downstream tests that depended on preflight passing.
        cat > .evolve/ledger.jsonl <<EOF
{"ts":"${now}","cycle":99,"role":"auditor","kind":"agent_subprocess","model":"opus","exit_code":0,"duration_s":"60","artifact_path":"$d/.evolve/runs/cycle-99/audit-report.md","artifact_sha256":"deadbeef","challenge_token":"x","git_head":"none","tree_state_sha":"none"}
EOF
        git add . >/dev/null 2>&1
        git commit -q -m "init" 2>&1 >/dev/null || true
    )
    echo "$d"
}

run_in() {
    local repo="$1"; shift
    # Override REPO_ROOT by symlink trick: the script computes its own root.
    # Instead invoke from the repo dir and let the script use its own derivation.
    # But preflight derives REPO_ROOT from BASH_SOURCE — we must symlink the script in.
    local script_dir="$repo/scripts/release"
    mkdir -p "$script_dir"
    cp "$PREFLIGHT" "$script_dir/preflight.sh"
    # Also copy gate-test suites so step 5 can find them (we'll usually --skip-tests).
    mkdir -p "$repo/scripts"
    for s in guards-test.sh ship-integration-test.sh role-gate-test.sh phase-gate-precondition-test.sh; do
        echo '#!/usr/bin/env bash' > "$repo/scripts/$s"
        echo 'exit 0' >> "$repo/scripts/$s"
        chmod +x "$repo/scripts/$s"
    done
    bash "$repo/scripts/release/preflight.sh" "$@" 2>&1
}

cleanup_repos=()
trap 'for r in "${cleanup_repos[@]}"; do rm -rf "$r"; done' EXIT

# === Test 1: clean tree, valid bump, recent PASS, all checks → exit 0 =========
header "Test 1: happy path → exit 0"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
set +e; out=$(run_in "$r" 1.0.1 --skip-tests); rc=$?; set -e
[ "$rc" = "0" ] && pass "happy path (rc=0)" || fail_ "rc=$rc out=$out"

# === Test 2: dirty tree → exit 1 =============================================
header "Test 2: dirty working tree → exit 1"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
echo "dirty" > "$r/dirty.txt"
git -C "$r" add dirty.txt
set +e; out=$(run_in "$r" 1.0.1 --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "dirty tree denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 3: detached HEAD → exit 1 ==========================================
header "Test 3: detached HEAD → exit 1"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
git -C "$r" checkout --detach HEAD 2>/dev/null
set +e; out=$(run_in "$r" 1.0.1 --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "detached HEAD denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 4: target-version not semver → exit 1 ===============================
header "Test 4: invalid semver target → exit 1"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
set +e; out=$(run_in "$r" "not-a-version" --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "non-semver denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 5: target-version equals current → exit 1 ===========================
header "Test 5: target equals current → exit 1"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
set +e; out=$(run_in "$r" 1.0.0 --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "no-op bump denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 6: target-version less than current → exit 1 ========================
header "Test 6: target < current → exit 1"
r=$(make_repo "2.0.0"); cleanup_repos+=("$r")
set +e; out=$(run_in "$r" 1.5.0 --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "downgrade denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 7: missing audit ledger → exit 1 ====================================
header "Test 7: no ledger.jsonl → exit 1"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
rm -f "$r/.evolve/ledger.jsonl"
set +e; out=$(run_in "$r" 1.0.1 --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "missing ledger denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 8: audit verdict not PASS → exit 1 ==================================
header "Test 8: audit-report.md verdict WARN → exit 1"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
sed -i.bak 's/Verdict: PASS/Verdict: WARN/' "$r/.evolve/runs/cycle-99/audit-report.md"
rm -f "$r/.evolve/runs/cycle-99/audit-report.md.bak"
set +e; out=$(run_in "$r" 1.0.1 --skip-tests); rc=$?; set -e
[ "$rc" = "1" ] && pass "non-PASS verdict denied (rc=1)" || fail_ "rc=$rc out=$out"

# === Test 9: --dry-run never executes mutations / never fails on test suite ===
header "Test 9: --dry-run prints would-do, exits 0"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
# Remove the test scripts entirely — dry-run should still pass.
rm -f "$r/scripts/guards-test.sh" "$r/scripts/ship-integration-test.sh" \
      "$r/scripts/role-gate-test.sh" "$r/scripts/phase-gate-precondition-test.sh"
set +e; out=$(run_in "$r" 1.0.1 --dry-run); rc=$?; set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "DRY-RUN"; then
    pass "dry-run honored (rc=0, mentions DRY-RUN)"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 10: --skip-tests bypasses step 5 only ===============================
header "Test 10: --skip-tests still runs steps 1-4"
r=$(make_repo "1.0.0"); cleanup_repos+=("$r")
# First call to run_in primes scripts/ with passing test scripts, then we
# overwrite them with failing variants. With --skip-tests, preflight should
# still pass overall (because step 5 is skipped, not because the scripts pass).
mkdir -p "$r/scripts"
for s in guards-test.sh ship-integration-test.sh role-gate-test.sh phase-gate-precondition-test.sh; do
    echo '#!/usr/bin/env bash' > "$r/scripts/$s"
    echo 'exit 1' >> "$r/scripts/$s"
    chmod +x "$r/scripts/$s"
done
# Manually set up release/ dir without the run_in helper (which would overwrite our exit-1 scripts).
mkdir -p "$r/scripts/release"
cp "$PREFLIGHT" "$r/scripts/release/preflight.sh"
set +e; out=$(bash "$r/scripts/release/preflight.sh" 1.0.1 --skip-tests 2>&1); rc=$?; set -e
[ "$rc" = "0" ] && pass "skip-tests bypasses gate suites (rc=0)" || fail_ "rc=$rc out=$out"

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
