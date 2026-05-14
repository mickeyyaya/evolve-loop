#!/usr/bin/env bash
# ACS predicate: cycle 43, AC-3+4
# Verify P-NEW-18 and P-NEW-19 sections exist in token-reduction-roadmap.md
#
# metadata:
#   cycle: 43
#   id: 003
#   slug: p-new-18-and-19-exist
#   acceptance_criterion: "P-NEW-18 and P-NEW-19 sections exist in docs/architecture/token-reduction-roadmap.md"

set -uo pipefail

ROADMAP="docs/architecture/token-reduction-roadmap.md"
OUTPUT_DIR=".evolve/runs/cycle-43/acs-output"
mkdir -p "$OUTPUT_DIR"

if [ ! -f "$ROADMAP" ]; then
    echo "FAIL: $ROADMAP not found" | tee "$OUTPUT_DIR/003-result.txt"
    exit 1
fi

PASS=1

if ! grep -q "P-NEW-18" "$ROADMAP"; then
    echo "FAIL: P-NEW-18 section not found in $ROADMAP" | tee "$OUTPUT_DIR/003-result.txt"
    PASS=0
fi

if ! grep -q "P-NEW-19" "$ROADMAP"; then
    echo "FAIL: P-NEW-19 section not found in $ROADMAP" | tee "$OUTPUT_DIR/003-result.txt"
    PASS=0
fi

if [ "$PASS" = "1" ]; then
    echo "PASS: P-NEW-18 and P-NEW-19 sections both exist in roadmap" | tee "$OUTPUT_DIR/003-result.txt"
    exit 0
else
    exit 1
fi
