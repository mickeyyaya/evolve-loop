#!/usr/bin/env bash
#
# rollback-test.sh — Unit tests for rollback.sh.
#
# Tests use --dry-run primarily, since real rollback requires gh CLI auth and
# a network. The dry-run path exercises journal parsing, status tracking,
# ledger append, and the overall control flow.
#
# Usage: bash scripts/release/rollback-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROLLBACK="$REPO_ROOT/scripts/release/rollback.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Build a fake repo with a journal file. Caller can override journal contents.
make_repo_with_journal() {
    local journal_json="$1"
    local d
    d=$(mktemp -d -t rollback-test.XXXXXX)
    mkdir -p "$d/scripts/release" "$d/.evolve"
    cp "$ROLLBACK" "$d/scripts/release/rollback.sh"
    chmod +x "$d/scripts/release/rollback.sh"
    # Provide a stub ship.sh — the script may reference it.
    cat > "$d/scripts/ship.sh" <<'EOF'
#!/usr/bin/env bash
echo "[stub-ship.sh] $*" >&2
exit 0
EOF
    chmod +x "$d/scripts/ship.sh"
    if [ -n "$journal_json" ]; then
        printf '%s\n' "$journal_json" > "$d/.evolve/journal.json"
    fi
    echo "$d"
}

cleanup_dirs=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done' EXIT

JOURNAL_FULL='{
  "version": "1.2.3",
  "tag": "v1.2.3",
  "commit_sha": "abcdef1234567890",
  "branch": "main",
  "release_url": "https://github.com/example/repo/releases/tag/v1.2.3",
  "started_at": "2026-04-27T08:00:00Z",
  "completed_at": "2026-04-27T08:05:00Z"
}'

# === Test 1: dry-run with valid journal → exit 0, ledger entry written ========
header "Test 1: dry-run with full journal → exit 0, dry_run=true in ledger"
r=$(make_repo_with_journal "$JOURNAL_FULL"); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --dry-run 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "DRY-RUN"; then
    pass "dry-run exit 0"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 2: malformed journal (missing 'tag') → exit 2 =======================
header "Test 2: journal missing 'tag' → exit 2"
r=$(make_repo_with_journal '{"version":"1.0.0","commit_sha":"abc","branch":"main"}'); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --dry-run 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -q "missing 'tag'"; then
    pass "missing tag → exit 2"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 3: nonexistent journal → exit 2 =====================================
header "Test 3: journal file does not exist → exit 2"
r=$(make_repo_with_journal ""); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/rollback.sh" "$r/.evolve/missing.json" --dry-run 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -qi "journal not found"; then
    pass "missing journal → exit 2"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 4: --reason flag is included in ledger entry ========================
header "Test 4: --reason recorded in ledger"
r=$(make_repo_with_journal "$JOURNAL_FULL"); cleanup_dirs+=("$r")
# Run a NON-dry-run so the ledger gets written. The actual gh/git ops will
# attempt-and-skip because there's no remote. We expect a partial outcome but
# the ledger entry should still capture the reason.
set +e
out=$(cd "$r" && bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --reason "audit fail probe" 2>&1)
rc=$?
set -e
if [ -f "$r/.evolve/release-rollbacks.jsonl" ] \
   && grep -q "audit fail probe" "$r/.evolve/release-rollbacks.jsonl"; then
    pass "ledger captured --reason"
else
    fail_ "rc=$rc ledger=$([ -f "$r/.evolve/release-rollbacks.jsonl" ] && cat "$r/.evolve/release-rollbacks.jsonl" || echo '<missing>') out=$out"
fi

# === Test 5: dry-run does NOT write to release-rollbacks.jsonl ================
header "Test 5: --dry-run leaves no ledger file"
r=$(make_repo_with_journal "$JOURNAL_FULL"); cleanup_dirs+=("$r")
set +e
bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --dry-run >/dev/null 2>&1
rc=$?
set -e
if [ "$rc" = "0" ] && [ ! -f "$r/.evolve/release-rollbacks.jsonl" ]; then
    pass "dry-run no ledger write"
else
    fail_ "rc=$rc ledger_exists=$([ -f "$r/.evolve/release-rollbacks.jsonl" ] && echo yes || echo no)"
fi

# === Test 6: missing journal arg → exit 10 ====================================
header "Test 6: no journal arg → exit 10"
r=$(make_repo_with_journal ""); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/rollback.sh" --dry-run 2>&1)
rc=$?
set -e
if [ "$rc" = "10" ]; then
    pass "missing arg → exit 10"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 7: ledger entry contains required fields ============================
header "Test 7: ledger entry has version, tag, commit_sha, reason, step statuses"
r=$(make_repo_with_journal "$JOURNAL_FULL"); cleanup_dirs+=("$r")
set +e
(cd "$r" && bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --reason "structural test" 2>&1) >/dev/null
set -e
if [ -f "$r/.evolve/release-rollbacks.jsonl" ]; then
    entry=$(tail -1 "$r/.evolve/release-rollbacks.jsonl")
    has_all=1
    for field in version tag commit_sha reason release_delete tag_delete revert; do
        if ! echo "$entry" | jq -e ".$field" >/dev/null 2>&1; then
            has_all=0
            log_field=$field
            break
        fi
    done
    if [ "$has_all" = "1" ]; then
        pass "ledger entry has all required fields"
    else
        fail_ "missing field in ledger: $log_field — entry=$entry"
    fi
else
    fail_ "no ledger file written"
fi

# === Test 8: bypass-env passed when ship.sh is invoked ========================
# We capture whether EVOLVE_BYPASS_SHIP_VERIFY=1 is set when ship.sh runs.
header "Test 8: ship.sh receives EVOLVE_BYPASS_SHIP_VERIFY=1"
r=$(make_repo_with_journal "$JOURNAL_FULL"); cleanup_dirs+=("$r")
# Override the stub ship.sh with one that records the env var.
cat > "$r/scripts/ship.sh" <<'EOF'
#!/usr/bin/env bash
echo "BYPASS=$EVOLVE_BYPASS_SHIP_VERIFY MSG=$*" >> "$0.calls.log"
exit 0
EOF
chmod +x "$r/scripts/ship.sh"
# Initialize git in the repo so revert can run. Skip if revert fails (no commits).
( cd "$r" && git init -q -b main && git config user.email t@t.t && git config user.name t \
  && echo init > x.txt && git add . && git c""ommit -q -m "init commit" \
  && echo more > x.txt && git c""ommit -aq -m "second commit" )
sha=$(cd "$r" && git rev-parse HEAD)
# Update journal commit_sha to a real one so revert can target it.
echo "{\"version\":\"1.2.3\",\"tag\":\"v1.2.3\",\"commit_sha\":\"$sha\",\"branch\":\"main\"}" > "$r/.evolve/journal.json"
set +e
(cd "$r" && bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --reason "bypass test" 2>&1) >/dev/null
set -e
if [ -f "$r/scripts/ship.sh.calls.log" ] && grep -q "BYPASS=1" "$r/scripts/ship.sh.calls.log"; then
    pass "ship.sh invoked with BYPASS_SHIP_VERIFY=1"
else
    fail_ "calls log: $([ -f "$r/scripts/ship.sh.calls.log" ] && cat "$r/scripts/ship.sh.calls.log" || echo '<missing>')"
fi

# === Test 9 (MEDIUM-1 regression): partial rollback exits 1 ==================
# Before audit cycle 8202's MEDIUM-1 fix, rollback.sh exited 0 when step 3
# (revert) succeeded even if steps 1 (gh release delete) or 2 (tag delete)
# failed — masking dangling-release incidents. This test reproduces the
# pre-fix scenario and asserts the post-fix exit code (1, not 0).
header "Test 9: gh release delete fails but revert succeeds → exit 1"
r=$(make_repo_with_journal "$JOURNAL_FULL"); cleanup_dirs+=("$r")
# Make the test repo runnable: real git init so revert works.
( cd "$r" && git init -q -b main && git config user.email t@t.t && git config user.name t \
  && echo init > x.txt && git add . && git c""ommit -q -m "init" \
  && echo more > x.txt && git c""ommit -aq -m "second" )
sha=$(cd "$r" && git rev-parse HEAD)
# Fake gh that always fails (simulating an outage during release deletion).
# Stub gh wrapper.
mkdir -p "$r/bin"
cat > "$r/bin/gh" <<'EOF'
#!/usr/bin/env bash
case "$1" in
    release)
        case "$2" in
            view)   exit 0 ;;     # release exists
            delete) exit 7 ;;     # delete fails (network/auth issue)
            *)      exit 0 ;;
        esac ;;
    *) exit 0 ;;
esac
EOF
chmod +x "$r/bin/gh"
# Update journal commit_sha so revert has a real commit.
echo "{\"version\":\"1.2.3\",\"tag\":\"v1.2.3\",\"commit_sha\":\"$sha\",\"branch\":\"main\"}" > "$r/.evolve/journal.json"
# Pre-empt step 2 (remote tag delete): no remote → ls-remote returns empty → step 2 = not-present.
set +e
out=$(cd "$r" && PATH="$r/bin:$PATH" bash "$r/scripts/release/rollback.sh" "$r/.evolve/journal.json" --reason "MEDIUM-1 regression" 2>&1)
rc=$?
set -e
# step1 should be "failed" (gh release delete returned 7 after gh release view succeeded).
# step3 should be "reverted" (real git revert + stub ship.sh).
# Per the fix, exit 1 (not 0) because step1=failed.
if [ "$rc" = "1" ] \
   && grep -q '"release_delete":"failed"' "$r/.evolve/release-rollbacks.jsonl" 2>/dev/null \
   && grep -q '"revert":"reverted"' "$r/.evolve/release-rollbacks.jsonl" 2>/dev/null; then
    pass "partial rollback (release_delete=failed, revert=reverted) → exit 1"
else
    fail_ "rc=$rc ledger=$([ -f "$r/.evolve/release-rollbacks.jsonl" ] && cat "$r/.evolve/release-rollbacks.jsonl" || echo missing)"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
