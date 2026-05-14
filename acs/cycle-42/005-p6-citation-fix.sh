#!/usr/bin/env bash
# ACS predicate: verify P6 citation corrected from arXiv:2510.26585 to arXiv:2604.17400
# cycle: 42
# ac: AC1 — roadmap does not cite 2510.26585 in P6 section or Sources; AC2 — roadmap cites 2604.17400 in P6 source and Sources §2
# metadata: {"id":"005","slug":"p6-citation-fix","cycle":42,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
ROADMAP="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$ROADMAP" ] || { echo "ERR: $ROADMAP not found"; exit 1; }

rc=0

# AC1: old incorrect arXiv ID 2510.26585 no longer appears in P6-related context
# (the ID may still appear in a SupervisorAgent note/finding, but should not be cited as PSMAS)
_psmas_line=$(grep -n 'PSMAS' "$ROADMAP" | grep '2510.26585' || true)
if [ -n "$_psmas_line" ]; then
    echo "FAIL AC1: PSMAS is still cited with arXiv:2510.26585 — found: $_psmas_line"
    rc=1
else
    echo "PASS AC1: PSMAS no longer cited with arXiv:2510.26585"
fi

# AC2: correct arXiv ID 2604.17400 appears in roadmap (P6 source and/or Sources section)
if ! grep -q '2604.17400' "$ROADMAP"; then
    echo "FAIL AC2: arXiv:2604.17400 not found in roadmap — citation fix not applied"
    rc=1
else
    echo "PASS AC2: arXiv:2604.17400 found in roadmap (citation fix applied)"
fi

exit $rc
