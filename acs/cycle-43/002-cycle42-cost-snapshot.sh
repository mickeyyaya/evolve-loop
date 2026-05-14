#!/usr/bin/env bash
# ACS predicate: cycle 43, AC-2
# Verify cycle-42 cost snapshot row ($5.80) exists in roadmap status table
#
# metadata:
#   cycle: 43
#   id: 002
#   slug: cycle42-cost-snapshot
#   acceptance_criterion: "Cycle-42 cost snapshot row ($5.80) exists in token-reduction-roadmap.md status section"

set -uo pipefail

ROADMAP="docs/architecture/token-reduction-roadmap.md"
OUTPUT_DIR=".evolve/runs/cycle-43/acs-output"
mkdir -p "$OUTPUT_DIR"

if [ ! -f "$ROADMAP" ]; then
    echo "FAIL: $ROADMAP not found" | tee "$OUTPUT_DIR/002-result.txt"
    exit 1
fi

if grep -q "Cycle-42 cost snapshot" "$ROADMAP" && grep -q '\$5\.80' "$ROADMAP"; then
    echo "PASS: cycle-42 cost snapshot (\$5.80) exists in roadmap" | tee "$OUTPUT_DIR/002-result.txt"
    exit 0
else
    echo "FAIL: cycle-42 cost snapshot or \$5.80 not found in $ROADMAP" | tee "$OUTPUT_DIR/002-result.txt"
    exit 1
fi
