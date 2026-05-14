#!/usr/bin/env bash
#
# estimate-quota-reset.sh — Compute wake-up time after a Claude Code quota hit.
#
# Auto-resume Layer 1 helper. Pure compute, no side effects beyond stderr
# logs. Designed so subagent-run.sh can capture stdout via $(...) and feed it
# straight into cycle-state.sh's checkpoint schema (Layer 2).
#
# Source priority (highest first):
#   1. EVOLVE_QUOTA_RESET_AT env var — operator-supplied ISO 8601 override
#   2. Hint file at $WORKSPACE/quota-reset-hint.txt — Anthropic's "resets HH:MMam"
#      message captured by the Layer-1 stderr filter in scripts/cli_adapters/claude.sh
#   3. Fallback: now + EVOLVE_QUOTA_RESET_HOURS (default 5h25min — Anthropic Pro/Max
#      window of 5h plus 25min jitter buffer)
#
# Usage:
#   bash estimate-quota-reset.sh [WORKSPACE_DIR]
#
# Output (stdout, exactly 2 lines):
#   line 1: ISO 8601 timestamp in local TZ (e.g. 2026-05-14T05:20:00+0800)
#   line 2: source=operator-override | source=parsed | source=default
#
# Exit codes:
#   0 — success
#   1 — fatal (cannot compute timestamp via either GNU or BSD date)
#
# bash 3.2 compatible. No declare -A, no mapfile, no GNU-only sed flags. Tries
# GNU date first then falls back to BSD date for portability across macOS/Linux.

set -uo pipefail

WORKSPACE="${1:-}"

log() { echo "[estimate-quota-reset] $*" >&2; }

# Portable epoch → ISO 8601 (local TZ).
epoch_to_iso() {
    local epoch="$1"
    if date -d "@$epoch" "+%Y-%m-%dT%H:%M:%S%z" 2>/dev/null; then
        return 0
    fi
    if date -r "$epoch" "+%Y-%m-%dT%H:%M:%S%z" 2>/dev/null; then
        return 0
    fi
    return 1
}

# Source 1 — explicit operator override.
if [ -n "${EVOLVE_QUOTA_RESET_AT:-}" ]; then
    log "source=operator-override value=$EVOLVE_QUOTA_RESET_AT"
    printf '%s\nsource=operator-override\n' "$EVOLVE_QUOTA_RESET_AT"
    exit 0
fi

# Source 2 — parsed hint file.
if [ -n "$WORKSPACE" ] && [ -d "$WORKSPACE" ]; then
    HINT_FILE="$WORKSPACE/quota-reset-hint.txt"
    if [ -s "$HINT_FILE" ]; then
        # Strip whitespace; cap at 32 chars to defend against pathological input.
        HINT=$(tr -d '\r\n[:space:]' < "$HINT_FILE" | head -c 32)
        # Accept "resets 5:20am" or just "5:20am". Pattern: HH:MM followed by am/pm.
        TIME_MATCH=$(echo "$HINT" | grep -oiE '[0-9]{1,2}:[0-9]{2}(am|pm)' | head -1)
        if [ -n "$TIME_MATCH" ]; then
            HH=$(echo "$TIME_MATCH" | grep -oE '^[0-9]{1,2}')
            MM=$(echo "$TIME_MATCH" | grep -oE ':[0-9]{2}' | tr -d ':')
            AMPM=$(echo "$TIME_MATCH" | grep -oiE '(am|pm)$' | tr '[:upper:]' '[:lower:]')
            # 12h → 24h
            if [ "$AMPM" = "pm" ] && [ "$HH" -lt 12 ]; then
                HH=$((HH + 12))
            elif [ "$AMPM" = "am" ] && [ "$HH" -eq 12 ]; then
                HH=0
            fi
            TODAY=$(date "+%Y-%m-%d")
            CAND_STR="${TODAY} $(printf '%02d:%02d:00' "$HH" "$MM")"
            CAND_EPOCH=""
            # Try BSD date first (macOS), then GNU.
            if CAND_EPOCH=$(date -j -f "%Y-%m-%d %H:%M:%S" "$CAND_STR" "+%s" 2>/dev/null); then
                :
            elif CAND_EPOCH=$(date -d "$CAND_STR" "+%s" 2>/dev/null); then
                :
            fi
            if [ -n "$CAND_EPOCH" ]; then
                NOW_EPOCH=$(date +%s)
                # If candidate time has already passed today, assume the operator
                # means tomorrow's matching time (typical for "resets 5:20am" when
                # current time is 11pm previous day).
                if [ "$CAND_EPOCH" -le "$NOW_EPOCH" ]; then
                    CAND_EPOCH=$((CAND_EPOCH + 86400))
                fi
                CAND_ISO=$(epoch_to_iso "$CAND_EPOCH")
                if [ -n "$CAND_ISO" ]; then
                    log "source=parsed hint='$TIME_MATCH' wake-at=$CAND_ISO"
                    printf '%s\nsource=parsed\n' "$CAND_ISO"
                    exit 0
                fi
            fi
            log "WARN: time '$TIME_MATCH' parsed but date conversion failed; falling through"
        else
            log "WARN: hint '$HINT' has no HH:MM(am|pm) match; falling through"
        fi
    fi
fi

# Source 3 — fallback now + N hours.
DEFAULT_HOURS="${EVOLVE_QUOTA_RESET_HOURS:-5.4167}"
SECONDS_OFFSET=$(awk -v h="$DEFAULT_HOURS" 'BEGIN { printf "%d", h * 3600 }')
NOW_EPOCH=$(date +%s)
WAKE_EPOCH=$((NOW_EPOCH + SECONDS_OFFSET))
WAKE_ISO=$(epoch_to_iso "$WAKE_EPOCH")
if [ -z "$WAKE_ISO" ]; then
    log "FATAL: cannot compute ISO timestamp from epoch $WAKE_EPOCH (neither GNU nor BSD date worked)"
    exit 1
fi
log "source=default-${DEFAULT_HOURS}h wake-at=$WAKE_ISO"
printf '%s\nsource=default\n' "$WAKE_ISO"
exit 0
