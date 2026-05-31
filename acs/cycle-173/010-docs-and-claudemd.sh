#!/usr/bin/env bash
# AC-ID: cycle-173-010-docs-and-claudemd
# AC-source: scout-report.md T2 "docs/architecture/phase-timing-and-diagnostics.md
#   exists with >=2 mentions of duration_ms and >=2 of failure-diag" + "CLAUDE.md
#   contains phase-timing.json + failure-diag.json entries" + F-4 "the doc was
#   never created; CLAUDE.md lacks the rows."
#
# acs-predicate: config-check — this is a DOCUMENTATION-CONTENT check. The
# "system under test" is a markdown artifact with no runnable behavior, so
# grep-of-file-content is the correct (waived) predicate shape per the predicate-
# quality SKILL. The new doc is also dual-checked (disk + git-tracked) because a
# gitignored/untracked doc is silently dropped at ship (cycle-92 class).
#
# Exit: 0 = GREEN, 1 = RED. Bash 3.2 compatible.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
[ -n "$ROOT" ] || { echo "RED: not in a git work tree" >&2; exit 1; }
cd "$ROOT" || { echo "RED: cd failed" >&2; exit 1; }

DOC="docs/architecture/phase-timing-and-diagnostics.md"

# Dual-check: the new doc must exist on disk AND be git-tracked (else dropped at ship).
if [ ! -f "$DOC" ]; then
  echo "RED: $DOC missing on disk (F-4: never created in cycles 171/172)" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$DOC" >/dev/null 2>&1; then
  echo "RED: $DOC is untracked — run 'git add' so it survives ship (gitignore/staging drop)" >&2
  exit 1
fi

dm="$(grep -c 'duration_ms' "$DOC")"
fd="$(grep -c 'failure-diag' "$DOC")"
if [ "$dm" -lt 2 ]; then
  echo "RED: $DOC mentions duration_ms $dm time(s), want >=2" >&2
  exit 1
fi
if [ "$fd" -lt 2 ]; then
  echo "RED: $DOC mentions failure-diag $fd time(s), want >=2" >&2
  exit 1
fi

# CLAUDE.md must document both observability outputs by filename.
if ! grep -q 'phase-timing.json' CLAUDE.md; then
  echo "RED: CLAUDE.md is missing a 'phase-timing.json' entry" >&2
  exit 1
fi
if ! grep -q 'failure-diag.json' CLAUDE.md; then
  echo "RED: CLAUDE.md is missing a 'failure-diag.json' entry" >&2
  exit 1
fi

echo "PASS: $DOC present+tracked (duration_ms x$dm, failure-diag x$fd); CLAUDE.md documents both outputs"
exit 0
