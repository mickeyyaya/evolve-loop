#!/usr/bin/env bash
#
# marketplace-poll-test.sh — Unit tests for marketplace-poll.sh.
#
# Each test creates a fake marketplace dir (a plain directory with
# .claude-plugin/plugin.json — NOT a git checkout for most tests; the script's
# pull_marketplace silently no-ops when there's no .git, which is the right
# behavior for the test harness).
#
# Includes the **stale-version regression test** (Test 3): poll a marketplace
# that will never match → exit 1 within --max-wait-s.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
POLL="$REPO_ROOT/scripts/release/marketplace-poll.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# Make a fake marketplace dir at the given version.
make_marketplace() {
    local version="$1"
    local d
    d=$(mktemp -d -t mkpoll.XXXXXX)
    mkdir -p "$d/.claude-plugin"
    cat > "$d/.claude-plugin/plugin.json" <<EOF
{"name":"evolve-loop","version":"${version}"}
EOF
    echo "$d"
}

# Build a fake REPO_ROOT for tests that need release.sh — most tests don't reach
# the release.sh refresh step (they fail via timeout before that), but Test 5
# needs it. We'll provide a stub release.sh that just exits 0.
make_fake_repo() {
    local d
    d=$(mktemp -d -t mkpoll-repo.XXXXXX)
    mkdir -p "$d/scripts/release"
    cp "$POLL" "$d/scripts/release/marketplace-poll.sh"
    chmod +x "$d/scripts/release/marketplace-poll.sh"
    cat > "$d/scripts/release.sh" <<'EOF'
#!/usr/bin/env bash
echo "[fake-release.sh] $@" >&2
exit 0
EOF
    chmod +x "$d/scripts/release.sh"
    echo "$d"
}

cleanup_dirs=()
trap 'for d in "${cleanup_dirs[@]}"; do rm -rf "$d"; done' EXIT

# === Test 1: marketplace already at target → match on first poll → exit 0 =====
header "Test 1: match on first poll → exit 0"
m=$(make_marketplace "1.2.3"); cleanup_dirs+=("$m")
r=$(make_fake_repo); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.2.3 \
    --marketplace-dir "$m" --max-wait-s 5 --poll-interval-s 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "converged to v1.2.3"; then
    pass "first-poll match (rc=0)"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 2: marketplace catches up after 2 polls → exit 0 ====================
header "Test 2: matches after N polls → exit 0"
m=$(make_marketplace "0.9.0"); cleanup_dirs+=("$m")
r=$(make_fake_repo); cleanup_dirs+=("$r")
# Schedule a background bump to v1.2.3 after 2 seconds.
( sleep 2; echo '{"name":"evolve-loop","version":"1.2.3"}' > "$m/.claude-plugin/plugin.json" ) &
bg_pid=$!
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.2.3 \
    --marketplace-dir "$m" --max-wait-s 10 --poll-interval-s 1 2>&1)
rc=$?
wait $bg_pid 2>/dev/null
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "converged to v1.2.3"; then
    pass "matches after delay (rc=0)"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 3: STALE-VERSION REGRESSION TEST — never matches → exit 1 ==========
header "Test 3: stale-version regression — never matches, exit 1 within deadline"
m=$(make_marketplace "0.9.0"); cleanup_dirs+=("$m")
r=$(make_fake_repo); cleanup_dirs+=("$r")
# Marketplace stays at 0.9.0; we want 1.2.3; max_wait=4s, interval=1s.
start=$(date -u +%s)
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.2.3 \
    --marketplace-dir "$m" --max-wait-s 4 --poll-interval-s 1 2>&1)
rc=$?
set -e
elapsed=$(( $(date -u +%s) - start ))
if [ "$rc" = "1" ] && echo "$out" | grep -q "TIMEOUT" && [ "$elapsed" -le 8 ]; then
    pass "stale-version timeout (rc=1, ${elapsed}s)"
else
    fail_ "rc=$rc elapsed=${elapsed}s out=$out"
fi

# === Test 4: missing marketplace dir → exit 2 (runtime error) =================
header "Test 4: missing marketplace dir → exit 2"
r=$(make_fake_repo); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.0.0 \
    --marketplace-dir "/tmp/does-not-exist-mkpoll-$$" --max-wait-s 2 --poll-interval-s 1 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -qi "not found"; then
    pass "missing dir → exit 2"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 5: CACHE-REFRESH ORDERING REGRESSION — release.sh runs after match ==
# The original bug: release.sh ran before push, found stale marketplace,
# left installed_plugins.json untouched. The fix: poll-first, only invoke
# release.sh AFTER marketplace converged. We verify that the script DOES
# invoke release.sh exactly once after convergence.
header "Test 5: cache-refresh ordering — release.sh invoked AFTER convergence"
m=$(make_marketplace "1.2.3"); cleanup_dirs+=("$m")
r=$(mktemp -d -t mkpoll-repo.XXXXXX); cleanup_dirs+=("$r")
mkdir -p "$r/scripts/release"
cp "$POLL" "$r/scripts/release/marketplace-poll.sh"
chmod +x "$r/scripts/release/marketplace-poll.sh"
# Fake release.sh that records the version arg it was called with.
SENTINEL="$r/release-sh-called.log"
cat > "$r/scripts/release.sh" <<EOF
#!/usr/bin/env bash
echo "called with: \$*" >> "$SENTINEL"
exit 0
EOF
chmod +x "$r/scripts/release.sh"
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.2.3 \
    --marketplace-dir "$m" --max-wait-s 5 --poll-interval-s 1 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && [ -f "$SENTINEL" ] && grep -q "called with: 1.2.3" "$SENTINEL"; then
    invocations=$(wc -l < "$SENTINEL" | tr -d ' ')
    if [ "$invocations" = "1" ]; then
        pass "release.sh called exactly once with target version"
    else
        fail_ "release.sh called $invocations times (expected 1)"
    fi
else
    fail_ "rc=$rc sentinel exists=$([ -f "$SENTINEL" ] && echo yes || echo no) out=$out"
fi

# === Test 6: malformed plugin.json in marketplace → exit 2 ====================
header "Test 6: marketplace plugin.json missing → exit 2"
m=$(mktemp -d -t mkpoll-bad.XXXXXX); cleanup_dirs+=("$m")
mkdir -p "$m/.claude-plugin"
# No plugin.json file at all.
r=$(make_fake_repo); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.0.0 \
    --marketplace-dir "$m" --max-wait-s 2 --poll-interval-s 1 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ]; then
    pass "missing plugin.json → exit 2"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 7: --dry-run never polls, never invokes release.sh ==================
header "Test 7: --dry-run prints intent, exits 0, makes no calls"
m=$(make_marketplace "0.9.0"); cleanup_dirs+=("$m")
r=$(make_fake_repo); cleanup_dirs+=("$r")
SENTINEL="$r/release-sh-called.log"
cat > "$r/scripts/release.sh" <<EOF
#!/usr/bin/env bash
echo "DRY-RUN-LEAK: \$*" >> "$SENTINEL"
exit 1
EOF
chmod +x "$r/scripts/release.sh"
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.2.3 \
    --marketplace-dir "$m" --max-wait-s 60 --poll-interval-s 1 --dry-run 2>&1)
rc=$?
set -e
if [ "$rc" = "0" ] && echo "$out" | grep -q "DRY-RUN" && [ ! -f "$SENTINEL" ]; then
    pass "dry-run no side effects (rc=0)"
else
    fail_ "rc=$rc sentinel=$([ -f "$SENTINEL" ] && echo yes || echo no) out=$out"
fi

# === Test 8: invalid --max-wait-s rejected ====================================
header "Test 8: --max-wait-s non-integer → exit 10"
r=$(make_fake_repo); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.0.0 \
    --max-wait-s abc --poll-interval-s 1 2>&1)
rc=$?
set -e
if [ "$rc" = "10" ]; then
    pass "non-integer max-wait rejected (rc=10)"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 9: invalid target semver rejected ===================================
header "Test 9: non-semver target → exit 2"
r=$(make_fake_repo); cleanup_dirs+=("$r")
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" "garbage" --max-wait-s 2 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ]; then
    pass "bad target rejected (rc=2)"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 10: release.sh failure surfaces as exit 2 ===========================
header "Test 10: release.sh exits non-zero → poll exits 2"
m=$(make_marketplace "1.2.3"); cleanup_dirs+=("$m")
r=$(mktemp -d -t mkpoll-repo.XXXXXX); cleanup_dirs+=("$r")
mkdir -p "$r/scripts/release"
cp "$POLL" "$r/scripts/release/marketplace-poll.sh"
chmod +x "$r/scripts/release/marketplace-poll.sh"
cat > "$r/scripts/release.sh" <<'EOF'
#!/usr/bin/env bash
exit 7
EOF
chmod +x "$r/scripts/release.sh"
set +e
out=$(bash "$r/scripts/release/marketplace-poll.sh" 1.2.3 \
    --marketplace-dir "$m" --max-wait-s 5 --poll-interval-s 1 2>&1)
rc=$?
set -e
if [ "$rc" = "2" ] && echo "$out" | grep -qi "release.sh exited"; then
    pass "release.sh failure propagates (rc=2)"
else
    fail_ "rc=$rc out=$out"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
