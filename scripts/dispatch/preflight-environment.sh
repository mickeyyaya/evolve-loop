#!/usr/bin/env bash
#
# preflight-environment.sh — Single capability-detection probe (v8.25.0).
#
# WHY THIS EXISTS
#
# Pre-v8.25.0, evolve-loop accumulated 6+ env flags as escape hatches for
# environment-specific failure modes:
#   - EVOLVE_SANDBOX_FALLBACK_ON_EPERM (v8.22.0)
#   - EVOLVE_SKIP_WORKTREE             (v8.23.4)
#   - EVOLVE_BYPASS_SHIP_VERIFY        (legacy emergency override)
#   - EVOLVE_DISPATCH_VERIFY=0         (legacy)
#   - EVOLVE_DISPATCH_REPEAT_THRESHOLD (v8.24.0)
#   - EVOLVE_TASK_MODE                 (v8.13.5)
#
# Each was a reactive fix for a specific symptom. The aggregate effect was
# operator archaeology: "which flag do I set for this failure?"
#
# v8.25.0 collapses the discoverability problem by probing the host
# environment ONCE at dispatcher start and emitting a JSON capability profile.
# The dispatcher reads the profile and auto-configures the auto-relaxable
# flags. Operator overrides via direct edit of $EVOLVE_PROJECT_ROOT/.evolve/
# environment.json — a single observable file instead of remembering N env
# vars.
#
# The probe runs in privileged shell context (no agent-controllable input)
# and produces deterministic output. Tier-1 kernel hooks (phase-gate, ledger
# SHA, role-gate, ship-gate) verify behavior post-execution regardless of
# profile contents — so a malicious profile cannot weaken anti-gaming.
#
# DESIGN PRINCIPLE — Discover, Decide, Log, Verify
#
# Each entry in the profile is the result of:
#   - Discover: a deterministic shell-level probe (test write, command -v, etc.)
#   - Decide:   a rule that maps probe results to env-var assignments
#   - Log:      a one-line summary string in the profile
#   - Verify:   Tier-1 hooks ensure runtime behavior matches expected posture
#
# This script implements Discover + Decide + Log. Verify happens elsewhere.
#
# USAGE
#
#   bash scripts/dispatch/preflight-environment.sh              # probe + emit JSON to stdout
#   bash scripts/dispatch/preflight-environment.sh --write      # also persist to .evolve/environment.json
#   bash scripts/dispatch/preflight-environment.sh --summary    # print human-readable summary instead of JSON
#   bash scripts/dispatch/preflight-environment.sh --help
#
# ENV
#
#   EVOLVE_PROFILE_OVERRIDE=<path>   — read pre-existing profile from path instead
#                                      of probing (used by tests)
#
# EXIT CODES
#
#   0 — profile emitted (probe always succeeds; missing tools/paths are
#       reported in the JSON, not as errors)
#   1 — usage error
#  10 — bad arguments
#
# OUTPUT SCHEMA (v3 — adds inner_sandbox; v2 added worktree_base)
#
#   {
#     "schema_version": 2,
#     "probed_at": "ISO-8601 UTC",
#     "host": {
#       "os": "darwin|linux",
#       "os_version": "<uname -r>",
#       "shell": "<basename $SHELL>"
#     },
#     "claude_code": {
#       "nested": true|false,
#       "claudecode_env": "<value of CLAUDECODE or null>"
#     },
#     "sandbox": {
#       "sandbox_exec_available": true|false,
#       "bwrap_available": true|false,
#       "expected_to_work": true|false,
#       "reason": "<one-line summary>"
#     },
#     "filesystem": {
#       "state_dir_writable": true|false,
#       "in_project_worktrees_writable": true|false,
#       "tmpdir_writable": true|false,
#       "cache_dir_writable": true|false,
#       "state_dir": "<path>"
#     },
#     "cli_binaries": {
#       "claude": "<path or null>",
#       "gemini": "<path or null>",
#       "codex": "<path or null>",
#       "jq": "<path or null>",
#       "git": "<path or null>"
#     },
#     "auto_config": {
#       "EVOLVE_SANDBOX_FALLBACK_ON_EPERM": "0|1",
#       "worktree_base": "<absolute path>",
#       "worktree_base_reason": "<plain-English why this path was selected>",
#       "inner_sandbox": true|false,
#       "inner_sandbox_reason": "<plain-English why true or false>",
#       "reasoning": "<aggregate reasoning>"
#     }
#   }
#
# v2 removed EVOLVE_SKIP_WORKTREE from auto_config: per-cycle worktree
# isolation is non-negotiable; we just relocate the worktree to a sandbox-
# friendly path instead of skipping it.
#
# v3 adds inner_sandbox: when true, claude-adapter wraps phase agents in
# sandbox-exec (defense-in-depth). When false, the wrapping is skipped and
# Tier-1 kernel hooks (phase-gate-precondition, role-gate, ledger-SHA) plus
# the OUTER Claude Code OS sandbox + claude --add-dir provide the trust
# boundary. nested-Claude is the dominant case where inner_sandbox=false
# because the outer OS sandbox already provides isolation and the inner
# sandbox-exec only intersects (cannot expand) what the outer permits.
# Operator overrides via EVOLVE_FORCE_INNER_SANDBOX=1 or EVOLVE_INNER_SANDBOX=0.

set -uo pipefail

__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self

MODE=json
WRITE=0

while [ $# -gt 0 ]; do
    case "$1" in
        --write)   WRITE=1 ;;
        --summary) MODE=summary ;;
        --json)    MODE=json ;;
        --help|-h)
            sed -n '2,80p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*)
            echo "preflight-environment.sh: unknown flag: $1" >&2
            exit 10
            ;;
        *)
            echo "preflight-environment.sh: unexpected positional: $1" >&2
            exit 10
            ;;
    esac
    shift
done

command -v jq >/dev/null 2>&1 || { echo "preflight-environment.sh: jq is required" >&2; exit 1; }

# --- Probes ----------------------------------------------------------------

PROBED_AT=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
HOST_OS=""
case "$OSTYPE" in
    darwin*) HOST_OS=darwin ;;
    linux*)  HOST_OS=linux ;;
    *)       HOST_OS="other" ;;
esac
HOST_OS_VERSION=$(uname -r 2>/dev/null || echo "unknown")
HOST_SHELL=$(basename "${SHELL:-/bin/sh}")

# Nested-Claude detection (delegate to canonical script).
if [ -x "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/detect-nested-claude.sh" ]; then
    NESTED_RESULT=$(bash "$EVOLVE_PLUGIN_ROOT/scripts/dispatch/detect-nested-claude.sh" 2>/dev/null || echo "standalone")
else
    NESTED_RESULT=standalone
fi
NESTED_BOOL=$([ "$NESTED_RESULT" = "nested" ] && echo true || echo false)
CLAUDECODE_VAL="${CLAUDECODE:-}"

# Sandbox capability.
SANDBOX_EXEC_AVAILABLE=false
BWRAP_AVAILABLE=false
command -v sandbox-exec >/dev/null 2>&1 && SANDBOX_EXEC_AVAILABLE=true
command -v bwrap        >/dev/null 2>&1 && BWRAP_AVAILABLE=true

SANDBOX_EXPECTED_TO_WORK=false
SANDBOX_REASON=""
case "$HOST_OS" in
    darwin)
        if [ "$SANDBOX_EXEC_AVAILABLE" = "true" ]; then
            if [ "$NESTED_BOOL" = "true" ]; then
                SANDBOX_EXPECTED_TO_WORK=false
                SANDBOX_REASON="Darwin nested-Claude: sandbox_apply() returns EPERM (rc=71)"
            else
                SANDBOX_EXPECTED_TO_WORK=true
                SANDBOX_REASON="Darwin standalone: sandbox-exec available and parent unsandboxed"
            fi
        else
            SANDBOX_REASON="Darwin: sandbox-exec binary not on PATH"
        fi
        ;;
    linux)
        if [ "$BWRAP_AVAILABLE" = "true" ]; then
            SANDBOX_EXPECTED_TO_WORK=true
            SANDBOX_REASON="Linux: bwrap available; nested namespaces supported"
        else
            SANDBOX_REASON="Linux: bwrap binary not on PATH"
        fi
        ;;
    *)
        SANDBOX_REASON="Unsupported OS: $HOST_OS — sandbox not enforced"
        ;;
esac

# Filesystem writability.
STATE_DIR="$EVOLVE_PROJECT_ROOT/.evolve"
mkdir -p "$STATE_DIR" 2>/dev/null || true

# probe_writable creates the directory if missing (mkdir -p), then attempts a
# touch-and-delete sentinel. Returns 0 if writable, 1 otherwise. Failure to
# create or touch means the path is unsuitable for cycle artifacts.
probe_writable() {
    local dir="$1"
    [ -d "$dir" ] || mkdir -p "$dir" 2>/dev/null
    [ -d "$dir" ] || return 1
    local probe="${dir}/.preflight-probe.$$"
    if { : > "$probe"; } 2>/dev/null; then
        rm -f "$probe" 2>/dev/null
        return 0
    fi
    return 1
}

STATE_DIR_WRITABLE=false
probe_writable "$STATE_DIR" && STATE_DIR_WRITABLE=true

# --- Worktree base selection -----------------------------------------------
#
# Per-cycle worktree isolation is non-negotiable: it provides fast-rollback
# (drop the worktree, no project pollution), audit binding (tree state SHA
# bound to the cycle's specific tree), and forensic preservation (failed
# worktrees retained for inspection until cleanup). We must select a base
# directory that the runtime can actually write to.
#
# Priority order (first writable wins):
#
#   1. EVOLVE_WORKTREE_BASE explicitly set by operator → trusted
#   2. In-project .evolve/worktrees/ AND standalone shell → preferred
#      (best for inspection — cd into worktree from project root)
#   3. $TMPDIR/evolve-loop/<8-char-project-hash>/ → sandbox-friendly default
#      (Darwin nested-Claude allows TMPDIR writes; ephemeral but worktrees
#      are always cleaned up at cycle end anyway)
#   4. ~/Library/Caches/evolve-loop/<hash>/ (macOS) or
#      ${XDG_CACHE_HOME:-~/.cache}/evolve-loop/<hash>/ (Linux) — persistent
#      fallback when TMPDIR is itself blocked
#
# If none of these work, worktree_base is empty and the dispatcher will fail
# loud at startup. Operator can either fix permissions, set
# EVOLVE_WORKTREE_BASE, or set EVOLVE_SKIP_WORKTREE=1 (emergency only,
# operator must explicitly opt in to losing isolation).
PROBE_IN_PROJECT="$EVOLVE_PROJECT_ROOT/.evolve/worktrees"
IN_PROJECT_WORKTREES_WRITABLE=false
TMPDIR_WRITABLE=false
CACHE_DIR_WRITABLE=false

# 8-char hash of project root for namespacing across multiple projects sharing
# the same TMPDIR / cache dir. Use shasum (BSD/macOS) or sha256sum (Linux).
if command -v shasum >/dev/null 2>&1; then
    PROJECT_HASH=$(printf '%s' "$EVOLVE_PROJECT_ROOT" | shasum -a 256 | head -c 8)
elif command -v sha256sum >/dev/null 2>&1; then
    PROJECT_HASH=$(printf '%s' "$EVOLVE_PROJECT_ROOT" | sha256sum | head -c 8)
else
    PROJECT_HASH="default"
fi

WORKTREE_BASE=""
WORKTREE_BASE_REASON=""

# Probe in-project (always probe so the JSON reports state, even when not chosen)
if probe_writable "$PROBE_IN_PROJECT"; then
    IN_PROJECT_WORKTREES_WRITABLE=true
fi

# Probe TMPDIR
PROBE_TMPDIR=""
if [ -n "${TMPDIR:-}" ]; then
    PROBE_TMPDIR="${TMPDIR%/}/evolve-loop/${PROJECT_HASH}"
    if probe_writable "$PROBE_TMPDIR"; then
        TMPDIR_WRITABLE=true
    fi
fi

# Probe cache dir
case "$HOST_OS" in
    darwin) PROBE_CACHE="$HOME/Library/Caches/evolve-loop/${PROJECT_HASH}" ;;
    linux)  PROBE_CACHE="${XDG_CACHE_HOME:-$HOME/.cache}/evolve-loop/${PROJECT_HASH}" ;;
    *)      PROBE_CACHE="$HOME/.cache/evolve-loop/${PROJECT_HASH}" ;;
esac
if probe_writable "$PROBE_CACHE"; then
    CACHE_DIR_WRITABLE=true
fi

# Apply priority order to select the base.
if [ -n "${EVOLVE_WORKTREE_BASE:-}" ]; then
    if probe_writable "$EVOLVE_WORKTREE_BASE"; then
        WORKTREE_BASE="$EVOLVE_WORKTREE_BASE"
        WORKTREE_BASE_REASON="operator-provided EVOLVE_WORKTREE_BASE (writable)"
    else
        WORKTREE_BASE_REASON="WARN: operator-provided EVOLVE_WORKTREE_BASE=$EVOLVE_WORKTREE_BASE is not writable; falling through"
    fi
fi
if [ -z "$WORKTREE_BASE" ] && [ "$NESTED_BOOL" = "false" ] && [ "$IN_PROJECT_WORKTREES_WRITABLE" = "true" ]; then
    WORKTREE_BASE="$PROBE_IN_PROJECT"
    WORKTREE_BASE_REASON="standalone shell: in-project location preferred (easy operator inspection)"
fi
if [ -z "$WORKTREE_BASE" ] && [ "$TMPDIR_WRITABLE" = "true" ]; then
    WORKTREE_BASE="$PROBE_TMPDIR"
    WORKTREE_BASE_REASON="TMPDIR (sandbox-friendly default for nested-Claude)"
fi
if [ -z "$WORKTREE_BASE" ] && [ "$CACHE_DIR_WRITABLE" = "true" ]; then
    WORKTREE_BASE="$PROBE_CACHE"
    WORKTREE_BASE_REASON="user cache dir (TMPDIR unavailable)"
fi
if [ -z "$WORKTREE_BASE" ] && [ "$IN_PROJECT_WORKTREES_WRITABLE" = "true" ]; then
    # Last resort: even nested-Claude can sometimes write in-project
    WORKTREE_BASE="$PROBE_IN_PROJECT"
    WORKTREE_BASE_REASON="in-project (TMPDIR/cache unavailable; isolation degraded if parent sandbox blocks at exec time)"
fi
# If still empty, the dispatcher will fail loud at startup with the operator-
# action message embedded in auto_config.reasoning below.

# CLI binaries.
which_or_null() { command -v "$1" 2>/dev/null || echo ""; }
PATH_CLAUDE=$(which_or_null claude)
PATH_GEMINI=$(which_or_null gemini)
PATH_CODEX=$(which_or_null codex)
PATH_JQ=$(which_or_null jq)
PATH_GIT=$(which_or_null git)

# --- Decide: auto_config ---------------------------------------------------
#
# Rules:
#
# EVOLVE_SANDBOX_FALLBACK_ON_EPERM:
#   - If nested-Claude (Darwin) → set 1 (sandbox-exec startup will EPERM)
#   - Otherwise → leave 0 (use the sandbox normally)
#
# worktree_base:
#   - Already selected above by priority order (operator-set > in-project-
#     standalone > TMPDIR > cache dir > in-project-fallback). The dispatcher
#     exports this as EVOLVE_WORKTREE_BASE; run-cycle.sh reads it.
#
# auto_config NO LONGER recommends EVOLVE_SKIP_WORKTREE. Per-cycle worktree
# isolation is non-negotiable for fast-rollback and audit binding. If no
# writable base could be found, auto_config.reasoning embeds an OPERATOR
# ACTION block; the dispatcher fails loud at startup rather than silently
# skipping the worktree.

AUTO_FALLBACK_ON_EPERM=0
if [ "$NESTED_BOOL" = "true" ]; then
    AUTO_FALLBACK_ON_EPERM=1
fi

# v8.25.1: inner_sandbox decision.
#
# In nested-Claude, the OUTER Claude Code OS sandbox already provides
# process-level isolation. The INNER sandbox-exec wrapper added by
# claude-adapter is a SECOND nested sandbox layer. Because nested
# sandboxes can only intersect (restrict further), never union (expand),
# the inner wrapper:
#   - Adds NO new protection (the outer already provides OS isolation)
#   - DOES introduce execution-time EPERM that the v8.22.0 startup-EPERM
#     fallback can't catch (writes fail mid-build, no auto-recovery)
#
# When inner_sandbox=false, claude-adapter skips the sandbox-exec wrapping.
# Tier-1 kernel hooks remain enforced (phase-gate-precondition, role-gate,
# ledger-SHA verification all run OUTSIDE the sandbox layer entirely).
# claude --add-dir continues to gate Edit/Write tool paths.
#
# Standalone shell with sandbox-exec/bwrap available: inner_sandbox=true
# (defense-in-depth, no nesting issue).
INNER_SANDBOX=true
INNER_SANDBOX_REASON=""
if [ "$NESTED_BOOL" = "true" ]; then
    INNER_SANDBOX=false
    INNER_SANDBOX_REASON="nested-Claude: outer Claude Code OS sandbox + Tier-1 hooks suffice; inner sandbox-exec adds friction without protection (intersect-only nesting)"
elif [ "$SANDBOX_EXPECTED_TO_WORK" = "false" ]; then
    INNER_SANDBOX=false
    INNER_SANDBOX_REASON="sandbox not expected to work on this host: $SANDBOX_REASON"
else
    INNER_SANDBOX_REASON="standalone shell with working sandbox: defense-in-depth enabled"
fi

if [ -n "$WORKTREE_BASE" ]; then
    if [ "$NESTED_BOOL" = "true" ]; then
        AUTO_REASONING="nested-Claude detected. Sandbox startup-fallback enabled (EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1). Worktree relocated to sandbox-friendly path: $WORKTREE_BASE ($WORKTREE_BASE_REASON). Inner sandbox-exec DISABLED ($INNER_SANDBOX_REASON). Tier-1 kernel hooks (phase-gate, role-gate, ledger SHA) keep enforcing."
    else
        AUTO_REASONING="standalone shell. Worktree base: $WORKTREE_BASE ($WORKTREE_BASE_REASON). Inner sandbox-exec: $INNER_SANDBOX ($INNER_SANDBOX_REASON)."
    fi
else
    AUTO_REASONING="ERROR: no writable worktree base. Tried in-project ($IN_PROJECT_WORKTREES_WRITABLE), TMPDIR ($TMPDIR_WRITABLE), cache dir ($CACHE_DIR_WRITABLE). OPERATOR ACTION: set EVOLVE_WORKTREE_BASE to a writable directory, or run from a different shell with broader permissions. Last-resort: EVOLVE_SKIP_WORKTREE=1 (loses per-cycle isolation, NOT recommended)."
fi

# --- Emit ------------------------------------------------------------------

PROFILE_JSON=$(jq -n \
    --arg probed_at        "$PROBED_AT" \
    --arg os               "$HOST_OS" \
    --arg os_version       "$HOST_OS_VERSION" \
    --arg shell            "$HOST_SHELL" \
    --argjson nested       "$NESTED_BOOL" \
    --arg claudecode       "$CLAUDECODE_VAL" \
    --argjson sb_exec      "$SANDBOX_EXEC_AVAILABLE" \
    --argjson bwrap        "$BWRAP_AVAILABLE" \
    --argjson sb_works     "$SANDBOX_EXPECTED_TO_WORK" \
    --arg sb_reason        "$SANDBOX_REASON" \
    --argjson state_w      "$STATE_DIR_WRITABLE" \
    --argjson inproj_w     "$IN_PROJECT_WORKTREES_WRITABLE" \
    --argjson tmp_w        "$TMPDIR_WRITABLE" \
    --argjson cache_w      "$CACHE_DIR_WRITABLE" \
    --arg state_dir        "$STATE_DIR" \
    --arg path_claude      "$PATH_CLAUDE" \
    --arg path_gemini      "$PATH_GEMINI" \
    --arg path_codex       "$PATH_CODEX" \
    --arg path_jq          "$PATH_JQ" \
    --arg path_git         "$PATH_GIT" \
    --arg auto_eperm       "$AUTO_FALLBACK_ON_EPERM" \
    --arg auto_wt_base     "$WORKTREE_BASE" \
    --arg auto_wt_reason   "$WORKTREE_BASE_REASON" \
    --argjson auto_inner   "$INNER_SANDBOX" \
    --arg auto_inner_rsn   "$INNER_SANDBOX_REASON" \
    --arg auto_reason      "$AUTO_REASONING" \
    '{
        schema_version: 3,
        probed_at: $probed_at,
        host: {
            os: $os,
            os_version: $os_version,
            shell: $shell
        },
        claude_code: {
            nested: $nested,
            claudecode_env: (if $claudecode == "" then null else $claudecode end)
        },
        sandbox: {
            sandbox_exec_available: $sb_exec,
            bwrap_available:        $bwrap,
            expected_to_work:       $sb_works,
            reason:                 $sb_reason
        },
        filesystem: {
            state_dir_writable:           $state_w,
            in_project_worktrees_writable: $inproj_w,
            tmpdir_writable:              $tmp_w,
            cache_dir_writable:           $cache_w,
            state_dir:                    $state_dir
        },
        cli_binaries: {
            claude: (if $path_claude == "" then null else $path_claude end),
            gemini: (if $path_gemini == "" then null else $path_gemini end),
            codex:  (if $path_codex  == "" then null else $path_codex  end),
            jq:     (if $path_jq     == "" then null else $path_jq     end),
            git:    (if $path_git    == "" then null else $path_git    end)
        },
        auto_config: {
            EVOLVE_SANDBOX_FALLBACK_ON_EPERM: $auto_eperm,
            worktree_base:                    $auto_wt_base,
            worktree_base_reason:             $auto_wt_reason,
            inner_sandbox:                    $auto_inner,
            inner_sandbox_reason:             $auto_inner_rsn,
            reasoning:                        $auto_reason
        }
    }')

# Optional persist.
if [ "$WRITE" = "1" ]; then
    target="$EVOLVE_PROJECT_ROOT/.evolve/environment.json"
    mkdir -p "$(dirname "$target")" 2>/dev/null || true
    tmp="${target}.tmp.$$"
    if printf '%s\n' "$PROFILE_JSON" > "$tmp" 2>/dev/null && mv -f "$tmp" "$target" 2>/dev/null; then
        echo "[preflight-environment] wrote profile: $target" >&2
    else
        rm -f "$tmp" 2>/dev/null
        echo "[preflight-environment] WARN: could not persist profile to $target (state dir unwritable)" >&2
        # Don't fail — profile output to stdout still works.
    fi
fi

case "$MODE" in
    json)
        printf '%s\n' "$PROFILE_JSON"
        ;;
    summary)
        echo "Environment Profile (probed $PROBED_AT)"
        echo "  Host:             $HOST_OS $HOST_OS_VERSION ($HOST_SHELL)"
        echo "  Nested-Claude:    $NESTED_BOOL"
        echo "  Sandbox works:    $SANDBOX_EXPECTED_TO_WORK ($SANDBOX_REASON)"
        echo "  State writable:   $STATE_DIR_WRITABLE"
        echo "  Worktree probes:  in-project=$IN_PROJECT_WORKTREES_WRITABLE tmpdir=$TMPDIR_WRITABLE cache=$CACHE_DIR_WRITABLE"
        echo "  Auto-config:"
        echo "    EVOLVE_SANDBOX_FALLBACK_ON_EPERM=$AUTO_FALLBACK_ON_EPERM"
        echo "    worktree_base=${WORKTREE_BASE:-<NONE>}"
        echo "    worktree_base_reason: $WORKTREE_BASE_REASON"
        echo "    inner_sandbox=$INNER_SANDBOX"
        echo "    inner_sandbox_reason: $INNER_SANDBOX_REASON"
        echo "    Reasoning: $AUTO_REASONING"
        ;;
esac

exit 0
