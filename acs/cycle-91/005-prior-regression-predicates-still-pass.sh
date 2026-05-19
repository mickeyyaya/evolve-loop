#!/usr/bin/env bash
# AC-ID: cycle-91-005-prior-regression-predicates-still-pass
# Description: Asserts that the three regression predicates broken in the
#   original cycle-91 run remain PASS after this cycle's edits — i.e., that
#   the structural-preventives implementation introduces NO new regression to
#   the data-layer remediation already committed in cycle-90 commit 940da5d.
#   The three predicates:
#     - acs/regression-suite/cycle-49/006-claude-md-schema.sh
#     - acs/regression-suite/cycle-89/003-claude-md-research-env-vars.sh
#     - acs/regression-suite/cycle-89/004-research-tool-adr-exists.sh
# Evidence: intent.md:acceptance_checks bullet 5.
# Author: tdd-engineer (cycle-91)
# Created: 2026-05-20
# Acceptance-of: build-report.md row "cycle-91 RED predicates PASS confirmed"
#
# Behavioral: invokes each predicate fresh and captures exit code + final
# line. A mutant that re-trims CLAUDE.md (e.g., "consolidating" the research
# env-var table) breaks this predicate. A mutant that deletes the ADR
# breaks this predicate. The predicates' own outputs are reproduced in
# stderr for diagnostic visibility.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
AC_ID="cycle-91-005-prior-regression-predicates-still-pass"

# List of predicates that the lesson specifically named as broken in the
# original cycle-91 run. Each MUST PASS after this cycle's edits.
PREDICATES="
acs/regression-suite/cycle-49/006-claude-md-schema.sh
acs/regression-suite/cycle-89/003-claude-md-research-env-vars.sh
acs/regression-suite/cycle-89/004-research-tool-adr-exists.sh
"

failed=""
missing_predicate=""

for p in $PREDICATES; do
  full="$REPO_ROOT/$p"
  if [ ! -f "$full" ]; then
    missing_predicate="${missing_predicate} $p"
    continue
  fi
  if [ ! -x "$full" ]; then
    # Run via bash explicitly if not executable (don't fail on chmod issue).
    runner="bash $full"
  else
    runner="$full"
  fi
  set +e
  out=$(cd "$REPO_ROOT" && $runner 2>&1)
  rc=$?
  set -e
  if [ "$rc" -ne 0 ]; then
    failed="${failed} $p(rc=$rc)"
    echo "[diagnostic] predicate $p output:" >&2
    printf '%s\n' "$out" | sed 's/^/    /' >&2
  fi
done

if [ -n "$missing_predicate" ]; then
  echo "RED $AC_ID: required predicate files missing:${missing_predicate}" >&2
  exit 1
fi

if [ -n "$failed" ]; then
  echo "RED $AC_ID: prior-broken predicates regressed:${failed}" >&2
  exit 1
fi

echo "GREEN $AC_ID: all 3 prior-broken predicates (cycle-49/006, cycle-89/003, cycle-89/004) remain PASS"
exit 0
