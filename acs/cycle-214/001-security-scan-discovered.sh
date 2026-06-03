#!/usr/bin/env bash
# ACS cycle-214 — security-scan user phase is discovered, valid, and listed.
#
# BEHAVIORAL: invokes the `evolve` binary so the real DiscoverUserSpecs →
# ValidateUserSpec → Merge → list pipeline runs. `OK <name>` and SOURCE=user
# only appear when a genuine, optional, kind:llm PhaseSpec is on disk — adding a
# magic string to a source file cannot satisfy this.
set -uo pipefail

ROOT="$(git rev-parse --show-toplevel)"
BIN="${EVOLVE_GO_BIN:-$ROOT/go/bin/evolve}"
REL=".evolve/phases/security-scan/phase.json"

[ -x "$BIN" ] || { echo "RED: evolve binary missing at $BIN"; exit 1; }

# Dual-check: on disk AND git-tracked (cycle-92: gitignored worktree files pass
# [ -f ] but vanish at ship).
[ -f "$ROOT/$REL" ] || { echo "RED: $REL missing on disk"; exit 1; }
git -C "$ROOT" ls-files --error-unmatch "$REL" >/dev/null 2>&1 \
  || { echo "RED: $REL untracked — may be gitignored / dropped at ship"; exit 1; }

# Behavioral: validator accepts it as a real user PhaseSpec.
if ! EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases validate security-scan 2>/dev/null \
   | grep -q "^OK    security-scan$"; then
  echo "RED: 'evolve phases validate security-scan' did not report OK"; exit 1
fi

# Behavioral: discovered + merged as a USER phase (not registry-edited).
if ! EVOLVE_PROJECT_ROOT="$ROOT" "$BIN" phases list 2>/dev/null \
   | grep -E "^security-scan[[:space:]]" | grep -q "user"; then
  echo "RED: security-scan not listed with SOURCE=user"; exit 1
fi

echo "GREEN: security-scan discovered, valid, listed as user phase"
exit 0
