#!/usr/bin/env bash
# ACS predicate: verify P-NEW-13 roadmap status is DONE (cycle 42)
# cycle: 42
# ac: AC1 — status table row for P-NEW-13 contains "DONE (cycle 42)"; AC2 — P-NEW-13 field table target cycle updated to DONE
# metadata: {"id":"003","slug":"p-new-13-done","cycle":42,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
ROADMAP="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$ROADMAP" ] || { echo "ERR: $ROADMAP not found"; exit 1; }

rc=0

# AC1: status table row for P-NEW-13 contains "DONE (cycle 42)"
if ! grep -q 'P-NEW-13.*DONE (cycle 42)' "$ROADMAP"; then
    echo "FAIL AC1: status table row for P-NEW-13 does not contain 'DONE (cycle 42)'"
    rc=1
else
    echo "PASS AC1: status table row for P-NEW-13 contains 'DONE (cycle 42)'"
fi

# AC2: P-NEW-13 field table 'Target cycle' updated to DONE (not 43+)
if grep -q 'Target cycle.*43+' "$ROADMAP"; then
    echo "FAIL AC2: P-NEW-13 field table Target cycle still shows '43+' — should be DONE (cycle 42)"
    rc=1
else
    echo "PASS AC2: P-NEW-13 field table Target cycle no longer shows '43+' (updated to DONE)"
fi

exit $rc
