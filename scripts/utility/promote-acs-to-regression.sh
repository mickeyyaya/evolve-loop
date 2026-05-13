#!/usr/bin/env bash
#
# promote-acs-to-regression.sh — Move shipped cycle's predicates into the
# accumulating regression-suite/.
#
# After a cycle ships successfully (acs-verdict.json == PASS), its predicates
# in acs/cycle-N/ become part of the canonical regression suite that all
# future cycles must keep GREEN. This script does the atomic move.
#
# Per EGPS contract: "every prior cycle's predicate must remain GREEN" —
# enforced by the next cycle's run-acs-suite.sh scanning acs/regression-suite/.
#
# Usage:
#   bash scripts/utility/promote-acs-to-regression.sh <cycle>
#   bash scripts/utility/promote-acs-to-regression.sh <cycle> --dry-run
#
# Exit codes:
#   0 — promoted (or no-op when already promoted)
#   1 — source acs/cycle-N/ missing or empty
#  10 — bad arguments

set -uo pipefail

CYCLE=""
DRY_RUN=0

while [ $# -gt 0 ]; do
    case "$1" in
        --dry-run) DRY_RUN=1 ;;
        --help|-h) sed -n '2,18p' "$0" | sed 's/^# \{0,1\}//'; exit 0 ;;
        --*) echo "[promote-acs] unknown flag: $1" >&2; exit 10 ;;
        *)
            [ -z "$CYCLE" ] && CYCLE="$1" || { echo "[promote-acs] too many args" >&2; exit 10; }
            ;;
    esac
    shift
done

[ -n "$CYCLE" ] || { echo "[promote-acs] usage: $0 <cycle> [--dry-run]" >&2; exit 10; }
[[ "$CYCLE" =~ ^[0-9]+$ ]] || { echo "[promote-acs] cycle must be integer" >&2; exit 10; }

PROJECT_ROOT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
SRC="${EVOLVE_ACS_DIR_OVERRIDE:-$PROJECT_ROOT/acs}/cycle-$CYCLE"
DST="${EVOLVE_ACS_DIR_OVERRIDE:-$PROJECT_ROOT/acs}/regression-suite/cycle-$CYCLE"

if [ ! -d "$SRC" ]; then
    echo "[promote-acs] no predicates to promote (source $SRC absent)"
    exit 1
fi

count=$(find "$SRC" -maxdepth 1 -name "*.sh" -type f 2>/dev/null | wc -l | tr -d ' ')
if [ "$count" = "0" ]; then
    echo "[promote-acs] no .sh predicates in $SRC"
    exit 1
fi

if [ -d "$DST" ]; then
    echo "[promote-acs] already promoted: $DST exists (idempotent no-op)"
    exit 0
fi

if [ "$DRY_RUN" = "1" ]; then
    echo "[promote-acs] DRY-RUN: would move $count predicate(s) from $SRC to $DST"
    exit 0
fi

mkdir -p "$(dirname "$DST")"
mv "$SRC" "$DST"
echo "[promote-acs] OK: promoted $count predicate(s) → $DST"
