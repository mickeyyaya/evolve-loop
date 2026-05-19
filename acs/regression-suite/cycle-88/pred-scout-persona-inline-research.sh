#!/usr/bin/env bash
# AC-ID: cycle-88-scout-persona-inline-research
#
# Verifies Cycle B persona edit on agents/evolve-scout.md:
#   NEGATIVES (must be GONE):
#     1. No "research-brief.md" references (Phase 1 brief path is retired).
#     2. No "Phase 1" mentions (Phase 1 is no longer scheduled).
#     3. No "research-cache-protocol" or "research-cache-staging" pointers
#        (Phase B cache scaffolding is part of the retired research phase).
#     4. No "Read Research Brief" section heading (the old §5).
#     5. No "Phase 1 concept cards" boost language.
#
#   POSITIVES (must be PRESENT):
#     6. Inline research directive: a heading or paragraph that explicitly
#        instructs Scout to do upfront research itself (one of "inline" +
#        "research" appearing in the same line or adjacent context — looking
#        for a deliberate Cycle-B directive, not incidental matches).
#     7. Quota awareness: explicit reference to all three research tools by
#        name — WebSearch, WebFetch, and kb-search.sh (the latter from Cycle
#        A's helper script).
#     8. Downstream schema contract preserved: persona still names the three
#        critical task-proposal fields — `targetFiles`, `complexity`,
#        `researchBacking` — so Triage/TDD/Builder downstream readers do not
#        regress.
#
# Behavioral: combines forbidden-token grep (negatives) with required-token grep
# AND co-occurrence (positives). Mutants that just delete the brief-read
# section without adding the inline directive fail (6)/(7); mutants that add an
# inline section but drop the schema field names fail (8); mutants that just
# rename "Phase 1" to "Phase one" fail (2) only if exact, but criterion (1)
# (research-brief.md) is a separate axis hard to spoof.
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
SCOUT_FILE="$REPO_ROOT/agents/evolve-scout.md"

if [ ! -f "$SCOUT_FILE" ]; then
  echo "RED cycle-88-scout-persona-inline-research: agents/evolve-scout.md missing at $SCOUT_FILE"
  exit 1
fi

fail=0
errors=""

# ── NEGATIVES ──
forbidden_patterns=(
  "research-brief\.md"
  "Phase 1"
  "research-cache-protocol"
  "research-cache-staging"
  "Read Research Brief"
)
for pat in "${forbidden_patterns[@]}"; do
  if grep -qE "$pat" "$SCOUT_FILE"; then
    errors="${errors}\n  evolve-scout.md still mentions forbidden pattern: $pat"
    fail=$((fail + 1))
  fi
done

# Phase-1 concept-cards boost language: looser regex so a renamed variant
# (e.g., "Phase 1 Concept Candidates: +2") still trips. We accept ANY of:
#   "Phase 1 Concept", "Phase 1 concept cards"
if grep -qiE "Phase[[:space:]]+1[[:space:]]+[Cc]oncept" "$SCOUT_FILE"; then
  errors="${errors}\n  evolve-scout.md still references 'Phase 1 Concept[...]' boost"
  fail=$((fail + 1))
fi

# ── POSITIVES ──
# (6) Inline research directive: look for an explicit heading or directive line
# containing 'inline' near 'research' (case-insensitive). We require BOTH:
#   - A heading with "Inline" in it OR
#   - A line containing both "inline" and "research"
inline_heading=$(grep -ciE '^#+[[:space:]].*inline.*research' "$SCOUT_FILE" 2>/dev/null || echo 0)
inline_line=$(grep -ciE 'inline.*research|research.*inline' "$SCOUT_FILE" 2>/dev/null || echo 0)
if [ "${inline_heading:-0}" -lt 1 ] 2>/dev/null && [ "${inline_line:-0}" -lt 1 ] 2>/dev/null; then
  errors="${errors}\n  evolve-scout.md missing inline-research directive (need either a heading containing 'inline research' or a line co-mentioning inline + research)"
  fail=$((fail + 1))
fi

# (7) Quota awareness — all three research tool names.
for tool in WebSearch WebFetch kb-search; do
  if ! grep -qE "$tool" "$SCOUT_FILE"; then
    errors="${errors}\n  evolve-scout.md missing reference to research tool: $tool"
    fail=$((fail + 1))
  fi
done

# (7a) Numeric quota mention — at least one of "quota", "≤", "max", or a number
# adjacent to a tool name. We loosely look for the literal word "quota" since
# the intent explicitly calls for "explicit quota awareness".
if ! grep -qiE 'quota' "$SCOUT_FILE"; then
  errors="${errors}\n  evolve-scout.md does not mention 'quota' (intent requires explicit quota awareness)"
  fail=$((fail + 1))
fi

# (8) Downstream schema preserved.
for field in targetFiles complexity researchBacking; do
  if ! grep -qE "\b$field\b" "$SCOUT_FILE"; then
    errors="${errors}\n  evolve-scout.md no longer references downstream task field: $field (schema regression)"
    fail=$((fail + 1))
  fi
done

if [ $fail -gt 0 ]; then
  echo "RED cycle-88-scout-persona-inline-research: $fail issue(s)"
  printf "%b\n" "$errors" >&2
  exit 1
fi
echo "GREEN cycle-88-scout-persona-inline-research: Phase 1 + research-brief refs purged; inline-research directive + WebSearch/WebFetch/kb-search quota awareness present; downstream schema preserved"
exit 0
