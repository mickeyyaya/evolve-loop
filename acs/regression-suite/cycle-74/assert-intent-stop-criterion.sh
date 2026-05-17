#!/bin/bash
# Assert Intent STOP CRITERION was tightened per C74 calibration
# Baseline (pre-patch): grep exits 1 for both patterns — non-tautological
set -uo pipefail
INTENT_MD="agents/evolve-intent.md"
grep -q "Emergency Exit" "$INTENT_MD" && grep -q "Hard Stop" "$INTENT_MD"
