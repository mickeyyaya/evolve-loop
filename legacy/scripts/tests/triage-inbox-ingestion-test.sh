#!/usr/bin/env bash
# triage-inbox-ingestion-test.sh — Inbox ingestion schema + structure tests (v9.5.0+).
# Tests inbox file format written by inject-task.sh and the reconcile-compatible
# schema transformation contract (Layer-3 reference: agents/evolve-triage-reference.md).

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
CLI="$REPO_ROOT/scripts/utility/inject-task.sh"
SCRATCH=$(mktemp -d)

PASS=0; FAIL=0
pass() { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

cleanup() { rm -rf "$SCRATCH"; }
trap cleanup EXIT

make_project() {
    local root="$SCRATCH/proj-$RANDOM"
    mkdir -p "$root/.evolve"
    printf '{"carryoverTodos":[],"instinctSummary":[],"failedApproaches":[]}\n' \
        > "$root/.evolve/state.json"
    echo "$root"
}

# Transform inbox JSON to reconcile-compatible schema (mirrors Triage Step 0 logic).
transform_to_reconcile() {
    local inbox_json="$1"
    local cycle="${2:-1}"
    echo "$inbox_json" | jq -c \
        --argjson cycle "$cycle" \
        '{
          id: .id,
          action: .action,
          priority: .priority,
          weight: (if .weight == null then 0.5 else .weight end),
          evidence_pointer: .evidence_pointer,
          defer_count: 0,
          cycles_unpicked: 0,
          first_seen_cycle: $cycle,
          last_seen_cycle: $cycle,
          _inbox_source: {
            operator_note: .operator_note,
            injected_at: .injected_at,
            injected_by: .injected_by
          }
        }'
}

# --- Test 1: inbox file has all required fields ------------------------------
header "Test 1: inbox file schema — required fields present"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --action "schema check" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json | head -1)
for field in id action priority injected_at injected_by; do
    val=$(jq -r ".$field" "$f" 2>/dev/null)
    [ -n "$val" ] && [ "$val" != "null" ] && pass "$field present" || fail "$field missing or null"
done

# --- Test 2: evidence_pointer auto-synthesized when absent -------------------
header "Test 2: evidence_pointer auto-synthesized"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority MEDIUM --action "no evidence" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json | head -1)
ep=$(jq -r '.evidence_pointer' "$f")
echo "$ep" | grep -q "^inbox-injection://" && \
    pass "evidence_pointer auto-synthesized with inbox-injection:// prefix" || \
    fail "expected inbox-injection:// prefix, got '$ep'"

# --- Test 3: reconcile schema — all required fields filled -------------------
header "Test 3: transform to reconcile-compatible schema"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --weight 0.9 --action "reconcile test" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json | head -1)
raw=$(cat "$f")
reconciled=$(transform_to_reconcile "$raw" 5)
for field in id action priority weight evidence_pointer defer_count cycles_unpicked first_seen_cycle last_seen_cycle; do
    val=$(echo "$reconciled" | jq -r ".$field")
    [ "$val" != "null" ] && pass "reconcile.$field set: $val" || fail "reconcile.$field is null"
done
dc=$(echo "$reconciled" | jq -r '.defer_count')
cu=$(echo "$reconciled" | jq -r '.cycles_unpicked')
fsc=$(echo "$reconciled" | jq -r '.first_seen_cycle')
[ "$dc" = "0" ]  && pass "defer_count=0"        || fail "defer_count should be 0, got $dc"
[ "$cu" = "0" ]  && pass "cycles_unpicked=0"     || fail "cycles_unpicked should be 0, got $cu"
[ "$fsc" = "5" ] && pass "first_seen_cycle=5"    || fail "expected first_seen_cycle=5, got $fsc"

# --- Test 4: weight defaults to 0.5 in reconcile schema ----------------------
header "Test 4: absent weight → 0.5 in reconcile schema"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority LOW --action "no weight" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json | head -1)
raw=$(cat "$f")
w_inbox=$(echo "$raw" | jq -r '.weight')
reconciled=$(transform_to_reconcile "$raw" 1)
w_reconcile=$(echo "$reconciled" | jq -r '.weight')
[ "$w_inbox" = "null" ] && pass "inbox weight is null (unset)" || fail "expected null inbox weight, got $w_inbox"
[ "$w_reconcile" = "0.5" ] && pass "reconcile weight defaults to 0.5" || fail "expected 0.5, got $w_reconcile"

# --- Test 5: _inbox_source preserves operator metadata ----------------------
header "Test 5: _inbox_source wraps operator metadata"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH --action "metadata test" --note "my note" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json | head -1)
raw=$(cat "$f")
reconciled=$(transform_to_reconcile "$raw" 1)
src_note=$(echo "$reconciled" | jq -r '._inbox_source.operator_note')
src_injby=$(echo "$reconciled" | jq -r '._inbox_source.injected_by')
src_injat=$(echo "$reconciled" | jq -r '._inbox_source.injected_at')
[ "$src_note" = "my note" ]  && pass "_inbox_source.operator_note preserved" || fail "expected 'my note', got '$src_note'"
[ "$src_injby" = "operator" ] && pass "_inbox_source.injected_by=operator"   || fail "wrong injected_by: $src_injby"
[ -n "$src_injat" ] && [ "$src_injat" != "null" ] && pass "_inbox_source.injected_at present" || fail "_inbox_source.injected_at missing"

# --- Test 6: priority + weight tie-break ordering ----------------------------
header "Test 6: priority + weight tie-break ordering"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH   --weight 0.3 --action "high-low"    >/dev/null
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH   --weight 0.9 --action "high-top"    >/dev/null
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority HIGH   --weight 0.7 --action "high-mid"    >/dev/null
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority MEDIUM --weight 0.8 --action "medium-task" >/dev/null
# Simulate Triage sort: priority desc, weight desc
sorted=$(for f in "$PROJ/.evolve/inbox/"*.json; do
    [ -f "$f" ] || continue
    jq -r '[.priority,.weight,.action] | @tsv' "$f"
done | awk '{
    p=$1; w=$2+0; a=$3;
    if (p=="HIGH") pn=3; else if (p=="MEDIUM") pn=2; else pn=1;
    print pn "\t" w "\t" a
}' | sort -rn -k1 -k2 | awk '{print $3}')
first=$(echo "$sorted" | head -1)
last=$(echo "$sorted"  | tail -1)
[ "$first" = "high-top" ]    && pass "highest-weight HIGH task sorts first" || fail "expected high-top first, got '$first'"
[ "$last" = "medium-task" ]  && pass "MEDIUM task sorts last"               || fail "expected medium-task last, got '$last'"

# --- Test 7: multi-project isolation ----------------------------------------
header "Test 7: multi-project isolation"
PROJ_A=$(make_project)
PROJ_B=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ_A" bash "$CLI" --priority HIGH --action "project A task" >/dev/null
count_b=$(ls "$PROJ_B/.evolve/inbox/"*.json 2>/dev/null | wc -l | tr -d ' ')
[ "$count_b" -eq 0 ] && \
    pass "project B inbox empty after injecting into project A" || \
    fail "isolation breach: project B got $count_b files"

# --- Test 8: reconcile jq pass-through preserves _inbox_source ---------------
header "Test 8: _inbox_source survives jq '. + {cycles_unpicked: N}' pass-through"
PROJ=$(make_project)
EVOLVE_PROJECT_ROOT="$PROJ" bash "$CLI" --priority MEDIUM --action "passthrough test" --note "preserved" >/dev/null
f=$(ls "$PROJ/.evolve/inbox/"*.json | head -1)
raw=$(cat "$f")
reconciled=$(transform_to_reconcile "$raw" 3)
# Simulate reconcile-carryover-todos.sh updating cycles_unpicked (pass-through)
updated=$(echo "$reconciled" | jq -c '. + {cycles_unpicked: 1}')
src_note=$(echo "$updated" | jq -r '._inbox_source.operator_note')
[ "$src_note" = "preserved" ] && \
    pass "_inbox_source preserved after jq . + {cycles_unpicked: 1}" || \
    fail "expected 'preserved', got '$src_note'"

# Helper: run the content-verify awk logic from Step-0a on a given SHA in a given repo
# Returns the non_state_changes count
content_verify_count() {
    local repo_dir="$1" sha="$2"
    git -C "$repo_dir" show --stat "$sha" 2>/dev/null | awk '
        /\|/ && $0 !~ /\.evolve\/(inbox|state\.json|ledger|runs|worktrees)/ { count++ }
        END { print count+0 }
    '
}

# Helper: create a minimal git repo for content-verify tests
make_git_repo() {
    local root="$SCRATCH/repo-$RANDOM"
    mkdir -p "$root"
    git -C "$root" init -q
    git -C "$root" config user.email "test@test.com"
    git -C "$root" config user.name "Test"
    # Initial empty commit so HEAD exists
    git -C "$root" commit --allow-empty -q -m "init"
    echo "$root"
}

# --- Test 9: fraudulent commit (only .evolve/inbox/ changes) → INTEGRITY_BREACH ----
header "Test 9: content-verify — fraudulent commit (inbox-only) → non_state_changes=0"
REPO=$(make_git_repo)
# Create a fraudulent commit: only .evolve/inbox/ files, no code files
mkdir -p "$REPO/.evolve/inbox"
printf '{"id":"c38-inbox-audit","action":"recover scripts","priority":"HIGH"}\n' \
    > "$REPO/.evolve/inbox/c38-inbox-audit.json"
git -C "$REPO" add .evolve/inbox/c38-inbox-audit.json
git -C "$REPO" commit -q -m "feat: cycle 39 — c38-inbox-audit: recover inbox-audit.sh"
FRAUD_SHA=$(git -C "$REPO" log --format=%H -1)
non_state=$(content_verify_count "$REPO" "$FRAUD_SHA")
[ "$non_state" -eq 0 ] && \
    pass "fraudulent commit has 0 non-state changes → INTEGRITY_BREACH (not skip_shipped)" || \
    fail "expected 0 non-state changes for fraudulent commit, got $non_state"
# Also verify the SHA itself exists (sanity check)
git -C "$REPO" show --stat "$FRAUD_SHA" >/dev/null 2>&1 && \
    pass "fraudulent commit SHA accessible" || \
    fail "fraudulent commit SHA not accessible"

# --- Test 10: real code commit → non_state_changes > 0 → skip_shipped ----------
header "Test 10: content-verify — real code commit → non_state_changes>0 → skip_shipped"
REPO=$(make_git_repo)
# Create a real code commit: has a non-.evolve/ file
mkdir -p "$REPO/scripts/utility"
printf '#!/usr/bin/env bash\n# inbox-audit.sh recovered\necho "ok"\n' \
    > "$REPO/scripts/utility/inbox-audit.sh"
git -C "$REPO" add scripts/utility/inbox-audit.sh
git -C "$REPO" commit -q -m "feat: cycle 39 — c38-inbox-audit: recover inbox-audit.sh"
REAL_SHA=$(git -C "$REPO" log --format=%H -1)
non_state=$(content_verify_count "$REPO" "$REAL_SHA")
[ "$non_state" -gt 0 ] && \
    pass "real code commit has $non_state non-state change(s) → skip_shipped accepted" || \
    fail "expected >0 non-state changes for real commit, got $non_state"
# Also verify a mixed commit (code + inbox files) still counts as skip_shipped
mkdir -p "$REPO/.evolve/inbox"
printf '{"id":"c38-2","action":"another task","priority":"LOW"}\n' \
    > "$REPO/.evolve/inbox/c38-2.json"
git -C "$REPO" add .evolve/inbox/c38-2.json
git -C "$REPO" commit -q -m "feat: cycle 39 — c38-2: mixed commit"
MIXED_SHA=$(git -C "$REPO" log --format=%H -1)
mixed_non_state=$(content_verify_count "$REPO" "$MIXED_SHA")
[ "$mixed_non_state" -eq 0 ] && \
    pass "inbox-only mixed commit still has 0 non-state changes" || \
    fail "expected 0 non-state for inbox-only mixed commit, got $mixed_non_state"

# --- Summary ------------------------------------------------------------------
echo
echo "Results: $PASS passed, $FAIL failed"
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
