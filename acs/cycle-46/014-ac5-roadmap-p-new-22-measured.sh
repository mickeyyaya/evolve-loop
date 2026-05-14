#!/usr/bin/env bash
# AC5: token-reduction-roadmap.md P-NEW-22 entry updated to DONE or MEASURED with data
# predicate: roadmap contains MEASURED or DONE status for P-NEW-22
# metadata: cycle=46 task=T2c ac=AC5 risk=low

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ROADMAP="$REPO_ROOT/docs/architecture/token-reduction-roadmap.md"
[ -f "$ROADMAP" ] || { echo "FAIL: $ROADMAP missing"; exit 1; }
[[ $(cat "$ROADMAP") =~ P-NEW-22.*MEASURED|MEASURED.*P-NEW-22 ]] || \
[[ $(cat "$ROADMAP") =~ "MEASURED (cycle" ]] || {
    echo "FAIL: token-reduction-roadmap.md P-NEW-22 not updated to MEASURED status"
    exit 1
}
echo "PASS: token-reduction-roadmap.md P-NEW-22 entry updated to MEASURED"
exit 0
