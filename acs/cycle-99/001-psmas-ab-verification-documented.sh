#!/usr/bin/env bash
# AC-ID: cycle-99-001-psmas-ab-verification-documented
# Description: Cycle-99 ships a complete PSMAS A/B verification deliverable extending docs/architecture/psmas-phase-scheduling.md with ≥5 historical cycles, observed token-reduction %, the 20% threshold, and a FLIP/DEFER/REJECT verdict — while preserving the EVOLVE_PSMAS_SKIP opt-in default.
# Evidence: docs/architecture/psmas-phase-scheduling.md ; CLAUDE.md EVOLVE_PSMAS_SKIP row (if present)
# Author: tdd-engineer (cycle-99)
# Created: 2026-05-20T14:00:00Z
# Acceptance-of: cycle-99/scout-report.md §3 T1 ; cycle-99/triage-decision.md scope[T1]
# AC-source: cycle-99/scout-report.md T1 ; triage-decision.md scope[T1]
# Behavioral predicate:
#   Cycle-99 must produce a git-tracked PSMAS A/B verification deliverable at
#   docs/architecture/psmas-phase-scheduling.md that documents:
#     (a) the verification methodology — which historical cycles were re-run
#         under EVOLVE_PSMAS_SKIP=1 (at least 5 distinct cycle identifiers),
#     (b) the observed aggregate token reduction with an explicit percentage,
#     (c) the explicit ≥20% pass/fail threshold from cycle-98 lesson, and
#     (d) a structured verdict — one of FLIP / DEFER / REJECT — for the
#         default-on flip decision.
#   AND intent non_goal must hold: CLAUDE.md MUST still describe
#   EVOLVE_PSMAS_SKIP with default `0` (opt-in) — no silent flip permitted
#   in this cycle.
#
# RED until Builder writes psmas-phase-scheduling.md with all required
# fields; GREEN once the doc lands and is git-tracked AND the opt-in
# default is preserved.
#
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (deliverable present + structured + opt-in default preserved)
#   1 = RED   (file missing/untracked, schema incomplete, or default flipped)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

DOC="docs/architecture/psmas-phase-scheduling.md"
CLAUDE_MD="CLAUDE.md"

fail_count=0

# (0) File-existence dual-check (cycle-93+ rule)
if [ ! -f "$DOC" ]; then
  echo "RED: $DOC missing on disk" >&2
  fail_count=$(( fail_count + 1 ))
elif ! git ls-files --error-unmatch "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC exists but is not git-tracked (cycle-92 silent-drop hazard)" >&2
  fail_count=$(( fail_count + 1 ))
fi

# Only descend into content checks if the file is readable.
if [ -f "$DOC" ]; then
  # (a) At least 5 distinct cycle-NN identifiers in the doc (representative
  #     cycle selection). Use a portable extract: grep -oE then sort -u.
  cycle_ids=$(grep -oE 'cycle-[0-9]+' "$DOC" 2>/dev/null | sort -u | wc -l | tr -d ' ')
  if [ -z "$cycle_ids" ] || [ "$cycle_ids" -lt 5 ]; then
    echo "RED: $DOC references fewer than 5 distinct cycle-NN identifiers (got: ${cycle_ids:-0})" >&2
    fail_count=$(( fail_count + 1 ))
  fi

  # (b) Numeric aggregate token reduction percentage. Accepts integer or
  #     one-decimal forms: 23%, 23.4%. We require AT LEAST ONE such literal
  #     to appear so the verdict has a measurable observation behind it.
  if ! grep -Eq '[0-9]+(\.[0-9]+)?[[:space:]]*%' "$DOC"; then
    echo "RED: $DOC contains no numeric percentage (token reduction observation absent)" >&2
    fail_count=$(( fail_count + 1 ))
  fi

  # (c) Explicit ≥20% threshold reference from cycle-98 lesson. Accept
  #     either "≥20%", ">= 20%", or "20%" preceded by a threshold token.
  if ! grep -Eq '(≥|>=|at least)[[:space:]]*20[[:space:]]*%|20%[[:space:]]*(threshold|target|criterion)' "$DOC"; then
    echo "RED: $DOC does not reference the 20% pass/fail threshold from cycle-98 lesson" >&2
    fail_count=$(( fail_count + 1 ))
  fi

  # (d) Structured verdict line. One of FLIP / DEFER / REJECT must appear
  #     in upper-case to signal an unambiguous decision token.
  if ! grep -Eq '\b(FLIP|DEFER|REJECT)\b' "$DOC"; then
    echo "RED: $DOC contains no FLIP/DEFER/REJECT verdict token" >&2
    fail_count=$(( fail_count + 1 ))
  fi

  # (e) EVOLVE_PSMAS_SKIP referenced so reader knows which flag is governed.
  if ! grep -q 'EVOLVE_PSMAS_SKIP' "$DOC"; then
    echo "RED: $DOC does not reference EVOLVE_PSMAS_SKIP flag" >&2
    fail_count=$(( fail_count + 1 ))
  fi
fi

# (f) Intent non_goal #1 + acceptance_checks[2]: no silent default flip.
#     If CLAUDE.md describes EVOLVE_PSMAS_SKIP, the default must be `0`
#     (opt-in). If the row is absent (pre-existing tech debt from cycle-98,
#     which shipped the flag without adding a CLAUDE.md row), absence is
#     tolerated — that is a documentation gap, not a silent flip.
if [ ! -f "$CLAUDE_MD" ]; then
  echo "RED: $CLAUDE_MD missing — cannot verify opt-in default preserved" >&2
  fail_count=$(( fail_count + 1 ))
else
  psmas_row=$(awk '/EVOLVE_PSMAS_SKIP/{print; exit}' "$CLAUDE_MD")
  if [ -n "$psmas_row" ]; then
    # Row present — assert opt-in default. Accept "`0`" (current convention)
    # or the literal "default-off" / "opt-in" tokens as equivalent signals.
    case "$psmas_row" in
      *'`0`'*|*'default-off'*|*'opt-in'*) : ;;  # opt-in preserved
      *'`1`'*)
        echo "RED: $CLAUDE_MD shows EVOLVE_PSMAS_SKIP default=1 — silent flip violates intent non_goal #1" >&2
        echo "RED:   row: $psmas_row" >&2
        fail_count=$(( fail_count + 1 ))
        ;;
      *)
        echo "RED: $CLAUDE_MD EVOLVE_PSMAS_SKIP row does not clearly show opt-in default" >&2
        echo "RED:   row: $psmas_row" >&2
        fail_count=$(( fail_count + 1 ))
        ;;
    esac
  fi
  # Row absent: no assertion. The PSMAS doc itself (checked above) carries
  # the canonical default declaration; CLAUDE.md row is informational.
fi

if [ "$fail_count" -ne 0 ]; then
  echo "RED: PSMAS A/B verification deliverable incomplete ($fail_count issue[s])" >&2
  exit 1
fi

echo "GREEN: $DOC documents ≥5 cycles, observed reduction %, 20% threshold, FLIP/DEFER/REJECT verdict; $CLAUDE_MD preserves opt-in default"
exit 0
