#!/usr/bin/env bash
# AC-ID:         cycle-45-003
# Description:   evolve-builder.md contains Trajectory Compression section (P-NEW-21)
# Evidence:      agents/evolve-builder.md:Tool-Result Trajectory Compression
# Author:        builder
# Created:       2026-05-14T07:15:00Z
# Acceptance-of: build-report.md T3 — P-NEW-21 trajectory compression persona section
set -uo pipefail

BUILDER_MD="agents/evolve-builder.md"

if [ ! -f "$BUILDER_MD" ]; then
  echo "ERROR: $BUILDER_MD not found" >&2
  exit 1
fi

count=$(grep -c "Trajectory Compression" "$BUILDER_MD" 2>/dev/null || echo 0)

if [ "$count" -ge 1 ]; then
  echo "PASS: Trajectory Compression section found ($count occurrence(s))"
  exit 0
else
  echo "FAIL: 'Trajectory Compression' not found in $BUILDER_MD" >&2
  exit 1
fi
