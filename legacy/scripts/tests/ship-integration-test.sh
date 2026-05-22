#!/usr/bin/env bash
#
# ship-integration-test.sh — End-to-end tests for scripts/lifecycle/ship.sh.
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
# v8.29.0: unset evolve-loop env vars that leak from the Claude Code parent session.
# Without this, resolve-roots.sh's idempotency guard fires (EVOLVE_RESOLVE_ROOTS_LOADED=1)
# and ship.sh reads the production state.json (with stale expected_ship_sha) instead
# of each test's fresh repo state.
unset EVOLVE_PROJECT_ROOT EVOLVE_PLUGIN_ROOT EVOLVE_RESOLVE_ROOTS_LOADED

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SHIP_SH="$REPO_ROOT/scripts/lifecycle/ship.sh"
RESOLVE_ROOTS_SH="$REPO_ROOT/scripts/lifecycle/resolve-roots.sh"  # v8.18.0: ship.sh sources this
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
    mkdir -p "$repo/scripts/lifecycle" "$repo/.evolve/runs/cycle-1"
    cp "$SHIP_SH" "$repo/scripts/lifecycle/ship.sh"
    chmod +x "$repo/scripts/lifecycle/ship.sh"
    # v8.18.0: ship.sh sources resolve-roots.sh from its own dir; copy it too.
    cp "$RESOLVE_ROOTS_SH" "$repo/scripts/lifecycle/resolve-roots.sh"
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
bash scripts/lifecycle/ship.sh "test commit" >/tmp/ship-out 2>&1
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
bash scripts/lifecycle/ship.sh "feat: test" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ]; then
    pass "PASS audit + matching state → ship succeeded (rc=0)"
else
    fail "expected rc=0, got rc=$RC; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test C: v8.28.0 — WARN verdict ships by default (fluent mode) ----------
# Pre-v8.28.0: WARN was treated like FAIL (block ship). v8.28.0 inverts:
# WARN ships by default, with a log line. Operator opts back to strict
# blocking via EVOLVE_STRICT_AUDIT=1.
header "Test C: v8.28.0 — WARN audit ships by default (fluent)"
REPO=$(make_repo)
cd "$REPO"
echo "warn change" >> fixture.txt
seed_audit "$REPO" "WARN"
BARE_C="$SCRATCH/remote-test-c-$RANDOM.git"
git init -q --bare "$BARE_C"
git remote add origin "$BARE_C"
git branch -M main
set +e
bash scripts/lifecycle/ship.sh "feat: shipping with WARN" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ] && grep -q "audit verdict: WARN — shipping" /tmp/ship-out; then
    pass "WARN ships by default + logs the relaxation"
else
    fail "expected rc=0 with WARN-shipping log; got rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test C2: v8.28.0 — EVOLVE_STRICT_AUDIT=1 restores WARN → block ---------
header "Test C2: v8.28.0 — EVOLVE_STRICT_AUDIT=1 → WARN audit refuses"
REPO=$(make_repo)
cd "$REPO"
echo "warn change strict" >> fixture.txt
seed_audit "$REPO" "WARN"
set +e
EVOLVE_STRICT_AUDIT=1 bash scripts/lifecycle/ship.sh "should not ship" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "EVOLVE_STRICT_AUDIT=1" /tmp/ship-out; then
    pass "STRICT_AUDIT=1 restores legacy WARN→block"
else
    fail "expected rc=2 with strict-audit message; got rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
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
bash scripts/lifecycle/ship.sh "should refuse" >/tmp/ship-out 2>&1
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
bash scripts/lifecycle/ship.sh "should refuse" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "HEAD has moved" /tmp/ship-out 2>/dev/null; then
    pass "wrong HEAD → exit 2 with HEAD-moved error"
else
    fail "expected rc=2 with HEAD-moved error; got rc=$RC; output: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test F: v8.32.0 — ship.sh modified WITHIN same plugin version → refuses ---
# The threat model that matters: tampering of ship.sh while plugin version
# is unchanged. v8.32.0's version-aware TOFU still catches this.
header "Test F: v8.32.0 — ship.sh modified within same plugin version → refuses"
REPO=$(make_repo)
cd "$REPO"
# v8.32.0: seed plugin.json so PLUGIN_VERSION is non-empty (otherwise the
# legacy-migration path fires on second ship and we'd never see the same-
# version-different-SHA path which is what we want to test).
mkdir -p "$REPO/.claude-plugin"
echo '{"version":"1.0.0"}' > "$REPO/.claude-plugin/plugin.json"
seed_audit "$REPO" "PASS"
BARE="$SCRATCH/remote-test-f-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
echo "audited" > audited.txt
seed_audit "$REPO" "PASS"
# First run: pins SHA + version=1.0.0, commits. Should succeed.
set +e; bash scripts/lifecycle/ship.sh "first ship" >/tmp/ship-out 2>&1; RC1=$?; set -e
# Modify ship.sh — simulates tampering. plugin.json:version stays at 1.0.0.
echo "# malicious comment" >> scripts/lifecycle/ship.sh
# Second ship: same version, different SHA → REAL tampering → rc=2.
echo "another change" > another.txt
seed_audit "$REPO" "PASS"
set +e; bash scripts/lifecycle/ship.sh "second ship" >/tmp/ship-out 2>&1; RC2=$?; set -e
if [ "$RC2" = "2" ] && grep -q "WITHIN plugin version" /tmp/ship-out 2>/dev/null; then
    pass "v8.32.0: same-version-different-SHA → integrity fail (first rc=$RC1, second rc=$RC2)"
else
    fail "expected second rc=2 with within-version error; got rc=$RC2; output: $(tail -3 /tmp/ship-out)"
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
EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/lifecycle/ship.sh "emergency" >/tmp/ship-out 2>&1
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
bash scripts/lifecycle/ship.sh --class release "release: v9.99.99" >/tmp/ship-out 2>&1
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
bash scripts/lifecycle/ship.sh --class manual "manual change" </dev/null >/tmp/ship-out 2>&1
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
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --class manual "ci change" </dev/null >/tmp/ship-out 2>&1
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
EVOLVE_BYPASS_SHIP_VERIFY=1 bash scripts/lifecycle/ship.sh "legacy bypass" </dev/null >/tmp/ship-out 2>&1
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
bash scripts/lifecycle/ship.sh --class garbage "msg" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "1" ] && grep -q "invalid --class" /tmp/ship-out; then
    pass "invalid class rejected"
else
    fail "rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test M: v8.27.0 — auditor exit_code=1 + Verdict: PASS → ship succeeds ---
# Pre-v8.27.0 BUG: ship-gate rejected ANY non-zero exit_code, even when the
# audit-report.md declared Verdict: PASS. This conflicted with the auditor
# CLI's Unix-convention semantics (exit 1 = findings present, normal).
# v8.27.0 accepts exit 0 OR 1 if the artifact verdict + SHA + freshness all
# verify. Anti-gaming preserved by SHA + Verdict-text checks.
header "Test M: v8.27.0 — auditor exit_code=1 + Verdict:PASS → ship succeeds"
REPO=$(make_repo)
cd "$REPO"
echo "modified for exit-1 test" >> fixture.txt
# Inline seed: like seed_audit but with exit_code=1 in the ledger entry.
audit_path="$REPO/.evolve/runs/cycle-1/audit-report.md"
cat > "$audit_path" <<EOF
<!-- challenge-token: testtoken123 -->
# Audit Report — Cycle 1

Verdict: PASS

Findings noted but not blocking. Test fixture for v8.27.0 exit-1 semantics.
EOF
sha=$(sha256 "$audit_path")
head_sha=$(git -C "$REPO" rev-parse HEAD)
tree_sha=$(git -C "$REPO" diff HEAD | (
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
))
# CRITICAL: exit_code is 1 here, not 0 — this is the v8.27.0 case
cat > "$REPO/.evolve/ledger.jsonl" <<EOF
{"ts":"2026-04-27T00:00:00Z","cycle":1,"role":"auditor","kind":"agent_subprocess","model":"sonnet","exit_code":1,"duration_s":"30","artifact_path":"$audit_path","artifact_sha256":"$sha","challenge_token":"testtoken123","git_head":"$head_sha","tree_state_sha":"$tree_sha"}
EOF
BARE="$SCRATCH/remote-test-m-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
bash scripts/lifecycle/ship.sh "feat: ship with exit-1" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "0" ]; then
    pass "exit_code=1 + Verdict:PASS → ship succeeded (v8.27.0 fluency fix)"
else
    fail "expected rc=0, got rc=$RC; output: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test N: v8.27.0 — auditor exit_code=2 (true error) → ship STILL refuses ---
# Anti-gaming regression: the v8.27.0 relaxation accepts 0 or 1 only.
# exit 2+ indicates a true crash/error and must still block ship.
header "Test N: v8.27.0 — auditor exit_code=2 → ship refuses (anti-gaming preserved)"
REPO=$(make_repo)
cd "$REPO"
echo "modified for exit-2 test" >> fixture.txt
audit_path="$REPO/.evolve/runs/cycle-1/audit-report.md"
cat > "$audit_path" <<EOF
<!-- challenge-token: testtoken123 -->
# Audit Report — Cycle 1

Verdict: PASS

(But auditor exit code claims error state.)
EOF
sha=$(sha256 "$audit_path")
head_sha=$(git -C "$REPO" rev-parse HEAD)
tree_sha=$(git -C "$REPO" diff HEAD | (
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
))
cat > "$REPO/.evolve/ledger.jsonl" <<EOF
{"ts":"2026-04-27T00:00:00Z","cycle":1,"role":"auditor","kind":"agent_subprocess","model":"sonnet","exit_code":2,"duration_s":"30","artifact_path":"$audit_path","artifact_sha256":"$sha","challenge_token":"testtoken123","git_head":"$head_sha","tree_state_sha":"$tree_sha"}
EOF
BARE="$SCRATCH/remote-test-n-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
bash scripts/lifecycle/ship.sh "feat: ship with exit-2" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "Auditor exited 2" /tmp/ship-out; then
    pass "exit_code=2 → ship refused (rc=2) with diagnostic"
else
    fail "expected rc=2 with 'Auditor exited 2', got rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test O: v8.27.0 anti-gaming regression — exit_code=0 + Verdict:FAIL → refuses ---
# Verdict text is the source of truth. Even exit_code=0 must NOT bypass the
# Verdict: PASS requirement.
header "Test O: v8.27.0 — exit_code=0 + Verdict:FAIL → ship refuses (verdict text wins)"
REPO=$(make_repo)
cd "$REPO"
echo "modified for verdict-fail test" >> fixture.txt
audit_path="$REPO/.evolve/runs/cycle-1/audit-report.md"
cat > "$audit_path" <<EOF
<!-- challenge-token: testtoken123 -->
# Audit Report — Cycle 1

Verdict: FAIL

Critical issues found. Do not ship.
EOF
sha=$(sha256 "$audit_path")
head_sha=$(git -C "$REPO" rev-parse HEAD)
tree_sha=$(git -C "$REPO" diff HEAD | (
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
))
cat > "$REPO/.evolve/ledger.jsonl" <<EOF
{"ts":"2026-04-27T00:00:00Z","cycle":1,"role":"auditor","kind":"agent_subprocess","model":"sonnet","exit_code":0,"duration_s":"30","artifact_path":"$audit_path","artifact_sha256":"$sha","challenge_token":"testtoken123","git_head":"$head_sha","tree_state_sha":"$tree_sha"}
EOF
BARE="$SCRATCH/remote-test-o-$RANDOM.git"
git init -q --bare "$BARE"
git remote add origin "$BARE"
git branch -M main
set +e
bash scripts/lifecycle/ship.sh "feat: ship with verdict fail" >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -qE "Verdict: ?FAIL|auditor explicitly rejected" /tmp/ship-out; then
    pass "exit_code=0 + Verdict:FAIL → ship refused (verdict text caught it; anti-gaming preserved)"
else
    fail "expected rc=2 with verdict diagnostic, got rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test P: v8.30.0 — dual-verdict (PASS + FAIL) → ship refuses with explanation ---
# Real audit-report observed in cycle-25: header "## Verdict\n**FAIL**" +
# per-eval section showing PASS for all 4 evals. Pre-v8.30.0, ship-gate
# blocked with "declares Verdict: FAIL". v8.30.0 surfaces it explicitly as
# auditor inconsistency.
header "Test P: v8.30.0 — dual-verdict (PASS + FAIL) → refuse with auditor-inconsistency message"
REPO=$(make_repo)
cd "$REPO"
echo "dual-verdict change" >> fixture.txt
audit_path="$REPO/.evolve/runs/cycle-1/audit-report.md"
cat > "$audit_path" <<EOF
<!-- challenge-token: testtoken123 -->
# Audit Report — Cycle 1

## Verdict
**FAIL**

But also somewhere in this report:
Verdict: PASS

(simulating cycle-25's actual audit-report.md inconsistency)
EOF
sha=$(sha256 "$audit_path")
head_sha=$(git -C "$REPO" rev-parse HEAD)
tree_sha=$(git -C "$REPO" diff HEAD | (
    if command -v sha256sum >/dev/null 2>&1; then sha256sum | awk '{print $1}';
    else shasum -a 256 | awk '{print $1}'; fi
))
cat > "$REPO/.evolve/ledger.jsonl" <<EOF
{"ts":"2026-04-27T00:00:00Z","cycle":1,"role":"auditor","kind":"agent_subprocess","model":"sonnet","exit_code":0,"duration_s":"30","artifact_path":"$audit_path","artifact_sha256":"$sha","challenge_token":"testtoken123","git_head":"$head_sha","tree_state_sha":"$tree_sha"}
EOF
set +e
bash scripts/lifecycle/ship.sh "ship dual-verdict" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
if [ "$RC" = "2" ] && grep -q "BOTH 'Verdict: FAIL' AND 'Verdict: PASS'" /tmp/ship-out; then
    pass "dual-verdict refused with auditor-inconsistency message"
else
    fail "rc=$RC; tail: $(tail -3 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test Q: v8.32.0 — plugin version bump + ship.sh SHA change → re-pin ----
# The dominant cause of SHA changes is plugin updates. Pre-v8.32.0 caught
# every update as INTEGRITY-FAIL. v8.32.0 detects version mismatch and
# re-pins automatically.
header "Test Q: v8.32.0 — plugin version bump → re-pin SHA, ship continues"
REPO=$(make_repo)
cd "$REPO"
mkdir -p "$REPO/.claude-plugin"
echo '{"version":"1.0.0"}' > "$REPO/.claude-plugin/plugin.json"
seed_audit "$REPO" "PASS"
BARE_Q="$SCRATCH/remote-test-q-$RANDOM.git"
git init -q --bare "$BARE_Q"
git remote add origin "$BARE_Q"
git branch -M main
echo "first audited" > q1.txt
seed_audit "$REPO" "PASS"
set +e; bash scripts/lifecycle/ship.sh "first ship at v1.0.0" >/tmp/ship-out 2>&1; RC1=$?; set -e
# Bump plugin version AND modify ship.sh to simulate plugin update
echo '{"version":"1.1.0"}' > "$REPO/.claude-plugin/plugin.json"
echo "# v1.1.0 ship.sh tweak" >> scripts/lifecycle/ship.sh
echo "second" > q2.txt
seed_audit "$REPO" "PASS"
set +e; bash scripts/lifecycle/ship.sh "ship at v1.1.0" >/tmp/ship-out 2>&1; RC2=$?; set -e
if [ "$RC2" = "0" ] && grep -q "plugin version changed: '1.0.0' → '1.1.0'" /tmp/ship-out; then
    pass "v8.32.0: plugin version bump auto-re-pins (first rc=$RC1, second rc=$RC2)"
else
    fail "expected rc=0 with version-change log; got rc=$RC2; tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test R: v8.32.0 — legacy state.json (SHA-only pin) → migrate ---------
# Pre-v8.32.0 state.json has expected_ship_sha but no expected_ship_version.
# v8.32.0 detects this and migrates to version-aware schema.
header "Test R: v8.32.0 — legacy SHA-only pin migrates on first run"
REPO=$(make_repo)
cd "$REPO"
mkdir -p "$REPO/.claude-plugin"
echo '{"version":"2.0.0"}' > "$REPO/.claude-plugin/plugin.json"
# Seed state.json with a pin matching the CURRENT ship.sh SHA but no version
ACTUAL=$(sha256 "$REPO/scripts/lifecycle/ship.sh")
jq --arg sha "$ACTUAL" '. + {expected_ship_sha: $sha}' "$REPO/.evolve/state.json" > "$REPO/.evolve/state.json.tmp" \
    && mv "$REPO/.evolve/state.json.tmp" "$REPO/.evolve/state.json"
seed_audit "$REPO" "PASS"
BARE_R="$SCRATCH/remote-test-r-$RANDOM.git"
git init -q --bare "$BARE_R"
git remote add origin "$BARE_R"
git branch -M main
echo "audited" > r.txt
seed_audit "$REPO" "PASS"
set +e; bash scripts/lifecycle/ship.sh "ship after migration" >/tmp/ship-out 2>&1; RC=$?; set -e
new_ver=$(jq -r '.expected_ship_version // empty' "$REPO/.evolve/state.json")
if [ "$RC" = "0" ] && [ "$new_ver" = "2.0.0" ] && grep -qE "(migrating legacy SHA-only pin|schema migration)" /tmp/ship-out; then
    pass "v8.32.0: legacy SHA-only pin migrated to version-aware (rc=$RC, version='$new_ver')"
else
    fail "rc=$RC; new_ver='$new_ver'; tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test S: v8.34.0 — successful cycle ship advances state.json:lastCycleNumber ---
# Pre-v8.34, ship.sh committed/pushed but never wrote state.json:lastCycleNumber.
# The dispatcher's next iteration computed last_before unchanged → ran_cycle =
# the SAME cycle number → 5-repeat circuit-breaker fired prematurely on
# legitimate runs. v8.34.0 has ship.sh advance the counter from cycle-state.json:cycle_id.
header "Test S: v8.34.0 — cycle ship advances state.json:lastCycleNumber"
REPO=$(make_repo)
cd "$REPO"
echo "v8.34.0 cycle ship test" >> fixture.txt
seed_audit "$REPO" "PASS"
# Seed cycle-state.json so ship.sh can read cycle_id.
echo '{"cycle_id":1,"phase":"ship"}' > "$REPO/.evolve/cycle-state.json"
# Initial state.json has lastCycleNumber=0
jq '. + {lastCycleNumber: 0}' "$REPO/.evolve/state.json" > "$REPO/.evolve/state.json.tmp" \
    && mv "$REPO/.evolve/state.json.tmp" "$REPO/.evolve/state.json"
BARE_S="$SCRATCH/remote-test-s-$RANDOM.git"
git init -q --bare "$BARE_S"
git remote add origin "$BARE_S"
git branch -M main
set +e; bash scripts/lifecycle/ship.sh "feat: cycle 1 work" >/tmp/ship-out 2>&1; RC=$?; set -e
new_lcn=$(jq -r '.lastCycleNumber // 0' "$REPO/.evolve/state.json")
if [ "$RC" = "0" ] && [ "$new_lcn" = "1" ] && grep -q "advanced state.json:lastCycleNumber to 1" /tmp/ship-out; then
    pass "v8.34.0: cycle ship advanced lastCycleNumber 0→1"
else
    fail "rc=$RC, lastCycleNumber=$new_lcn (expected 1); tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test T: v8.34.0 — manual class does NOT advance lastCycleNumber --------
# Defensive: only --class cycle commits represent a "completed cycle." Manual
# (operator) commits and release commits don't have cycle semantics.
header "Test T: v8.34.0 — manual ship leaves lastCycleNumber unchanged"
REPO=$(make_repo)
cd "$REPO"
echo "manual change v8.34" >> fixture.txt
echo '{"cycle_id":99,"phase":"ship"}' > "$REPO/.evolve/cycle-state.json"
jq '. + {lastCycleNumber: 5}' "$REPO/.evolve/state.json" > "$REPO/.evolve/state.json.tmp" \
    && mv "$REPO/.evolve/state.json.tmp" "$REPO/.evolve/state.json"
BARE_T="$SCRATCH/remote-test-t-$RANDOM.git"
git init -q --bare "$BARE_T"
git remote add origin "$BARE_T"
git branch -M main
set +e
EVOLVE_SHIP_AUTO_CONFIRM=1 bash scripts/lifecycle/ship.sh --class manual "manual: ad-hoc fix" </dev/null >/tmp/ship-out 2>&1
RC=$?
set -e
final_lcn=$(jq -r '.lastCycleNumber // 0' "$REPO/.evolve/state.json")
if [ "$RC" = "0" ] && [ "$final_lcn" = "5" ]; then
    pass "v8.34.0: manual ship preserved lastCycleNumber=5 (no advance)"
else
    fail "rc=$RC, lastCycleNumber=$final_lcn (expected 5 unchanged); tail: $(tail -5 /tmp/ship-out)"
fi
cd "$REPO_ROOT"

# --- Test U: v8.34.0 — actual-diff footer appended to commit message --------
# Records file list + line counts in `git log` so reviewers can spot
# message-vs-diff divergence. Not a blocking layer — just transparency.
header "Test U: v8.34.0 — actual-diff footer appended to cycle commit"
REPO=$(make_repo)
cd "$REPO"
echo "diff transparency test" >> fixture.txt
echo "new file content" > newfile.txt
seed_audit "$REPO" "PASS"
echo '{"cycle_id":2,"phase":"ship"}' > "$REPO/.evolve/cycle-state.json"
BARE_U="$SCRATCH/remote-test-u-$RANDOM.git"
git init -q --bare "$BARE_U"
git remote add origin "$BARE_U"
git branch -M main
set +e
bash scripts/lifecycle/ship.sh "feat: claims do not match" >/tmp/ship-out 2>&1
RC=$?
set -e
last_msg=$(git -C "$REPO" log -1 --format='%B')
if [ "$RC" = "0" ] \
   && echo "$last_msg" | grep -q "## Actual diff (v8.34.0+)" \
   && echo "$last_msg" | grep -qE "Files modified \([0-9]+\)" \
   && echo "$last_msg" | grep -q "fixture.txt" \
   && echo "$last_msg" | grep -q "newfile.txt"; then
    pass "v8.34.0: actual-diff footer appended with file list"
else
    fail "rc=$RC; commit message tail: $(echo "$last_msg" | tail -10)"
fi
cd "$REPO_ROOT"

# --- Test V: v8.34.0 — release class skips diff footer (release commits don't need it) ---
# Release commits are version bumps + CHANGELOG; the file list is structurally
# well-defined (plugin.json, marketplace.json, SKILL.md, README.md, CHANGELOG.md)
# and the footer adds bulk without value.
header "Test V: v8.34.0 — release class skips actual-diff footer"
REPO=$(make_repo)
cd "$REPO"
echo "release content" >> fixture.txt
BARE_V="$SCRATCH/remote-test-v-$RANDOM.git"
git init -q --bare "$BARE_V"
git remote add origin "$BARE_V"
git branch -M main
set +e
bash scripts/lifecycle/ship.sh --class release "release: v9.0.0" >/tmp/ship-out 2>&1
RC=$?
set -e
last_msg=$(git -C "$REPO" log -1 --format='%B')
if [ "$RC" = "0" ] && ! echo "$last_msg" | grep -q "## Actual diff"; then
    pass "v8.34.0: release class commit has no actual-diff footer"
else
    fail "rc=$RC; release commit message: $(echo "$last_msg" | head -10)"
fi
cd "$REPO_ROOT"

# --- Summary ----------------------------------------------------------------
echo
echo "==========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "==========================================="
[ "$FAIL" -eq 0 ]
