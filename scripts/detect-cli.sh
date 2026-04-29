#!/usr/bin/env bash
#
# detect-cli.sh — Identify which AI coding CLI is currently driving the skill.
#
# Used by skills/evolve-loop/reference/platform-detect.md and CLAUDE.md to
# select the correct platform overlay (tools + runtime). The skill consults
# this script at activation; it can be called from any shell, no env required.
#
# Usage:
#   bash scripts/detect-cli.sh           # prints one of: claude, gemini, codex, unknown
#   bash scripts/detect-cli.sh --json    # prints {"cli":"...","reason":"..."}
#   EVOLVE_PLATFORM=gemini bash scripts/detect-cli.sh
#       # operator override; honoured verbatim if non-empty
#
# Probe order (priority high → low):
#   1. EVOLVE_PLATFORM       — explicit operator override
#   2. CLAUDE_CODE_INTERACTIVE / CLAUDE_CODE_SESSION_ID → claude
#   3. GEMINI_CLI / GEMINI_API_KEY → gemini   (only if claude probes failed)
#   4. CODEX_HOME / CODEX_API_KEY → codex
#   5. unknown

set -uo pipefail

EMIT_JSON=0
case "${1:-}" in
    --json) EMIT_JSON=1 ;;
    --help|-h)
        sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
        exit 0
        ;;
    "" ) ;;
    *) echo "[detect-cli] unknown flag: $1" >&2; exit 10 ;;
esac

cli="unknown"
reason="no probe matched"

if [ -n "${EVOLVE_PLATFORM:-}" ]; then
    cli="$EVOLVE_PLATFORM"
    reason="explicit override via EVOLVE_PLATFORM"
elif [ -n "${CLAUDE_CODE_INTERACTIVE:-}" ] || [ -n "${CLAUDE_CODE_SESSION_ID:-}" ]; then
    cli="claude"
    reason="CLAUDE_CODE_* env detected"
elif [ -n "${GEMINI_CLI:-}" ] || [ -n "${GEMINI_API_KEY:-}" ]; then
    cli="gemini"
    reason="GEMINI_* env detected"
elif [ -n "${CODEX_HOME:-}" ] || [ -n "${CODEX_API_KEY:-}" ]; then
    cli="codex"
    reason="CODEX_* env detected"
fi

if [ "$EMIT_JSON" = "1" ]; then
    printf '{"cli":"%s","reason":"%s"}\n' "$cli" "$reason"
else
    echo "$cli"
fi

exit 0
