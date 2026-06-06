#!/usr/bin/env bash
# ACS cycle-242/003 — POSTHOC templates reference REAL runner artifact names
# (cycle-241 audit MEDIUM: templates said builder-usage.json; the runner
# emits build-usage.json / phase-timing.json).
#
# acs-predicate: config-check
# Waiver rationale: the criterion IS document content-parity — the system
# under test is the template text itself; there is no subprocess to invoke.
# Content-parity form only (cycle-242 goal): never commit-presence.
set -uo pipefail

TOP=$(git rev-parse --show-toplevel)
REF="$TOP/agents/evolve-builder-reference.md"
SCHEMA="$TOP/docs/architecture/posthoc-schema.md"

rc=0

for f in "$REF" "$SCHEMA"; do
  [ -f "$f" ] || { echo "RED: $f missing on disk" >&2; rc=1; continue; }
  if grep -q 'builder-usage\.json\|builder-timing\.json' "$f"; then
    echo "RED: ${f##*/} still references builder-usage.json/builder-timing.json" >&2
    rc=1
  fi
  if ! grep -q 'build-usage\.json' "$f"; then
    echo "RED: ${f##*/} lacks build-usage.json — corrected name absent (line deleted?)" >&2
    rc=1
  fi
  if ! grep -q 'POSTHOC: jq' "$f"; then
    echo "RED: ${f##*/} lost its POSTHOC jq sentinel" >&2
    rc=1
  fi
done

[ "$rc" -eq 0 ] && { echo "PASS"; exit 0; }
exit 1
