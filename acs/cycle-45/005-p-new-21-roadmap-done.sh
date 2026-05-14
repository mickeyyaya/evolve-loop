#!/usr/bin/env bash
# AC-ID:         cycle-45-005
# Description:   token-reduction-roadmap.md marks P-NEW-21 as DONE (cycle 45)
# Evidence:      docs/architecture/token-reduction-roadmap.md:P-NEW-21
# Author:        builder
# Created:       2026-05-14T07:15:00Z
# Acceptance-of: build-report.md T3 — P-NEW-21 roadmap status update
set -uo pipefail

ROADMAP="docs/architecture/token-reduction-roadmap.md"

if [ ! -f "$ROADMAP" ]; then
  echo "ERROR: $ROADMAP not found" >&2
  exit 1
fi

if grep -q "P-NEW-21.*DONE (cycle 45)" "$ROADMAP"; then
  echo "PASS: P-NEW-21 marked DONE (cycle 45) in roadmap"
  exit 0
else
  echo "FAIL: P-NEW-21 not marked DONE (cycle 45) in $ROADMAP" >&2
  grep "P-NEW-21" "$ROADMAP" | head -3 >&2
  exit 1
fi
