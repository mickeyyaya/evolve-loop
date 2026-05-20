#!/usr/bin/env bash
# AC-ID: cycle-93-002-profiles-agents-md-git-tracked
# AC-source: cycle-93/intent.md AC-2
# Behavioral predicate: .evolve/profiles/AGENTS.md must (a) exist on disk
# AND (b) be tracked by git. Filesystem existence alone is the cycle-92
# failure mode: the file was present in the worktree but gitignored, so it
# never reached main.
#
# Two-step check (combine, do NOT short-circuit):
#   1. [ -f PATH ]                      — disk presence
#   2. git ls-files --error-unmatch PATH — git tracking
# Both must succeed.
#
# Bash 3.2 compatible. RED until Builder fixes .gitignore AND creates
# the file AND `git add`s it.
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

TARGET=".evolve/profiles/AGENTS.md"

if [ ! -f "$TARGET" ]; then
  echo "RED: $TARGET does not exist on disk" >&2
  exit 1
fi

if ! git ls-files --error-unmatch "$TARGET" >/dev/null 2>&1; then
  echo "RED: $TARGET exists on disk but is NOT tracked by git" >&2
  echo "      (gitignored-deliverable-survives-worktree-not-ship pattern)" >&2
  git check-ignore -v "$TARGET" >&2 2>/dev/null || true
  exit 1
fi

# Sanity: non-empty content. A zero-byte tracked file is a tombstone-ish
# artifact and indicates Builder did not actually fill the schema doc in.
size=$(wc -c < "$TARGET" | tr -d ' ')
if [ "$size" -lt 64 ]; then
  echo "RED: $TARGET is suspiciously small ($size bytes)" >&2
  exit 1
fi

echo "GREEN: $TARGET exists on disk, git-tracked, $size bytes"
exit 0
