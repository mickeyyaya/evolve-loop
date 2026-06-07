#!/usr/bin/env bash
# acs-predicate: config-check
# ACS cycle-247 — recover-wave2-phases AC1 + AC3 + AC4.
# Inherently a file-presence/tracking check (the deliverable IS config files);
# uses the mandatory dual-check pattern (cycle-93+): [ -f ] alone passes for
# gitignored worktree files that ship would silently drop (cycle-92 mode), so
# git ls-files --error-unmatch is the load-bearing guard.
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
for p in benchmark-gate fuzz-probe cleanup-sweep rollback-plan; do
  check ".evolve/phases/$p/phase.json" || rc=1   # AC1
  check ".evolve/phases/$p/agent.md"   || rc=1   # AC1
  check ".evolve/profiles/$p.json"     || rc=1   # AC3
  check "agents/evolve-$p.md"          || rc=1   # AC3
done
# AC4 — cherry-pick completeness: cycle-246 suites + eval ride along on aea56ca.
check "acs/cycle-246/001-wave2-phases-validate.sh"   || rc=1
check "acs/cycle-246/002-wave2-artifacts-tracked.sh" || rc=1
check "acs/cycle-246/003-wave2-content-contracts.sh" || rc=1
check "tests/test-phases-quality-gates.sh"           || rc=1
check ".evolve/evals/phases-quality-gates.md"        || rc=1

[ "$rc" -eq 0 ] && echo "GREEN: all wave-2 artifacts present AND git-tracked" >&2
exit "$rc"
