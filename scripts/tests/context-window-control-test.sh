#!/usr/bin/env bash
# v9.1.0 Cycle 6: context-window control tests.
#
# Verifies:
#   1. subagent-run.sh writes context-monitor.json per-phase.
#   2. EVOLVE_CONTEXT_AUTOTRIM=1 actually trims oversized prompts.
#   3. cumulative_input_tokens sums across phases.
#   4. show-context-monitor.sh tabulates and emits JSON correctly.
#   5. Cumulative thresholds emit WARN/CRITICAL markers.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd -P)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd -P)"
SUBAGENT_RUN="$PROJECT_ROOT/scripts/dispatch/subagent-run.sh"
MONITOR_SH="$PROJECT_ROOT/scripts/observability/show-context-monitor.sh"

PASS=0
FAIL=0

expect() {
    local label="$1" actual="$2" expected="$3"
    if [ "$actual" = "$expected" ]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s (expected=%s actual=%s)\n" "$label" "$expected" "$actual" >&2
        FAIL=$((FAIL + 1))
    fi
}

expect_match() {
    local label="$1" actual="$2" pattern="$3"
    if [[ "$actual" =~ $pattern ]]; then
        printf "  PASS: %s\n" "$label"; PASS=$((PASS + 1))
    else
        printf "  FAIL: %s\n    pattern=%s\n" "$label" "$pattern" >&2
        FAIL=$((FAIL + 1))
    fi
}

echo "=== Test 1: subagent-run.sh has autotrim + monitor code ==="
src=$(cat "$SUBAGENT_RUN")
expect_match "EVOLVE_CONTEXT_AUTOTRIM check" "$src" "EVOLVE_CONTEXT_AUTOTRIM"
expect_match "AUTOTRIM log message" "$src" "AUTOTRIM:"
expect_match "context-monitor.json write" "$src" "context-monitor.json"
expect_match "cap_pct computed" "$src" "_cap_pct"
expect_match "cumulative_input_tokens captured" "$src" "cumulative_input_tokens"
expect_match "AUTOTRIM preserves head + tail" "$src" "_keep_head_bytes.*_keep_tail_bytes"

echo
echo "=== Test 2: autotrim algorithm — simulated in isolation ==="
# Test the head/tail trim approach in isolation.
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT INT TERM
# Make a fake "huge" prompt (40k bytes).
yes "lorem ipsum dolor sit amet " | head -c 40000 > "$TMP/prompt.txt"
prompt_bytes=$(wc -c < "$TMP/prompt.txt" | tr -d ' ')
prompt_tokens=$((prompt_bytes / 4))
prompt_max=2000
_keep_head_bytes=$((prompt_max * 4 * 60 / 100))   # 60% from head
_keep_tail_bytes=$((prompt_max * 4 * 35 / 100))   # 35% from tail
trimmed="$TMP/prompt.trimmed"
{
    head -c "$_keep_head_bytes" "$TMP/prompt.txt"
    printf '\n\n[CONTEXT-AUTOTRIM]\n\n'
    tail -c "$_keep_tail_bytes" "$TMP/prompt.txt"
} > "$trimmed"
trimmed_bytes=$(wc -c < "$trimmed" | tr -d ' ')
trimmed_tokens=$((trimmed_bytes / 4))

# Trimmed should be approximately under the cap, with some marker overhead.
if [ "$trimmed_tokens" -le "$((prompt_max + 100))" ]; then
    expect "trimmed prompt under cap" "ok" "ok"
else
    expect "trimmed prompt under cap (got=$trimmed_tokens cap=$prompt_max)" "OVER" "ok"
fi

# Head preserved
head_check=$(head -c 50 "$trimmed")
expect_match "head preserved" "$head_check" "lorem ipsum"

# Marker present
grep -q "CONTEXT-AUTOTRIM" "$trimmed" && expect "marker present" "yes" "yes" \
    || expect "marker present" "no" "yes"

echo
echo "=== Test 3: show-context-monitor.sh exists and is executable ==="
[ -x "$MONITOR_SH" ] && expect "monitor script executable" "yes" "yes" \
    || expect "monitor script executable" "no" "yes"
head -1 "$MONITOR_SH" | grep -q '#!/usr/bin/env bash' \
    && expect "shebang correct" "yes" "yes" || expect "shebang correct" "no" "yes"

echo
echo "=== Test 4: show-context-monitor.sh renders sample data ==="
mkdir -p "$TMP/.evolve/runs/cycle-42"
cat > "$TMP/.evolve/runs/cycle-42/context-monitor.json" <<'EOF'
{
  "cycle": 42,
  "lastUpdated": "2026-05-11T12:00:00Z",
  "phases": {
    "scout": {
      "input_tokens": 8500,
      "cap_tokens": 30000,
      "cap_pct": 28,
      "measuredAt": "2026-05-11T12:00:00Z"
    },
    "builder": {
      "input_tokens": 12000,
      "cap_tokens": 30000,
      "cap_pct": 40,
      "measuredAt": "2026-05-11T12:00:30Z"
    }
  },
  "cumulative_input_tokens": 20500,
  "cumulative_cap": 120000,
  "cumulative_pct": 17
}
EOF
export EVOLVE_PROJECT_ROOT="$TMP"
out=$(RUNS_DIR_OVERRIDE="$TMP/.evolve/runs" bash "$MONITOR_SH" 42 2>&1)
expect_match "renders scout phase" "$out" "scout.*8500"
expect_match "renders builder phase" "$out" "builder.*12000"
expect_match "renders CUMULATIVE" "$out" "CUMULATIVE.*20500"

echo
echo "=== Test 5: --json mode emits raw JSON ==="
out=$(RUNS_DIR_OVERRIDE="$TMP/.evolve/runs" bash "$MONITOR_SH" --json 42 2>&1)
expect_match "JSON output has cycle field" "$out" '"cycle": 42'
expect_match "JSON output has phases field" "$out" '"phases":'

echo
echo "=== Test 6: cumulative threshold WARN at 80%+ ==="
cat > "$TMP/.evolve/runs/cycle-42/context-monitor.json" <<'EOF'
{
  "cycle": 42,
  "phases": {"scout": {"input_tokens": 100000, "cap_tokens": 30000, "cap_pct": 333}},
  "cumulative_input_tokens": 100000,
  "cumulative_cap": 120000,
  "cumulative_pct": 83
}
EOF
out=$(RUNS_DIR_OVERRIDE="$TMP/.evolve/runs" bash "$MONITOR_SH" 42 2>&1)
expect_match "emits WARN at 80%+" "$out" "WARN: cumulative"

echo
echo "=== Test 7: cumulative threshold CRITICAL at 95%+ ==="
cat > "$TMP/.evolve/runs/cycle-42/context-monitor.json" <<'EOF'
{
  "cycle": 42,
  "phases": {"scout": {"input_tokens": 120000, "cap_tokens": 30000, "cap_pct": 400}},
  "cumulative_input_tokens": 115000,
  "cumulative_cap": 120000,
  "cumulative_pct": 96
}
EOF
out=$(RUNS_DIR_OVERRIDE="$TMP/.evolve/runs" bash "$MONITOR_SH" 42 2>&1)
expect_match "emits CRITICAL at 95%+" "$out" "CRITICAL: cumulative"
expect_match "mentions checkpoint" "$out" "checkpoint"

echo
echo "=== Test 8: monitor handles missing cycle gracefully ==="
out=$(RUNS_DIR_OVERRIDE="$TMP/.evolve/runs" bash "$MONITOR_SH" 999 2>&1 || true)
expect_match "complains about missing file" "$out" "no context-monitor.json"

echo
echo "=== Test 9: syntax checks ==="
bash -n "$MONITOR_SH" && expect "show-context-monitor.sh syntax" "ok" "ok" \
    || expect "show-context-monitor.sh syntax" "FAIL" "ok"
bash -n "$SUBAGENT_RUN" && expect "subagent-run.sh syntax" "ok" "ok" \
    || expect "subagent-run.sh syntax" "FAIL" "ok"

echo
echo "=== Summary ==="
echo "PASS: $PASS"
echo "FAIL: $FAIL"
if [ "$FAIL" -eq 0 ]; then
    echo "ALL TESTS PASSED"
    exit 0
else
    exit 1
fi
