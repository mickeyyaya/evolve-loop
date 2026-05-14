#!/usr/bin/env bash
#
# claude.sh — CLI adapter for Anthropic Claude Code.
#
# Translates an evolve-loop agent profile JSON into a fully-formed `claude -p`
# command. Called by scripts/dispatch/subagent-run.sh; not intended for direct use.
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

# v8.13.5 token-opt: declarative task-mode budget tiers.
# Profiles MAY define a budget_tiers map ({"research": 1.50, "deep": 2.50, ...}).
# When EVOLVE_TASK_MODE=<mode> is set AND the mode key exists in the profile's
# budget_tiers, that value is used. Falls back silently to the static
# max_budget_usd when no tier matches (with WARN if EVOLVE_TASK_MODE was set
# but the key wasn't found — caller's typo is loud, not silent).
# Precedence (set later, by EVOLVE_MAX_BUDGET_USD block below):
#   EVOLVE_MAX_BUDGET_USD > EVOLVE_TASK_MODE-resolved tier > profile default.
if [ -n "${EVOLVE_TASK_MODE:-}" ]; then
    TIER_VALUE=$(jq -r --arg mode "$EVOLVE_TASK_MODE" '.budget_tiers[$mode] // empty' "$PROFILE_PATH" 2>/dev/null || echo "")
    if [ -n "$TIER_VALUE" ] && [[ "$TIER_VALUE" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        echo "[claude-adapter] task-mode tier: $EVOLVE_TASK_MODE → \$$TIER_VALUE (was $MAX_BUDGET from profile $(basename "$PROFILE_PATH"))" >&2
        MAX_BUDGET="$TIER_VALUE"
    else
        echo "[claude-adapter] WARN: EVOLVE_TASK_MODE='$EVOLVE_TASK_MODE' has no matching budget_tiers entry in $(basename "$PROFILE_PATH") — using profile default $MAX_BUDGET" >&2
    fi
fi

# v8.60+ deprecation bridge: EVOLVE_BUDGET_CAP → EVOLVE_MAX_BUDGET_USD
# Emits one stderr WARN per process invocation (idempotency guard: _BUDGET_CAP_WARNED).
# EVOLVE_MAX_BUDGET_USD always wins when both are set. Removal target: v8.61+.
if [ -n "${EVOLVE_BUDGET_CAP:-}" ] && [ -z "${_BUDGET_CAP_WARNED:-}" ]; then
    export _BUDGET_CAP_WARNED=1
    if [[ "${EVOLVE_BUDGET_CAP}" =~ ^[0-9]+(\.[0-9]+)?$ ]] && (( $(echo "$EVOLVE_BUDGET_CAP > 0" | bc -l) )); then
        if [ -n "${EVOLVE_MAX_BUDGET_USD:-}" ]; then
            echo "[claude-adapter] DEPRECATED EVOLVE_BUDGET_CAP: EVOLVE_MAX_BUDGET_USD=$EVOLVE_MAX_BUDGET_USD takes precedence; remove EVOLVE_BUDGET_CAP" >&2
        else
            echo "[claude-adapter] DEPRECATED EVOLVE_BUDGET_CAP: use EVOLVE_MAX_BUDGET_USD instead (bridging for this invocation)" >&2
            export EVOLVE_MAX_BUDGET_USD="$EVOLVE_BUDGET_CAP"
        fi
    else
        echo "[claude-adapter] WARN: EVOLVE_BUDGET_CAP='$EVOLVE_BUDGET_CAP' invalid — falling through to default" >&2
    fi
fi

# v8.13.4 token-opt: per-invocation budget override.
# The static profile max_budget_usd is sized for typical workloads (codebase
# scan, modest builds). Research-heavy or unusually long tasks may need more
# headroom — cycle 8210's Scout invocation hit error_max_budget_usd at $0.51
# during research, wasting the partial work. Operators can now pass
# EVOLVE_MAX_BUDGET_USD=<value> to bump for ONE invocation without permanently
# raising the profile baseline. Logged loudly for auditability.
# Validation: must be a non-negative decimal; non-numeric values are ignored
# with a warning rather than silently corrupting the budget arg.
if [ -n "${EVOLVE_MAX_BUDGET_USD:-}" ]; then
    if [[ "$EVOLVE_MAX_BUDGET_USD" =~ ^[0-9]+(\.[0-9]+)?$ ]]; then
        echo "[claude-adapter] override max-budget-usd: $EVOLVE_MAX_BUDGET_USD (was $MAX_BUDGET from profile $(basename "$PROFILE_PATH"))" >&2
        MAX_BUDGET="$EVOLVE_MAX_BUDGET_USD"
    else
        echo "[claude-adapter] WARN: EVOLVE_MAX_BUDGET_USD='$EVOLVE_MAX_BUDGET_USD' is not a valid decimal — ignoring, using profile value $MAX_BUDGET" >&2
    fi
fi

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
# v9.2.0: stream-json enables real-time stdout.log mtime updates so
# phase-watchdog can detect genuine subagent activity instead of waiting
# for the end-of-run blob. Fixes cycle-36-style watchdog false-positive
# where Builder doing 15 min of API work produced zero workspace mtime
# touches. --verbose is a CLI hard requirement when --print + stream-json
# are combined (claude 2.1.140 rejects without it). Gated on EVOLVE_STREAM_JSON
# (default 1) per the project's verify→default-on ladder; set to 0 to fall
# back to the legacy single-blob format for one-cycle verification or rollback.
if [ "${EVOLVE_STREAM_JSON:-1}" = "1" ]; then
    CMD+=(--output-format stream-json --verbose)
else
    CMD+=(--output-format json)
fi

# v8.26.0: budget cap default-unlimited.
#
# Pre-v8.26.0, MAX_BUDGET came from the profile (~$0.18 Scout default, $0.50
# Intent, $1.00 Orchestrator). Complex meta-goals routinely exceeded these
# caps mid-thought, exiting subagents with BUDGET_EXCEEDED (rc=1) and aborting
# the cycle with no useful output — friction without protection (budget caps
# don't prevent reward hacking; they only limit cost).
#
# v8.26.0 sets `--max-budget-usd` to 999999 (effectively unlimited) by default.
# The flag is still passed (claude binary expects it) but the value never
# triggers BUDGET_EXCEEDED in any realistic cycle.
#
# Operator overrides (v8.60+: EVOLVE_BUDGET_CAP is deprecated; use EVOLVE_MAX_BUDGET_USD):
#   EVOLVE_MAX_BUDGET_USD=<value> — pin a hard cap (operator-set, highest priority)
#   EVOLVE_BUDGET_ENFORCE=1       — use the resolved-from-profile MAX_BUDGET
#                                   (legacy behavior; opt back in for cost discipline)
ORIGINAL_MAX_BUDGET="$MAX_BUDGET"
if [ -n "${EVOLVE_MAX_BUDGET_USD:-}" ]; then
    : # operator override already applied in step 3 above; preserve it, skip unlimited default
elif [ "${EVOLVE_BUDGET_ENFORCE:-0}" = "1" ]; then
    echo "[claude-adapter] EVOLVE_BUDGET_ENFORCE=1: using resolved budget \$$MAX_BUDGET (legacy strict cap)" >&2
else
    MAX_BUDGET=999999
    echo "[claude-adapter] budget cap unlimited (max-budget-usd=$MAX_BUDGET); was \$$ORIGINAL_MAX_BUDGET from profile. Set EVOLVE_MAX_BUDGET_USD=<value> for a hard cap, or EVOLVE_BUDGET_ENFORCE=1 to use the profile value." >&2
fi
# Always attach — claude binary expects the flag.
CMD+=(--max-budget-usd "$MAX_BUDGET")

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

# v8.61.0 Cycle A2: when EVOLVE_CACHE_PREFIX_V2=1 AND AGENT env is set,
# emit the static bedrock as a system prompt via --append-system-prompt.
# claude CLI's system prompt is cached separately from the user prompt and
# requires no manual cache-breakpoint management — the cleanest place for
# byte-stable role identity content.
#
# Also passes --exclude-dynamic-system-prompt-sections so per-machine
# sections (cwd, env info, memory paths, git status) move OUT of the
# system prompt and INTO the first user message — claude's docs note this
# "improves cross-user prompt-cache reuse" by keeping the system layer
# free of per-invocation entropy.
if [ "${EVOLVE_CACHE_PREFIX_V2:-0}" = "1" ] && [ -n "${AGENT:-}" ]; then
    _bic="${EVOLVE_PLUGIN_ROOT:-}/scripts/dispatch/build-invocation-context.sh"
    if [ -x "$_bic" ]; then
        _bedrock_text=$(bash "$_bic" "$AGENT" 2>/dev/null || true)
        # Honor ADVERSARIAL_AUDIT=0 by stripping the Adversarial Audit Mode
        # section (auditor only — other roles never include it).
        #
        # v9.0.1 HIGH-2 fix: section-aware strip. The previous awk pattern
        # (/^## Adversarial Audit Mode/{stop=1} !stop {print}) printed lines
        # until the section header, then stopped FOREVER — silently dropping
        # any content that happened to follow the section. Today the section
        # is the last block in the auditor bedrock, so the bug was latent;
        # but adding any future content after Adversarial Audit Mode would
        # be silently dropped under ADVERSARIAL_AUDIT=0.
        #
        # The fix: skip lines from `## Adversarial Audit Mode` until the
        # NEXT level-2 heading (or EOF), then resume printing. This excises
        # exactly the named section regardless of what surrounds it.
        if [ "$AGENT" = "auditor" ] && [ "${ADVERSARIAL_AUDIT:-1}" = "0" ] && [ -n "$_bedrock_text" ]; then
            _bedrock_text=$(echo "$_bedrock_text" | awk '
                /^## Adversarial Audit Mode/ { skip=1; next }
                skip && /^## / && !/^## Adversarial Audit Mode/ { skip=0 }
                !skip { print }
            ')
        fi
        if [ -n "$_bedrock_text" ]; then
            CMD+=(--append-system-prompt "$_bedrock_text")
            # NB: --exclude-dynamic-system-prompt-sections is already present
            # in every shipped profile's extra_flags, so it flows through via
            # EXTRA_FLAGS_ARR a few lines below — do NOT add it here too
            # (Cycle A3 fix: the duplicate from Cycle A2 was redundant).
            echo "[claude-adapter] system-prompt: bedrock attached (~$(echo "$_bedrock_text" | wc -c | tr -d ' ') bytes for role=$AGENT)" >&2
        else
            echo "[claude-adapter] WARN: build-invocation-context.sh produced empty output for AGENT=$AGENT" >&2
        fi
    else
        echo "[claude-adapter] WARN: EVOLVE_CACHE_PREFIX_V2=1 but build-invocation-context.sh missing or non-executable at $_bic" >&2
    fi
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
# A profile is "worktree-aware" iff its add_dir or sandbox.write_subpaths
# references the literal token {worktree_path}. Otherwise WORKTREE_PATH must
# NOT influence cwd — doing so breaks orchestrator/scout/auditor relative-path
# write permissions (e.g. Write(.evolve/runs/cycle-*/orchestrator-report.md)).
#
# v8.23.3 BUG-009 fix: subagent-run.sh exports EVOLVE_PROFILE_WORKTREE_AWARE=1
# when the original (pre-expansion) profile contains {worktree_path}. We trust
# that env hint as the canonical signal because v8.23.2's pre-expansion sweeps
# the literal token out of the profile JSON before this script reads it — the
# old `any(. == "{worktree_path}")` check would always return false on
# pre-expanded profiles, falling through to PWD and breaking the builder's
# cwd. The literal-string check remains as a fallback for direct claude.sh
# invocations that bypass subagent-run.sh (tests, ad-hoc).
if [ "${EVOLVE_PROFILE_WORKTREE_AWARE:-0}" = "1" ]; then
    WORKING_DIR="${WORKTREE_PATH:-$PWD}"
elif jq -e '((.add_dir // []) + (.sandbox.write_subpaths // [])) | any(. == "{worktree_path}")' "$PROFILE_PATH" > /dev/null 2>&1; then
    WORKING_DIR="${WORKTREE_PATH:-$PWD}"
else
    WORKING_DIR="$PWD"
fi

# --- Sandbox profile generation ----------------------------------------------
# When EVOLVE_SANDBOX=1 (or sandbox.enabled in profile and EVOLVE_SANDBOX unset),
# generate an inline macOS sandbox-exec profile (or bwrap argv on Linux) and
# wrap the claude command. This is OS-level enforcement of the same rules already
# expressed in disallowed_tools — belt-and-suspenders against subprocess escape
# (Anthropic's Secure Deployment Guide notes --allowedTools is a permission gate,
# not a sandbox; an agent can pipe to python3 to bypass the allowlist parser).
SANDBOX_USE=0
SANDBOX_USE_SOURCE="default"
if [ "${EVOLVE_SANDBOX:-}" = "1" ]; then
    SANDBOX_USE=1
    SANDBOX_USE_SOURCE="EVOLVE_SANDBOX=1"
elif [ "${EVOLVE_SANDBOX:-}" != "0" ]; then
    # No explicit env override; defer to profile.
    if jq -e '.sandbox.enabled == true' "$PROFILE_PATH" > /dev/null 2>&1; then
        SANDBOX_USE=1
        SANDBOX_USE_SOURCE="profile sandbox.enabled=true"
    fi
fi

# v8.25.1: inner_sandbox decision from preflight environment.json.
#
# In nested-Claude (CLAUDECODE / CLAUDE_CODE_* env present), the OUTER Claude
# Code OS sandbox already provides process-level isolation. Wrapping with a
# nested sandbox-exec adds NO new protection (nested sandboxes can only
# intersect — restrict further — never union — expand) AND introduces
# execution-time EPERM that the v8.22.0 startup-fallback can't catch.
#
# preflight-environment.sh writes auto_config.inner_sandbox to
# .evolve/environment.json. When false, claude-adapter skips sandbox-exec
# wrapping entirely. Tier-1 kernel hooks (phase-gate-precondition, role-gate,
# ledger-SHA verification) all run OUTSIDE the sandbox layer and continue
# to enforce structural integrity. claude --add-dir continues to gate
# Edit/Write tool paths.
#
# Deprecation bridge (v8.60.0+): EVOLVE_FORCE_INNER_SANDBOX=1 → EVOLVE_INNER_SANDBOX=1.
# Emits one stderr WARN per process invocation. Removal target: v8.61+.
if [ "${EVOLVE_FORCE_INNER_SANDBOX:-0}" = "1" ] && [ -z "${EVOLVE_INNER_SANDBOX:-}" ]; then
    echo "[claude-adapter] WARN: EVOLVE_FORCE_INNER_SANDBOX is deprecated; use EVOLVE_INNER_SANDBOX=1" >&2
    echo "[claude-adapter]   Removal target: v8.61+. Update scripts to EVOLVE_INNER_SANDBOX=1." >&2
    EVOLVE_INNER_SANDBOX=1
fi

# Override priority (highest first):
#   1. EVOLVE_INNER_SANDBOX=1       — operator force-enable (was EVOLVE_FORCE_INNER_SANDBOX=1)
#   2. EVOLVE_INNER_SANDBOX=0       — operator force-disable (explicit hatch)
#   3. environment.json:auto_config.inner_sandbox == false → SANDBOX_USE=0
#   4. (existing decision above stands)
#
# When inner_sandbox=false drives SANDBOX_USE=0, log loudly so operators see
# the posture and can tell defense-in-depth is preserved at Tier-1.
ENV_PROFILE_JSON=""
if [ -n "${EVOLVE_PROJECT_ROOT:-}" ] && [ -f "$EVOLVE_PROJECT_ROOT/.evolve/environment.json" ]; then
    ENV_PROFILE_JSON="$EVOLVE_PROJECT_ROOT/.evolve/environment.json"
fi

if [ "${EVOLVE_INNER_SANDBOX:-}" = "1" ]; then
    SANDBOX_USE=1
    SANDBOX_USE_SOURCE="EVOLVE_INNER_SANDBOX=1 (operator force-enable)"
elif [ "${EVOLVE_INNER_SANDBOX:-}" = "0" ]; then
    SANDBOX_USE=0
    SANDBOX_USE_SOURCE="EVOLVE_INNER_SANDBOX=0 (operator force-disable)"
elif [ -n "$ENV_PROFILE_JSON" ]; then
    # Use `has()` then `tostring` because jq's `// null` treats `false` as missing.
    auto_inner=$(jq -r '.auto_config | if has("inner_sandbox") then .inner_sandbox | tostring else "MISSING" end' "$ENV_PROFILE_JSON" 2>/dev/null)
    if [ "$auto_inner" = "false" ]; then
        if [ "$SANDBOX_USE" = "1" ]; then
            inner_reason=$(jq -r '.auto_config.inner_sandbox_reason // ""' "$ENV_PROFILE_JSON" 2>/dev/null)
            echo "[claude-adapter] inner sandbox-exec DISABLED (from environment.json:auto_config.inner_sandbox=false)" >&2
            echo "[claude-adapter]   reason: $inner_reason" >&2
            echo "[claude-adapter]   outer Claude Code OS sandbox + Tier-1 kernel hooks remain enforced" >&2
            echo "[claude-adapter]   role-gate + phase-gate-precondition + ledger-SHA verify writes" >&2
            echo "[claude-adapter]   (operator force-enable: EVOLVE_INNER_SANDBOX=1)" >&2
        fi
        SANDBOX_USE=0
        SANDBOX_USE_SOURCE="environment.json:auto_config.inner_sandbox=false"
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
; /tmp and /var/folders need BOTH read and write — claude -p writes bash tool
; output files (e.g. /tmp/claude-${UID}/<project>/<session>/tasks/*.output) and
; then reads them back to return the result to the LLM. Without read access
; the read fails with EPERM, which presented in cycles 8121-8128 audits as
; "Bash command execution was blocked" / "another Claude Code process deleted
; it during startup cleanup". Root cause was sandbox-exec's missing read rule,
; not concurrent-session collision. v8.12.4 fix.
(allow file-read* (subpath "/tmp"))
(allow file-read* (subpath "/private/tmp"))
(allow file-read* (subpath "/var/folders"))
(allow file-read* (subpath "/private/var/folders"))
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
    # When read_only_repo=true, explicitly DENY repo-wide writes BEFORE the
    # write_subpaths allow loop. Per sandbox-exec SBPL semantics, later rules
    # override earlier rules for overlapping subpaths — so a specific
    # write_subpath (e.g., .evolve/runs/cycle-*) re-permits that subdirectory.
    # The deny is belt-and-suspenders documentation of the contract: even
    # without it, the repo is implicitly write-denied because no broader allow
    # rule covers it. The explicit deny protects against future changes adding
    # a broader $HOME or repo allow.
    if [ "$read_only_repo" = "true" ]; then
        echo "(deny file-write* (subpath \"$repo_root\"))"
    fi
    while IFS= read -r wp; do
        [ -z "$wp" ] && continue
        # Substitute {worktree_path} placeholder if present.
        # v8.21.0: parity with the add_dir substitution at the top of this file
        # (lines ~99-108). Both substitution sites now fail loudly when
        # WORKTREE_PATH is unset. The previous silent `continue` at this site
        # collapsed the rule to nothing, masking BUG-001 (no component
        # provisioned the build worktree) for multiple releases.
        if [[ "$wp" == *"{worktree_path}"* ]]; then
            if [ -z "${WORKTREE_PATH:-}" ]; then
                echo "claude.sh: ERROR: profile sandbox.write_subpaths references {worktree_path} but WORKTREE_PATH is unset" >&2
                exit 2
            fi
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
echo "[claude-adapter] sandbox=$SANDBOX_USE (source: $SANDBOX_USE_SOURCE)" >&2
# v8.16.2: diagnostic — record runtime knob values so we can trace propagation.
echo "[claude-adapter] env: EVOLVE_SANDBOX_FALLBACK_ON_EPERM=${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-unset}" >&2

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

# v8.23.3 BUG-008 fix: capture inner-command exit codes via `|| EXIT_CODE=$?`
# instead of `; EXIT_CODE=$?`. The script enables `set -euo pipefail` at line 36;
# the bare-then-capture pattern caused `set -e` to exit the script *before*
# EXIT_CODE was assigned when sandbox-exec returned EPERM (rc=71). The Darwin-25.4
# nested-sandbox EPERM-fallback `if` block at line 354 NEVER ran in production
# because of this — the documented EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 workaround
# has been silently dead since v8.16.1. The fix: `|| EXIT_CODE=$?` lets `set -e`
# treat the failure as a logical-or branch (allowed), capturing the exit code
# correctly. All 5 EXIT_CODE assignments updated. Initialize EXIT_CODE=0 so the
# success path (exit 0 → `||` skipped → EXIT_CODE remains 0) works.
EXIT_CODE=0
if [ "$SANDBOX_USE" = "1" ]; then
    if [[ "$OSTYPE" == "darwin"* ]] && command -v sandbox-exec >/dev/null 2>&1; then
        SANDBOX_PROFILE_TXT=$(generate_macos_sandbox_profile)
        echo "[claude-adapter] wrapping in sandbox-exec ($(echo "$SANDBOX_PROFILE_TXT" | wc -l | tr -d ' ') profile lines)" >&2
        /usr/bin/sandbox-exec -p "$SANDBOX_PROFILE_TXT" "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG" || EXIT_CODE=$?
        # Sandbox EPERM fallback (v8.16.1+): on Darwin 25.4+, sandbox_apply()
        # returns EPERM when the parent process is itself sandboxed (nested
        # sandboxing disallowed). When EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1, detect
        # the specific error and retry unsandboxed with loud WARN. Opt-in only —
        # security-degraded but lets cycles run on Darwin 25.4 environments.
        if [ "$EXIT_CODE" -eq 71 ] && grep -q "sandbox_apply: Operation not permitted" "$STDERR_LOG" 2>/dev/null; then
            echo "[claude-adapter] DETECTED: sandbox-exec EPERM (Darwin nested sandbox disallowed)" >&2
            if [ "${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-0}" = "1" ]; then
                # v8.22.0: This flag is RE-AFFIRMED (un-deprecated from v8.21).
                # On Darwin 25.4+, sandbox_apply() returns EPERM when the parent
                # process is itself sandboxed — which is the canonical state when
                # /evolve-loop is invoked from inside Claude Code (the *primary*
                # use case for this skill). The dispatcher auto-enables the flag
                # when CLAUDECODE / CLAUDE_CODE_* env is detected (defense-in-depth
                # alongside SKILL.md). Anthropic's Secure Deployment Guide warns
                # that --allowedTools is "a permission gate, not a sandbox" — but
                # the kernel hooks (role-gate, ship-gate, phase-gate-precondition)
                # remain active on the unsandboxed retry, providing structurally-
                # enforced trust boundaries even without OS-level wrapping.
                echo "[claude-adapter] retry without sandbox-exec (nested-claude scenario)" >&2
                echo "[claude-adapter] note: kernel hooks (role-gate, ship-gate, phase-gate) remain enforced" >&2
                # v8.23.3: reset EXIT_CODE before the retry so the || guard is correct
                # (the retry is supposed to succeed; if it doesn't, capture its rc).
                EXIT_CODE=0
                "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG" || EXIT_CODE=$?
                echo "[claude-adapter] completed unsandboxed (rc=$EXIT_CODE)" >&2
            else
                echo "[claude-adapter] HINT: nested sandbox-exec is forbidden by Darwin 25.4+. To unblock:" >&2
                echo "  - From inside Claude Code: dispatcher auto-enables fallback (v8.22.0+)." >&2
                echo "  - From terminal: this should not happen; file an issue if it does." >&2
                echo "  - Manual override: EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 bash scripts/dispatch/evolve-loop-dispatch.sh ..." >&2
            fi
        fi
    elif command -v bwrap >/dev/null 2>&1; then
        REPO_ROOT_BWRAP="$(cd "$PWD" && pwd)"
        # Re-read allow_network from the profile so the bwrap branch matches the
        # macOS branch's network policy. Without this, --share-net was hardcoded
        # and the profile's allow_network: false (e.g., evaluator, retrospective)
        # was silently ignored on Linux. Equivalent to (deny network*) on macOS.
        BWRAP_ALLOW_NETWORK=$(jq -r '.sandbox.allow_network // true' "$PROFILE_PATH" 2>/dev/null)
        echo "[claude-adapter] wrapping in bwrap (Linux, network=$BWRAP_ALLOW_NETWORK)" >&2

        # Build bwrap argv as an array so --share-net is conditionally included.
        BWRAP_ARGS=(
            --ro-bind / /
            --bind "$REPO_ROOT_BWRAP/.evolve/runs" "$REPO_ROOT_BWRAP/.evolve/runs"
            --bind "${WORKTREE_PATH:-$REPO_ROOT_BWRAP}" "${WORKTREE_PATH:-$REPO_ROOT_BWRAP}"
            --tmpfs /tmp
            --proc /proc --dev /dev
        )
        if [ "$BWRAP_ALLOW_NETWORK" = "true" ]; then
            BWRAP_ARGS+=(--share-net)
        else
            BWRAP_ARGS+=(--unshare-net)
        fi
        bwrap "${BWRAP_ARGS[@]}" "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG" || EXIT_CODE=$?
    else
        echo "[claude-adapter] WARN: sandbox requested but neither sandbox-exec (macOS) nor bwrap (Linux) available; running unwrapped" >&2
        "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG" || EXIT_CODE=$?
    fi
else
    "${CMD[@]}" < "$PROMPT_FILE" >"$STDOUT_LOG" 2>"$STDERR_LOG" || EXIT_CODE=$?
fi

# v8.23.4 BUG-012 diagnostic: when the inner claude exits with an EPERM-like
# rc, dump enough context to the dispatcher's stderr that the operator can
# triage without re-running. Targets the "claude binary's own permission layer
# blocks writes despite --add-dir" symptom that v8.23.3 didn't fully eliminate
# in some nested-claude environments. Loud-but-bounded — tail the last 30
# lines of stderr, never log artifacts (could be huge).
if [ "$EXIT_CODE" -ne 0 ] && [ -s "$STDERR_LOG" ]; then
    if grep -qiE 'EACCES|EPERM|permission denied|operation not permitted|sandbox_apply' "$STDERR_LOG" 2>/dev/null; then
        echo "[claude-adapter] DIAGNOSTIC: inner claude exited rc=$EXIT_CODE with EPERM-class signal" >&2
        echo "[claude-adapter]   cwd at exec was: $WORKING_DIR" >&2
        echo "[claude-adapter]   --add-dir was: ${ADD_DIRS_ARR[*]:-<none>}" >&2
        echo "[claude-adapter]   sandbox use: $SANDBOX_USE  fallback: ${EVOLVE_SANDBOX_FALLBACK_ON_EPERM:-0}" >&2
        echo "[claude-adapter]   parent CLAUDECODE: ${CLAUDECODE:-<unset>} (set => parent is Claude Code)" >&2
        echo "[claude-adapter]   stderr tail (last 30 lines):" >&2
        tail -30 "$STDERR_LOG" 2>/dev/null | sed 's/^/    /' >&2
        echo "[claude-adapter] OPERATOR ACTION: if rc=71 (sandbox-exec EPERM), set EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1." >&2
        echo "[claude-adapter] OPERATOR ACTION: if rc=1 with 'EACCES' on a worktree path, set EVOLVE_SKIP_WORKTREE=1 (v8.23.4+)." >&2
        echo "[claude-adapter]                  caveat: skipping worktree disables isolation; builder edits land in main repo." >&2
        # v8.24.0: copy-paste rerun command. The dispatcher already auto-sets
        # both flags when nested-claude is detected, but a manual recovery from
        # a bare `bash run-cycle.sh` invocation needs an explicit rerun line.
        # Note: $EVOLVE_REINVOKE_CMD is exported by evolve-loop-dispatch.sh on
        # nested-claude detection (v8.24.0). Falls back to a generic hint.
        echo "[claude-adapter] RECOVERY (copy-paste):" >&2
        if [ -n "${EVOLVE_REINVOKE_CMD:-}" ]; then
            echo "    EVOLVE_SKIP_WORKTREE=1 EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 $EVOLVE_REINVOKE_CMD" >&2
        else
            echo "    EVOLVE_SKIP_WORKTREE=1 EVOLVE_SANDBOX_FALLBACK_ON_EPERM=1 <re-run last command>" >&2
        fi
    fi
fi

# v10.6.0 auto-resume Layer 1: capture Anthropic's "resets HH:MM(am|pm)"
# rate-limit message from stderr if present. estimate-quota-reset.sh
# (invoked by subagent-run.sh after _quota_likely fires) reads this file to
# compute a precise wake-up time. Silent on no match — fallback handles the
# common case where the outer Claude Code process intercepts the message at
# the auth layer and the nested subprocess gets empty stderr.
if [ -s "$STDERR_LOG" ]; then
    _quota_hint=$(grep -ioE 'resets +[0-9]{1,2}:[0-9]{2} *(am|pm)' "$STDERR_LOG" 2>/dev/null | head -1)
    if [ -n "$_quota_hint" ]; then
        _hint_dir=$(dirname "$STDERR_LOG")
        _hint_path="$_hint_dir/quota-reset-hint.txt"
        _hint_tmp="$_hint_path.tmp.$$"
        if printf '%s\n' "$_quota_hint" > "$_hint_tmp" 2>/dev/null && mv -f "$_hint_tmp" "$_hint_path" 2>/dev/null; then
            echo "[claude-adapter] quota-reset hint captured: $_quota_hint -> $_hint_path" >&2
        else
            rm -f "$_hint_tmp" 2>/dev/null || true
        fi
    fi
    unset _quota_hint _hint_dir _hint_path _hint_tmp
fi

exit "$EXIT_CODE"
