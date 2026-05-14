#!/usr/bin/env bash
# ACS predicate: verify P-NEW-16 roadmap status is DONE (cycle 42)
# cycle: 42
# ac: AC1 — status table row for P-NEW-16 contains "DONE (cycle 42)"
# metadata: {"id":"004","slug":"p-new-16-done","cycle":42,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
ROADMAP="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$ROADMAP" ] || { echo "ERR: $ROADMAP not found"; exit 1; }

rc=0

# AC1: status table row for P-NEW-16 contains "DONE (cycle 42)"
if ! grep -q 'P-NEW-16.*DONE (cycle 42)' "$ROADMAP"; then
    echo "FAIL AC1: status table row for P-NEW-16 does not contain 'DONE (cycle 42)'"
    rc=1
else
    echo "PASS AC1: status table row for P-NEW-16 contains 'DONE (cycle 42)'"
fi

exit $rc
