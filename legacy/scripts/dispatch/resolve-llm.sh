#!/usr/bin/env bash
#
# resolve-llm.sh — Pure-function LLM router for evolve-loop (ADR-1, v1.0).
#
# Given a role name and optional config path, emits a JSON object with
# {cli, model|model_tier, source} describing which CLI+model to use for
# that phase. Zero side effects. Safe to source from subagent-run.sh.
#
# Usage:
#   bash scripts/dispatch/resolve-llm.sh <role> [config_path]
#
#   role        — profile name (e.g., "scout", "builder", "auditor")
#   config_path — path to llm_config.json; defaults to .evolve/llm_config.json
#
# Output (stdout, always valid JSON on exit 0):
#   {"cli": "gemini", "model": "gemini-3-pro-preview", "source": "llm_config"}
#   {"cli": "claude", "model_tier": "sonnet", "source": "llm_config_fallback"}
#   {"cli": "claude", "model_tier": "sonnet", "source": "profile"}
#
# Resolution precedence (per ADR-1):
#   1. llm_config.phases.<role>     → source="llm_config"
#   2. llm_config._fallback         → source="llm_config_fallback"
#   3. Profile cli + model_tier_default → source="profile"
#   4. llm_config.json absent       → source="profile" (backward compat)
#
# Exit codes:
#   0 — success, JSON emitted to stdout
#   1 — profile JSON not found (no fallback possible)
#   2 — bad arguments
#
# Bash 3.2 compatible: no declare -A, no mapfile, no ${var^^}, no GNU-only flags.

set -uo pipefail

# ── Argument parsing ────────────────────────────────────────────────────────
ROLE=""
CONFIG_PATH=""

while [ $# -gt 0 ]; do
    case "$1" in
        --help|-h)
            sed -n '2,36p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*)
            echo "[resolve-llm] unknown flag: $1" >&2
            exit 2
            ;;
        *)
            if [ -z "$ROLE" ]; then
                ROLE="$1"
            elif [ -z "$CONFIG_PATH" ]; then
                CONFIG_PATH="$1"
            else
                echo "[resolve-llm] too many arguments" >&2
                exit 2
            fi
            ;;
    esac
    shift
done

if [ -z "$ROLE" ]; then
    echo "[resolve-llm] usage: resolve-llm.sh <role> [config_path]" >&2
    exit 2
fi

# ── Path resolution ────────────────────────────────────────────────────────
# Default config path if not provided
if [ -z "$CONFIG_PATH" ]; then
    _REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
    CONFIG_PATH="$_REPO_ROOT/.evolve/llm_config.json"
fi

# ── Profile location ────────────────────────────────────────────────────────
# Try EVOLVE_PLUGIN_ROOT first (plugin-installed), then project root
_find_profile() {
    local role="$1"
    local candidates=""

    # Candidate 1: EVOLVE_PLUGIN_ROOT (plugin install)
    if [ -n "${EVOLVE_PLUGIN_ROOT:-}" ]; then
        if [ -f "$EVOLVE_PLUGIN_ROOT/.evolve/profiles/${role}.json" ]; then
            echo "$EVOLVE_PLUGIN_ROOT/.evolve/profiles/${role}.json"
            return 0
        fi
    fi

    # Candidate 2: EVOLVE_PROJECT_ROOT (project root)
    if [ -n "${EVOLVE_PROJECT_ROOT:-}" ]; then
        if [ -f "$EVOLVE_PROJECT_ROOT/.evolve/profiles/${role}.json" ]; then
            echo "$EVOLVE_PROJECT_ROOT/.evolve/profiles/${role}.json"
            return 0
        fi
    fi

    # Candidate 3: git root (covers worktree scenarios)
    local git_root
    git_root=$(git rev-parse --show-toplevel 2>/dev/null) || git_root=""
    if [ -n "$git_root" ] && [ -f "$git_root/.evolve/profiles/${role}.json" ]; then
        echo "$git_root/.evolve/profiles/${role}.json"
        return 0
    fi

    # Not found
    return 1
}

# ── Helper: emit JSON result ────────────────────────────────────────────────
_emit_llm_config_result() {
    local cli="$1"
    local model="$2"
    local model_key="$3"
    local source="$4"
    # model_key is either "model" (exact) or "model_tier" (tier)
    printf '{"cli":"%s","%s":"%s","source":"%s"}\n' "$cli" "$model_key" "$model" "$source"
}

_emit_profile_result() {
    local cli="$1"
    local model_tier="$2"
    printf '{"cli":"%s","model_tier":"%s","source":"profile"}\n' "$cli" "$model_tier"
}

# ── Step 1: Try llm_config.json ─────────────────────────────────────────────
if [ -f "$CONFIG_PATH" ]; then
    # Validate it's parseable JSON
    if ! jq empty < "$CONFIG_PATH" 2>/dev/null; then
        echo "[resolve-llm] WARNING: $CONFIG_PATH is not valid JSON; falling back to profile" >&2
        # Fall through to profile
    else
        # Check for phases.<role>
        phase_entry=$(jq -r ".phases.${ROLE} // empty" < "$CONFIG_PATH" 2>/dev/null)
        if [ -n "$phase_entry" ] && [ "$phase_entry" != "null" ]; then
            cli=$(echo "$phase_entry" | jq -r '.cli // empty' 2>/dev/null)
            model=$(echo "$phase_entry" | jq -r '.model // empty' 2>/dev/null)

            if [ -n "$cli" ] && [ "$cli" != "null" ]; then
                if [ -n "$model" ] && [ "$model" != "null" ]; then
                    _emit_llm_config_result "$cli" "$model" "model" "llm_config"
                else
                    # phase entry has cli but no model — emit with model_tier if available
                    model_tier=$(echo "$phase_entry" | jq -r '.model_tier // empty' 2>/dev/null)
                    if [ -n "$model_tier" ] && [ "$model_tier" != "null" ]; then
                        _emit_llm_config_result "$cli" "$model_tier" "model_tier" "llm_config"
                    else
                        _emit_llm_config_result "$cli" "" "model" "llm_config"
                    fi
                fi
                exit 0
            fi
        fi

        # Step 2: Try _fallback
        fallback_entry=$(jq -r '._fallback // empty' < "$CONFIG_PATH" 2>/dev/null)
        if [ -n "$fallback_entry" ] && [ "$fallback_entry" != "null" ]; then
            cli=$(echo "$fallback_entry" | jq -r '.cli // empty' 2>/dev/null)
            model_tier=$(echo "$fallback_entry" | jq -r '.model_tier // empty' 2>/dev/null)

            if [ -n "$cli" ] && [ "$cli" != "null" ]; then
                if [ -z "$model_tier" ] || [ "$model_tier" = "null" ]; then
                    model_tier="sonnet"
                fi
                _emit_llm_config_result "$cli" "$model_tier" "model_tier" "llm_config_fallback"
                exit 0
            fi
        fi
    fi
fi

# ── Step 3: Profile fallback ─────────────────────────────────────────────────
profile_path=$(_find_profile "$ROLE") || {
    echo "[resolve-llm] ERROR: profile not found for role '$ROLE'" >&2
    exit 1
}

cli=$(jq -r '.cli // empty' < "$profile_path" 2>/dev/null)
model_tier=$(jq -r '.model_tier_default // empty' < "$profile_path" 2>/dev/null)

if [ -z "$cli" ] || [ "$cli" = "null" ]; then
    echo "[resolve-llm] ERROR: profile $profile_path missing .cli field" >&2
    exit 1
fi

if [ -z "$model_tier" ] || [ "$model_tier" = "null" ]; then
    model_tier="sonnet"
fi

_emit_profile_result "$cli" "$model_tier"
exit 0
