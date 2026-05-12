#!/usr/bin/env bash
#
# anchor-extract-test.sh — tests for extract_anchor() and
# emit_artifact_anchored() in role-context-builder.sh.
#
# v8.63.0 Campaign C Cycles C1+C2.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
RCB="$REPO_ROOT/scripts/lifecycle/role-context-builder.sh"

PASS=0
FAIL=0
TESTS_TOTAL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; TESTS_TOTAL=$((TESTS_TOTAL + 1)); }

# --- Setup: temporary fixture files -----------------------------------------
TMP_ROOT=$(mktemp -d)
trap 'rm -rf "$TMP_ROOT"' EXIT

# Fixture A: file with multiple anchors.
cat > "$TMP_ROOT/multi-anchor.md" <<'EOF'
# Header

Some preamble.

<!-- ANCHOR:proposed_tasks -->
## Selected Tasks

### Task 1: Foo
- Acceptance: Bar
- File: foo.sh

### Task 2: Baz
- Acceptance: Qux

<!-- ANCHOR:acceptance_criteria -->
## Acceptance Criteria Summary
- task-1: Foo passes
- task-2: Baz validates

<!-- ANCHOR:gap_analysis -->
## Gap Analysis
| Gap | Priority |
|-----|----------|
| One | High     |

End of file.
EOF

# Fixture B: file with NO anchor markers (legacy / pre-v8.63 artifacts).
cat > "$TMP_ROOT/no-anchor.md" <<'EOF'
# Legacy report

## Selected Tasks

### Task 1: Old format
- Acceptance: criterion

## Other section
content.
EOF

# Re-define inline (bash function isn't exported across subshells; sourcing
# role-context-builder.sh exits on missing args so we replicate the helper).
_extract_test() {
    awk -v anchor="$2" '
        $0 ~ "^[[:space:]]*<!--[[:space:]]+ANCHOR:[[:space:]]*" anchor "[[:space:]]*-->" {
            in_anchor=1; next
        }
        in_anchor && $0 ~ "^[[:space:]]*<!--[[:space:]]+ANCHOR:" { exit }
        in_anchor { print }
    ' "$1"
}

# --- Test 1: extract_anchor pulls the named region --------------------------
header "Test 1: extract_anchor pulls proposed_tasks region"
out=$(_extract_test "$TMP_ROOT/multi-anchor.md" "proposed_tasks")
if echo "$out" | grep -q "Task 1: Foo" && echo "$out" | grep -q "Task 2: Baz" \
   && ! echo "$out" | grep -q "Acceptance Criteria Summary" \
   && ! echo "$out" | grep -q "Gap Analysis"; then
    pass "proposed_tasks region extracted, neighbors excluded"
else
    fail_ "extracted region wrong; got: $(echo "$out" | head -3)"
fi

# --- Test 2: extract_anchor scopes to second anchor -------------------------
header "Test 2: extract_anchor pulls only acceptance_criteria region"
out=$(_extract_test "$TMP_ROOT/multi-anchor.md" "acceptance_criteria")
if echo "$out" | grep -q "task-1: Foo passes" \
   && ! echo "$out" | grep -q "Task 1: Foo" \
   && ! echo "$out" | grep -q "Gap Analysis"; then
    pass "acceptance_criteria region extracted, neighbors excluded"
else
    fail_ "acceptance_criteria region wrong; got: $out"
fi

# --- Test 3: extract_anchor returns empty for missing anchor ----------------
header "Test 3: extract_anchor returns empty for unknown anchor name"
out=$(_extract_test "$TMP_ROOT/multi-anchor.md" "nonexistent_anchor")
if [ -z "$out" ]; then
    pass "missing anchor returns empty (caller falls back)"
else
    fail_ "expected empty, got: $out"
fi

# --- Test 4: extract_anchor returns empty for legacy file (no anchors) -----
header "Test 4: extract_anchor on file without any anchor markers"
out=$(_extract_test "$TMP_ROOT/no-anchor.md" "proposed_tasks")
if [ -z "$out" ]; then
    pass "no-anchor file returns empty (graceful fallback signal)"
else
    fail_ "expected empty, got: $out"
fi

# --- Test 5: emit_artifact_anchored falls back when anchor missing ---------
# This requires invoking role-context-builder.sh in a way that exercises
# emit_artifact_anchored. We use the actual script with a fixture workspace.
header "Test 5: emit_artifact_anchored falls back to full file"
mkdir -p "$TMP_ROOT/.evolve/runs/cycle-99"
cp "$TMP_ROOT/no-anchor.md" "$TMP_ROOT/.evolve/runs/cycle-99/scout-report.md"
# Minimal intent.md so role-context-builder doesn't error on missing pieces.
cat > "$TMP_ROOT/.evolve/runs/cycle-99/intent.md" <<'INTENT'
goal: |
  Test
acceptance_checks:
  - check: "ok"
    how_verified: "manual"
INTENT
echo '{"failedApproaches":[],"instinctSummary":[],"carryoverTodos":[]}' > "$TMP_ROOT/.evolve/state.json"
out=$(EVOLVE_PROJECT_ROOT="$TMP_ROOT" EVOLVE_ANCHOR_EXTRACT=1 bash "$RCB" auditor 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null)
# auditor under EVOLVE_ANCHOR_EXTRACT=1 attempts emit_artifact_anchored on
# build-report's diff_summary anchor. build-report.md is missing here, so
# it should silently skip. scout-report.md exists but has no anchors, so
# emit_artifact_anchored falls back to full-file emission.
if echo "$out" | grep -q "Selected Tasks" && echo "$out" | grep -q "Other section"; then
    pass "auditor anchor mode falls back to full scout-report when anchors missing"
else
    fail_ "fallback didn't emit full file content; out: $(echo "$out" | head -10)"
fi

# --- Test 6: emit_artifact_anchored extracts when anchor present ------------
header "Test 6: emit_artifact_anchored extracts named region when present"
# Replace scout-report with the multi-anchor fixture.
cp "$TMP_ROOT/multi-anchor.md" "$TMP_ROOT/.evolve/runs/cycle-99/scout-report.md"
out=$(EVOLVE_PROJECT_ROOT="$TMP_ROOT" EVOLVE_ANCHOR_EXTRACT=1 bash "$RCB" triage 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null)
# triage under EVOLVE_ANCHOR_EXTRACT=1 reads scout-report's proposed_tasks
# anchor. Should contain Task 1/Task 2 but NOT Gap Analysis.
if echo "$out" | grep -q "Task 1: Foo" && echo "$out" | grep -q "Task 2: Baz" \
   && ! echo "$out" | grep -q "Gap Analysis"; then
    pass "triage anchor mode extracts proposed_tasks only"
else
    fail_ "anchor extraction failed; out: $(echo "$out" | head -10)"
fi

# --- Test 7: legacy mode (EVOLVE_ANCHOR_EXTRACT=0) reads full file ---------
header "Test 7: legacy mode (EVOLVE_ANCHOR_EXTRACT=0) reads full scout-report"
out=$(EVOLVE_ANCHOR_EXTRACT=0 EVOLVE_PROJECT_ROOT="$TMP_ROOT" bash "$RCB" triage 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null)
# Should contain everything in the multi-anchor fixture.
if echo "$out" | grep -q "Task 1: Foo" && echo "$out" | grep -q "Gap Analysis"; then
    pass "legacy mode reads full scout-report (no extraction)"
else
    fail_ "legacy mode lost content; out: $(echo "$out" | head -10)"
fi

# --- Test 8: provenance comment in extracted output -------------------------
header "Test 8: extracted output carries provenance comment"
out=$(EVOLVE_PROJECT_ROOT="$TMP_ROOT" EVOLVE_ANCHOR_EXTRACT=1 bash "$RCB" triage 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null)
if echo "$out" | grep -q "extracted from .* :: proposed_tasks"; then
    pass "provenance comment present"
else
    fail_ "no provenance comment in output"
fi

# --- Test 9: persona templates have ANCHOR markers --------------------------
header "Test 9: persona templates have ANCHOR markers"
markers_found=0
# evolve-scout.md anchors moved to evolve-scout-reference.md (Layer-3 split, cycle 24)
for f in agents/evolve-scout-reference.md agents/evolve-builder.md agents/evolve-auditor.md agents/evolve-retrospective.md; do
    if grep -q "<!-- ANCHOR:" "$REPO_ROOT/$f"; then
        markers_found=$((markers_found + 1))
    fi
done
if [ "$markers_found" = "4" ]; then
    pass "all 4 personas (scout/builder/auditor/retrospective) have ANCHOR markers"
else
    fail_ "expected 4 personas with ANCHOR markers, got $markers_found"
fi

# --- Test 10: specific expected anchors per role ----------------------------
header "Test 10: each persona has its expected anchor names"
declare -i ok=0
# evolve-scout.md anchors moved to evolve-scout-reference.md (Layer-3 split, cycle 24)
grep -q "ANCHOR:proposed_tasks"       "$REPO_ROOT/agents/evolve-scout-reference.md" && ok=$((ok+1))
grep -q "ANCHOR:acceptance_criteria"  "$REPO_ROOT/agents/evolve-scout-reference.md" && ok=$((ok+1))
grep -q "ANCHOR:gap_analysis"         "$REPO_ROOT/agents/evolve-scout-reference.md" && ok=$((ok+1))
grep -q "ANCHOR:diff_summary"         "$REPO_ROOT/agents/evolve-builder.md"       && ok=$((ok+1))
grep -q "ANCHOR:test_results"         "$REPO_ROOT/agents/evolve-builder.md"       && ok=$((ok+1))
grep -q "ANCHOR:verdict"              "$REPO_ROOT/agents/evolve-auditor.md"       && ok=$((ok+1))
grep -q "ANCHOR:defects"              "$REPO_ROOT/agents/evolve-auditor.md"       && ok=$((ok+1))
grep -q "ANCHOR:lessons"              "$REPO_ROOT/agents/evolve-retrospective.md" && ok=$((ok+1))
if [ "$ok" = "8" ]; then
    pass "all 8 expected anchor names present in templates"
else
    fail_ "expected 8 anchors, got $ok"
fi

# --- Test 11 (Cycle C3): profile JSONs declare context_anchors --------------
header "Test 11 (Cycle C3): expected profiles declare context_anchors"
declare -i ok2=0
for p in auditor triage plan-reviewer tdd-engineer; do
    if jq -e '(.context_anchors // []) | length > 0' "$REPO_ROOT/.evolve/profiles/${p}.json" >/dev/null 2>&1; then
        ok2=$((ok2+1))
    else
        echo "  MISSING: $p.json:context_anchors"
    fi
done
if [ "$ok2" = "4" ]; then
    pass "all 4 profiles (auditor/triage/plan-reviewer/tdd-engineer) declare context_anchors"
else
    fail_ "expected 4 profiles, got $ok2"
fi

# --- Test 12 (Cycle C3): context_anchors entries use file:anchor format -----
header "Test 12 (Cycle C3): context_anchors entries are file:anchor strings"
malformed=0
for p in auditor triage plan-reviewer tdd-engineer; do
    bad=$(jq -r '(.context_anchors // []) | map(select(type != "string" or (contains(":") | not))) | length' "$REPO_ROOT/.evolve/profiles/${p}.json" 2>/dev/null || echo "?")
    if [ "$bad" != "0" ]; then
        malformed=$((malformed+1))
        echo "  MALFORMED: $p.json has $bad non-string-or-no-colon entries"
    fi
done
if [ "$malformed" = "0" ]; then
    pass "all context_anchors entries are well-formed (file:anchor)"
else
    fail_ "$malformed profile(s) have malformed entries"
fi

# --- Test 13 (Cycle C3): role-context-builder reads profile context_anchors -
header "Test 13 (Cycle C3): triage anchor mode reads profile.context_anchors"
# Use the multi-anchor fixture from Test setup; create a minimal profile with
# context_anchors so we can verify the helper reads it (vs hardcoded fallback).
TMP_PROFILE_DIR="$TMP_ROOT/.evolve/profiles"
mkdir -p "$TMP_PROFILE_DIR"
cat > "$TMP_PROFILE_DIR/triage.json" <<'PROFILE'
{"context_anchors": ["scout-report.md:gap_analysis"]}
PROFILE
cp "$TMP_ROOT/multi-anchor.md" "$TMP_ROOT/.evolve/runs/cycle-99/scout-report.md"
# Use EVOLVE_PROFILES_DIR_OVERRIDE — the canonical test-seam (matches
# subagent-run.sh:49). EVOLVE_PLUGIN_ROOT alone gets overridden by
# resolve-roots.sh based on script location.
out=$(EVOLVE_PROJECT_ROOT="$TMP_ROOT" EVOLVE_PROFILES_DIR_OVERRIDE="$TMP_PROFILE_DIR" EVOLVE_ANCHOR_EXTRACT=1 bash "$RCB" triage 99 "$TMP_ROOT/.evolve/runs/cycle-99" 2>/dev/null)
# With profile.context_anchors=["scout-report.md:gap_analysis"], output should
# contain Gap Analysis content (not Selected Tasks, which is what the hardcoded
# fallback would produce).
if echo "$out" | grep -q "Gap Analysis" && echo "$out" | grep -q "anchored: gap_analysis"; then
    pass "triage anchor mode reads profile.context_anchors (gap_analysis, not hardcoded proposed_tasks)"
else
    fail_ "profile.context_anchors not honored; out: $(echo "$out" | head -10)"
fi

# --- Summary -----------------------------------------------------------------
echo
echo "=========================================="
echo "  Total tests: $TESTS_TOTAL"
echo "  Passed:      $PASS"
echo "  Failed:      $FAIL"
echo "=========================================="

[ "$FAIL" = "0" ]
