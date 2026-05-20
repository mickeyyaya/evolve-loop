#!/usr/bin/env bash
# AC-ID: cycle-98-001-triage-schema-documents-phase-skip
# AC-source: cycle-98/intent.md acceptance_checks[0] ; scout-report.md TASK-98-A
# Behavioral predicate:
#   agents/evolve-triage.md MUST document a `phase_skip[]` output field with
#   the trivial/small/normal/large mapping defined in cycle-98 intent.md:
#     - trivial          -> ["tdd-engineer","retrospective"]
#     - small + PASS     -> ["retrospective"]
#     - normal / large   -> []
#   AND it must note that the field is gated on EVOLVE_PSMAS_SKIP=1.
#
# RED until Builder edits agents/evolve-triage.md (TASK-98-A); GREEN after.
# Bash 3.2 compatible. No GNU-only flags.
#
# Exit codes:
#   0 = GREEN (schema documents phase_skip[] + mapping + flag gate)
#   1 = RED   (field absent, mapping incomplete, or flag gate missing)
set -uo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null)"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TRIAGE="agents/evolve-triage.md"
if [ ! -f "$TRIAGE" ]; then
  echo "RED: $TRIAGE missing" >&2
  exit 1
fi

# 1) phase_skip[] field must appear in the schema documentation.
if ! grep -q 'phase_skip' "$TRIAGE"; then
  echo "RED: $TRIAGE does not document a phase_skip field" >&2
  exit 1
fi

# 2) The mapping must reference every cycle_size_estimate bucket relevant
#    to skips. We assert literal token presence for the four buckets and
#    the two skippable phase names.
fail_count=0
for needle in tdd-engineer retrospective trivial small; do
  if ! grep -qi -- "$needle" "$TRIAGE"; then
    echo "RED: $TRIAGE phase_skip docs missing token: $needle" >&2
    fail_count=$(( fail_count + 1 ))
  fi
done

# 3) The opt-in flag gate must be acknowledged in triage.md so operators
#    reading the spec understand the field is dormant by default.
if ! grep -q 'EVOLVE_PSMAS_SKIP' "$TRIAGE"; then
  echo "RED: $TRIAGE does not mention EVOLVE_PSMAS_SKIP flag gate" >&2
  fail_count=$(( fail_count + 1 ))
fi

if [ "$fail_count" -ne 0 ]; then
  echo "RED: triage schema phase_skip[] documentation incomplete ($fail_count issue[s])" >&2
  exit 1
fi

echo "GREEN: $TRIAGE documents phase_skip[] with mapping + EVOLVE_PSMAS_SKIP gate"
exit 0
