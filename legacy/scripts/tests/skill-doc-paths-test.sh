#!/usr/bin/env bash
#
# skill-doc-paths-test.sh — Guard against pre-v8.47 stale `scripts/<name>.sh`
# references in skills/, .agents/skills/, .claude-plugin/commands/, and docs/.
#
# Why this test exists:
# v8.47.0 reorganized scripts/ into thematic subdirs (dispatch/, lifecycle/,
# failure/, observability/, verification/, utility/). The patcher excluded
# skills/ and .agents/skills/ because those are operator-facing surface,
# expected to be hand-curated. Result: SKILL.md's STRICT MODE one-liner
# referenced the OLD path, breaking `/evolve-loop` slash command invocation
# with rc=127 (find returned no match).
#
# This test grep's for any `scripts/<bare-name>.sh` reference in the
# operator-facing surfaces and FAILS if it finds one. Prevents the regression
# from coming back the next time someone moves a script.
#
# Bash 3.2 compatible.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# Names that moved into subdirs in v8.47.0. Hardcoded — if scripts move again,
# this list must be updated alongside the move.
MOVED_NAMES="
evolve-loop-dispatch.sh run-cycle.sh subagent-run.sh aggregator.sh
fanout-dispatch.sh detect-cli.sh detect-nested-claude.sh
preflight-environment.sh cycle-state.sh ship.sh phase-gate.sh
resolve-roots.sh failure-adapter.sh failure-classifications.sh
record-failure-to-state.sh merge-lesson-into-state.sh
index-investigations.sh state-prune.sh show-cycle-cost.sh
verify-ledger-chain.sh cycle-health-check.sh token-profiler.sh
evolve-status.py verify-eval.sh mutate-eval.sh eval-quality-check.sh
postedit-validate.sh doc-lint.sh complexity-check.sh context-budget.sh
calibrate.sh diff-complexity.sh probe-tool.sh setup-skill-inventory.sh
team-context.sh code-review-simplify.sh run-all-regression-tests.sh
release.sh
"

# Surfaces to audit. These are operator-facing or activation-path documents
# where a stale `scripts/<bare-name>.sh` reference will produce an actual
# breakage at runtime.
SURFACES=(
    "skills"
    ".agents/skills"
    ".claude-plugin/commands"
)

# === Test 1: no stale refs in skills/, .agents/skills/, .claude-plugin/commands/
header "Test 1: no stale 'scripts/<bare-name>.sh' references in operator-facing surfaces"
total_stale=0
stale_lines=""
for surf in "${SURFACES[@]}"; do
    surf_path="$REPO_ROOT/$surf"
    [ -d "$surf_path" ] || continue
    for name in $MOVED_NAMES; do
        matches=$(grep -rEn "(^|[^/])scripts/${name}([^/]|$)" "$surf_path" --include="*.md" --include="*.sh" 2>/dev/null || true)
        if [ -n "$matches" ]; then
            stale_lines="$stale_lines
$matches"
            total_stale=$((total_stale + $(echo "$matches" | wc -l | tr -d ' ')))
        fi
    done
done
if [ "$total_stale" = "0" ]; then
    pass "no stale references found across ${#SURFACES[@]} surfaces"
else
    fail_ "$total_stale stale references found:"
    echo "$stale_lines" | head -10
fi

# === Test 2: SKILL.md's find expression points at scripts/dispatch/ ===========
header "Test 2: SKILL.md STRICT MODE find expression uses scripts/dispatch/"
SKILL="$REPO_ROOT/skills/evolve-loop/SKILL.md"
if [ ! -f "$SKILL" ]; then
    fail_ "SKILL.md not found at $SKILL"
elif grep -q "marketplaces/evolve-loop/scripts/dispatch/evolve-loop-dispatch.sh" "$SKILL" \
   && grep -q "cache/evolve-loop/evolve-loop/.*/scripts/dispatch/evolve-loop-dispatch.sh" "$SKILL"; then
    pass "find expression matches both marketplace and cache install paths under scripts/dispatch/"
else
    fail_ "SKILL.md find expression missing scripts/dispatch/ — operators will hit rc=127 on /evolve-loop"
fi

# === Test 3: ship-gate.sh allowlists scripts/lifecycle/ship.sh =================
header "Test 3: ship-gate.sh allowlists scripts/lifecycle/ship.sh"
SHIP_GATE="$REPO_ROOT/scripts/guards/ship-gate.sh"
if [ -f "$SHIP_GATE" ] && grep -q "scripts/lifecycle/ship.sh" "$SHIP_GATE"; then
    pass "ship-gate.sh references scripts/lifecycle/ship.sh"
else
    fail_ "ship-gate.sh missing or doesn't reference scripts/lifecycle/ship.sh"
fi

# === Test 4: evolve-loop-dispatch.sh PATH includes all 6 v8.47 subdirs ========
header "Test 4: evolve-loop-dispatch.sh exports PATH with all v8.47 subdirs"
DISPATCH="$REPO_ROOT/scripts/dispatch/evolve-loop-dispatch.sh"
all_present=1
for sub in dispatch lifecycle failure observability verification utility; do
    if ! grep -q "scripts/$sub" "$DISPATCH" 2>/dev/null; then
        all_present=0
        echo "    missing PATH entry: scripts/$sub" >&2
    fi
done
if [ "$all_present" = "1" ]; then
    pass "all 6 v8.47 subdirs present in dispatcher PATH"
else
    fail_ "dispatcher PATH missing one or more v8.47 subdirs"
fi

# === Test 5: v8.51.0 — no stale 'tier-3-stub' or 'exits 99' references for codex
# Catches the v8.50→v8.51 doc-language regression: the audit found stale
# 'tier-3-stub' descriptions in README and AGENTS after v8.51 promoted Codex
# to tier-1 hybrid. Guard against reintroduction.
header "Test 5: v8.51.0 — no 'tier-3-stub' or codex 'exits 99' in operator-facing docs"
total_stale=0
for f in README.md AGENTS.md CLAUDE.md; do
    full="$REPO_ROOT/$f"
    [ -f "$full" ] || continue
    if grep -qE "tier-3-stub|codex\.sh.*exits 99|codex.*tier-3" "$full"; then
        total_stale=$((total_stale + 1))
        echo "    STALE in $f:" >&2
        grep -n -E "tier-3-stub|codex\.sh.*exits 99|codex.*tier-3" "$full" | head -2 >&2
    fi
done
if [ "$total_stale" = "0" ]; then
    pass "no stale tier-3-stub language in README/AGENTS/CLAUDE"
else
    fail_ "$total_stale files still reference stale codex tier"
fi

# === Test 6: v8.51.0 — codex-runtime.md exists (parity with claude/gemini) =====
header "Test 6: v8.51.0 — codex-runtime.md present (multi-CLI doc symmetry)"
RUNTIME_DIR="$REPO_ROOT/skills/evolve-loop/reference"
all_present=1
for cli in claude gemini codex; do
    if [ ! -f "$RUNTIME_DIR/${cli}-runtime.md" ]; then
        echo "    missing: ${cli}-runtime.md" >&2
        all_present=0
    fi
done
if [ "$all_present" = "1" ]; then
    pass "all 3 <cli>-runtime.md files present"
else
    fail_ "asymmetric runtime docs"
fi

# === Summary ====================================================================
echo
echo "==========================================="
echo "  Total: 6 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
