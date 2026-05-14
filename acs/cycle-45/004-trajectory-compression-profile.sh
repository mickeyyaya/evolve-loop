#!/usr/bin/env bash
# AC-ID:         cycle-45-004
# Description:   builder.json has context_compact_expired_tool_results=true (P-NEW-21)
# Evidence:      .evolve/profiles/builder.json:context_compact_expired_tool_results
# Author:        builder
# Created:       2026-05-14T07:15:00Z
# Acceptance-of: build-report.md T3 — P-NEW-21 trajectory compression profile fields
set -uo pipefail

PROFILE=".evolve/profiles/builder.json"

if [ ! -f "$PROFILE" ]; then
  echo "ERROR: $PROFILE not found" >&2
  exit 1
fi

if grep -q '"context_compact_expired_tool_results": true' "$PROFILE"; then
  echo "PASS: context_compact_expired_tool_results=true present"
  exit 0
else
  echo "FAIL: context_compact_expired_tool_results not set to true in $PROFILE" >&2
  exit 1
fi
