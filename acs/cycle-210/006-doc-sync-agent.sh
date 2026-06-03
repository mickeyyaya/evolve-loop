#!/usr/bin/env bash
# ACS cycle-210 / Task-3 AC1 — doc-sync agent prompt exists, git-TRACKED, has
# >=3 sections, and actually describes documentation generation/sync.
#
# Lexical-diversity note: this predicate greps for documentation vocabulary
# (distinct from predicate 003's spec-verification vocabulary) so the two
# agent checks cannot both be satisfied by one generic stub.
#
# RED at baseline (file absent / untracked); GREEN once Builder writes AND
# stages agents/evolve-doc-sync.md.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

FILE="agents/evolve-doc-sync.md"

[ -f "$FILE" ] || { echo "RED: $FILE missing on disk" >&2; exit 1; }
git ls-files --error-unmatch "$FILE" >/dev/null 2>&1 \
  || { echo "RED: $FILE untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }

SECTIONS=$(grep -c "^## " "$FILE" || true)
if [ "$SECTIONS" -lt 3 ]; then
  echo "RED: $FILE has only $SECTIONS sections, expected >= 3" >&2
  exit 1
fi

if ! grep -Eqi "documentation|changelog|doc.*generat|api.*doc|readme" "$FILE"; then
  echo "RED: $FILE does not describe documentation generation" >&2
  exit 1
fi

echo "GREEN: $FILE tracked, $SECTIONS sections, describes documentation" >&2
exit 0
