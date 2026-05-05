#!/usr/bin/env bash
#
# detect-nested-claude.sh — Probe whether evolve-loop is running inside Claude Code (v8.22.0).
#
# WHY THIS EXISTS
#
# When /evolve-loop is invoked from inside Claude Code (the primary use case
# for the slash-command), the parent process is itself running under
# Anthropic's sandbox-exec profile. macOS Darwin 25.4+'s sandbox kernel
# refuses to apply a NEW sandbox profile when the calling process is already
# sandboxed (sandbox_apply: Operation not permitted, rc=71).
#
# This script provides the canonical detection: returns "nested" if any
# Claude Code parent-env signal is present, "standalone" otherwise. The
# dispatcher (scripts/evolve-loop-dispatch.sh) reads this at startup and
# auto-enables EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 when nested — defense in
# depth alongside SKILL.md's slash-command auto-set.
#
# Usage:
#   bash scripts/detect-nested-claude.sh
#   # → stdout: "nested" or "standalone"
#   # → rc: always 0 (probe never fails)
#
# Detection signals (any one match → nested):
#   CLAUDECODE                — Claude Code's primary env-var beacon
#   CLAUDE_CODE_ENTRYPOINT    — set when Claude Code launches a subprocess
#   CLAUDE_CODE_EXECPATH      — set to the Claude Code binary path
#
# These are checked in order; first match wins. Empty/unset values are
# treated as not-set (standalone).
#
# Backward-compat: callers may also pass --quiet to suppress stdout (rc still 0).

set -uo pipefail

QUIET=0
while [ $# -gt 0 ]; do
    case "$1" in
        --quiet|-q) QUIET=1 ;;
        --help|-h)
            sed -n '2,30p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "detect-nested-claude.sh: unknown arg: $1" >&2
            exit 0
            ;;
    esac
    shift
done

is_set() {
    # A var is "set" if exported AND non-empty. Bash's [-n "${var:-}"] handles both.
    [ -n "${!1:-}" ]
}

if is_set CLAUDECODE || is_set CLAUDE_CODE_ENTRYPOINT || is_set CLAUDE_CODE_EXECPATH; then
    [ "$QUIET" = "0" ] && echo "nested"
else
    [ "$QUIET" = "0" ] && echo "standalone"
fi
exit 0
