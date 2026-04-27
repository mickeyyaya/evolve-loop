#!/usr/bin/env bash
#
# claude.sh — CLI adapter for Anthropic Claude Code.
#
# Translates an evolve-loop agent profile JSON into a fully-formed `claude -p`
# command. Called by scripts/subagent-run.sh; not intended for direct use.
#
# Inputs (env vars set by subagent-run.sh):
#   PROFILE_PATH        — absolute path to profile JSON
#   RESOLVED_MODEL      — resolved model alias (haiku/sonnet/opus)
#   PROMPT_FILE         — absolute path to file containing the prompt
#   CYCLE               — cycle number (integer)
#   WORKSPACE_PATH      — absolute path for per-cycle artifact directory
#   WORKTREE_PATH       — optional, absolute worktree dir for Builder
#   STDOUT_LOG          — absolute path to capture stdout
#   STDERR_LOG          — absolute path to capture stderr
#   ARTIFACT_PATH       — resolved expected artifact path (cycle substituted)
#
# Output: writes claude's stdout to STDOUT_LOG, stderr to STDERR_LOG, exits with
# claude's exit code.

set -euo pipefail

: "${PROFILE_PATH:?claude.sh: PROFILE_PATH unset}"
: "${RESOLVED_MODEL:?claude.sh: RESOLVED_MODEL unset}"
: "${PROMPT_FILE:?claude.sh: PROMPT_FILE unset}"
: "${CYCLE:?claude.sh: CYCLE unset}"
: "${WORKSPACE_PATH:?claude.sh: WORKSPACE_PATH unset}"
: "${STDOUT_LOG:?claude.sh: STDOUT_LOG unset}"
: "${STDERR_LOG:?claude.sh: STDERR_LOG unset}"

if ! command -v claude >/dev/null 2>&1; then
    echo "claude.sh: ERROR: 'claude' binary not found in PATH" >&2
    exit 127
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "claude.sh: ERROR: 'jq' binary not found in PATH" >&2
    exit 127
fi

# --- Read profile fields -----------------------------------------------------
# CRITICAL: Tool patterns may contain spaces (e.g., "Bash(python -m pytest:*)",
# "Bash(git status)"). They MUST be passed to claude as separate argv elements
# with each element internally preserving its spaces — otherwise bash word-
# splitting on `--allowedTools $JOINED_STRING` produces tokens like `Bash(python`
# and `-m`, the latter of which claude's CLI parser interprets as an unknown
# short option ("error: unknown option '-m'"). v8.12.0 shipped with that bug
# silently — validate-profile didn't catch it because it never executes claude.
# Portable array population (mapfile is bash 4+; macOS default bash is 3.2).
ALLOWED_TOOLS=()
DISALLOWED_TOOLS=()
ADD_DIRS_ARR=()
EXTRA_FLAGS_ARR=()
while IFS= read -r line; do [ -n "$line" ] && ALLOWED_TOOLS+=("$line"); done < <(jq -r '.allowed_tools[]?' "$PROFILE_PATH")
while IFS= read -r line; do [ -n "$line" ] && DISALLOWED_TOOLS+=("$line"); done < <(jq -r '.disallowed_tools[]?' "$PROFILE_PATH")
while IFS= read -r line; do [ -n "$line" ] && ADD_DIRS_ARR+=("$line"); done < <(jq -r '.add_dir[]?' "$PROFILE_PATH")
while IFS= read -r line; do [ -n "$line" ] && EXTRA_FLAGS_ARR+=("$line"); done < <(jq -r '.extra_flags[]?' "$PROFILE_PATH")
MAX_BUDGET=$(jq -r '.max_budget_usd' "$PROFILE_PATH")
MAX_TURNS=$(jq -r '.max_turns' "$PROFILE_PATH")
PERMISSION_MODE=$(jq -r '.permission_mode' "$PROFILE_PATH")

# Resolve add_dir, substituting {worktree_path} placeholder if present.
for i in "${!ADD_DIRS_ARR[@]}"; do
    if [[ "${ADD_DIRS_ARR[$i]}" == *"{worktree_path}"* ]]; then
        if [ -z "${WORKTREE_PATH:-}" ]; then
            echo "claude.sh: ERROR: profile references {worktree_path} but WORKTREE_PATH is unset" >&2
            exit 2
        fi
        ADD_DIRS_ARR[$i]="${ADD_DIRS_ARR[$i]//\{worktree_path\}/$WORKTREE_PATH}"
    fi
done

# --- Build command -----------------------------------------------------------
declare -a CMD
CMD=(claude -p --model "$RESOLVED_MODEL")
CMD+=(--permission-mode "$PERMISSION_MODE")
CMD+=(--output-format json)

# Budget cap is critical — only attach if numeric and > 0.
if [[ "$MAX_BUDGET" =~ ^[0-9]+(\.[0-9]+)?$ ]] && (( $(echo "$MAX_BUDGET > 0" | bc -l) )); then
    CMD+=(--max-budget-usd "$MAX_BUDGET")
fi

# Allowed/disallowed tools — each pattern as its own argv element so spaces
# inside parentheses survive shell tokenization.
if [ "${#ALLOWED_TOOLS[@]}" -gt 0 ]; then
    CMD+=(--allowedTools "${ALLOWED_TOOLS[@]}")
fi
if [ "${#DISALLOWED_TOOLS[@]}" -gt 0 ]; then
    CMD+=(--disallowedTools "${DISALLOWED_TOOLS[@]}")
fi

# Add-dir(s) — each path as its own argv element.
if [ "${#ADD_DIRS_ARR[@]}" -gt 0 ]; then
    CMD+=(--add-dir "${ADD_DIRS_ARR[@]}")
fi

# Extra flags (--bare, --no-session-persistence, etc.) — already valid CLI flags.
# CAVEAT: `--bare` makes claude refuse to read OAuth/keychain credentials,
# requiring ANTHROPIC_API_KEY in the env. Most Claude Code users authenticate
# via OAuth (no env var). Dropping --bare for those users so the subagent can
# authenticate. The remaining isolation flags (--no-session-persistence,
# --strict-mcp-config, --exclude-dynamic-system-prompt-sections) still apply.
# Override with EVOLVE_FORCE_BARE=1 if you do have ANTHROPIC_API_KEY set.
if [ "${#EXTRA_FLAGS_ARR[@]}" -gt 0 ]; then
    if [ -z "${ANTHROPIC_API_KEY:-}" ] && [ "${EVOLVE_FORCE_BARE:-0}" != "1" ]; then
        FILTERED=()
        for flag in "${EXTRA_FLAGS_ARR[@]}"; do
            if [ "$flag" = "--bare" ]; then
                echo "[claude-adapter] WARN: dropping --bare (no ANTHROPIC_API_KEY); set EVOLVE_FORCE_BARE=1 to retain" >&2
                continue
            fi
            FILTERED+=("$flag")
        done
        EXTRA_FLAGS_ARR=("${FILTERED[@]}")
    fi
    if [ "${#EXTRA_FLAGS_ARR[@]}" -gt 0 ]; then
        CMD+=("${EXTRA_FLAGS_ARR[@]}")
    fi
fi

# Working directory: worktree for Builder, repo root for everyone else.
WORKING_DIR="${WORKTREE_PATH:-$PWD}"

# --- Sandbox profile generation ----------------------------------------------
# When EVOLVE_SANDBOX=1 (or sandbox.enabled in profile and EVOLVE_SANDBOX unset),
# generate an inline macOS sandbox-exec profile (or bwrap argv on Linux) and
# wrap the claude command. This is OS-level enforcement of the same rules already
# expressed in disallowed_tools — belt-and-suspenders against subprocess escape
# (Anthropic's Secure Deployment Guide notes --allowedTools is a permission gate,
# not a sandbox; an agent can pipe to python3 to bypass the allowlist parser).
SANDBOX_USE=0
if [ "${EVOLVE_SANDBOX:-}" = "1" ]; then
    SANDBOX_USE=1
elif [ "${EVOLVE_SANDBOX:-}" != "0" ]; then
    # No explicit env override; defer to profile.
    if jq -e '.sandbox.enabled == true' "$PROFILE_PATH" > /dev/null 2>&1; then
        SANDBOX_USE=1
    fi
fi

generate_macos_sandbox_profile() {
    # Echoes a complete sandbox-exec profile string to stdout.
    local repo_root write_subpaths_json deny_subpaths_json read_only_repo allow_network
    repo_root="$(cd "$PWD" && pwd)"
    write_subpaths_json=$(jq -r '.sandbox.write_subpaths // [] | .[]' "$PROFILE_PATH" 2>/dev/null)
    deny_subpaths_json=$(jq -r '.sandbox.deny_subpaths // [] | .[]' "$PROFILE_PATH" 2>/dev/null)
    read_only_repo=$(jq -r '.sandbox.read_only_repo // false' "$PROFILE_PATH" 2>/dev/null)
    allow_network=$(jq -r '.sandbox.allow_network // true' "$PROFILE_PATH" 2>/dev/null)

    # Header — system.sb gives baseline access (dyld, libsystem, etc. needed for any binary)
    cat <<EOF
(version 1)
(deny default)
(import "system.sb")
(allow process-exec)
(allow process-fork)
(allow signal)
(allow sysctl-read)
(allow mach-lookup)
(allow ipc-posix-shm)
(allow file-read-metadata)
(allow file-read* (subpath "$repo_root"))
(allow file-read* (subpath "/usr"))
(allow file-read* (subpath "/System"))
(allow file-read* (subpath "/Library"))
(allow file-read* (subpath "/private/etc"))
(allow file-read* (subpath "/opt"))
(allow file-read* (subpath "/bin"))
(allow file-read* (subpath "/sbin"))
(allow file-read* (subpath "/var"))
(allow file-read* (subpath "/private/var"))
(allow file-read* (subpath "/dev"))
; Claude needs to read user config (auth, settings) and write its own session/cache state
(allow file-read* (subpath "$HOME"))
(allow file-write* (subpath "/private/tmp"))
(allow file-write* (subpath "/tmp"))
(allow file-write* (subpath "/var/folders"))
(allow file-write* (subpath "/private/var/folders"))
(allow file-write* (subpath "$HOME/.claude"))
(allow file-write* (subpath "$HOME/.cache"))
(allow file-write* (subpath "$HOME/.config"))
(allow file-write* (subpath "$HOME/Library/Caches"))
(allow file-write* (subpath "$HOME/Library/Application Support"))
EOF

    # Per-agent write paths.
    if [ "$read_only_repo" = "true" ]; then
        # Read-only repo mode: only the explicit write_subpaths inside the repo are writable.
        :
    fi
    while IFS= read -r wp; do
        [ -z "$wp" ] && continue
        # Substitute {worktree_path} placeholder if present.
        if [[ "$wp" == *"{worktree_path}"* ]]; then
            [ -z "${WORKTREE_PATH:-}" ] && continue
            wp="${wp//\{worktree_path\}/$WORKTREE_PATH}"
        fi
        # Resolve relative paths against repo_root.
        case "$wp" in
            /*) abs_wp="$wp" ;;
            *)  abs_wp="$repo_root/$wp" ;;
        esac
        # write_subpaths may contain glob patterns (e.g., "cycle-*"). sandbox-exec
        # subpath does NOT interpret globs — it requires a literal directory
        # prefix. Convert any path containing a '*' to its parent dir, which
        # widens scope to the parent (e.g., .evolve/runs/cycle-* → .evolve/runs/).
        # That's acceptable: the parent is the intended write zone, cycle-N
        # subdirs sit inside it.
        if [[ "$abs_wp" == *"*"* ]]; then
            abs_wp=$(dirname "$abs_wp")
        fi
        echo "(allow file-write* (subpath \"$abs_wp\"))"
    done <<< "$write_subpaths_json"

    # Per-agent deny paths (kernel-enforced mirror of disallowed_tools file patterns).
    while IFS= read -r dp; do
        [ -z "$dp" ] && continue
        case "$dp" in
            /*) abs_dp="$dp" ;;
            *)  abs_dp="$repo_root/$dp" ;;
        esac
        # Detect file vs subpath via heuristic: if it ends in .json/.jsonl/.md/.sh/.txt assume file (literal).
        if [[ "$abs_dp" =~ \.[a-z]{1,5}$ ]]; then
            echo "(deny file-write* (literal \"$abs_dp\"))"
        else
            echo "(deny file-write* (subpath \"$abs_dp\"))"
        fi
    done <<< "$deny_subpaths_json"

    # Network — Anthropic API requires outbound. Future: domain-allowlist proxy.
    if [ "$allow_network" = "true" ]; then
        echo '(allow network*)'
    else
        # Evaluator profile sets allow_network=false. Block all outbound.
        echo '(deny network*)'
        # But allow local Unix sockets (claude needs them for IPC).
        echo '(allow network* (local unix))'
    fi
}

# --- Print resolved command for audit trail ----------------------------------
echo "[claude-adapter] cwd=$WORKING_DIR" >&2
echo "[claude-adapter] command=${CMD[*]}" >&2
echo "[claude-adapter] prompt-file=$PROMPT_FILE" >&2
echo "[claude-adapter] artifact=$ARTIFACT_PATH" >&2
echo "[claude-adapter] max-turns=$MAX_TURNS (advisory; not enforced by claude flag)" >&2
echo "[claude-adapter] sandbox=$SANDBOX_USE" >&2

# Validate-only mode: print and exit without running.
if [ "${VALIDATE_ONLY:-0}" = "1" ]; then
    echo "[claude-adapter] VALIDATE_ONLY=1 — not executing" >&2
    if [ "$SANDBOX_USE" = "1" ] && [[ "$OSTYPE" == "darwin"* ]]; then
        echo "[claude-adapter] would-wrap-with: sandbox-exec -p '<inline profile>'" >&2
        echo "[claude-adapter] sandbox profile preview (first 20 lines):" >&2
        # Use a temp file to avoid SIGPIPE under set -o pipefail when head closes early.
        SANDBOX_PREVIEW_TMP=$(mktemp)
        generate_macos_sandbox_profile > "$SANDBOX_PREVIEW_TMP"
        head -20 "$SANDBOX_PREVIEW_TMP" | sed 's/^/  /' >&2
        rm -f "$SANDBOX_PREVIEW_TMP"
    fi
    exit 0
fi

# --- Execute -----------------------------------------------------------------
# Read prompt from PROMPT_FILE on stdin so command line stays clean.
cd "$WORKING_DIR"
if [ "$SANDBOX_USE" = "1" ]; then
    if [[ "$OSTYPE" == "darwin"* ]] && command -v sandbox-exec >/dev/null 2>&1; then
        SANDBOX_PROFILE_TXT=$(generate_macos_sandbox_profile)
        echo "[claude-adapter] wrapping in sandbox-exec ($(echo "$SANDBOX_PROFILE_TXT" | wc -l | tr -d ' ') profile lines)" >&2
        /usr/bin/sandbox-exec -p "$SANDBOX_PROFILE_TXT" "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG"
        EXIT_CODE=$?
    elif command -v bwrap >/dev/null 2>&1; then
        REPO_ROOT_BWRAP="$(cd "$PWD" && pwd)"
        echo "[claude-adapter] wrapping in bwrap (Linux)" >&2
        # Read-only repo with selective writable workspace.
        bwrap --ro-bind / / \
              --bind "$REPO_ROOT_BWRAP/.evolve/runs" "$REPO_ROOT_BWRAP/.evolve/runs" \
              --bind "${WORKTREE_PATH:-$REPO_ROOT_BWRAP}" "${WORKTREE_PATH:-$REPO_ROOT_BWRAP}" \
              --tmpfs /tmp \
              --share-net \
              --proc /proc --dev /dev \
              "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG"
        EXIT_CODE=$?
    else
        echo "[claude-adapter] WARN: sandbox requested but neither sandbox-exec (macOS) nor bwrap (Linux) available; running unwrapped" >&2
        "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG"
        EXIT_CODE=$?
    fi
else
    "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG"
    EXIT_CODE=$?
fi
exit "$EXIT_CODE"
