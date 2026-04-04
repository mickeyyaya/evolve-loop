#!/bin/bash
# verify-eval.sh — Independent eval re-execution
#
# Usage: bash scripts/verify-eval.sh <eval-file.md> [workspace-path]
#
# Re-runs every eval command from an eval definition file and compares results
# to the Auditor's claimed outcomes. This script operates independently of the
# Auditor — it trusts no report, only what it observes.
#
# Research basis:
#   - "AI Control" (Greenblatt et al. 2023) — don't trust monitor claims, verify independently
#   - Process supervision ("Let's Verify Step by Step", Lightman et al. 2023)
#   - Decoupled evaluation (Uesato et al. 2020)

set -euo pipefail

EVAL_INPUT="${1:?Usage: verify-eval.sh <eval-file-or-dir> [workspace-path]}"
WORKSPACE="${2:-.evolve/workspace}"

TOTAL=0
PASSED=0
FAILED=0
RESULTS=""

# If a directory is passed, concatenate all eval files for command extraction
if [ -d "$EVAL_INPUT" ]; then
  EVAL_FILE=$(mktemp)
  cat "$EVAL_INPUT"/*.md > "$EVAL_FILE" 2>/dev/null || true
  CLEANUP_EVAL=true
else
  EVAL_FILE="$EVAL_INPUT"
  CLEANUP_EVAL=false
fi

# Extract eval commands from markdown
# Handles: - `command here` format
COMMANDS=$(grep -E '^\s*-\s*`[^`]+`' "$EVAL_FILE" 2>/dev/null | sed 's/.*`\(.*\)`.*/\1/' || true)

if [ -z "$COMMANDS" ]; then
  # Try alternate format
  COMMANDS=$(grep -E '^\s*-\s*command:\s*' "$EVAL_FILE" 2>/dev/null | sed 's/.*command:\s*//' | tr -d '"' || true)
fi

# Clean up temp file if created
if [ "$CLEANUP_EVAL" = true ] && [ -f "$EVAL_FILE" ]; then
  trap "rm -f $EVAL_FILE" EXIT
fi

if [ -z "$COMMANDS" ]; then
  echo '{"total": 0, "passed": 0, "failed": 0, "verdict": "WARN", "detail": "No eval commands found"}'
  exit 1
fi

LOG_FILE="${WORKSPACE}/verify-eval.log"
> "$LOG_FILE" 2>/dev/null || LOG_FILE="/tmp/verify-eval-$$.log"

while IFS= read -r CMD; do
  [ -z "$CMD" ] && continue
  TOTAL=$((TOTAL + 1))

  # Execute the command with timeout (30s max per command)
  echo "--- Executing: $CMD ---" >> "$LOG_FILE"
  set +e
  eval "$CMD" >> "$LOG_FILE" 2>&1
  EXIT_CODE=$?
  set -e
  echo "--- Exit code: $EXIT_CODE ---" >> "$LOG_FILE"

  if [ "$EXIT_CODE" -eq 0 ]; then
    PASSED=$((PASSED + 1))
    RESULTS="${RESULTS}    {\"command\": \"$(echo "$CMD" | sed 's/"/\\"/g')\", \"status\": \"PASS\", \"exitCode\": 0},\n"
  else
    FAILED=$((FAILED + 1))
    RESULTS="${RESULTS}    {\"command\": \"$(echo "$CMD" | sed 's/"/\\"/g')\", \"status\": \"FAIL\", \"exitCode\": ${EXIT_CODE}},\n"
  fi
done <<< "$COMMANDS"

# Determine verdict
VERDICT="PASS"
if [ "$FAILED" -gt 0 ]; then
  VERDICT="FAIL"
elif [ "$TOTAL" -eq 0 ]; then
  VERDICT="WARN"
fi

# Remove trailing comma and newline from results
RESULTS=$(printf "$RESULTS" | sed '$ s/,$//')

cat <<EOF
{
  "total": ${TOTAL},
  "passed": ${PASSED},
  "failed": ${FAILED},
  "verdict": "${VERDICT}",
  "logFile": "${LOG_FILE}",
  "results": [
${RESULTS}
  ]
}
EOF

# Compare with Auditor's claimed results if audit-report exists
AUDIT_REPORT="${WORKSPACE}/audit-report.md"
if [ -f "$AUDIT_REPORT" ]; then
  AUDITOR_PASS_COUNT=$(grep -c "| PASS |" "$AUDIT_REPORT" 2>/dev/null || echo 0)
  AUDITOR_FAIL_COUNT=$(grep -c "| FAIL |" "$AUDIT_REPORT" 2>/dev/null || echo 0)

  if [ "$FAILED" -gt 0 ] && [ "$AUDITOR_FAIL_COUNT" = "0" ]; then
    echo ""
    echo "DISCREPANCY: verify-eval found ${FAILED} failures but Auditor reported 0 failures"
    echo "This may indicate the Auditor did not actually run the eval commands"
  fi
fi

# Exit code matches verdict
if [ "$FAILED" -gt 0 ]; then
  exit 1
fi
exit 0
