#!/usr/bin/env bash
#
# dispatch-parallel-test.sh — Unit tests for the dispatch-parallel command
# in scripts/dispatch/subagent-run.sh.
#
# dispatch-parallel reads the parent agent's profile.parallel_subtasks array
# and runs each subtask as a parallel worker via fanout-dispatch.sh, then
# aggregates worker artifacts via aggregator.sh into the canonical phase
# artifact (e.g., scout-report.md).
#
# Tests use an EVOLVE_FANOUT_TEST_EXECUTOR override so workers are simple
# stubs (no real LLM calls). The kernel-level worker recursion (cmd_run
# handling <role>-worker-<name>) is a separate follow-up surgery; this test
# verifies the dispatch + aggregate pipeline in isolation.
#
# Tests cover:
#   1. subagent-run.sh recognizes 'dispatch-parallel' subcommand
#   2. happy path: 3-subtask scout → workers run, aggregator produces canonical artifact
#   3. profile missing parallel_subtasks → fail with clear error
#   4. one worker fails → aggregator NOT called (or aggregator fails); dispatcher exits non-zero
#   5. parent ledger entry written with kind="agent_fanout"
#
# Bash 3.2 compatible per CLAUDE.md.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/dispatch/subagent-run.sh"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

fresh_workspace() {
    mktemp -d -t dispatch-parallel-test.XXXXXX
}

# Test environment: a temp PROFILES_DIR with a fake scout profile that
# declares 3 parallel subtasks. The test executor is a stub that writes
# token+body to the worker artifact path.
setup_env() {
    local ws="$1"
    mkdir -p "$ws/profiles"
    mkdir -p "$ws/workspace"
    # Minimal scout profile with parallel_subtasks.
    # Omit output_artifact so dispatch-parallel falls back to
    # <workspace>/<agent>-report.md (keeps the test hermetic).
    cat > "$ws/profiles/scout.json" <<JSON
{
  "cli": "claude",
  "model_tier_default": "sonnet",
  "max_budget_usd": 0.5,
  "parallel_subtasks": [
    { "name": "codebase", "prompt_template": "Focus on codebase analysis for cycle {cycle}." },
    { "name": "research", "prompt_template": "Web research for cycle {cycle}." },
    { "name": "evals",    "prompt_template": "Eval design for cycle {cycle}." }
  ],
  "allowed_tools": [],
  "sandbox": { "enabled": false }
}
JSON
    # Stub executor: receives WORKER_NAME, ARTIFACT, TOKEN as env. Writes a
    # valid artifact (token + body).
    cat > "$ws/stub-executor.sh" <<'BASH'
#!/usr/bin/env bash
set -e
mkdir -p "$(dirname "$EVOLVE_FANOUT_WORKER_ARTIFACT")"
{
    echo "<!-- challenge-token: $EVOLVE_FANOUT_WORKER_TOKEN -->"
    echo "Worker $EVOLVE_FANOUT_WORKER_NAME completed."
    echo "Cycle: $EVOLVE_FANOUT_CYCLE"
} > "$EVOLVE_FANOUT_WORKER_ARTIFACT"
BASH
    chmod +x "$ws/stub-executor.sh"
}

# --- Test 1: subcommand recognized -------------------------------------------
header "Test 1: subagent-run.sh recognizes 'dispatch-parallel' subcommand"
WS=$(fresh_workspace)
rc=0
"$SCRIPT" dispatch-parallel 2>"$WS/stderr" >/dev/null || rc=$?
# Should fail (no agent passed), but with a usage error, not "unknown subcommand".
if [ "$rc" -ne 0 ] && ! grep -qi "unknown\|invalid subcommand" "$WS/stderr"; then
    pass "dispatch-parallel subcommand recognized (failed for missing args, not unknown)"
else
    fail_ "dispatch-parallel not recognized; stderr: $(head -3 "$WS/stderr")"
fi
rm -rf "$WS"

# --- Test 2: happy path ------------------------------------------------------
header "Test 2: 3-subtask scout → workers run, aggregator produces canonical artifact"
WS=$(fresh_workspace)
setup_env "$WS"
mkdir -p "$WS/workspace/cycle-1"
rc=0
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
EVOLVE_LEDGER_OVERRIDE="$WS/ledger.jsonl" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >"$WS/stdout" 2>"$WS/stderr" || rc=$?
if [ "$rc" -eq 0 ]; then
    pass "dispatch-parallel happy path → exit 0"
else
    fail_ "expected exit 0, got $rc"
    echo "    stderr:"
    sed 's/^/      /' "$WS/stderr" | head -10
fi
# Each worker should have written its artifact.
# v8.23.0: count only worker artifacts (scout-codebase.md, scout-research.md, ...).
# The workers/ dir also holds cache-prefix.md (Task C) which is a shared input,
# not a per-worker output, so we exclude it from this count.
WORKER_COUNT=$(ls -1 "$WS/workspace/cycle-1/workers/"*.md 2>/dev/null | grep -v cache-prefix.md | wc -l | tr -d ' ')
if [ "$WORKER_COUNT" = "3" ]; then
    pass "3 worker artifacts written"
else
    fail_ "expected 3 worker artifacts, got $WORKER_COUNT"
    ls -la "$WS/workspace/cycle-1/workers/" 2>/dev/null || echo "    (workers dir missing)"
fi
# Aggregator should have produced canonical scout-report.md.
if [ -f "$WS/workspace/cycle-1/scout-report.md" ]; then
    pass "canonical scout-report.md produced"
else
    fail_ "scout-report.md missing"
fi
SECTIONS=$(grep -c "^## Worker:" "$WS/workspace/cycle-1/scout-report.md" 2>/dev/null || echo 0)
if [ "$SECTIONS" = "3" ]; then
    pass "scout-report.md has 3 worker sections"
else
    fail_ "expected 3 worker sections, got $SECTIONS"
fi
rm -rf "$WS"

# --- Test 3: profile missing parallel_subtasks -------------------------------
header "Test 3: profile missing parallel_subtasks → fail with clear error"
WS=$(fresh_workspace)
mkdir -p "$WS/profiles"
mkdir -p "$WS/workspace/cycle-1"
cat > "$WS/profiles/scout.json" <<JSON
{
  "cli": "claude",
  "model_tier_default": "sonnet",
  "output_artifact": ".evolve/runs/cycle-{cycle}/scout-report.md",
  "allowed_tools": [],
  "sandbox": { "enabled": false }
}
JSON
rc=0
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>"$WS/stderr" || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "missing parallel_subtasks → non-zero exit"
else
    fail_ "expected non-zero exit, got 0"
fi
if grep -qi "parallel_subtasks" "$WS/stderr"; then
    pass "stderr mentions parallel_subtasks"
else
    fail_ "stderr should explain missing parallel_subtasks; got: $(cat "$WS/stderr")"
fi
rm -rf "$WS"

# --- Test 4: one worker fails ------------------------------------------------
header "Test 4: one worker fails → dispatcher exits non-zero"
WS=$(fresh_workspace)
setup_env "$WS"
mkdir -p "$WS/workspace/cycle-1"
# Override stub to fail when WORKER_NAME=research.
cat > "$WS/stub-executor.sh" <<'BASH'
#!/usr/bin/env bash
set -e
if [ "$EVOLVE_FANOUT_WORKER_NAME" = "research" ]; then
    echo "simulated worker failure" >&2
    exit 5
fi
mkdir -p "$(dirname "$EVOLVE_FANOUT_WORKER_ARTIFACT")"
{
    echo "<!-- challenge-token: $EVOLVE_FANOUT_WORKER_TOKEN -->"
    echo "Worker $EVOLVE_FANOUT_WORKER_NAME completed."
} > "$EVOLVE_FANOUT_WORKER_ARTIFACT"
BASH
chmod +x "$WS/stub-executor.sh"
rc=0
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
EVOLVE_LEDGER_OVERRIDE="$WS/ledger.jsonl" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>"$WS/stderr" || rc=$?
if [ "$rc" -ne 0 ]; then
    pass "worker failure → dispatcher non-zero exit"
else
    fail_ "expected non-zero exit when worker fails"
fi
rm -rf "$WS"

# --- Test 5: parent ledger entry written -------------------------------------
header "Test 5: parent ledger entry written with kind=agent_fanout"
WS=$(fresh_workspace)
setup_env "$WS"
mkdir -p "$WS/workspace/cycle-1"
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
EVOLVE_LEDGER_OVERRIDE="$WS/ledger.jsonl" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>&1
if [ -f "$WS/ledger.jsonl" ] && grep -q '"kind":"agent_fanout"' "$WS/ledger.jsonl"; then
    pass "ledger contains agent_fanout entry"
else
    fail_ "ledger missing agent_fanout entry"
    echo "    ledger contents:"
    cat "$WS/ledger.jsonl" 2>/dev/null | sed 's/^/      /'
fi
# Ledger entry should reference all 3 worker children.
if grep -q '"workers":\[.*"codebase".*"research".*"evals".*\]' "$WS/ledger.jsonl" 2>/dev/null \
   || grep -q '"worker_count":3' "$WS/ledger.jsonl"; then
    pass "ledger entry references workers"
else
    fail_ "ledger entry should list/count workers"
fi
rm -rf "$WS"

# --- v8.23.0 integration tests (Tasks B + C + D) ----------------------------

# --- Test 6: Task C — cmd_dispatch_parallel writes cache-prefix.md ----------
header "Test 6: dispatch-parallel writes cache-prefix.md to workers/ dir"
WS=$(fresh_workspace)
setup_env "$WS"
mkdir -p "$WS/workspace/cycle-1"
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>&1
PREFIX="$WS/workspace/cycle-1/workers/cache-prefix.md"
if [ -f "$PREFIX" ] && [ -s "$PREFIX" ]; then
    if grep -q "Cache-prefix\|cache-prefix\|Cycle Goal\|Trust Boundary" "$PREFIX"; then
        pass "cache-prefix.md created with expected sections"
    else
        fail_ "cache-prefix.md exists but missing canonical sections"
    fi
else
    fail_ "cache-prefix.md not written"
fi
rm -rf "$WS"

# --- Test 7: Task C — cache-prefix.md is byte-identical across runs (idempotent) -
header "Test 7: cache-prefix.md is deterministic (same cycle/workspace → same SHA)"
WS=$(fresh_workspace)
setup_env "$WS"
mkdir -p "$WS/workspace/cycle-1"
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>&1
SHA1=$(shasum -a 256 "$WS/workspace/cycle-1/workers/cache-prefix.md" 2>/dev/null | awk '{print $1}')
# Re-run dispatch-parallel; prefix should be regenerated identically.
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>&1
SHA2=$(shasum -a 256 "$WS/workspace/cycle-1/workers/cache-prefix.md" 2>/dev/null | awk '{print $1}')
if [ -n "$SHA1" ] && [ "$SHA1" = "$SHA2" ]; then
    pass "cache-prefix bytes deterministic across re-runs"
else
    fail_ "expected identical SHAs; got SHA1=$SHA1 SHA2=$SHA2"
fi
rm -rf "$WS"

# --- Test 8: Task C — EVOLVE_FANOUT_CACHE_PREFIX=0 disables prefix generation -
header "Test 8: EVOLVE_FANOUT_CACHE_PREFIX=0 → no cache-prefix.md created"
WS=$(fresh_workspace)
setup_env "$WS"
mkdir -p "$WS/workspace/cycle-1"
EVOLVE_PROFILES_DIR_OVERRIDE="$WS/profiles" \
EVOLVE_FANOUT_TEST_EXECUTOR="$WS/stub-executor.sh" \
EVOLVE_FANOUT_CACHE_PREFIX=0 \
"$SCRIPT" dispatch-parallel scout 1 "$WS/workspace/cycle-1" >/dev/null 2>&1
if [ ! -f "$WS/workspace/cycle-1/workers/cache-prefix.md" ]; then
    pass "cache-prefix disabled → file not created"
else
    fail_ "cache-prefix.md should NOT exist when feature disabled"
fi
rm -rf "$WS"

# --- Summary -----------------------------------------------------------------
echo
echo "=== Summary ==="
echo "  PASS: $PASS"
echo "  FAIL: $FAIL"
[ "$FAIL" -eq 0 ]
