#!/usr/bin/env bash
# acs-predicate: config-check — eval-file presence is an inherent file-existence
# check (cycle-131 lesson: missing .evolve/evals/<slug>.md = automatic CRITICAL
# FAIL at audit). Grep waiver per tdd-engineer predicate-quality classification.
# AC-ID:         cycle-10-014
# Description:   Persistent regression eval .evolve/evals/wave-product-discovery-tdd-and-phases.md present on disk AND git-tracked (dual-check per cycle-93 rule), carrying the two cycle-9 fixes (C7 config-check waiver + C8 Integration-only queued check)
# Evidence:      .evolve/evals/wave-product-discovery-tdd-and-phases.md
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Key Finding #3 — wave-product-discovery-tdd-and-phases

set -uo pipefail
WORKTREE="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$WORKTREE" || { echo "RED: cannot cd to $WORKTREE" >&2; exit 1; }

EVAL=".evolve/evals/wave-product-discovery-tdd-and-phases.md"

# Check 1: disk presence
[ -f "$EVAL" ] || { echo "RED: $EVAL missing on disk" >&2; exit 1; }

# Check 2: git tracking — catches gitignored worktree files (cycle-92 defect)
git ls-files --error-unmatch "$EVAL" >/dev/null 2>&1 \
  || { echo "RED: $EVAL untracked — may be gitignored or not staged" >&2; exit 1; }

# Cycle-9 fix pins (auxiliary): the two corrected criteria must be present.
grep -q 'acs-predicate: config-check' "$EVAL" \
  || { echo "RED: C7 grep waiver missing from eval (cycle-9 root cause)" >&2; exit 1; }
grep -q 'Integration row' "$EVAL" \
  || { echo "RED: C8 no longer checks the Integration row stays queued" >&2; exit 1; }
# Fixed-string match: the cycle-9 defect was a grep PATTERN literal `Ops.*queued`
# in C8 (Ops is done since cycle 5). Prose mentions of Ops are fine; the regex
# literal must be gone. -F so this pin cannot match across prose words.
if grep -qF 'Ops.*queued' "$EVAL"; then
  echo "RED: eval still greps Ops.*queued — Ops is done since cycle 5 (cycle-9 C8 defect)" >&2
  exit 1
fi

echo "GREEN: corrected Product eval file present and git-tracked" >&2
exit 0
