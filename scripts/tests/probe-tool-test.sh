#!/usr/bin/env bash
#
# probe-tool-test.sh — Unit tests for probe-tool.sh.
#
# Usage: bash scripts/probe-tool-test.sh
# Exit 0 = all pass; non-zero = failures.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PROBE="$REPO_ROOT/scripts/probe-tool.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# === Test 1: a tool guaranteed to exist (bash itself) → exit 0, prints path ===
header "Test 1: probe bash → exit 0, prints path"
out=$(bash "$PROBE" bash 2>&1)
rc=$?
if [ "$rc" = "0" ] && echo "$out" | grep -qE "/bash$"; then
    pass "bash found"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 2: a tool that doesn't exist → exit 1 ===============================
header "Test 2: probe nonexistent-tool-xyz → exit 1"
set +e
out=$(bash "$PROBE" definitely-not-installed-xyz123 2>&1)
rc=$?
set -e
if [ "$rc" = "1" ] && echo "$out" | grep -qi "not found"; then
    pass "nonexistent returns rc=1"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 3: --quiet suppresses stdout/stderr ================================
header "Test 3: --quiet emits no output"
out=$(bash "$PROBE" bash --quiet 2>&1)
rc=$?
if [ "$rc" = "0" ] && [ -z "$out" ]; then
    pass "quiet mode silent"
else
    fail_ "rc=$rc len(out)=${#out}"
fi

# === Test 4: --json emits JSON for found case ================================
header "Test 4: --json on found tool emits proper JSON"
out=$(bash "$PROBE" bash --json 2>/dev/null)
rc=$?
if [ "$rc" = "0" ] && echo "$out" | jq -e '.tool == "bash" and .found == true and (.path | length > 0)' >/dev/null 2>&1; then
    pass "json found-shape correct"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 5: --json emits JSON for not-found case ============================
header "Test 5: --json on missing tool emits proper JSON"
set +e
out=$(bash "$PROBE" definitely-not-installed-xyz123 --json 2>/dev/null)
rc=$?
set -e
if [ "$rc" = "1" ] && echo "$out" | jq -e '.found == false and .path == null' >/dev/null 2>&1; then
    pass "json not-found-shape correct"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 6: missing positional arg → exit 10 ================================
header "Test 6: missing tool arg → exit 10"
set +e
out=$(bash "$PROBE" 2>&1)
rc=$?
set -e
if [ "$rc" = "10" ]; then
    pass "missing arg → exit 10"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 7: tool installed at a non-PATH location → finds it ================
# Place a fake "myprobe-tool" in /tmp/probe-test-bin/ and add to PATH... actually
# the script ALSO checks $HOME/.local/bin, so we simulate that.
header "Test 7: tool at \$HOME/.local/bin found via fallback probe"
fake_bin="/tmp/probe-test-fake-$$"
mkdir -p "$fake_bin"
cat > "$fake_bin/probe-fake-cli-$$" <<'EOF'
#!/usr/bin/env bash
echo "fake"
EOF
chmod +x "$fake_bin/probe-fake-cli-$$"
# Run probe with HOME pointed at a parent of $fake_bin so that .local/bin is checked.
# Actually simpler: run with PATH cleared, with HOME set so $HOME/.local/bin matches.
parent=$(dirname "$fake_bin")
# Move the binary into a $HOME/.local/bin layout.
fake_home=$(mktemp -d -t probe-home.XXXXXX)
mkdir -p "$fake_home/.local/bin"
mv "$fake_bin/probe-fake-cli-$$" "$fake_home/.local/bin/probe-fake-cli-$$"
chmod +x "$fake_home/.local/bin/probe-fake-cli-$$"
rmdir "$fake_bin" 2>/dev/null || true
# Run probe with PATH stripped of common dirs and HOME pointed at fake_home.
out=$(env -i HOME="$fake_home" PATH="/usr/bin:/bin" bash "$PROBE" "probe-fake-cli-$$" 2>&1)
rc=$?
rm -rf "$fake_home"
if [ "$rc" = "0" ] && echo "$out" | grep -q "$fake_home/.local/bin/probe-fake-cli-$$"; then
    pass "found at \$HOME/.local/bin"
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
