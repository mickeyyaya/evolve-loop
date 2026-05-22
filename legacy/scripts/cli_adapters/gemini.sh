#!/usr/bin/env bash
#
# gemini.sh — CLI adapter for Google Gemini CLI users (v8.51.0+).
#
# DESIGN
#
# Gemini CLI lacks three primitives evolve-loop's runtime depends on:
#   1. Non-interactive prompt mode (no `gemini -p` as of 2026-04)
#   2. --max-budget-usd cost cap
#   3. Subagent dispatch with profile-scoped permissions
#
# v8.15.0–v8.50.x: HYBRID DRIVER required `claude` binary on PATH; without it,
# the adapter exited 99 and the pipeline could not run.
#
# v8.51.0+: pipeline is CLI-INDEPENDENT. The adapter operates in two modes:
#   - HYBRID (claude binary present): delegate to claude.sh (full caps via
#     subprocess isolation, profile permissions, sandbox, budget cap).
#   - DEGRADED (claude binary missing): same-session execution. Pipeline
#     kernel hooks (role-gate, ship-gate, phase-gate-precondition, ledger SHA
#     chain) AND the v7.9.0+ Gemini Forgery defenses (artifact content
#     checks, git diff substance gate, state.json checksum, .sh write
#     protection, anti-forgery prompt) provide structural safety.
#
# Operators who require full hybrid (e.g., production with budget caps and
# subprocess isolation) can opt back into hard-fail with `--require-full` or
# `EVOLVE_GEMINI_REQUIRE_FULL=1`. Default is graceful degradation.
#
# CONTRACT
#
# Inputs (env vars set by subagent-run.sh — passed straight through to claude.sh
# in HYBRID mode; consumed in-process in DEGRADED mode):
#   PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, CYCLE, WORKSPACE_PATH,
#   STDOUT_LOG, STDERR_LOG, ARTIFACT_PATH
#   Optional: WORKTREE_PATH, VALIDATE_ONLY
#
# Modes:
#   (no args)       — run mode; prefer HYBRID, fall back to DEGRADED
#   --probe         — verify capability resolution; exit 0 always (logs tier)
#   --require-full  — alias for EVOLVE_GEMINI_REQUIRE_FULL=1; exit 99 if not HYBRID
#
# Test seams:
#   EVOLVE_GEMINI_CLAUDE_PATH overrides the claude probe — TESTING ONLY.
#   Honoured only when EVOLVE_TESTING=1. Same semantics as pre-v8.51.
#
# Exit codes:
#   0     — HYBRID delegation succeeded OR DEGRADED mode completed
#   99    — claude missing AND --require-full set; user must install claude OR drop --require-full
#  127    — internal error (claude.sh adapter missing in HYBRID mode)

set -uo pipefail

ADAPTER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$ADAPTER_DIR/../.." && pwd)"
CLAUDE_ADAPTER="$ADAPTER_DIR/claude.sh"
PROBE_TOOL="$REPO_ROOT/scripts/utility/probe-tool.sh"
CAP_CHECK="$ADAPTER_DIR/_capability-check.sh"

# Parse opt-in hard-fail flag.
REQUIRE_FULL="${EVOLVE_GEMINI_REQUIRE_FULL:-0}"
PROBE_ONLY=0
for arg in "$@"; do
    case "$arg" in
        --require-full) REQUIRE_FULL=1 ;;
        --probe)        PROBE_ONLY=1 ;;
    esac
done

# --- Test-seam gate (must run BEFORE detect_claude; --probe wrappers may
# redirect stderr, so warnings emitted here remain visible).
emit_test_seam_warnings() {
    if [ "${EVOLVE_GEMINI_CLAUDE_PATH+set}" != "set" ]; then
        return 0
    fi
    if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        echo "[gemini-adapter] WARN: test seam active (EVOLVE_GEMINI_CLAUDE_PATH=${EVOLVE_GEMINI_CLAUDE_PATH:-<empty>}); not for production" >&2
    else
        echo "[gemini-adapter] WARN: EVOLVE_GEMINI_CLAUDE_PATH set without EVOLVE_TESTING=1 — ignored. Set both to enable the test seam." >&2
    fi
}

detect_claude() {
    if [ "${EVOLVE_GEMINI_CLAUDE_PATH+set}" = "set" ] && [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ -z "$EVOLVE_GEMINI_CLAUDE_PATH" ]; then return 1; fi
        if [ -x "$EVOLVE_GEMINI_CLAUDE_PATH" ]; then echo "$EVOLVE_GEMINI_CLAUDE_PATH"; return 0; fi
        return 1
    fi
    if [ -x "$PROBE_TOOL" ]; then
        bash "$PROBE_TOOL" claude --quiet 2>/dev/null
        return $?
    fi
    command -v claude >/dev/null 2>&1
}

# NATIVE mode: invoke gemini binary directly when available.
# Test seam: EVOLVE_GEMINI_BINARY overrides detection (gated by EVOLVE_TESTING=1).
detect_gemini_native() {
    if [ "${EVOLVE_GEMINI_BINARY+set}" = "set" ] && [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ -z "$EVOLVE_GEMINI_BINARY" ]; then return 1; fi
        if [ -x "$EVOLVE_GEMINI_BINARY" ]; then echo "$EVOLVE_GEMINI_BINARY"; return 0; fi
        return 1
    fi
    command -v gemini >/dev/null 2>&1 || return 1
    command -v gemini
}

emit_native_test_seam_warnings() {
    [ "${EVOLVE_GEMINI_BINARY+set}" = "set" ] || return 0
    if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        echo "[gemini-adapter] WARN: native test seam active (EVOLVE_GEMINI_BINARY=${EVOLVE_GEMINI_BINARY:-<empty>}); not for production" >&2
    else
        echo "[gemini-adapter] WARN: EVOLVE_GEMINI_BINARY set without EVOLVE_TESTING=1 — ignored." >&2
    fi
}

print_missing_claude_error() {
    cat >&2 <<'MSGEOF'
[gemini-adapter] ERROR (exit 99): claude binary not found AND --require-full opted in

You requested HYBRID-only mode (--require-full or EVOLVE_GEMINI_REQUIRE_FULL=1)
but the `claude` binary is not on PATH. Install Claude CLI from
https://claude.ai/code OR remove --require-full to fall back to DEGRADED mode
(same-session execution; pipeline runs with reduced isolation guarantees).

DEGRADED mode is the default. Pipeline kernel hooks and the v7.9.0+ Gemini
Forgery defenses provide structural safety even without a claude subprocess —
see docs/architecture/platform-compatibility.md and docs/incidents/gemini-forgery.md.
MSGEOF
}

print_degraded_warnings() {
    cat >&2 <<'MSGEOF'
[gemini-adapter] DEGRADED MODE active — claude binary not on PATH.

What this means:
  - subprocess_isolation: subagents run in your current Gemini session (not
    isolated subprocesses). Builder + Auditor share session memory.
  - budget_cap: no native cost cap. Consider EVOLVE_RUN_TIMEOUT for an
    external bound.
  - sandbox: bash invocations only; OS-level sandboxing unavailable.
  - profile_permissions: Gemini cannot apply per-subagent tool restrictions.
  - challenge_token: post-hoc artifact verification (vs embedded in profile).

What still works (pipeline-level structural safety):
  - role-gate, ship-gate, phase-gate-precondition: enforce on bash commands.
  - Ledger SHA chain: tamper-evident, prev_hash linked.
  - Gemini Forgery defenses (v7.9.0+): artifact content checks, git diff
    substance gate, state.json checksum, .sh write protection, anti-forgery
    prompt inoculation.
  - cycle-state.json: kernel-managed, advances only via cycle-state.sh.

To upgrade to HYBRID (full caps), install claude binary:
  https://claude.ai/code
To enforce HYBRID-only, re-run with --require-full or
EVOLVE_GEMINI_REQUIRE_FULL=1.
MSGEOF
}

# --- Mode: --probe -----------------------------------------------------------
if [ "$PROBE_ONLY" = "1" ]; then
    emit_test_seam_warnings
    # Resolve quality tier via capability check
    if [ -x "$CAP_CHECK" ]; then
        tier=$(bash "$CAP_CHECK" gemini 2>/dev/null | jq -r '.quality_tier' 2>/dev/null)
    else
        tier="unknown"
    fi
    if detect_claude >/dev/null 2>&1; then
        echo "[gemini-adapter] PROBE OK: claude binary present; resolved tier=$tier" >&2
    else
        echo "[gemini-adapter] PROBE OK: claude binary missing; resolved tier=$tier (DEGRADED mode active)" >&2
    fi
    exit 0
fi

# --- Mode: VALIDATE_ONLY (dry-run from cmd_validate_profile) -----------------
# Emit resolved env and exit 0 without invoking the real CLI.
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
    echo "[gemini-adapter] VALIDATE_ONLY=1 — not executing" >&2
    echo "[gemini-adapter] resolved: cli=gemini model=${RESOLVED_MODEL:-unset} source=${CLI_RESOLUTION_SOURCE:-unset} cap_budget_native=${CAP_BUDGET_NATIVE:-unset}" >&2
    exit 0
fi

# --- Mode: run (decide NATIVE, HYBRID, or DEGRADED) -------------------------
emit_test_seam_warnings
emit_native_test_seam_warnings

# NATIVE mode: gemini binary present AND capabilities enable non_interactive_prompt.
# Takes priority over HYBRID so operators with both binaries get true native execution.
_GEMINI_NATIVE_CAP="false"
_GEMINI_CAP_FILE="$ADAPTER_DIR/gemini.capabilities.json"
if [ "${EVOLVE_TESTING:-0}" = "1" ] && [ -n "${EVOLVE_GEMINI_CAP_FILE:-}" ]; then
    _GEMINI_CAP_FILE="$EVOLVE_GEMINI_CAP_FILE"
fi
if [ -f "$_GEMINI_CAP_FILE" ] && command -v jq >/dev/null 2>&1; then
    _GEMINI_NATIVE_CAP=$(jq -r '.supports.non_interactive_prompt | if . == null then "false" else tostring end' \
        "$_GEMINI_CAP_FILE" 2>/dev/null || echo "false")
fi
if [ "$_GEMINI_NATIVE_CAP" = "true" ]; then
    _GEMINI_BIN=$(detect_gemini_native 2>/dev/null) || _GEMINI_BIN=""
    if [ -n "$_GEMINI_BIN" ] && [ -x "$_GEMINI_BIN" ] && [ -n "${PROMPT_FILE:-}" ]; then
        # Strict on the redirect targets (production fails loud); soft default
        # on RESOLVED_MODEL/PROFILE_PATH so test harnesses (predicate 005, 011)
        # that exercise the dispatch shell without setting these still work.
        : "${STDOUT_LOG:?gemini-native: STDOUT_LOG unset}"
        : "${STDERR_LOG:?gemini-native: STDERR_LOG unset}"
        RESOLVED_MODEL="${RESOLVED_MODEL:-gemini-3.1-pro-preview}"
        PROFILE_PATH="${PROFILE_PATH:-/dev/null}"

        echo "[gemini-adapter] NATIVE mode: invoking gemini binary directly (cli_resolution=native, model=$RESOLVED_MODEL)" >&2

        _g_argv=("$_GEMINI_BIN" -p "$(cat "$PROMPT_FILE")" -m "$RESOLVED_MODEL" \
                 --output-format json --approval-mode yolo --skip-trust)

        if [ -n "${WORKSPACE_PATH:-}" ] && [ -d "$WORKSPACE_PATH" ]; then
            _g_argv+=(--include-directories "$WORKSPACE_PATH")
        fi
        if [ -n "${WORKTREE_PATH:-}" ] && [ -d "$WORKTREE_PATH" ]; then
            _g_argv+=(--include-directories "$WORKTREE_PATH")
        fi

        _g_start_ms=$(($(date +%s%N 2>/dev/null || python3 -c 'import time;print(int(time.time()*1e9))')/1000000))
        _g_raw="${WORKSPACE_PATH:-/tmp}/${AGENT:-phase}-gemini-raw.json"
        "${_g_argv[@]}" >"$_g_raw" 2>"$STDERR_LOG"
        _g_rc=$?
        _g_end_ms=$(($(date +%s%N 2>/dev/null || python3 -c 'import time;print(int(time.time()*1e9))')/1000000))
        _g_dur_ms=$((_g_end_ms - _g_start_ms))

        if [ ! -s "$_g_raw" ]; then
            echo "[gemini-adapter] ERROR: gemini produced no stdout (rc=$_g_rc, dur=${_g_dur_ms}ms)" >&2
            : > "$STDOUT_LOG"
            exit "${_g_rc:-1}"
        fi

        # Translate gemini stats → claude-style usage envelope.
        # Pricing for gemini-3.1-pro-preview: $2/M input, $12/M output (<=200k ctx).
        # Source: https://devtk.ai/en/models/gemini-3-1-pro/ (verified 2026-05-15).
        # Note: cache_read_input_tokens billed at full rate here; gemini's actual
        # cache pricing is lower, so cost is slightly over-estimated.
        _g_in_price=$(awk 'BEGIN{print 2.0/1000000.0}')
        _g_out_price=$(awk 'BEGIN{print 12.0/1000000.0}')

        _claude_envelope=$(jq -c \
            --arg model "$RESOLVED_MODEL" \
            --argjson in_price "$_g_in_price" \
            --argjson out_price "$_g_out_price" \
            --argjson dur "$_g_dur_ms" '
            (.stats.models | to_entries[0].value.tokens) as $t |
            ($t.input // 0) as $itok |
            ($t.candidates // 0) as $otok |
            ($t.cached // 0) as $ctok |
            ($itok * $in_price + $otok * $out_price) as $cost |
            {
                duration_ms: $dur,
                num_turns: 1,
                total_cost_usd: $cost,
                gemini_error: (.error // null),
                usage: {
                    input_tokens: $itok,
                    output_tokens: $otok,
                    cache_read_input_tokens: $ctok,
                    cache_creation_input_tokens: 0,
                    thinking_tokens: ($t.thoughts // 0),
                    tool_tokens: ($t.tool // 0)
                },
                modelUsage: {
                    ($model): {
                        inputTokens: $itok,
                        outputTokens: $otok,
                        cacheReadInputTokens: $ctok,
                        cacheCreationInputTokens: 0,
                        webSearchRequests: 0,
                        costUSD: $cost,
                        contextWindow: 2000000,
                        maxOutputTokens: 65536
                    }
                }
            }' "$_g_raw" 2>/dev/null)

        if [ -z "$_claude_envelope" ]; then
            echo "[gemini-adapter] WARN: failed to translate gemini JSON; emitting zero-cost stub" >&2
            _claude_envelope=$(jq -nc --arg model "$RESOLVED_MODEL" --argjson dur "$_g_dur_ms" '{
                duration_ms: $dur, num_turns: 1, total_cost_usd: 0,
                gemini_translate_error: true,
                usage: {input_tokens:0, output_tokens:0, cache_read_input_tokens:0, cache_creation_input_tokens:0},
                modelUsage: {($model): {inputTokens:0, outputTokens:0, cacheReadInputTokens:0, cacheCreationInputTokens:0, webSearchRequests:0, costUSD:0, contextWindow:2000000, maxOutputTokens:65536}}
            }')
        fi

        # STDOUT_LOG = (a) raw response text + (b) claude-style usage envelope as
        # the LAST line. subagent-run.sh greps for `"usage"` and takes tail -1.
        {
            jq -r '.response // ""' "$_g_raw" 2>/dev/null || cat "$_g_raw"
            echo
            echo "$_claude_envelope"
        } > "$STDOUT_LOG"

        echo "[gemini-adapter] NATIVE done: rc=$_g_rc dur=${_g_dur_ms}ms cost=\$$(echo "$_claude_envelope" | jq -r '.total_cost_usd')" >&2
        exit "$_g_rc"
    fi
fi

if detect_claude >/dev/null 2>&1; then
    # HYBRID mode
    if [ ! -x "$CLAUDE_ADAPTER" ]; then
        echo "[gemini-adapter] ERROR (exit 127): claude.sh adapter missing: $CLAUDE_ADAPTER" >&2
        exit 127
    fi
    echo "[gemini-adapter] HYBRID mode: delegating to claude.sh" >&2
    exec bash "$CLAUDE_ADAPTER"
fi

# claude binary missing.
if [ "$REQUIRE_FULL" = "1" ]; then
    print_missing_claude_error
    exit 99
fi

# DEGRADED mode (default).
print_degraded_warnings

# In DEGRADED mode the calling LLM (Gemini) is expected to produce the
# artifact directly using its file-write tools. Our job is to validate that
# the artifact was produced and emit a stub stdout log so the pipeline can
# proceed. We DO NOT make an LLM call here — there's no `gemini -p` to call.
# The orchestrator prompt instructs the LLM to write the artifact during the
# same-session conversation; subagent-run.sh's verify_artifact then checks it.
#
# Required env vars (set by subagent-run.sh):
: "${PROFILE_PATH:?gemini-degraded: PROFILE_PATH unset}"
: "${PROMPT_FILE:?gemini-degraded: PROMPT_FILE unset}"
: "${ARTIFACT_PATH:?gemini-degraded: ARTIFACT_PATH unset}"
: "${STDOUT_LOG:?gemini-degraded: STDOUT_LOG unset}"
: "${STDERR_LOG:?gemini-degraded: STDERR_LOG unset}"

# Emit a structured stdout log that subagent-run.sh can parse for cost
# accounting (zeroed in degraded mode — no LLM invocation here).
{
    echo '{"degraded_mode": true, "adapter": "gemini",'
    echo '"reason": "claude binary missing; pipeline runs in same-session execution",'
    echo '"cost_usd": 0, "duration_ms": 0,'
    echo '"prompt_file": "'"$PROMPT_FILE"'",'
    echo '"artifact_path": "'"$ARTIFACT_PATH"'"}'
} > "$STDOUT_LOG"

echo "[gemini-adapter] DEGRADED mode complete; LLM is expected to write $ARTIFACT_PATH directly" >&2
echo "[gemini-adapter] subagent-run.sh's artifact verification will confirm the write" >&2

# Exit 0 — pipeline continues. If the LLM didn't write the artifact, the
# pipeline's verify_artifact step (in subagent-run.sh) will catch it and fail
# at the next gate.
exit 0
