#!/usr/bin/env bash
# ACS predicate: T4 — claude.sh emits cost-overrun abnormal event on budget-exceeded exit
# cycle: 47
# task: T4
# severity: MEDIUM
set -uo pipefail

CLAUDE_ADAPTER="scripts/cli_adapters/claude.sh"

# Verify the event type is wired
if ! grep -q 'cost-overrun' "$CLAUDE_ADAPTER" 2>/dev/null; then
    echo "FAIL: claude.sh does not emit cost-overrun abnormal event" >&2
    exit 1
fi

# Verify it checks for the budget-exceeded signal
if ! grep -q 'error_max_budget_usd' "$CLAUDE_ADAPTER" 2>/dev/null; then
    echo "FAIL: claude.sh does not check for error_max_budget_usd in stdout log" >&2
    exit 1
fi

# Verify the event writes to abnormal-events.jsonl
if ! grep -q 'abnormal-events.jsonl' "$CLAUDE_ADAPTER" 2>/dev/null; then
    echo "FAIL: claude.sh does not write to abnormal-events.jsonl" >&2
    exit 1
fi

# Verify the event only fires on non-zero exit (guard: EXIT_CODE -ne 0)
if ! grep -A3 'cost-overrun' "$CLAUDE_ADAPTER" 2>/dev/null | grep -q 'EXIT_CODE'; then
    # Check if the guard is nearby (within the enclosing if block)
    if ! grep -B5 'error_max_budget_usd' "$CLAUDE_ADAPTER" 2>/dev/null | grep -q 'EXIT_CODE.*-ne 0'; then
        echo "FAIL: cost-overrun event is not guarded by EXIT_CODE -ne 0 check" >&2
        exit 1
    fi
fi

echo "PASS: claude.sh correctly wires cost-overrun abnormal event on budget-exceeded exit"
exit 0
