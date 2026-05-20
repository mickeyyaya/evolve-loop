#!/usr/bin/env bash
# AC-ID: cycle-94-004-stream-json-operator-visibility
# AC-source: cycle-94/intent.md acceptance_check #5
# Behavioral predicate: L2 — scripts/dispatch/subagent-run.sh emits a
# post-phase summary line of the form `[<agent>] turn N/M $X.XX` to
# operator stderr, sourced from the existing usage_sidecar JSON.
#
# Requirements:
#   * The block reads the usage_sidecar (num_turns + total_cost_usd)
#     and the effective profile (max_turns)
#   * Output goes to stderr (>&2), NOT to stdout (which carries the
#     parseable claude -p JSON — must remain unchanged)
#   * The summary line uses the literal pattern `[<agent>] turn N/M`
#
# Rationale: stream-json data already flows through subagent-run.sh
# (claude.sh:157–161 uses `--output-format stream-json --verbose`) but
# operator visibility is silent until the cycle completes. A one-line
# summary per phase, on stderr, surfaces progress without breaking
# captured stdout parsers.
#
# RED until Builder adds the block; GREEN once present.
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}}"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="scripts/dispatch/subagent-run.sh"
if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET not found" >&2
  exit 1
fi

# Check 1: an L2 block or banner mentioning stream-json operator visibility
if ! grep -Eiq 'stream.?json[[:space:]]+operator[[:space:]]+visibility|L2[[:space:]]*[:.]' "$TARGET"; then
  echo "RED: $TARGET missing L2 stream-json operator visibility banner" >&2
  exit 1
fi

# Check 2: usage_sidecar read for num_turns and total_cost_usd
if ! grep -q 'num_turns' "$TARGET"; then
  echo "RED: $TARGET does not extract num_turns" >&2
  exit 1
fi
if ! grep -q 'total_cost_usd' "$TARGET"; then
  echo "RED: $TARGET does not extract total_cost_usd" >&2
  exit 1
fi

# Check 3: the literal output pattern `[<agent>] turn` appears in an
# echo / printf paired with `>&2`. Search a focused window so an
# unrelated `>&2` elsewhere doesn't satisfy the predicate.
# Find any line with `] turn` and check it (or the next 2 lines) goes
# to stderr via >&2.
turn_line=$(grep -n '] turn' "$TARGET" | head -1 | cut -d: -f1)
if [ -z "$turn_line" ]; then
  echo "RED: $TARGET does not emit '[<agent>] turn ...' summary string" >&2
  exit 1
fi
start_line=$turn_line
end_line=$((turn_line + 2))
window=$(sed -n "${start_line},${end_line}p" "$TARGET")
if ! echo "$window" | grep -q '>&2'; then
  echo "RED: '[<agent>] turn ...' summary not routed to stderr (>&2)" >&2
  exit 1
fi

echo "GREEN: stream-json post-phase summary routed to operator stderr"
exit 0
