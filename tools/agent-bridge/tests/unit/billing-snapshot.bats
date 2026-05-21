#!/usr/bin/env bats
# T11 (unit) — bridge_billing_snapshot + bridge_billing_compare on synthetic data
# Live billing-path verification is in tests/billing/verify-subscription-path.bats

setup() {
  BRIDGE_LIB_DIR="${BATS_TEST_DIRNAME}/../../lib"
  export BRIDGE_LIB_DIR
  source "${BRIDGE_LIB_DIR}/billing-snapshot.sh"
  WS="$(mktemp -d "${BATS_TMPDIR:-/tmp}/bridge-t11u-XXXXXX")"
  export WS
}

teardown() {
  [[ -n "${WS:-}" && -d "$WS" ]] && rm -rf "$WS"
}

# Helper: write a synthetic snapshot JSON
synth_snap() {
  local path="$1"; shift
  jq -n "$@" > "$path"
}

@test "T11U.1 — snapshot writes valid JSON with required fields" {
  out=$(bridge_billing_snapshot "$WS" smoke)
  [ -f "$out" ]
  jq -e '.ts and .label and (.anthropic_api_key_in_env != null)' "$out"
}

@test "T11U.2 — compare with API_KEY=yes in after → FAIL rc=1" {
  before="$WS/before.json"
  after="$WS/after.json"
  synth_snap "$before" '{ts:"t1", label:"before", anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"123", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  synth_snap "$after"  '{ts:"t2", label:"after",  anthropic_api_key_in_env:"yes", anthropic_base_url_in_env:"", keychain_expires_at:"123", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  run bridge_billing_compare "$before" "$after"
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *"ANTHROPIC_API_KEY"* ]]
}

@test "T11U.3 — compare with BASE_URL set in after → FAIL rc=1" {
  before="$WS/before.json"
  after="$WS/after.json"
  synth_snap "$before" '{ts:"t1", label:"before", anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"123", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  synth_snap "$after"  '{ts:"t2", label:"after",  anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"http://localhost:4000", keychain_expires_at:"123", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  run bridge_billing_compare "$before" "$after"
  [ "$status" -eq 1 ]
  [[ "$output" == *"FAIL"* ]]
  [[ "$output" == *"BASE_URL"* ]]
}

@test "T11U.4 — compare with mtime rotation → PASS (strong)" {
  before="$WS/before.json"
  after="$WS/after.json"
  synth_snap "$before" '{ts:"t1", label:"before", anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"1000", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  synth_snap "$after"  '{ts:"t2", label:"after",  anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"2000", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  run bridge_billing_compare "$before" "$after"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS (strong)"* ]]
  [[ "$output" == *"rotated"* ]]
}

@test "T11U.5 — compare with keychain present, no rotation, no leak → PASS (strong via keychain)" {
  before="$WS/before.json"
  after="$WS/after.json"
  synth_snap "$before" '{ts:"t1", label:"before", anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"1000", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  synth_snap "$after"  '{ts:"t2", label:"after",  anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"1000", keychain_subscription_type:"max", cred_token_hash:"absent"}'
  run bridge_billing_compare "$before" "$after"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS (strong via keychain)"* ]]
}

@test "T11U.6 — compare with no keychain, no creds → INCONCLUSIVE rc=2" {
  before="$WS/before.json"
  after="$WS/after.json"
  synth_snap "$before" '{ts:"t1", label:"before", anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"", keychain_subscription_type:"", cred_token_hash:"absent"}'
  synth_snap "$after"  '{ts:"t2", label:"after",  anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"", keychain_subscription_type:"", cred_token_hash:"absent"}'
  run bridge_billing_compare "$before" "$after"
  [ "$status" -eq 2 ]
  [[ "$output" == *"INCONCLUSIVE"* ]]
}

@test "T11U.7 — compare with cred-hash rotation → PASS (legacy creds)" {
  before="$WS/before.json"
  after="$WS/after.json"
  synth_snap "$before" '{ts:"t1", label:"before", anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"", keychain_subscription_type:"", cred_token_hash:"hash-old"}'
  synth_snap "$after"  '{ts:"t2", label:"after",  anthropic_api_key_in_env:"no", anthropic_base_url_in_env:"", keychain_expires_at:"", keychain_subscription_type:"", cred_token_hash:"hash-new"}'
  run bridge_billing_compare "$before" "$after"
  [ "$status" -eq 0 ]
  [[ "$output" == *"PASS"* ]]
}

@test "T11U.8 — compare with missing snapshot file → INCONCLUSIVE rc=2" {
  run bridge_billing_compare "$WS/does-not-exist-before.json" "$WS/does-not-exist-after.json"
  [ "$status" -eq 2 ]
}

@test "T11U.9 — snapshot fails with missing args → rc=2" {
  run bridge_billing_snapshot
  [ "$status" -eq 2 ]
}

@test "T11U.10 — snapshot from real host produces valid jq-parseable JSON" {
  out=$(bridge_billing_snapshot "$WS" host)
  # Should be fully valid; all fields present
  jq -e '.anthropic_api_key_in_env and .keychain_subscription_type and (.cred_size != null)' "$out"
}
