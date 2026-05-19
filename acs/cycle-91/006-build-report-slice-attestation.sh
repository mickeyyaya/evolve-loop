#!/usr/bin/env bash
# AC-ID: cycle-91-006-build-report-slice-attestation
# Description: Self-attestation predicate. Verifies that Builder's own
#   build-report.md for cycle 91 includes the verbatim PASS/FAIL output line
#   of `run-regression-suite-slice.sh` invoked against THIS cycle's touched
#   files. Required tokens:
#     (a) a literal line matching `<num>/<num> PASS` (or `<num>/<num> FAIL <ids>`)
#         appearing in build-report.md — this is the verbatim slice output
#     (b) a section header or label tying that line to the slice run
#         (look for `run-regression-suite-slice.sh` mentioned in the report
#         OR a "Regression Slice" / "Pre-handoff slice" section header).
# Evidence: intent.md:acceptance_checks bullet 6; triage-decision.md
#   "Recommendation to Builder" bullet 2.
# Author: tdd-engineer (cycle-91)
# Created: 2026-05-20
# Acceptance-of: build-report.md row "build-report.md self-attestation:
#   verbatim slice output included"
#
# Behavioral: a mutant that runs the slice but forgets to paste the output
# into build-report.md fails. A mutant that pastes the output but doesn't
# label it (so future operators can't tell what the line is) fails. A
# mutant that hard-codes a fake "1/1 PASS" string without actually running
# the script fails AC-005 above (because the slice is then never exercised
# against the prior-broken predicates); this predicate is paired with 005
# to close the falsification loop.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
CYCLE="${EVOLVE_CYCLE_NUMBER:-91}"

# Build-report is written to the runs directory (artifact pattern from profile:
# .evolve/runs/cycle-{cycle}/build-report.md).
REPORT_CANDIDATES="
$REPO_ROOT/.evolve/runs/cycle-${CYCLE}/build-report.md
/Users/danleemh/ai/claude/evolve-loop/.evolve/runs/cycle-${CYCLE}/build-report.md
${EVOLVE_WORKSPACE:-}/build-report.md
"
AC_ID="cycle-91-006-build-report-slice-attestation"

REPORT=""
for cand in $REPORT_CANDIDATES; do
  if [ -n "$cand" ] && [ -f "$cand" ]; then
    REPORT="$cand"
    break
  fi
done

if [ -z "$REPORT" ]; then
  echo "RED $AC_ID: build-report.md not found at any expected path:" >&2
  for cand in $REPORT_CANDIDATES; do
    [ -n "$cand" ] && echo "    $cand" >&2
  done
  exit 1
fi

# (a) Verbatim slice output line: `<num>/<num> PASS` or `<num>/<num> FAIL <ids>`.
#     We require PASS (this cycle's touched files are predicate-reachable but
#     the prior remediation is complete, so the slice MUST be PASS).
slice_line=$(grep -E '^[[:space:]]*[0-9]+/[0-9]+ (PASS|FAIL)' "$REPORT" | head -1)
if [ -z "$slice_line" ]; then
  echo "RED $AC_ID: build-report.md does not contain a verbatim slice output line ('N/M PASS' or 'N/M FAIL')" >&2
  echo "  inspected: $REPORT" >&2
  exit 1
fi

# Must be PASS (this cycle's touched files are remediation-complete).
if ! printf '%s' "$slice_line" | grep -qE 'PASS'; then
  echo "RED $AC_ID: slice line is FAIL — Builder shipped without remediating: $slice_line" >&2
  exit 1
fi

# (b) Slice run is labelled — either the script name appears in the report,
#     or a "Regression Slice" / "Pre-handoff Slice" section header is present.
labelled=0
if grep -qF 'run-regression-suite-slice.sh' "$REPORT"; then
  labelled=1
fi
if grep -qiE '(regression[- ]slice|pre-handoff[- ]slice|predicate[- ]slice)' "$REPORT"; then
  labelled=1
fi

if [ "$labelled" -ne 1 ]; then
  echo "RED $AC_ID: slice output appears in build-report.md but is not labelled (no 'run-regression-suite-slice.sh' nor 'Regression Slice'/'Pre-handoff Slice' header)" >&2
  exit 1
fi

echo "GREEN $AC_ID: build-report.md contains verbatim slice output line and identifies the slice run"
echo "  slice line: $slice_line"
exit 0
