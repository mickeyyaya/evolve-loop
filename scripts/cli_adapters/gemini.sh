#!/usr/bin/env bash
#
# gemini.sh — HYBRID CLI adapter for Google Gemini CLI users (v8.15.0+).
#
# DESIGN
#
# Gemini CLI lacks three primitives evolve-loop's runtime depends on:
#   1. Non-interactive prompt mode (no `gemini -p` as of 2026-04)
#   2. --max-budget-usd cost cap
#   3. Subagent dispatch with profile-scoped permissions
#
# The forgery precedent (docs/incidents/gemini-forgery.md) shows what happens
# when evolve-loop runs directly on Gemini without these primitives:
# fabricated artifacts, hallucinated git history, forged ledger entries.
# Rather than rebuild that surface, this adapter delegates to claude.sh.
# Gemini provides the conversational front-end; Claude provides the isolated
# execution back-end. Both binaries must be installed.
#
# CONTRACT
#
# Inputs (env vars set by subagent-run.sh — passed straight through to claude.sh):
#   PROFILE_PATH, RESOLVED_MODEL, PROMPT_FILE, CYCLE, WORKSPACE_PATH,
#   STDOUT_LOG, STDERR_LOG, ARTIFACT_PATH
#   Optional: WORKTREE_PATH, VALIDATE_ONLY
#
# Modes:
#   (no args)   — run mode; verifies claude is available, then exec's claude.sh
#   --probe     — verify claude availability without requiring env vars;
#                 exit 0 if found, 99 if not
#
# Test seam:
#   EVOLVE_GEMINI_CLAUDE_PATH overrides the claude probe.
#     unset             → normal probe via scripts/probe-tool.sh
#     empty string      → simulate "claude not found" (forced missing)
#     non-empty path    → use that path verbatim (must be executable)
#
# Exit codes:
#   0     — delegated to claude.sh (run mode), exit code is claude.sh's
#   0     — claude binary present (--probe mode)
#   99    — claude binary not available; user must install Claude CLI
#  127    — internal error (claude.sh adapter missing)

set -uo pipefail

ADAPTER_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$ADAPTER_DIR/../.." && pwd)"
CLAUDE_ADAPTER="$ADAPTER_DIR/claude.sh"
PROBE_TOOL="$REPO_ROOT/scripts/probe-tool.sh"

# --- Probe whether claude is available ---------------------------------------
# Returns 0 (and prints path to stdout) if found, non-zero otherwise.
# Honours the EVOLVE_GEMINI_CLAUDE_PATH testing seam.
detect_claude() {
    if [ "${EVOLVE_GEMINI_CLAUDE_PATH+set}" = "set" ]; then
        # Test seam: explicit override.
        if [ -z "$EVOLVE_GEMINI_CLAUDE_PATH" ]; then
            return 1  # explicitly forced missing
        fi
        if [ -x "$EVOLVE_GEMINI_CLAUDE_PATH" ]; then
            echo "$EVOLVE_GEMINI_CLAUDE_PATH"
            return 0
        fi
        return 1  # path provided but not executable
    fi
    # Normal probe via probe-tool.sh (canonical multi-location lookup).
    if [ -x "$PROBE_TOOL" ]; then
        bash "$PROBE_TOOL" claude --quiet 2>/dev/null
        return $?
    fi
    # Fallback if probe-tool.sh is missing (should not happen in a healthy repo).
    command -v claude >/dev/null 2>&1
}

# --- Error message printer ---------------------------------------------------
print_missing_claude_error() {
    cat >&2 <<'EOF'
[gemini-adapter] ERROR (exit 99): claude binary not found

evolve-loop on Gemini CLI uses a HYBRID DRIVER pattern: Gemini provides the
conversational front-end, but actual subagent execution runs through Claude
to preserve the trust boundary (sandbox-exec, role-gate, ship-gate, phase-
gate-precondition). This requires Claude CLI to be installed.

Install Claude CLI from https://claude.ai/code, then verify:
  command -v claude

Why this is required:
  Gemini CLI lacks non-interactive prompt mode, --max-budget-usd, and
  subagent dispatch — primitives evolve-loop's safety story depends on.
  See docs/incidents/gemini-forgery.md for the historical incident that
  motivated this design choice, and skills/evolve-loop/reference/gemini-
  runtime.md for the architectural rationale.

Alternative: run evolve-loop from Claude Code directly by setting the
profile's "cli" field to "claude" and invoking from a Claude Code session.
EOF
}

# --- Mode: --probe -----------------------------------------------------------
if [ "${1:-}" = "--probe" ]; then
    if detect_claude >/dev/null 2>&1; then
        echo "[gemini-adapter] OK: claude binary available; hybrid driver ready" >&2
        exit 0
    fi
    print_missing_claude_error
    exit 99
fi

# --- Mode: run (delegate to claude.sh) ---------------------------------------
if ! detect_claude >/dev/null 2>&1; then
    print_missing_claude_error
    exit 99
fi

if [ ! -x "$CLAUDE_ADAPTER" ]; then
    echo "[gemini-adapter] ERROR (exit 127): claude.sh adapter missing or not executable: $CLAUDE_ADAPTER" >&2
    exit 127
fi

echo "[gemini-adapter] hybrid-mode: delegating to claude.sh (Gemini drives, Claude executes)" >&2
exec bash "$CLAUDE_ADAPTER" "$@"
