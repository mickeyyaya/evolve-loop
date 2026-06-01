#!/usr/bin/env bash
# ACS cycle-187 — Task 2 AC-3/AC-4: self-healing-gaps.md records the cycle-187
# work and annotates the deferred gaps.
#
# acs-predicate: doc-check
# Documentation-content criteria (no runtime to invoke): the doc must (AC-3)
# reference the retro/build-planner backfill coverage added this cycle, and
# (AC-4) carry an explicit deferred / by-design annotation on the gaps Scout
# chose NOT to fix (3, 4, 7, 8). Content greps are the correct waived tool.
set -uo pipefail
DOC="$(git rev-parse --show-toplevel)/docs/architecture/self-healing-gaps.md"

[ -f "$DOC" ] || { echo "RED: $DOC missing" >&2; exit 1; }

# AC-3: the new retro + build-planner backfill coverage is referenced.
grep -qiE 'retro|retrospective' "$DOC" \
  || { echo "RED: self-healing-gaps.md does not reference retro backfill (T2 AC-3)" >&2; exit 1; }
grep -qi 'build-planner' "$DOC" \
  || { echo "RED: self-healing-gaps.md does not reference build-planner backfill (T2 AC-3)" >&2; exit 1; }

# AC-4: deferred / by-design annotation must be present (the deliberate
# non-fix of gaps 3/4/7/8 must be documented, not silent).
grep -qiE 'deferred|by[- ]design' "$DOC" \
  || { echo "RED: self-healing-gaps.md lacks a deferred/by-design annotation (T2 AC-4)" >&2; exit 1; }

echo "PASS: self-healing-gaps.md updated for cycle-187 (T2 AC-3/AC-4)"
exit 0
