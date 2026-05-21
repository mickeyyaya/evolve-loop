#!/usr/bin/env bash
#
# agy.sh — CLI adapter for Antigravity CLI (agy binary) users (v10.19.0+).
#
# DESIGN
#
# Antigravity CLI (agy) supports non-interactive prompt mode via `agy -p`
# with `--dangerously-skip-permissions` and `--add-dir` for workspace access.
# agy emits plain text only — no JSON envelope. This adapter appends a
# hardcoded zero-cost envelope (cost_blind:true) as the last STDOUT_LOG line
# to satisfy the subagent-run.sh contract (grep "usage" + tail -1).
#
# Modes:
#   NATIVE  — agy binary on PATH: invoke `agy -p` directly, append zero-cost envelope.
#   HYBRID  — claude on PATH, agy missing: delegate to claude.sh (full caps).
#   DEGRADED — neither binary: emit stub JSON, exit 0 (same-session orchestration).
#
# Operators who require NATIVE or HYBRID (e.g., production with budget caps)
# can opt into hard-fail with `--require-full` or `EVOLVE_AGY_REQUIRE_FULL=1`.
# Default is graceful degradation.
#
# CONTRACT
#
# Inputs (env vars set by subagent-run.sh — passed straight through to claude.sh
# in HYBRID mode; consumed in-process in DEGRADED/NATIVE modes):
#   PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, CYCLE, WORKSPACE_PATH,
#   STDOUT_LOG, STDERR_LOG, ARTIFACT_PATH
#   Optional: WORKTREE_PATH, VALIDATE_ONLY
#
# Modes:
#   (no args)       — run mode; prefer NATIVE, then HYBRID, fall back to DEGRADED
#   --probe         — verify capability resolution; exit 0 always (logs tier)
#   --require-full  — alias for EVOLVE_AGY_REQUIRE_FULL=1; exit 99 if not NATIVE/HYBRID
#
# Test seams (EVOLVE_TESTING=1 only):
#   EVOLVE_AGY_BINARY      — override agy binary path for NATIVE detection
#   EVOLVE_AGY_CLAUDE_PATH — override claude binary path for HYBRID detection
#   EVOLVE_AGY_CAP_FILE    — override agy.capabilities.json path
#
# Exit codes:
#   0   — NATIVE/HYBRID/DEGRADED completed
#   99  — agy+claude missing AND --require-full set
#  127  — claude.sh adapter missing in HYBRID mode

set -uo pipefail

# --- Optional bridge delegation (DEFAULT-ON) --------------------------------
# When `bridge` is installed AND reports schema_version=1, this adapter
# delegates to `bridge launch --cli=agy`. Bridge picks up
# PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, etc. from env automatically
# (its env-var fallback contract).
#
# Default: ENABLED. Force-disable with `EVOLVE_USE_BRIDGE=0` (e.g., for CI
# bit-for-bit reproducibility, or to debug the native adapter path).
#
# If bridge is NOT installed, or schema mismatches, the existing native
# adapter runs unchanged. Zero regression for users who don't install bridge.
#
# Bridge is a user-installable, optional dependency. See
# docs/architecture/cli-adapters.md for installation instructions.
if [[ "${EVOLVE_USE_BRIDGE:-1}" != "0" ]] && command -v bridge >/dev/null 2>&1; then
  _bridge_schema=$(bridge --json version 2>/dev/null | jq -r '.schema_version' 2>/dev/null || echo "")
  if [[ "$_bridge_schema" == "1" ]]; then
    # Per-CLI support probe — does bridge actually have a working driver for
    # this CLI on this host? tier=none means manifest missing OR underlying
    # binary not on PATH. In either case, exec'ing bridge would fail; fall
    # through to the native adapter which may handle it differently.
    _bridge_tier=$(bridge --json probe --cli=agy 2>/dev/null | jq -r '.results[0].tier // "none"' 2>/dev/null || echo "none")
    if [[ "$_bridge_tier" != "none" && "$_bridge_tier" != "null" ]]; then
      echo "[agy] delegating to bridge launch --cli=agy (tier=$_bridge_tier; default-on; EVOLVE_USE_BRIDGE=0 to disable)" >&2
      exec bridge launch --cli=agy
    else
      echo "[agy] WARN: bridge installed but cli=agy tier=$_bridge_tier (manifest missing or binary not on PATH); falling back to native adapter" >&2
    fi
  else
    echo "[agy] WARN: bridge installed but schema_version='$_bridge_schema' (expected 1); falling back to native adapter" >&2
  fi
  unset _bridge_schema _bridge_tier
fi


ADAPTER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$ADAPTER_DIR/../.." && pwd)"
CLAUDE_ADAPTER="$ADAPTER_DIR/claude.sh"
PROBE_TOOL="$REPO_ROOT/scripts/utility/probe-tool.sh"
CAP_CHECK="$ADAPTER_DIR/_capability-check.sh"

REQUIRE_FULL="${EVOLVE_AGY_REQUIRE_FULL:-0}"
PROBE_ONLY=0
for arg in "$@"; do
    case "$arg" in
        --require-full) REQUIRE_FULL=1 ;;
        --probe)        PROBE_ONLY=1 ;;
    esac
done

# --- Test-seam gate -----------------------------------------------------------
emit_test_seam_warnings() {
    if [ "${EVOLVE_AGY_CLAUDE_PATH+set}" = "set" ]; then
        if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
            echo "[agy-adapter] WARN: claude test seam active (EVOLVE_AGY_CLAUDE_PATH=${EVOLVE_AGY_CLAUDE_PATH:-<empty>}); not for production" >&2
        else
            echo "[agy-adapter] WARN: EVOLVE_AGY_CLAUDE_PATH set without EVOLVE_TESTING=1 — ignored." >&2
        fi
    fi
}

emit_native_test_seam_warnings() {
    [ "${EVOLVE_AGY_BINARY+set}" = "set" ] || return 0
    if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        echo "[agy-adapter] WARN: native test seam active (EVOLVE_AGY_BINARY=${EVOLVE_AGY_BINARY:-<empty>}); not for production" >&2
    else
        echo "[agy-adapter] WARN: EVOLVE_AGY_BINARY set without EVOLVE_TESTING=1 — ignored." >&2
    fi
}

detect_claude() {
    if [ "${EVOLVE_AGY_CLAUDE_PATH+set}" = "set" ] && [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ -z "$EVOLVE_AGY_CLAUDE_PATH" ]; then return 1; fi
        if [ -x "$EVOLVE_AGY_CLAUDE_PATH" ]; then echo "$EVOLVE_AGY_CLAUDE_PATH"; return 0; fi
        return 1
    fi
    if [ -x "$PROBE_TOOL" ]; then
        bash "$PROBE_TOOL" claude --quiet 2>/dev/null
        return $?
    fi
    command -v claude >/dev/null 2>&1
}

detect_agy_native() {
    if [ "${EVOLVE_AGY_BINARY+set}" = "set" ] && [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ -z "$EVOLVE_AGY_BINARY" ]; then return 1; fi
        if [ -x "$EVOLVE_AGY_BINARY" ]; then echo "$EVOLVE_AGY_BINARY"; return 0; fi
        return 1
    fi
    command -v agy >/dev/null 2>&1 || return 1
    command -v agy
}

print_missing_agy_error() {
    cat >&2 <<'MSGEOF'
[agy-adapter] ERROR (exit 99): neither agy nor claude binary found AND --require-full opted in

You requested NATIVE/HYBRID-only mode (--require-full or EVOLVE_AGY_REQUIRE_FULL=1)
but neither `agy` nor `claude` binary is on PATH. Install Antigravity CLI from
https://antigravity.ai OR Claude CLI from https://claude.ai/code, OR remove
--require-full to fall back to DEGRADED mode (same-session execution).

DEGRADED mode is the default. Pipeline kernel hooks provide structural safety
even without a subprocess — see docs/architecture/platform-compatibility.md.
MSGEOF
}

print_degraded_warnings() {
    cat >&2 <<'MSGEOF'
[agy-adapter] DEGRADED MODE active — neither agy nor claude binary on PATH.

What this means:
  - subprocess_isolation: subagents run in your current session (not isolated
    subprocesses). Builder + Auditor share session memory.
  - budget_cap: no native cost cap. Consider EVOLVE_RUN_TIMEOUT for an
    external bound.
  - sandbox: bash invocations only; OS-level sandboxing unavailable.
  - profile_permissions: no per-subagent tool restrictions.
  - challenge_token: post-hoc artifact verification only.

What still works (pipeline-level structural safety):
  - role-gate, ship-gate, phase-gate-precondition: enforce on bash commands.
  - Ledger SHA chain: tamper-evident, prev_hash linked.
  - cycle-state.json: kernel-managed, advances only via cycle-state.sh.

To upgrade to NATIVE, install agy binary:
  https://antigravity.ai
To upgrade to HYBRID (full caps via claude subprocess), install claude binary:
  https://claude.ai/code
To enforce NATIVE/HYBRID-only, re-run with --require-full or
EVOLVE_AGY_REQUIRE_FULL=1.
MSGEOF
}

# --- Mode: --probe -----------------------------------------------------------
if [ "$PROBE_ONLY" = "1" ]; then
    emit_test_seam_warnings
    if [ -x "$CAP_CHECK" ]; then
        tier=$(bash "$CAP_CHECK" antigravity 2>/dev/null | jq -r '.quality_tier' 2>/dev/null) || tier="unknown"
    else
        tier="unknown"
    fi
    if detect_agy_native >/dev/null 2>&1; then
        echo "[agy-adapter] PROBE OK: agy binary present; resolved tier=$tier (NATIVE mode)" >&2
    elif detect_claude >/dev/null 2>&1; then
        echo "[agy-adapter] PROBE OK: agy binary missing, claude present; resolved tier=$tier (HYBRID mode)" >&2
    else
        echo "[agy-adapter] PROBE OK: neither agy nor claude binary; resolved tier=$tier (DEGRADED mode)" >&2
    fi
    exit 0
fi

# --- Mode: VALIDATE_ONLY (dry-run from cmd_validate_profile) -----------------
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
    echo "[agy-adapter] VALIDATE_ONLY=1 — not executing" >&2
    echo "[agy-adapter] resolved: cli=antigravity model=${RESOLVED_MODEL:-unset} source=${CLI_RESOLUTION_SOURCE:-unset} cap_budget_native=${CAP_BUDGET_NATIVE:-unset}" >&2
    exit 0
fi

# --- Mode: run (decide NATIVE, HYBRID, or DEGRADED) -------------------------
emit_test_seam_warnings
emit_native_test_seam_warnings

# NATIVE mode: agy binary present AND capabilities enable non_interactive_prompt.
_AGY_NATIVE_CAP="false"
_AGY_CAP_FILE="$ADAPTER_DIR/agy.capabilities.json"
if [ "${EVOLVE_TESTING:-0}" = "1" ] && [ -n "${EVOLVE_AGY_CAP_FILE:-}" ]; then
    _AGY_CAP_FILE="$EVOLVE_AGY_CAP_FILE"
fi
if [ -f "$_AGY_CAP_FILE" ] && command -v jq >/dev/null 2>&1; then
    _AGY_NATIVE_CAP=$(jq -r '.supports.non_interactive_prompt | if . == null then "false" else tostring end' \
        "$_AGY_CAP_FILE" 2>/dev/null || echo "false")
fi
if [ "$_AGY_NATIVE_CAP" = "true" ]; then
    _AGY_BIN=$(detect_agy_native 2>/dev/null) || _AGY_BIN=""
    if [ -n "$_AGY_BIN" ] && [ -x "$_AGY_BIN" ] && [ -n "${PROMPT_FILE:-}" ]; then
        : "${STDOUT_LOG:?agy-native: STDOUT_LOG unset}"
        : "${STDERR_LOG:?agy-native: STDERR_LOG unset}"
        PROFILE_PATH="${PROFILE_PATH:-/dev/null}"

        echo "[agy-adapter] NATIVE mode: invoking agy binary directly (cli_resolution=native)" >&2

        _a_start_ms=$(($(date +%s%N 2>/dev/null || python3 -c 'import time;print(int(time.time()*1e9))')/1000000))

        # Build argv — agy emits plain text, no --output-format flag.
        set -- "$_AGY_BIN" -p "$(cat "$PROMPT_FILE")" --dangerously-skip-permissions

        if [ -n "${WORKSPACE_PATH:-}" ] && [ -d "$WORKSPACE_PATH" ]; then
            set -- "$@" --add-dir "$WORKSPACE_PATH"
        fi
        if [ -n "${WORKTREE_PATH:-}" ] && [ -d "$WORKTREE_PATH" ]; then
            set -- "$@" --add-dir "$WORKTREE_PATH"
        fi

        "$@" >"$STDOUT_LOG" 2>"$STDERR_LOG"
        _a_rc=$?

        _a_end_ms=$(($(date +%s%N 2>/dev/null || python3 -c 'import time;print(int(time.time()*1e9))')/1000000))
        _a_dur_ms=$((_a_end_ms - _a_start_ms))

        # Append zero-cost envelope as the LAST line of STDOUT_LOG.
        # subagent-run.sh greps for "usage" then takes tail -1.
        # agy emits plain text only — no JSON stats to translate.
        _a_envelope=$(printf '{"duration_ms":%d,"num_turns":1,"total_cost_usd":0,"cost_blind":true,"adapter":"agy","usage":{"input_tokens":0,"output_tokens":0,"cache_read_input_tokens":0,"cache_creation_input_tokens":0},"modelUsage":{}}' "$_a_dur_ms")
        echo "" >> "$STDOUT_LOG"
        echo "$_a_envelope" >> "$STDOUT_LOG"

        echo "[agy-adapter] NATIVE done: rc=$_a_rc dur=${_a_dur_ms}ms cost_blind=true" >&2
        exit "$_a_rc"
    fi
fi

# HYBRID mode: agy missing, but claude is on PATH.
if detect_claude >/dev/null 2>&1; then
    if [ ! -x "$CLAUDE_ADAPTER" ]; then
        echo "[agy-adapter] ERROR (exit 127): claude.sh adapter missing: $CLAUDE_ADAPTER" >&2
        exit 127
    fi
    echo "[agy-adapter] HYBRID mode: delegating to claude.sh" >&2
    exec bash "$CLAUDE_ADAPTER"
fi

# Neither agy nor claude binary found.
if [ "$REQUIRE_FULL" = "1" ]; then
    print_missing_agy_error
    exit 99
fi

# DEGRADED mode (default).
print_degraded_warnings

: "${PROFILE_PATH:?agy-degraded: PROFILE_PATH unset}"
: "${PROMPT_FILE:?agy-degraded: PROMPT_FILE unset}"
: "${ARTIFACT_PATH:?agy-degraded: ARTIFACT_PATH unset}"
: "${STDOUT_LOG:?agy-degraded: STDOUT_LOG unset}"
: "${STDERR_LOG:?agy-degraded: STDERR_LOG unset}"

{
    echo '{"degraded_mode": true, "adapter": "agy",'
    echo '"reason": "neither agy nor claude binary found; pipeline runs in same-session execution",'
    echo '"cost_usd": 0, "duration_ms": 0,'
    echo '"cost_blind": true,'
    echo '"prompt_file": "'"$PROMPT_FILE"'",'
    echo '"artifact_path": "'"$ARTIFACT_PATH"'"}'
} > "$STDOUT_LOG"

echo "[agy-adapter] DEGRADED mode complete; LLM is expected to write $ARTIFACT_PATH directly" >&2
echo "[agy-adapter] subagent-run.sh's artifact verification will confirm the write" >&2
exit 0
