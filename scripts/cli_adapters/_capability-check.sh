#!/usr/bin/env bash
#
# _capability-check.sh — Resolve a CLI adapter's capability manifest at runtime.
#
# Reads scripts/cli_adapters/<adapter>.capabilities.json + runs declared probes,
# emits a resolved capability map per dimension. Output is JSON — machine-
# readable for subagent-run.sh consumption and for bin/check-caps display.
#
# v8.51.0+
#
# Usage:
#   bash scripts/cli_adapters/_capability-check.sh <adapter>             # JSON to stdout
#   bash scripts/cli_adapters/_capability-check.sh <adapter> --human     # human-readable table
#   bash scripts/cli_adapters/_capability-check.sh <adapter> --probe-only # just emit probe results
#   bash scripts/cli_adapters/_capability-check.sh --list-adapters       # discover available manifests
#
# Output JSON shape (default):
#   {
#     "adapter": "<name>",
#     "version": 1,
#     "resolved": {
#       "subprocess_isolation": {"mode": "full|hybrid|degraded|none", "warning": "..."},
#       "budget_cap": {...},
#       ...
#     },
#     "quality_tier": "full|hybrid|degraded|none",
#     "warnings": ["...", ...],
#     "probes": {"claude_on_path": true|false, ...}
#   }
#
# Exit codes:
#   0  — manifest read + probes ran cleanly
#   1  — manifest missing, malformed, or unrecognized adapter
#  10  — bad arguments

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

ADAPTER=""
HUMAN=0
PROBE_ONLY=0
LIST=0

while [ $# -gt 0 ]; do
    case "$1" in
        --human)         HUMAN=1 ;;
        --probe-only)    PROBE_ONLY=1 ;;
        --list-adapters) LIST=1 ;;
        --help|-h)
            sed -n '2,28p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        --*) echo "[capability-check] unknown flag: $1" >&2; exit 10 ;;
        *)
            if [ -z "$ADAPTER" ]; then ADAPTER="$1"
            else echo "[capability-check] extra positional arg: $1" >&2; exit 10
            fi ;;
    esac
    shift
done

# --- list-adapters mode -----------------------------------------------------
if [ "$LIST" = "1" ]; then
    for m in "$SCRIPT_DIR"/*.capabilities.json; do
        [ -f "$m" ] || continue
        jq -r '.adapter' "$m" 2>/dev/null
    done
    exit 0
fi

[ -n "$ADAPTER" ] || { echo "[capability-check] usage: <adapter> [--human|--probe-only]" >&2; exit 10; }

MANIFEST="$SCRIPT_DIR/${ADAPTER}.capabilities.json"
if [ ! -f "$MANIFEST" ]; then
    echo "[capability-check] manifest missing: $MANIFEST" >&2
    exit 1
fi
if ! jq empty "$MANIFEST" 2>/dev/null; then
    echo "[capability-check] manifest malformed: $MANIFEST" >&2
    exit 1
fi

# --- Probe registry ---------------------------------------------------------
# Each probe is a function returning 0=true, non-0=false. Add new probes here.

probe_claude_on_path() {
    # v8.51.0: honor adapter-specific test seams. Either EVOLVE_GEMINI_CLAUDE_PATH
    # (legacy seam, predates capability framework) or EVOLVE_CODEX_CLAUDE_PATH
    # forces claude probe to a specific value when EVOLVE_TESTING=1. If both are
    # set, the more-specific (codex) wins for codex adapter. We can't tell which
    # adapter is being checked from inside the probe, so honor either-set as
    # "override active". Empty = forced missing; non-empty = use that path.
    if [ "${EVOLVE_TESTING:-0}" = "1" ]; then
        if [ "${EVOLVE_GEMINI_CLAUDE_PATH+set}" = "set" ]; then
            if [ -z "${EVOLVE_GEMINI_CLAUDE_PATH:-}" ]; then return 1; fi
            [ -x "${EVOLVE_GEMINI_CLAUDE_PATH:-}" ] && return 0 || return 1
        fi
        if [ "${EVOLVE_CODEX_CLAUDE_PATH+set}" = "set" ]; then
            if [ -z "${EVOLVE_CODEX_CLAUDE_PATH:-}" ]; then return 1; fi
            [ -x "${EVOLVE_CODEX_CLAUDE_PATH:-}" ] && return 0 || return 1
        fi
    fi
    command -v claude >/dev/null 2>&1
}

probe_sandbox_exec_available() {
    [ "$(uname -s)" = "Darwin" ] && command -v sandbox-exec >/dev/null 2>&1
}

probe_bwrap_available() {
    [ "$(uname -s)" = "Linux" ] && command -v bwrap >/dev/null 2>&1
}

run_probe() {
    local check="$1"
    case "$check" in
        claude_on_path)            probe_claude_on_path ;;
        sandbox_exec_available)    probe_sandbox_exec_available ;;
        bwrap_available)           probe_bwrap_available ;;
        *) return 2 ;;  # unknown probe
    esac
}

# --- Run all probes declared in the manifest --------------------------------
# Build a JSON object mapping probe name -> bool
PROBES_JSON="{}"
PROBE_NAMES=$(jq -r '.probes // [] | .[].check' "$MANIFEST" | sort -u)
for p in $PROBE_NAMES; do
    if run_probe "$p"; then
        PROBES_JSON=$(echo "$PROBES_JSON" | jq --arg k "$p" '. + {($k): true}')
    else
        PROBES_JSON=$(echo "$PROBES_JSON" | jq --arg k "$p" '. + {($k): false}')
    fi
done

if [ "$PROBE_ONLY" = "1" ]; then
    echo "$PROBES_JSON" | jq -c .
    exit 0
fi

# --- Resolve capability modes ----------------------------------------------
# For each capability:
#   - if value is a string → that's the resolved mode (no probe needed)
#   - if value is an object with modes/default → check applicable probes,
#     pick the first probe's if_true_mode if probe true, else if_false_mode,
#     else fall back to .default
RESOLVED="{}"
WARNINGS_JSON="[]"

CAPS=$(jq -r '.capabilities | keys[]' "$MANIFEST")
for cap in $CAPS; do
    cap_def=$(jq --arg c "$cap" '.capabilities[$c]' "$MANIFEST")
    cap_type=$(echo "$cap_def" | jq -r 'type')

    if [ "$cap_type" = "string" ]; then
        # Fixed mode
        mode=$(echo "$cap_def" | jq -r '.')
        warn=""
    else
        # Object — resolve via probes
        default=$(echo "$cap_def" | jq -r '.default')
        warn=$(echo "$cap_def" | jq -r '.warning // ""')
        mode="$default"

        # Find probes that apply to this capability
        applicable_probes=$(jq -r --arg c "$cap" '.probes // [] | .[] | select(.applies_to // [] | length == 0 or contains([$c])) | .check + "|" + .if_true_mode + "|" + (.if_false_mode // "")' "$MANIFEST")

        for probe_def in $applicable_probes; do
            check=$(echo "$probe_def" | awk -F'|' '{print $1}')
            if_true=$(echo "$probe_def" | awk -F'|' '{print $2}')
            if_false=$(echo "$probe_def" | awk -F'|' '{print $3}')
            probe_result=$(echo "$PROBES_JSON" | jq -r --arg k "$check" '.[$k] // "unknown"')
            if [ "$probe_result" = "true" ]; then
                mode="$if_true"
                break
            elif [ "$probe_result" = "false" ] && [ -n "$if_false" ]; then
                mode="$if_false"
                # keep iterating in case a later probe gives if_true match
            fi
        done
    fi

    # Build resolved entry for this capability
    resolved_entry=$(jq -n --arg m "$mode" --arg w "$warn" '{mode: $m, warning: $w}')
    RESOLVED=$(echo "$RESOLVED" | jq --arg c "$cap" --argjson v "$resolved_entry" '. + {($c): $v}')

    # If degraded or none, surface the warning
    if [ "$mode" = "degraded" ] || [ "$mode" = "none" ]; then
        if [ -n "$warn" ]; then
            WARNINGS_JSON=$(echo "$WARNINGS_JSON" | jq --arg w "[$cap=$mode] $warn" '. + [$w]')
        fi
    fi
done

# Aggregate quality_tier: lowest tier across all capabilities (none < degraded < hybrid < full)
TIER_RANK_NONE=0
TIER_RANK_DEGRADED=1
TIER_RANK_HYBRID=2
TIER_RANK_FULL=3
mode_rank() {
    case "$1" in
        none)     echo 0 ;;
        degraded) echo 1 ;;
        hybrid)   echo 2 ;;
        full)     echo 3 ;;
        *)        echo 0 ;;
    esac
}
rank_to_mode() {
    case "$1" in
        0) echo none ;;
        1) echo degraded ;;
        2) echo hybrid ;;
        3) echo full ;;
    esac
}

LOW_RANK=3
for cap in $CAPS; do
    m=$(echo "$RESOLVED" | jq -r --arg c "$cap" '.[$c].mode')
    r=$(mode_rank "$m")
    if [ "$r" -lt "$LOW_RANK" ]; then
        LOW_RANK="$r"
    fi
done
QUALITY_TIER=$(rank_to_mode "$LOW_RANK")

# --- Emit -------------------------------------------------------------------
ADAPTER_NAME=$(jq -r '.adapter' "$MANIFEST")
VERSION=$(jq -r '.version' "$MANIFEST")
NOTES=$(jq -r '.notes // ""' "$MANIFEST")

OUTPUT=$(jq -n \
    --arg adapter "$ADAPTER_NAME" \
    --argjson version "$VERSION" \
    --argjson resolved "$RESOLVED" \
    --arg tier "$QUALITY_TIER" \
    --argjson warnings "$WARNINGS_JSON" \
    --argjson probes "$PROBES_JSON" \
    --arg notes "$NOTES" \
    '{adapter: $adapter, version: $version, resolved: $resolved, quality_tier: $tier, warnings: $warnings, probes: $probes, notes: $notes}')

if [ "$HUMAN" = "1" ]; then
    echo "Adapter: $(echo "$OUTPUT" | jq -r '.adapter')"
    echo "Quality tier: $(echo "$OUTPUT" | jq -r '.quality_tier')"
    echo
    printf '%-25s %-12s %s\n' "Capability" "Mode" "Warning"
    printf '%-25s %-12s %s\n' "------------------------" "----------" "------------------------"
    for cap in $CAPS; do
        mode=$(echo "$OUTPUT" | jq -r --arg c "$cap" '.resolved[$c].mode')
        warning=$(echo "$OUTPUT" | jq -r --arg c "$cap" '.resolved[$c].warning' | head -c 60)
        if [ "$mode" = "degraded" ] || [ "$mode" = "none" ]; then
            mode_disp="⚠ $mode"
        else
            mode_disp="✓ $mode"
        fi
        printf '%-25s %-12s %s\n' "$cap" "$mode_disp" "$warning"
    done
    echo
    if [ -n "$NOTES" ]; then
        echo "Notes: $NOTES"
    fi
else
    echo "$OUTPUT" | jq -c .
fi
exit 0
