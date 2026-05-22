#!/usr/bin/env bash
# doctor-subscription-auth.sh — Detect which credential path evolve-loop will use.
#
# Vendor-agnostic: just inspects which credential signals are present in the
# operator's environment and reports the resolution order. Does not opine
# on which mode the operator should choose — that is configured at the CLI
# installation level, not here.
#
# Auth mode detection order (first match wins):
#   1. EVOLVE_ANTHROPIC_BASE_URL or ANTHROPIC_BASE_URL set → CUSTOM_PROXY
#   2. ANTHROPIC_API_KEY set → API_KEY
#   3. ~/.claude/.credentials.json readable + claudeAiOauth.accessToken non-empty → SUBSCRIPTION_OAUTH
#   4. Otherwise → MISCONFIGURED
#
# Usage:
#   bash scripts/utility/doctor-subscription-auth.sh
#   bash scripts/utility/doctor-subscription-auth.sh --json
#   bash scripts/utility/doctor-subscription-auth.sh --help
#
# Exit code: always 0 (advisory only)

set -uo pipefail

_usage() {
    cat >&2 <<'EOF'
Usage: bash scripts/utility/doctor-subscription-auth.sh [--json] [--help]

Flags:
  --json    Emit {"mode":"...","notes":"..."} on stdout
  --help    Print this help and exit

Auth modes detected:
  CUSTOM_PROXY        EVOLVE_ANTHROPIC_BASE_URL or ANTHROPIC_BASE_URL is set
  API_KEY             ANTHROPIC_API_KEY is set
  SUBSCRIPTION_OAUTH  ~/.claude/.credentials.json has a valid OAuth token
  MISCONFIGURED       None of the above — cannot authenticate

Exit code is always 0 (advisory only).
EOF
}

# --- credential file path (overridable in tests) ---
_CRED_FILE="${EVOLVE_DOCTOR_CRED_FILE_OVERRIDE:-$HOME/.claude/.credentials.json}"

_detect_mode() {
    # Priority 1: custom proxy endpoint
    if [ -n "${EVOLVE_ANTHROPIC_BASE_URL:-}" ] || [ -n "${ANTHROPIC_BASE_URL:-}" ]; then
        printf 'CUSTOM_PROXY'
        return
    fi

    # Priority 2: API key
    if [ -n "${ANTHROPIC_API_KEY:-}" ]; then
        printf 'API_KEY'
        return
    fi

    # Priority 3: subscription OAuth token
    if [ -r "$_CRED_FILE" ]; then
        # Use grep -o to extract token value without jq (bash-3.2 safe)
        local token
        token=$(grep -o '"accessToken":"[^"]*"' "$_CRED_FILE" 2>/dev/null \
                | grep -o '"[^"]*"$' \
                | tr -d '"' \
                | tr -d '[:space:]' \
                || true)
        if [ -n "$token" ]; then
            printf 'SUBSCRIPTION_OAUTH'
            return
        fi
    fi

    printf 'MISCONFIGURED'
}

_notes_for_mode() {
    local mode="$1"
    case "$mode" in
        CUSTOM_PROXY)
            local base="${EVOLVE_ANTHROPIC_BASE_URL:-${ANTHROPIC_BASE_URL:-}}"
            printf 'Routing via custom proxy endpoint: %s' "$base"
            ;;
        API_KEY)
            printf 'Using ANTHROPIC_API_KEY — API credits will be deducted per call'
            ;;
        SUBSCRIPTION_OAUTH)
            printf 'Using Claude Code subscription auth (~/.claude/.credentials.json)'
            ;;
        MISCONFIGURED)
            printf 'No valid auth found. Set ANTHROPIC_API_KEY, EVOLVE_ANTHROPIC_BASE_URL, or log in via Claude Code'
            ;;
        *)
            printf 'Unknown mode: %s' "$mode"
            ;;
    esac
}

_next_step_for_mode() {
    local mode="$1"
    case "$mode" in
        CUSTOM_PROXY)
            printf 'Verify your proxy endpoint is reachable and speaks POST /v1/messages'
            ;;
        API_KEY)
            printf 'No action needed; ensure your key has sufficient credits'
            ;;
        SUBSCRIPTION_OAUTH)
            printf 'No action needed; subscription auth active'
            ;;
        MISCONFIGURED)
            printf 'Run: claude login  (or export ANTHROPIC_API_KEY=<key>)'
            ;;
    esac
}

_json_escape() {
    # Minimal JSON string escaping without external tools (bash-3.2 safe)
    local s="$1"
    # Escape backslash first, then double-quote, then control characters
    s="${s//\\/\\\\}"
    s="${s//\"/\\\"}"
    printf '%s' "$s"
}

main() {
    local json_mode=0

    for arg in "$@"; do
        case "$arg" in
            --help|-h) _usage; exit 0 ;;
            --json)    json_mode=1 ;;
            *) printf '[doctor-subscription-auth] Unknown flag: %s\n' "$arg" >&2 ;;
        esac
    done

    local mode
    mode=$(_detect_mode)

    local notes
    notes=$(_notes_for_mode "$mode")

    local next_step
    next_step=$(_next_step_for_mode "$mode")

    if [ "$json_mode" = "1" ]; then
        local notes_esc next_esc
        notes_esc=$(_json_escape "$notes")
        next_esc=$(_json_escape "$next_step")
        printf '{"mode":"%s","notes":"%s","next_step":"%s"}\n' \
            "$mode" "$notes_esc" "$next_esc"
    else
        printf '[auth-mode] %s\n' "$mode"
        printf '[notes]     %s\n' "$notes"
        printf '[next]      %s\n' "$next_step"
    fi
}

main "$@"
exit 0
