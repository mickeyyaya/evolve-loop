#!/usr/bin/env bash
# AC-ID: cycle-104-002-builder-step3-preserved
# AC-source: scout-report.md AC EG-104-02 (lines 215-218); intent.md acceptance_checks[1]
# Behavioral predicate (PRESERVATION invariant — start GREEN, must stay GREEN):
#   agents/evolve-builder.md must continue to contain the literal heading
#   "### Step 3: Design (chain-of-thought required)" verbatim. Cycle 104
#   adds Step 2.8 (advisory build-plan read) BEFORE Step 3; it must NOT
#   modify, rename, or remove Step 3. Step 3 remains the authoritative
#   design driver in advisory mode (v10.20). Removal is deferred to cycle
#   105 enforce phase.
#
# Mutation spec (cycle-104-002-MUT):
#   Mutant: Step 3 heading deleted                              -> must FAIL.
#   Mutant: Step 3 heading renamed (e.g., "Step 3: Plan")       -> must FAIL.
#   Mutant: Step 3 heading altered (case/punctuation change)    -> must FAIL.
#
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN (preservation invariant holds)
#   1 = RED   (Step 3 missing or altered)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

BUILDER="agents/evolve-builder.md"

if [ ! -f "$BUILDER" ]; then
  echo "RED: $BUILDER does not exist" >&2
  exit 1
fi

# Required: literal heading must appear exactly.
EXPECTED='### Step 3: Design (chain-of-thought required)'

if ! grep -qF "$EXPECTED" "$BUILDER"; then
  echo "RED: $BUILDER missing verbatim heading: $EXPECTED" >&2
  echo "Found these 'Step 3' lines instead:" >&2
  grep -nE '^###[[:space:]]+Step[[:space:]]+3' "$BUILDER" >&2 || true
  exit 1
fi

echo "GREEN: $BUILDER preserves verbatim '$EXPECTED'"
exit 0
