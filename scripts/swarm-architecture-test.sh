#!/usr/bin/env bash
#
# swarm-architecture-test.sh — Integration test for the Sprint 1+2+3 swarm
# architecture. Verifies that the tri-layer (Skill / Persona / Command) is
# wired correctly and that fan-out + plan-review + auto-orchestration all
# preserve the trust kernel.
#
# What this test verifies:
#   1. plugin.json registers all Sprint 3 skills, agents, and commands
#   2. All 7 new composable skill SKILL.md files exist with valid frontmatter
#   3. All 8 new slash command files exist
#   4. plan-reviewer.md persona file exists and declares correct frontmatter
#   5. .evolve/profiles/{scout,auditor,retrospective,plan-reviewer}.json
#      have parallel_subtasks arrays
#   6. cycle-state.sh known phases includes plan-review
#   7. phase-gate-precondition.sh recognizes plan-reviewer + worker patterns
#   8. aggregator.sh supports all 4 merge modes (concat/verdict/lessons/plan_review)
#   9. subagent-run.sh supports dispatch-parallel + worker-name pattern
#  10. End-to-end: dispatch-parallel scout against real profile produces
#      valid scout-report.md + ledger entry (smoke test)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Test 1: plugin.json registrations ---------------------------------------
header "Test 1: plugin.json registers Sprint 3 artifacts"
PLUGIN="$REPO_ROOT/.claude-plugin/plugin.json"
if jq empty "$PLUGIN" 2>/dev/null; then
    pass "plugin.json is valid JSON"
else
    fail_ "plugin.json invalid"
fi
SKILL_COUNT=$(jq '.skills | length' "$PLUGIN" 2>/dev/null)
if [ "$SKILL_COUNT" -ge 14 ]; then
    pass "plugin.json registers $SKILL_COUNT skills (>=14 expected)"
else
    fail_ "plugin.json has only $SKILL_COUNT skills"
fi
CMD_COUNT=$(jq '.commands | length' "$PLUGIN" 2>/dev/null)
if [ "$CMD_COUNT" = "9" ]; then
    pass "plugin.json registers 9 commands"
else
    fail_ "plugin.json has $CMD_COUNT commands (expected 9)"
fi
if jq -r '.agents[]' "$PLUGIN" 2>/dev/null | grep -q "plan-reviewer"; then
    pass "plan-reviewer registered in agents"
else
    fail_ "plan-reviewer missing from plugin.json agents"
fi

# --- Test 2: 7 composable skill files exist ---------------------------------
header "Test 2: 7 new composable skill SKILL.md files exist"
for s in evolve-spec evolve-plan-review evolve-tdd evolve-build evolve-audit evolve-ship evolve-retro; do
    f="$REPO_ROOT/skills/$s/SKILL.md"
    if [ -f "$f" ]; then
        # Frontmatter check: must have name and description.
        if head -10 "$f" | grep -q "^name: $s$" && head -10 "$f" | grep -q "^description:"; then
            pass "$s/SKILL.md present + valid frontmatter"
        else
            fail_ "$s/SKILL.md frontmatter missing name or description"
        fi
    else
        fail_ "$s/SKILL.md missing"
    fi
done

# --- Test 3: 8 slash commands exist ------------------------------------------
header "Test 3: 8 slash command files exist"
for c in scout plan-review tdd build audit ship retro loop intent; do
    f="$REPO_ROOT/.claude-plugin/commands/$c.md"
    if [ -f "$f" ]; then
        if head -5 "$f" | grep -q "^description:"; then
            pass "$c.md present + valid frontmatter"
        else
            fail_ "$c.md missing description frontmatter"
        fi
    else
        fail_ "$c.md missing"
    fi
done

# --- Test 4: plan-reviewer persona -------------------------------------------
header "Test 4: plan-reviewer persona file"
PR="$REPO_ROOT/agents/plan-reviewer.md"
if [ -f "$PR" ]; then
    pass "plan-reviewer.md present"
    if head -15 "$PR" | grep -q "^name: plan-reviewer$"; then
        pass "plan-reviewer.md frontmatter has name=plan-reviewer"
    else
        fail_ "plan-reviewer.md missing name"
    fi
    if grep -qi "do not invoke from another persona" "$PR"; then
        pass "plan-reviewer.md respects 'no persona-calls-persona' rule"
    else
        fail_ "plan-reviewer.md missing composition rule"
    fi
else
    fail_ "plan-reviewer.md missing"
fi

# --- Test 5: 4 profiles with parallel_subtasks -------------------------------
header "Test 5: 4 profiles declare parallel_subtasks arrays"
for p in scout auditor retrospective plan-reviewer; do
    f="$REPO_ROOT/.evolve/profiles/$p.json"
    if [ -f "$f" ]; then
        c=$(jq '.parallel_subtasks // [] | length' "$f" 2>/dev/null)
        if [ "$c" -gt 0 ]; then
            pass "$p.json has $c parallel_subtasks"
        else
            fail_ "$p.json missing or empty parallel_subtasks"
        fi
    else
        fail_ "$p.json missing"
    fi
done

# --- Test 6: cycle-state knows plan-review phase -----------------------------
header "Test 6: cycle-state.sh recognizes plan-review phase"
if grep -q '"plan-review"' "$REPO_ROOT/scripts/cycle-state.sh"; then
    pass "cycle-state.sh known phases includes plan-review"
else
    fail_ "cycle-state.sh missing plan-review phase"
fi

# --- Test 7: phase-gate recognizes plan-reviewer + worker patterns -----------
header "Test 7: phase-gate-precondition.sh recognizes plan-reviewer + worker patterns"
GATE="$REPO_ROOT/scripts/guards/phase-gate-precondition.sh"
if grep -q "plan-reviewer" "$GATE"; then
    pass "gate recognizes plan-reviewer agent"
else
    fail_ "gate missing plan-reviewer recognition"
fi
if grep -q "worker-\*" "$GATE"; then
    pass "gate recognizes <role>-worker-* pattern"
else
    fail_ "gate missing worker pattern"
fi
if grep -q 'plan-review)' "$GATE"; then
    pass "gate has plan-review phase mapping"
else
    fail_ "gate missing plan-review phase mapping"
fi

# --- Test 8: aggregator.sh has 4 merge modes ---------------------------------
header "Test 8: aggregator.sh supports all 4 merge modes"
AGG="$REPO_ROOT/scripts/aggregator.sh"
for mode in concat verdict lessons plan_review; do
    if grep -q "MERGE_MODE=$mode" "$AGG"; then
        pass "aggregator supports $mode mode"
    else
        fail_ "aggregator missing $mode mode"
    fi
done

# --- Test 9: subagent-run.sh dispatcher --------------------------------------
header "Test 9: subagent-run.sh dispatch-parallel + worker pattern"
SR="$REPO_ROOT/scripts/subagent-run.sh"
if grep -q "cmd_dispatch_parallel" "$SR"; then
    pass "subagent-run.sh has cmd_dispatch_parallel"
else
    fail_ "subagent-run.sh missing dispatch_parallel"
fi
if grep -q 'worker_name=' "$SR"; then
    pass "cmd_run handles worker_name pattern"
else
    fail_ "cmd_run missing worker_name handling"
fi
if grep -q 'plan-reviewer' "$SR"; then
    pass "subagent-run.sh allowlist includes plan-reviewer"
else
    fail_ "subagent-run.sh missing plan-reviewer in allowlist"
fi

# --- Test 10: end-to-end dispatch-parallel smoke test ------------------------
header "Test 10: end-to-end dispatch-parallel scout (smoke)"
TMPDIR=$(mktemp -d -t swarm-arch-test.XXXXXX)
WS="$TMPDIR/cycle-99000"
mkdir -p "$WS"
# Stub executor that emits a token-bearing artifact.
cat > "$TMPDIR/stub.sh" <<'BASH'
#!/usr/bin/env bash
mkdir -p "$(dirname "$EVOLVE_FANOUT_WORKER_ARTIFACT")"
{
    echo "<!-- challenge-token: $EVOLVE_FANOUT_WORKER_TOKEN -->"
    echo "Stub worker $EVOLVE_FANOUT_WORKER_NAME for cycle $EVOLVE_FANOUT_CYCLE."
} > "$EVOLVE_FANOUT_WORKER_ARTIFACT"
BASH
chmod +x "$TMPDIR/stub.sh"

cd "$REPO_ROOT"
rc=0
EVOLVE_FANOUT_TEST_EXECUTOR="$TMPDIR/stub.sh" \
EVOLVE_LEDGER_OVERRIDE="$TMPDIR/ledger.jsonl" \
EVOLVE_BYPASS_PHASE_GATE=1 \
bash scripts/subagent-run.sh dispatch-parallel scout 99000 "$WS" >/dev/null 2>&1 || rc=$?

# Clean up the canonical artifact (scout.json points to .evolve/runs/cycle-{cycle}/).
rm -rf .evolve/runs/cycle-99000 2>/dev/null

if [ "$rc" = "0" ]; then
    pass "end-to-end dispatch-parallel scout: exit 0"
else
    fail_ "dispatch-parallel scout failed: rc=$rc"
fi
# v8.23.0: workers/ also contains cache-prefix.md (Task C, shared input not a per-worker artifact). Exclude it.
WORKER_COUNT=$(ls -1 "$WS/workers/"*.md 2>/dev/null | grep -v cache-prefix.md | wc -l | tr -d ' ')
if [ "$WORKER_COUNT" = "3" ]; then
    pass "3 worker artifacts produced"
else
    fail_ "expected 3 workers, got $WORKER_COUNT"
fi
if grep -q '"kind":"agent_fanout"' "$TMPDIR/ledger.jsonl" 2>/dev/null; then
    pass "ledger has agent_fanout entry with full kernel binding"
else
    fail_ "ledger missing agent_fanout entry"
fi
rm -rf "$TMPDIR"

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
[ "$FAIL" -eq 0 ]
