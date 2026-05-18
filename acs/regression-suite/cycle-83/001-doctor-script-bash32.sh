#!/usr/bin/env bash
# AC1: doctor-subscription-auth.sh exists and is bash-3.2 safe.
set -uo pipefail
ROOT="${WORKTREE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
SCRIPT="$ROOT/scripts/utility/doctor-subscription-auth.sh"
[ -f "$SCRIPT" ] || { echo "RED AC1: $SCRIPT missing"; exit 1; }
if grep -nE 'declare -A|mapfile|readarray|sed -i '\'''\''|date -d|\$\{[a-zA-Z_]+\^\^\}|\$\{[a-zA-Z_]+,,\}' "$SCRIPT" >/dev/null; then
    echo "RED AC1: bash-3.2 violation in $SCRIPT"
    exit 1
fi
/bin/bash "$SCRIPT" --json >/dev/null 2>&1 || { echo "RED AC1: failed under /bin/bash"; exit 1; }
echo "GREEN AC1: doctor exists; bash-3.2 safe; runs under /bin/bash 3.2.57"
exit 0
