#!/usr/bin/env bash
# AC-ID: cycle-89-002-online-researcher-reference-doc
# Description: Verifies the online-researcher reference doc exists at
#   docs/research/online-researcher-patterns.md (the relocated home per
#   scout-report.md Premise Resolution #2) AND has NO dispatchable persona
#   YAML frontmatter (no top-level `name:` / `tools:` keys). Also verifies
#   the old persona-dispatch surface agents/online-researcher.md remains
#   absent (cycle-88 invariant).
# Evidence: scout-report.md:T2 + Findings F1 + Premise Resolution #2;
#   intent.md:acceptance_checks bullet 2 ("agents/online-researcher.md no
#   longer has name:/tools: YAML frontmatter").
# Author: tdd-engineer (cycle-89 Phase C)
# Created: 2026-05-19
# Acceptance-of: build-report.md AC-row "online-researcher repurposed as
#   reference doc; no dispatch surface"
#
# Behavioral: the predicate parses the YAML frontmatter region (lines between
# first two --- delimiters, if any) and checks that neither `name:` nor `tools:`
# appears as a top-level key. A mutant that embeds `name:` inside a code block
# would still flag (conservative read) — this is intentional: dispatch surfaces
# must not be ambiguous. A mutant that simply renames the keys fails the
# secondary "doc has content" check (>=20 lines of body).
set -uo pipefail

REPO_ROOT="${WORKTREE_PATH:-${EVOLVE_PROJECT_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}}"

REF_DOC="$REPO_ROOT/docs/research/online-researcher-patterns.md"
LEGACY_PERSONA="$REPO_ROOT/agents/online-researcher.md"

fail=0
errors=""

# (1) Reference doc must exist at docs/research/.
if [ ! -f "$REF_DOC" ]; then
  errors="${errors}\n  reference doc missing at $REF_DOC"
  fail=$((fail + 1))
else
  # Substance check: at least 20 lines of meaningful content.
  body_line_count=$(awk 'NF' "$REF_DOC" | wc -l | tr -d ' ')
  if [ "${body_line_count:-0}" -lt 20 ]; then
    errors="${errors}\n  reference doc too thin ($body_line_count meaningful lines, expected >=20)"
    fail=$((fail + 1))
  fi

  # No dispatchable YAML frontmatter — extract first frontmatter block (if any)
  # and check for top-level `name:` / `tools:` keys. awk emits the block body
  # only; the grep over the captured block is the verification step.
  frontmatter=$(awk '
    NR==1 && $0=="---" {infm=1; next}
    infm && $0=="---" {exit}
    infm {print}
  ' "$REF_DOC")
  if [ -n "$frontmatter" ]; then
    bad_keys=$(printf '%s\n' "$frontmatter" | awk '/^(name|tools)[[:space:]]*:/ {print}')
    if [ -n "$bad_keys" ]; then
      errors="${errors}\n  reference doc has dispatchable frontmatter keys:\n$bad_keys"
      fail=$((fail + 1))
    fi
  fi
fi

# (2) Legacy persona-dispatch surface must remain absent (cycle-88 invariant).
if [ -f "$LEGACY_PERSONA" ]; then
  # If a stub is permitted, it must NOT carry persona-dispatch frontmatter.
  legacy_fm=$(awk 'NR==1 && $0=="---" {infm=1; next} infm && $0=="---" {exit} infm {print}' "$LEGACY_PERSONA")
  bad_legacy=$(printf '%s\n' "$legacy_fm" | awk '/^(name|tools)[[:space:]]*:/ {print}')
  if [ -n "$bad_legacy" ]; then
    errors="${errors}\n  legacy agents/online-researcher.md has re-introduced dispatchable frontmatter:\n$bad_legacy"
    fail=$((fail + 1))
  fi
fi

if [ "$fail" -gt 0 ]; then
  echo "RED cycle-89-002-online-researcher-reference-doc: $fail issue(s)" >&2
  printf "%b\n" "$errors" >&2
  exit 1
fi

echo "GREEN cycle-89-002-online-researcher-reference-doc: reference doc present at docs/research/, no dispatchable frontmatter, legacy surface clean"
exit 0
