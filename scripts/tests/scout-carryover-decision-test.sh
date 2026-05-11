#!/usr/bin/env bash
#
# scout-carryover-decision-test.sh — v8.57.0 Layer S smoke tests.
# Verifies that:
#   - phase-gate gate_research_to_discover requires '## Carryover Decisions'
#     in scout-report.md when state.json:carryoverTodos[] is non-empty.
#   - When carryoverTodos[] is empty, the section is NOT required (no gate
#     escalation; backward-compatible).
#   - Scout persona prompt mentions carryoverTodos consultation.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRATCH=$(mktemp -d)
PG="$REPO_ROOT/scripts/lifecycle/phase-gate.sh"
SCOUT_PERSONA="$REPO_ROOT/agents/evolve-scout.md"

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Build a fake project with a workspace + state.json + ledger
make_repo() {
    local has_carryover="$1"  # "true" or "false"
    local root="$SCRATCH/repo-$RANDOM"
    mkdir -p "$root/.evolve/runs/cycle-1" "$root/.evolve/evals"
    if [ "$has_carryover" = "true" ]; then
        cat > "$root/.evolve/state.json" <<EOF
{
  "instinctSummary": [],
  "carryoverTodos": [
    {"id":"todo-A","action":"Add validation","priority":"high","evidence_pointer":"audit.md#D1","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1,"cycles_unpicked":0}
  ],
  "failedApproaches": []
}
EOF
    else
        cat > "$root/.evolve/state.json" <<EOF
{
  "instinctSummary": [],
  "carryoverTodos": [],
  "failedApproaches": []
}
EOF
    fi
    : > "$root/.evolve/ledger.jsonl"
    # Append a real agent_subprocess ledger entry so check_subagent_ledger_match
    # can find it (an empty/missing entry trips set -euo pipefail via grep-no-match).
    # The SHA must match the scout-report we'll write — fill it in after.
    echo '{"role":"scout","cycle":1,"ts":"2026-05-10T00:00:00Z","kind":"agent_subprocess","exit_code":0,"artifact_path":"'"$root/.evolve/runs/cycle-1/scout-report.md"'","artifact_sha256":"PLACEHOLDER"}' > "$root/.evolve/ledger.jsonl"
    # Stub eval needs:
    #  1. A file-under-test that mutate-eval.sh can mutate
    #  2. An eval that actually executes the function (behavior-based, not grep)
    # Otherwise mutate-eval exits 2 (no applicable mutations OR tautological)
    # and set -e in phase-gate.sh aborts before our carryover check runs.
    mkdir -p "$root/src"
    cat > "$root/src/widget.js" <<JS
function widget() {
  if (1 === 1) {
    return "ok";
  }
  return "fail";
}
module.exports = { widget };
JS
    cat > "$root/.evolve/evals/stub.md" <<EOF
# Eval: stub
## Code Graders
- \`[code]\` \`node -e 'const{widget}=require("$root/src/widget.js");if(widget()!=="ok")process.exit(1)'\`
## Thresholds
- All checks: pass@1 = 1.0
EOF
    echo "$root"
}

# After writing scout-report, recompute SHA and rewrite the ledger entry.
# The fixture's check_subagent_ledger_match check requires an exact SHA match.
update_ledger_sha() {
    local root="$1" ws="$2"
    local sha
    if command -v sha256sum >/dev/null 2>&1; then
        sha=$(sha256sum "$ws/scout-report.md" | awk '{print $1}')
    else
        sha=$(shasum -a 256 "$ws/scout-report.md" | awk '{print $1}')
    fi
    echo '{"role":"scout","cycle":1,"ts":"2026-05-10T00:00:00Z","kind":"agent_subprocess","exit_code":0,"artifact_path":"'"$ws/scout-report.md"'","artifact_sha256":"'"$sha"'"}' > "$root/.evolve/ledger.jsonl"
}

# Build a scout-report.md that may or may not have ## Carryover Decisions.
# Verbose enough (>=50 words) to pass check_artifact_substance.
write_scout_report() {
    local ws="$1" with_section="$2"
    cat > "$ws/scout-report.md" <<'EOF'
<!-- challenge: test -->
# Cycle 1 Scout Report

## Discovery Summary
- Scan mode: full
- Files analyzed: 12 source files plus 4 configuration files in src/ and config/
- Research performed against latest patterns and conventions
- Instincts applied: zero relevant entries this cycle

## Key Findings
- Finding A — input validation gap on user-provided JSON in src/x.ts:42 needs schema check
- Finding B — module exports drift in src/y.ts:18 reference deleted helper function from src/z.ts

## Hypotheses
| # | Hypothesis | Evidence | Confidence | Source |
| 1 | Adding zod schema fixes Finding A | src/x.ts:42 raw JSON.parse | 0.85 | code |

## Selected Tasks

### Task 1: add-validation
- Slug: add-validation
- Type: stability
- Complexity: S
- Files to modify: src/x.ts, src/types.ts
- Acceptance Criteria: zod schema rejects malformed input
- Eval: written to evals/add-validation.md
EOF
    if [ "$with_section" = "true" ]; then
        cat >> "$ws/scout-report.md" <<'EOF'

## Carryover Decisions
- todo-A: include, reason: aligns with current cycle goal — adding validation
EOF
    fi
}

# --- Test 1: persona references carryoverTodos consultation ----------------
header "Test 1: agents/evolve-scout.md instructs carryoverTodos consultation"
if grep -qi "carryoverTodos" "$SCOUT_PERSONA"; then
    pass "scout persona references carryoverTodos"
else
    fail "scout persona does not mention carryoverTodos"
fi
if grep -qi "Carryover Decisions" "$SCOUT_PERSONA"; then
    pass "scout persona prescribes ## Carryover Decisions section"
else
    fail "scout persona does not prescribe Carryover Decisions section"
fi

# --- Test 2: gate ALLOWS scout-report missing the section when no carryovers
header "Test 2: gate does not require section when carryoverTodos[] empty"
ROOT=$(make_repo false)
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report "$WS" false  # no Carryover Decisions section
update_ledger_sha "$ROOT" "$WS"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" discover-to-build 1 "$WS" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" = "0" ] && pass "gate ALLOWS missing section when no carryoverTodos" || \
    fail "gate REJECTED (rc=$RC) — expected ALLOW for empty carryoverTodos"

# --- Test 3: gate BLOCKS missing section when carryoverTodos non-empty -----
header "Test 3: gate BLOCKS scout-report missing section when carryoverTodos non-empty"
ROOT=$(make_repo true)
WS="$ROOT/.evolve/runs/cycle-1"
write_scout_report "$WS" false  # no Carryover Decisions section
update_ledger_sha "$ROOT" "$WS"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" discover-to-build 1 "$WS" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" != "0" ] && pass "gate DENIES missing section when carryoverTodos exist" || \
    fail "gate ALLOWED missing section despite non-empty carryoverTodos (rc=$RC)"

# --- Test 4: gate ALLOWS scout-report with the section ---------------------
header "Test 4: gate ALLOWS scout-report containing ## Carryover Decisions"
write_scout_report "$WS" true  # with section
update_ledger_sha "$ROOT" "$WS"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$PG" discover-to-build 1 "$WS" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" = "0" ] && pass "gate ALLOWS scout-report with Carryover Decisions section" || \
    fail "gate REJECTED valid scout-report (rc=$RC)"

# --- Test 5 (v9.0.3): scout persona has Turn budget section ----------------
header "Test 5 (v9.0.3): scout persona has Turn budget section"
if grep -q "^## Turn budget" "$SCOUT_PERSONA" \
   && grep -qE "(8.{1,3}12 turns|Target.*8|Maximum.{1,4}15)" "$SCOUT_PERSONA"; then
    pass "Turn budget section + 8-12 target / 15 max present in persona"
else
    fail "Turn budget section or turn-count target missing from scout persona"
fi

# --- Test 6 (v9.0.3): scout.json max_turns tightened to 15 or less --------
header "Test 6 (v9.0.3): scout max_turns tightened to 15 or less"
SCOUT_PROFILE="$REPO_ROOT/.evolve/profiles/scout.json"
if command -v jq >/dev/null 2>&1; then
    mt=$(jq -r '.max_turns' "$SCOUT_PROFILE")
    if [ "$mt" -le 15 ]; then
        pass "scout max_turns=$mt (≤ 15)"
    else
        fail "scout max_turns=$mt exceeds v9.0.3 target of 15"
    fi
fi

# --- Test 7 (v9.0.3): scout persona instructs main flow to skip web -------
# Web tools remain in profile for fan-out research sub-scout, but main-flow
# scout is instructed NOT to use them. Verify the persona language is present.
header "Test 7 (v9.0.3): scout persona defers web research to Phase 1"
if grep -qE "Skip web research in the main flow|WebSearch.*fan-out|main.path scout does NOT" "$SCOUT_PERSONA"; then
    pass "persona defers web research to Phase 1 / fan-out research sub-scout"
else
    fail "persona missing v9.0.3 web-research deferral guidance"
fi

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
