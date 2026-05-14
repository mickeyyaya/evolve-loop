#!/usr/bin/env bash
# AC-ID:         cycle-45-002
# Description:   evolve-builder.md STOP CRITERION contains turn-budget-respected gate
# Evidence:      agents/evolve-builder.md:turn-budget-respected
# Author:        builder
# Created:       2026-05-14T07:15:00Z
# Acceptance-of: build-report.md T2 — Harden Builder STOP CRITERION with Turn-Count Gate
set -uo pipefail

BUILDER_MD="agents/evolve-builder.md"

if [ ! -f "$BUILDER_MD" ]; then
  echo "ERROR: $BUILDER_MD not found" >&2
  exit 1
fi

count=$(grep -c "turn-budget-respected" "$BUILDER_MD" 2>/dev/null || echo 0)

if [ "$count" -ge 1 ]; then
  echo "PASS: turn-budget-respected found ($count occurrence(s))"
  exit 0
else
  echo "FAIL: turn-budget-respected not found in $BUILDER_MD" >&2
  exit 1
fi
