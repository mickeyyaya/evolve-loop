#!/usr/bin/env bash
# AC-ID: cycle-90-004-knowledge-stewardship-rule
# Description: Verifies that AGENTS.md contains the Knowledge Stewardship Rule
#   (Day-One) — as a numbered cross-CLI invariant matching Plan §5D blockquote
#   verbatim on its load-bearing phrases. Placement in AGENTS.md (not CLAUDE.md)
#   is intentional: rule must apply to Gemini and Codex agents too once Phase 0
#   lands.
# Evidence: intent.md success-criteria row "grep -q 'Knowledge Stewardship
#   Rule' AGENTS.md AND rule text matches Plan §5D verbatim";
#   plan §5D blockquote text at lines 467-469 of the plan file.
# Author: tdd-engineer (cycle-90)
# Created: 2026-05-19
# Acceptance-of: build-report.md row "5D: Knowledge Stewardship Rule codified
#   in AGENTS.md as numbered cross-CLI invariant"
#
# Behavioral: checks for FIVE load-bearing phrases that together encode the
# rule's semantics — not a single grep on the title. Even a mutant that
# inserts the heading but elides the body cannot satisfy all five phrases.
# Phrases chosen are the actionable clauses of the rule (location guidance,
# never-delete policy, archival path format, severity classification).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"
AGENTS_MD="$REPO_ROOT/AGENTS.md"
AC_ID="cycle-90-004-knowledge-stewardship-rule"

if [ ! -f "$AGENTS_MD" ]; then
  echo "RED $AC_ID: AGENTS.md not found at $AGENTS_MD" >&2
  exit 1
fi

# Five load-bearing phrases from Plan §5D (verbatim, case-sensitive). Each one
# tests a distinct semantic clause of the rule; missing any one means the rule
# is incomplete.
declare_phrases() {
  cat <<'EOF'
Knowledge Stewardship Rule
docs/research/
knowledge-base/research/
Never delete; always archive
HIGH-severity audit defect
EOF
}

missing=""
while IFS= read -r phrase; do
  [ -z "$phrase" ] && continue
  # grep -F: literal (no regex). -q: quiet. We deliberately do not use word
  # boundaries because the canonical text embeds these phrases mid-sentence.
  if ! grep -Fq "$phrase" "$AGENTS_MD"; then
    missing="${missing}\n  missing phrase: ${phrase}"
  fi
done < <(declare_phrases)

if [ -n "$missing" ]; then
  echo "RED $AC_ID: AGENTS.md missing load-bearing stewardship-rule phrases" >&2
  printf "%b\n" "$missing" >&2
  exit 1
fi

# Additionally verify the rule lives under AGENTS.md (not just a stray
# mention) by requiring the title phrase be on its own heading-style line OR
# adjacent to a blockquote/list marker — i.e., visually delineated. This
# prevents a mutant from passing by burying the words inside an unrelated
# paragraph.
if ! grep -nE '^(#+|>|[*-]) .*Knowledge Stewardship Rule' "$AGENTS_MD" >/dev/null 2>&1; then
  echo "RED $AC_ID: 'Knowledge Stewardship Rule' present but not in a heading/list/blockquote line" >&2
  exit 1
fi

echo "GREEN $AC_ID: AGENTS.md codifies Knowledge Stewardship Rule (5/5 phrases + delineated heading)"
exit 0
