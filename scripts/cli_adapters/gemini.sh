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

# --- Mode: run (decide HYBRID or DEGRADED) -----------------------------------
emit_test_seam_warnings
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
