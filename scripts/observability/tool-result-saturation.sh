#!/usr/bin/env bash
# tool-result-saturation.sh — c45-P-NEW-6 (cycle 36)
# Reads context-monitor.json from a cycle workspace and estimates
# tool-result saturation per phase. Emits a diagnostic table and
# recommendation when overhead exceeds thresholds.
#
# Usage:
#   tool-result-saturation.sh [--workspace <path>] [--cycle <N>] [--json]
#   tool-result-saturation.sh --help
#
# Exit codes: 0 (clean or no data), 1 (usage error)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$REPO_ROOT}"

_workspace=""
_cycle=""
_json_mode=0

usage() {
    cat <<EOF
Usage: $(basename "$0") [OPTIONS]

Estimate tool-result saturation from a cycle's context-monitor.json.

Options:
  --workspace <path>   Path to cycle workspace (e.g. .evolve/runs/cycle-N)
  --cycle <N>          Cycle number; derives workspace as .evolve/runs/cycle-N
  --json               Emit JSON output instead of table
  --help               Show this help

If neither --workspace nor --cycle is given, uses the current cycle from
cycle-state.json if available.

Exit 0: success (or no data available). Exit 1: usage error.
EOF
}

while [ $# -gt 0 ]; do
    case "$1" in
        --workspace) _workspace="$2"; shift 2 ;;
        --cycle)     _cycle="$2"; shift 2 ;;
        --json)      _json_mode=1; shift ;;
        --help)      usage; exit 0 ;;
        *) echo "Unknown option: $1" >&2; usage >&2; exit 1 ;;
    esac
done

# Resolve workspace path
if [ -z "$_workspace" ]; then
    if [ -n "$_cycle" ]; then
        _workspace="$PROJECT_ROOT/.evolve/runs/cycle-${_cycle}"
    elif [ -f "$PROJECT_ROOT/.evolve/cycle-state.json" ] && command -v jq >/dev/null 2>&1; then
        _cid=$(jq -r '.cycle_id // empty' "$PROJECT_ROOT/.evolve/cycle-state.json" 2>/dev/null || true)
        [ -n "$_cid" ] && _workspace="$PROJECT_ROOT/.evolve/runs/cycle-${_cid}"
    fi
fi

_monitor=""
if [ -n "$_workspace" ]; then
    _monitor="${_workspace}/context-monitor.json"
fi

# Empty workspace — exit 0 cleanly (diagnostic script must not block pipelines)
if [ -z "$_monitor" ] || [ ! -f "$_monitor" ]; then
    echo "[tool-result-saturation] No context-monitor.json found (workspace=${_workspace:-<unresolved>}). Nothing to report." >&2
    exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
    echo "[tool-result-saturation] jq not available; cannot parse context-monitor.json." >&2
    exit 0
fi

# Parse per-phase entries from context-monitor.json.
# Schema (v9.1.0): { "<agent>": { "input_tokens": N, "cap": N, "cap_pct": N, "ts": "..." } }
_phases=$(jq -r 'keys[]' "$_monitor" 2>/dev/null | sort || true)

if [ -z "$_phases" ]; then
    echo "[tool-result-saturation] context-monitor.json is empty or unparseable." >&2
    exit 0
fi

if [ "$_json_mode" = "1" ]; then
    jq --argjson threshold 0.7 '
      to_entries | map({
        phase: .key,
        input_tokens: .value.input_tokens,
        cap: .value.cap,
        cap_pct: .value.cap_pct,
        saturation_flag: (if (.value.cap_pct // 0) >= ($threshold * 100) then "HIGH" else "ok" end)
      })' "$_monitor"
    exit 0
fi

# Table mode
printf "\n%-20s %12s %8s %8s  %s\n" "Phase" "InputTokens" "Cap" "Cap%" "Saturation"
printf "%-20s %12s %8s %8s  %s\n" "--------------------" "------------" "--------" "--------" "----------"

_high_count=0
while IFS= read -r _phase; do
    _tokens=$(jq -r ".\"${_phase}\".input_tokens // 0" "$_monitor" 2>/dev/null || echo 0)
    _cap=$(jq -r ".\"${_phase}\".cap // 0" "$_monitor" 2>/dev/null || echo 0)
    _pct=$(jq -r ".\"${_phase}\".cap_pct // 0" "$_monitor" 2>/dev/null || echo 0)
    _flag="ok"
    if [ "$_pct" -ge 70 ] 2>/dev/null; then
        _flag="HIGH"
        _high_count=$((_high_count + 1))
    fi
    printf "%-20s %12s %8s %7s%%  %s\n" "$_phase" "$_tokens" "$_cap" "$_pct" "$_flag"
done <<EOF
$_phases
EOF

echo ""
if [ "$_high_count" -gt 0 ]; then
    echo "RECOMMENDATION: ${_high_count} phase(s) at >=70% context saturation."
    echo "  Apply Tool-Result Hygiene rules in those phases (see agents/evolve-*.md)."
    echo "  Consider EVOLVE_CONTEXT_AUTOTRIM=1 or reduce scout-report anchor scope."
else
    echo "No phases above 70% saturation threshold. Context usage is healthy."
fi
echo ""
