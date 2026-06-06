#!/usr/bin/env bash
# ACS cycle-217 / Task-2 AC1 — behavior-baseline, behavior-compare and
# smell-scan exist as USER phases and pass `evolve phases validate` (exit 0).
#
# Behavioral: invokes the real evolve binary as a subprocess and asserts on
# its EXIT CODE (DiscoverUserSpecs → Merge → ValidateUserSpec).
# DUAL CHECK (cycle-92/209 lesson): every deliverable file must be git-tracked.
#
# RED at baseline (phase dirs absent); GREEN once Builder ships + stages all 3.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

BIN="${EVOLVE_GO_BIN:-$TOP/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$TOP/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary not found (go/bin/evolve or go/evolve)" >&2; exit 1; }

for p in behavior-baseline behavior-compare smell-scan; do
  if ! EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases validate "$p" >/dev/null 2>&1; then
    echo "RED: evolve phases validate $p exits non-zero" >&2
    exit 1
  fi
  if ! EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases list 2>/dev/null | grep -E "^${p}[[:space:]]" | grep -q "user"; then
    echo "RED: $p not listed with SOURCE=user in evolve phases list" >&2
    exit 1
  fi
  for base in phase.json agent.md; do
    f=".evolve/phases/$p/$base"
    [ -f "$f" ] || { echo "RED: $f missing on disk" >&2; exit 1; }
    git ls-files --error-unmatch "$f" >/dev/null 2>&1 \
      || { echo "RED: $f untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }
  done
done

echo "GREEN: refactor trio (behavior-baseline/behavior-compare/smell-scan) validate as tracked user phases" >&2
exit 0
