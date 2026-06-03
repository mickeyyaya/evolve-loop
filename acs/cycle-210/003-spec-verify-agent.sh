#!/usr/bin/env bash
# ACS cycle-210 / Task-2 AC1 — spec-verifier agent prompt exists, git-TRACKED,
# has >=3 sections, and actually describes acceptance-criteria / spec
# verification (not an empty stub).
#
# RED at baseline (file absent / untracked); GREEN once Builder writes AND
# stages agents/evolve-spec-verifier.md with the required content.
set -uo pipefail
TOP=$(git rev-parse --show-toplevel) || { echo "RED: not a git repo" >&2; exit 1; }
cd "$TOP" || { echo "RED: cannot cd to repo root" >&2; exit 1; }

FILE="agents/evolve-spec-verifier.md"

[ -f "$FILE" ] || { echo "RED: $FILE missing on disk" >&2; exit 1; }
git ls-files --error-unmatch "$FILE" >/dev/null 2>&1 \
  || { echo "RED: $FILE untracked — would be dropped at ship (cycle-209 mode)" >&2; exit 1; }

SECTIONS=$(grep -c "^## " "$FILE" || true)
if [ "$SECTIONS" -lt 3 ]; then
  echo "RED: $FILE has only $SECTIONS sections, expected >= 3" >&2
  exit 1
fi

if ! grep -Eqi "acceptance.criteria|spec.*verif|predicate.*coverage|verif.*spec" "$FILE"; then
  echo "RED: $FILE does not describe spec/acceptance-criteria verification" >&2
  exit 1
fi

echo "GREEN: $FILE tracked, $SECTIONS sections, describes spec verification" >&2
exit 0
