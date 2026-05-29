#!/usr/bin/env bash
# AC3: subagent-run.sh advisory event is gated by _EVOLVE_AUTH_MODE_LOGGED
#      and writes to abnormal-events.jsonl (NOT ledger.jsonl).
set -uo pipefail
ROOT="${WORKTREE:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)}"
F="$ROOT/scripts/dispatch/subagent-run.sh"
[ -f "$F" ] || { echo "RED AC3: $F missing"; exit 1; }

# Gating present
grep -q '_EVOLVE_AUTH_MODE_LOGGED' "$F" || { echo "RED AC3.1: gate variable missing"; exit 1; }
grep -q 'export _EVOLVE_AUTH_MODE_LOGGED' "$F" || { echo "RED AC3.2: gate not exported"; exit 1; }

# Block writes to abnormal-events.jsonl, not ledger.jsonl
awk '/v10.15.0:.*subscription-auth-mode/,/^fi$/' "$F" > /tmp/c83-block.$$
grep -q 'abnormal-events.jsonl' /tmp/c83-block.$$ || { echo "RED AC3.3: block does not target abnormal-events.jsonl"; rm -f /tmp/c83-block.$$; exit 1; }
if grep -q 'ledger.jsonl' /tmp/c83-block.$$; then
    # only allowed in comment referring to hash-chain preservation
    if grep -v '^[[:space:]]*#' /tmp/c83-block.$$ | grep -q 'ledger.jsonl'; then
        echo "RED AC3.4: non-comment line references ledger.jsonl in advisory block"
        rm -f /tmp/c83-block.$$
        exit 1
    fi
fi
rm -f /tmp/c83-block.$$

# Event payload uses event_type subscription-auth-mode
grep -q '"event_type":"subscription-auth-mode"' "$F" || { echo "RED AC3.5: event_type missing"; exit 1; }

echo "GREEN AC3: gated by _EVOLVE_AUTH_MODE_LOGGED; target=abnormal-events.jsonl; ledger.jsonl untouched"
exit 0
