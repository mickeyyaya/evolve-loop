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
#   bash scripts/record-failure-to-state.sh <workspace_path> <verdict>
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

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
STATE="$REPO_ROOT/.evolve/state.json"

log() { echo "[record-failure] $*" >&2; }
fail() { log "FAIL: $*"; exit 1; }

[ $# -ge 2 ] || fail "usage: record-failure-to-state.sh <workspace_path> <verdict>"

WORKSPACE="$1"
VERDICT="$2"

[ -d "$WORKSPACE" ] || fail "workspace not found: $WORKSPACE"

case "$VERDICT" in
    FAIL|WARN|SHIP_GATE_DENIED) ;;
    *) fail "verdict must be FAIL, WARN, or SHIP_GATE_DENIED — got: $VERDICT" ;;
esac

AUDIT_REPORT="$WORKSPACE/audit-report.md"
[ -f "$AUDIT_REPORT" ] || fail "audit-report.md missing in workspace: $AUDIT_REPORT"

command -v jq >/dev/null 2>&1 || fail "jq required"

# Extract cycle number from workspace path: .evolve/runs/cycle-N → N
CYCLE=$(basename "$WORKSPACE" | sed -n 's/^cycle-\([0-9]*\).*$/\1/p')
[ -n "$CYCLE" ] || CYCLE=0

# Extract structured defects from the audit report.
# Each defect line we recognize matches:
#   - HIGH/MEDIUM/LOW severity heading
#   - "Defects Found" section
#   - File:line references
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

if [ -n "$VERDICT_IN_REPORT" ] && [ "$VERDICT_IN_REPORT" != "$VERDICT" ]; then
    log "WARN: passed verdict '$VERDICT' differs from audit-report.md verdict '$VERDICT_IN_REPORT'"
fi

# Compute audit-report.md SHA256 (for forensic integrity check later).
if command -v sha256sum >/dev/null 2>&1; then
    REPORT_SHA=$(sha256sum "$AUDIT_REPORT" | awk '{print $1}')
else
    REPORT_SHA=$(shasum -a 256 "$AUDIT_REPORT" | awk '{print $1}')
fi

# Capture git head + tree state at the moment of recording.
GIT_HEAD=$(git rev-parse HEAD 2>/dev/null || echo "unknown")
if command -v sha256sum >/dev/null 2>&1; then
    TREE_SHA=$(git diff HEAD 2>/dev/null | sha256sum | awk '{print $1}')
else
    TREE_SHA=$(git diff HEAD 2>/dev/null | shasum -a 256 | awk '{print $1}')
fi

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

# Append the entry. Schema (intentionally minimal — full retrospective happens
# on batch, this is just the raw fact-record):
#   { ts, cycle, verdict, auditReportPath, auditReportSha256, gitHead, treeStateSha,
#     defects: [{severity, title}], retrospected: false }
TMP=$(mktemp)
jq --arg ts "$NOW_TS" \
   --argjson cycle "$CYCLE" \
   --arg verdict "$VERDICT" \
   --arg path "$AUDIT_REPORT" \
   --arg sha "$REPORT_SHA" \
   --arg gh "$GIT_HEAD" \
   --arg ts2 "$TREE_SHA" \
   --argjson defects "$DEFECTS_JSON" \
   '.failedApproaches += [{
       ts: $ts,
       cycle: $cycle,
       verdict: $verdict,
       auditReportPath: $path,
       auditReportSha256: $sha,
       gitHead: $gh,
       treeStateSha: $ts2,
       defects: $defects,
       retrospected: false
   }]' "$STATE" > "$TMP"
mv "$TMP" "$STATE"

DEFECT_COUNT=$(echo "$DEFECTS_JSON" | jq 'length')
log "OK: recorded $VERDICT for cycle $CYCLE ($DEFECT_COUNT defects); state.json.failedApproaches[-1] now contains the entry"
log "to retrospect later: bash scripts/subagent-run.sh retrospective <cycle> <workspace> on this entry"

exit 0
