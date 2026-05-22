#!/usr/bin/env bash
#
# record-failure-to-state.sh — Lightweight failure recorder for the
# orchestrator's FAIL/WARN/SHIP_GATE_DENIED branch.
#
# Per the v8.12.3 design pivot: instead of running the evolve-retrospective
# subagent immediately on every cycle failure (expensive, ~$0.50 per FAIL),
# the orchestrator just records the failure FACTS into state.json. Pattern
# extraction, lesson synthesis, and retrospective writing happen later via
# a separate, opt-in batch command (`/retrospect` or scheduled cron) that
# processes accumulated failures.
#
# This is the equivalent of "log first, analyze later" for failures.
#
# Usage:
#   bash scripts/failure/record-failure-to-state.sh <workspace_path> <verdict>
#
#   workspace_path — .evolve/runs/cycle-N/ containing audit-report.md
#   verdict        — FAIL | WARN | SHIP_GATE_DENIED
#
# Reads:   $workspace_path/audit-report.md (defects, severity, files)
# Writes:  .evolve/state.json (appends to failedApproaches[])
# Returns: 0 on success, 1 on missing/malformed input
#
# The retrospective subagent (agents/evolve-retrospective.md) remains
# available but is invoked separately on a batch of failures, not per cycle.

set -uo pipefail

# v8.18.0: dual-root — state.json is a writable artifact under the user's project.
__rr_self="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$__rr_self/../lifecycle/resolve-roots.sh"
unset __rr_self
if [ -n "${EVOLVE_STATE_OVERRIDE:-}" ] && [ -z "${EVOLVE_STATE_FILE_OVERRIDE:-}" ]; then
    echo "[deprecation] EVOLVE_STATE_OVERRIDE is renamed to EVOLVE_STATE_FILE_OVERRIDE" >&2
    EVOLVE_STATE_FILE_OVERRIDE="$EVOLVE_STATE_OVERRIDE"
fi
STATE="${EVOLVE_STATE_FILE_OVERRIDE:-$EVOLVE_PROJECT_ROOT/.evolve/state.json}"

log() { echo "[record-failure] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

[ $# -ge 2 ] || fail "usage: record-failure-to-state.sh <workspace_path> <verdict>"

WORKSPACE="$1"
VERDICT="$2"

[ -d "$WORKSPACE" ] || fail "workspace not found: $WORKSPACE"

case "$VERDICT" in
    FAIL|WARN|SHIP_GATE_DENIED) ;;
    # v8.16.1+: extended verdicts for orchestrator adaptive behavior. These can
    # be recorded WITHOUT a complete audit-report.md (because the audit phase
    # may have been skipped due to recurring infrastructure failure).
    WARN-NO-AUDIT|BLOCKED-RECURRING-AUDIT-FAIL|BLOCKED-RECURRING-BUILD-FAIL|BLOCKED-SYSTEMIC) ;;
    *) fail "verdict must be FAIL, WARN, SHIP_GATE_DENIED, WARN-NO-AUDIT, or BLOCKED-* — got: $VERDICT" ;;
esac

AUDIT_REPORT="$WORKSPACE/audit-report.md"
# WARN-NO-AUDIT and BLOCKED-* verdicts may be recorded without an audit report
# (the orchestrator skipped the audit phase due to a recurring infrastructure
# failure). For other verdicts the audit-report.md is required.
case "$VERDICT" in
    WARN-NO-AUDIT|BLOCKED-*) AUDIT_REPORT_REQUIRED=0 ;;
    *)                       AUDIT_REPORT_REQUIRED=1 ;;
esac
if [ "$AUDIT_REPORT_REQUIRED" = "1" ] && [ ! -f "$AUDIT_REPORT" ]; then
    fail "audit-report.md missing in workspace: $AUDIT_REPORT"
fi

command -v jq >/dev/null 2>&1 || fail "jq required"

# Extract cycle number from workspace path: .evolve/runs/cycle-N → N
CYCLE=$(basename "$WORKSPACE" | sed -n 's/^cycle-\([0-9]*\).*$/\1/p')
[ -n "$CYCLE" ] || CYCLE=0

# Extract structured defects from the audit report.
# Each defect line we recognize matches:
#   - HIGH/MEDIUM/LOW severity heading
#   - "Defects Found" section
#   - File:line references
if [ ! -f "$AUDIT_REPORT" ]; then
    DEFECTS_JSON="[]"
    VERDICT_IN_REPORT=""
else
DEFECTS_JSON=$(awk '
    BEGIN { in_defects = 0; n = 0 }
    /^## Defects Found/ { in_defects = 1; next }
    /^## / && in_defects { in_defects = 0 }
    in_defects && /^### \[(HIGH|MEDIUM|LOW|CRITICAL)\]|^### (HIGH|MEDIUM|LOW|CRITICAL)[ -]/ {
        line = $0
        sub(/^### /, "", line)
        # Extract severity from prefix patterns: "[HIGH] ..." or "HIGH — ..." or "HIGH - ..."
        severity = "unknown"
        title = line
        if (match(line, /^\[(HIGH|MEDIUM|LOW|CRITICAL)\][[:space:]]*/) ) {
            severity = substr(line, 2, RLENGTH - 3)
            title = substr(line, RLENGTH + 1)
        } else if (match(line, /^(HIGH|MEDIUM|LOW|CRITICAL)[[:space:]]*[—-][[:space:]]*/) ) {
            severity = substr(line, 1, index(line, " ") - 1)
            title = substr(line, RLENGTH + 1)
        }
        gsub(/"/, "\\\"", title)
        gsub(/\\/, "\\\\", title)
        # Strip trailing whitespace
        sub(/[[:space:]]+$/, "", title)
        if (n > 0) printf ","
        printf "{\"severity\":\"%s\",\"title\":\"%s\"}", severity, title
        n++
    }
' "$AUDIT_REPORT")
DEFECTS_JSON="[$DEFECTS_JSON]"

# Extract verdict line for sanity check
VERDICT_IN_REPORT=$(grep -oE '^Verdict[[:space:]]*:[[:space:]]*\*\*?(PASS|WARN|FAIL|SHIP_GATE_DENIED)' "$AUDIT_REPORT" | head -1 | sed 's/.*\*\*\?//;s/\*\*//' | tr -d ':[:space:]')
fi  # close "if [ ! -f $AUDIT_REPORT ]; then ... else"

if [ -n "$VERDICT_IN_REPORT" ] && [ "$VERDICT_IN_REPORT" != "$VERDICT" ]; then
    log "WARN: passed verdict '$VERDICT' differs from audit-report.md verdict '$VERDICT_IN_REPORT'"
fi

# Compute audit-report.md SHA256 (for forensic integrity check later).
# Skipped when audit-report.md doesn't exist (WARN-NO-AUDIT / BLOCKED-* verdicts).
if [ -f "$AUDIT_REPORT" ]; then
    if command -v sha256sum >/dev/null 2>&1; then
        REPORT_SHA=$(sha256sum "$AUDIT_REPORT" | awk '{print $1}')
    else
        REPORT_SHA=$(shasum -a 256 "$AUDIT_REPORT" | awk '{print $1}')
    fi
else
    REPORT_SHA=""
fi

# Capture git head + tree state at the moment of recording.
# Use git rev-parse HEAD^{tree} (the committed tree object SHA) instead of
# hashing `git diff HEAD`. After builder commits, git diff HEAD is empty, making
# the diff-hash always equal to SHA-256("") = e3b0c44... — forensically useless.
# The tree-object SHA is content-addressable and stable across re-recording at
# the same commit; falls back to "unknown" if not in a git repo or no commits.
GIT_HEAD=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
TREE_SHA=$(git rev-parse 'HEAD^{tree}' 2>/dev/null) || TREE_SHA="unknown"
[ -z "$TREE_SHA" ] && TREE_SHA="unknown"

# Init state.json if missing.
if [ ! -f "$STATE" ]; then
    log "creating new state.json"
    echo '{"failedApproaches": []}' > "$STATE"
fi

# Ensure failedApproaches array exists.
if ! jq -e '.failedApproaches' "$STATE" >/dev/null 2>&1; then
    TMP=$(mktemp)
    jq '. + {failedApproaches: []}' "$STATE" > "$TMP" && mv "$TMP" "$STATE"
fi

NOW_TS=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# v8.22.0: source classification helpers and derive structured classification
# + expiresAt from the verdict. Backward-compat: existing readers that look at
# `verdict` still see it; new readers (failure-adapter.sh) use `classification`.
. "$EVOLVE_PLUGIN_ROOT/scripts/failure/failure-classifications.sh"
CLASSIFICATION=$(failure_normalize_legacy "$VERDICT")
EXPIRES_AT=$(failure_compute_expires_at "$CLASSIFICATION" "$NOW_TS")

# Append the entry. v8.22 schema adds: classification, expiresAt. Existing
# fields preserved for backward compat. FIFO cap (50) applied at append time.
TMP=$(mktemp)
jq --arg ts "$NOW_TS" \
   --argjson cycle "$CYCLE" \
   --arg verdict "$VERDICT" \
   --arg classification "$CLASSIFICATION" \
   --arg expires_at "$EXPIRES_AT" \
   --arg path "$AUDIT_REPORT" \
   --arg sha "$REPORT_SHA" \
   --arg gh "$GIT_HEAD" \
   --arg ts2 "$TREE_SHA" \
   --argjson defects "$DEFECTS_JSON" \
   '.failedApproaches = (((.failedApproaches // []) + [{
       ts: $ts,
       cycle: $cycle,
       verdict: $verdict,
       classification: $classification,
       recordedAt: $ts,
       expiresAt: $expires_at,
       auditReportPath: $path,
       auditReportSha256: $sha,
       gitHead: $gh,
       treeStateSha: $ts2,
       defects: $defects,
       retrospected: false
   }]) | (if length > 50 then .[length-50:] else . end))' "$STATE" > "$TMP"
mv "$TMP" "$STATE"

DEFECT_COUNT=$(echo "$DEFECTS_JSON" | jq 'length')
log "OK: recorded $VERDICT (classification=$CLASSIFICATION, expires=$EXPIRES_AT) for cycle $CYCLE ($DEFECT_COUNT defects)"
log "to retrospect later: bash scripts/dispatch/subagent-run.sh retrospective <cycle> <workspace> on this entry"

exit 0
