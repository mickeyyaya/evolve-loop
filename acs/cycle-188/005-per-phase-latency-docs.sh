#!/usr/bin/env bash
# acs-predicate: config-check
# ACS cycle-188 — Task 2 AC6 (DOC, waived config-check): the per-phase latency
# ceiling override is documented. CLAUDE.md's env-var table must carry a
# second `_LATENCY_CEILING_S` entry beyond the global row (the per-phase
# variant), and phase-timing-and-diagnostics.md must mention LATENCY_CEILING.
# Doc-presence is inherently a grep; the behavioral half is covered by 003.
set -uo pipefail
TOP="$(git rev-parse --show-toplevel)"
claude="$TOP/CLAUDE.md"
doc="$TOP/docs/architecture/phase-timing-and-diagnostics.md"

count=$(grep -o "_LATENCY_CEILING_S" "$claude" | wc -l | tr -d ' ')
if [ "$count" -lt 2 ]; then
  echo "RED: CLAUDE.md has $count _LATENCY_CEILING_S refs (<2): per-phase override not documented" >&2
  exit 1
fi
if ! grep -q "LATENCY_CEILING" "$doc"; then
  echo "RED: phase-timing-and-diagnostics.md does not document the per-phase LATENCY_CEILING override" >&2
  exit 1
fi
echo "PASS: per-phase latency ceiling documented in CLAUDE.md + phase-timing-and-diagnostics.md"
exit 0
