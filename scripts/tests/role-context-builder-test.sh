#!/usr/bin/env bash
#
# role-context-builder-test.sh — v8.56.0 Layer B per-role context filter.
# Verifies each agent role gets only its declared inputs (and not the
# kitchen sink), and that EVOLVE_PROMPT_MAX_TOKENS guard fires correctly.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
HELPER="$REPO_ROOT/scripts/lifecycle/role-context-builder.sh"
SCRATCH=$(mktemp -d)

PASS=0
FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Build a fake project root with realistic cycle artifacts.
make_repo() {
    local root="$SCRATCH/repo-$RANDOM"
    mkdir -p "$root/.evolve/runs/cycle-1"
    cat > "$root/.evolve/state.json" <<EOF
{
  "instinctSummary": [
    {"id":"inst-L001","pattern":"shell-edge-case","confidence":0.8,"type":"failure-lesson","errorCategory":"reasoning"},
    {"id":"inst-L002","pattern":"missing-validation","confidence":0.7,"type":"failure-lesson","errorCategory":"context"}
  ],
  "carryoverTodos": [
    {"id":"todo-1","action":"Add tests for X","priority":"high","evidence_pointer":"audit.md#D1","defer_count":0,"first_seen_cycle":1,"last_seen_cycle":1}
  ],
  "failedApproaches": []
}
EOF
    # Cycle artifacts
    echo "<!-- intent --> Goal: build feature X with acceptance Y" > "$root/.evolve/runs/cycle-1/intent.md"
    echo "## Scout report — backlog: [task1, task2, task3]" > "$root/.evolve/runs/cycle-1/scout-report.md"
    echo "## Build report — diff applied to file Z" > "$root/.evolve/runs/cycle-1/build-report.md"
    echo "## Audit report — Verdict: PASS" > "$root/.evolve/runs/cycle-1/audit-report.md"
    echo "## Retrospective — root cause was X" > "$root/.evolve/runs/cycle-1/retrospective-report.md"
    echo "$root"
}

# --- Test 1: Scout receives todos+instincts+intent, NOT build/audit ---------
header "Test 1: Scout context excludes build-report and audit-report"
ROOT=$(make_repo)
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" scout 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
if echo "$OUT" | grep -q "carryoverTodos"; then pass "scout includes carryoverTodos"; else fail "scout missing carryoverTodos"; fi
if echo "$OUT" | grep -q "instinctSummary"; then pass "scout includes instinctSummary"; else fail "scout missing instinctSummary"; fi
if echo "$OUT" | grep -q "intent.md"; then pass "scout references intent.md"; else fail "scout missing intent reference"; fi
if echo "$OUT" | grep -q "build-report"; then fail "scout LEAKED build-report"; else pass "scout excludes build-report"; fi
if echo "$OUT" | grep -q "audit-report"; then fail "scout LEAKED audit-report"; else pass "scout excludes audit-report"; fi

# --- Test 2: Builder gets scout backlog + intent, NOT retrospective ---------
header "Test 2: Builder context excludes retrospective theory + full instincts"
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" builder 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
if echo "$OUT" | grep -q "scout-report"; then pass "builder includes scout-report"; else fail "builder missing scout-report"; fi
if echo "$OUT" | grep -q "intent.md"; then pass "builder includes intent.md"; else fail "builder missing intent.md"; fi
if echo "$OUT" | grep -q "retrospective-report"; then fail "builder LEAKED retrospective-report"; else pass "builder excludes retrospective"; fi

# --- Test 3: Auditor gets build-report + intent acceptance, NOT retrospective ---
header "Test 3: Auditor context excludes retrospective + raw scout research"
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" auditor 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
if echo "$OUT" | grep -q "build-report"; then pass "auditor includes build-report"; else fail "auditor missing build-report"; fi
if echo "$OUT" | grep -q "intent.md"; then pass "auditor includes intent.md"; else fail "auditor missing intent.md"; fi
if echo "$OUT" | grep -q "retrospective-report"; then fail "auditor LEAKED retrospective"; else pass "auditor excludes retrospective"; fi

# --- Test 4: Retrospective gets ALL artifacts (it's the synthesizer) --------
header "Test 4: Retrospective sees every phase artifact"
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" retrospective 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
for art in scout-report build-report audit-report intent.md; do
    if echo "$OUT" | grep -q "$art"; then pass "retrospective references $art"; else fail "retrospective missing $art"; fi
done

# --- Test 5: Plan-reviewer gets scout-report + carryoverTodos, NOT build ----
header "Test 5: Plan-reviewer excludes build artifacts"
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" plan_review 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
if echo "$OUT" | grep -q "scout-report"; then pass "plan_review includes scout-report"; else fail "plan_review missing scout-report"; fi
if echo "$OUT" | grep -q "carryoverTodos"; then pass "plan_review includes carryoverTodos"; else fail "plan_review missing carryoverTodos"; fi
if echo "$OUT" | grep -q "build-report"; then fail "plan_review LEAKED build-report"; else pass "plan_review excludes build-report"; fi

# --- Test 6: TDD-engineer gets scout backlog + intent, NOT retrospective ---
header "Test 6: TDD-engineer context lean"
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" tdd 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
if echo "$OUT" | grep -q "scout-report"; then pass "tdd includes scout-report"; else fail "tdd missing scout-report"; fi
if echo "$OUT" | grep -q "retrospective-report"; then fail "tdd LEAKED retrospective"; else pass "tdd excludes retrospective"; fi

# --- Test 7: Triage gets backlog + carryoverTodos, NOT build/audit/retro ---
header "Test 7: Triage (Layer C) gets backlog + carryoverTodos only"
OUT=$(EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" triage 1 "$ROOT/.evolve/runs/cycle-1" 2>/dev/null)
if echo "$OUT" | grep -q "scout-report"; then pass "triage includes scout-report (backlog)"; else fail "triage missing scout-report"; fi
if echo "$OUT" | grep -q "carryoverTodos"; then pass "triage includes carryoverTodos"; else fail "triage missing carryoverTodos"; fi
if echo "$OUT" | grep -q "build-report"; then fail "triage LEAKED build-report"; else pass "triage excludes build-report"; fi
if echo "$OUT" | grep -q "retrospective-report"; then fail "triage LEAKED retrospective"; else pass "triage excludes retrospective"; fi

# --- Test 8: Unknown role exits 2 with a clear error ------------------------
header "Test 8: unknown role exits 2"
set +e
EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" notarole 1 "$ROOT/.evolve/runs/cycle-1" >/dev/null 2>&1
RC=$?
set -e
[ "$RC" = "2" ] && pass "exit 2 on unknown role" || fail "expected 2, got $RC"

# --- Test 9: token cap guard — over-cap emits WARN ---------------------------
header "Test 9: EVOLVE_PROMPT_MAX_TOKENS guard"
# Create a giant scout-report so the builder context exceeds cap (deterministic, no SIGPIPE)
awk 'BEGIN{for(i=0;i<5000;i++) print "x x x x x x x x x x"}' > "$ROOT/.evolve/runs/cycle-1/scout-report.md"
WARN_OUT=$(EVOLVE_PROMPT_MAX_TOKENS=100 EVOLVE_PROJECT_ROOT="$ROOT" bash "$HELPER" builder 1 "$ROOT/.evolve/runs/cycle-1" 2>&1 >/dev/null)
if echo "$WARN_OUT" | grep -qi "exceeds.*max.*tokens\|over.*cap"; then
    pass "WARN emitted when context exceeds cap"
else
    fail "no WARN about token cap; got: $WARN_OUT"
fi

# --- Summary ----------------------------------------------------------------
rm -rf "$SCRATCH"
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
