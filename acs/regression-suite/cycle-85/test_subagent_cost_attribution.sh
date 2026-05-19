#!/usr/bin/env bash
# ACS predicate — cycle 85
# Verifies that subagent-run.sh accumulates per-invocation cost into state.json.
set -uo pipefail

SCRIPT="${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}/scripts/dispatch/subagent-run.sh"

[ -f "$SCRIPT" ] || { echo "FAIL: subagent-run.sh not found at $SCRIPT" >&2; exit 1; }

if ! grep -qF 'cost-attribution' "$SCRIPT"; then
    echo "FAIL: subagent-run.sh missing cost-attribution accumulation block" >&2
    exit 1
fi

if ! grep -qF 'cycleAccruedCostUSD' "$SCRIPT"; then
    echo "FAIL: subagent-run.sh missing cycleAccruedCostUSD accumulation" >&2
    exit 1
fi

if ! grep -qF 'total_cost_usd' "$SCRIPT"; then
    echo "FAIL: subagent-run.sh missing total_cost_usd read from usage sidecar" >&2
    exit 1
fi

echo "PASS: subagent-run.sh contains per-invocation cost attribution accumulation"
exit 0
