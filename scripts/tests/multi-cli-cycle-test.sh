#!/usr/bin/env bash
#
# multi-cli-cycle-test.sh — Verifies that subagent-run.sh dispatch routes each
# phase to the CLI adapter declared in its profile.cli field.
#
# What this test covers (the regression gate the multi-LLM design was missing):
#   - Scout profile with cli=gemini → gemini adapter is invoked
#   - Builder profile with cli=claude → claude adapter is invoked
#   - Auditor profile with cli=codex → codex adapter is invoked
#   - _capability-compose.sh correctly returns the minimum tier
#
# Test design:
#   Uses EVOLVE_PROFILES_DIR_OVERRIDE to inject synthetic profiles with mixed
#   cli fields, then runs `subagent-run.sh --validate-profile <agent>` for each
#   phase. VALIDATE_ONLY=1 causes adapters to emit `[<name>-adapter]` to stderr
#   without making any LLM call. EVOLVE_TESTING=1 + empty EVOLVE_*_CLAUDE_PATH
#   forces gemini/codex adapters into DEGRADED mode, eliminating any dependency
#   on real Gemini or Codex CLIs.
#
# Pass condition: all ASSERTs pass, exit 0.
# Bash 3.2 compatible. No real LLM invocations required.

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUBAGENT_RUN="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
COMPOSE_SH="$REPO_ROOT/scripts/cli_adapters/_capability-compose.sh"

PASS=0
FAIL=0

pass()   { echo "  PASS: $*"; PASS=$((PASS + 1)); }
fail_()  { echo "  FAIL: $*"; FAIL=$((FAIL + 1)); }
header() { echo; echo "=== $* ==="; }

# --- Preflight: required files exist -----------------------------------------
header "Preflight: required scripts exist"

if [ -f "$SUBAGENT_RUN" ]; then
    pass "subagent-run.sh present"
else
    echo "  FATAL: subagent-run.sh missing at $SUBAGENT_RUN" >&2
    exit 2
fi

if [ -f "$COMPOSE_SH" ]; then
    pass "_capability-compose.sh present"
else
    fail_ "_capability-compose.sh missing — create scripts/cli_adapters/_capability-compose.sh"
fi

# --- Temp profiles setup -----------------------------------------------------
TMP_PROFILES=$(mktemp -d)
trap 'rm -rf "$TMP_PROFILES"' EXIT

write_profile() {
    local role="$1" cli="$2"
    cat > "$TMP_PROFILES/${role}.json" <<PROFILE
{
  "name": "${role}",
  "role": "${role}",
  "cli": "${cli}",
  "model_tier_default": "sonnet",
  "allowed_tools": ["Read"],
  "disallowed_tools": [],
  "add_dir": ["."],
  "max_budget_usd": 0.5,
  "max_turns": 10,
  "permission_mode": "bypassPermissions",
  "output_artifact": ".evolve/runs/cycle-{cycle}/${role}-report.md",
  "challenge_token_required": false
}
PROFILE
}

# Base env vars used by all tests.
export EVOLVE_PLUGIN_ROOT="$REPO_ROOT"
export EVOLVE_PROJECT_ROOT="$REPO_ROOT"
export EVOLVE_PROFILES_DIR_OVERRIDE="$TMP_PROFILES"
# Clear the idempotency guard so resolve-roots.sh re-derives from EVOLVE_PLUGIN_ROOT.
unset EVOLVE_RESOLVE_ROOTS_LOADED 2>/dev/null || true

# --- Test 1: scout cli=gemini → [gemini-adapter] invoked ---------------------
header "Test 1: scout profile cli=gemini → gemini adapter invoked"
write_profile scout gemini

LAST_STDERR=$(
    export EVOLVE_TESTING=1
    export EVOLVE_GEMINI_CLAUDE_PATH=""
    bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 1>/dev/null
) || true

if echo "$LAST_STDERR" | grep -q "\[gemini-adapter\]"; then
    pass "scout cli=gemini → [gemini-adapter] found in stderr"
else
    fail_ "scout cli=gemini: expected [gemini-adapter] in stderr; got: $(echo "$LAST_STDERR" | head -2)"
fi

if echo "$LAST_STDERR" | grep -q "\[claude-adapter\]"; then
    fail_ "scout cli=gemini unexpectedly invoked [claude-adapter] (routing leaked to wrong adapter)"
else
    pass "scout cli=gemini: [claude-adapter] correctly absent from stderr"
fi

# --- Test 2: builder cli=claude → [claude-adapter] invoked -------------------
header "Test 2: builder profile cli=claude → claude adapter invoked"
write_profile builder claude

LAST_STDERR=$(
    bash "$SUBAGENT_RUN" --validate-profile builder 2>&1 1>/dev/null
) || true

if echo "$LAST_STDERR" | grep -q "\[claude-adapter\]"; then
    pass "builder cli=claude → [claude-adapter] found in stderr"
else
    fail_ "builder cli=claude: expected [claude-adapter] in stderr; got: $(echo "$LAST_STDERR" | head -2)"
fi

# --- Test 3: auditor cli=codex → [codex-adapter] invoked ---------------------
header "Test 3: auditor profile cli=codex → codex adapter invoked"
write_profile auditor codex

LAST_STDERR=$(
    export EVOLVE_TESTING=1
    export EVOLVE_CODEX_CLAUDE_PATH=""
    bash "$SUBAGENT_RUN" --validate-profile auditor 2>&1 1>/dev/null
) || true

if echo "$LAST_STDERR" | grep -q "\[codex-adapter\]"; then
    pass "auditor cli=codex → [codex-adapter] found in stderr"
else
    fail_ "auditor cli=codex: expected [codex-adapter] in stderr; got: $(echo "$LAST_STDERR" | head -2)"
fi

if echo "$LAST_STDERR" | grep -q "\[claude-adapter\]"; then
    fail_ "auditor cli=codex unexpectedly invoked [claude-adapter]"
else
    pass "auditor cli=codex: [claude-adapter] correctly absent from stderr"
fi

# --- Test 4: degraded quality_tier in gemini adapter stderr ------------------
header "Test 4: gemini degraded mode emits DEGRADED tier"

LAST_STDERR=$(
    export EVOLVE_TESTING=1
    export EVOLVE_GEMINI_CLAUDE_PATH=""
    bash "$SUBAGENT_RUN" --validate-profile scout 2>&1 1>/dev/null
) || true

if echo "$LAST_STDERR" | grep -qi "DEGRADED\|degraded"; then
    pass "gemini adapter in DEGRADED mode emits degraded warning in stderr"
else
    fail_ "gemini adapter missing DEGRADED marker; got: $(echo "$LAST_STDERR" | head -3)"
fi

# --- Test 5: _capability-compose.sh correctness -------------------------------
header "Test 5: _capability-compose.sh returns minimum tier"

if [ ! -f "$COMPOSE_SH" ]; then
    fail_ "_capability-compose.sh missing — skipping compose tests"
else
    result=$(bash "$COMPOSE_SH" full hybrid degraded none 2>/dev/null)
    if [ "$result" = "none" ]; then
        pass "_capability-compose.sh: full hybrid degraded none → none"
    else
        fail_ "_capability-compose.sh: expected 'none', got '$result' for 'full hybrid degraded none'"
    fi

    result=$(bash "$COMPOSE_SH" full hybrid 2>/dev/null)
    if [ "$result" = "hybrid" ]; then
        pass "_capability-compose.sh: full hybrid → hybrid"
    else
        fail_ "_capability-compose.sh: expected 'hybrid', got '$result' for 'full hybrid'"
    fi

    result=$(bash "$COMPOSE_SH" full full full 2>/dev/null)
    if [ "$result" = "full" ]; then
        pass "_capability-compose.sh: full full full → full"
    else
        fail_ "_capability-compose.sh: expected 'full', got '$result' for 'full full full'"
    fi

    result=$(bash "$COMPOSE_SH" degraded 2>/dev/null)
    if [ "$result" = "degraded" ]; then
        pass "_capability-compose.sh: single arg degraded → degraded"
    else
        fail_ "_capability-compose.sh: expected 'degraded', got '$result' for single arg"
    fi

    result=$(bash "$COMPOSE_SH" 2>/dev/null)
    if [ "$result" = "none" ]; then
        pass "_capability-compose.sh: no args → none (safe default)"
    else
        fail_ "_capability-compose.sh: expected 'none' for no args, got '$result'"
    fi
fi

# --- Summary ------------------------------------------------------------------
echo
echo "Results: $PASS PASS, $FAIL FAIL"
if [ "$FAIL" -gt 0 ]; then
    echo "FAIL" >&2
    exit 1
fi
echo "PASS — multi-CLI dispatch routing verified"
exit 0
