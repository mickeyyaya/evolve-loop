#!/usr/bin/env bash
# acs-predicate: config-check
# ACS cycle-188 — Task 1 AC6 (DOC, waived config-check): ADR-0026 Stage 1 #5
# marked done and self-healing-gaps.md records the new gap-closure. Both files
# must reference the shipped ledger Kind `stop_review` — the literal that proves
# the trail is real, not just prose. Doc-presence is inherently a grep; the
# behavioral half of this feature is covered by predicates 001 + 002.
set -uo pipefail
TOP="$(git rev-parse --show-toplevel)"
adr="$TOP/docs/architecture/adr/0026-self-healing-review-layer.md"
gaps="$TOP/docs/architecture/self-healing-gaps.md"

if ! grep -q "stop_review" "$adr"; then
  echo "RED: ADR-0026 does not reference the shipped 'stop_review' ledger Kind (Stage 1 #5 not closed)" >&2
  exit 1
fi
if ! grep -q "stop_review" "$gaps"; then
  echo "RED: self-healing-gaps.md does not record the stop_review ledger-trail closure" >&2
  exit 1
fi
echo "PASS: ADR-0026 Stage 1 #5 + self-healing-gaps.md reference stop_review"
exit 0
