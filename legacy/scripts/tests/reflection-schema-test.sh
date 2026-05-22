#!/usr/bin/env bash
#
# reflection-schema-test.sh — Validate the Reflection Journal YAML schema.
#
# Tests the agents/reflection-journal-schema.md spec by parsing synthetic
# YAML fixtures through the aggregator's grep-based extractor.
# The aggregator must:
#   - Accept well-formed reflections (all required fields, valid enums)
#   - Skip reflections with reflection_confidence below the threshold
#   - Tolerate optional fields being absent
#
# Schema reference: agents/reflection-journal-schema.md
#
# Tests:
#   T1. Valid YAML with all required fields → aggregator counts it
#   T2. Low-confidence YAML (0.2) → aggregator skips its categories
#   T3. YAML with phase_smooth=true and empty slowdowns → no category emitted
#   T4. Multiple slowdown categories → each appears in rollup
#   T5. friction_received_from → pair appears in rollup
#
# Usage: bash scripts/tests/reflection-schema-test.sh
# Exit:  0 if all assertions pass; non-zero otherwise.
#
# bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
AGGREGATOR="$REPO_ROOT/scripts/observability/aggregate-reflections.sh"
SCHEMA_DOC="$REPO_ROOT/agents/reflection-journal-schema.md"
TEMPLATES="$REPO_ROOT/agents/agent-templates.md"

if [ ! -x "$AGGREGATOR" ]; then
    echo "FAIL: aggregator not executable at $AGGREGATOR"
    exit 1
fi

# Schema may live in either the standalone file or agent-templates.md
if [ ! -f "$SCHEMA_DOC" ] && ! grep -q "Reflection Journal Schema" "$TEMPLATES" 2>/dev/null; then
    echo "FAIL: schema spec not found (looked in $SCHEMA_DOC and $TEMPLATES)"
    exit 1
fi

FIXTURE_DIR="${TMPDIR:-/tmp}/reflection-schema-test.$$"
mkdir -p "$FIXTURE_DIR/runs"
trap 'rm -rf "$FIXTURE_DIR"' EXIT

pass=0
fail=0

write_fixture() {
    local cyc="$1"
    local phase="$2"
    local body="$3"
    local d="$FIXTURE_DIR/runs/cycle-$cyc"
    mkdir -p "$d"
    printf '%s\n' "$body" > "$d/${phase}-reflection.yaml"
}

assert_in_json() {
    local label="$1"
    local needle="$2"
    local json="$3"
    if printf '%s' "$json" | grep -q "$needle"; then
        echo "PASS: $label"
        pass=$((pass+1))
    else
        echo "FAIL: $label — expected '$needle' in output:"
        printf '  %s\n' "$json"
        fail=$((fail+1))
    fi
}

assert_not_in_json() {
    local label="$1"
    local needle="$2"
    local json="$3"
    if printf '%s' "$json" | grep -q "$needle"; then
        echo "FAIL: $label — unexpected '$needle' in output:"
        printf '  %s\n' "$json"
        fail=$((fail+1))
    else
        echo "PASS: $label"
        pass=$((pass+1))
    fi
}

# T1. Valid YAML with required fields — should be counted as 1 reflection
write_fixture 1001 scout '
schema_version: 1
cycle: 1001
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: research-quota
    evidence: "scout-stdout.log:line=42"
    severity: medium
friction_received_from: []
suggested_improvements:
  - action: "Bump kb-search quota to 30"
    target_file: ".evolve/profiles/scout.json"
    evidence_pointer: "scout-stdout.log:line=42"
    priority: medium
reflection_confidence: 0.8
phase_tracker_refs:
  latency_ms: 1000
  cost_usd: 0.5
  turns: 10
'

out1=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --format=json 2>/dev/null)
assert_in_json "T1: valid YAML counted (reflections_found:1)" '"reflections_found":1' "$out1"
assert_in_json "T1: research-quota category in rollup" '"category":"research-quota"' "$out1"

# T2. Low-confidence YAML — its categories should be SKIPPED in aggregation
rm -rf "$FIXTURE_DIR/runs"
mkdir -p "$FIXTURE_DIR/runs"
write_fixture 2001 scout '
schema_version: 1
cycle: 2001
phase: scout
agent: evolve-scout
phase_smooth: false
slowdowns:
  - category: tool-error
    evidence: "log:line=1"
    severity: low
reflection_confidence: 0.2
phase_tracker_refs:
  latency_ms: 100
  cost_usd: 0.1
  turns: 1
'
out2=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --format=json 2>/dev/null)
assert_not_in_json "T2: low-confidence YAML category excluded from rollup" '"category":"tool-error"' "$out2"

# T3. phase_smooth=true with no slowdowns — counted but contributes no category
rm -rf "$FIXTURE_DIR/runs"
mkdir -p "$FIXTURE_DIR/runs"
write_fixture 3001 audit '
schema_version: 1
cycle: 3001
phase: audit
agent: evolve-auditor
phase_smooth: true
slowdowns: []
friction_received_from: []
suggested_improvements:
  - action: "Add cross-reference for plan adherence"
    target_file: "agents/evolve-auditor.md"
    evidence_pointer: "build-report.md#known-gap"
    priority: low
reflection_confidence: 0.9
phase_tracker_refs:
  latency_ms: 500
  cost_usd: 0.3
  turns: 5
'
out3=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --format=json 2>/dev/null)
assert_in_json "T3: smooth phase reflection counted" '"reflections_found":1' "$out3"
assert_in_json "T3: smooth phase yields empty slowdown_categories" '"slowdown_categories":\[\]' "$out3"

# T4. Multiple slowdown categories — each appears in rollup
rm -rf "$FIXTURE_DIR/runs"
mkdir -p "$FIXTURE_DIR/runs"
write_fixture 4001 build '
schema_version: 1
cycle: 4001
phase: build
agent: evolve-builder
phase_smooth: false
slowdowns:
  - category: tool-batching
    evidence: "builder-stdout.log:line=99"
    severity: high
  - category: profile-restriction
    evidence: "builder-stderr.log:line=12"
    severity: medium
reflection_confidence: 0.75
phase_tracker_refs:
  latency_ms: 60000
  cost_usd: 1.2
  turns: 30
'
out4=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --format=json 2>/dev/null)
assert_in_json "T4: tool-batching category emitted" '"category":"tool-batching"' "$out4"
assert_in_json "T4: profile-restriction category emitted" '"category":"profile-restriction"' "$out4"

# T5. friction_received_from — pair shows in rollup
rm -rf "$FIXTURE_DIR/runs"
mkdir -p "$FIXTURE_DIR/runs"
write_fixture 5001 tdd '
schema_version: 1
cycle: 5001
phase: tdd
agent: evolve-tdd-engineer
phase_smooth: false
slowdowns:
  - category: ambiguous-input
    evidence: "scout-report.md#ac-2"
    severity: medium
friction_received_from:
  - upstream_phase: scout
    issue: "AC#2 was untestable"
    evidence: "scout-report.md#ac-2"
reflection_confidence: 0.7
phase_tracker_refs:
  latency_ms: 30000
  cost_usd: 0.6
  turns: 20
'
out5=$("$AGGREGATOR" --runs-dir "$FIXTURE_DIR/runs" --format=json 2>/dev/null)
assert_in_json "T5: scout->tdd friction pair in rollup" '"upstream":"scout","downstream":"tdd"' "$out5"

echo ""
echo "reflection-schema-test: $pass PASS, $fail FAIL"
if [ "$fail" -gt 0 ]; then
    exit 1
fi
exit 0
