#!/usr/bin/env bash
# ACS cycle-210 / Task-1 AC2+AC3+AC4 — KB article carries >=3 external
# citations, classifies candidates (adopt/adapt/reject), and contains NO
# phase-removal language.
#
# AC4 is the NEGATIVE axis (the strongest anti-no-op signal): a stub KB file
# that merely lists 5 headings would pass AC1 but must still be REJECTED if it
# suggests deleting an existing phase. Three load-bearing checks below.
#
# RED at baseline (file absent). After Builder: GREEN only when citations and
# classification are present AND removal language is absent.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

FILE="knowledge-base/research/missing-development-phases-2026-06.md"
[ -f "$FILE" ] || { echo "RED: $FILE missing on disk" >&2; exit 1; }

# AC2 — >=3 external source citations.
CITATIONS=$(grep -Eic "https://|arxiv|github\.com|source:|vendor doc" "$FILE" || true)
if [ "$CITATIONS" -lt 3 ]; then
  echo "RED: only $CITATIONS external citations, expected >= 3" >&2
  exit 1
fi

# AC3 — adopt/adapt/reject classification present.
if ! grep -Eqi "adopt|adapt|reject" "$FILE"; then
  echo "RED: no adopt/adapt/reject classification language found" >&2
  exit 1
fi

# AC4 (NEGATIVE) — must NOT propose removing existing phases.
if grep -Eqi "remove.*phase|delete.*phase|replace.*(scout|audit|build|ship)" "$FILE"; then
  echo "RED: document suggests removing/replacing an existing phase (additive-only required)" >&2
  exit 1
fi

echo "GREEN: $CITATIONS citations, classification present, no removal language" >&2
exit 0
