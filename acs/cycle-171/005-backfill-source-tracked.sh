#!/usr/bin/env bash
# ACS cycle-171 T3 — backfill package source exists AND is git-tracked.
# cycle-93 dual-check: [ -f ] alone passes for a gitignored worktree file that
# would be silently dropped at ship. Both disk-presence and git-tracking required.
set -uo pipefail
top=$(git rev-parse --show-toplevel)
rel="go/internal/backfill/backfill.go"
[ -f "$top/$rel" ] || { echo "RED: $rel missing on disk" >&2; exit 1; }
git -C "$top" ls-files --error-unmatch "$rel" >/dev/null 2>&1 \
  || { echo "RED: $rel untracked — may be gitignored or unstaged" >&2; exit 1; }
echo "GREEN: $rel present on disk and git-tracked" >&2
