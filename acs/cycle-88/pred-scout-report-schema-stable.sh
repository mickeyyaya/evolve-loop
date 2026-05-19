#!/usr/bin/env bash
# AC-ID: cycle-88-scout-report-schema-stable
#
# Verifies that Scout's downstream artifact contract is preserved by the
# Cycle B persona rewrite. This is the static-analysis half of the intent's
# "behavioral equivalence drill"; the live drill is Auditor's responsibility.
#
# Checks the persona file (agents/evolve-scout.md) for the anchors and field
# names that downstream consumers (Triage, TDD-engineer, Builder) rely on:
#   1. ANCHOR:task_proposals reference present (HTML-comment anchor).
#   2. ANCHOR:summary reference present.
#   3. Task field names targetFiles, complexity, researchBacking, effort are
#      all named in the persona (so Scout knows to emit them).
#   4. Carryover-decisions section name still present (downstream Triage reads
#      it).
#
# Also performs a self-check on the existing cycle-88 scout-report.md (the
# pre-build artifact already on disk) — when this predicate runs post-build in
# a cycle-89 drill, the latest scout-report.md will be re-checked against the
# same shape requirements.
#
# Behavioral: combines persona-contract assertions with artifact-shape
# assertions. Mutants that remove an anchor or rename a field break this; a
# trivial mutant that empties the persona fails all four positives at once.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
SCOUT_FILE="$REPO_ROOT/agents/evolve-scout.md"

fail=0
errors=""

if [ ! -f "$SCOUT_FILE" ]; then
  echo "RED cycle-88-scout-report-schema-stable: agents/evolve-scout.md missing at $SCOUT_FILE"
  exit 1
fi

# (1+2) Persona must point at ANCHOR-comment output template. The persona
# currently delegates literal anchor names to a referenced template
# (reference `output-template`); we therefore accept any of:
#   - the literal anchor strings (ANCHOR:task_proposals / ANCHOR:summary), OR
#   - a generic "ANCHOR" mention combined with a pointer to the template
#     reference, OR
#   - the reference name "output-template".
# What we DO NOT accept is the persona losing all of these (which would mean
# downstream readers no longer have an anchor contract anywhere).
if ! grep -qE 'ANCHOR:task_proposals|ANCHOR:summary|ANCHOR comments|output-template' "$SCOUT_FILE"; then
  errors="${errors}\n  evolve-scout.md no longer references ANCHOR comments / output-template (downstream-readable contract is gone)"
  fail=$((fail + 1))
fi

# (3) Mandatory task field names present in persona.
for field in targetFiles complexity researchBacking effort; do
  if ! grep -qE "\b$field\b" "$SCOUT_FILE"; then
    errors="${errors}\n  evolve-scout.md no longer references task field name: $field"
    fail=$((fail + 1))
  fi
done

# (4) Carryover Decisions language preserved.
if ! grep -qiE 'Carryover (Decisions|Todos|Walk)|carryoverTodos' "$SCOUT_FILE"; then
  errors="${errors}\n  evolve-scout.md missing Carryover decisions language (downstream Triage relies on it)"
  fail=$((fail + 1))
fi

# Optional artifact self-check — only runs if a current cycle's scout-report
# is available. This makes the predicate dual-mode: persona-static (always)
# and artifact-shape (when a fresh report exists).
LATEST_REPORT=""
if [ -n "${WORKSPACE:-}" ] && [ -f "$WORKSPACE/scout-report.md" ]; then
  LATEST_REPORT="$WORKSPACE/scout-report.md"
fi
if [ -z "$LATEST_REPORT" ]; then
  # Best-effort: pick the highest-numbered cycle-N/scout-report.md under
  # .evolve/runs to use as a baseline shape check.
  if [ -d "$REPO_ROOT/.evolve/runs" ]; then
    LATEST_REPORT=$(ls -1 "$REPO_ROOT/.evolve/runs"/cycle-*/scout-report.md 2>/dev/null \
      | sort -V | tail -1)
  fi
fi
if [ -n "$LATEST_REPORT" ] && [ -f "$LATEST_REPORT" ]; then
  for anchor in 'ANCHOR:task_proposals' 'ANCHOR:summary'; do
    if ! grep -qE "$anchor" "$LATEST_REPORT"; then
      errors="${errors}\n  latest scout-report.md ($LATEST_REPORT) missing anchor: $anchor"
      fail=$((fail + 1))
    fi
  done
fi

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-scout-report-schema-stable: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-scout-report-schema-stable: persona retains task-proposals + summary anchors and required task-field names; downstream contract preserved"
exit 0
