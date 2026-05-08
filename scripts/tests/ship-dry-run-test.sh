#!/usr/bin/env bash
#
# ship-dry-run-test.sh — Tests for scripts/lifecycle/ship.sh --dry-run (v8.50.0).
#
# Verifies the dry-run path:
#   - Exits 0 with no git mutations (HEAD unchanged, no commits, no pushes).
#   - Preserves all read-only checks (audit binding, TOFU SHA, sequence).
#   - Writes a journal preview to .evolve/release-journal/dry-run-<ts>.json.
#   - Honors --class cycle / manual / release alongside --dry-run.
#   - Refuses (exit 2) when audit is missing or invalid, even in dry-run —
#     dry-run validates the pipeline, it does not bypass integrity checks.
#
# Tests are designed to run independently; each `make_repo` returns a fresh
# path under $SCRATCH. No real git/gh side effects.

set -uo pipefail

unset EVOLVE_BYPASS_SHIP_VERIFY
unset EVOLVE_SHIP_RELEASE_NOTES
unset EVOLVE_PROJECT_ROOT EVOLVE_PLUGIN_ROOT EVOLVE_RESOLVE_ROOTS_LOADED

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SHIP_SH="$REPO_ROOT/scripts/lifecycle/ship.sh"
RESOLVE_ROOTS_SH="$REPO_ROOT/scripts/lifecycle/resolve-roots.sh"
SCRATCH=$(mktemp -d -t "ship-dryrun-XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
    else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# Mirror of ship-integration-test.sh:make_repo, locally redefined to keep this
# test file self-contained.
make_repo() {
    local repo="$SCRATCH/repo-$RANDOM"
    mkdir -p "$repo/scripts/lifecycle" "$repo/.evolve/runs/cycle-1"
    cp "$SHIP_SH" "$repo/scripts/lifecycle/ship.sh"
    chmod +x "$repo/scripts/lifecycle/ship.sh"
    cp "$RESOLVE_ROOTS_SH" "$repo/scripts/lifecycle/resolve-roots.sh"
    cat > "$repo/.gitignore" <<EOF
.evolve/
EOF
    : > "$repo/.evolve/ledger.jsonl"
    echo '{}' > "$repo/.evolve/state.json"
    echo "fixture line 1" > "$repo/fixture.txt"
    cd "$repo"
    git init -q
    git config user.email "test@evolve-loop.test"
    git config user.name "Test User"
    git config core.hooksPath /dev/null
    git add -A
    git commit -q -m "initial test repo"
    cd "$REPO_ROOT" >/dev/null
    echo "$repo"
}

seed_audit() {
    local repo="$1" verdict="$2"
    local override_head="${3:-}" override_tree="${4:-}"
    local audit_path="$repo/.evolve/runs/cycle-1/audit-report.md"
    cat > "$audit_path" <<EOF
<!-- challenge-token: testtoken123 -->
# Audit Report — Cycle 1

Verdict: ${verdict}

All criteria met (test fixture).
EOF
    local sha; sha=$(sha256 "$audit_path")
    local head_sha
    if [ -n "$override_head" ]; then
        head_sha="$override_head"
    else
        head_sha=$(git -C "$repo" rev-parse HEAD)
    fi
    local tree_sha
    if [ -n "$override_tree" ]; then
        tree_sha="$override_tree"
    else
        tree_sha=$(git -C "$repo" diff HEAD | (
            if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
            else shasum -a 256 | awk '{print $1}'; fi
        ))
    fi
    cat > "$repo/.evolve/ledger.jsonl" <<EOF
{"ts":"2026-04-27T00:00:00Z","cycle":1,"role":"auditor","kind":"agent_subprocess","model":"sonnet","exit_code":0,"duration_s":"30","artifact_path":"$audit_path","artifact_sha256":"$sha","challenge_token":"testtoken123","git_head":"$head_sha","tree_state_sha":"$tree_sha"}
EOF
}

# === Test 1: --dry-run flag accepted, exits 0 on PASS audit =====================
header "Test 1: --dry-run + PASS audit → exit 0, no commit"
REPO=$(make_repo)
cd "$REPO"
echo "dry-run change" >> fixture.txt
seed_audit "$REPO" "PASS"
HEAD_BEFORE=$(git rev-parse HEAD)
set +e
bash scripts/lifecycle/ship.sh --dry-run "feat: test dry-run" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
HEAD_AFTER=$(git rev-parse HEAD)
if [ "$RC" = "0" ] && [ "$HEAD_BEFORE" = "$HEAD_AFTER" ]; then
    pass "PASS audit + --dry-run → rc=0 with HEAD unchanged"
else
    fail_ "rc=$RC HEAD_BEFORE=$HEAD_BEFORE HEAD_AFTER=$HEAD_AFTER; tail: $(tail -3 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Test 2: --dry-run emits "[DRY-RUN] would: ..." log lines ===================
header "Test 2: --dry-run logs would-be operations"
REPO=$(make_repo)
cd "$REPO"
echo "log change" >> fixture.txt
seed_audit "$REPO" "PASS"
set +e
bash scripts/lifecycle/ship.sh --dry-run "feat: log test" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ] && grep -q "DRY-RUN" /tmp/ship-dry-out && grep -q "DRY-RUN DONE" /tmp/ship-dry-out; then
    pass "dry-run log markers present (DRY-RUN, DRY-RUN DONE)"
else
    fail_ "missing DRY-RUN log lines; rc=$RC tail: $(tail -5 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Test 3: --dry-run writes journal preview ====================================
header "Test 3: --dry-run writes .evolve/release-journal/dry-run-<ts>.json"
REPO=$(make_repo)
cd "$REPO"
echo "preview change" >> fixture.txt
seed_audit "$REPO" "PASS"
set +e
bash scripts/lifecycle/ship.sh --dry-run "feat: preview test" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
preview=$(ls -1 "$REPO/.evolve/release-journal/dry-run-"*.json 2>/dev/null | head -1)
if [ "$RC" = "0" ] && [ -n "$preview" ] && jq empty "$preview" 2>/dev/null; then
    # Verify schema fields present
    class=$(jq -r '.class' "$preview")
    msg=$(jq -r '.commit_msg' "$preview")
    # commit_msg may include the v8.34 actual-diff footer; just verify the
    # operator-supplied prefix is present.
    if [ "$class" = "cycle" ] && [[ "$msg" == "feat: preview test"* ]]; then
        pass "preview journal valid JSON with class=$class msg-prefix matches"
    else
        fail_ "preview missing fields: class=$class msg-prefix=${msg:0:40}"
    fi
else
    fail_ "no preview journal written or invalid JSON; preview=$preview rc=$RC"
fi
cd "$REPO_ROOT"

# === Test 4: --dry-run preserves audit-binding (no audit → still refuses rc=2) ==
header "Test 4: --dry-run + missing audit → rc=2 (integrity preserved)"
REPO=$(make_repo)
cd "$REPO"
echo "no audit" >> fixture.txt
# Intentionally do NOT seed_audit — ledger is empty
set +e
bash scripts/lifecycle/ship.sh --dry-run "should refuse" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ]; then
    pass "missing audit + --dry-run → rc=2 (integrity check still runs)"
else
    fail_ "expected rc=2 (no audit), got rc=$RC; tail: $(tail -3 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Test 5: --dry-run + --class manual + auto-confirm → exit 0, no commit =====
header "Test 5: --dry-run + --class manual + EVOLVE_SHIP_AUTO_CONFIRM=1 → exit 0"
REPO=$(make_repo)
cd "$REPO"
echo "manual change" >> fixture.txt
HEAD_BEFORE=$(git rev-parse HEAD)
set +e
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --dry-run --class manual "feat: manual dry" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
HEAD_AFTER=$(git rev-parse HEAD)
if [ "$RC" = "0" ] && [ "$HEAD_BEFORE" = "$HEAD_AFTER" ]; then
    pass "manual + --dry-run + auto-confirm → rc=0 with HEAD unchanged"
else
    fail_ "rc=$RC HEAD_BEFORE=$HEAD_BEFORE HEAD_AFTER=$HEAD_AFTER; tail: $(tail -5 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Test 6: --dry-run + --class release → exit 0, no commit ===================
header "Test 6: --dry-run + --class release → exit 0"
REPO=$(make_repo)
cd "$REPO"
echo "release change" >> fixture.txt
HEAD_BEFORE=$(git rev-parse HEAD)
set +e
bash scripts/lifecycle/ship.sh --dry-run --class release "release: v999.0.0" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
HEAD_AFTER=$(git rev-parse HEAD)
if [ "$RC" = "0" ] && [ "$HEAD_BEFORE" = "$HEAD_AFTER" ]; then
    pass "release + --dry-run → rc=0 with HEAD unchanged"
else
    fail_ "rc=$RC HEAD_BEFORE=$HEAD_BEFORE HEAD_AFTER=$HEAD_AFTER; tail: $(tail -5 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Test 7: --dry-run does NOT advance state.json:lastCycleNumber ==============
header "Test 7: --dry-run + class=cycle leaves state.json:lastCycleNumber untouched"
REPO=$(make_repo)
cd "$REPO"
# Seed cycle-state.json so the lastCycleNumber-bump branch would fire in real run
mkdir -p .evolve
cat > .evolve/cycle-state.json <<EOF
{"cycle_id":42,"phase":"ship"}
EOF
echo '{"lastCycleNumber":1}' > .evolve/state.json
echo "state-bump change" >> fixture.txt
seed_audit "$REPO" "PASS"
set +e
bash scripts/lifecycle/ship.sh --dry-run "feat: state-bump" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
LAST=$(jq -r '.lastCycleNumber' .evolve/state.json)
if [ "$RC" = "0" ] && [ "$LAST" = "1" ]; then
    pass "state.json:lastCycleNumber preserved at 1 (would-be-42 not applied)"
else
    fail_ "rc=$RC lastCycleNumber=$LAST (expected 1)"
fi
cd "$REPO_ROOT"

# === Test 8: --dry-run records would-be operations in DRY_RUN_OPS journal ======
header "Test 8: --dry-run preview lists would_have operations"
REPO=$(make_repo)
cd "$REPO"
echo "ops change" >> fixture.txt
seed_audit "$REPO" "PASS"
set +e
bash scripts/lifecycle/ship.sh --dry-run "feat: ops" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
preview=$(ls -1 "$REPO/.evolve/release-journal/dry-run-"*.json 2>/dev/null | head -1)
if [ "$RC" = "0" ] && [ -n "$preview" ]; then
    ops=$(jq -r '.would_have | length' "$preview" 2>/dev/null || echo 0)
    has_commit=$(jq -r '.would_have | map(test("commit")) | any' "$preview" 2>/dev/null || echo "false")
    has_push=$(jq -r '.would_have | map(test("push")) | any' "$preview" 2>/dev/null || echo "false")
    if [ "$ops" -ge 2 ] && [ "$has_commit" = "true" ] && [ "$has_push" = "true" ]; then
        pass "would_have records both commit + push (count=$ops)"
    else
        fail_ "ops=$ops has_commit=$has_commit has_push=$has_push; preview body: $(cat "$preview")"
    fi
else
    fail_ "no preview written; rc=$RC"
fi
cd "$REPO_ROOT"

# === Test 9: --dry-run + EVOLVE_SHIP_RELEASE_NOTES → records gh-release op ====
header "Test 9: --dry-run + EVOLVE_SHIP_RELEASE_NOTES → records gh-release op"
REPO=$(make_repo)
cd "$REPO"
mkdir -p .claude-plugin
echo '{"version":"99.0.0"}' > .claude-plugin/plugin.json
echo "release-notes change" >> fixture.txt
seed_audit "$REPO" "PASS"
set +e
EVOLVE_SHIP_RELEASE_NOTES="test release notes" \
    bash scripts/lifecycle/ship.sh --dry-run "feat: notes" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
preview=$(ls -1 "$REPO/.evolve/release-journal/dry-run-"*.json 2>/dev/null | head -1)
if [ "$RC" = "0" ] && [ -n "$preview" ]; then
    has_release=$(jq -r '.would_have | map(test("gh-release")) | any' "$preview" 2>/dev/null || echo "false")
    if [ "$has_release" = "true" ]; then
        pass "gh-release op recorded in preview"
    else
        fail_ "gh-release op not recorded; preview: $(cat "$preview")"
    fi
else
    fail_ "no preview; rc=$RC tail: $(tail -3 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Test 10: --dry-run + remote unset → rc=0 (push is skipped, no remote needed)
header "Test 10: --dry-run + no remote → rc=0 (push is simulated, not attempted)"
REPO=$(make_repo)
cd "$REPO"
# Deliberately do NOT add a remote — real ship would fail at push step.
echo "no remote change" >> fixture.txt
seed_audit "$REPO" "PASS"
set +e
bash scripts/lifecycle/ship.sh --dry-run "feat: no remote" >/tmp/ship-dry-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ]; then
    pass "--dry-run skips push, rc=0 even without remote"
else
    fail_ "rc=$RC; expected 0 (push should be simulated); tail: $(tail -5 /tmp/ship-dry-out)"
fi
cd "$REPO_ROOT"

# === Summary ====================================================================
echo
echo "==========================================="
echo "  Total: 10 tests"
echo "  PASS:  $PASS"
echo "  FAIL:  $FAIL"
echo "==========================================="
[ "$FAIL" -eq 0 ] && exit 0 || exit 1
