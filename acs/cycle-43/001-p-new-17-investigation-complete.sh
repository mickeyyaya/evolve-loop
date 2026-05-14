#!/usr/bin/env bash
# ACS predicate: cycle 43, AC-1
# Verify P-NEW-17 status is INVESTIGATION-COMPLETE in token-reduction-roadmap.md
#
# metadata:
#   cycle: 43
#   id: 001
#   slug: p-new-17-investigation-complete
#   acceptance_criterion: "docs/architecture/token-reduction-roadmap.md P-NEW-17 status changed to INVESTIGATION-COMPLETE with evidence"

set -uo pipefail

ROADMAP="docs/architecture/token-reduction-roadmap.md"
OUTPUT_DIR=".evolve/runs/cycle-43/acs-output"
mkdir -p "$OUTPUT_DIR"

if [ ! -f "$ROADMAP" ]; then
    echo "FAIL: $ROADMAP not found" | tee "$OUTPUT_DIR/001-result.txt"
    exit 1
fi

if grep -q "INVESTIGATION-COMPLETE" "$ROADMAP"; then
    echo "PASS: P-NEW-17 status contains INVESTIGATION-COMPLETE" | tee "$OUTPUT_DIR/001-result.txt"
    exit 0
else
    echo "FAIL: P-NEW-17 status does not contain INVESTIGATION-COMPLETE in $ROADMAP" | tee "$OUTPUT_DIR/001-result.txt"
    exit 1
fi
