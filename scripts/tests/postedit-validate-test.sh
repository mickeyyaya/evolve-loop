#!/usr/bin/env bash
#
# postedit-validate-test.sh — Unit tests for postedit-validate.sh.
#
# Strategy: each test creates a temp file with known content, feeds the hook
# a payload pointing at it, and asserts on exit code (always 0) and stderr
# contents (WARN messages for invalid files).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HOOK="$REPO_ROOT/scripts/postedit-validate.sh"

PASS=0; FAIL=0; TESTS_TOTAL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

run_hook() {
    local file_path="$1"
    local payload
    payload=$(printf '{"tool_input":{"file_path":"%s"}}' "$file_path")
    bash "$HOOK" <<< "$payload" 2>&1
}

cleanup_files=()
trap 'for f in "${cleanup_files[@]}"; do rm -rf "$f"; done' EXIT

# === Test 1: valid JSON → exit 0, no WARN ====================================
header "Test 1: valid .json → no WARN on stderr"
f=$(mktemp -t postedit.XXXXXX).json; cleanup_files+=("$f")
echo '{"valid":true,"n":42}' > "$f"
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && ! echo "$out" | grep -q "WARN"; then
    pass "valid json silent"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 2: invalid JSON → exit 0 (PostToolUse can't block), WARN on stderr ===
header "Test 2: invalid .json → WARN on stderr, exit 0"
f=$(mktemp -t postedit.XXXXXX).json; cleanup_files+=("$f")
echo '{"missing":closing_brace' > "$f"
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && echo "$out" | grep -qi "invalid JSON\|does NOT parse"; then
    pass "invalid json WARN issued"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 3: valid bash → silent ==============================================
header "Test 3: valid .sh → no WARN"
f=$(mktemp -t postedit.XXXXXX).sh; cleanup_files+=("$f")
cat > "$f" <<'EOF'
#!/usr/bin/env bash
echo hello
EOF
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && ! echo "$out" | grep -q "WARN"; then
    pass "valid bash silent"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 4: bash syntax error → WARN ========================================
header "Test 4: invalid .sh → WARN on stderr"
f=$(mktemp -t postedit.XXXXXX).sh; cleanup_files+=("$f")
cat > "$f" <<'EOF'
#!/usr/bin/env bash
if [ -z "$x"
echo missing-then-and-fi
EOF
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && echo "$out" | grep -qi "syntax error"; then
    pass "bash syntax error WARN"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 5: bash 3.2 incompat (declare -A) → WARN ===========================
# bash 3.2's `bash -n` may not flag `declare -A` as a syntax error
# (it's parsed as a builtin call with bad args), so this test verifies
# the validator doesn't false-negative on at least basic syntax.
header "Test 5: bash with unbalanced if → WARN"
f=$(mktemp -t postedit.XXXXXX).sh; cleanup_files+=("$f")
cat > "$f" <<'EOF'
#!/usr/bin/env bash
if [ "$1" = "x" ]; then
    echo no-fi
EOF
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && echo "$out" | grep -qi "syntax error"; then
    pass "unbalanced if caught"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 6: valid python → silent ===========================================
header "Test 6: valid .py → no WARN"
f=$(mktemp -t postedit.XXXXXX).py; cleanup_files+=("$f")
cat > "$f" <<'EOF'
def main():
    print("hello")

main()
EOF
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && ! echo "$out" | grep -q "WARN"; then
    pass "valid python silent"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 7: python syntax error → WARN ======================================
header "Test 7: invalid .py → WARN"
f=$(mktemp -t postedit.XXXXXX).py; cleanup_files+=("$f")
cat > "$f" <<'EOF'
def main(:
    print("missing paren")
EOF
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && echo "$out" | grep -qi "compile error"; then
    pass "python compile error WARN"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 8: .md file → no-op (silent) =======================================
header "Test 8: .md (unvalidated extension) → silent"
f=$(mktemp -t postedit.XXXXXX).md; cleanup_files+=("$f")
echo "# random markdown {{not valid yaml or json}}" > "$f"
out=$(run_hook "$f")
rc=$?
if [ "$rc" = "0" ] && [ -z "$out" ]; then
    pass ".md silently skipped"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 9: missing file → no-op ============================================
header "Test 9: file does not exist → no error"
out=$(run_hook "/tmp/doesnotexist-postedit-$$.sh")
rc=$?
if [ "$rc" = "0" ]; then
    pass "missing file silently skipped"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 10: bypass env honored ==============================================
header "Test 10: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1 silences even invalid"
f=$(mktemp -t postedit.XXXXXX).json; cleanup_files+=("$f")
echo '{broken' > "$f"
out=$(EVOLVE_BYPASS_POSTEDIT_VALIDATE=1 bash "$HOOK" <<< "{\"tool_input\":{\"file_path\":\"$f\"}}" 2>&1)
rc=$?
if [ "$rc" = "0" ] && ! echo "$out" | grep -q "WARN: just-edited"; then
    pass "bypass silences validator"
else
    fail_ "rc=$rc out=$out"
fi

# === Test 11: empty payload → silent + read-only guards.log doesn't leak ====
# Audit cycle 8205 DEFECT-1 caught a redirection-order bug where bash's own
# "Operation not permitted" stderr leaked when guards.log existed but was
# unwritable (auditor sandbox / read-only CI). The auditor's reproduction
# used `chmod 0444` ON THE EXISTING FILE (not chmod -w on its parent dir,
# which is a different syscall codepath). This test mirrors the auditor.
header "Test 11: read-only guards.log → exit 0 silent (no stderr leak)"
# Build an isolated REPO_ROOT-like dir that mimics the auditor sandbox.
d=$(mktemp -d -t pe-readonly.XXXXXX); cleanup_files+=("$d")
mkdir -p "$d/.evolve" "$d/scripts"
touch "$d/.evolve/guards.log"
chmod 0444 "$d/.evolve/guards.log"
cp "$HOOK" "$d/scripts/postedit-validate.sh"
# Empty payload — exercises only the early log() call path.
out=$(echo "" | bash "$d/scripts/postedit-validate.sh" 2>&1)
rc=$?
chmod 0644 "$d/.evolve/guards.log" 2>/dev/null || true
if [ "$rc" = "0" ] && [ -z "$out" ]; then
    pass "read-only guards.log → silent (rc=0, no stderr)"
else
    fail_ "rc=$rc out_len=${#out} out=$out"
fi

# === Summary ==================================================================
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="
[ "$FAIL" = "0" ] && exit 0 || exit 1
