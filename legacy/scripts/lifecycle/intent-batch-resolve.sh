#!/usr/bin/env bash
#
# intent-batch-resolve.sh — Compute INTENT_MODE, BATCH_ID, and GOAL_HASH for incremental intent.
#
# Reads goal text (from argument or workspace intent.md frontmatter), computes a
# SHA256 hash, compares against state.json:currentBatch.goalHash to decide whether
# this cycle needs a full intent run or a delta patch.
#
# Output (stdout): shell-eval-safe lines:
#   INTENT_MODE=full|delta
#   BATCH_ID=<batch-id>
#   GOAL_HASH=<sha256>
#
# Exit 0 always. On errors, outputs INTENT_MODE=full (safe default).
#
# Usage:
#   bash legacy/scripts/lifecycle/intent-batch-resolve.sh [<goal-text>]
#   bash legacy/scripts/lifecycle/intent-batch-resolve.sh --help

set -uo pipefail

__self_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__self_dir/resolve-roots.sh" 2>/dev/null || true

STATE_FILE="${EVOLVE_PROJECT_ROOT:-}/.evolve/state.json"

log() { echo "[intent-batch-resolve] $*" >&2; }

# ── Help ──────────────────────────────────────────────────────────────────────

if [ "${1:-}" = "--help" ] || [ "${1:-}" = "-h" ]; then
    cat >&2 <<'EOF'
intent-batch-resolve.sh — Compute INTENT_MODE, BATCH_ID, and GOAL_HASH

Usage:
  bash legacy/scripts/lifecycle/intent-batch-resolve.sh [<goal-text>]
  bash legacy/scripts/lifecycle/intent-batch-resolve.sh --help

Arguments:
  <goal-text>  Optional. If omitted, reads goal from stdin or workspace intent.md.

Output (stdout, shell-eval-safe):
  INTENT_MODE=full|delta
  BATCH_ID=<batch-id>
  GOAL_HASH=<sha256>

Environment:
  EVOLVE_INTENT_DELTA   When 0 or unset, always outputs INTENT_MODE=full.
  EVOLVE_PROJECT_ROOT   Path to the project root (for state.json).
  WORKSPACE             Path to current cycle workspace (for intent.md read).

Decision logic:
  1. EVOLVE_INTENT_DELTA=0 → INTENT_MODE=full (env-off)
  2. No currentBatch in state.json → INTENT_MODE=full (first batch cycle)
  3. goalHash mismatch → INTENT_MODE=full (goal changed)
  4. lastAuditVerdict=FAIL → INTENT_MODE=full (Karpathy Rule)
  5. else → INTENT_MODE=delta

Exit: 0 always. Errors fall through to INTENT_MODE=full.
EOF
    exit 0
fi

# ── Hash utilities ────────────────────────────────────────────────────────────

sha256_of() {
    local text="$1"
    if command -v sha256sum >/dev/null 2>&1; then
        printf '%s' "$text" | sha256sum | awk '{print $1}'
    elif command -v shasum >/dev/null 2>&1; then
        printf '%s' "$text" | shasum -a 256 | awk '{print $1}'
    else
        # Fallback: use cksum (always available, not SHA but better than nothing)
        log "WARN: sha256sum and shasum not found; using cksum fallback (weak hash)"
        printf '%s' "$text" | cksum | awk '{print "cksum-" $1}'
    fi
}

normalize_goal() {
    local raw="$1"
    # Collapse whitespace, lowercase. Bash 3.2: use tr for lowercase.
    printf '%s\n' "$raw" | tr '[:upper:]' '[:lower:]' | tr -s '[:space:]' ' ' | sed 's/^ //; s/ $//'
}

# ── Read goal text ────────────────────────────────────────────────────────────

GOAL_TEXT=""

if [ $# -gt 0 ] && [ "$1" != "--help" ]; then
    GOAL_TEXT="$*"
elif [ -n "${WORKSPACE:-}" ] && [ -f "$WORKSPACE/intent.md" ]; then
    # Extract goal from YAML frontmatter
    GOAL_TEXT=$(awk '/^---$/{n++; next} n==1 && /^goal:/{sub(/^goal: */, ""); print; exit}' "$WORKSPACE/intent.md" 2>/dev/null || echo "")
fi

if [ -z "$GOAL_TEXT" ]; then
    log "no goal text available; defaulting to full mode"
    ts=$(date -u +"%Y%m%dT%H%M%SZ" 2>/dev/null || echo "unknown")
    printf 'INTENT_MODE=full\nBATCH_ID=batch-%s\nGOAL_HASH=\n' "$ts"
    exit 0
fi

# ── Compute hash ──────────────────────────────────────────────────────────────

normalized=$(normalize_goal "$GOAL_TEXT")
GOAL_HASH=$(sha256_of "$normalized")
ts=$(date -u +"%Y%m%dT%H%M%SZ" 2>/dev/null || echo "ts-unknown")
BATCH_ID="batch-${ts}"

# ── Default: full mode when EVOLVE_INTENT_DELTA is off ───────────────────────

if [ "${EVOLVE_INTENT_DELTA:-0}" != "1" ]; then
    printf 'INTENT_MODE=full\nBATCH_ID=%s\nGOAL_HASH=%s\n' "$BATCH_ID" "$GOAL_HASH"
    exit 0
fi

# ── Read state.json:currentBatch ──────────────────────────────────────────────

CURRENT_HASH=""
LAST_AUDIT_VERDICT=""

if [ -f "$STATE_FILE" ] && command -v jq >/dev/null 2>&1; then
    CURRENT_HASH=$(jq -r '.currentBatch.goalHash // ""' "$STATE_FILE" 2>/dev/null || echo "")
    LAST_AUDIT_VERDICT=$(jq -r '.lastAuditVerdict // ""' "$STATE_FILE" 2>/dev/null || echo "")
    # Re-use existing batchId if goal hash matches
    existing_batch=$(jq -r '.currentBatch.batchId // ""' "$STATE_FILE" 2>/dev/null || echo "")
    if [ -n "$existing_batch" ] && [ "$CURRENT_HASH" = "$GOAL_HASH" ]; then
        BATCH_ID="$existing_batch"
    fi
fi

# ── Decision logic ────────────────────────────────────────────────────────────

INTENT_MODE="full"

if [ -z "$CURRENT_HASH" ]; then
    log "no currentBatch in state.json — INTENT_MODE=full (first batch cycle)"
elif [ "$CURRENT_HASH" != "$GOAL_HASH" ]; then
    log "goalHash mismatch (stored=$CURRENT_HASH computed=$GOAL_HASH) — INTENT_MODE=full"
elif [ "$LAST_AUDIT_VERDICT" = "FAIL" ]; then
    log "lastAuditVerdict=FAIL — INTENT_MODE=full (Karpathy Rule: re-examine premises)"
else
    INTENT_MODE="delta"
    log "goalHash match, no fail audit — INTENT_MODE=delta"
fi

printf 'INTENT_MODE=%s\nBATCH_ID=%s\nGOAL_HASH=%s\n' "$INTENT_MODE" "$BATCH_ID" "$GOAL_HASH"
exit 0
