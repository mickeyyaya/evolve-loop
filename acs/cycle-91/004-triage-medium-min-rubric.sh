#!/usr/bin/env bash
# AC-ID: cycle-91-004-triage-medium-min-rubric
# Description: Verifies that agents/evolve-triage.md was updated to rate any
#   cycle whose touched files appear in `grep -rl <basename> acs/regression-suite/`
#   results as MEDIUM minimum risk, regardless of content domain. Three
#   required tokens:
#     (a) `regression-suite` — the predicate-graph identifier
#     (b) `MEDIUM` — the minimum risk floor
#     (c) language that overrides the docs-domain=low-risk heuristic
#         (look for one of: override | regardless | minimum | floor | even if
#         | docs)  AND a co-occurring "regression-suite" or "predicate" token
#         on the same line/window.
# Evidence: intent.md:acceptance_checks bullet 4; intent.md:interfaces bullet 4.
# Author: tdd-engineer (cycle-91)
# Created: 2026-05-20
# Acceptance-of: build-report.md row "agents/evolve-triage.md MEDIUM-min rubric"
#
# Behavioral: a mutant that adds the MEDIUM floor in prose but doesn't tie it
# to "regression-suite" fails — the floor must be triggered by predicate-graph
# reachability specifically. A mutant that adds the regression-suite anchor
# but uses LOW instead of MEDIUM fails the floor check.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
PERSONA="$REPO_ROOT/agents/evolve-triage.md"
AC_ID="cycle-91-004-triage-medium-min-rubric"

if [ ! -f "$PERSONA" ]; then
  echo "RED $AC_ID: agents/evolve-triage.md not found at $PERSONA" >&2
  exit 1
fi

missing=""

# (a) Literal `regression-suite`
if ! grep -qF 'regression-suite' "$PERSONA"; then
  missing="${missing} 'regression-suite'"
fi

# (b) Literal `MEDIUM` (case-sensitive — the risk label is conventionally caps)
if ! grep -qF 'MEDIUM' "$PERSONA"; then
  missing="${missing} 'MEDIUM'"
fi

# (c) Override-language proximity: regression-suite/predicate token + override
#     language within a 6-line window. We check for any of these override
#     vocabulary terms appearing near the regression-suite/predicate anchor.
override_ok=$(awk '
  BEGIN { IGNORECASE = 1 }
  {
    buf[NR % 8] = tolower($0)
    have_anchor = 0
    have_override = 0
    have_medium = 0
    for (i = 0; i < 8; i++) {
      if (index(buf[i], "regression-suite") > 0 || index(buf[i], "predicate-graph") > 0 || index(buf[i], "predicate-reachable") > 0) {
        have_anchor = 1
      }
      if (buf[i] ~ /override|regardless|minimum|floor|even if|docs-domain/) {
        have_override = 1
      }
      if (index(buf[i], "medium") > 0) {
        have_medium = 1
      }
    }
    if (have_anchor && have_override && have_medium) { found = 1; print "OK"; exit }
  }
  END { if (!found) print "MISSING" }
' "$PERSONA")

if [ "$override_ok" != "OK" ]; then
  missing="${missing} override-rule-near-regression-suite+MEDIUM"
fi

if [ -n "$missing" ]; then
  echo "RED $AC_ID: agents/evolve-triage.md missing required tokens:${missing}" >&2
  exit 1
fi

echo "GREEN $AC_ID: agents/evolve-triage.md rates predicate-graph-reachable cycles as MEDIUM minimum (overrides docs-domain=low)"
exit 0
