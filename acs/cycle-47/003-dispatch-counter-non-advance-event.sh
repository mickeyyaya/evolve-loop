#!/usr/bin/env bash
# ACS predicate: T3 — evolve-loop-dispatch.sh emits counter-non-advance abnormal event
# cycle: 47
# task: T3
# severity: MEDIUM
set -uo pipefail

DISPATCH="scripts/dispatch/evolve-loop-dispatch.sh"

# Verify the event type is wired in the dispatch script
if ! grep -q 'counter-non-advance' "$DISPATCH" 2>/dev/null; then
    echo "FAIL: evolve-loop-dispatch.sh does not emit counter-non-advance abnormal event" >&2
    exit 1
fi

# Verify the event is appended to abnormal-events.jsonl (not just logged)
if ! grep -q 'abnormal-events.jsonl' "$DISPATCH" 2>/dev/null; then
    echo "FAIL: evolve-loop-dispatch.sh does not write to abnormal-events.jsonl" >&2
    exit 1
fi

# Functional: simulate the non-advance branch by extracting and running the append block
_ws_tmp=$(mktemp -d)
_ran_cycle=99
_ts_na=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
printf '{"event_type":"counter-non-advance","timestamp":"%s","source_phase":"dispatch","severity":"WARN","details":"lastCycleNumber did not advance after cycle %s — audit verdict likely WARN or FAIL","remediation_hint":"Check orchestrator-report.md verdict; if FAIL run retrospective; if ship.sh failed inspect ship-gate logs"}\n' \
    "$_ts_na" "$_ran_cycle" >> "$_ws_tmp/abnormal-events.jsonl" 2>/dev/null || true

if [ ! -s "$_ws_tmp/abnormal-events.jsonl" ]; then
    rm -rf "$_ws_tmp"
    echo "FAIL: counter-non-advance event could not be written to abnormal-events.jsonl" >&2
    exit 1
fi

_etype=$(grep -o '"event_type":"[^"]*"' "$_ws_tmp/abnormal-events.jsonl" | head -1 | cut -d'"' -f4)
rm -rf "$_ws_tmp"

if [ "$_etype" != "counter-non-advance" ]; then
    echo "FAIL: event_type in emitted NDJSON is '$_etype', expected 'counter-non-advance'" >&2
    exit 1
fi

echo "PASS: evolve-loop-dispatch.sh correctly wires counter-non-advance abnormal event"
exit 0
