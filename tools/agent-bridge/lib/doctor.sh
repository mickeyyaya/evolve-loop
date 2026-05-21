#!/usr/bin/env bash
# lib/doctor.sh — per-CLI auth + binary pre-flight checks
#
# Two depths:
#   shallow (default): file/env signals only, zero cost
#   --deep:            also runs a tiny live noop per headless CLI (~$0.01 each)
#
# Output: JSON when $BRIDGE_JSON=1, human table otherwise.
#
# Exit codes:
#   0  all CLIs verdict=ready
#   1  at least one verdict=warning
#   2  at least one verdict=blocked
#   10 bad flags

# --- per-CLI shallow probes ------------------------------------------------

# Each returns a JSON object describing this CLI's state.
# Common envelope: {cli, binary{present,path,version}, auth{configured,source,subscription_type,expires_at}, env_warnings[], deep_probe{}, verdict}

_doctor_binary_info() {
  local binary="$1"
  if command -v "$binary" >/dev/null 2>&1; then
    local path version
    path="$(command -v "$binary")"
    version="$("$binary" --version 2>&1 | head -1 | tr -d '\r' || echo unknown)"
    jq -n --arg path "$path" --arg version "$version" \
      '{present: true, path: $path, version: $version}'
  else
    jq -n '{present: false, path: null, version: null}'
  fi
}

# Claude auth probe (keychain first, then file fallback)
_doctor_auth_claude() {
  local kc_blob expires sub_type cred="$HOME/.claude/.credentials.json"
  if command -v security >/dev/null 2>&1; then
    kc_blob=$(security find-generic-password -s "Claude Code-credentials" -w 2>/dev/null || echo "")
    if [[ -n "$kc_blob" ]]; then
      expires=$(echo "$kc_blob" | grep -oE '"expiresAt":[0-9]+' | head -1 | cut -d: -f2)
      sub_type=$(echo "$kc_blob" | grep -oE '"subscriptionType":"[^"]*"' | head -1 | cut -d'"' -f4)
      jq -n --arg sub "$sub_type" --arg exp "${expires:-0}" \
        '{configured: true, source: "keychain", subscription_type: $sub, expires_at: ($exp | tonumber? // 0)}'
      return 0
    fi
  fi
  if [[ -r "$cred" ]]; then
    local size; size=$(wc -c < "$cred" | tr -d ' ')
    jq -n --arg size "$size" '{configured: true, source: "file:credentials.json", subscription_type: null, expires_at: null, file_size_bytes: ($size | tonumber? // 0)}'
    return 0
  fi
  jq -n '{configured: false, source: null, subscription_type: null, expires_at: null, hint: "Run `claude login` or check macOS Keychain"}'
}

# Codex auth probe (ChatGPT-account auth file)
_doctor_auth_codex() {
  local auth="$HOME/.codex/auth.json"
  if [[ -r "$auth" ]]; then
    local size; size=$(wc -c < "$auth" | tr -d ' ')
    if jq -e . "$auth" >/dev/null 2>&1; then
      jq -n --arg size "$size" '{configured: true, source: "file:~/.codex/auth.json", subscription_type: "chatgpt-account", expires_at: null, file_size_bytes: ($size | tonumber? // 0)}'
      return 0
    fi
    jq -n '{configured: false, source: null, subscription_type: null, expires_at: null, hint: "~/.codex/auth.json present but not valid JSON; re-run `codex login`"}'
    return 0
  fi
  jq -n '{configured: false, source: null, subscription_type: null, expires_at: null, hint: "Run `codex login` (ChatGPT account) or set OPENAI_API_KEY + BRIDGE_ALLOW_OPENAI_API_KEY=1"}'
}

# Agy auth probe (OAuth; vendor name in Keychain or local config dir)
_doctor_auth_agy() {
  local kc_blob
  if command -v security >/dev/null 2>&1; then
    # Try several plausible service names (agy/Antigravity may use either)
    for svc in "Antigravity" "antigravity" "agy" "Antigravity CLI"; do
      kc_blob=$(security find-generic-password -s "$svc" -w 2>/dev/null || true)
      if [[ -n "$kc_blob" ]]; then
        jq -n --arg svc "$svc" '{configured: true, source: ("keychain:" + $svc), subscription_type: "google-ai", expires_at: null}'
        return 0
      fi
    done
  fi
  # Fall back to common config dirs
  for d in "$HOME/.config/agy" "$HOME/.agy" "$HOME/Library/Application Support/Antigravity"; do
    if [[ -d "$d" ]]; then
      jq -n --arg d "$d" '{configured: true, source: ("file:" + $d), subscription_type: "google-ai", expires_at: null}'
      return 0
    fi
  done
  jq -n '{configured: false, source: null, subscription_type: null, expires_at: null, hint: "Run `agy` interactively once to trigger OAuth login + accept directory trust"}'
}

# --- env-leak warnings ------------------------------------------------------

_doctor_env_warnings() {
  local cli="$1"
  local warnings=()
  case "$cli" in
    claude-p|claude-tmux)
      [[ -n "${ANTHROPIC_API_KEY:-}" ]] && warnings+=("ANTHROPIC_API_KEY is set; would route through API billing, not subscription")
      [[ -n "${ANTHROPIC_BASE_URL:-}" ]] && warnings+=("ANTHROPIC_BASE_URL is set; proxy mode would invalidate subscription billing")
      ;;
    codex|codex-tmux)
      if [[ -n "${OPENAI_API_KEY:-}" ]] && [[ "${BRIDGE_ALLOW_OPENAI_API_KEY:-0}" != "1" ]]; then
        warnings+=("OPENAI_API_KEY is set; bridge will refuse without BRIDGE_ALLOW_OPENAI_API_KEY=1")
      fi
      ;;
    agy|agy-tmux)
      # agy is OAuth-only; env vars don't affect billing path
      ;;
  esac
  if [[ ${#warnings[@]} -eq 0 ]]; then
    echo '[]'
  else
    printf '%s\n' "${warnings[@]}" | jq -R . | jq -s .
  fi
}

# --- deep probe (live noop) ------------------------------------------------

# Runs a 30s-bounded one-shot call for the headless variant of a CLI.
# Emits {ran: bool, passed: bool|null, duration_ms: int|null, stderr_excerpt: string|null}
_doctor_deep_probe() {
  local cli="$1"
  case "$cli" in
    claude-tmux|codex-tmux|agy-tmux)
      jq -n '{ran: false, passed: null, duration_ms: null, note: "skipped: deep probe runs the headless variant only"}'
      return 0
      ;;
  esac
  local start_ms end_ms duration_ms rc=1 stderr_excerpt=""
  start_ms=$(perl -MTime::HiRes=time -e 'printf "%d", time*1000')
  local tmpf; tmpf=$(mktemp -t bridge-doctor-deep-XXXXXX)
  case "$cli" in
    claude-p)
      ( claude -p "Reply only: PROBE_OK" --model haiku ) >/dev/null 2>"$tmpf" &
      local pid=$!
      ( sleep 30 && kill $pid 2>/dev/null ) &
      local watcher=$!
      wait $pid 2>/dev/null; rc=$?
      kill $watcher 2>/dev/null || true
      ;;
    codex)
      ( echo "Reply only: PROBE_OK" | codex exec ) >/dev/null 2>"$tmpf" &
      local pid=$!
      ( sleep 30 && kill $pid 2>/dev/null ) &
      local watcher=$!
      wait $pid 2>/dev/null; rc=$?
      kill $watcher 2>/dev/null || true
      ;;
    agy)
      ( agy -p "Reply only: PROBE_OK" ) >/dev/null 2>"$tmpf" &
      local pid=$!
      ( sleep 30 && kill $pid 2>/dev/null ) &
      local watcher=$!
      wait $pid 2>/dev/null; rc=$?
      kill $watcher 2>/dev/null || true
      ;;
    *)
      rc=0
      ;;
  esac
  end_ms=$(perl -MTime::HiRes=time -e 'printf "%d", time*1000')
  duration_ms=$((end_ms - start_ms))
  stderr_excerpt=$(head -c 300 "$tmpf" 2>/dev/null | tr -d '\000' || echo "")
  rm -f "$tmpf"

  local passed=false
  [[ $rc -eq 0 ]] && passed=true

  jq -n --argjson passed "$passed" --arg duration "$duration_ms" --arg stderr "$stderr_excerpt" \
    '{ran: true, passed: $passed, duration_ms: ($duration | tonumber), stderr_excerpt: $stderr}'
}

# --- single-CLI doctor result ----------------------------------------------

_doctor_one() {
  local cli="$1"
  local deep="$2"
  local manifest_path="${BRIDGE_LIB_DIR}/manifests/${cli}.json"
  local binary="${cli%-tmux}"   # claude-tmux→claude, codex-tmux→codex, agy-tmux→agy
  case "$cli" in
    claude-p) binary="claude" ;;
  esac

  local bin_info auth_info env_warn deep_info verdict notes
  bin_info=$(_doctor_binary_info "$binary")
  case "$cli" in
    claude-p|claude-tmux) auth_info=$(_doctor_auth_claude) ;;
    codex|codex-tmux)     auth_info=$(_doctor_auth_codex) ;;
    agy|agy-tmux)         auth_info=$(_doctor_auth_agy) ;;
    *)                    auth_info='{"configured":false,"source":null,"subscription_type":null,"expires_at":null,"hint":"unknown CLI"}' ;;
  esac
  env_warn=$(_doctor_env_warnings "$cli")
  if [[ "$deep" == "1" ]]; then
    deep_info=$(_doctor_deep_probe "$cli")
  else
    deep_info='{"ran":false,"passed":null,"duration_ms":null}'
  fi

  # Verdict logic
  local bin_present auth_ok warn_count deep_ran deep_passed
  bin_present=$(echo "$bin_info"  | jq -r '.present')
  auth_ok=$(echo "$auth_info"     | jq -r '.configured')
  warn_count=$(echo "$env_warn"   | jq 'length')
  deep_ran=$(echo "$deep_info"    | jq -r '.ran')
  deep_passed=$(echo "$deep_info" | jq -r '.passed')

  if [[ "$bin_present" != "true" ]]; then
    verdict="blocked"
  elif [[ "$auth_ok" != "true" ]]; then
    verdict="blocked"
  elif [[ "$deep_ran" == "true" && "$deep_passed" != "true" ]]; then
    verdict="blocked"
  elif [[ "$warn_count" -gt 0 ]]; then
    verdict="warning"
  else
    verdict="ready"
  fi

  jq -n \
    --arg cli "$cli" \
    --arg verdict "$verdict" \
    --argjson binary "$bin_info" \
    --argjson auth "$auth_info" \
    --argjson env_warnings "$env_warn" \
    --argjson deep_probe "$deep_info" \
    '{
      cli: $cli,
      binary: $binary,
      auth: $auth,
      env_warnings: $env_warnings,
      deep_probe: $deep_probe,
      verdict: $verdict
    }'
}

# --- public entrypoint ------------------------------------------------------

# bridge_doctor [--cli=NAME] [--deep]
# Emits JSON to stdout (when $BRIDGE_JSON=1) or human table.
# Returns 0/1/2 per the verdict summary.
bridge_doctor() {
  local cli_filter="" deep=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --cli=*)   cli_filter="${1#--cli=}" ;;
      --cli)     [[ $# -ge 2 ]] || { echo "[doctor] --cli requires a value" >&2; return 10; }; cli_filter="$2"; shift ;;
      --deep)    deep=1 ;;
      --help|-h) bridge_doctor_help; return 0 ;;
      *)         echo "[doctor] unknown flag: $1" >&2; return 10 ;;
    esac
    shift
  done

  local manifests_dir="${BRIDGE_LIB_DIR}/manifests"
  local results=()
  for m in "$manifests_dir"/*.json; do
    [[ -f "$m" ]] || continue
    local cli; cli=$(basename "$m" .json)
    if [[ -n "$cli_filter" && "$cli" != "$cli_filter" ]]; then
      continue
    fi
    results+=("$(_doctor_one "$cli" "$deep")")
  done

  if [[ ${#results[@]} -eq 0 ]]; then
    echo "[doctor] no CLIs match filter: ${cli_filter:-<none>}" >&2
    return 10
  fi

  local results_json
  results_json=$(printf '%s\n' "${results[@]}" | jq -s .)
  local ready warning blocked
  ready=$(  echo "$results_json" | jq '[.[] | select(.verdict=="ready"  )] | length')
  warning=$(echo "$results_json" | jq '[.[] | select(.verdict=="warning")] | length')
  blocked=$(echo "$results_json" | jq '[.[] | select(.verdict=="blocked")] | length')

  if [[ "${BRIDGE_JSON:-0}" == "1" ]]; then
    jq -n \
      --arg scanned_at "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
      --arg host "$(uname -s)/$(uname -r)" \
      --argjson deep "$deep" \
      --argjson results "$results_json" \
      --argjson ready "$ready" --argjson warning "$warning" --argjson blocked "$blocked" \
      '{
        scanned_at: $scanned_at,
        host: $host,
        deep: ($deep == 1),
        results: $results,
        summary: {ready: $ready, warning: $warning, blocked: $blocked}
      }'
  else
    # Human-readable table
    printf '\n%-14s %-8s  %s\n' "CLI" "VERDICT" "NOTES"
    printf '%-14s %-8s  %s\n' "----" "-------" "-----"
    echo "$results_json" | jq -r '.[] |
      [.cli, .verdict,
       (if .binary.present then
          (.binary.version // "unknown") + " · " +
          (if .auth.configured then "auth=" + (.auth.source // "?") else "auth=MISSING" end) +
          (if (.env_warnings | length) > 0 then "  ⚠ " + (.env_warnings[0]) else "" end) +
          (if .deep_probe.ran then "  · deep=" + (if .deep_probe.passed then "PASS" else "FAIL" end) + "(" + (.deep_probe.duration_ms | tostring) + "ms)" else "" end)
        else "binary not on PATH"
        end)
      ] | @tsv' | while IFS=$'\t' read -r col_cli col_verdict col_notes; do
        printf '%-14s %-8s  %s\n' "$col_cli" "$col_verdict" "$col_notes"
      done
    printf '\nsummary: ready=%d warning=%d blocked=%d\n' "$ready" "$warning" "$blocked"
  fi

  if [[ "$blocked" -gt 0 ]]; then return 2; fi
  if [[ "$warning" -gt 0 ]]; then return 1; fi
  return 0
}

bridge_doctor_help() {
  cat <<'DH'
bridge doctor — pre-flight auth + binary check for each CLI

Usage:
  bridge [--json] doctor [--cli=NAME] [--deep]

Flags:
  --cli=NAME    Restrict to a single CLI (e.g. claude-tmux)
  --deep        Also run a tiny live noop call per headless CLI (~$0.01 each)
                Tmux variants are skipped — `--deep` covers their headless
                backend, which is a reasonable proxy for "auth is alive".
  --json        (top-level) Emit JSON to stdout instead of a human table

Exit codes:
  0  all CLIs verdict=ready
  1  at least one verdict=warning
  2  at least one verdict=blocked
  10 bad flags

JSON schema (when --json):
  {
    "scanned_at": "ISO-8601",
    "host": "Darwin/25.4.0",
    "deep": false,
    "results": [
      {
        "cli": "claude-p",
        "binary": {"present": bool, "path": "…", "version": "…"},
        "auth": {"configured": bool, "source": "keychain|file|null", "subscription_type": "…", "expires_at": int|null},
        "env_warnings": [string, …],
        "deep_probe": {"ran": bool, "passed": bool|null, "duration_ms": int|null},
        "verdict": "ready|warning|blocked"
      }, …
    ],
    "summary": {"ready": N, "warning": N, "blocked": N}
  }
DH
}
