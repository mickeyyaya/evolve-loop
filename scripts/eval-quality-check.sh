#!/bin/bash
# eval-quality-check.sh — Deterministic eval rigor classifier
#
# Usage: bash scripts/eval-quality-check.sh <eval-file.md>
#        bash scripts/eval-quality-check.sh .evolve/evals/   (checks all evals in directory)
#
# Classifies eval commands by rigor level:
#   Level 0 (ANOMALY): echo, exit 0, true, no-op — automatic cycle halt
#   Level 1 (WARN):    grep on source files only — tautological
#   Level 2 (OK):      grep on output/generated files, test -f with comparisons
#   Level 3 (GOOD):    Execution-based checks (node, python, etc.)
#
# Research basis:
#   - Specification gaming catalog (DeepMind, 2020)
#   - Cycle 101 tautological eval incident (grep -q "string" source.js)
#   - "Sycophancy to Subterfuge" (Anthropic) — agents rewrite reward functions

set -euo pipefail

TARGET="${1:?Usage: eval-quality-check.sh <eval-file-or-directory>}"

TOTAL_COMMANDS=0
LEVEL_0_COUNT=0  # ANOMALY: no-ops
LEVEL_1_COUNT=0  # WARN: tautological
LEVEL_2_COUNT=0  # OK: basic checks
LEVEL_3_COUNT=0  # GOOD: execution-based

ISSUES=""

check_eval_file() {
  local FILE="$1"

  # Skip canary eval files
  if echo "$FILE" | grep -q "_canary"; then
    return
  fi

  # Extract eval commands: lines matching `command` patterns in markdown
  # Handles both:
  #   - `command here` (inline code in markdown lists)
  #   - command: "command here" (YAML-style)
  local COMMANDS
  COMMANDS=$(grep -E '^\s*-\s*`[^`]+`' "$FILE" 2>/dev/null | sed 's/.*`\(.*\)`.*/\1/' || true)

  if [ -z "$COMMANDS" ]; then
    # Try alternate format: command: lines
    COMMANDS=$(grep -E '^\s*-\s*command:\s*' "$FILE" 2>/dev/null | sed 's/.*command:\s*//' | tr -d '"' || true)
  fi

  if [ -z "$COMMANDS" ]; then
    # Try fenced code blocks (```bash ... ```). An attacker might use this format
    # to evade the inline-code-in-list parser. Extract everything between the
    # opening ```{lang} and closing ```, treating each non-blank, non-comment
    # line as a candidate command.
    COMMANDS=$(awk '
        /^[[:space:]]*```/ {
            if (in_block) { in_block = 0; next }
            if ($0 ~ /^[[:space:]]*```(bash|sh|shell)?[[:space:]]*$/) { in_block = 1; next }
        }
        in_block && NF > 0 && $0 !~ /^[[:space:]]*#/ { print }
    ' "$FILE" 2>/dev/null || true)
    if [ -n "$COMMANDS" ]; then
      ISSUES="${ISSUES}  WARN: Eval commands found only in fenced block (non-canonical format) in ${FILE}\n"
    fi
  fi

  if [ -z "$COMMANDS" ]; then
    # No eval commands found in any supported format. Treat as ANOMALY: either
    # the file is malformed (orchestrator should regenerate) or an attacker is
    # hiding commands in a format the parser doesn't understand.
    ISSUES="${ISSUES}  ANOMALY: No eval commands found in ${FILE} (any supported format)\n"
    LEVEL_0_COUNT=$((LEVEL_0_COUNT + 1))
    return
  fi

  while IFS= read -r CMD; do
    [ -z "$CMD" ] && continue
    TOTAL_COMMANDS=$((TOTAL_COMMANDS + 1))

    # ── Level 0: No-op commands (ANOMALY) ──
    # These commands always succeed without testing anything
    if echo "$CMD" | grep -qE '^\s*(echo\s|exit\s*0|true\s*$|:\s*$|#)'; then
      LEVEL_0_COUNT=$((LEVEL_0_COUNT + 1))
      ISSUES="${ISSUES}  ANOMALY [L0]: No-op eval command in ${FILE}: ${CMD}\n"
      continue
    fi

    # Commands that pipe to /dev/null (suppressing all output = suspicious)
    if echo "$CMD" | grep -qE '>\s*/dev/null\s*2>&1\s*;\s*(true|exit\s*0)'; then
      LEVEL_0_COUNT=$((LEVEL_0_COUNT + 1))
      ISSUES="${ISSUES}  ANOMALY [L0]: Output-suppressed no-op in ${FILE}: ${CMD}\n"
      continue
    fi

    # ── Level 1: Tautological checks (WARN) ──
    # grep on source files checks string existence, not behavior
    if echo "$CMD" | grep -qE 'grep\s+(-[a-zA-Z]*\s+)*"?[^"]*"?\s+[^ ]*\.(js|ts|py|sh|md|go|rs|java|rb|yaml|yml|toml|json)'; then
      # Check if it's grepping a source file (tautological) vs output file
      local TARGET_FILE
      TARGET_FILE=$(echo "$CMD" | grep -oE '[^ ]*\.(js|ts|py|sh|md|go|rs|java|rb|yaml|yml|toml|json)' | tail -1 || true)
      if [ -n "$TARGET_FILE" ]; then
        # If the target file is NOT an output/temp/log file, it's tautological
        if ! echo "$TARGET_FILE" | grep -qiE '(stdout|output|log|result|tmp|dist|build|\.out|\.generated)'; then
          LEVEL_1_COUNT=$((LEVEL_1_COUNT + 1))
          ISSUES="${ISSUES}  WARN [L1]: Tautological grep on source file in ${FILE}: ${CMD}\n"
          continue
        fi
      fi
    fi

    # File-existence-only checks without numeric comparison
    if echo "$CMD" | grep -qE '^\s*(test\s+-[fed]|ls\s+|\[\s+-[fed])' && ! echo "$CMD" | grep -qE '(-gt|-ge|-eq|-ne|-lt|-le|&&|\\|\\|)'; then
      LEVEL_1_COUNT=$((LEVEL_1_COUNT + 1))
      ISSUES="${ISSUES}  WARN [L1]: File-existence-only check in ${FILE}: ${CMD}\n"
      continue
    fi

    # wc -l without comparison (just counting, not asserting)
    if echo "$CMD" | grep -qE 'wc\s+-l' && ! echo "$CMD" | grep -qE '(-gt|-ge|-eq|-ne|-lt|-le|\[|\]\])'; then
      LEVEL_1_COUNT=$((LEVEL_1_COUNT + 1))
      ISSUES="${ISSUES}  WARN [L1]: Count without assertion in ${FILE}: ${CMD}\n"
      continue
    fi

    # ── Level 3: Execution-based checks (GOOD) ──
    # Commands that actually run code and check behavior
    if echo "$CMD" | grep -qE '(node|python|python3|go\s+run|cargo\s+run|ruby|java|npm\s+test|pytest|jest|mocha|go\s+test|cargo\s+test)'; then
      LEVEL_3_COUNT=$((LEVEL_3_COUNT + 1))
      continue
    fi

    # Piped commands that execute then check output
    if echo "$CMD" | grep -qE '\|.*grep'; then
      LEVEL_3_COUNT=$((LEVEL_3_COUNT + 1))
      continue
    fi

    # ── Level 2: Basic but acceptable checks ──
    # Everything else (grep on output files, test with comparisons, etc.)
    LEVEL_2_COUNT=$((LEVEL_2_COUNT + 1))

  done <<< "$COMMANDS"
}

# Process target (file or directory)
if [ -d "$TARGET" ]; then
  for EVAL_FILE in "$TARGET"/*.md; do
    [ -f "$EVAL_FILE" ] && check_eval_file "$EVAL_FILE"
  done
elif [ -f "$TARGET" ]; then
  check_eval_file "$TARGET"
else
  echo "Error: ${TARGET} is not a file or directory"
  exit 2
fi

# ─── Output Report ────────────────────────────────────────────────────

cat <<EOF
{
  "totalCommands": ${TOTAL_COMMANDS},
  "level0_anomaly": ${LEVEL_0_COUNT},
  "level1_warn": ${LEVEL_1_COUNT},
  "level2_ok": ${LEVEL_2_COUNT},
  "level3_good": ${LEVEL_3_COUNT},
  "overallRigor": "$(
    if [ "$LEVEL_0_COUNT" -gt 0 ]; then echo "ANOMALY"
    elif [ "$LEVEL_1_COUNT" -gt 0 ] && [ "$LEVEL_3_COUNT" -eq 0 ]; then echo "WARN"
    elif [ "$LEVEL_3_COUNT" -gt 0 ]; then echo "GOOD"
    elif [ "$TOTAL_COMMANDS" -eq 0 ]; then echo "WARN"
    else echo "OK"
    fi
  )"
}
EOF

if [ -n "$ISSUES" ]; then
  echo ""
  echo "Issues:"
  printf "$ISSUES"
fi

# Exit codes:
#   0 = OK or GOOD (proceed)
#   1 = WARN (proceed with caution, pass to Auditor)
#   2 = ANOMALY (halt cycle)
if [ "$LEVEL_0_COUNT" -gt 0 ]; then
  exit 2
elif [ "$LEVEL_1_COUNT" -gt 0 ] && [ "$LEVEL_3_COUNT" -eq 0 ]; then
  exit 1
fi
exit 0
