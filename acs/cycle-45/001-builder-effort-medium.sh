#!/usr/bin/env bash
# AC-ID:         cycle-45-001
# Description:   builder.json effort_level is "medium" (not "high")
# Evidence:      .evolve/profiles/builder.json:effort_level
# Author:        builder
# Created:       2026-05-14T07:15:00Z
# Acceptance-of: build-report.md T1 — Fix Builder effort_level high→medium
set -uo pipefail

PROFILE=".evolve/profiles/builder.json"

if [ ! -f "$PROFILE" ]; then
  echo "ERROR: $PROFILE not found" >&2
  exit 1
fi

effort=$(grep '"effort_level"' "$PROFILE" | tr -d ' ",' | cut -d: -f2)

if [ "$effort" = "medium" ]; then
  echo "PASS: effort_level=$effort"
  exit 0
else
  echo "FAIL: effort_level=$effort (expected medium)" >&2
  exit 1
fi
