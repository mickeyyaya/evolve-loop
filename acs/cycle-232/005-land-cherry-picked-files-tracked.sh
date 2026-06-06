#!/usr/bin/env bash
# acs-predicate: config-check — inherently a file-presence/tracking check
#                (the cherry-picked artifacts themselves; behavior is covered
#                by predicates 001-004).
# AC-ID:         cycle-232-t1-AC7
# Description:   cherry-picked cycle-230 ACS predicates + eval files are tracked (dual-check: disk + git index)
# Evidence:      acs/cycle-230/*.sh + .evolve/evals/*.md (from 201f7cb)
# Author:        tdd-engineer
# Created:       2026-06-06T00:00:00Z
# Acceptance-of: scout-report.md Task 1 (land-audited-resolution-fix) AC7

set -uo pipefail

ROOT="${EVOLVE_WORKTREE_PATH:-${WORKTREE_PATH:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
cd "$ROOT" || { echo "RED: cannot cd to $ROOT" >&2; exit 1; }

FILES="
acs/cycle-230/001-auditor-doc-trim.sh
acs/cycle-230/002-phase-naming-lint.sh
acs/cycle-230/003-acs-suite-root-autosolve.sh
acs/cycle-230/004-ledger-skip-source.sh
.evolve/evals/acs-suite-root-autosolve.md
.evolve/evals/ledger-skip-source.md
.evolve/evals/phase-naming-lint.md
.evolve/evals/auditor-doc-trim.md
"

rc=0
for f in $FILES; do
  # Check 1: disk presence.
  [ -f "$f" ] || { echo "RED: $f missing on disk" >&2; rc=1; continue; }
  # Check 2: git tracking — catches gitignore silencing (.evolve/* is ignored;
  # cherry-pick stages these via the index, but a re-create without -f drops
  # them at ship — the cycle-92 defect mode).
  git ls-files --error-unmatch "$f" >/dev/null 2>&1 \
    || { echo "RED: $f untracked — gitignore may silently drop it at ship" >&2; rc=1; }
done

[ "$rc" -eq 0 ] && echo "GREEN: all 8 cherry-picked artifacts on disk AND tracked" >&2
exit "$rc"
