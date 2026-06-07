#!/usr/bin/env bash
# ACS — cycle-249 task `runner-base-cycle-context`
# acs-predicate: structural-refactor-check (grep waiver) — this criterion
# IS source structure: "each refactored cluster: shared logic in one code
# location, grep finds no remaining copy" (intent AC2 / scout gate row 1).
# The behavioral anchor lives in 001- (byte-parity + projection
# equivalence); this predicate adds the subprocess compile proof and the
# zero-remaining-copies sweep.
#
# Marker: `- cycle: %d` is the distinctive format string of the duplicated
# core block (present in all 10 pre-refactor sites; scout's published
# pattern '"## Cycle Context"' matches nothing even pre-refactor — quoted
# literal never occurs — so this predicate uses the precise marker).
#
# Exclusions, with reasons:
#   /runner/ — the single allowed home of the block (BaseCycleContext)
#   /retro/  — NOT a copy: retro emits previous_verdict instead of
#              goal_hash; migrating it would change prompt bytes, which
#              intent non-goal #1 forbids ("do not change runtime behavior")
#   _test.go — tests may pin expected prompt bytes
set -uo pipefail

top=$(git rev-parse --show-toplevel)
cd "$top/go" || { echo "RED: cannot cd to go module" >&2; exit 1; }

# Behavioral portion: the refactored tree must compile.
if ! go build ./... >/dev/null 2>&1; then
  echo "RED: go build ./... failed — refactor does not compile" >&2
  exit 1
fi

count=$(grep -rn -- '- cycle: %d' "$top/go/internal/phases/" --include='*.go' \
  | grep -v '/runner/' | grep -v '/retro/' | grep -v '_test.go' | wc -l | tr -d ' ')
if [ "$count" -ne 0 ]; then
  echo "RED: $count phase file(s) outside runner/ still hand-build the core Cycle Context block:" >&2
  grep -rn -- '- cycle: %d' "$top/go/internal/phases/" --include='*.go' \
    | grep -v '/runner/' | grep -v '/retro/' | grep -v '_test.go' >&2
  exit 1
fi

echo "GREEN: core block lives only in runner/ (retro excluded by design — different field set)"
exit 0
