#!/usr/bin/env bash
# AC-ID: cycle-91-002-builder-pre-handoff-instruction
# Description: Verifies that agents/evolve-builder.md was updated to mandate
#   the pre-handoff `run-regression-suite-slice.sh` invocation and the
#   verbatim inclusion of its output in build-report.md. The three literal
#   phrases that MUST coexist:
#     (a) `run-regression-suite-slice.sh` — the canonical script name
#     (b) `before writing build-report` — the timing anchor
#     (c) `verbatim` (in proximity to build-report) — the inclusion contract
# Evidence: intent.md:acceptance_checks bullet 2; intent.md:interfaces bullet 2.
# Author: tdd-engineer (cycle-91)
# Created: 2026-05-20
# Acceptance-of: build-report.md row "agents/evolve-builder.md updated with
#   pre-handoff slice instruction"
#
# Behavioral: greps for the three required literal tokens. A mutant that
# only mentions the script in passing (without the timing + verbatim
# contract) fails. A mutant that adds the script reference only to the
# reference doc (evolve-builder-reference.md) and not the canonical
# persona doc fails because this predicate checks the canonical file.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
PERSONA="$REPO_ROOT/agents/evolve-builder.md"
AC_ID="cycle-91-002-builder-pre-handoff-instruction"

if [ ! -f "$PERSONA" ]; then
  echo "RED $AC_ID: agents/evolve-builder.md not found at $PERSONA" >&2
  exit 1
fi

missing=""

# (a) Literal script name
if ! grep -qF 'run-regression-suite-slice.sh' "$PERSONA"; then
  missing="${missing} run-regression-suite-slice.sh"
fi

# (b) Timing anchor — case-insensitive to tolerate "Before writing build-report"
if ! grep -qiE 'before writing build-report' "$PERSONA"; then
  missing="${missing} 'before writing build-report'"
fi

# (c) Verbatim-inclusion contract — require both "verbatim" and "build-report"
#     to appear, ideally close together. We assert at least one line in the
#     file contains BOTH tokens, OR the literal phrase "include … verbatim … in build-report"
#     (allowing for prose variation).
if ! awk '
  /verbatim/ && /build-report/ { found = 1; exit }
  END { exit (found ? 0 : 1) }
' "$PERSONA"; then
  # Try a broader paragraph-level proximity check (verbatim + build-report
  # within a 6-line window).
  if ! awk '
    {
      buf[NR % 6] = $0
      have_verbatim = 0
      have_report = 0
      for (i = 0; i < 6; i++) {
        if (index(buf[i], "verbatim") > 0) have_verbatim = 1
        if (index(buf[i], "build-report") > 0) have_report = 1
      }
      if (have_verbatim && have_report) { found = 1; exit }
    }
    END { exit (found ? 0 : 1) }
  ' "$PERSONA"; then
    missing="${missing} verbatim-near-build-report-contract"
  fi
fi

if [ -n "$missing" ]; then
  echo "RED $AC_ID: agents/evolve-builder.md missing required tokens:${missing}" >&2
  exit 1
fi

echo "GREEN $AC_ID: agents/evolve-builder.md mandates pre-handoff slice run with verbatim build-report inclusion"
exit 0
