#!/usr/bin/env bash
# ACS predicate: verify retrospective) case added to role-gate.sh (cycle 71)
# cycle: 71
# ac: AC4 — role-gate.sh contains retrospective) case; no regression in existing cases
# metadata: {"id":"001","slug":"role-gate-retrospective","cycle":71,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
GATE="$REPO_ROOT/scripts/guards/role-gate.sh"
[ -f "$GATE" ] || { echo "ERR: $GATE not found"; exit 1; }

rc=0

# AC4a: retrospective) case exists in role-gate.sh
if ! grep -q "retrospective)" "$GATE"; then
    echo "FAIL AC4a: retrospective) case not found in role-gate.sh"
    rc=1
else
    echo "PASS AC4a: retrospective) case present in role-gate.sh"
fi

# AC4b: learn) case still present (no regression)
if ! grep -q "learn)" "$GATE"; then
    echo "FAIL AC4b: learn) case missing from role-gate.sh (regression)"
    rc=1
else
    echo "PASS AC4b: learn) case still present (no regression)"
fi

# AC4c: retrospective) allows instincts/lessons/*.yaml writes
if ! grep -A5 "retrospective)" "$GATE" | grep -q "instincts/lessons"; then
    echo "FAIL AC4c: retrospective) case does not allow instincts/lessons/*.yaml"
    rc=1
else
    echo "PASS AC4c: retrospective) case allows instincts/lessons/*.yaml"
fi

exit $rc
