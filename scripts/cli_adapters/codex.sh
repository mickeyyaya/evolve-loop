#!/usr/bin/env bash
#
# codex.sh — CLI adapter for OpenAI Codex CLI (v8.51.0+).
#
# Mirrors gemini.sh's pattern: HYBRID delegation when claude binary is on PATH,
# DEGRADED same-session execution otherwise. Pipeline runs in either mode.
#
# DESIGN
#
# OpenAI Codex CLI as of 2026-05 lacks the same primitives Gemini lacks:
#   1. Confirmed non-interactive prompt mode interface
#   2. --max-budget-usd cost cap flag
#   3. Subagent dispatch with profile-scoped permissions
#
# Until those primitives exist, the adapter operates in two modes:
#   - HYBRID (claude binary present): delegate to claude.sh for full caps
#   - DEGRADED (claude missing): same-session execution. Pipeline kernel hooks
#     (role-gate, ship-gate, phase-gate-precondition, ledger SHA chain) and
#     v7.9.0+ forgery defenses provide structural safety.
#
# v8.51.0 reframes Codex from tier-3 stub to tier-1 hybrid. Native Codex
# adapter (no claude required) tracked for v8.54.0.
#
# CONTRACT
#
# Inputs (env vars set by subagent-run.sh):
#   PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, CYCLE, WORKSPACE_PATH,
#   STDOUT_LOG, STDERR_LOG, ARTIFACT_PATH
#   Optional: WORKTREE_PATH, VALIDATE_ONLY
#
# Modes:
#   (no args)        — run mode (HYBRID preferred, DEGRADED fallback)
#   --probe          — verify resolved tier; exit 0 always
#   --require-full   — alias for EVOLVE_CODEX_REQUIRE_FULL=1; exit 99 if not HYBRID
#
# Test seam:
#   EVOLVE_CODEX_CLAUDE_PATH (gated by EVOLVE_TESTING=1) — same semantics as
#   the gemini.sh seam.
#
# Exit codes:
#   0     — HYBRID delegation succeeded OR DEGRADED mode completed
#   99    — claude missing AND --require-full set
#  127    — internal error (claude.sh adapter missing in HYBRID mode)

set -uo pipefail

ADAPTER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$ADAPTER_DIR/../.." && pwd)"
CLAUDE_ADAPTER="$ADAPTER_DIR/claude.sh"
PROBE_TOOL="$REPO_ROOT/scripts/utility/probe-tool.sh"
CAP_CHECK="$ADAPTER_DIR/_capability-check.sh"

REQUIRE_FULL="${EVOLVE_CODEX_REQUIRE_FULL:-0}"
PROBE_ONLY=0
for arg in "$@"; do
    case "$arg" in
        --require-full) REQUIRE_FULL=1 ;;
        --probe)        PROBE_ONLY=1 ;;
    esac
done

emit_test_seam_warnings() {
    if [ "${EVOLVE_CODEX_CLAUDE_PATH+set}" != "set" ]; then
        return 0
    fi
    if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        echo "[codex-adapter] WARN: test seam active (EVOLVE_CODEX_CLAUDE_PATH=${EVOLVE_CODEX_CLAUDE_PATH:-<empty>}); not for production" >&2
    else
        echo "[codex-adapter] WARN: EVOLVE_CODEX_CLAUDE_PATH set without EVOLVE_TESTING=1 — ignored. Set both to enable the test seam." >&2
    fi
}

detect_claude() {
    if [ "${EVOLVE_CODEX_CLAUDE_PATH+set}" = "set" ] && [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ -z "$EVOLVE_CODEX_CLAUDE_PATH" ]; then return 1; fi
        if [ -x "$EVOLVE_CODEX_CLAUDE_PATH" ]; then echo "$EVOLVE_CODEX_CLAUDE_PATH"; return 0; fi
        return 1
    fi
    if [ -x "$PROBE_TOOL" ]; then
        bash "$PROBE_TOOL" claude --quiet 2>/dev/null
        return $?
    fi
    command -v claude >/dev/null 2>&1
}

# NATIVE mode: invoke codex binary directly when available.
# Test seam: EVOLVE_CODEX_BINARY overrides detection (gated by EVOLVE_TESTING=1).
detect_codex_native() {
    if [ "${EVOLVE_CODEX_BINARY+set}" = "set" ] && [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ -z "$EVOLVE_CODEX_BINARY" ]; then return 1; fi
        if [ -x "$EVOLVE_CODEX_BINARY" ]; then echo "$EVOLVE_CODEX_BINARY"; return 0; fi
        return 1
    fi
    command -v codex >/dev/null 2>&1 || return 1
    command -v codex
}

emit_native_test_seam_warnings() {
    [ "${EVOLVE_CODEX_BINARY+set}" = "set" ] || return 0
    if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        echo "[codex-adapter] WARN: native test seam active (EVOLVE_CODEX_BINARY=${EVOLVE_CODEX_BINARY:-<empty>}); not for production" >&2
    else
        echo "[codex-adapter] WARN: EVOLVE_CODEX_BINARY set without EVOLVE_TESTING=1 — ignored." >&2
    fi
}

print_missing_claude_error() {
    cat >&2 <<'MSGEOF'
[codex-adapter] ERROR (exit 99): claude binary not found AND --require-full opted in

You requested HYBRID-only mode (--require-full or EVOLVE_CODEX_REQUIRE_FULL=1)
but the `claude` binary is not on PATH. Install Claude CLI from
https://claude.ai/code OR remove --require-full to fall back to DEGRADED mode.

DEGRADED mode is the default. Pipeline kernel hooks and v7.9.0+ forgery
defenses provide structural safety even without a claude subprocess. See
docs/architecture/platform-compatibility.md.
MSGEOF
}

print_degraded_warnings() {
    cat >&2 <<'MSGEOF'
[codex-adapter] DEGRADED MODE active — claude binary not on PATH.

What this means:
  - subprocess_isolation: subagents run in your current Codex session.
  - budget_cap: no native cost cap. Consider EVOLVE_RUN_TIMEOUT for a bound.
  - sandbox: bash-invocation gates only.
  - profile_permissions: Codex cannot apply per-subagent tool restrictions.
  - challenge_token: post-hoc verification.

What still works (pipeline-level safety):
  - role-gate, ship-gate, phase-gate-precondition: enforce on bash commands.
  - Ledger SHA chain: tamper-evident.
  - Forgery defenses (v7.9.0+): artifact content checks, git diff substance,
    state.json checksum, .sh write protection, anti-forgery prompt.

Upgrade to HYBRID by installing claude binary: https://claude.ai/code
Enforce HYBRID-only with --require-full or EVOLVE_CODEX_REQUIRE_FULL=1.
MSGEOF
}

# --- Mode: --probe -----------------------------------------------------------
if [ "$PROBE_ONLY" = "1" ]; then
    emit_test_seam_warnings
    if [ -x "$CAP_CHECK" ]; then
        tier=$(bash "$CAP_CHECK" codex 2>/dev/null | jq -r '.quality_tier' 2>/dev/null)
    else
        tier="unknown"
    fi
    if detect_claude >/dev/null 2>&1; then
        echo "[codex-adapter] PROBE OK: claude binary present; resolved tier=$tier" >&2
    else
        echo "[codex-adapter] PROBE OK: claude binary missing; resolved tier=$tier (DEGRADED mode active)" >&2
    fi
    exit 0
fi

# --- Mode: VALIDATE_ONLY (dry-run from cmd_validate_profile) -----------------
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
    echo "[codex-adapter] VALIDATE_ONLY=1 — not executing" >&2
    echo "[codex-adapter] resolved: cli=codex model=${RESOLVED_MODEL:-unset} source=${CLI_RESOLUTION_SOURCE:-unset} cap_budget_native=${CAP_BUDGET_NATIVE:-unset}" >&2
    exit 0
fi

# --- Mode: run (decide NATIVE, HYBRID, or DEGRADED) -------------------------
emit_test_seam_warnings
emit_native_test_seam_warnings

# NATIVE mode: codex binary present AND capabilities enable non_interactive_prompt.
# Takes priority over HYBRID so operators with both binaries get true native execution.
_CODEX_NATIVE_CAP="false"
if [ -f "$ADAPTER_DIR/codex.capabilities.json" ] && command -v jq >/dev/null 2>&1; then
    _CODEX_NATIVE_CAP=$(jq -r '.supports.non_interactive_prompt | if . == null then "false" else tostring end' \
        "$ADAPTER_DIR/codex.capabilities.json" 2>/dev/null || echo "false")
fi
if [ "$_CODEX_NATIVE_CAP" = "true" ]; then
    _CODEX_BIN=$(detect_codex_native 2>/dev/null) || _CODEX_BIN=""
    if [ -n "$_CODEX_BIN" ] && [ -x "$_CODEX_BIN" ] && [ -n "${PROMPT_FILE:-}" ]; then
        echo "[codex-adapter] NATIVE mode: invoking codex binary directly (cli_resolution=native)" >&2
        exec "$_CODEX_BIN" < "$PROMPT_FILE"
    fi
fi

if detect_claude >/dev/null 2>&1; then
    if [ ! -x "$CLAUDE_ADAPTER" ]; then
        echo "[codex-adapter] ERROR (exit 127): claude.sh adapter missing: $CLAUDE_ADAPTER" >&2
        exit 127
    fi
    echo "[codex-adapter] HYBRID mode: delegating to claude.sh" >&2
    exec bash "$CLAUDE_ADAPTER"
fi

if [ "$REQUIRE_FULL" = "1" ]; then
    print_missing_claude_error
    exit 99
fi

# DEGRADED mode (default).
print_degraded_warnings

: "${PROFILE_PATH:?codex-degraded: PROFILE_PATH unset}"
: "${PROMPT_FILE:?codex-degraded: PROMPT_FILE unset}"
: "${ARTIFACT_PATH:?codex-degraded: ARTIFACT_PATH unset}"
: "${STDOUT_LOG:?codex-degraded: STDOUT_LOG unset}"
: "${STDERR_LOG:?codex-degraded: STDERR_LOG unset}"

{
    echo '{"degraded_mode": true, "adapter": "codex",'
    echo '"reason": "claude binary missing; pipeline runs in same-session execution",'
    echo '"cost_usd": 0, "duration_ms": 0,'
    echo '"prompt_file": "'"$PROMPT_FILE"'",'
    echo '"artifact_path": "'"$ARTIFACT_PATH"'"}'
} > "$STDOUT_LOG"

echo "[codex-adapter] DEGRADED mode complete; LLM is expected to write $ARTIFACT_PATH directly" >&2
echo "[codex-adapter] subagent-run.sh's artifact verification will confirm the write" >&2
exit 0
