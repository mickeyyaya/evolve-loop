#!/usr/bin/env bash
# ACS predicate: verify 'tester' is present in both subagent-run.sh allowlists
# cycle: 41
# ac: AC1 — line-457 regex includes tester; AC2 — line-1066 parallel-dispatch regex includes tester; AC4 — line-18 usage comment includes tester
# metadata: {"id":"001","slug":"tester-allowlist","cycle":41,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
SCRIPT="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
[ -f "$SCRIPT" ] || { echo "ERR: $SCRIPT not found"; exit 1; }

rc=0

# AC1: main allowlist regex includes tester
if grep -q "tester" "$SCRIPT" | grep -q "^(scout\|.*tester" 2>/dev/null; then
    :
fi
if ! grep -E '^\s+\[\[ "\$agent_role" =~ \^\(' "$SCRIPT" | grep -q "tester"; then
    echo "FAIL AC1: cmd_run agent_role regex does not include 'tester'"
    rc=1
else
    echo "PASS AC1: cmd_run agent_role regex includes 'tester'"
fi

# AC2: parallel-dispatch allowlist regex includes tester
if ! grep -E '^\s+\[\[ "\$agent" =~ \^\(' "$SCRIPT" | grep -q "tester"; then
    echo "FAIL AC2: cmd_dispatch_parallel agent regex does not include 'tester'"
    rc=1
else
    echo "PASS AC2: cmd_dispatch_parallel agent regex includes 'tester'"
fi

# AC4: usage comment line 18 area includes tester
if ! grep -E '^#\s+<agent>' "$SCRIPT" | grep -q "tester"; then
    echo "FAIL AC4: usage comment does not list 'tester' in agent list"
    rc=1
else
    echo "PASS AC4: usage comment lists 'tester'"
fi

exit $rc
