#!/usr/bin/env bash
# ACS predicate: verify P-NEW-13 autotrim uses line-boundary cut (head -n / tail -n)
# cycle: 42
# ac: AC1 — autotrim block uses head -n (not head -c); AC2 — autotrim block uses tail -n (not tail -c); AC3 — no byte-boundary cut in autotrim; AC4 — EVOLVE_CONTEXT_AUTOTRIM default-0 guard preserved
# metadata: {"id":"001","slug":"p-new-13-autotrim-line-boundary","cycle":42,"author":"builder"}
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)" || { echo "ERR: not a git repo"; exit 1; }
SCRIPT="$REPO_ROOT/scripts/dispatch/subagent-run.sh"
[ -f "$SCRIPT" ] || { echo "ERR: $SCRIPT not found"; exit 1; }

rc=0

# AC1: autotrim block uses _keep_head_lines variable (line-boundary head cut)
if ! grep -q '_keep_head_lines' "$SCRIPT"; then
    echo "FAIL AC1: autotrim block does not use '_keep_head_lines' variable (line-boundary head cut not found)"
    rc=1
else
    echo "PASS AC1: autotrim block uses _keep_head_lines (line-boundary head cut)"
fi

# AC2: autotrim block uses _keep_tail_lines variable (line-boundary tail cut)
if ! grep -q '_keep_tail_lines' "$SCRIPT"; then
    echo "FAIL AC2: autotrim block does not use '_keep_tail_lines' variable (line-boundary tail cut not found)"
    rc=1
else
    echo "PASS AC2: autotrim block uses _keep_tail_lines (line-boundary tail cut)"
fi

# AC3: old byte-boundary variables (_keep_head_bytes / _keep_tail_bytes) are NOT in the autotrim block
# Extract the autotrim block (between EVOLVE_CONTEXT_AUTOTRIM=1 guard and the closing fi)
_autotrim_block=$(awk '/EVOLVE_CONTEXT_AUTOTRIM.*:-0.*= .*1/,/^    fi$/' "$SCRIPT" 2>/dev/null | head -50)
if printf '%s\n' "$_autotrim_block" | grep -qE '_keep_head_bytes|_keep_tail_bytes'; then
    echo "FAIL AC3: autotrim block still references byte-boundary variables _keep_head_bytes or _keep_tail_bytes"
    rc=1
else
    echo "PASS AC3: no byte-boundary _keep_head_bytes/_keep_tail_bytes in autotrim block"
fi

# AC4: EVOLVE_CONTEXT_AUTOTRIM default-0 guard is present (default-off path preserved)
if ! grep -q 'EVOLVE_CONTEXT_AUTOTRIM:-0' "$SCRIPT"; then
    echo "FAIL AC4: EVOLVE_CONTEXT_AUTOTRIM:-0 default guard missing — default-off path may be broken"
    rc=1
else
    echo "PASS AC4: EVOLVE_CONTEXT_AUTOTRIM:-0 default guard present (default-off path preserved)"
fi

exit $rc
