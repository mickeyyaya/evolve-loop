#!/usr/bin/env bash
# ACS — cycle 239 / intent AC1: eval files exist for every scout-selected slug
#
# acs-predicate: config-check
# Classification: WAIVED GREP (inherent artifact-presence check). The
# cycle-238 audit returned CRITICAL because the three scout-selected slugs
# had no .evolve/evals/<slug>.md in the workspace — discovered only at audit
# time, costing the whole cycle. This predicate moves that discovery to
# suite-run time. There is no system to subprocess-invoke here: the AC *is*
# "these files exist and are non-empty" (.evolve/ is gitignored, so the
# git-tracking half of the file-existence dual-check rule is structurally
# inapplicable — disk presence in the active tree is the contract).
#
# Cycle-scoped note: runs against the ACTIVE tree's .evolve/evals/. In the
# cycle-239 worktree these are the canonical score_cap-schema versions
# authored by the TDD phase; on main they are the scout-authored copies.
# Existence + non-emptiness is asserted; schema is not, to avoid a false
# RED if re-run on main post-ship.
set -uo pipefail
top=$(git rev-parse --show-toplevel)

fail=0
for slug in profile-provenance-field persona-tools-coherence-gate persona-output-artifact-coherence; do
  f="$top/.evolve/evals/$slug.md"
  if [ ! -s "$f" ]; then
    echo "RED: missing or empty eval file: $f (cycle-238 CRITICAL recurrence)" >&2
    fail=1
  fi
done
[ "$fail" -eq 0 ] || exit 1

echo "GREEN: all 3 scout-selected slugs have non-empty .evolve/evals/<slug>.md" >&2
exit 0
