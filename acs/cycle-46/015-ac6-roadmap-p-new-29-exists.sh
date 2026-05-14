#!/usr/bin/env bash
# AC6: token-reduction-roadmap.md contains P-NEW-29 entry
# predicate: grep for P-NEW-29 in roadmap
# metadata: cycle=46 task=T3b ac=AC6 risk=low

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROADMAP="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$ROADMAP" ] || { echo "FAIL: $ROADMAP missing"; exit 1; }
[[ $(cat "$ROADMAP") =~ P-NEW-29 ]] || { echo "FAIL: P-NEW-29 not found in $ROADMAP"; exit 1; }
echo "PASS: token-reduction-roadmap.md contains P-NEW-29 entry"
exit 0
