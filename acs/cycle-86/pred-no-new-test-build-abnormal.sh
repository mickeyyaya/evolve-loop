#!/usr/bin/env bash
# AC-ID: cycle-86-no-new-test-build-abnormal
# Verifies that no NEW abnormal events of type `ship-refused` or `turn-overrun`
# are emitted by the test or build (Builder) phases of cycle 86.
#
# Permitted (pre-existing, not a violation):
#   - The cycle-86 intent-phase `turn-overrun` WARN — that record was logged
#     before this acceptance check exists and is the very thing being dismissed.
#
# Forbidden (would mean we re-introduced the underlying signal during the
# very cycle that is supposed to dismiss it):
#   - Any `ship-refused` or `turn-overrun` entry whose `source_phase` matches
#     `tdd-engineer`, `builder`, `tester`, or `subagent-run` with agent in
#     {tdd-engineer, builder, tester}.
set -uo pipefail
REPO_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
CYCLE_LOG="$REPO_ROOT/.evolve/runs/cycle-86/abnormal-events.jsonl"

# Absence of the cycle log is the trivially-green case.
if [ ! -f "$CYCLE_LOG" ]; then
  echo "GREEN cycle-86-no-new-test-build-abnormal: no cycle-86 abnormal-events.jsonl present"
  exit 0
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "RED cycle-86-no-new-test-build-abnormal: jq not on PATH" >&2
  exit 1
fi

# Count violations: event_type in {ship-refused, turn-overrun} AND details/source
# implicate test or build phases. We accept the existing intent-phase WARN by
# excluding `agent=intent` substrings.
violations=$(jq -r --slurp '
  [ .[]
    | select(.event_type == "ship-refused" or .event_type == "turn-overrun")
    | select(
        (.details // "") | test("agent=(tdd-engineer|builder|tester)")
      )
  ] | length
' "$CYCLE_LOG" 2>/dev/null)

if [ -z "$violations" ]; then
  echo "RED cycle-86-no-new-test-build-abnormal: jq parse failed on $CYCLE_LOG" >&2
  exit 1
fi

if [ "$violations" -ne 0 ]; then
  echo "RED cycle-86-no-new-test-build-abnormal: $violations forbidden event(s) from test/build phase(s)" >&2
  jq -r --slurp '
    .[] | select(.event_type == "ship-refused" or .event_type == "turn-overrun")
        | select((.details // "") | test("agent=(tdd-engineer|builder|tester)"))
        | "  - \(.event_type) \(.severity) \(.details)"
  ' "$CYCLE_LOG" >&2
  exit 1
fi

echo "GREEN cycle-86-no-new-test-build-abnormal: no forbidden events from test/build phases"
exit 0
