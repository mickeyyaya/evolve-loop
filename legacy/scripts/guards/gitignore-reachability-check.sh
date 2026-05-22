#!/usr/bin/env bash
# Pre-Builder-handoff gate: verify that deliverable paths are reachable (not gitignored).
#
# Usage: bash scripts/guards/gitignore-reachability-check.sh <path> [<path>...]
#
# Exits 0  — all paths pass (git would accept them; none are gitignored).
# Exits 1  — at least one path is gitignored; names offender on stderr.
#
# Prevents the cycle-92 failure mode where a deliverable matched .gitignore
# and was silently dropped at ship. Both [ -f ] AND git ls-files --error-unmatch
# must pass for a deliverable to be considered "reachable" (per cycle-93+ dual-check
# rule). This guard focuses on the gitignore half; callers should separately
# verify file existence.
#
# Bash 3.2 compatible. No GNU-only flags. Atomic: no writes to the tree.
set -uo pipefail

if [ "$#" -eq 0 ]; then
  echo "Usage: $0 <path> [<path>...]" >&2
  exit 1
fi

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "gitignore-reachability-check: not inside a git work tree" >&2
  exit 1
fi

fail=0

for target_path in "$@"; do
  # git check-ignore exits 0 when the path IS ignored, non-zero when it is NOT.
  # We want to flag paths that ARE ignored, so a 0 exit code is the failure case.
  if git check-ignore -q "$target_path" 2>/dev/null; then
    # Path is gitignored — emit the offending path to stderr for operator visibility.
    ignore_rule=$(git check-ignore -v "$target_path" 2>/dev/null || echo "(rule unknown)")
    echo "gitignore-reachability-check: BLOCKED: $target_path is matched by .gitignore rule: $ignore_rule" >&2
    fail=1
  fi
done

if [ "$fail" -ne 0 ]; then
  exit 1
fi

exit 0
