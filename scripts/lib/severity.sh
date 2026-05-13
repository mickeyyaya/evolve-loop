#!/usr/bin/env bash
#
# severity.sh — System-wide severity vocabulary for evolve-loop observability.
#
# Three-tier severity model used by phase-observer, future health checks,
# and any component that emits a verdict. Numeric values allow `severity_gte`
# threshold comparisons in shell without parsing strings.
#
# Usage (source from another script):
#   source "$EVOLVE_PLUGIN_ROOT/scripts/lib/severity.sh"
#   if severity_gte "$obs_severity" WARN; then echo "warn-or-worse"; fi
#
# Decision matrix for the orchestrator:
#   INFO     → log only, no action
#   WARN     → note in phase report, no intervention
#   INCIDENT → read suggested_action, act
#
# Canonical reference: docs/architecture/observer-severity.md
#
# DO NOT change these numeric values without bumping all consumers — they are
# load-bearing for sort order and comparison logic.

# Guard against multiple-sourcing.
if [ -n "${EVOLVE_SEVERITY_SH_LOADED:-}" ]; then
    return 0 2>/dev/null || exit 0
fi
EVOLVE_SEVERITY_SH_LOADED=1

# Numeric values: spaced by 10 so future intermediate tiers (e.g., NOTICE=15)
# can be added without breaking comparisons.
readonly SEVERITY_INFO=10
readonly SEVERITY_WARN=20
readonly SEVERITY_INCIDENT=30

# String → int. Returns 0 for unknown.
severity_name_to_int() {
    case "$1" in
        INFO|info)         echo "$SEVERITY_INFO" ;;
        WARN|warn)         echo "$SEVERITY_WARN" ;;
        INCIDENT|incident) echo "$SEVERITY_INCIDENT" ;;
        *)                 echo "0" ;;
    esac
}

# Int → string. Unknown values render as UNKNOWN.
severity_int_to_name() {
    case "$1" in
        "$SEVERITY_INFO")     echo "INFO" ;;
        "$SEVERITY_WARN")     echo "WARN" ;;
        "$SEVERITY_INCIDENT") echo "INCIDENT" ;;
        *)                    echo "UNKNOWN" ;;
    esac
}

# Returns 0 (truthy in shell) if $1 >= $2. Both args may be names or ints.
severity_gte() {
    local lhs rhs
    case "$1" in
        [0-9]*) lhs="$1" ;;
        *)      lhs=$(severity_name_to_int "$1") ;;
    esac
    case "$2" in
        [0-9]*) rhs="$2" ;;
        *)      rhs=$(severity_name_to_int "$2") ;;
    esac
    [ "$lhs" -ge "$rhs" ]
}
