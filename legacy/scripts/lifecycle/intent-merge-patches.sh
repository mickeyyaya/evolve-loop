#!/usr/bin/env bash
#
# intent-merge-patches.sh — Apply intent delta patches or handle [intent-unchanged] marker.
#
# Reads an intent-delta.md file and either:
#   - Exits 0 immediately if the file contains [intent-unchanged]
#   - Applies field patches from the delta to the base intent.md
#   - Writes the merged result atomically via tmp + mv
#
# Usage:
#   bash scripts/lifecycle/intent-merge-patches.sh [<intent-file>] [<delta-file>]
#
# Defaults:
#   intent-file  $WORKSPACE/intent.md
#   delta-file   $WORKSPACE/intent-delta.md
#
# Exit codes:
#   0  — success (unchanged marker, or patches applied, or no delta file)
#   1  — error (intent file missing when delta patches needed)

set -uo pipefail

log()  { echo "[intent-merge-patches] $*" >&2; }
warn() { echo "[intent-merge-patches] WARN: $*" >&2; }

# ── Argument resolution ───────────────────────────────────────────────────────

INTENT_FILE="${1:-}"
DELTA_FILE="${2:-}"

if [ -z "$INTENT_FILE" ]; then
    INTENT_FILE="${WORKSPACE:-}/intent.md"
fi
if [ -z "$DELTA_FILE" ]; then
    DELTA_FILE="${WORKSPACE:-}/intent-delta.md"
fi

# ── No delta file ─────────────────────────────────────────────────────────────

if [ ! -f "$DELTA_FILE" ]; then
    log "no delta file at $DELTA_FILE — nothing to merge"
    exit 0
fi

# ── Check for [intent-unchanged] marker ───────────────────────────────────────

delta_content=$(cat "$DELTA_FILE" 2>/dev/null || echo "")

# Trim leading/trailing whitespace for comparison
trimmed=$(printf '%s\n' "$delta_content" | sed 's/^[[:space:]]*//' | sed 's/[[:space:]]*$//' | head -1)

if [ "$trimmed" = "[intent-unchanged]" ]; then
    log "[intent-unchanged] marker detected — no-op, intent.md unchanged"
    exit 0
fi

# ── Delta patch application ───────────────────────────────────────────────────

if [ ! -f "$INTENT_FILE" ]; then
    log "ERROR: intent file not found at $INTENT_FILE — cannot apply delta patches"
    exit 1
fi

log "applying delta patches from $DELTA_FILE to $INTENT_FILE"

# Strategy: the delta file contains a "## Changed fields" section describing
# field updates. For this v0.1 implementation, we append a delta annotation
# block to the intent.md rather than doing in-place YAML surgery (which would
# require a YAML parser). The annotation block is read by downstream agents.
#
# Future cycles (incremental-intent cycle 2+) can implement proper YAML-aware
# field-level merging once the format is validated in production.

TMP_FILE="${INTENT_FILE}.tmp.$$"

{
    cat "$INTENT_FILE"
    printf '\n<!-- DELTA APPLIED FROM: %s -->\n' "$(basename "$DELTA_FILE")"
    printf '<!-- CYCLE DELTA START -->\n'
    cat "$DELTA_FILE"
    printf '\n<!-- CYCLE DELTA END -->\n'
} > "$TMP_FILE"

if mv -f "$TMP_FILE" "$INTENT_FILE" 2>/dev/null; then
    log "delta applied atomically to $INTENT_FILE"
else
    rm -f "$TMP_FILE" 2>/dev/null || true
    warn "atomic mv failed — intent.md unchanged"
    exit 1
fi

exit 0
