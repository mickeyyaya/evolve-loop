#!/usr/bin/env bash
# AC-ID: cycle-93-003-builder-attestation-step
# AC-source: cycle-93/intent.md AC-3
# Behavioral predicate: agents/evolve-builder.md must document a
# pre-handoff `git ls-files --error-unmatch` attestation that BLOCKS when
# any delivered file is untracked.
#
# Rationale: cycle-92 had a regression-slice run in the worktree but no
# git-tracking attestation. The slice's `[ -f ]` checks passed for the
# gitignored AGENTS.md, so Builder declared "done" and the file was
# silently dropped at ship. The attestation closes that loophole.
#
# Bash 3.2 compatible. Two checks:
#   1. The profile mentions `git ls-files --error-unmatch`
#   2. Context near the match references a "Pre-handoff" / "Attestation"
#      section heading so this isn't a stray mention in unrelated prose.
#
# Exit codes:
#   0 = GREEN
#   1 = RED
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TARGET="agents/evolve-builder.md"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET missing on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1; then
  echo "RED: $TARGET exists but is not git-tracked" >&2
  exit 1
fi

# Check 1: literal command appears
if ! grep -q 'git ls-files --error-unmatch' "$TARGET"; then
  echo "RED: $TARGET does not document 'git ls-files --error-unmatch'" >&2
  exit 1
fi

# Check 2: the mention occurs near an attestation-/handoff-shaped section
# heading. We scan headings (#, ##, ###) for one of:
#   - "Attestation"
#   - "Git Tracking"
#   - "Pre-handoff" (case-insensitive)
# AND verify the literal command sits within 60 lines after such a heading.
nlines=$(wc -l < "$TARGET" | tr -d ' ')

heading_line=$(grep -n -i -E '^#+ .*(attestation|git tracking|pre[- ]?handoff)' "$TARGET" \
  | head -n 1 | cut -d: -f1)
cmd_line=$(grep -n 'git ls-files --error-unmatch' "$TARGET" \
  | head -n 1 | cut -d: -f1)

if [ -z "$heading_line" ]; then
  echo "RED: no Attestation/Git-Tracking/Pre-handoff section heading in $TARGET" >&2
  exit 1
fi
if [ -z "$cmd_line" ]; then
  echo "RED: no git ls-files command line resolved in $TARGET" >&2
  exit 1
fi

# Allow cmd_line to be at or after heading_line; reject reversed order.
if [ "$cmd_line" -lt "$heading_line" ]; then
  echo "RED: ls-files command at L$cmd_line precedes attestation heading at L$heading_line" >&2
  exit 1
fi

# Distance check: must be within 60 source lines.
dist=$(( cmd_line - heading_line ))
if [ "$dist" -gt 60 ]; then
  echo "RED: attestation heading (L$heading_line) too far from ls-files command (L$cmd_line); dist=$dist (>60)" >&2
  exit 1
fi

echo "GREEN: $TARGET documents git ls-files --error-unmatch attestation (heading L$heading_line, cmd L$cmd_line, dist=$dist, file len=$nlines)"
exit 0
