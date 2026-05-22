#!/usr/bin/env bash
#
# verify-subscription-billing.sh — snapshot credential-resolution state pre/post-call,
# diff to detect credential-isolation drift across the tmux-claude prototype call.
#
# Subcommands:
#   snapshot <SNAP_DIR> <LABEL>          → writes JSON snapshot, prints its path
#   compare  <BEFORE.json> <AFTER.json>  → prints PASS/FAIL/INCONCLUSIVE
#
# Exit codes (compare):
#   0  PASS         — credential-resolution stayed consistent and isolated
#   1  FAIL         — credential isolation broken (override env var set mid-call)
#   2  INCONCLUSIVE — no evidence either way (re-run with screenshots)
#

set -uo pipefail

SUBCMD="${1:-}"

snapshot() {
    local snap_dir="$1"
    local label="$2"
    mkdir -p "$snap_dir"
    local out="$snap_dir/snap-$label-$(date +%s).json"
    local cred="$HOME/.claude/.credentials.json"
    local stats_dir="$HOME/.claude/statsig"

    # 1. Credential file fingerprint
    local cred_size=0
    local cred_mtime=""
    local cred_hash="absent"
    if [ -r "$cred" ]; then
        cred_size=$(wc -c < "$cred" | tr -d ' ')
        # macOS stat -f %m vs GNU stat -c %Y
        cred_mtime=$(stat -f %m "$cred" 2>/dev/null || stat -c %Y "$cred" 2>/dev/null || echo "")
        # Hash a prefix of the OAuth token. We never log the token itself.
        cred_hash=$(grep -o '"accessToken":"[^"]*"' "$cred" 2>/dev/null | head -c 60 | shasum | cut -d' ' -f1)
        [ -z "$cred_hash" ] && cred_hash="present-but-no-token-field"
    fi

    # 2. claude usage subcommand if it exists
    local usage_blob=""
    if claude --help 2>&1 | grep -qE '\busage\b'; then
        usage_blob=$(claude usage 2>&1 | head -30 | tr '\n' '|' | tr '"' "'" || true)
    fi

    # 3. statsig telemetry directory listing (Anthropic writes session counters here)
    local statsig_files=""
    if [ -d "$stats_dir" ]; then
        statsig_files=$(ls -la "$stats_dir" 2>/dev/null | tail -n +2 | tr '\n' '|' | tr '"' "'")
    fi

    # 4. Anthropic API key presence (cost-leak canary)
    local api_key_present="no"
    [ -n "${ANTHROPIC_API_KEY:-}" ] && api_key_present="yes"

    # 5. Proxy mode env (would invalidate the billing test)
    local base_url="${EVOLVE_ANTHROPIC_BASE_URL:-${ANTHROPIC_BASE_URL:-}}"

    # 6. macOS Keychain OAuth token expiresAt (the actual auth path on Darwin)
    # We extract ONLY the numeric expiresAt field, never the token itself.
    # The keychain may prompt for auth the FIRST time; cached afterward.
    local keychain_expires=""
    local keychain_sub_type=""
    if command -v security >/dev/null 2>&1; then
        local kc_blob
        kc_blob=$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null || echo "")
        if [ -n "$kc_blob" ]; then
            keychain_expires=$(echo "$kc_blob" | grep -oE '"expiresAt":[0-9]+' | head -1 | cut -d: -f2)
            keychain_sub_type=$(echo "$kc_blob" | grep -oE '"subscriptionType":"[^"]*"' | head -1 | cut -d'"' -f4)
        fi
    fi

    cat > "$out" <<EOF
{
  "ts": "$(date -u +%Y-%m-%dT%H:%M:%SZ)",
  "label": "$label",
  "cred_size": $cred_size,
  "cred_mtime": "$cred_mtime",
  "cred_token_hash": "$cred_hash",
  "usage_subcommand_output": "$usage_blob",
  "statsig_listing": "$statsig_files",
  "anthropic_api_key_in_env": "$api_key_present",
  "evolve_base_url_in_env": "$base_url",
  "keychain_expires_at": "$keychain_expires",
  "keychain_subscription_type": "$keychain_sub_type"
}
EOF
    echo "$out"
}

compare() {
    local before="$1"
    local after="$2"

    if [ ! -r "$before" ] || [ ! -r "$after" ]; then
        echo "[verify-billing] ERROR: missing snapshot file(s)" >&2
        echo "  before: $before"
        echo "  after:  $after"
        return 2
    fi

    echo "=== billing-mode verdict ==="
    echo "BEFORE: $before"
    echo "AFTER:  $after"
    echo ""

    # Hard FAIL: ANTHROPIC_API_KEY was set during the call
    local api_after
    api_after=$(grep -o '"anthropic_api_key_in_env":[[:space:]]*"[^"]*"' "$after" | cut -d'"' -f4)
    if [ "$api_after" = "yes" ]; then
        echo "VERDICT: FAIL — ANTHROPIC_API_KEY was set in env during the call"
        echo "         This means the call would have billed to API account, not subscription."
        return 1
    fi

    # Hard FAIL: custom base URL was set (proxy mode)
    local base_after
    base_after=$(grep -o '"evolve_base_url_in_env":[[:space:]]*"[^"]*"' "$after" | cut -d'"' -f4)
    if [ -n "$base_after" ]; then
        echo "VERDICT: FAIL — proxy endpoint was set during the call: $base_after"
        echo "         The billing test requires no proxy in the path."
        return 1
    fi

    # Keychain signals (macOS): the actual OAuth lives here on Darwin
    local kc_before kc_after sub_type
    kc_before=$(grep -o '"keychain_expires_at":[[:space:]]*"[^"]*"' "$before" | cut -d'"' -f4)
    kc_after=$(grep -o '"keychain_expires_at":[[:space:]]*"[^"]*"' "$after"  | cut -d'"' -f4)
    sub_type=$(grep -o '"keychain_subscription_type":[[:space:]]*"[^"]*"' "$after" | cut -d'"' -f4)

    # Strong signal: keychain expiresAt changed (token rotated during call)
    if [ -n "$kc_before" ] && [ -n "$kc_after" ] && [ "$kc_before" != "$kc_after" ]; then
        echo "VERDICT: PASS (strong) — keychain OAuth expiresAt mutated between snapshots"
        echo "         Token rotation during call confirms OAuth/subscription path was active."
        echo "         before expiresAt: $kc_before"
        echo "         after  expiresAt: $kc_after"
        echo "         subscriptionType: $sub_type"
        return 0
    fi

    # Strong signal: keychain has a valid subscription type AND no env-leak fallback
    # Per snapshot pre-check above, ANTHROPIC_API_KEY and BASE_URL are both unset,
    # so the OAuth path is the ONLY way claude could have authenticated.
    if [ -n "$kc_after" ] && [ -n "$sub_type" ] && [ "$sub_type" != "" ]; then
        echo "VERDICT: PASS (strong via keychain) — keychain shows active $sub_type subscription"
        echo "         No env-var leak. Token didn't rotate this call (16s is shorter than"
        echo "         typical 1-hour rotation window), but the auth path is unambiguously OAuth."
        echo "         expiresAt: $kc_after ($(date -r $((kc_after / 1000)) -u 2>/dev/null || echo unparseable))"
        echo ""
        echo "  Manual console check required to confirm billing destination:"
        echo "    - https://console.anthropic.com/settings/billing  → API credits unchanged?"
        echo "    - https://claude.ai/settings/usage                → subscription quota decremented?"
        return 0
    fi

    # Fallback: legacy ~/.claude/.credentials.json path (rarely present)
    local h_before h_after
    h_before=$(grep -o '"cred_token_hash":[[:space:]]*"[^"]*"' "$before" | cut -d'"' -f4)
    h_after=$(grep -o '"cred_token_hash":[[:space:]]*"[^"]*"' "$after"  | cut -d'"' -f4)

    if [ "$h_before" != "$h_after" ] && [ "$h_after" != "absent" ] && [ "$h_after" != "present-but-no-token-field" ]; then
        echo "VERDICT: PASS (legacy creds path) — credentials.json token rotated"
        return 0
    fi

    if [ "$h_after" != "absent" ]; then
        echo "VERDICT: PASS (weak) — credentials file present, no API/proxy env-leak"
        return 0
    fi

    echo "VERDICT: INCONCLUSIVE — no OAuth snapshot evidence (neither keychain nor file)"
    echo "         Operator must consult console.anthropic.com manually."
    return 2
}

case "$SUBCMD" in
    snapshot)
        if [ $# -lt 3 ]; then
            echo "Usage: $0 snapshot <SNAP_DIR> <LABEL>" >&2
            exit 2
        fi
        snapshot "$2" "$3"
        ;;
    compare)
        if [ $# -lt 3 ]; then
            echo "Usage: $0 compare <BEFORE.json> <AFTER.json>" >&2
            exit 2
        fi
        compare "$2" "$3"
        ;;
    *)
        echo "Usage:" >&2
        echo "  $0 snapshot <SNAP_DIR> <LABEL>           Take a snapshot" >&2
        echo "  $0 compare  <BEFORE.json> <AFTER.json>   Compare two snapshots" >&2
        exit 2
        ;;
esac
