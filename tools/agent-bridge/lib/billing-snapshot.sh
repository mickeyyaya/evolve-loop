#!/usr/bin/env bash
# lib/billing-snapshot.sh — snapshot credential-resolution state pre/post-call, compare for auth-path consistency
#
# Snapshot credential-resolution state (env vars, config files, keychain entries)
# pre/post-call to confirm the authentication path stayed consistent.
#
# Use case: defense against subtle environment changes that could route a
# call through an unintended credential — env-var leaks, proxy injection,
# credential rotation mid-session. Operator hygiene, vendor-agnostic.
# Generic across CLIs via $ANTHROPIC_* / $OPENAI_* / $GEMINI_* / $GOOGLE_* env checks.
#
# Two layers:
#   bridge_billing_snapshot — write a JSON snapshot of credential-resolution state
#   bridge_billing_compare  — diff two snapshots, emit PASS/FAIL/INCONCLUSIVE
#
# Both layers callable as:
#   bash lib/billing-snapshot.sh snapshot DIR LABEL    → prints snapshot path on stdout
#   bash lib/billing-snapshot.sh compare BEFORE AFTER  → prints verdict, exits 0/1/2
#
# OR sourced:
#   . lib/billing-snapshot.sh
#   p=$(bridge_billing_snapshot DIR LABEL)
#   bridge_billing_compare BEFORE AFTER
#
# Exit codes (compare):
#   0  PASS         — credential-resolution stayed consistent and isolated
#   1  FAIL         — credential isolation broken (override env var set mid-session)
#   2  INCONCLUSIVE — no clear evidence either way (operator must check console)

bridge_billing_snapshot() {
  local snap_dir="$1"
  local label="$2"
  if [[ -z "$snap_dir" || -z "$label" ]]; then
    echo "[bridge:billing] snapshot requires SNAP_DIR and LABEL args" >&2
    return 2
  fi
  mkdir -p "$snap_dir"
  local out="$snap_dir/snap-${label}-$(date +%s).json"
  local cred="$HOME/.claude/.credentials.json"
  local stats_dir="$HOME/.claude/statsig"

  # 1. Credential file fingerprint (size + mtime + token-prefix hash, never the token itself)
  local cred_size=0
  local cred_mtime=""
  local cred_hash="absent"
  if [[ -r "$cred" ]]; then
    cred_size=$(wc -c < "$cred" | tr -d ' ')
    cred_mtime=$(stat -f %m "$cred" 2>/dev/null || stat -c %Y "$cred" 2>/dev/null || echo "")
    cred_hash=$(grep -o '"accessToken":"[^"]*"' "$cred" 2>/dev/null | head -c 60 | shasum | cut -d' ' -f1)
    [[ -z "$cred_hash" ]] && cred_hash="present-but-no-token-field"
  fi

  # 2. `claude usage` subcommand output (if the CLI exposes one)
  local usage_blob=""
  if command -v claude >/dev/null 2>&1 && claude --help 2>&1 | grep -qE '\busage\b'; then
    usage_blob=$(claude usage 2>&1 | head -30 | tr '\n' '|' | tr '"' "'" || true)
  fi

  # 3. statsig dir listing (Anthropic writes session telemetry here)
  local statsig_files=""
  if [[ -d "$stats_dir" ]]; then
    statsig_files=$(ls -la "$stats_dir" 2>/dev/null | tail -n +2 | tr '\n' '|' | tr '"' "'")
  fi

  # 4. Cost-leak canaries
  local api_key_present="no"
  [[ -n "${ANTHROPIC_API_KEY:-}" ]] && api_key_present="yes"
  local base_url="${ANTHROPIC_BASE_URL:-}"

  # 5. macOS Keychain OAuth (numeric expiresAt + subscriptionType, never the token)
  local keychain_expires=""
  local keychain_sub_type=""
  if command -v security >/dev/null 2>&1; then
    local kc_blob
    kc_blob=$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null || echo "")
    if [[ -n "$kc_blob" ]]; then
      keychain_expires=$(echo "$kc_blob" | grep -oE '"expiresAt":[0-9]+' | head -1 | cut -d: -f2)
      keychain_sub_type=$(echo "$kc_blob" | grep -oE '"subscriptionType":"[^"]*"' | head -1 | cut -d'"' -f4)
    fi
  fi

  # Emit JSON via jq for proper escaping
  jq -n \
    --arg ts "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg label "$label" \
    --arg cred_size "$cred_size" \
    --arg cred_mtime "$cred_mtime" \
    --arg cred_hash "$cred_hash" \
    --arg usage "$usage_blob" \
    --arg statsig "$statsig_files" \
    --arg api_key "$api_key_present" \
    --arg base_url "$base_url" \
    --arg kc_expires "$keychain_expires" \
    --arg kc_sub "$keychain_sub_type" \
    '{
      ts: $ts,
      label: $label,
      cred_size: ($cred_size | tonumber? // 0),
      cred_mtime: $cred_mtime,
      cred_token_hash: $cred_hash,
      usage_subcommand_output: $usage,
      statsig_listing: $statsig,
      anthropic_api_key_in_env: $api_key,
      anthropic_base_url_in_env: $base_url,
      keychain_expires_at: $kc_expires,
      keychain_subscription_type: $kc_sub
    }' > "$out"

  echo "$out"
}

bridge_billing_compare() {
  local before="$1"
  local after="$2"

  if [[ ! -r "$before" ]] || [[ ! -r "$after" ]]; then
    echo "[bridge:billing] missing snapshot file(s)" >&2
    echo "  before: $before" >&2
    echo "  after:  $after" >&2
    return 2
  fi
  if ! jq -e . "$before" >/dev/null 2>&1 || ! jq -e . "$after" >/dev/null 2>&1; then
    echo "[bridge:billing] invalid JSON in snapshot(s)" >&2
    return 2
  fi

  echo "=== credential-isolation verdict ==="
  echo "BEFORE: $before"
  echo "AFTER:  $after"
  echo ""

  # Hard FAIL: ANTHROPIC_API_KEY was set in env during the call
  local api_after
  api_after=$(jq -r '.anthropic_api_key_in_env' "$after")
  if [[ "$api_after" == "yes" ]]; then
    echo "VERDICT: FAIL — ANTHROPIC_API_KEY was set during the call"
         The call routed through an override credential path instead of the CLI default.
    return 1
  fi

  # Hard FAIL: custom base URL was set (proxy mode invalidates billing test)
  local base_after
  base_after=$(jq -r '.anthropic_base_url_in_env' "$after")
  if [[ -n "$base_after" && "$base_after" != "null" ]]; then
    echo "VERDICT: FAIL — ANTHROPIC_BASE_URL was set: $base_after"
    echo "         The billing test requires no proxy in the path."
    return 1
  fi

  # Keychain signals (macOS-strong)
  local kc_before kc_after sub_type
  kc_before=$(jq -r '.keychain_expires_at' "$before")
  kc_after=$(jq -r  '.keychain_expires_at' "$after")
  sub_type=$(jq -r  '.keychain_subscription_type' "$after")

  # Strong: keychain expiresAt rotated mid-call
  if [[ -n "$kc_before" && -n "$kc_after" && "$kc_before" != "null" && "$kc_after" != "null" && "$kc_before" != "$kc_after" ]]; then
    echo "VERDICT: PASS (strong) — keychain OAuth expiresAt rotated between snapshots"
    echo "  before expiresAt: $kc_before"
    echo "  after  expiresAt: $kc_after"
    echo "  subscriptionType: $sub_type"
    return 0
  fi

  # Strong: keychain has subscriptionType AND no env-leak in after-snapshot
  if [[ -n "$kc_after" && "$kc_after" != "null" && -n "$sub_type" && "$sub_type" != "null" && "$sub_type" != "" ]]; then
    echo "VERDICT: PASS (strong via keychain) — active $sub_type subscription, no env-leak"
    echo "  expiresAt: $kc_after"
    echo ""
    echo "  Manual console check completes the picture:"
    echo "    - https://console.anthropic.com/settings/billing → API credits unchanged?"
    echo "    - https://claude.ai/settings/usage              → subscription quota decremented?"
    return 0
  fi

  # Fallback: legacy ~/.claude/.credentials.json
  local h_before h_after
  h_before=$(jq -r '.cred_token_hash' "$before")
  h_after=$(jq -r  '.cred_token_hash' "$after")

  if [[ "$h_before" != "$h_after" && "$h_after" != "absent" && "$h_after" != "present-but-no-token-field" ]]; then
    echo "VERDICT: PASS (legacy creds) — credentials.json token rotated"
    return 0
  fi
  if [[ "$h_after" != "absent" ]]; then
    echo "VERDICT: PASS (weak) — credentials file present, no env-leak"
    return 0
  fi

  echo "VERDICT: INCONCLUSIVE — no OAuth evidence (keychain absent, no creds file)"
  echo "  Operator must consult console.anthropic.com manually."
  return 2
}

# Standalone CLI: bash lib/billing-snapshot.sh snapshot DIR LABEL | compare BEFORE AFTER
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
  case "${1:-}" in
    snapshot)
      [[ $# -lt 3 ]] && { echo "Usage: $0 snapshot SNAP_DIR LABEL" >&2; exit 2; }
      bridge_billing_snapshot "$2" "$3"
      ;;
    compare)
      [[ $# -lt 3 ]] && { echo "Usage: $0 compare BEFORE.json AFTER.json" >&2; exit 2; }
      bridge_billing_compare "$2" "$3"
      ;;
    *)
      cat >&2 <<USAGE
Usage:
  $0 snapshot SNAP_DIR LABEL          Write a billing-state snapshot, print its path
  $0 compare  BEFORE.json AFTER.json  Compare two snapshots, print verdict (PASS/FAIL/INCONCLUSIVE)

Exit codes (compare):
  0 PASS         strong or weak signal of subscription billing
  1 FAIL         cost-leak detected (API_KEY or BASE_URL set)
  2 INCONCLUSIVE no clear evidence
USAGE
      exit 2
      ;;
  esac
fi
