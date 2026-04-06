#!/usr/bin/env bash
# code-review-simplify.sh — Pipeline layer for the code-review-simplify skill
# Usage: code-review-simplify.sh [GIT_REF] [--json]
# Runs deterministic pattern checks on changed files and reports findings.
# GIT_REF defaults to HEAD~1 (last commit's changes).

set -euo pipefail

REF="${1:-HEAD~1}"
JSON_OUTPUT=false
[[ "${2:-}" == "--json" ]] && JSON_OUTPUT=true

FINDINGS=()
ISSUE_COUNT=0
WARN_COUNT=0

add_finding() {
  local severity="$1" dimension="$2" file="$3" line="$4" message="$5"
  FINDINGS+=("${severity}:${dimension}:${file}:${line}:${message}")
  if [[ "$severity" == "HIGH" || "$severity" == "CRITICAL" ]]; then
    ISSUE_COUNT=$((ISSUE_COUNT + 1))
  else
    WARN_COUNT=$((WARN_COUNT + 1))
  fi
}

# Get changed files
CHANGED_FILES=$(git diff "$REF" --name-only 2>/dev/null || echo "")
if [[ -z "$CHANGED_FILES" ]]; then
  echo "No changed files detected (ref: $REF)"
  exit 0
fi

DIFF_LINES=$(git diff "$REF" --numstat 2>/dev/null | awk '{s+=$1+$2} END {print s+0}')
FILE_COUNT=$(echo "$CHANGED_FILES" | wc -l | tr -d ' ')

# Determine tier
TIER="lightweight"
if [[ "$DIFF_LINES" -gt 200 || "$FILE_COUNT" -gt 10 ]]; then
  TIER="full"
elif [[ "$DIFF_LINES" -gt 50 || "$FILE_COUNT" -gt 3 ]]; then
  TIER="standard"
fi

# === PIPELINE CHECKS ===

# Check 1: File length (> 800 lines = maintainability warning)
while IFS= read -r file; do
  [[ -f "$file" ]] || continue
  lines=$(wc -l < "$file" | tr -d ' ')
  if [[ "$lines" -gt 800 ]]; then
    add_finding "MEDIUM" "maintainability" "$file" "0" "File length ${lines} exceeds 800-line limit"
  fi
done <<< "$CHANGED_FILES"

# Check 2: Function length (> 50 lines)
while IFS= read -r file; do
  [[ -f "$file" ]] || continue
  ext="${file##*.}"
  case "$ext" in
    sh|bash|py|js|ts|tsx|jsx|go|java|rs)
      # Count lines in function-like blocks (simplified heuristic)
      awk '
        /^(function |func |def |fn )/ || /^[a-zA-Z_]+\(\)/ {
          if (func_name != "" && func_lines > 50) {
            printf "MEDIUM:maintainability:%s:%d:Function %s is %d lines (>50)\n", FILENAME, NR-func_lines, func_name, func_lines
          }
          func_name = $0; func_lines = 0
        }
        func_name != "" { func_lines++ }
        END {
          if (func_name != "" && func_lines > 50) {
            printf "MEDIUM:maintainability:%s:%d:Function %s is %d lines (>50)\n", FILENAME, NR-func_lines, func_name, func_lines
          }
        }
      ' "$file" 2>/dev/null > /tmp/crs_func_findings.txt
      while IFS= read -r finding; do
        FINDINGS+=("$finding")
        WARN_COUNT=$((WARN_COUNT + 1))
      done < /tmp/crs_func_findings.txt
      ;;
  esac
done <<< "$CHANGED_FILES"

# Check 3: Nesting depth (> 4 levels)
while IFS= read -r file; do
  [[ -f "$file" ]] || continue
  max_indent=$(awk '{ match($0, /^[[:space:]]*/); indent=RLENGTH/2; if(indent>max) max=indent } END { print max+0 }' "$file")
  if [[ "$max_indent" -gt 4 ]]; then
    add_finding "MEDIUM" "maintainability" "$file" "0" "Max nesting depth ${max_indent} exceeds 4 levels"
  fi
done <<< "$CHANGED_FILES"

# Check 4: Hardcoded secrets detection
SECRET_PATTERNS='(password|secret|api[_-]?key|token|private[_-]?key)\s*[:=]\s*["\x27][^\s"'\'']{8,}'
while IFS= read -r file; do
  [[ -f "$file" ]] || continue
  # Skip binary files and common false-positive files
  [[ "$file" == *.md ]] && continue
  [[ "$file" == *.json ]] && continue
  matches=$(grep -nEi "$SECRET_PATTERNS" "$file" 2>/dev/null | head -5 || true)
  if [[ -n "$matches" ]]; then
    while IFS= read -r match; do
      line_num=$(echo "$match" | cut -d: -f1)
      add_finding "CRITICAL" "security" "$file" "$line_num" "Potential hardcoded secret detected"
    done <<< "$matches"
  fi
done <<< "$CHANGED_FILES"

# Check 5: Complexity (if complexity-check.sh exists)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -x "$SCRIPT_DIR/complexity-check.sh" ]]; then
  while IFS= read -r file; do
    [[ -f "$file" ]] || continue
    result=$("$SCRIPT_DIR/complexity-check.sh" "$file" --threshold 15 2>/dev/null || true)
    if echo "$result" | grep -q "EXCEEDED"; then
      while IFS= read -r line; do
        if echo "$line" | grep -q "EXCEEDED"; then
          func=$(echo "$line" | cut -d: -f2)
          score=$(echo "$line" | grep -oE 'complexity=[0-9]+' | cut -d= -f2)
          add_finding "MEDIUM" "maintainability" "$file" "0" "Function $func has complexity $score (>15)"
        fi
      done <<< "$result"
    fi
  done <<< "$CHANGED_FILES"
fi

# Check 6: Near-duplicate detection (simplified — exact line match)
while IFS= read -r file; do
  [[ -f "$file" ]] || continue
  # Find blocks of 6+ identical consecutive non-blank lines
  dup_count=$(awk '
    NF > 0 { line[NR] = $0 }
    END {
      dupes = 0
      for (i = 1; i <= NR; i++) {
        for (j = i+6; j <= NR; j++) {
          match = 0
          for (k = 0; k < 6; k++) {
            if (line[i+k] == line[j+k]) match++
          }
          if (match >= 6) { dupes++; break }
        }
        if (dupes > 0) break
      }
      print dupes
    }
  ' "$file" 2>/dev/null || echo 0)
  if [[ "$dup_count" -gt 0 ]]; then
    add_finding "LOW" "maintainability" "$file" "0" "Potential near-duplicate code blocks detected"
  fi
done <<< "$CHANGED_FILES"

# Security-sensitive file detection (for tier escalation)
SECURITY_SENSITIVE=false
while IFS= read -r file; do
  if echo "$file" | grep -qEi 'auth|login|password|token|secret|payment|billing|checkout|eval|grader|agents/|skills/.*/SKILL'; then
    SECURITY_SENSITIVE=true
    break
  fi
done <<< "$CHANGED_FILES"

if [[ "$SECURITY_SENSITIVE" == "true" && "$TIER" != "full" ]]; then
  TIER="full"
fi

# === OUTPUT ===
echo "# Code Review Pipeline Report"
echo ""
echo "## Summary"
echo "- **Tier:** $TIER"
echo "- **Changed:** $FILE_COUNT files, $DIFF_LINES lines"
echo "- **Issues:** $ISSUE_COUNT critical/high, $WARN_COUNT medium/low"
echo "- **Security-sensitive:** $SECURITY_SENSITIVE"
echo ""

if [[ ${#FINDINGS[@]} -gt 0 ]]; then
  echo "## Findings"
  printf "| Severity | Dimension | File | Line | Description |\n"
  printf "|----------|-----------|------|------|-------------|\n"
  for finding in "${FINDINGS[@]}"; do
    IFS=':' read -r sev dim file line msg <<< "$finding"
    printf "| %s | %s | %s | %s | %s |\n" "$sev" "$dim" "$file" "$line" "$msg"
  done
else
  echo "## Findings"
  echo "No issues detected by pipeline checks."
fi

echo ""
echo "## Recommendation"
if [[ "$ISSUE_COUNT" -gt 0 ]]; then
  echo "FAIL — $ISSUE_COUNT critical/high issues require attention before shipping."
  exit 1
elif [[ "$WARN_COUNT" -gt 3 ]]; then
  echo "WARN — $WARN_COUNT medium/low issues detected. Consider simplification."
  exit 0
else
  echo "PASS — Pipeline checks clean."
  exit 0
fi
