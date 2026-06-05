#!/usr/bin/env bash
# ACS cycle-217 / Task-1 AC1+AC2 — fault-localization and reproduce-bug exist
# as USER phases and pass `evolve phases validate` (exit 0).
#
# Behavioral: invokes the real evolve binary (DiscoverUserSpecs → Merge →
# ValidateUserSpec) as a subprocess; asserts on its EXIT CODE. A magic string
# in any markdown file cannot make this pass — only valid phase specs can.
#
# DUAL CHECK (cycle-92/209 lesson): each deliverable file must be git-tracked,
# not just present on disk; untracked files are silently dropped at ship.
#
# RED at baseline (phase dirs absent); GREEN once Builder ships + stages both.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

BIN="${EVOLVE_GO_BIN:-$TOP/go/bin/evolve}"
[ -x "$BIN" ] || BIN="$TOP/go/evolve"
[ -x "$BIN" ] || { echo "RED: evolve binary not found (go/bin/evolve or go/evolve)" >&2; exit 1; }

for p in fault-localization reproduce-bug; do
  # Behavioral core: the validator must accept the spec.
  if ! EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases validate "$p" >/dev/null 2>&1; then
    echo "RED: evolve phases validate $p exits non-zero" >&2
    exit 1
  fi
  # Registered through the user-phase path (zero-Go ADR-0035), not the builtin registry.
  if ! EVOLVE_PROJECT_ROOT="$TOP" "$BIN" phases list 2>/dev/null | grep -E "^${p}[[:space:]]" | grep -q "user"; then
    echo "RED: $p not listed with SOURCE=user in evolve phases list" >&2
    exit 1
  fi
  # Dual-check: both deliverable files tracked.
  for base in phase.json agent.md; do
    f=".evolve/phases/$p/$base"
    [ -f "$f" ] || { echo "RED: $f missing on disk" >&2; exit 1; }
    git ls-files --error-unmatch "$f" >/dev/null 2>&1 \
      || { echo "RED: $f untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }
  done
done

echo "GREEN: fault-localization + reproduce-bug validate as tracked user phases" >&2
exit 0
