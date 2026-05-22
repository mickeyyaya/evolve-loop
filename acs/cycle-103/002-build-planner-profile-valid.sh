#!/usr/bin/env bash
# AC-ID: cycle-103-002-build-planner-profile-valid
# AC-source: scout-report.md AC-1 (lines 320, 342-345), profile spec lines 82-103
# Behavioral predicate:
#   .evolve/profiles/build-planner.json must be valid JSON with:
#     - parallel_eligible == false  (single-writer invariant)
#     - max_turns == 10             (per spec)
#     - max_budget_usd == 0.30      (per spec)
#     - challenge_token_required == true
#     - output_artifact contains "build-plan.md"
#
# Mutation spec (cycle-103-002-MUT):
#   Mutant: max_turns: 25            -> must FAIL.
#   Mutant: parallel_eligible: true  -> must FAIL (would break single-writer).
#   Mutant: max_budget_usd: 1.50     -> must FAIL.
#   Mutant: output_artifact missing  -> must FAIL.
#   Mutant: invalid JSON             -> must FAIL.
#
# Bash 3.2 compatible. Depends on jq.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

PROFILE=".evolve/profiles/build-planner.json"

if [ ! -f "$PROFILE" ]; then
  echo "RED: $PROFILE does not exist" >&2
  exit 1
fi

# Valid JSON?
if ! jq -e . "$PROFILE" >/dev/null 2>&1; then
  echo "RED: $PROFILE is not valid JSON" >&2
  jq . "$PROFILE" 2>&1 | head -5 >&2 || true
  exit 1
fi

check_field() {
  local field="$1"
  local expected="$2"
  local actual
  actual="$(jq -r "$field" "$PROFILE" 2>/dev/null)"
  if [ "$actual" != "$expected" ]; then
    echo "RED: $PROFILE field $field: got '$actual', expected '$expected'" >&2
    return 1
  fi
  return 0
}

check_field '.parallel_eligible' 'false' || exit 1
check_field '.max_turns' '10' || exit 1
check_field '.challenge_token_required' 'true' || exit 1

# max_budget_usd: jq -r emits "0.3" for 0.30; compare numerically via awk.
budget="$(jq -r '.max_budget_usd' "$PROFILE" 2>/dev/null)"
if [ "$budget" = "null" ] || [ -z "$budget" ]; then
  echo "RED: $PROFILE missing max_budget_usd" >&2
  exit 1
fi
diff_ok="$(awk -v a="$budget" -v b="0.30" 'BEGIN { d = a - b; if (d < 0) d = -d; print (d < 1e-9) ? "ok" : "no" }')"
if [ "$diff_ok" != "ok" ]; then
  echo "RED: $PROFILE max_budget_usd: got '$budget', expected '0.30'" >&2
  exit 1
fi

# output_artifact contains build-plan.md
output_artifact="$(jq -r '.output_artifact' "$PROFILE" 2>/dev/null)"
case "$output_artifact" in
  *build-plan.md*) : ;;
  *)
    echo "RED: $PROFILE output_artifact='$output_artifact' does not contain 'build-plan.md'" >&2
    exit 1
    ;;
esac

echo "GREEN: $PROFILE valid (parallel_eligible=false, max_turns=10, max_budget_usd=0.30, output_artifact ok)"
exit 0
