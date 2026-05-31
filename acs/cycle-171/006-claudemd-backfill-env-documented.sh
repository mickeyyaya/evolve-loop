#!/usr/bin/env bash
# acs-predicate: config-check — doc-presence of an env-var row is inherently a
# grep; there is no system-under-test to invoke for "the docs mention X". Waived
# per the Predicate Quality config-check exemption; Auditor reviews waiver validity.
# ACS cycle-171 T3 AC-7 — EVOLVE_BACKFILL_ENABLED documented in CLAUDE.md.
set -uo pipefail
top=$(git rev-parse --show-toplevel)
grep -q "EVOLVE_BACKFILL_ENABLED" "$top/CLAUDE.md" \
  || { echo "RED: EVOLVE_BACKFILL_ENABLED not documented in CLAUDE.md env-var table" >&2; exit 1; }
echo "GREEN: EVOLVE_BACKFILL_ENABLED documented in CLAUDE.md" >&2
