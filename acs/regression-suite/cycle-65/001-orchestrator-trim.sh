#!/usr/bin/env bash
# AC-ID:         cycle-65-001
# Description:   Orchestrator persona size reduced by >20%
# Evidence:      agents/evolve-orchestrator.md
# Author:        builder
# Created:       2026-05-15T22:00:00Z
# Acceptance-of: build-report.md AC#1

set -euo pipefail

# Previous size was 35604 bytes.
# 20% reduction = 35604 * 0.8 = 28483 bytes.
# Current size should be < 28483.

NEW_SIZE=$(wc -c < agents/evolve-orchestrator.md | tr -d ' ')

if [ "$NEW_SIZE" -lt 28483 ]; then
    echo "PASS: New size $NEW_SIZE is less than 28483 (threshold for 20% reduction)"
    exit 0
else
    echo "FAIL: New size $NEW_SIZE is not less than 28483"
    exit 1
fi
