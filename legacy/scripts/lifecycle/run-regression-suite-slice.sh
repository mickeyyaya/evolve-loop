#!/usr/bin/env bash
# run-regression-suite-slice.sh — Pre-handoff regression-suite slice runner (cycle-91+).
#
# PURPOSE: Builder MUST invoke this script before writing build-report.md whenever
# the cycle's touched files intersect the grep targets of existing regression-suite
# predicates. See agents/evolve-builder.md "Pre-handoff Regression Slice" section.
#
# HEURISTIC: reachability is computed via `grep -rl <basename> acs/regression-suite/`
# for each touched-file basename. This covers grep/fgrep/rg target references in
# predicate scripts. awk/jq/cat-based targets are NOT captured — flagged here for
# future-cycle expansion but out-of-scope per cycle-91 lesson.
#
# USAGE:
#   # via stdin (primary contract):
#   printf 'CLAUDE.md\nagents/evolve-builder.md\n' | bash legacy/scripts/lifecycle/run-regression-suite-slice.sh
#
#   # via argv (alternative contract):
#   bash legacy/scripts/lifecycle/run-regression-suite-slice.sh CLAUDE.md agents/evolve-builder.md
#
# OUTPUT:
#   Final stdout line: "N/N PASS" or "N/M FAIL <space-separated-ids>"
#   Empty-slice case: "0/0 PASS — no predicate-graph reachability"
#
# EXIT CODES:
#   0  — all slice predicates GREEN (or empty slice)
#   1  — at least one slice predicate RED
#   2  — infra error (repo root not found, acs/regression-suite missing)

set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
SUITE_DIR="$REPO_ROOT/acs/regression-suite"

if [ ! -d "$SUITE_DIR" ]; then
  echo "ERROR: acs/regression-suite not found at $SUITE_DIR" >&2
  exit 2
fi

# Collect touched-file paths: from argv if given, else from stdin.
touched_paths=""
if [ "$#" -gt 0 ]; then
  for arg in "$@"; do
    touched_paths="${touched_paths}${arg}
"
  done
else
  touched_paths=$(cat)
fi

if [ -z "$(printf '%s' "$touched_paths" | tr -d '[:space:]')" ]; then
  echo "0/0 PASS — no touched files provided"
  exit 0
fi

# Extract unique basenames from touched paths.
basenames=""
while IFS= read -r path; do
  [ -z "$path" ] && continue
  bn=$(basename "$path")
  [ -z "$bn" ] && continue
  # Deduplicate: skip if already in list.
  case "$basenames" in
    *"
${bn}
"*) ;;
    *) basenames="${basenames}${bn}
" ;;
  esac
done <<EOF
$touched_paths
EOF

# Build slice: collect unique predicate scripts that reference any touched basename.
slice_scripts=""

while IFS= read -r bn; do
  [ -z "$bn" ] && continue
  # Find predicate scripts referencing this basename.
  matches=$(grep -rl "$bn" "$SUITE_DIR" 2>/dev/null || true)
  if [ -n "$matches" ]; then
    while IFS= read -r m; do
      [ -z "$m" ] && continue
      # Deduplicate.
      case "$slice_scripts" in
        *"
${m}
"*) ;;
        *) slice_scripts="${slice_scripts}${m}
" ;;
      esac
    done <<MEOF
$matches
MEOF
  fi
done <<EOF
$basenames
EOF

# Empty-slice case: no reachable predicates.
if [ -z "$(printf '%s' "$slice_scripts" | tr -d '[:space:]')" ]; then
  echo "0/0 PASS — no predicate-graph reachability"
  exit 0
fi

# Run the slice.
total=0
passed=0
failed_ids=""

while IFS= read -r script; do
  [ -z "$script" ] && continue
  [ ! -f "$script" ] && continue
  total=$((total + 1))
  # Derive a short ID from the script path (relative to SUITE_DIR).
  rel_path="${script#$SUITE_DIR/}"
  script_id=$(basename "$rel_path" .sh)
  parent=$(basename "$(dirname "$rel_path")")
  pred_id="${parent}/${script_id}"

  set +e
  bash "$script" >/dev/null 2>&1
  rc=$?
  set -e

  if [ "$rc" -eq 0 ]; then
    passed=$((passed + 1))
  else
    failed_ids="${failed_ids} ${pred_id}"
  fi
done <<EOF
$slice_scripts
EOF

if [ "$passed" -eq "$total" ]; then
  echo "${passed}/${total} PASS"
  exit 0
else
  echo "${passed}/${total} FAIL${failed_ids}"
  exit 1
fi
