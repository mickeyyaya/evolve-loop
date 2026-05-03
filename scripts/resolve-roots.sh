#!/usr/bin/env bash
#
# resolve-roots.sh — Dual-root path resolver (v8.18.0).
#
# WHY THIS EXISTS
#
# Pre-v8.18.0, every kernel script computed:
#
#   REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
#
# and used REPO_ROOT for BOTH read paths (scripts/, agents/, .evolve/profiles/)
# AND write paths (.evolve/state.json, .evolve/ledger.jsonl, .evolve/runs/).
# That works in development (cwd == repo). It breaks under the Claude Code
# plugin install pattern:
#
#   - Scripts live at:    ~/.claude/plugins/cache/evolve-loop/evolve-loop/X.Y.Z/
#   - User project at:    ~/projects/<their-app>/
#   - Claude Code session is sandboxed to the user project (cwd)
#   - Writes to ~/.claude/ are blocked as a sensitive path
#
# Symptom: orchestrator subagent fails to write orchestrator-report.md or run
# scripts/cycle-state.sh. The 2026-05-03 boundary incident exposed this.
#
# THE FIX
#
# Two distinct roots, sourced into every kernel script:
#
#   EVOLVE_PLUGIN_ROOT   — where this script lives (read-only resources)
#   EVOLVE_PROJECT_ROOT  — where state/ledger/runs/instincts get written
#
# When invoked from the dev repo, both equal — behavior is unchanged.
# When invoked as a plugin, they diverge cleanly.
#
# RESOLUTION ORDER for EVOLVE_PROJECT_ROOT:
#   1. Explicit env var EVOLVE_PROJECT_ROOT (if non-empty) — for tests / overrides
#   2. `git rev-parse --show-toplevel` from current working directory
#   3. $PWD as last-resort fallback
#
# EVOLVE_PLUGIN_ROOT is always derived from this file's location.
#
# IDEMPOTENCY
#
# Sourcing twice is a no-op (guarded by EVOLVE_RESOLVE_ROOTS_LOADED). Scripts
# can source freely without worrying about double-resolution or env churn.
#
# USAGE
#
#   #!/usr/bin/env bash
#   set -uo pipefail
#   _self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
#   . "$_self/resolve-roots.sh"
#   # Now use:
#   #   $EVOLVE_PLUGIN_ROOT/scripts/foo.sh   (read-only)
#   #   $EVOLVE_PROJECT_ROOT/.evolve/state.json  (writable)
#
# bash 3.2 compatible. No associative arrays. No GNU-only date/sed.

# Idempotency guard — re-sourcing keeps existing values.
#
# `return 0 2>/dev/null || exit 0` handles two cases cleanly:
#   - Sourced (the supported mode): return 0 succeeds, body is skipped.
#   - Executed directly (mistake, but recoverable): return prints to stderr
#     (suppressed) and fails; exit 0 then exits the subshell cleanly so the
#     body never runs twice. This preserves the no-op contract under both
#     invocation styles. Pre-fix used `|| true` which silently re-ran the body.
if [ -n "${EVOLVE_RESOLVE_ROOTS_LOADED:-}" ]; then
    return 0 2>/dev/null || exit 0
fi

# --- EVOLVE_PLUGIN_ROOT — derived from this file's location -----------------
#
# BASH_SOURCE[0] is the path to *this* script (resolve-roots.sh) regardless of
# who sourced it. dirname/.. gives the install root.
__rr_self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
EVOLVE_PLUGIN_ROOT="$(cd "$__rr_self_dir/.." && pwd)"
unset __rr_self_dir

# --- EVOLVE_PROJECT_ROOT — env override, then git toplevel, then $PWD --------
if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
    # Honor explicit override; resolve to absolute (without -P so symlinks
    # behave consistently with how callers passed the path).
    if [ -d "$EVOLVE_PROJECT_ROOT" ]; then
        EVOLVE_PROJECT_ROOT="$(cd "$EVOLVE_PROJECT_ROOT" && pwd)"
    fi
    # If the override points to a non-existent dir, leave it as-is — caller's
    # responsibility. Don't silently rewrite to something else.
else
    # Try git toplevel of cwd. Suppress stderr because outside a git repo
    # `git rev-parse` prints "fatal: not a git repository" to stderr and exits
    # 128 — we don't want that noise in the orchestrator's stream.
    __rr_git_top="$(git rev-parse --show-toplevel 2>/dev/null || true)"
    if [ -n "$__rr_git_top" ] && [ -d "$__rr_git_top" ]; then
        EVOLVE_PROJECT_ROOT="$__rr_git_top"
    else
        # Final fallback: cwd. Resolve to absolute via cd-pwd (handles relative
        # cwd from sub-shells correctly).
        EVOLVE_PROJECT_ROOT="$(pwd)"
    fi
    unset __rr_git_top
fi

# --- Writability indicator — useful for callers that want to fail fast ------
#
# Some scripts (ledger appenders, state writers) want to verify the project
# root is actually writable before attempting work. This is a cheap probe;
# we set the flag once here.
if [ -w "$EVOLVE_PROJECT_ROOT" ]; then
    EVOLVE_PROJECT_WRITABLE=1
else
    EVOLVE_PROJECT_WRITABLE=0
fi

# Mark loaded so re-sourcing is a no-op.
EVOLVE_RESOLVE_ROOTS_LOADED=1

export EVOLVE_PLUGIN_ROOT EVOLVE_PROJECT_ROOT EVOLVE_PROJECT_WRITABLE EVOLVE_RESOLVE_ROOTS_LOADED
