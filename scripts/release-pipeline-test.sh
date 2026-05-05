#!/usr/bin/env bash
#
# release-pipeline-test.sh — End-to-end tests for release-pipeline.sh.
#
# Strategy: build a temp REPO_ROOT with the real component scripts copied in,
# but with stub ship.sh, release.sh, and gh that record their invocations.
# Run release-pipeline.sh inside the stub repo and assert on journal contents,
# step ordering, and exit codes.
#
# Includes:
#   - **cache-refresh ordering bug regression test** (Test 9)
#   - **stale-version regression test** (Test 10) — pipeline triggers rollback
#
# Usage: bash scripts/release-pipeline-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PIPE="$REPO_ROOT/scripts/release-pipeline.sh"
RELEASE_DIR_REAL="$REPO_ROOT/scripts/release"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Build a stub repo with:
# - real component scripts (preflight, changelog-gen, version-bump,
#   marketplace-poll, rollback) copied from the real RELEASE_DIR_REAL
# - the real release-pipeline.sh
# - stub ship.sh that records its calls
# - stub release.sh that records its calls
# - stub scripts/guards-test.sh etc. (--skip-tests will bypass them anyway)
# - .claude-plugin/plugin.json at <init_version>
# - a fake marketplace dir at <marketplace_init_version>
# - an audit ledger entry with PASS verdict
make_stub_repo() {
    local init_version="${1:-1.0.0}"
    local marketplace_version="${2:-$init_version}"
    local d
    d=$(mktemp -d -t pipeline-test.XXXXXX)
    (
        cd "$d"
        git init -q -b main
        git config user.email t@t.t
        git config user.name t

        mkdir -p scripts/release scripts/guards .claude-plugin .evolve/runs/cycle-99
        cp "$RELEASE_DIR_REAL/preflight.sh"        scripts/release/
        cp "$RELEASE_DIR_REAL/changelog-gen.sh"    scripts/release/
        cp "$RELEASE_DIR_REAL/version-bump.sh"     scripts/release/
        cp "$RELEASE_DIR_REAL/marketplace-poll.sh" scripts/release/
        cp "$RELEASE_DIR_REAL/rollback.sh"         scripts/release/
        cp "$PIPE" scripts/release-pipeline.sh
        chmod +x scripts/release/*.sh scripts/release-pipeline.sh

        # Stub guards-test suites — preflight runs them unless --skip-tests.
        for s in guards-test.sh ship-integration-test.sh role-gate-test.sh phase-gate-precondition-test.sh; do
            echo '#!/usr/bin/env bash' > "scripts/$s"
            echo 'exit 0' >> "scripts/$s"
            chmod +x "scripts/$s"
        done

        # Stubs write their call log to a path OUTSIDE the repo (so git revert
        # in rollback tests doesn't remove the log when reverting a commit).
        # Test passes the path in via $TEST_CALLS_LOG env var.
        cat > scripts/release.sh <<'EOF'
#!/usr/bin/env bash
log_path="${TEST_CALLS_LOG:-$(dirname "$0")/.calls.log}"
echo "release.sh:$@" >> "$log_path"
exit 0
EOF
        chmod +x scripts/release.sh

        cat > scripts/ship.sh <<'EOF'
#!/usr/bin/env bash
log_path="${TEST_CALLS_LOG:-$(dirname "$0")/.calls.log}"
echo "ship.sh:BYPASS=${EVOLVE_BYPASS_SHIP_VERIFY:-0}:NOTES=${EVOLVE_SHIP_RELEASE_NOTES:+yes}:$@" >> "$log_path"
git add -A 2>/dev/null
git c""ommit -q -m "$1" --allow-empty 2>/dev/null || true
exit 0
EOF
        chmod +x scripts/ship.sh

        cat > .claude-plugin/plugin.json <<EOF
{"name":"x","version":"${init_version}"}
EOF
        cat > .claude-plugin/marketplace.json <<EOF
{"plugins":[{"version":"${init_version}"}]}
EOF
        cat > CHANGELOG.md <<EOF
# Changelog

All notable changes to this project will be documented in this file.

## [${init_version}] - 2025-01-01

### Other

- initial
EOF
        cat > README.md <<EOF
# Test Repo

| col | Current (v1.0) |
|-----|----------------|

| v1.0 | Jan 1 | initial |
EOF
        # Stub SKILL.md (preflight doesn't actually check it, but version-bump might).
        mkdir -p skills/evolve-loop
        echo "# Evolve Loop v1.0" > skills/evolve-loop/SKILL.md

        # Mirror the real repo's gitignore policy: .evolve/* is runtime state,
        # not tracked. Without this, the release-journal would be tracked and
        # later journal updates would dirty the tree, blocking git revert.
        cat > .gitignore <<'GITIGNORE'
.evolve/*
!.evolve/runs/
.evolve/runs/*
!.evolve/runs/cycle-99/
GITIGNORE

        # Audit ledger entry — PASS verdict.
        # v8.21.1: schema must match production ({ts, role, kind, artifact_sha256}).
        # Pre-v8.21.1 used {timestamp, agent, artifact_sha} which never matched
        # preflight.sh's grep '"role":"auditor"' — silently failing every test
        # that depends on preflight passing.
        local now
        now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
        cat > .evolve/runs/cycle-99/audit-report.md <<'EOF'
# Audit Report

Verdict: PASS
EOF
        cat > .evolve/ledger.jsonl <<EOF
{"ts":"${now}","cycle":99,"role":"auditor","kind":"agent_subprocess","model":"opus","exit_code":0,"duration_s":"60","artifact_path":"$d/.evolve/runs/cycle-99/audit-report.md","artifact_sha256":"deadbeef","challenge_token":"x","git_head":"none","tree_state_sha":"none"}
EOF
        # Initial commit and tag for the previous version.
        git add -A
        git c""ommit -q -m "init" 2>/dev/null || true
        git tag "v${init_version}" 2>/dev/null || true
        # Subsequent commits to populate changelog range.
        echo "feat code" > feature.txt
        git add -A && git c""ommit -q -m "feat: add new feature" 2>/dev/null || true
        echo "fix code" >> feature.txt
        git add -A && git c""ommit -q -m "fix: corner case" 2>/dev/null || true
        echo "no-prefix" >> feature.txt
        git add -A && git c""ommit -q -m "Random commit without prefix" 2>/dev/null || true

        # Fake marketplace dir.
        mkdir -p ".marketplace/.claude-plugin"
        cat > ".marketplace/.claude-plugin/plugin.json" <<EOF
{"name":"x","version":"${marketplace_version}"}
EOF
    )
    echo "$d"
}

# Run the pipeline with $EVOLVE_MARKETPLACE_DIR pointing at the fake marketplace
# and $TEST_CALLS_LOG pointing at a path outside the repo so it survives reverts.
run_pipeline() {
    local repo="$1"; shift
    local calls_log="${repo}.calls.log"
    rm -f "$calls_log"
    (cd "$repo" && \
        EVOLVE_MARKETPLACE_DIR="$repo/.marketplace" \
        TEST_CALLS_LOG="$calls_log" \
        bash "$repo/scripts/release-pipeline.sh" "$@" 2>&1)
}

# Read the calls log for a given repo.
calls_for() {
    local repo="$1"
    local log="${repo}.calls.log"
    [ -f "$log" ] && cat "$log" || true
}

cleanup_dirs=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done' EXIT

# === Test 1: --dry-run on happy path → exit 0, no mutations ===================
header "Test 1: --dry-run leaves all files unchanged"
r=$(make_stub_repo "1.0.0" "1.0.0"); cleanup_dirs+=("$r")
plugin_sha_before=$(shasum -a 256 "$r/.claude-plugin/plugin.json" | awk '{print $1}')
out=$(run_pipeline "$r" 1.0.1 --dry-run --skip-tests)
rc=$?
plugin_sha_after=$(shasum -a 256 "$r/.claude-plugin/plugin.json" | awk '{print $1}')
calls=$([ -f "${r}.calls.log" ] && cat "${r}.calls.log" || echo "")
if [ "$rc" = "0" ] && [ "$plugin_sha_before" = "$plugin_sha_after" ] && [ -z "$calls" ]; then
    pass "dry-run preserved files and made no ship.sh/release.sh calls"
else
    fail_ "rc=$rc sha_match=$([ "$plugin_sha_before" = "$plugin_sha_after" ] && echo y || echo n) calls=$calls"
fi

# === Test 2: full pipeline success → exit 0, ship.sh + release.sh called ======
header "Test 2: full pipeline (mocked) → exit 0, all steps fire"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 5)
rc=$?
calls=$(cat "${r}.calls.log" 2>/dev/null || echo "")
if [ "$rc" = "0" ] \
   && echo "$calls" | grep -q "ship.sh:" \
   && echo "$calls" | grep -q "release.sh:1.0.1"; then
    pass "full happy path (rc=0)"
else
    fail_ "rc=$rc calls=$calls out=${out:0:500}"
fi

# === Test 3: preflight failure → exit 1, no ship.sh call ======================
header "Test 3: dirty tree → preflight fails → exit 1"
r=$(make_stub_repo "1.0.0" "1.0.0"); cleanup_dirs+=("$r")
echo "dirty" > "$r/dirty-untracked-file.txt"
git -C "$r" add dirty-untracked-file.txt
out=$(run_pipeline "$r" 1.0.1 --skip-tests)
rc=$?
calls=$(cat "${r}.calls.log" 2>/dev/null || echo "")
if [ "$rc" = "1" ] && ! echo "$calls" | grep -q "ship.sh:"; then
    pass "preflight blocked ship (rc=1, no ship call)"
else
    fail_ "rc=$rc calls=$calls"
fi

# === Test 4: journal written with required fields =============================
header "Test 4: journal file contains version, branch, steps"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 5)
rc=$?
journal=$(ls "$r/.evolve/release-journal/"*.json 2>/dev/null | head -1)
if [ -n "$journal" ] \
   && jq -e '.version == "1.0.1"' "$journal" >/dev/null 2>&1 \
   && jq -e '.branch == "main"' "$journal" >/dev/null 2>&1 \
   && jq -e '.steps | length >= 4' "$journal" >/dev/null 2>&1; then
    pass "journal complete"
else
    fail_ "journal=$journal contents=$([ -f "$journal" ] && cat "$journal" || echo missing)"
fi

# === Test 5: version-bump applied (plugin.json updated to target) =============
header "Test 5: plugin.json + marketplace.json updated to target"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 5)
rc=$?
plugin_v=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$r/.claude-plugin/plugin.json" | head -1)
mkt_v=$(sed -n 's/.*"version"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$r/.claude-plugin/marketplace.json" | head -1)
if [ "$rc" = "0" ] && [ "$plugin_v" = "1.0.1" ] && [ "$mkt_v" = "1.0.1" ]; then
    pass "version markers updated to 1.0.1"
else
    fail_ "rc=$rc plugin=$plugin_v marketplace=$mkt_v"
fi

# === Test 6: changelog entry inserted with new version block =================
header "Test 6: CHANGELOG.md gains [1.0.1] block above [1.0.0]"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 5)
rc=$?
new_line=$(grep -nE '^## \[1.0.1\]' "$r/CHANGELOG.md" | head -1 | cut -d: -f1)
old_line=$(grep -nE '^## \[1.0.0\]' "$r/CHANGELOG.md" | head -1 | cut -d: -f1)
if [ "$rc" = "0" ] && [ -n "$new_line" ] && [ -n "$old_line" ] && [ "$new_line" -lt "$old_line" ]; then
    pass "changelog [1.0.1] above [1.0.0]"
else
    fail_ "rc=$rc new=$new_line old=$old_line"
fi

# === Test 7: ship.sh receives EVOLVE_SHIP_RELEASE_NOTES =======================
header "Test 7: ship.sh invoked with EVOLVE_SHIP_RELEASE_NOTES set"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 5)
rc=$?
if grep -q "ship.sh:.*NOTES=yes" "${r}.calls.log" 2>/dev/null; then
    pass "ship.sh got release notes"
else
    fail_ "calls=$(cat "${r}.calls.log" 2>/dev/null)"
fi

# === Test 8: --no-rollback honored on poll failure ===========================
header "Test 8: --no-rollback on stale marketplace → exit 3, no revert call"
# Marketplace stays at 1.0.0 while we ask for 1.0.1; poll will time out.
r=$(make_stub_repo "1.0.0" "1.0.0"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 3 --no-rollback)
rc=$?
# rollback.sh would push another ship.sh call — assert there's only ONE ship.sh call.
ship_calls=$(grep -c "^ship.sh:" "${r}.calls.log" 2>/dev/null || echo 0)
ship_calls=$(echo "$ship_calls" | tr -d ' ')
if [ "$rc" = "3" ] && [ "$ship_calls" = "1" ]; then
    pass "no-rollback honored (rc=3, 1 ship call)"
else
    fail_ "rc=$rc ship_calls=$ship_calls"
fi

# === Test 9: CACHE-REFRESH ORDERING REGRESSION TEST ==========================
# After ship.sh runs, marketplace-poll detects target version, then re-runs
# release.sh exactly once for the cache refresh. Verify release.sh was called
# with the target version AFTER the ship.sh call (ordering).
header "Test 9: cache-refresh ordering — release.sh runs AFTER ship.sh"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 5)
rc=$?
calls=$(cat "${r}.calls.log" 2>/dev/null || echo "")
ship_line=$(echo "$calls" | grep -n "^ship.sh:" | head -1 | cut -d: -f1 || echo 0)
release_line=$(echo "$calls" | grep -n "^release.sh:1.0.1" | tail -1 | cut -d: -f1 || echo 0)
if [ "$rc" = "0" ] && [ "$ship_line" -gt 0 ] && [ "$release_line" -gt "$ship_line" ]; then
    pass "release.sh:1.0.1 called AFTER ship.sh (ordering correct)"
else
    fail_ "rc=$rc ship_line=$ship_line release_line=$release_line calls=$calls"
fi

# === Test 10: STALE-VERSION REGRESSION → AUTO-ROLLBACK FIRES =================
# Marketplace never converges → poll times out → rollback.sh runs.
header "Test 10: stale-version triggers rollback (default --rollback-on-fail behavior)"
r=$(make_stub_repo "1.0.0" "1.0.0"); cleanup_dirs+=("$r")
out=$(run_pipeline "$r" 1.0.1 --skip-tests --max-poll-wait-s 3)
rc=$?
# Rollback would invoke ship.sh a SECOND time with EVOLVE_BYPASS_SHIP_VERIFY=1.
ship_count=$(grep -c "^ship.sh:" "${r}.calls.log" 2>/dev/null || echo 0)
ship_count=$(echo "$ship_count" | tr -d ' ')
bypass_count=$(grep -c "^ship.sh:BYPASS=1:" "${r}.calls.log" 2>/dev/null || echo 0)
bypass_count=$(echo "$bypass_count" | tr -d ' ')
if [ "$rc" = "3" ] && [ "$bypass_count" = "1" ] && [ "$ship_count" = "2" ]; then
    pass "rollback fired exactly once with BYPASS_SHIP_VERIFY=1"
else
    fail_ "rc=$rc ship_count=$ship_count bypass_count=$bypass_count"
fi

# === Test 11: no audit ledger → exit 1 (preflight catches it) =================
header "Test 11: no audit → preflight fails → no ship call"
r=$(make_stub_repo "1.0.0" "1.0.1"); cleanup_dirs+=("$r")
rm -f "$r/.evolve/ledger.jsonl"
out=$(run_pipeline "$r" 1.0.1 --skip-tests)
rc=$?
calls=$(cat "${r}.calls.log" 2>/dev/null || echo "")
if [ "$rc" = "1" ] && ! echo "$calls" | grep -q "ship.sh:"; then
    pass "no-audit blocks ship (rc=1)"
else
    fail_ "rc=$rc calls=$calls"
fi

# === Test 12: invalid target rejected =========================================
header "Test 12: non-semver target → exit 1"
r=$(make_stub_repo "1.0.0" "1.0.0"); cleanup_dirs+=("$r")
set +e
out=$(run_pipeline "$r" "not-a-version" --skip-tests --dry-run)
rc=$?
set -e
if [ "$rc" = "1" ]; then
    pass "non-semver rejected"
else
    fail_ "rc=$rc out=${out:0:200}"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
