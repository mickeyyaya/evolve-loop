#!/usr/bin/env bash
# AC3: reconcile-carryover-todos.sh contains abnormal-events.jsonl handling
# predicate: grep for the promotion code in reconcile-carryover-todos.sh
# metadata: cycle=47 task=T1c ac=AC3 risk=low

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT="$REPO_ROOT/scripts/lifecycle/reconcile-carryover-todos.sh"
[ -f "$SCRIPT" ] || { echo "FAIL: $SCRIPT missing"; exit 1; }
[[ $(cat "$SCRIPT") =~ abnormal-events.jsonl ]] || { echo "FAIL: reconcile-carryover-todos.sh has no abnormal-events.jsonl handling"; exit 1; }
echo "PASS: reconcile-carryover-todos.sh contains abnormal-events.jsonl handling"
exit 0
