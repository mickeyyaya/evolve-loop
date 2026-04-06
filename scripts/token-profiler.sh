#!/usr/bin/env bash
# token-profiler.sh — Measure token footprint of all skill, agent, and phase files.
# Usage: token-profiler.sh [--json] [--save-baseline] [--compare]
# Outputs a ranked table of files by estimated token count (1 line ≈ 15 tokens).
# --save-baseline: Save current measurements to .evolve/token-baseline.json
# --compare: Compare current measurements against saved baseline, show delta
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE_FILE="$REPO_ROOT/.evolve/token-baseline.json"
JSON_OUTPUT=false
SAVE_BASELINE=false
COMPARE_MODE=false

for arg in "$@"; do
  case "$arg" in
    --json) JSON_OUTPUT=true ;;
    --save-baseline) SAVE_BASELINE=true ;;
    --compare) COMPARE_MODE=true ;;
  esac
done

declare -a ENTRIES=()
TOTAL_LINES=0
TOTAL_TOKENS=0

add_entry() {
  local category="$1" file="$2"
  local full_path="$REPO_ROOT/$file"
  [[ -f "$full_path" ]] || return 0
  local lines
  lines=$(wc -l < "$full_path" | tr -d ' ')
  local tokens=$((lines * 15))
  ENTRIES+=("$tokens|$lines|$category|$file")
  TOTAL_LINES=$((TOTAL_LINES + lines))
  TOTAL_TOKENS=$((TOTAL_TOKENS + tokens))
}

# Scan skill directories
for skill_dir in "$REPO_ROOT"/skills/*/; do
  skill_name=$(basename "$skill_dir")
  [[ -f "$skill_dir/SKILL.md" ]] && add_entry "skill:$skill_name" "skills/$skill_name/SKILL.md"
  if [[ -d "$skill_dir/reference" ]]; then
    for ref_file in "$skill_dir"/reference/*.md; do
      [[ -f "$ref_file" ]] && add_entry "skill:$skill_name/ref" "skills/$skill_name/reference/$(basename "$ref_file")"
    done
  fi
  # Scan other .md files in skill dir (phases, protocols, etc.)
  for md_file in "$skill_dir"/*.md; do
    [[ -f "$md_file" ]] || continue
    local_name=$(basename "$md_file")
    [[ "$local_name" == "SKILL.md" ]] && continue
    add_entry "skill:$skill_name" "skills/$skill_name/$local_name"
  done
done

# Scan agent files
for agent_file in "$REPO_ROOT"/agents/*.md; do
  [[ -f "$agent_file" ]] && add_entry "agent" "agents/$(basename "$agent_file")"
done

# Scan scripts
for script_file in "$REPO_ROOT"/scripts/*.sh; do
  [[ -f "$script_file" ]] && add_entry "script" "scripts/$(basename "$script_file")"
done

# Sort entries by token count (descending)
IFS=$'\n' SORTED=($(printf '%s\n' "${ENTRIES[@]}" | sort -t'|' -k1 -rn)); unset IFS

if $JSON_OUTPUT; then
  echo "{"
  echo "  \"totalLines\": $TOTAL_LINES,"
  echo "  \"totalTokens\": $TOTAL_TOKENS,"
  echo "  \"files\": ["
  for i in "${!SORTED[@]}"; do
    IFS='|' read -r tokens lines category file <<< "${SORTED[$i]}"
    comma=","
    [[ $i -eq $((${#SORTED[@]} - 1)) ]] && comma=""
    echo "    {\"tokens\": $tokens, \"lines\": $lines, \"category\": \"$category\", \"file\": \"$file\"}$comma"
  done
  echo "  ]"
  echo "}"
else
  echo "# Token Footprint Report"
  echo ""
  echo "Total: $TOTAL_LINES lines, ~$TOTAL_TOKENS tokens"
  echo ""
  printf "| %-6s | %-6s | %-25s | %-55s |\n" "Tokens" "Lines" "Category" "File"
  printf "|--------|--------|---------------------------|----------------------------------------------------------|\n"
  for entry in "${SORTED[@]}"; do
    IFS='|' read -r tokens lines category file <<< "$entry"
    printf "| %-6s | %-6s | %-25s | %-55s |\n" "$tokens" "$lines" "$category" "$file"
  done
  echo ""

  # Summary by category (awk-based for shell compatibility)
  echo "## Summary by Category"
  echo ""
  printf "| %-30s | %-8s | %-8s |\n" "Category" "Lines" "Tokens"
  printf "|--------------------------------|----------|----------|\n"
  printf '%s\n' "${SORTED[@]}" | awk -F'|' '{
    cat = $3; sub(/\/.*/, "", cat)
    lines[cat] += $2; tokens[cat] += $1
  } END {
    for (cat in tokens) printf "| %-30s | %-8d | %-8d |\n", cat, lines[cat], tokens[cat]
  }' | sort -t'|' -k3 -rn
fi

# Save baseline
if $SAVE_BASELINE; then
  echo "{" > "$BASELINE_FILE"
  echo "  \"timestamp\": \"$(date -u +"%Y-%m-%dT%H:%M:%SZ")\"," >> "$BASELINE_FILE"
  echo "  \"totalLines\": $TOTAL_LINES," >> "$BASELINE_FILE"
  echo "  \"totalTokens\": $TOTAL_TOKENS," >> "$BASELINE_FILE"
  echo "  \"files\": {" >> "$BASELINE_FILE"
  for i in "${!SORTED[@]}"; do
    IFS='|' read -r tokens lines category file <<< "${SORTED[$i]}"
    comma=","
    [[ $i -eq $((${#SORTED[@]} - 1)) ]] && comma=""
    echo "    \"$file\": $tokens$comma" >> "$BASELINE_FILE"
  done
  echo "  }" >> "$BASELINE_FILE"
  echo "}" >> "$BASELINE_FILE"
  echo "Baseline saved to $BASELINE_FILE ($TOTAL_TOKENS tokens)" >&2
fi

# Compare against baseline
if $COMPARE_MODE; then
  if [[ ! -f "$BASELINE_FILE" ]]; then
    echo "No baseline found. Run --save-baseline first." >&2
    exit 1
  fi
  BASELINE_TOTAL=$(python3 -c "import json; print(json.load(open('$BASELINE_FILE'))['totalTokens'])")
  BASELINE_TS=$(python3 -c "import json; print(json.load(open('$BASELINE_FILE'))['timestamp'])")
  DELTA=$((TOTAL_TOKENS - BASELINE_TOTAL))
  SIGN="+"
  [[ "$DELTA" -lt 0 ]] && SIGN=""
  echo ""
  echo "## Comparison vs Baseline ($BASELINE_TS)"
  echo ""
  echo "| Metric | Baseline | Current | Delta |"
  echo "|--------|----------|---------|-------|"
  echo "| Total tokens | $BASELINE_TOTAL | $TOTAL_TOKENS | ${SIGN}${DELTA} |"
  echo ""
  echo "### Per-File Changes"
  printf "| %-55s | %-8s | %-8s | %-8s |\n" "File" "Baseline" "Current" "Delta"
  printf "|---------------------------------------------------------|----------|----------|----------|\n"
  for entry in "${SORTED[@]}"; do
    IFS='|' read -r tokens lines category file <<< "$entry"
    baseline_tokens=$(python3 -c "import json; d=json.load(open('$BASELINE_FILE')); print(d['files'].get('$file', 0))" 2>/dev/null || echo 0)
    file_delta=$((tokens - baseline_tokens))
    [[ "$file_delta" -eq 0 ]] && continue
    file_sign="+"
    [[ "$file_delta" -lt 0 ]] && file_sign=""
    printf "| %-55s | %-8s | %-8s | %-8s |\n" "$file" "$baseline_tokens" "$tokens" "${file_sign}${file_delta}"
  done
fi
