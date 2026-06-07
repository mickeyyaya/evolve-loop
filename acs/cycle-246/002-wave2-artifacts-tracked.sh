#!/usr/bin/env bash
# ACS cycle-246/002 — AC5 + AC6: per Wave-2 phase, all four artifacts exist,
# are non-empty, AND are git-tracked (cycle-92 gitignore footgun: [ -f ] alone
# passes in the worktree but the file is silently dropped at ship if
# untracked; .evolve/phases has a known gitignore shadow → needs git add -f).
# Profiles must additionally parse as JSON with a matching .name.
# acs-predicate: config-check (artifact presence/tracking IS the deliverable)
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"

for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  for rel in ".evolve/phases/$p/phase.json" ".evolve/phases/$p/agent.md" \
             "agents/evolve-$p.md" ".evolve/profiles/$p.json"; do
    [ -s "$ROOT/$rel" ] \
      || { echo "RED: $rel missing or empty on disk" >&2; exit 1; }
    git -C "$ROOT" ls-files --error-unmatch "$rel" >/dev/null 2>&1 \
      || { echo "RED: $rel untracked — may be gitignored/dropped at ship" >&2; exit 1; }
  done
  jq -e --arg n "$p" '.name == $n' "$ROOT/.evolve/profiles/$p.json" >/dev/null 2>&1 \
    || { echo "RED: profiles/$p.json not valid JSON with name == $p" >&2; exit 1; }
done

echo "PASS"; exit 0
