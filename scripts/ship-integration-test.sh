#!/usr/bin/env bash
#
# ship-integration-test.sh — End-to-end tests for scripts/ship.sh.
#
# Each test creates a fresh temp git repo, copies ship.sh into it, seeds
# fake .evolve/ledger.jsonl + audit-report.md, then invokes ship.sh and
# asserts behavior. Never touches the real repo.
#
# Tests are designed to run independently; each `make_repo` returns a
# fresh path under $SCRATCH.
#
# Usage: bash scripts/ship-integration-test.sh

set -uo pipefail

unset EVOLVE_BYPASS_SHIP_VERIFY
unset EVOLVE_SHIP_RELEASE_NOTES

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SHIP_SH="$REPO_ROOT/scripts/ship.sh"
RESOLVE_ROOTS_SH="$REPO_ROOT/scripts/resolve-roots.sh"  # v8.18.0: ship.sh sources this
SCRATCH=$(mktemp -d -t "ship-integration-XXXXXX")
trap 'rm -rf "$SCRATCH"' EXIT

PASS=0
FAIL=0
pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail()   { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

sha256() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | awk '{print $1}';
    else shasum -a 256 "$1" | awk '{print $1}'; fi
}

# Create a fresh git repo with ship.sh installed and basic state.
make_repo() {
    local repo="$SCRATCH/repo-$RANDOM"
    mkdir -p "$repo/scripts" "$repo/.evolve/runs/cycle-1"
    cp "$SHIP_SH" "$repo/scripts/ship.sh"
    chmod +x "$repo/scripts/ship.sh"
    # v8.18.0: ship.sh sources resolve-roots.sh from its own dir; copy it too.
    cp "$RESOLVE_ROOTS_SH" "$repo/scripts/resolve-roots.sh"
    # Mimic production .gitignore — .evolve/ is runtime state, not tracked.
    # Without this, the TOFU SHA pin (which writes to .evolve/state.json)
    # would mutate the tracked tree and trip ship.sh's own tree-state check.
    cat > "$repo/.gitignore" <<EOF
.evolve/
EOF
    : > "$repo/.evolve/ledger.jsonl"
    echo '{}' > "$repo/.evolve/state.json"
    # A test-fixture file that tests can modify without breaking ship.sh's
    # self-SHA pin.
    echo "fixture line 1" > "$repo/fixture.txt"
    cd "$repo"
    git init -q
    git config user.email "test@evolve-loop.test"
    git config user.name "Test User"
    # Disable any pre-commit hooks the user's global config might inject
    git config core.hooksPath /dev/null
    git add -A
    git commit -q -m "initial test repo"
    cd "$REPO_ROOT" >/dev/null
    echo "$repo"
}

# Seed an Auditor ledger entry with PASS verdict, current HEAD, current tree state.
# Args: $1=repo $2=verdict ($3=optional override head) ($4=optional override tree-sha)
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

# --- Test A: no audit ledger → ship.sh refuses ------------------------------
header "Test A: no auditor ledger entry → ship.sh refuses"
REPO=$(make_repo)
cd "$REPO"
set +e
bash scripts/ship.sh "test commit" >/tmp/ship-out 2>&1
RC=$?
set -e
[ "$RC" = "2" ] && pass "no audit → exit 2 (rc=$RC)" || fail "expected rc=2, got rc=$RC; output: $(tail -3 /tmp/ship-out)"
cd "$REPO_ROOT"

# --- Test B: PASS audit, matching state → ship.sh succeeds (commit only) ----
header "Test B: PASS audit + matching state → ship.sh commits"
REPO=$(make_repo)
cd "$REPO"
# Modify a tracked fixture file (not ship.sh — that would break self-SHA).
echo "modified content" >> fixture.txt
seed_audit "$REPO" "PASS"
# Need a remote for `git push` — set a fake one and use --dry-run via env override
# Easier: create a bare repo as "remote"
BARE="$SCRATCH/remote-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
bash scripts/ship.sh "feat: test" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ]; then
    pass "PASS audit + matching state → ship succeeded (rc=0)"
else
    fail "expected rc=0, got rc=$RC; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test C: WARN verdict → ship.sh refuses ---------------------------------
header "Test C: WARN audit → ship.sh refuses"
REPO=$(make_repo)
cd "$REPO"
echo "another change" > other.txt
seed_audit "$REPO" "WARN"
set +e
bash scripts/ship.sh "should not ship" >/tmp/ship-out 2>&1
RC=$?
set -e
[ "$RC" = "2" ] && pass "WARN verdict → exit 2 (rc=$RC)" || fail "expected rc=2, got rc=$RC"
cd "$REPO_ROOT"

# --- Test D: PASS audit but tracked-file change since audit → refuses --------
header "Test D: PASS audit, then modify tracked file → ship.sh refuses (tree-state mismatch)"
REPO=$(make_repo)
cd "$REPO"
# Modify the tracked fixture file BEFORE the audit so the audit captures
# this state.
echo "version 1 of audited content" >> fixture.txt
seed_audit "$REPO" "PASS"
# Now modify the SAME tracked file AFTER audit — this changes git diff HEAD
echo "version 2 — added after audit" >> fixture.txt
set +e
bash scripts/ship.sh "should refuse" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "tree-state mismatch\|uncommitted changes" /tmp/ship-out 2>/dev/null; then
    pass "post-audit change → exit 2 with tree-state error"
else
    fail "expected rc=2 with tree-state error; got rc=$RC; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test E: PASS audit but HEAD has moved → ship.sh refuses ---------------
header "Test E: PASS audit with old HEAD → ship.sh refuses (HEAD mismatch)"
REPO=$(make_repo)
cd "$REPO"
seed_audit "$REPO" "PASS" "0000000000000000000000000000000000000000" ""
set +e
bash scripts/ship.sh "should refuse" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "HEAD has moved" /tmp/ship-out 2>/dev/null; then
    pass "wrong HEAD → exit 2 with HEAD-moved error"
else
    fail "expected rc=2 with HEAD-moved error; got rc=$RC; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test F: ship.sh modified post-pin → refuses ----------------------------
header "Test F: ship.sh modified after SHA pinned → refuses"
REPO=$(make_repo)
cd "$REPO"
seed_audit "$REPO" "PASS"
# First run pins the SHA (TOFU); needs a remote configured
BARE="$SCRATCH/remote-test-f-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
echo "audited" > audited.txt
seed_audit "$REPO" "PASS"
# First run: pins SHA + commits. Should succeed.
set +e; bash scripts/ship.sh "first ship" >/tmp/ship-out 2>&1; RC1=$?; set -e
# Now MODIFY ship.sh to simulate tampering
echo "# malicious comment" >> scripts/ship.sh
# Try to ship again — should refuse due to SHA mismatch
echo "another change" > another.txt
seed_audit "$REPO" "PASS"
set +e; bash scripts/ship.sh "second ship" >/tmp/ship-out 2>&1; RC2=$?; set -e
if [ "$RC2" = "2" ] && grep -q "ship.sh has been modified" /tmp/ship-out 2>/dev/null; then
    pass "ship.sh modification detected (first rc=$RC1, second rc=$RC2)"
else
    fail "expected second rc=2 with self-SHA error; got rc=$RC2; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test G: bypass switch allows ship despite no audit ----------------------
header "Test G: EVOLVE_BYPASS_SHIP_VERIFY=1 allows ship without audit"
REPO=$(make_repo)
cd "$REPO"
echo "emergency change" > emergency.txt
BARE="$SCRATCH/remote-test-g-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/ship.sh "emergency" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ]; then
    pass "bypass allows ship without audit (rc=0)"
else
    fail "expected rc=0 with bypass; got rc=$RC; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test H: --class release skips audit and ships --------------------------
# release class is for scripts/release-pipeline.sh use; skips audit-binding
# (because version-bump.sh mutates files post-audit) but logs RELEASE class.
header "Test H: v8.25.0 --class release ships without audit"
REPO=$(make_repo)
cd "$REPO"
echo "release bump" > release.txt
BARE="$SCRATCH/remote-test-h-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
bash scripts/ship.sh --class release "release: v9.99.99" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ] && grep -q "class: release" /tmp/ship-out; then
    pass "--class release ships and logs class line"
else
    fail "rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test I: --class manual without tty refuses ------------------------------
# manual class requires interactive y/N; if stdin is not a tty AND no
# auto-confirm env, ship.sh must refuse rather than silently proceed.
header "Test I: v8.25.0 --class manual without tty + no auto-confirm → refuses"
REPO=$(make_repo)
cd "$REPO"
echo "manual change" > manual.txt
BARE="$SCRATCH/remote-test-i-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
bash scripts/ship.sh --class manual "manual change" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "requires interactive stdin" /tmp/ship-out; then
    pass "non-tty manual class refused with rc=2 + helpful message"
else
    fail "rc=$RC; tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test J: --class manual + EVOLVE_SHIP_AUTO_CONFIRM=1 ships --------------
# CI mode: auto-confirm bypasses the y/N prompt. This is the migration path
# for scripts that used to set EVOLVE_BYPASS_SHIP_VERIFY=1.
header "Test J: v8.25.0 --class manual + AUTO_CONFIRM=1 → ships"
REPO=$(make_repo)
cd "$REPO"
echo "ci change" > ci.txt
BARE="$SCRATCH/remote-test-j-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/ship.sh --class manual "ci change" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ] && grep -q "auto-confirmed" /tmp/ship-out; then
    pass "manual + auto-confirm ships and logs auto-confirmed"
else
    fail "rc=$RC; tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test K: legacy EVOLVE_BYPASS_SHIP_VERIFY emits deprecation warning -----
# Backward compat: EVOLVE_BYPASS_SHIP_VERIFY=1 still works but logs a
# DEPRECATION line pointing to --class manual.
header "Test K: v8.25.0 legacy BYPASS_SHIP_VERIFY → deprecation warning logged"
REPO=$(make_repo)
cd "$REPO"
echo "bridge change" > bridge.txt
BARE="$SCRATCH/remote-test-k-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/ship.sh "legacy bypass" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ] && grep -q "DEPRECATION: EVOLVE_BYPASS_SHIP_VERIFY=1 is deprecated" /tmp/ship-out; then
    pass "legacy bypass works + emits deprecation pointer to --class manual"
else
    fail "rc=$RC; tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test L: invalid --class argument rejected -------------------------------
header "Test L: v8.25.0 --class garbage → rejected with rc=1"
REPO=$(make_repo)
cd "$REPO"
set +e
bash scripts/ship.sh --class garbage "msg" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "1" ] && grep -q "invalid --class" /tmp/ship-out; then
    pass "invalid class rejected"
else
    fail "rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Summary ----------------------------------------------------------------
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
