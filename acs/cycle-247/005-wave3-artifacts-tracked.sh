#!/usr/bin/env bash
# acs-predicate: config-check
# ACS cycle-247 — phases-release-and-memory AC1 + AC5.
# File-presence/tracking dual-check (cycle-93+ rule): disk presence AND
# git ls-files tracking for every wave-3 artifact. The eval file lives under
# gitignored .evolve/evals/ and REQUIRES git add -f (issue-#11 note in
# .gitignore) — this predicate is what catches a forgotten -f.
set -uo pipefail

ROOT=$(git rev-parse --show-toplevel)
cd "$ROOT"

check() {
  local path="$1"
  [ -f "$path" ] || { echo "RED: $path missing on disk" >&2; return 1; }
  git ls-files --error-unmatch "$path" >/dev/null 2>&1 \
    || { echo "RED: $path untracked — gitignore shadow? needs git add -f" >&2; return 1; }
  return 0
}

rc=0
for p in changelog-sync post-ship-monitor api-contract-design context-condense; do
  check ".evolve/phases/$p/phase.json" || rc=1   # AC1
  check ".evolve/phases/$p/agent.md"   || rc=1   # AC1
  check ".evolve/profiles/$p.json"     || rc=1   # AC5
  check "agents/evolve-$p.md"          || rc=1   # AC5
done
check ".evolve/evals/phases-release-and-memory.md" || rc=1
check ".evolve/evals/recover-wave2-phases.md"      || rc=1

[ "$rc" -eq 0 ] && echo "GREEN: all wave-3 artifacts present AND git-tracked" >&2
exit "$rc"
