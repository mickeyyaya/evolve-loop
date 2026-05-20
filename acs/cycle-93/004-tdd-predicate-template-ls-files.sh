#!/usr/bin/env bash
# AC-ID: cycle-93-004-tdd-predicate-template-ls-files
# AC-source: cycle-93/intent.md AC-4
# Behavioral predicate: agents/evolve-tdd-engineer.md must instruct that
# file-existence predicates combine `[ -f "$PATH" ]` with
# `git ls-files --error-unmatch "$PATH"`. The bare-disk check is what
# allowed cycle-92's gitignored-deliverable defect to slip past TDD.
#
# Two checks:
#   1. The literal command `git ls-files --error-unmatch` appears
#   2. The doc still shows the disk-presence idiom `[ -f` somewhere near
#      the predicate template / authoring rules (so the dual-check
#      intent is captured, not just a passing keyword)
#
# Bash 3.2 compatible.
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

TARGET="agents/evolve-tdd-engineer.md"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET missing on disk" >&2
  exit 1
fi
if ! git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1; then
  echo "RED: $TARGET exists but is not git-tracked" >&2
  exit 1
fi

# Check 1: literal command present
if ! grep -q 'git ls-files --error-unmatch' "$TARGET"; then
  echo "RED: $TARGET does not document 'git ls-files --error-unmatch' cross-check" >&2
  exit 1
fi

# Check 2: dual-check intent — the doc still references the bare-disk
# `[ -f` idiom somewhere. Regex covers `[ -f "$PATH"` and `[ -f $PATH`
# style references (single or double bracket).
if ! grep -Eq '\[\[? -f ' "$TARGET"; then
  echo "RED: $TARGET no longer mentions the [ -f \"\$PATH\" ] idiom; dual-check pairing not documented" >&2
  exit 1
fi

# Check 3: same paragraph/section coherence — ls-files command and `[ -f`
# mention occur within 30 lines of each other (suggests they belong to
# the same rule rather than being unrelated drive-by mentions).
ls_line=$(grep -n 'git ls-files --error-unmatch' "$TARGET" | head -n 1 | cut -d: -f1)
f_line=$(grep -En '\[\[? -f ' "$TARGET" | head -n 1 | cut -d: -f1)

if [ -z "$ls_line" ] || [ -z "$f_line" ]; then
  echo "RED: could not resolve line numbers for dual-check (ls_line='$ls_line', f_line='$f_line')" >&2
  exit 1
fi

# absolute distance
if [ "$ls_line" -gt "$f_line" ]; then
  dist=$(( ls_line - f_line ))
else
  dist=$(( f_line - ls_line ))
fi
if [ "$dist" -gt 30 ]; then
  echo "RED: ls-files (L$ls_line) and [-f (L$f_line) are $dist lines apart (>30) — dual-check rule not co-located" >&2
  exit 1
fi

echo "GREEN: $TARGET documents dual-check (ls-files L$ls_line, [ -f L$f_line, dist=$dist)"
exit 0
