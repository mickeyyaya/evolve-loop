#!/bin/bash
# Assert Scout STOP CRITERION was tightened per C73 calibration
set -uo pipefail
SCOUT_MD="agents/evolve-scout.md"
grep -q "turn 10" "$SCOUT_MD" && grep -q "turn 7" "$SCOUT_MD" && grep -q "turn 5" "$SCOUT_MD"
