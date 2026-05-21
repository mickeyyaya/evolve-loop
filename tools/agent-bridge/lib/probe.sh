#!/usr/bin/env bash
# lib/probe.sh — detect available CLIs and their capability tier
#
# Usage (sourced; depends on manifest-loader.sh having been sourced too):
#   . lib/manifest-loader.sh
#   . lib/probe.sh
#   probe_all > probe.json
#   probe_one claude-tmux > one.json
#
# Tier semantics (per cli manifest):
#   full     — binary present + min version met + all tier_dependencies[full] satisfied
#   hybrid   — declared as max tier; all tier_dependencies[hybrid] satisfied
#   degraded — binary present but some declared dependencies missing
#   none     — binary missing OR manifest marks stub=true

# Resolve effective tier for a CLI based on manifest + on-host binary presence.
# Outputs the tier string on stdout (one of: full|hybrid|degraded|none).
probe_resolve_tier() {
  local cli="$1"
  if ! manifest_load "$cli" >/dev/null; then
    echo "none"
    return 0
  fi
  if [[ "$bridge_manifest_stub" == "true" ]]; then
    echo "none"
    return 0
  fi
  if ! command -v "$bridge_manifest_binary" >/dev/null 2>&1; then
    echo "none"
    return 0
  fi

  local declared="$bridge_manifest_default_tier"
  if [[ -z "$declared" || "$declared" == "none" ]]; then
    declared="full"
  fi

  # Verify all dependencies for the declared tier
  local deps_for_tier
  deps_for_tier=$(jq -r ".tier_dependencies[\"$declared\"] // [] | .[]" "$bridge_manifest_path" 2>/dev/null)
  local all_deps_present=1
  while IFS= read -r dep; do
    [[ -z "$dep" ]] && continue
    if ! command -v "$dep" >/dev/null 2>&1; then
      all_deps_present=0
      break
    fi
  done <<<"$deps_for_tier"

  if [[ $all_deps_present -eq 1 ]]; then
    echo "$declared"
  else
    echo "degraded"
  fi
}

# Emit a JSON object for a single CLI: {cli, tier, binary, version, stub}
probe_one() {
  local cli="$1"
  if ! manifest_load "$cli" >/dev/null; then
    jq -n --arg c "$cli" '{cli:$c, tier:"none", binary:null, version:null, stub:false, error:"manifest not found"}'
    return 0
  fi

  local tier; tier=$(probe_resolve_tier "$cli")
  local binary_path="" version=""

  if [[ "$bridge_manifest_stub" != "true" ]] && command -v "$bridge_manifest_binary" >/dev/null 2>&1; then
    binary_path=$(command -v "$bridge_manifest_binary")
    # Best-effort version extraction; CLIs differ in --version output shape
    version=$("$bridge_manifest_binary" --version 2>&1 | head -1 | tr -d '\r')
  fi

  jq -n \
    --arg cli "$cli" --arg tier "$tier" \
    --arg binary "$binary_path" --arg version "$version" \
    --argjson stub "$bridge_manifest_stub" \
    '{
       cli: $cli,
       tier: $tier,
       binary: (if $binary == "" then null else $binary end),
       version: (if $version == "" then null else $version end),
       stub: $stub
     }'
}

# Emit JSON for all known CLIs (one per manifest in lib/manifests/).
# Optional first arg: filter to a single CLI (used by --cli= flag).
probe_all() {
  local filter="${1:-}"
  local manifests_dir="${BRIDGE_LIB_DIR}/manifests"
  if [[ ! -d "$manifests_dir" ]]; then
    echo "[bridge:probe] manifests dir missing: $manifests_dir" >&2
    return 1
  fi

  local results_file
  results_file="$(mktemp -t bridge-probe-XXXXXX)"
  printf '[' > "$results_file"
  local first=1
  for m in "$manifests_dir"/*.json; do
    [[ -f "$m" ]] || continue
    local cli; cli=$(basename "$m" .json)
    if [[ -n "$filter" && "$cli" != "$filter" ]]; then
      continue
    fi
    if [[ $first -eq 1 ]]; then first=0; else printf ',' >> "$results_file"; fi
    probe_one "$cli" >> "$results_file"
  done
  printf ']' >> "$results_file"

  local os_str
  os_str="$(uname -s)/$(uname -r)"

  jq -n --arg os "$os_str" --slurpfile results "$results_file" \
    '{os:$os, results:($results[0])}'

  rm -f "$results_file"
  return 0
}
