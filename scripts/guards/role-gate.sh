#!/usr/bin/env bash
#
# role-gate.sh — PreToolUse hook for Claude Code Edit/Write tool calls (v8.13.1).
#
# Enforces: "only the active agent for this phase can write only to its
# allowlisted paths." If no cycle is in progress, transparent passthrough.
#
# Decision tree:
#   1. Read JSON payload from stdin → extract tool_input.file_path.
#   2. If .evolve/cycle-state.json missing → ALLOW (no cycle in progress).
#   3. If file_path under always-safe dirs (/tmp, /var/folders, $HOME/.claude/) → ALLOW.
#   4. If file_path under cycle_state.active_worktree → ALLOW (Builder's worktree).
#   5. Match canonical file_path against per-phase allowlist:
#        calibrate/research/discover  →  .evolve/runs/cycle-N/*
#        build                        →  worktree/** + .evolve/runs/cycle-N/*
#        audit                        →  .evolve/runs/cycle-N/audit-report.md|handoff-auditor.json
#        ship                         →  version-bump files only
#        learn                        →  .evolve/runs/cycle-N/orchestrator-report.md
#                                        + .evolve/instincts/lessons/*.yaml
#                                        + .evolve/state.json
#   6. Match → ALLOW. No match → DENY with explicit message.
#
# Bypass: EVOLVE_BYPASS_ROLE_GATE=1 (logged WARN; emergency only).
#
# Exit codes:
#   0 — allow
#   2 — deny (Claude Code surfaces stderr to LLM)

set -uo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
GUARDS_LOG="$REPO_ROOT/.evolve/guards.log"
CYCLE_STATE_FILE="${EVOLVE_CYCLE_STATE_FILE:-$REPO_ROOT/.evolve/cycle-state.json}"

mkdir -p "$(dirname "$GUARDS_LOG")"

log() {
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$ts] [role-gate] $*" >> "$GUARDS_LOG"
}

deny() {
    local msg="$1"
    log "DENY: $msg"
    echo "[role-gate] DENY: $msg" >&2
    echo "[role-gate] To bypass (emergency only): export EVOLVE_BYPASS_ROLE_GATE=1" >&2
    exit 2
}

# ---- Read payload ----------------------------------------------------------

PAYLOAD="$(cat || true)"
if [ -z "$PAYLOAD" ]; then
    log "no-payload (manual invocation?); ALLOW"
    exit 0
fi

FILE_PATH=""
if command -v jq >/dev/null 2>&1; then
    FILE_PATH=$(echo "$PAYLOAD" | jq -r '.tool_input.file_path // empty' 2>/dev/null || true)
fi
if [ -z "$FILE_PATH" ]; then
    FILE_PATH=$(echo "$PAYLOAD" | sed -n 's/.*"file_path"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -1)
fi

if [ -z "$FILE_PATH" ]; then
    log "no file_path in payload; ALLOW"
    exit 0
fi

# ---- Bypass switch ---------------------------------------------------------

if [ "${EVOLVE_BYPASS_ROLE_GATE:-0}" = "1" ]; then
    log "WARN: EVOLVE_BYPASS_ROLE_GATE=1 — bypassing for: ${FILE_PATH}"
    echo "[role-gate] WARN: bypass active; gate not enforcing" >&2
    exit 0
fi

# ---- No cycle → ALLOW ------------------------------------------------------

if [ ! -f "$CYCLE_STATE_FILE" ]; then
    log "no cycle-state; ALLOW ${FILE_PATH}"
    exit 0
fi

# ---- Canonicalize file_path ------------------------------------------------
# Claude Code passes absolute paths usually, but a relative path could appear
# if a tool is mid-cd. Normalize against the cwd of the parent (= invoking
# session). We can read cwd from payload too, fall back to repo root.

CWD=""
if command -v jq >/dev/null 2>&1; then
    CWD=$(echo "$PAYLOAD" | jq -r '.cwd // empty' 2>/dev/null || true)
fi
[ -z "$CWD" ] && CWD="$REPO_ROOT"

ABS_PATH="$FILE_PATH"
case "$ABS_PATH" in
    /*) ;;
    *)  ABS_PATH="$CWD/$FILE_PATH" ;;
esac

# Canonicalize (strip /./, /../). The file may not exist yet (Write creates
# new files); use a parent-dir realpath strategy.
canonicalize() {
    local p="$1"
    if [ -e "$p" ]; then
        # File exists — straight realpath if available, otherwise pwd.
        if command -v realpath >/dev/null 2>&1; then
            realpath "$p" 2>/dev/null && return
        fi
        (cd "$(dirname "$p")" 2>/dev/null && printf '%s/%s\n' "$(pwd)" "$(basename "$p")") 2>/dev/null
    else
        # File doesn't exist — canonicalize parent.
        local parent="$(dirname "$p")"
        local base="$(basename "$p")"
        local cparent
        if [ -d "$parent" ]; then
            if command -v realpath >/dev/null 2>&1; then
                cparent=$(realpath "$parent" 2>/dev/null)
            fi
            [ -z "$cparent" ] && cparent=$(cd "$parent" 2>/dev/null && pwd)
        fi
        [ -z "$cparent" ] && cparent="$parent"
        printf '%s/%s\n' "$cparent" "$base"
    fi
}

CANON_PATH="$(canonicalize "$ABS_PATH")"
[ -z "$CANON_PATH" ] && CANON_PATH="$ABS_PATH"

# ---- Always-safe directories ------------------------------------------------

case "$CANON_PATH" in
    /tmp/*|/private/tmp/*|/var/folders/*|/private/var/folders/*)
        log "ALLOW (transient dir): $CANON_PATH"
        exit 0
        ;;
esac

if [ -n "${HOME:-}" ]; then
    case "$CANON_PATH" in
        "$HOME"/.claude/*)
            log "ALLOW (user .claude dir): $CANON_PATH"
            exit 0
            ;;
    esac
fi

# ---- Read cycle state ------------------------------------------------------

if ! command -v jq >/dev/null 2>&1; then
    # Without jq we can't reliably parse — fail open with WARN, since the
    # surrounding system already runs jq for ship-gate.
    log "WARN: jq missing; cannot parse cycle-state — ALLOW $CANON_PATH"
    exit 0
fi

PHASE=$(jq -r '.phase // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)
ACTIVE_WT=$(jq -r '.active_worktree // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)
WORKSPACE_PATH=$(jq -r '.workspace_path // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)
CYCLE_ID=$(jq -r '.cycle_id // empty' "$CYCLE_STATE_FILE" 2>/dev/null || true)

# Defensive: strip trailing slash so case-glob matching is robust (audit LOW-2).
ACTIVE_WT="${ACTIVE_WT%/}"
WORKSPACE_PATH="${WORKSPACE_PATH%/}"

# v8.58.0 Layer I: canonicalize the worktree boundary (the target is already
# canonicalized at line 130). Asymmetric symlink resolution between target and
# boundary caused DENY false-positives on legitimate writes when the worktree
# path included a symlink (e.g., macOS $TMPDIR is /var/folders → /private/var/
# folders). Now both sides resolve symlinks before the glob comparison.
if [ -n "$ACTIVE_WT" ] && [ -d "$ACTIVE_WT" ] && command -v realpath >/dev/null 2>&1; then
    _canonical_wt=$(realpath "$ACTIVE_WT" 2>/dev/null) && [ -n "$_canonical_wt" ] && ACTIVE_WT="$_canonical_wt"
    unset _canonical_wt
fi

if [ -z "$PHASE" ] || [ -z "$WORKSPACE_PATH" ]; then
    log "cycle-state malformed (phase=$PHASE, workspace=$WORKSPACE_PATH); ALLOW"
    exit 0
fi

# Resolve workspace to absolute path.
case "$WORKSPACE_PATH" in
    /*) ABS_WORKSPACE="$WORKSPACE_PATH" ;;
    *)  ABS_WORKSPACE="$REPO_ROOT/$WORKSPACE_PATH" ;;
esac

# ---- Active worktree → always allow (Builder writes there) -----------------

if [ -n "$ACTIVE_WT" ]; then
    case "$CANON_PATH" in
        "$ACTIVE_WT"|"$ACTIVE_WT"/*)
            log "ALLOW (active worktree): $CANON_PATH"
            exit 0
            ;;
    esac
fi

# ---- Per-phase allowlist ---------------------------------------------------

# Helper: case-glob match $1 against any of the remaining args.
match_any() {
    local p="$1"; shift
    local pat
    for pat in "$@"; do
        # shellcheck disable=SC2254
        case "$p" in $pat) return 0 ;; esac
    done
    return 1
}

allow_for_phase() {
    local phase="$1"
    local path="$2"
    local ws="$3"

    # Workspace dir applies in all phases as a baseline (the cycle's own files).
    if match_any "$path" "$ws" "$ws/*"; then
        return 0
    fi

    case "$phase" in
        calibrate|research|discover)
            # Workspace-only — already handled above.
            return 1
            ;;
        build)
            # Workspace + worktree (active_worktree handled above).
            return 1
            ;;
        audit)
            # Audit can only write its report and handoff under the workspace.
            # The workspace baseline above is broader than this, but we keep
            # the case here so future tightening (eg. read_only_repo + only
            # audit-report.md) is a localized edit.
            match_any "$path" \
                "$ws/audit-report.md" \
                "$ws/handoff-auditor.json" \
                "$ws/audit-*.md" \
                "$ws/audit-*.json"
            ;;
        ship)
            # Version-bump files only. Git ops go via ship.sh (ship-gate enforces).
            match_any "$path" \
                "$REPO_ROOT/.claude-plugin/plugin.json" \
                "$REPO_ROOT/.claude-plugin/marketplace.json" \
                "$REPO_ROOT/CHANGELOG.md" \
                "$REPO_ROOT/README.md" \
                "$REPO_ROOT/skills/evolve-loop/SKILL.md" \
                "$REPO_ROOT/.agents/skills/evolve-loop/SKILL.md"
            ;;
        learn)
            match_any "$path" \
                "$ws/orchestrator-report.md" \
                "$REPO_ROOT/.evolve/instincts/lessons/*.yaml" \
                "$REPO_ROOT/.evolve/state.json"
            ;;
        *)
            return 1
            ;;
    esac
}

if allow_for_phase "$PHASE" "$CANON_PATH" "$ABS_WORKSPACE"; then
    log "ALLOW phase=$PHASE cycle=$CYCLE_ID path=$CANON_PATH"
    exit 0
fi

deny "phase=$PHASE cycle=$CYCLE_ID disallows write to: $CANON_PATH (workspace=$ABS_WORKSPACE worktree=${ACTIVE_WT:-<none>})"
