#!/usr/bin/env bash
# AC1: scripts/tests/abnormal-event-capture-test.sh exits 0
# predicate: the abnormal-event capture test suite must pass cleanly
# metadata: cycle=46 task=T1a ac=AC1 risk=low

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TEST="$REPO_ROOT/scripts/tests/abnormal-event-capture-test.sh"
[ -f "$TEST" ] || { echo "FAIL: $TEST missing"; exit 1; }
[ -x "$TEST" ] || { echo "FAIL: $TEST not executable"; exit 1; }
bash "$TEST" >/dev/null 2>&1 || { echo "FAIL: $TEST exited non-zero"; exit 1; }
echo "PASS: abnormal-event-capture-test.sh exits 0"
exit 0
