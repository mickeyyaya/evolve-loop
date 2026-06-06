#!/usr/bin/env bash
# AC-ID:         cycle-233-AC5-binary-guard (supplementary to AC5 manual+checklist)
# acs-predicate: config-check — repo-state guard; the system under test is the
#                git tree itself (the leak being guarded is a COMMITTED binary,
#                which only git can observe). Waiver per TDD predicate rules.
# Description:   cycle diff vs merge-base(main) must not touch go/evolve (tracked-binary leak guard)
# Evidence:      289f25c carries a go/evolve Bin delta; intent constraint: "do not commit the binary"
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: intent.md AC5 sub-clause "no tracked go/evolve binary in the commit"

set -uo pipefail

base=$(git merge-base main HEAD 2>/dev/null) \
  || base=$(git merge-base origin/main HEAD 2>/dev/null) \
  || { echo "GREEN: no main ref resolvable — guard vacuous" >&2; exit 0; }

if git diff --name-only "$base"..HEAD | grep -qx 'go/evolve'; then
  echo "RED: go/evolve binary modified in cycle diff ($base..HEAD) — drop it:" >&2
  echo "     git checkout HEAD~1 -- go/evolve && git commit --amend --no-edit" >&2
  exit 1
fi

echo "GREEN: no go/evolve churn in cycle diff ($base..HEAD)" >&2
exit 0
