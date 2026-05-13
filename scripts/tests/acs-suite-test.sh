#!/usr/bin/env bash
#
# acs-suite-test.sh — Tests for EGPS v10.0.0 predicate suite infrastructure:
#   - scripts/lib/acs-schema.sh
#   - scripts/verification/validate-predicate.sh
#   - scripts/lifecycle/run-acs-suite.sh
#   - scripts/utility/promote-acs-to-regression.sh
#
# Convention: "bash scripts/tests/acs-suite-test.sh — N/N PASS"

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"

SCHEMA="$PROJECT_ROOT/scripts/lib/acs-schema.sh"
VALIDATE="$PROJECT_ROOT/scripts/verification/validate-predicate.sh"
RUNNER="$PROJECT_ROOT/scripts/lifecycle/run-acs-suite.sh"
PROMOTE="$PROJECT_ROOT/scripts/utility/promote-acs-to-regression.sh"

PASS=0; FAIL=0; TOTAL=0
pass() { PASS=$((PASS+1)); TOTAL=$((TOTAL+1)); echo "  PASS: $1"; }
fail() { FAIL=$((FAIL+1)); TOTAL=$((TOTAL+1)); echo "  FAIL: $1"; }
header() { echo; echo "=== $* ==="; }

# Helper to create a valid predicate with full metadata.
write_valid_predicate() {
    local path=$1 ac_id=$2 ec=$3
    cat > "$path" <<F
#!/usr/bin/env bash
# AC-ID:         $ac_id
# Description:   test predicate
# Evidence:      synthetic
# Author:        test-harness
# Created:       2026-05-14T00:00:00Z
# Acceptance-of: synthetic
result=\$(echo done)
[ "\$result" = "done" ] && exit $ec
exit 1
F
    chmod +x "$path"
}

# ───────────────────────────────────────────────────────────────────────────
header "TEST 1 — acs-schema.sh: constants + filename validator"
out=$(bash -c "source $SCHEMA; echo \$ACS_EXIT_GREEN \$ACS_EXIT_RED \$ACS_EXIT_BANNED \$ACS_VERDICT_PASS \$ACS_VERDICT_FAIL")
[ "$out" = "0 1 3 PASS FAIL" ] && pass "constants resolve correctly: $out" || fail "constants got: $out"

out=$(bash -c "source $SCHEMA; acs_filename_valid 001-foo.sh && echo Y || echo N")
[ "$out" = "Y" ] && pass "filename 001-foo.sh accepted" || fail "filename 001-foo.sh rejected ($out)"

out=$(bash -c "source $SCHEMA; acs_filename_valid foo.sh && echo Y || echo N")
[ "$out" = "N" ] && pass "filename foo.sh rejected (no NNN prefix)" || fail "filename foo.sh accepted ($out)"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 2 — validate-predicate.sh: valid predicate passes"
TMP=$(mktemp -d -t acs-test.XXXXXX)
write_valid_predicate "$TMP/001-good.sh" "cycle-99-001" 0
out=$(bash "$VALIDATE" --json "$TMP/001-good.sh")
res=$(echo "$out" | jq -r '.result')
[ "$res" = "valid" ] && pass "well-formed predicate passes validator" || fail "expected valid got result=$res"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 3 — validate-predicate.sh: grep-only is BANNED"
TMP=$(mktemp -d -t acs-test.XXXXXX)
cat > "$TMP/001-grep.sh" <<'F'
#!/usr/bin/env bash
# AC-ID: cycle-99-001
# Description: bad
# Evidence: n
# Author: test
# Created: 2026-05-14
# Acceptance-of: n
grep -q "x" /etc/hostname
exit $?
F
chmod +x "$TMP/001-grep.sh"
out=$(bash "$VALIDATE" --json "$TMP/001-grep.sh"); rc=$?
[ "$rc" = "3" ] && pass "grep-only exits 3 (BANNED)" || fail "expected rc=3 got $rc"
v=$(echo "$out" | jq -r '.violations[].code' | tr '\n' ' ')
echo "$v" | grep -q "BANNED_GREP_ONLY" && pass "violation code BANNED_GREP_ONLY present" || fail "expected BANNED_GREP_ONLY in $v"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 4 — validate-predicate.sh: network call is BANNED"
TMP=$(mktemp -d -t acs-test.XXXXXX)
cat > "$TMP/001-net.sh" <<'F'
#!/usr/bin/env bash
# AC-ID: cycle-99-001
# Description: bad
# Evidence: n
# Author: test
# Created: 2026-05-14
# Acceptance-of: n
curl -fsSL https://example.com >/dev/null
exit $?
F
chmod +x "$TMP/001-net.sh"
out=$(bash "$VALIDATE" --json "$TMP/001-net.sh"); rc=$?
[ "$rc" = "3" ] && pass "network exits 3 (BANNED)" || fail "expected rc=3 got $rc"
v=$(echo "$out" | jq -r '.violations[].code' | tr '\n' ' ')
echo "$v" | grep -q "BANNED_NETWORK" && pass "violation code BANNED_NETWORK present" || fail "expected BANNED_NETWORK"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 5 — validate-predicate.sh: missing metadata fails"
TMP=$(mktemp -d -t acs-test.XXXXXX)
cat > "$TMP/001-meta.sh" <<'F'
#!/usr/bin/env bash
exit 0
F
chmod +x "$TMP/001-meta.sh"
out=$(bash "$VALIDATE" --json "$TMP/001-meta.sh"); rc=$?
[ "$rc" = "2" ] && pass "missing-metadata exits 2 (invalid)" || fail "expected rc=2 got $rc"
v=$(echo "$out" | jq -r '.violations[].code' | tr '\n' ' ')
echo "$v" | grep -q "MISSING_HEADER_AC-ID" && pass "violation MISSING_HEADER_AC-ID present" || fail "AC-ID header check did not fire"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 6 — run-acs-suite.sh: bootstrap (no predicates) → PASS"
TMP=$(mktemp -d -t acs-test.XXXXXX)
mkdir -p "$TMP/empty-acs"
EVOLVE_PROJECT_ROOT="$TMP" bash "$RUNNER" 99 --acs-dir "$TMP/empty-acs" --json > "$TMP/v.json" 2>/dev/null
verdict=$(jq -r '.verdict' "$TMP/v.json")
[ "$verdict" = "PASS" ] && pass "bootstrap (no predicates) verdict=PASS" || fail "expected PASS got $verdict"
note=$(jq -r '.note // ""' "$TMP/v.json")
echo "$note" | grep -qi "bootstrap" && pass "bootstrap note present" || fail "no bootstrap note"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 7 — run-acs-suite.sh: 2 green predicates → PASS"
TMP=$(mktemp -d -t acs-test.XXXXXX)
mkdir -p "$TMP/cycle-99"
write_valid_predicate "$TMP/cycle-99/001-a.sh" "cycle-99-001" 0
write_valid_predicate "$TMP/cycle-99/002-b.sh" "cycle-99-002" 0
bash "$RUNNER" 99 --acs-dir "$TMP" --json > "$TMP/v.json" 2>/dev/null
verdict=$(jq -r '.verdict' "$TMP/v.json")
green=$(jq -r '.green_count' "$TMP/v.json")
red=$(jq -r '.red_count' "$TMP/v.json")
[ "$verdict" = "PASS" ] && [ "$green" = "2" ] && [ "$red" = "0" ] \
    && pass "2 green → verdict=PASS green=2 red=0" || fail "got verdict=$verdict green=$green red=$red"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 8 — run-acs-suite.sh: 1 green + 1 red → FAIL"
TMP=$(mktemp -d -t acs-test.XXXXXX)
mkdir -p "$TMP/cycle-99"
write_valid_predicate "$TMP/cycle-99/001-green.sh" "cycle-99-001" 0
cat > "$TMP/cycle-99/002-red.sh" <<'F'
#!/usr/bin/env bash
# AC-ID:         cycle-99-002
echo "intentional fail" >&2
exit 1
F
chmod +x "$TMP/cycle-99/002-red.sh"
bash "$RUNNER" 99 --acs-dir "$TMP" --json > "$TMP/v.json" 2>/dev/null; rc=$?
verdict=$(jq -r '.verdict' "$TMP/v.json")
red=$(jq -r '.red_count' "$TMP/v.json")
red_ids=$(jq -r '.red_ids | join(",")' "$TMP/v.json")
[ "$verdict" = "FAIL" ] && pass "1 red predicate → verdict=FAIL" || fail "expected FAIL got $verdict"
[ "$red" = "1" ] && pass "red_count=1" || fail "expected red_count=1 got $red"
[ "$rc" = "1" ] && pass "runner exits 1 on FAIL" || fail "expected rc=1 got $rc"
echo "$red_ids" | grep -q "cycle-99-002" && pass "red_ids contains cycle-99-002" || fail "red_ids=$red_ids"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 9 — run-acs-suite.sh: regression-suite predicates included"
TMP=$(mktemp -d -t acs-test.XXXXXX)
mkdir -p "$TMP/cycle-99" "$TMP/regression-suite/cycle-30"
write_valid_predicate "$TMP/cycle-99/001-new.sh" "cycle-99-001" 0
write_valid_predicate "$TMP/regression-suite/cycle-30/001-old.sh" "cycle-30-001" 0
bash "$RUNNER" 99 --acs-dir "$TMP" --json > "$TMP/v.json" 2>/dev/null
this_cycle=$(jq -r '.predicate_suite.this_cycle_count' "$TMP/v.json")
regress=$(jq -r '.predicate_suite.regression_suite_count' "$TMP/v.json")
total=$(jq -r '.predicate_suite.total' "$TMP/v.json")
[ "$this_cycle" = "1" ] && [ "$regress" = "1" ] && [ "$total" = "2" ] \
    && pass "predicate_suite: this=1 regress=1 total=2" || fail "got this=$this_cycle regress=$regress total=$total"
# Verify is_regression flag set correctly
reg_flag=$(jq -r '.results[] | select(.predicate | contains("regression-suite")) | .is_regression' "$TMP/v.json")
[ "$reg_flag" = "true" ] && pass "is_regression=true on regression-suite predicate" || fail "is_regression=$reg_flag"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
header "TEST 10 — promote-acs-to-regression.sh: moves predicate, idempotent"
TMP=$(mktemp -d -t acs-test.XXXXXX)
mkdir -p "$TMP/acs/cycle-99"
write_valid_predicate "$TMP/acs/cycle-99/001-foo.sh" "cycle-99-001" 0
EVOLVE_PROJECT_ROOT="$TMP" bash "$PROMOTE" 99 >/dev/null 2>&1; rc=$?
[ "$rc" = "0" ] && pass "first promotion exit 0" || fail "expected rc=0 got $rc"
[ -d "$TMP/acs/regression-suite/cycle-99" ] && pass "destination dir created" || fail "destination missing"
[ -x "$TMP/acs/regression-suite/cycle-99/001-foo.sh" ] && pass "predicate moved + executable bit preserved" || fail "predicate not at expected dest"
[ ! -d "$TMP/acs/cycle-99" ] && pass "source dir removed after move" || fail "source dir still exists"
# Idempotent retry
EVOLVE_PROJECT_ROOT="$TMP" bash "$PROMOTE" 99 >/dev/null 2>&1; rc=$?
# Second call: source absent so returns 1 (acceptable; ship.sh will see no source). 
# Note: future enhancement could make this 0 + idempotent-noop.
[ "$rc" = "1" ] && pass "second promotion (source absent) returns 1 — explicit signal" || fail "unexpected rc=$rc"
rm -rf "$TMP"

# ───────────────────────────────────────────────────────────────────────────
echo
echo "=== acs-suite-test.sh — $PASS/$TOTAL PASS ($FAIL fail) ==="
[ "$FAIL" = "0" ] && exit 0 || exit 1
