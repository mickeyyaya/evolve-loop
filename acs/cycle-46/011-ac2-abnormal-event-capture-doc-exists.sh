#!/usr/bin/env bash
# AC2: docs/architecture/abnormal-event-capture.md exists and has >=200 chars
# predicate: documentation file present and substantive
# metadata: cycle=47 task=T1b ac=AC2 risk=low

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
DOC="$REPO_ROOT/docs/architecture/abnormal-event-capture.md"
[ -f "$DOC" ] || { echo "FAIL: $DOC missing"; exit 1; }
LEN=$(wc -c < "$DOC" | tr -d ' ')
[ "$LEN" -ge 200 ] || { echo "FAIL: $DOC too short ($LEN chars, need >=200)"; exit 1; }
echo "PASS: abnormal-event-capture.md exists ($LEN chars)"
exit 0
