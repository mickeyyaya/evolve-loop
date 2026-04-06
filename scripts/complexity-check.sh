#!/usr/bin/env bash
# complexity-check.sh — Lightweight cognitive complexity checker
# Usage: complexity-check.sh <file> [--threshold N]
# Counts control flow keywords per function and reports complexity scores.
# Exit 0 if all functions below threshold, exit 1 if any exceed.

set -euo pipefail

THRESHOLD=15
FILE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --threshold) THRESHOLD="$2"; shift 2 ;;
    -h|--help) echo "Usage: complexity-check.sh <file> [--threshold N]"; exit 0 ;;
    *) FILE="$1"; shift ;;
  esac
done

if [[ -z "$FILE" ]]; then
  echo "Usage: complexity-check.sh <file> [--threshold N]" >&2
  exit 2
fi

if [[ ! -f "$FILE" ]]; then
  echo "ERROR: File not found: $FILE" >&2
  exit 2
fi

# Detect language from extension
EXT="${FILE##*.}"
EXCEEDED=0
FUNC_COUNT=0

# Count control flow keywords that add to cognitive complexity
# Keywords: if, else if, for, while, case/switch, catch, &&, ||, nested ternary
count_complexity() {
  local content="$1"
  local score=0

  # Control flow keywords (each adds 1)
  score=$((score + $(echo "$content" | grep -cE '^\s*(if|else if|elif|for|while|do|switch|case|catch|except)\b' 2>/dev/null || echo 0)))

  # Nesting increments (each nesting level adds 1 per nested control flow)
  local nesting_depth
  nesting_depth=$(echo "$content" | awk '
    /\{/ { depth++ }
    /\}/ { depth-- }
    /^\s*(if|for|while|switch|case)/ && depth > 1 { extra += depth - 1 }
    END { print extra+0 }
  ')
  score=$((score + nesting_depth))

  echo "$score"
}

# Extract functions and score each one
case "$EXT" in
  sh|bash)
    # Bash functions: name() { or function name {
    while IFS= read -r func_name; do
      [[ -z "$func_name" ]] && continue
      FUNC_COUNT=$((FUNC_COUNT + 1))
      # Extract function body (simplified — count keywords in entire file for now)
      escaped_name=$(printf '%s' "$func_name" | sed 's/[.[\*^$/]/\\&/g')
      body=$(sed -n "/^${escaped_name}()/,/^}/p" "$FILE" 2>/dev/null || echo "")
      if [[ -z "$body" ]]; then
        body=$(sed -n "/function ${escaped_name}/,/^}/p" "$FILE" 2>/dev/null || echo "")
      fi
      score=$(count_complexity "$body")
      status="OK"
      if [[ "$score" -gt "$THRESHOLD" ]]; then
        status="EXCEEDED"
        EXCEEDED=1
      fi
      printf "%s:%s:complexity=%d:threshold=%d:%s\n" "$FILE" "$func_name" "$score" "$THRESHOLD" "$status"
    done < <(grep -oE '^[a-zA-Z_][a-zA-Z0-9_]*\(\)' "$FILE" | sed 's/()//' 2>/dev/null; grep -oE '^function [a-zA-Z_][a-zA-Z0-9_]*' "$FILE" | sed 's/function //' 2>/dev/null)
    ;;
  py|js|ts|tsx|jsx|go|java|rs)
    # All code languages — extract functions by keyword
    while IFS= read -r func_name; do
      [[ -z "$func_name" ]] && continue
      FUNC_COUNT=$((FUNC_COUNT + 1))
      escaped_name=$(printf '%s' "$func_name" | sed 's/[.[\*^$/]/\\&/g')
      body=$(sed -n "/${escaped_name}/,/^[^ \t}\)]/p" "$FILE" 2>/dev/null | head -100 || echo "")
      score=$(count_complexity "$body")
      status="OK"
      if [[ "$score" -gt "$THRESHOLD" ]]; then status="EXCEEDED"; EXCEEDED=1; fi
      printf "%s:%s:complexity=%d:threshold=%d:%s\n" "$FILE" "$func_name" "$score" "$THRESHOLD" "$status"
    done < <(grep -oE '(function|func|fn|def)\s+[a-zA-Z_][a-zA-Z0-9_]*' "$FILE" | sed -E 's/(function|func|fn|def)\s+//')
    ;;
  *)
    # Generic/markdown: treat whole file as one unit
    score=$(count_complexity "$(cat "$FILE")")
    status="OK"
    if [[ "$score" -gt "$THRESHOLD" ]]; then status="EXCEEDED"; EXCEEDED=1; fi
    printf "%s:(file):complexity=%d:threshold=%d:%s\n" "$FILE" "$score" "$THRESHOLD" "$status"
    ;;
esac

# If no functions found, check file as a whole
if [[ "$FUNC_COUNT" -eq 0 && "$EXT" != "md" ]]; then
  score=$(count_complexity "$(cat "$FILE")")
  status="OK"
  if [[ "$score" -gt "$THRESHOLD" ]]; then
    status="EXCEEDED"
    EXCEEDED=1
  fi
  printf "%s:(file):complexity=%d:threshold=%d:%s\n" "$FILE" "$score" "$THRESHOLD" "$status"
fi

exit "$EXCEEDED"
