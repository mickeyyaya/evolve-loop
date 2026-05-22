#!/usr/bin/env bash
#
# verify-ledger-chain-test.sh — Tests for v8.37.0 tamper-evident ledger.
#
# Synthesizes various ledger states (clean, mutated, truncated, forged,
# pre-v8.37/v8.37 mixed) and verifies the verifier returns the right exit
# code and surfaces the right diagnostics.
#
# Each test creates a tmp dir; uses --ledger flag to point verifier at the
# synthesized file. Never touches the real .evolve/ledger.jsonl.
#
# Usage: bash scripts/verify-ledger-chain-test.sh

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/observability/verify-ledger-chain.sh"
SCRATCH=$(mktemp -d -t "verify-ledger-XXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

sha256_stdin() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
}

# Helper: synthesize an N-entry chained ledger with a matching tip file.
# Args: <out-dir> <count>
make_clean_chain() {
    local outdir="$1" count="$2"
    local ledger="$outdir/ledger.jsonl"
    local tip="$outdir/ledger.tip"
    : > "$ledger"
    local prev_hash="0000000000000000000000000000000000000000000000000000000000000000"
    local i seq new_line new_sha
    for ((i=0; i<count; i++)); do
        seq=$i
        new_line=$(jq -nc \
            --arg ts "2026-05-07T00:00:0${i}Z" \
            --argjson cycle 1 \
            --arg role "scout" \
            --argjson exit_code 0 \
            --argjson entry_seq "$seq" \
            --arg prev_hash "$prev_hash" \
            '{ts: $ts, cycle: $cycle, role: $role, kind: "agent_subprocess",
              exit_code: $exit_code, entry_seq: $entry_seq, prev_hash: $prev_hash}')
        printf '%s\n' "$new_line" >> "$ledger"
        new_sha=$(printf '%s' "$new_line" | sha256_stdin)
        prev_hash="$new_sha"
    done
    # Write tip = last entry's SHA + seq
    printf '%s:%s\n' "$((count-1))" "$prev_hash" > "$tip"
}

# === Test 1: clean 5-entry chain → exit 0 ===================================
header "Test 1: clean 5-entry chain → exit 0"
out=$(mktemp -d -p "$SCRATCH" "t1.XXXX")
make_clean_chain "$out" 5
if bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet; then
    pass "clean chain → rc=0"
else
    fail_ "rc=$? (expected 0)"
fi

# === Test 2: mutated entry → chain break detected ==========================
header "Test 2: mutate entry 3 → chain break at entry 4"
out=$(mktemp -d -p "$SCRATCH" "t2.XXXX")
make_clean_chain "$out" 5
# Mutate entry index 2 (3rd line) — change its ts
sed -i.bak 's/"ts":"2026-05-07T00:00:02Z"/"ts":"2099-99-99T99:99:99Z"/' "$out/ledger.jsonl"
rm -f "$out/ledger.jsonl.bak"
bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet
rc=$?
if [ "$rc" = "1" ]; then
    pass "mutation → rc=1 (chain break)"
else
    fail_ "rc=$rc (expected 1)"
fi
# JSON output should identify break at entry_seq=3
json=$(bash "$SCRIPT" --ledger "$out/ledger.jsonl" --json 2>&1)
break_seq=$(echo "$json" | jq -r '.first_break_seq')
if [ "$break_seq" = "3" ]; then
    pass "first_break_seq=3 (entry whose prev_hash no longer matches)"
else
    fail_ "first_break_seq=$break_seq (expected 3); json: $json"
fi

# === Test 3: truncation → tip mismatch detected ============================
header "Test 3: remove last 2 lines → tip mismatch (rc=2)"
out=$(mktemp -d -p "$SCRATCH" "t3.XXXX")
make_clean_chain "$out" 5
# Remove last 2 lines
head -3 "$out/ledger.jsonl" > "$out/ledger.tmp" && mv "$out/ledger.tmp" "$out/ledger.jsonl"
bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet
rc=$?
if [ "$rc" = "2" ]; then
    pass "truncation → rc=2 (tip mismatch)"
else
    fail_ "rc=$rc (expected 2)"
fi

# === Test 4: forgery insertion → next entry's chain breaks =================
header "Test 4: insert fake entry between 2 and 3 → break detected"
out=$(mktemp -d -p "$SCRATCH" "t4.XXXX")
make_clean_chain "$out" 5
# Capture lines, insert a fake at position 3 (after 2nd entry)
head -2 "$out/ledger.jsonl" > "$out/ledger.tmp"
echo '{"ts":"2026-05-07T00:99:99Z","cycle":1,"role":"forge","kind":"agent_subprocess","exit_code":0,"entry_seq":99,"prev_hash":"deadbeef00000000000000000000000000000000000000000000000000000000"}' >> "$out/ledger.tmp"
tail -3 "$out/ledger.jsonl" >> "$out/ledger.tmp"
mv "$out/ledger.tmp" "$out/ledger.jsonl"
bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet
rc=$?
if [ "$rc" = "1" ]; then
    pass "forgery insertion → rc=1 (chain break)"
else
    fail_ "rc=$rc (expected 1)"
fi

# === Test 5: mixed pre-v8.37 + v8.37 entries → tolerated ===================
header "Test 5: pre-v8.37 + v8.37 mixed → soft-start boundary tolerated"
out=$(mktemp -d -p "$SCRATCH" "t5.XXXX")
mkdir -p "$out"
ledger="$out/ledger.jsonl"
# 3 pre-v8.37 entries (no prev_hash)
echo '{"ts":"2026-05-07T00:00:00Z","cycle":1,"role":"intent","kind":"agent_subprocess","exit_code":0}' > "$ledger"
echo '{"ts":"2026-05-07T00:00:01Z","cycle":1,"role":"scout","kind":"agent_subprocess","exit_code":0}' >> "$ledger"
echo '{"ts":"2026-05-07T00:00:02Z","cycle":1,"role":"builder","kind":"agent_subprocess","exit_code":0}' >> "$ledger"
# Compute SHA of last pre-v8.37 line
last_pre_line=$(tail -1 "$ledger")
last_pre_sha=$(printf '%s' "$last_pre_line" | sha256_stdin)
# Add 2 v8.37 entries chaining from there
new_line=$(jq -nc --argjson seq 3 --arg prev "$last_pre_sha" \
    '{ts:"2026-05-07T00:00:03Z",cycle:1,role:"auditor",kind:"agent_subprocess",exit_code:0,entry_seq:$seq,prev_hash:$prev}')
echo "$new_line" >> "$ledger"
last_sha=$(printf '%s' "$new_line" | sha256_stdin)
new_line=$(jq -nc --argjson seq 4 --arg prev "$last_sha" \
    '{ts:"2026-05-07T00:00:04Z",cycle:1,role:"orchestrator",kind:"agent_subprocess",exit_code:0,entry_seq:$seq,prev_hash:$prev}')
echo "$new_line" >> "$ledger"
last_sha=$(printf '%s' "$new_line" | sha256_stdin)
printf '%s:%s\n' "4" "$last_sha" > "$out/ledger.tip"
if bash "$SCRIPT" --ledger "$ledger" --quiet; then
    pass "mixed pre-v8.37/v8.37 → rc=0 (soft-start tolerated)"
else
    fail_ "rc=$? (expected 0); output: $(bash "$SCRIPT" --ledger "$ledger" 2>&1 | tail -5)"
fi

# === Test 6: tip-only corruption → rc=2 ====================================
header "Test 6: clean ledger but corrupt tip file → rc=2"
out=$(mktemp -d -p "$SCRATCH" "t6.XXXX")
make_clean_chain "$out" 3
# Corrupt tip
echo "99:0000000000000000000000000000000000000000000000000000000000000000" > "$out/ledger.tip"
bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet
rc=$?
if [ "$rc" = "2" ]; then
    pass "corrupted tip → rc=2"
else
    fail_ "rc=$rc (expected 2)"
fi

# === Test 7: first-entry zero-init ==========================================
header "Test 7: brand-new ledger first entry uses zero-init prev_hash"
out=$(mktemp -d -p "$SCRATCH" "t7.XXXX")
mkdir -p "$out"
make_clean_chain "$out" 1
# The first entry's prev_hash should be 64 zeros
first_prev=$(jq -r '.prev_hash' "$out/ledger.jsonl")
expected_zero="0000000000000000000000000000000000000000000000000000000000000000"
if [ "$first_prev" = "$expected_zero" ] && bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet; then
    pass "first entry has zero-init prev_hash; verifier accepts"
else
    fail_ "first_prev=$first_prev (expected zeros); verifier rc=$?"
fi

# === Test 8: duplicate prev_hash → rc=1 ====================================
header "Test 8: two entries share same prev_hash → fan-out race detected"
out=$(mktemp -d -p "$SCRATCH" "t8.XXXX")
make_clean_chain "$out" 3
# Append a 4th entry that REUSES entry 3's prev_hash (simulating a fan-out
# race). To do this without breaking the chain otherwise, both new entries
# would naturally have the same prev_hash if they raced.
last_line=$(tail -1 "$out/ledger.jsonl")
prev_hash_for_3rd=$(echo "$last_line" | jq -r '.prev_hash')
new_dup=$(jq -nc --argjson seq 3 --arg prev "$prev_hash_for_3rd" \
    '{ts:"2026-05-07T00:00:99Z",cycle:1,role:"forge",kind:"agent_subprocess",exit_code:0,entry_seq:$seq,prev_hash:$prev}')
echo "$new_dup" >> "$out/ledger.jsonl"
# Update tip to point at the new (dup) line
new_dup_sha=$(printf '%s' "$new_dup" | sha256_stdin)
printf '%s:%s\n' "3" "$new_dup_sha" > "$out/ledger.tip"
bash "$SCRIPT" --ledger "$out/ledger.jsonl" --quiet
rc=$?
# Should detect duplicate prev_hash → rc=1
if [ "$rc" = "1" ]; then
    pass "duplicate prev_hash → rc=1 (fan-out anomaly flagged)"
else
    fail_ "rc=$rc (expected 1); ledger:"
    cat "$out/ledger.jsonl"
fi

# === Test 9: --json schema is parseable ====================================
header "Test 9: --json output is valid JSON with documented fields"
out=$(mktemp -d -p "$SCRATCH" "t9.XXXX")
make_clean_chain "$out" 3
json=$(bash "$SCRIPT" --ledger "$out/ledger.jsonl" --json 2>&1)
if echo "$json" | jq -e 'has("total_entries") and has("chain_breaks") and has("tip_match") and has("status")' >/dev/null 2>&1; then
    pass "JSON has required fields"
else
    fail_ "JSON malformed or missing fields: $json"
fi

# === Test 10: empty ledger → rc=0 with status=empty ========================
header "Test 10: nonexistent ledger → rc=0 (nothing to verify)"
bash "$SCRIPT" --ledger "$SCRATCH/does-not-exist.jsonl" --quiet
rc=$?
if [ "$rc" = "0" ]; then
    pass "empty/missing ledger → rc=0"
else
    fail_ "rc=$rc (expected 0)"
fi

# === Summary ================================================================
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
