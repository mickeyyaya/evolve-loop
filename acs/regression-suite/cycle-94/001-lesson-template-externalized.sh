#!/usr/bin/env bash
# AC-ID: cycle-94-001-lesson-template-externalized
# AC-source: cycle-94/intent.md acceptance_check #3 + #4
# Behavioral predicate: P5 — retrospective YAML template lives at
#   skills/evolve-loop/lesson-template.yaml and NOT inline in
#   agents/evolve-retrospective.md. The retrospective profile must
#   reference Read(skills/evolve-loop/lesson-template.yaml) explicitly.
#
# Rationale: cycle-94 P5 externalizes the retrospective lesson schema
# so the persona prompt sheds N tokens per retrospective invocation.
# The template lives in skills/, the persona Reads it, and the profile
# documents the dependency explicitly.
#
# RED before Builder applies the diff; GREEN after.
# Bash 3.2 compatible.
#
# Exit codes:
#   0 = GREEN (all four checks pass)
#   1 = RED   (any check fails)
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null)}}"
if [ -z "$REPO_ROOT" ]; then
  echo "RED: not inside a git work tree" >&2
  exit 1
fi
cd "$REPO_ROOT" || { echo "RED: cd failed" >&2; exit 1; }

TEMPLATE="skills/evolve-loop/lesson-template.yaml"
PERSONA="agents/evolve-retrospective.md"
PROFILE=".evolve/profiles/retrospective.json"

# Check 1: template exists and is non-empty
if [ ! -s "$TEMPLATE" ]; then
  echo "RED: $TEMPLATE missing or empty" >&2
  exit 1
fi

# Check 2: persona references the externalized template by relative path
if ! grep -Fq 'skills/evolve-loop/lesson-template.yaml' "$PERSONA"; then
  echo "RED: $PERSONA does not reference skills/evolve-loop/lesson-template.yaml" >&2
  exit 1
fi

# Check 3: persona no longer contains the stale "Use the schema below" phrase.
# The schema is no longer "below" — it lives in the external template.
if grep -Fq 'Use the schema below' "$PERSONA"; then
  echo "RED: $PERSONA still contains stale text 'Use the schema below'" >&2
  grep -n 'Use the schema below' "$PERSONA" >&2 || true
  exit 1
fi

# Check 4: profile allowed_tools contains explicit Read(<template>) entry.
# Use jq for structured query — falls back to grep if jq unavailable.
if command -v jq >/dev/null 2>&1; then
  if ! jq -e '.allowed_tools | index("Read(skills/evolve-loop/lesson-template.yaml)")' \
       "$PROFILE" >/dev/null 2>&1; then
    echo "RED: $PROFILE allowed_tools missing Read(skills/evolve-loop/lesson-template.yaml)" >&2
    exit 1
  fi
else
  if ! grep -Fq '"Read(skills/evolve-loop/lesson-template.yaml)"' "$PROFILE"; then
    echo "RED: $PROFILE allowed_tools missing Read(skills/evolve-loop/lesson-template.yaml)" >&2
    exit 1
  fi
fi

echo "GREEN: lesson-template externalized; persona updated; profile allows explicit Read"
exit 0
