#!/usr/bin/env bash
#
# postedit-validate.sh — PostToolUse hook for Edit|Write (v8.13.3).
#
# Validates the file that was just edited/written by extension:
#   .json → jq empty (parse check)
#   .sh   → bash -n (syntax check)
#   .py   → python3 -m py_compile (compile check)
# Other extensions: silent no-op.
#
# This hook NEVER blocks (PostToolUse fires AFTER the tool ran). On a
# detected issue, it emits a stderr WARN that Claude Code surfaces to the
# LLM, prompting an immediate re-edit. The /insights audit identified the
# `declare -A` and regex `\b` truncation bug classes — both would be caught
# instantly by `bash -n`.
#
# Bypass: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1 (logged WARN; emergency only —
# expected only if validators are themselves broken).
#
# Exit codes:
#   0 — always (PostToolUse can't block; we report via stderr only)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GUARDS_LOG="$REPO_ROOT/.evolve/guards.log"

# mkdir + log writes are best-effort — read-only sandboxes (auditor profile,
# CI environments) make .evolve/ unwritable. We never want logging failure
# to leak to stderr because Claude Code surfaces stderr to the LLM as a
# spurious WARN. Audit cycle 8204 DEFECT-1 fix.
mkdir -p "$(dirname "$GUARDS_LOG")" 2>/dev/null || true

log() {
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    # Bash processes redirections left-to-right: 2>/dev/null MUST appear
    # BEFORE >> "$GUARDS_LOG", otherwise bash's "Operation not permitted"
    # message escapes to the original stderr before the redirect activates.
    # Audit cycle 8205 DEFECT-1 fix (RC2 had the wrong order).
    echo "[$ts] [postedit-validate] $*" 2>/dev/null >> "$GUARDS_LOG" || true
}

warn_to_llm() {
    # PostToolUse stderr is visible to the LLM via Claude Code's reminder mechanism.
    echo "[postedit-validate] $*" >&2
}

# ---- Read payload ----------------------------------------------------------

PAYLOAD="$(cat || true)"
[ -n "$PAYLOAD" ] || { log "no-payload; skip"; exit 0; }

if [ "${EVOLVE_BYPASS_POSTEDIT_VALIDATE:-0}" = "1" ]; then
    log "WARN: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1; bypassing"
    exit 0
fi

FILE_PATH=""
if command -v jq >/dev/null 2>&1; then
    FILE_PATH=$(echo "$PAYLOAD" | jq -r '.tool_input.file_path // empty' 2>/dev/null || true)
fi
if [ -z "$FILE_PATH" ]; then
    FILE_PATH=$(echo "$PAYLOAD" | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
fi

[ -n "$FILE_PATH" ] || { log "no file_path in payload; skip"; exit 0; }
[ -f "$FILE_PATH" ] || { log "file does not exist (deleted?): $FILE_PATH; skip"; exit 0; }

# ---- Dispatch by extension -------------------------------------------------

case "$FILE_PATH" in
    *.json)
        # jq empty exits 0 on valid JSON, non-zero on parse error.
        if command -v jq >/dev/null 2>&1; then
            if err=$(jq empty "$FILE_PATH" 2>&1); then
                log "OK: $FILE_PATH (json)"
            else
                log "WARN: invalid JSON in $FILE_PATH: $err"
                warn_to_llm "WARN: just-edited file $FILE_PATH does NOT parse as valid JSON: $err"
                warn_to_llm "  Re-read and fix before continuing. Bypass: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1."
            fi
        else
            log "WARN: jq not installed; cannot validate $FILE_PATH"
        fi
        ;;
    *.sh)
        # bash -n is a syntax check; doesn't execute the script.
        if err=$(bash -n "$FILE_PATH" 2>&1); then
            log "OK: $FILE_PATH (bash syntax)"
        else
            log "WARN: bash syntax error in $FILE_PATH: $err"
            warn_to_llm "WARN: just-edited file $FILE_PATH has a bash syntax error: $err"
            warn_to_llm "  Common causes: bash 4+ features (declare -A, mapfile) on a 3.2 target; unbalanced quotes; missing fi/done."
            warn_to_llm "  Re-read and fix before continuing. Bypass: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1."
        fi
        ;;
    *.py)
        # py_compile is a compile-check; doesn't execute.
        local_python=""
        if command -v python3 >/dev/null 2>&1; then
            local_python="python3"
        elif command -v python >/dev/null 2>&1; then
            local_python="python"
        fi
        if [ -n "$local_python" ]; then
            if err=$("$local_python" -m py_compile "$FILE_PATH" 2>&1); then
                log "OK: $FILE_PATH (py_compile)"
                # py_compile leaves __pycache__/ behind; clean up the bytecode for the just-checked file.
                rm -rf "$(dirname "$FILE_PATH")/__pycache__/$(basename "$FILE_PATH" .py)".*.pyc 2>/dev/null || true
            else
                log "WARN: python compile error in $FILE_PATH: $err"
                warn_to_llm "WARN: just-edited file $FILE_PATH has a Python compile error: $err"
                warn_to_llm "  Re-read and fix before continuing. Bypass: EVOLVE_BYPASS_POSTEDIT_VALIDATE=1."
            fi
        else
            log "WARN: python not installed; cannot validate $FILE_PATH"
        fi
        ;;
    *)
        # Other extensions (md, yaml, txt, etc.): silent no-op.
        :
        ;;
esac

# Always exit 0 — PostToolUse cannot block.
exit 0
