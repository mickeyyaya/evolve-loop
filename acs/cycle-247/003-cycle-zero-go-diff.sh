#!/usr/bin/env bash
# ACS cycle-247 — recover-wave2-phases AC5 + phases-release-and-memory AC6.
# Behavioral via git subprocess: both cycle-247 tasks are declared zero-Go
# (config-only). Asserts no *.go file differs between the pre-cycle baseline
# (f01a323, scout-report "Last commit") and HEAD. One predicate serves both
# tasks because the criterion is identical and cycle-scoped (baseline..HEAD
# spans every cycle-247 commit).
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

BASELINE_SHA="f01a323"

git cat-file -e "$BASELINE_SHA" 2>/dev/null \
  || { echo "RED: baseline $BASELINE_SHA not in object store — cannot attest zero-Go" >&2; exit 1; }

go_changes=$(git diff --name-only "$BASELINE_SHA"..HEAD -- 'go' | grep '\.go$' || true)
if [ -n "$go_changes" ]; then
  echo "RED: Go sources changed since $BASELINE_SHA (cycle declared config-only):" >&2
  echo "$go_changes" >&2
  exit 1
fi
echo "GREEN: zero .go changes since $BASELINE_SHA" >&2
exit 0
