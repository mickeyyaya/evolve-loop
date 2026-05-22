#!/usr/bin/env bash
#
# aggregate-reflections-test.sh — Cross-cycle rollup integration test.
#
# Synthesizes 5 cycle dirs with deterministic reflection YAMLs and asserts
# that aggregate-reflections.sh produces the expected category counts,
# friction pairs, and recurring suggestions.
#
# Scenario:
#   - 5 cycles (numbered 9001-9005), each containing scout/build/audit reflections
#   - research-quota appears in scout reflections for cycles 9001, 9002, 9003, 9005 → 4/5
#   - tool-error appears in build reflections for cycles 9003, 9005 → 2/5
#   - intent→scout friction appears in scout reflections for 9001, 9002, 9004 → 3 occurrences
#   - "Bump kb-search quota" suggestion appears in scout reflections for 9001, 9003, 9005 → 3 cycles
#
# Tests:
#   T1. window=5 picks up all 5 cycles
#   T2. research-quota shows 4/5 in human rollup
#   T3. tool-error shows 2/5 in JSON
#   T4. intent→scout friction shows 3 occurrences
#   T5. Bump-kb-search suggestion shows in top recurring
#   T6. --phase scout filter restricts to scout reflections only
#   T7. --window 2 picks up only most recent 2 cycles
#
# Usage: bash legacy/scripts/tests/aggregate-reflections-test.sh
# Exit:  0 if all PASS; non-zero otherwise.
#
# bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
AGGREGATOR="$REPO_ROOT/legacy/scripts/observability/aggregate-reflections.sh"

if [ ! -x "$AGGREGATOR" ]; then
    echo "FAIL: aggregator not executable at $AGGREGATOR"
    exit 1
fi

FIXTURE_DIR="${TMPDIR:-/tmp}/aggregate-reflections-test.$$"
mkdir -p "$FIXTURE_DIR/runs"
trap 'rm -rf "$FIXTURE_DIR"' EXIT

pass=0
fail=0

assert_pattern() {
    local label="$1"
    local needle="$2"
    local haystack="$3"
    if printf '%s' "$haystack" | grep -q "$needle"; then
        echo "PASS: $label"
        pass=$((pass+1))
    else
        echo "FAIL: $label — expected '$needle'"
        printf '  output: %s\n' "$haystack" | head -c 500
        printf '\n'
        fail=$((fail+1))
    fi
}

assert_not_pattern() {
    local label="$1"
    local needle="$2"
    local haystack="$3"
    if printf '%s' "$haystack" | grep -q "$needle"; then
        echo "FAIL: $label — unexpected '$needle'"
        fail=$((fail+1))
    else
        echo "PASS: $label"
        pass=$((pass+1))
    fi
}

write_yaml() {
    local cyc="$1"
    local phase="$2"
    local body="$3"
    local d="$FIXTURE_DIR/runs/cycle-$cyc"
    mkdir -p "$d"
    printf '%s\n' "$body" > "$d/${phase}-reflection.yaml"
}

# ---- Cycle 9001: research-quota + intent→scout friction + bump-kb suggestion ----
write_yaml 9001 scout '
schema_version: 1
cycle: 9001
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=10"
    severity: medium
friction_received_from:
  - upstream_phase: intent
    issue: "challenged_premises empty"
    evidence: "intent.md#challenged"
suggested_improvements:
  - action: "Bump kb-search quota to 30"
    target_file: ".evolve/profiles/scout.json"
    evidence_pointer: "scout-stdout.log:line=10"
    priority: medium
reflection_confidence: 0.8
phase_tracker_refs:
  latency_ms: 1000
  cost_usd: 0.5
  turns: 10
'
write_yaml 9001 build '
schema_version: 1
cycle: 9001
phase: build
agent: evolve-builder
phase_smooth: true
slowdowns: []
friction_received_from: []
suggested_improvements:
  - action: "Continue current discipline"
    target_file: "agents/evolve-builder.md"
    evidence_pointer: "build-report.md"
    priority: low
reflection_confidence: 0.9
phase_tracker_refs:
  latency_ms: 30000
  cost_usd: 1.0
  turns: 20
'

# ---- Cycle 9002: research-quota + intent→scout friction ----
write_yaml 9002 scout '
schema_version: 1
cycle: 9002
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=20"
    severity: medium
friction_received_from:
  - upstream_phase: intent
    issue: "scope ambiguous"
    evidence: "intent.md#scope"
suggested_improvements:
  - action: "Tighten intent scope rubric"
    target_file: "agents/evolve-intent.md"
    evidence_pointer: "intent.md#scope"
    priority: medium
reflection_confidence: 0.7
phase_tracker_refs:
  latency_ms: 1200
  cost_usd: 0.6
  turns: 12
'

# ---- Cycle 9003: research-quota + tool-error + bump-kb suggestion ----
write_yaml 9003 scout '
schema_version: 1
cycle: 9003
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=30"
    severity: high
suggested_improvements:
  - action: "Bump kb-search quota to 30"
    target_file: ".evolve/profiles/scout.json"
    evidence_pointer: "scout-stdout.log:line=30"
    priority: high
reflection_confidence: 0.85
phase_tracker_refs:
  latency_ms: 1500
  cost_usd: 0.7
  turns: 15
'
write_yaml 9003 build '
schema_version: 1
cycle: 9003
phase: build
agent: evolve-builder
phase_smooth: false
slowdowns:
  - category: tool-error
    evidence: "builder-stderr.log:line=99"
    severity: medium
suggested_improvements:
  - action: "Add retry logic to subprocess wrapper"
    target_file: "legacy/scripts/dispatch/subagent-run.sh"
    evidence_pointer: "builder-stderr.log:line=99"
    priority: medium
reflection_confidence: 0.75
phase_tracker_refs:
  latency_ms: 60000
  cost_usd: 1.5
  turns: 30
'

# ---- Cycle 9004: intent→scout friction only ----
write_yaml 9004 scout '
schema_version: 1
cycle: 9004
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: ambiguous-input
    evidence: "intent.md#goal"
    severity: low
friction_received_from:
  - upstream_phase: intent
    issue: "goal phrasing vague"
    evidence: "intent.md#goal"
suggested_improvements:
  - action: "Add concrete-example check to intent"
    target_file: "agents/evolve-intent.md"
    evidence_pointer: "intent.md#goal"
    priority: low
reflection_confidence: 0.6
phase_tracker_refs:
  latency_ms: 900
  cost_usd: 0.4
  turns: 8
'

# ---- Cycle 9005: research-quota + tool-error + bump-kb suggestion ----
write_yaml 9005 scout '
schema_version: 1
cycle: 9005
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=50"
    severity: high
suggested_improvements:
  - action: "Bump kb-search quota to 30"
    target_file: ".evolve/profiles/scout.json"
    evidence_pointer: "scout-stdout.log:line=50"
    priority: high
reflection_confidence: 0.9
phase_tracker_refs:
  latency_ms: 1100
  cost_usd: 0.55
  turns: 11
'
write_yaml 9005 build '
schema_version: 1
cycle: 9005
phase: build
agent: evolve-builder
phase_smooth: false
slowdowns:
  - category: tool-error
    evidence: "builder-stderr.log:line=200"
    severity: high
suggested_improvements:
  - action: "Increase subprocess timeout to 600s"
    target_file: "legacy/scripts/dispatch/subagent-run.sh"
    evidence_pointer: "builder-stderr.log:line=200"
    priority: high
reflection_confidence: 0.8
phase_tracker_refs:
  latency_ms: 70000
  cost_usd: 1.7
  turns: 35
'

# ---- T1. window=5 picks up all 5 cycles ----
out_h=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --window 5 2>/dev/null)
assert_pattern "T1: all 5 cycles scanned" "last 5 cycle" "$out_h"

# ---- T2. research-quota shows 4/5 in human rollup ----
assert_pattern "T2: research-quota 4/5" "research-quota.*4/5" "$out_h"

# ---- T3. tool-error shows 2/5 in JSON ----
out_j=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --window 5 --format=json 2>/dev/null)
assert_pattern "T3: tool-error 2 cycles (JSON)" '"category":"tool-error","cycles":2' "$out_j"

# ---- T4. intent→scout friction appears with 3 occurrences ----
assert_pattern "T4: intent->scout friction 3 occurrences (JSON)" '"upstream":"intent","downstream":"scout","occurrences":3' "$out_j"

# ---- T5. Bump-kb-search suggestion appears in top recurring (3 cycles: 9001, 9003, 9005) ----
assert_pattern "T5: bump-kb-search top suggestion (3 cycles)" '"action":"Bump kb-search quota to 30","cycle_count":3' "$out_j"

# ---- T6. --phase scout filter restricts to scout reflections only ----
out_scout=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --window 5 --phase scout --format=json 2>/dev/null)
assert_not_pattern "T6: --phase scout excludes tool-error (build only)" '"category":"tool-error"' "$out_scout"
assert_pattern "T6: --phase scout still has research-quota" '"category":"research-quota"' "$out_scout"

# ---- T7. --window 2 picks only most recent 2 cycles (9004, 9005) ----
out_w2=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --window 2 --format=json 2>/dev/null)
assert_pattern "T7: window=2 scanned 2 cycles" '"cycles_scanned":2' "$out_w2"
# 9004 has ambiguous-input, 9005 has research-quota+tool-error; no research-quota 4/5
assert_pattern "T7: research-quota 1 cycle in window=2" '"category":"research-quota","cycles":1' "$out_w2"

echo ""
echo "aggregate-reflections-test: $pass PASS, $fail FAIL"
if [ "$fail" -gt 0 ]; then
    exit 1
fi
exit 0
