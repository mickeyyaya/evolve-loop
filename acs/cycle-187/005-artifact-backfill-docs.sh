#!/usr/bin/env bash
# ACS cycle-187 — Task 2 AC-1/AC-2: artifact-backfill.md documents the newly
# covered retro + build-planner phases.
#
# acs-predicate: doc-check
# This is a documentation-content criterion: the Header Map table in
# artifact-backfill.md must now list retro and build-planner. A doc file has no
# runtime behavior to invoke, so a content grep is the correct (waived) tool.
# The behavioral backfill changes themselves are pinned by predicate 001.
set -uo pipefail
DOC="$(git rev-parse --show-toplevel)/docs/architecture/artifact-backfill.md"

[ -f "$DOC" ] || { echo "RED: $DOC missing" >&2; exit 1; }

# AC-1: retrospective/retro coverage documented.
grep -qiE 'retro|retrospective' "$DOC" \
  || { echo "RED: artifact-backfill.md does not mention retro (T2 AC-1)" >&2; exit 1; }
# AC-2: build-planner coverage documented.
grep -qi 'build-planner' "$DOC" \
  || { echo "RED: artifact-backfill.md does not mention build-planner (T2 AC-2)" >&2; exit 1; }
# Anchor the new phases to their artifact filenames so the table row is real,
# not a passing-mention.
grep -qi 'retrospective-report.md' "$DOC" \
  || { echo "RED: artifact-backfill.md missing retrospective-report.md filename" >&2; exit 1; }
grep -qi 'build-plan.md' "$DOC" \
  || { echo "RED: artifact-backfill.md missing build-plan.md filename" >&2; exit 1; }

echo "PASS: artifact-backfill.md documents retro + build-planner (T2 AC-1/AC-2)"
exit 0
